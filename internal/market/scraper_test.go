package market

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"testing"
	"testing/synctest"
	"time"

	"github.com/akshualy/api.merframe.net/internal/wfm"
)

type fakeSource struct {
	catalog  wfm.Catalog
	orders   map[string][]wfm.Order
	closed   map[string]wfm.ClosedStatistics
	failures map[string]error
	fetched  []string
}

func (f *fakeSource) Items(context.Context) (wfm.Catalog, error) {
	f.fetched = append(f.fetched, "items")
	return f.catalog, nil
}

func (f *fakeSource) Orders(_ context.Context, slug string) ([]wfm.Order, error) {
	f.fetched = append(f.fetched, "orders:"+slug)
	if err := f.failures[slug]; err != nil {
		return nil, err
	}

	orders, ok := f.orders[slug]
	if !ok {
		return nil, wfm.ErrNotFound
	}
	return orders, nil
}

func (f *fakeSource) Statistics(_ context.Context, slug string) (wfm.ClosedStatistics, error) {
	f.fetched = append(f.fetched, "statistics:"+slug)
	closed, ok := f.closed[slug]
	if !ok {
		return wfm.ClosedStatistics{}, wfm.ErrNotFound
	}
	return closed, nil
}

func (f *fakeSource) count(call string) int {
	n := 0

	for _, c := range f.fetched {
		if c == call {
			n++
		}
	}
	return n
}

type fakeStore struct {
	catalog Catalog
	prices  map[string]Price
	history map[string][]Day
	dropped []string
}

func newFakeStore() *fakeStore {
	return &fakeStore{prices: map[string]Price{}, history: map[string][]Day{}}
}

func (f *fakeStore) Catalog(context.Context) (Catalog, error) { return f.catalog, nil }

func (f *fakeStore) ReplaceCatalog(_ context.Context, c Catalog) (int64, error) {
	f.catalog = c

	kept := map[string]bool{}
	for _, item := range c.Items {
		kept[item.Slug] = true
	}

	var dropped int64
	for slug := range f.prices {
		if !kept[slug] {
			delete(f.prices, slug)
			f.dropped = append(f.dropped, slug)
			dropped++
		}
	}
	return dropped, nil
}

func (f *fakeStore) Prices(context.Context) (map[string]Price, error) {
	out := make(map[string]Price, len(f.prices))
	for slug, price := range f.prices {
		out[slug] = price
	}
	return out, nil
}

func (f *fakeStore) SavePrice(_ context.Context, slug string, p Price, days []Day) error {
	f.prices[slug] = p

	if days != nil {
		f.history[slug] = days
	}
	return nil
}

type fakePublisher struct {
	catalogs int
}

func (f *fakePublisher) Catalog(context.Context, Catalog) error {
	f.catalogs++
	return nil
}

func catalogOf(items ...wfm.Item) wfm.Catalog {
	return wfm.Catalog{Items: items, Document: []byte("[]")}
}

func sellOrder(platinum int, status string) wfm.Order {
	var o wfm.Order
	o.Type = "sell"
	o.Platinum = platinum
	o.Visible = true
	o.User.Status = status
	return o
}

var intervals = Intervals{
	Prime:      30 * time.Minute,
	Hot:        4 * time.Hour,
	Cold:       24 * time.Hour,
	Statistics: 24 * time.Hour,
	Catalog:    24 * time.Hour,
}

func runFor(t *testing.T, s *Scraper, d time.Duration) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()
	time.Sleep(d)
	cancel()

	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("Run returned %v", err)
	}
}

