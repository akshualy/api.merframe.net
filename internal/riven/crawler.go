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

// Demand is what the api was asked for. A weapon is due when it was asked for after its last crawl and that crawl is older than before; weapons never crawled come first.
type Demand interface {
	DueAuctions(ctx context.Context, before time.Time) ([]string, error)
	MarkAuctionsCrawled(ctx context.Context, slug string, at time.Time) error
}

const (
	searchCap        = 500
	demandCheckPause = 10 * time.Second
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
	source    AuctionSource
	demand    Demand
	publisher Publisher
	pace      *throttle.Throttle
	interval  time.Duration
	log       *slog.Logger

	requests int
}

func NewCrawler(source AuctionSource, demand Demand, publisher Publisher, spacing, interval time.Duration, log *slog.Logger) *Crawler {
	return &Crawler{
		source:    source,
		demand:    demand,
		publisher: publisher,
		pace:      throttle.Every(spacing),
		interval:  interval,
		log:       log,
	}
}

// Run takes one weapon at a time, so a weapon asked for during a crawl is not queued behind the rest of the list.
func (c *Crawler) Run(ctx context.Context) error {
	for {
		due, err := c.demand.DueAuctions(ctx, time.Now().Add(-c.interval))
		if err != nil {
			return fmt.Errorf("loading the wanted weapons: %w", err)
		}

		if len(due) > 0 {
			if err := c.crawlWeapon(ctx, due[0]); err != nil {
				return err
			}
			continue
		}

		if err := throttle.Sleep(ctx, demandCheckPause); err != nil {
			return err
		}
	}
}

// crawlWeapon marks a failed crawl as done too, so the weapon waits for the interval and a new request like any other.
func (c *Crawler) crawlWeapon(ctx context.Context, slug string) error {
	c.requests = 0
	auctions, err := c.crawl(ctx, slug)
	if ctx.Err() != nil {
		return ctx.Err()
	}

	if err != nil {
		c.log.Warn("crawling auctions failed", "slug", slug, "error", err)
		return c.demand.MarkAuctionsCrawled(ctx, slug, time.Now())
	}

	if err := c.publisher.Auctions(ctx, auctions); err != nil {
		return fmt.Errorf("publishing the auctions of %s: %w", slug, err)
	}

	c.log.Info("weapon auctions crawled", "slug", slug, "listings", len(auctions.Auctions), "requests", c.requests)
	return c.demand.MarkAuctionsCrawled(ctx, slug, auctions.UpdatedAt)
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
