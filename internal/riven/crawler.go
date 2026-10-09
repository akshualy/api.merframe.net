package riven

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/akshualy/api.merframe.net/internal/throttle"
	"github.com/akshualy/api.merframe.net/internal/wfm"
)

type AuctionSource interface {
	SearchAuctions(ctx context.Context, search wfm.AuctionSearch) ([]wfm.Auction, error)
}

const (
	searchCap      = 500
	noWeaponsPause = time.Minute
)

var polarities = []string{"madurai", "vazarin", "naramon", "zenurik"}

var rerollBuckets = []struct{ low, high *int }{
	{new(0), new(0)},
	{new(1), new(4)},
	{new(5), new(14)},
	{new(15), new(49)},
	{new(50), nil},
}

type Crawler struct {
	source       AuctionSource
	store        Store
	publisher    Publisher
	pace         *throttle.Throttle
	passInterval time.Duration
	log          *slog.Logger

	requests int
}

func NewCrawler(source AuctionSource, store Store, publisher Publisher, spacing, passInterval time.Duration, log *slog.Logger) *Crawler {
	return &Crawler{
		source:       source,
		store:        store,
		publisher:    publisher,
		pace:         throttle.Every(spacing),
		passInterval: passInterval,
		log:          log,
	}
}

func (c *Crawler) Run(ctx context.Context) error {
	for {
		data, err := c.store.Riven(ctx)
		if err != nil {
			return fmt.Errorf("loading the riven document: %w", err)
		}

		if len(data.Weapons) == 0 {
			if err := throttle.Sleep(ctx, noWeaponsPause); err != nil {
				return err
			}
			continue
		}

		elapsed, err := c.runPass(ctx, data.Weapons)
		if err != nil {
			return err
		}

		if err := throttle.Sleep(ctx, c.passInterval-elapsed); err != nil {
			return err
		}
	}
}

func (c *Crawler) runPass(ctx context.Context, weapons []Weapon) (time.Duration, error) {
	started := time.Now()
	c.requests = 0

	crawled := 0
	for _, weapon := range weapons {
		auctions, err := c.crawl(ctx, weapon.Slug)
		if err != nil {
			if ctx.Err() != nil {
				return 0, ctx.Err()
			}

			c.log.Warn("crawling auctions failed", "slug", weapon.Slug, "error", err)
			continue
		}

		if err := c.publisher.Auctions(ctx, auctions); err != nil {
			return 0, fmt.Errorf("publishing the auctions of %s: %w", weapon.Slug, err)
		}

		crawled++
	}

	elapsed := time.Since(started)
	c.log.Info("riven auction pass finished", "crawled", crawled, "weapons", len(weapons), "requests", c.requests, "seconds", int(elapsed.Seconds()))
	return elapsed, nil
}

// crawl filters by polarity and reroll ranges. Every search is cheapest first, so a capped range only has listings a price comparison needs.
func (c *Crawler) crawl(ctx context.Context, slug string) (WeaponAuctions, error) {
	found := map[string]wfm.Auction{}
	var order []string
	collect := func(results []wfm.Auction) {
		for _, auction := range results {
			if _, seen := found[auction.ID]; !seen {
				order = append(order, auction.ID)
			}

			found[auction.ID] = auction
		}
	}

	whole, err := c.search(ctx, wfm.AuctionSearch{Weapon: slug})
	if err != nil {
		return WeaponAuctions{}, err
	}

	collect(whole)
	truncated := false
	if len(whole) >= searchCap {
		for _, polarity := range polarities {
			byPolarity, err := c.search(ctx, wfm.AuctionSearch{Weapon: slug, Polarity: polarity})
			if err != nil {
				return WeaponAuctions{}, err
			}

			collect(byPolarity)
			if len(byPolarity) < searchCap {
				continue
			}

			for _, bucket := range rerollBuckets {
				results, err := c.search(ctx, wfm.AuctionSearch{Weapon: slug, Polarity: polarity, ReRollsMin: bucket.low, ReRollsMax: bucket.high})
				if err != nil {
					return WeaponAuctions{}, err
				}

				collect(results)
				truncated = truncated || len(results) >= searchCap
			}
		}
	}

	raw := make([]wfm.Auction, len(order))
	for i, id := range order {
		raw[i] = found[id]
	}
	return WeaponAuctions{Slug: slug, UpdatedAt: time.Now(), Truncated: truncated, Auctions: openAuctions(raw)}, nil
}

func (c *Crawler) search(ctx context.Context, search wfm.AuctionSearch) ([]wfm.Auction, error) {
	if err := c.pace.Wait(ctx); err != nil {
		return nil, err
	}

	c.requests++
	return c.source.SearchAuctions(ctx, search)
}