func TestScraperFirstPass(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		source := &fakeSource{
			catalog: catalogOf(
				wfm.Item{Slug: "mesa_prime_set", Tags: []string{"prime"}},
				wfm.Item{Slug: "serration", MaxRank: new(10)},
				wfm.Item{Slug: "vaulted_thing"},
			),
			orders: map[string][]wfm.Order{
				"mesa_prime_set": {sellOrder(300, "ingame")},
				"serration":      {},
			},
		}
		store := newFakeStore()
		publisher := &fakePublisher{}
		s := NewScraper(source, store, publisher, intervals, slog.New(slog.DiscardHandler))
		runFor(t, s, time.Minute)

		if len(store.prices) != 2 {
			t.Fatalf("priced %d items, want 2: %+v", len(store.prices), store.prices)
		}

		if got := store.prices["mesa_prime_set"]; got.Tier != TierPrime || *got.SellR0 != 300 {
			t.Errorf("mesa_prime_set = %+v, want prime at 300", got)
		}

		if got := store.prices["serration"]; got.Tier != TierCold {
			t.Errorf("serration tier = %s, want cold", got.Tier)
		}

		if _, priced := store.prices["vaulted_thing"]; priced {
			t.Error("vaulted_thing was priced, want untradable")
		}

		if store.catalog.RefreshedAt.IsZero() || len(store.catalog.Items) != 3 || publisher.catalogs != 1 {
			t.Errorf("catalog saved %+v, published %d times", store.catalog, publisher.catalogs)
		}

		want := []string{"items", "orders:mesa_prime_set", "statistics:mesa_prime_set", "orders:serration", "statistics:serration", "orders:vaulted_thing"}
		if !slices.Equal(source.fetched, want) {
			t.Errorf("fetched %v, want %v", source.fetched, want)
		}
	})
}

func TestScraperTierIntervals(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		source := &fakeSource{
			catalog: catalogOf(
				wfm.Item{Slug: "mesa_prime_set", Tags: []string{"prime"}},
				wfm.Item{Slug: "serration"},
			),
			orders: map[string][]wfm.Order{
				"mesa_prime_set": {sellOrder(300, "ingame")},
				"serration":      {sellOrder(5, "online")},
			},
		}
		store := newFakeStore()
		s := NewScraper(source, store, &fakePublisher{}, intervals, slog.New(slog.DiscardHandler))
		runFor(t, s, 31*time.Minute)

		if prime, hot := source.count("orders:mesa_prime_set"), source.count("orders:serration"); prime != 2 || hot != 1 {
			t.Errorf("after 31 minutes prime fetched %d times, hot %d, want 2 and 1", prime, hot)
		}
	})
}

func TestScraperDueOrder(t *testing.T) {
	s := NewScraper(nil, nil, nil, intervals, slog.New(slog.DiscardHandler))
	now := time.Now()
	s.setCatalog(Catalog{Items: []wfm.Item{
		{Slug: "fresh"}, {Slug: "old_hot"}, {Slug: "older_hot"}, {Slug: "stats_only"},
		{Slug: "prime_new", Tags: []string{"prime"}}, {Slug: "unpriced"}, {Slug: "dead"}, {Slug: "failed"},
	}})
	s.prices = map[string]Price{
		"fresh":      {Tier: TierHot, UpdatedAt: now, StatisticsAt: now},
		"old_hot":    {Tier: TierHot, UpdatedAt: now.Add(-5 * time.Hour), StatisticsAt: now},
		"older_hot":  {Tier: TierHot, UpdatedAt: now.Add(-6 * time.Hour), StatisticsAt: now},
		"stats_only": {Tier: TierHot, UpdatedAt: now, StatisticsAt: now.Add(-25 * time.Hour)},
		"prime_new":  {Tier: TierPrime, UpdatedAt: now.Add(-31 * time.Minute), StatisticsAt: now},
		"failed":     {Tier: TierHot, UpdatedAt: now.Add(-9 * time.Hour), StatisticsAt: now},
	}
	s.untradable["dead"] = true
	s.failedAt["failed"] = now.Add(-time.Minute)
	got := s.due(now)
	want := []dueItem{
		{slug: "prime_new"}, {slug: "unpriced", withStats: true}, {slug: "older_hot"}, {slug: "old_hot"}, {slug: "stats_only", withStats: true},
	}
	if !slices.Equal(got, want) {
		t.Errorf("due = %+v, want %+v", got, want)
	}

	if later := s.due(now.Add(failedRetryPause)); !slices.Contains(later, dueItem{slug: "failed"}) {
		t.Errorf("after the pause due = %+v, want failed back in it", later)
	}
}

