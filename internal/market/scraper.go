package market

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/akshualy/api.merframe.net/internal/throttle"
	"github.com/akshualy/api.merframe.net/internal/wfm"
)

type Source interface {
	Items(ctx context.Context) (wfm.Catalog, error)
	Orders(ctx context.Context, slug string) ([]wfm.Order, error)
	Statistics(ctx context.Context, slug string) (wfm.ClosedStatistics, error)
}

type Store interface {
	Catalog(ctx context.Context) (Catalog, error)
	ReplaceCatalog(ctx context.Context, c Catalog) (dropped int64, err error)
	Prices(ctx context.Context) (map[string]Price, error)
	SavePrice(ctx context.Context, slug string, p Price, days []Day) error
}

type Publisher interface {
	Catalog(ctx context.Context, c Catalog) error
}

type Intervals struct {
	Prime      time.Duration
	Hot        time.Duration
	Cold       time.Duration
	Statistics time.Duration
	Catalog    time.Duration
}

const (
	idlePause         = 15 * time.Second
	catalogRetryPause = 5 * time.Minute
	failedRetryPause  = 5 * time.Minute
)

type Scraper struct {
	source    Source
	store     Store
	publisher Publisher
	intervals Intervals
	log       *slog.Logger

	catalog    Catalog
	items      map[string]wfm.Item
	prime      map[string]bool
	prices     map[string]Price
	untradable map[string]bool
	failedAt   map[string]time.Time
}

func NewScraper(source Source, store Store, publisher Publisher, intervals Intervals, log *slog.Logger) *Scraper {
	return &Scraper{
		source:     source,
		store:      store,
		publisher:  publisher,
		intervals:  intervals,
		log:        log,
		items:      map[string]wfm.Item{},
		prime:      map[string]bool{},
		untradable: map[string]bool{},
		failedAt:   map[string]time.Time{},
	}
}

func (s *Scraper) Run(ctx context.Context) error {
	if err := s.load(ctx); err != nil {
		return err
	}

	for {
		if s.catalogStale(time.Now()) {
			if err := s.refreshCatalog(ctx); err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}

				s.log.Warn("refreshing the item list failed", "error", err)
				if err := throttle.Sleep(ctx, catalogRetryPause); err != nil {
					return err
				}
				continue
			}
		}

		due := s.due(time.Now())
		if len(due) == 0 {
			if err := throttle.Sleep(ctx, idlePause); err != nil {
				return err
			}
			continue
		}

		if err := s.reprice(ctx, due); err != nil {
			return err
		}
	}
}

func (s *Scraper) load(ctx context.Context) error {
	catalog, err := s.store.Catalog(ctx)
	if err != nil {
		return fmt.Errorf("loading the item list: %w", err)
	}

	s.setCatalog(catalog)

	prices, err := s.store.Prices(ctx)
	if err != nil {
		return fmt.Errorf("loading the price table: %w", err)
	}

	s.prices = prices

	if len(catalog.Items) > 0 {
		if err := s.publisher.Catalog(ctx, catalog); err != nil {
			return fmt.Errorf("publishing the item list: %w", err)
		}
	}

	s.log.Info("price table loaded", "items", len(prices), "known", len(s.items))
	return nil
}

func (s *Scraper) setCatalog(c Catalog) {
	s.catalog = c
	s.items = make(map[string]wfm.Item, len(c.Items))
	s.prime = map[string]bool{}

	for _, item := range c.Items {
		s.items[item.Slug] = item
		if slices.Contains(item.Tags, "prime") {
			s.prime[item.Slug] = true
		}
	}
}

func (s *Scraper) catalogStale(now time.Time) bool {
	return now.Sub(s.catalog.RefreshedAt) > s.intervals.Catalog
}

func (s *Scraper) refreshCatalog(ctx context.Context) error {
	fetched, err := s.source.Items(ctx)
	if err != nil {
		return err
	}

	catalog := Catalog{Catalog: fetched, RefreshedAt: time.Now()}
	dropped, err := s.store.ReplaceCatalog(ctx, catalog)
	if err != nil {
		return fmt.Errorf("saving the item list: %w", err)
	}

	if err := s.publisher.Catalog(ctx, catalog); err != nil {
		return fmt.Errorf("publishing the item list: %w", err)
	}

	s.setCatalog(catalog)
	clear(s.untradable)
	clear(s.failedAt)

	for slug := range s.prices {
		if _, known := s.items[slug]; !known {
			delete(s.prices, slug)
		}
	}

	s.log.Info("item list refreshed", "items", len(s.items), "dropped", dropped)
	return nil
}

