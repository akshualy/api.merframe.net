package postgres

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/akshualy/api.merframe.net/internal/blob"
	"github.com/akshualy/api.merframe.net/internal/market"
	"github.com/akshualy/api.merframe.net/internal/riven"
	"github.com/akshualy/api.merframe.net/internal/wfm"
)

func testStore(t *testing.T) *Store {
	t.Helper()

	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	s, err := Open(t.Context(), url)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(s.Close)

	if err := s.Migrate(t.Context()); err != nil {
		t.Fatal(err)
	}

	_, err = s.pool.Exec(t.Context(), "TRUNCATE prices, history, catalog, riven, blobs")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestPricesRoundTrip(t *testing.T) {
	s := testStore(t)
	ctx := t.Context()
	now := time.Now().UTC().Truncate(time.Microsecond)
	price := market.Price{
		Quote:      market.Quote{SellR0: new(29), SellR0Online: new(29), BuyR0: new(10), SellCount: 7, BuyCount: 2},
		Statistics: market.Statistics{Volume48h: 27, Median48h: new(30.0), DaysTraded90d: 40},
		Tier:       market.TierHot, UpdatedAt: now, StatisticsAt: now.Add(-time.Hour),
	}
	days := []market.Day{
		{Day: time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC), Volume: 23, Median: 30, Min: 20, Max: 40, WA: 29.043},
		{Day: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), Volume: 5, Median: 31, Min: 30, Max: 35, WA: 31.5},
	}
	if err := s.SavePrice(ctx, "secura_dual_cestra", price, days); err != nil {
		t.Fatal(err)
	}

	if err := s.SavePrice(ctx, "forma_blueprint", market.Price{Tier: market.TierCold, UpdatedAt: now, StatisticsAt: now}, nil); err != nil {
		t.Fatal(err)
	}

	got, err := s.Price(ctx, "secura_dual_cestra")
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(price, got); diff != "" {
		t.Errorf("price differs (-want +got):\n%s", diff)
	}

	history, err := s.History(ctx, "secura_dual_cestra")
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(days, history); diff != "" {
		t.Errorf("history differs (-want +got):\n%s", diff)
	}

	if none, err := s.History(ctx, "forma_blueprint"); err != nil || len(none) != 0 {
		t.Errorf("history of an item without one = %v, %v", none, err)
	}

	price.SellR0 = new(25)
	if err := s.SavePrice(ctx, "secura_dual_cestra", price, nil); err != nil {
		t.Fatal(err)
	}

	all, err := s.Prices(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(all) != 2 || *all["secura_dual_cestra"].SellR0 != 25 {
		t.Errorf("table after the update: %+v", all)
	}

	if history, _ := s.History(ctx, "secura_dual_cestra"); len(history) != 2 {
		t.Error("an update without days must keep the history")
	}

	dropped, err := s.ReplaceCatalog(ctx, market.Catalog{Catalog: wfm.Catalog{Items: []wfm.Item{{Slug: "forma_blueprint"}}, Document: []byte(`[{"slug": "forma_blueprint"}]`)}, RefreshedAt: now})
	if err != nil {
		t.Fatal(err)
	}

	if dropped != 1 {
		t.Errorf("dropped %d rows, want 1", dropped)
	}

	if _, err := s.Price(ctx, "secura_dual_cestra"); !errors.Is(err, ErrMissing) {
		t.Errorf("err after drop = %v, want ErrMissing", err)
	}

	if history, _ := s.History(ctx, "secura_dual_cestra"); len(history) != 0 {
		t.Error("dropping a price must drop its history")
	}
}

func TestDocumentsRoundTrip(t *testing.T) {
	s := testStore(t)
	ctx := t.Context()
	now := time.Now().UTC().Truncate(time.Microsecond)

	if empty, err := s.Catalog(ctx); err != nil || !empty.RefreshedAt.IsZero() {
		t.Errorf("empty catalog = %+v, %v", empty, err)
	}

	catalog := market.Catalog{Catalog: wfm.Catalog{Items: []wfm.Item{{Slug: "forma_blueprint"}}, Document: []byte(`[{"slug": "forma_blueprint", "tags": ["prime"]}]`)}, RefreshedAt: now}
	if _, err := s.ReplaceCatalog(ctx, catalog); err != nil {
		t.Fatal(err)
	}

	stored, err := s.Catalog(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if !stored.RefreshedAt.Equal(now) || string(stored.Document) != string(catalog.Document) || len(stored.Items) != 1 || stored.Items[0].Slug != "forma_blueprint" {
		t.Errorf("catalog = %+v", stored)
	}

	if empty, err := s.Riven(ctx); err != nil || empty.Weapons != nil {
		t.Errorf("riven on a fresh store = %+v, %v", empty, err)
	}

	data := riven.Document{
		WeaponsUpdatedAt: now,
		Weapons:          []riven.Weapon{{Slug: "torid", Name: "Torid", Disposition: 0.75}},
		Attributes:       []riven.Attribute{{Slug: "critical_chance", GameRef: "WeaponCritChanceMod"}},
		GoodRolls: map[string]riven.GoodRoll{"torid": {
			Weapon: "torid", WeaponClass: "primary",
			Alternatives: []riven.Alternative{{Mandatory: []riven.StatRef{{Abbr: "CC", Tags: []string{"WeaponCritChanceMod"}}}}},
		}},
	}
	if err := s.SaveRiven(ctx, data); err != nil {
		t.Fatal(err)
	}

	got, err := s.Riven(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(data, got); diff != "" {
		t.Errorf("riven differs (-want +got):\n%s", diff)
	}

	if !got.GoodRollsUpdatedAt.IsZero() {
		t.Error("a zero timestamp must come back zero")
	}
}

func TestBlobsNotify(t *testing.T) {
	s := testStore(t)
	ctx := t.Context()

	listener, err := s.ListenBlobs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close(ctx)
	b := blob.Build([]byte(`{"count":0}`))

	if err := s.SaveBlob(ctx, "prices", b, time.Now()); err != nil {
		t.Fatal(err)
	}

	waitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	name, err := listener.Next(waitCtx)
	if err != nil {
		t.Fatal(err)
	}

	if name != "prices" {
		t.Errorf("notified %q", name)
	}

	got, err := s.Blob(ctx, "prices")
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(b, got); diff != "" {
		t.Errorf("blob differs (-want +got):\n%s", diff)
	}

	tags, err := s.BlobTags(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if tags["prices"] != b.ETag {
		t.Errorf("tags = %v", tags)
	}

	if _, err := s.Blob(ctx, "riven"); !errors.Is(err, ErrMissing) {
		t.Errorf("unpublished blob err = %v", err)
	}
}
