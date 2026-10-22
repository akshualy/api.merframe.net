package riven

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"testing/synctest"
	"time"

	"github.com/akshualy/api.merframe.net/internal/wfm"
)

type fakeStore struct {
	data Document
}

func newFakeStore() *fakeStore {
	return &fakeStore{}
}

func (f *fakeStore) Riven(context.Context) (Document, error) { return f.data, nil }

func (f *fakeStore) SaveRiven(_ context.Context, d Document) error {
	f.data = d
	return nil
}

type fakePublisher struct {
	riven    int
	auctions map[string]WeaponAuctions
}

func (f *fakePublisher) Riven(context.Context, Document) error {
	f.riven++
	return nil
}

func (f *fakePublisher) Auctions(_ context.Context, w WeaponAuctions) error {
	if f.auctions == nil {
		f.auctions = map[string]WeaponAuctions{}
	}

	f.auctions[w.Slug] = w
	return nil
}

type fakeSource struct {
	weapons    []wfm.RivenWeapon
	attributes []wfm.RivenAttribute
}

func (f *fakeSource) RivenWeapons(context.Context) ([]wfm.RivenWeapon, error) {
	return f.weapons, nil
}

func (f *fakeSource) RivenAttributes(context.Context) ([]wfm.RivenAttribute, error) {
	return f.attributes, nil
}

type fakeSheet struct {
	calls int
}

func (f *fakeSheet) Tab(_ context.Context, name string) (string, error) {
	f.calls++

	if name != "Primary" {
		return "\"WEAPON\",\"POSITIVE STATS:\",\"NEGATIVE STATS:\"\n", nil
	}
	return "\"WEAPON\",\"POSITIVE STATS:\",\"NEGATIVE STATS:\"\n\"torid\",\"CD CC\",\"IMP\"\n", nil
}

// start runs a loop in the background and returns the call that cancels it and checks how it ended.
func start(t *testing.T, run func(context.Context) error) (stop func()) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- run(ctx) }()
	return func() {
		cancel()
		if err := <-done; !errors.Is(err, context.Canceled) {
			t.Fatalf("Run returned %v", err)
		}
	}
}

func runUntilCancelled(t *testing.T, run func(context.Context) error, d time.Duration) {
	t.Helper()
	stop := start(t, run)
	time.Sleep(d)
	stop()
}

func rifle(slug string) wfm.RivenWeapon {
	w := wfm.RivenWeapon{Slug: slug, Group: "primary", RivenType: "rifle", Disposition: 1}
	w.I18n.En.Name = slug
	return w
}

func TestRefresherDaily(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		source := &fakeSource{weapons: []wfm.RivenWeapon{rifle("torid")}, attributes: []wfm.RivenAttribute{{Slug: "critical_chance"}}}
		sheet := &fakeSheet{}
		store := newFakeStore()
		publisher := &fakePublisher{}
		r := NewRefresher(source, sheet, store, publisher, 24*time.Hour, slog.New(slog.DiscardHandler))
		runUntilCancelled(t, r.Run, 25*time.Hour)

		if sheet.calls != 8 {
			t.Errorf("sheet fetched %d tabs, want 4 tabs twice", sheet.calls)
		}

		if publisher.riven != 2 {
			t.Errorf("riven published %d times, want 2", publisher.riven)
		}

		if len(store.data.Weapons) != 1 || len(store.data.GoodRolls) != 1 || store.data.GoodRolls["torid"].WeaponClass != "primary" {
			t.Errorf("stored data = %+v", store.data)
		}
	})
}
