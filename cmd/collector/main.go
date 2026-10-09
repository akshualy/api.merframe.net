package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/akshualy/api.merframe.net/internal/config"
	"github.com/akshualy/api.merframe.net/internal/market"
	"github.com/akshualy/api.merframe.net/internal/postgres"
	"github.com/akshualy/api.merframe.net/internal/publish"
	"github.com/akshualy/api.merframe.net/internal/riven"
	"github.com/akshualy/api.merframe.net/internal/sheets"
	"github.com/akshualy/api.merframe.net/internal/throttle"
	"github.com/akshualy/api.merframe.net/internal/wfm"
)

const httpTimeout = 10 * time.Second

func main() {
	if err := run(context.Background(), os.Getenv, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, getenv func(string) string, stdout io.Writer) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.LoadCollector(getenv)
	if err != nil {
		return err
	}

	log := slog.New(slog.NewJSONHandler(stdout, nil))

	db, err := postgres.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := db.Migrate(ctx); err != nil {
		return err
	}

	hc := &http.Client{Timeout: httpTimeout}
	shared := throttle.New(cfg.MarketRate)
	publisher := publish.New(db, cfg.PublishInterval, log.With("loop", "publisher"))
	scraper := market.NewScraper(
		wfm.New(hc, shared, "price scraper", cfg.Contact),
		db,
		publisher,
		market.Intervals{
			Prime:      cfg.PrimeInterval,
			Hot:        cfg.HotInterval,
			Cold:       cfg.ColdInterval,
			Statistics: cfg.StatisticsInterval,
			Catalog:    cfg.CatalogInterval,
		},
		log.With("loop", "prices"),
	)
	refresher := riven.NewRefresher(
		wfm.New(hc, shared, "riven data", cfg.Contact),
		sheets.New(hc, "riven data", cfg.Contact),
		db,
		publisher,
		cfg.RivenInterval,
		log.With("loop", "riven"),
	)
	crawler := riven.NewCrawler(
		wfm.New(hc, shared, "riven auctions", cfg.Contact),
		db,
		publisher,
		cfg.AuctionSpacing,
		cfg.AuctionPassInterval,
		log.With("loop", "auctions"),
	)
	log.Info("collector started", "market_rate", cfg.MarketRate, "publish_interval", cfg.PublishInterval.String())

	group, gctx := errgroup.WithContext(ctx)
	group.Go(func() error { return scraper.Run(gctx) })
	group.Go(func() error { return refresher.Run(gctx) })
	group.Go(func() error { return crawler.Run(gctx) })
	group.Go(func() error { return publisher.Run(gctx) })
	if err := group.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}

	log.Info("collector stopped")
	return nil
}