type dueItem struct {
	slug      string
	withStats bool
}

// due lists items past their tier interval or with stale statistics. Primes first, then oldest first.
func (s *Scraper) due(now time.Time) []dueItem {
	type candidate struct {
		prime     bool
		updatedAt time.Time
		dueItem
	}
	var found []candidate
	for slug := range s.items {
		if s.untradable[slug] || now.Sub(s.failedAt[slug]) < failedRetryPause {
			continue
		}

		prime := s.prime[slug]
		price, priced := s.prices[slug]
		if !priced {
			found = append(found, candidate{prime: prime, dueItem: dueItem{slug: slug, withStats: true}})
			continue
		}

		stale := now.Sub(price.UpdatedAt) >= s.tierInterval(price.Tier)
		statsStale := now.Sub(price.StatisticsAt) >= s.intervals.Statistics
		if stale || statsStale {
			found = append(found, candidate{prime: prime, updatedAt: price.UpdatedAt, dueItem: dueItem{slug: slug, withStats: statsStale}})
		}
	}

	slices.SortFunc(found, func(a, b candidate) int {
		if a.prime != b.prime {
			if a.prime {
				return -1
			}
			return 1
		}

		if c := a.updatedAt.Compare(b.updatedAt); c != 0 {
			return c
		}
		return strings.Compare(a.slug, b.slug)
	})

	out := make([]dueItem, len(found))
	for i, c := range found {
		out[i] = c.dueItem
	}
	return out
}

func (s *Scraper) tierInterval(tier Tier) time.Duration {
	switch tier {
	case TierPrime:
		return s.intervals.Prime
	case TierHot:
		return s.intervals.Hot
	default:
		return s.intervals.Cold
	}
}

func (s *Scraper) reprice(ctx context.Context, due []dueItem) error {
	started := time.Now()

	priced := 0
	for _, item := range due {
		price, days, err := s.fetchPrice(ctx, item.slug, item.withStats)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}

			s.failedAt[item.slug] = time.Now()
			s.log.Warn("pricing failed", "slug", item.slug, "error", err)
			continue
		}

		if price == nil {
			continue
		}

		if err := s.store.SavePrice(ctx, item.slug, *price, days); err != nil {
			return fmt.Errorf("saving the price of %s: %w", item.slug, err)
		}

		s.prices[item.slug] = *price
		delete(s.failedAt, item.slug)

		priced++
	}

	counts := s.tierCounts()
	s.log.Info("repriced due items",
		"priced", priced, "due", len(due), "seconds", int(time.Since(started).Seconds()),
		"prime", counts[TierPrime], "hot", counts[TierHot], "cold", counts[TierCold],
		"untradable", len(s.untradable))
	return nil
}

// fetchPrice only returns prices for items warframe.market trades, and days only when we get statistics.
func (s *Scraper) fetchPrice(ctx context.Context, slug string, withStats bool) (*Price, []Day, error) {
	orders, err := s.source.Orders(ctx, slug)
	if errors.Is(err, wfm.ErrNotFound) {
		s.untradable[slug] = true
		return nil, nil, nil
	}

	if err != nil {
		return nil, nil, err
	}

	item := s.items[slug]
	previous := s.prices[slug]
	price := Price{
		Quote:        quoteOf(orders, item),
		Statistics:   previous.Statistics,
		StatisticsAt: previous.StatisticsAt,
		UpdatedAt:    time.Now(),
	}
	var days []Day
	if withStats {
		closed, err := s.source.Statistics(ctx, slug)
		if err != nil && !errors.Is(err, wfm.ErrNotFound) {
			return nil, nil, err
		}

		if err == nil {
			v := closedVariant(item, closed)
			days, err = dailySeries(closed.Last90Days, v)
			if err != nil {
				return nil, nil, err
			}

			price.Statistics = computeStatistics(closed.Last48Hours, days, v, price.UpdatedAt)
		}

		price.StatisticsAt = price.UpdatedAt
	}

	price.Tier = tierOf(s.prime[slug], price.active() || price.Volume48h > 0)
	return &price, days, nil
}

func (s *Scraper) tierCounts() map[Tier]int {
	counts := map[Tier]int{}
	for _, price := range s.prices {
		counts[price.Tier]++
	}
	return counts
}
