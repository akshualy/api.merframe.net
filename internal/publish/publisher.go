package publish

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/akshualy/api.merframe.net/internal/blob"
	"github.com/akshualy/api.merframe.net/internal/market"
	"github.com/akshualy/api.merframe.net/internal/riven"
	"github.com/akshualy/api.merframe.net/internal/throttle"
)

type Store interface {
	Prices(ctx context.Context) (map[string]market.Price, error)
	SaveBlob(ctx context.Context, name string, b *blob.Blob, publishedAt time.Time) error
}

const (
	PricesBlob  = "prices"
	RivenBlob   = "riven"
	CatalogBlob = "items"
)

const firstPublishRetry = time.Minute

func AuctionsBlob(slug string) string {
	return "auctions/" + slug
}

type Publisher struct {
	store     Store
	interval  time.Duration
	log       *slog.Logger
	published map[string]string
}

func New(store Store, interval time.Duration, log *slog.Logger) *Publisher {
	return &Publisher{store: store, interval: interval, log: log, published: map[string]string{}}
}

// Run checks every minute until the first table is published. A new deploy is ready once the prices exist.
func (p *Publisher) Run(ctx context.Context) error {
	for {
		if err := p.prices(ctx); err != nil {
			return err
		}

		wait := p.interval
		if p.published[PricesBlob] == "" {
			wait = firstPublishRetry
		}

		if err := throttle.Sleep(ctx, wait); err != nil {
			return err
		}
	}
}

func (p *Publisher) prices(ctx context.Context) error {
	prices, err := p.store.Prices(ctx)
	if err != nil {
		return fmt.Errorf("loading the price table: %w", err)
	}

	if len(prices) == 0 {
		return nil
	}

	body, err := encodePrices(prices)
	if err != nil {
		return err
	}
	return p.save(ctx, PricesBlob, body)
}

func (p *Publisher) Riven(ctx context.Context, d riven.Document) error {
	body, err := encodeRiven(d)
	if err != nil {
		return err
	}
	return p.save(ctx, RivenBlob, body)
}

func (p *Publisher) Catalog(ctx context.Context, c market.Catalog) error {
	body, err := encodeCatalog(c)
	if err != nil {
		return err
	}
	return p.save(ctx, CatalogBlob, body)
}

func (p *Publisher) Auctions(ctx context.Context, w riven.WeaponAuctions) error {
	body, err := encodeAuctions(w)
	if err != nil {
		return err
	}
	return p.save(ctx, AuctionsBlob(w.Slug), body)
}

// save skips a body that has the same hash as the last one. Listeners are only informed about changes.
func (p *Publisher) save(ctx context.Context, name string, body []byte) error {
	etag := blob.ETagOf(body)
	if p.published[name] == etag {
		return nil
	}

	b := blob.Build(body)

	if err := p.store.SaveBlob(ctx, name, b, time.Now()); err != nil {
		return fmt.Errorf("publishing %s: %w", name, err)
	}

	p.published[name] = etag
	p.log.Debug("blob published", "name", name, "bytes", len(body), "brotli", len(b.Brotli))
	return nil
}