func TestScraperCatalogRefresh(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		source := &fakeSource{
			catalog: catalogOf(wfm.Item{Slug: "serration"}),
			orders:  map[string][]wfm.Order{"serration": {}},
		}
		store := newFakeStore()
		store.prices["old_name"] = Price{Tier: TierCold, UpdatedAt: time.Now(), StatisticsAt: time.Now()}
		s := NewScraper(source, store, &fakePublisher{}, intervals, slog.New(slog.DiscardHandler))
		runFor(t, s, time.Minute)

		if _, kept := s.prices["old_name"]; kept || len(s.items) != 1 {
			t.Errorf("prices %v after the refresh, want old_name gone", s.prices)
		}
	})
}

func TestScraperFailedItem(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		source := &fakeSource{
			catalog:  catalogOf(wfm.Item{Slug: "a"}, wfm.Item{Slug: "b"}),
			orders:   map[string][]wfm.Order{"a": {}, "b": {}},
			failures: map[string]error{"a": &wfm.StatusError{Status: 500}},
		}
		store := newFakeStore()
		s := NewScraper(source, store, &fakePublisher{}, intervals, slog.New(slog.DiscardHandler))
		runFor(t, s, failedRetryPause+time.Minute)

		if _, priced := store.prices["b"]; !priced {
			t.Error("b not priced, want priced despite a failing")
		}

		if _, priced := store.prices["a"]; priced {
			t.Error("a priced, want left out after its failure")
		}

		if got := source.count("orders:a"); got != 2 {
			t.Errorf("a fetched %d times in %v, want 2 (once, then once after the pause)", got, failedRetryPause+time.Minute)
		}
	})
}

func TestScraperStatisticsCarry(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		source := &fakeSource{
			catalog: catalogOf(wfm.Item{Slug: "a"}),
			orders:  map[string][]wfm.Order{"a": {}},
		}
		store := newFakeStore()
		carried := Statistics{Volume48h: 9, Median48h: new(12.0)}
		store.prices["a"] = Price{Statistics: carried, Tier: TierHot, UpdatedAt: time.Now().Add(-5 * time.Hour), StatisticsAt: time.Now().Add(-25 * time.Hour)}
		store.history["a"] = []Day{{Volume: 1}}
		s := NewScraper(source, store, &fakePublisher{}, intervals, slog.New(slog.DiscardHandler))
		runFor(t, s, time.Minute)

		got := store.prices["a"]
		if got.Statistics != carried || len(store.history["a"]) != 1 {
			t.Errorf("after a statistics 404 price = %+v, history %v, want the carried figures and the old history", got, store.history["a"])
		}

		if !got.StatisticsAt.Equal(got.UpdatedAt) {
			t.Errorf("StatisticsAt = %v, want the attempt time so the item is not due again at once", got.StatisticsAt)
		}

		if got.Tier != TierHot {
			t.Errorf("tier = %s, want hot from the carried volume", got.Tier)
		}

		if got := source.count("statistics:a"); got != 1 {
			t.Errorf("statistics fetched %d times, want 1", got)
		}
	})
}

func TestScraperEmptySeries(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		source := &fakeSource{
			catalog: catalogOf(wfm.Item{Slug: "a"}),
			orders:  map[string][]wfm.Order{"a": {}},
			closed:  map[string]wfm.ClosedStatistics{"a": {}},
		}
		store := newFakeStore()
		store.history["a"] = []Day{{Volume: 1}}
		s := NewScraper(source, store, &fakePublisher{}, intervals, slog.New(slog.DiscardHandler))
		runFor(t, s, time.Minute)

		if days, ok := store.history["a"]; !ok || len(days) != 0 {
			t.Errorf("history = %v, want replaced by an empty series", days)
		}
	})
}
