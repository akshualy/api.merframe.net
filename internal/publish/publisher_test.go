package publish

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/akshualy/api.merframe.net/internal/blob"
	"github.com/akshualy/api.merframe.net/internal/market"
)

type fakeStore struct {
	mu     sync.Mutex
	prices map[string]market.Price
	saved  []string
}

func (f *fakeStore) Prices(context.Context) (map[string]market.Price, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.prices, nil
}

func (f *fakeStore) SaveBlob(_ context.Context, name string, _ *blob.Blob, _ time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.saved = append(f.saved, name)
	return nil
}

func (f *fakeStore) setPrices(prices map[string]market.Price) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.prices = prices
}

func (f *fakeStore) savedCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.saved)
}

func TestFirstPublish(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		store := &fakeStore{}
		p := New(store, 15*time.Minute, slog.New(slog.DiscardHandler))
		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan error, 1)
		go func() { done <- p.Run(ctx) }()
		synctest.Wait()

		if store.savedCount() != 0 {
			t.Fatal("published an empty table")
		}

		store.setPrices(map[string]market.Price{"forma_blueprint": {UpdatedAt: time.Now()}})
		time.Sleep(firstPublishRetry)
		synctest.Wait()

		if store.savedCount() != 1 {
			t.Errorf("saved %d blobs a minute after prices appeared, want 1", store.savedCount())
		}

		time.Sleep(14 * time.Minute)
		synctest.Wait()

		if store.savedCount() != 1 {
			t.Errorf("saved %d blobs before the interval passed, want still 1", store.savedCount())
		}

		time.Sleep(time.Minute)
		synctest.Wait()

		if store.savedCount() != 1 {
			t.Errorf("saved %d blobs for an unchanged table, want still 1", store.savedCount())
		}

		cancel()

		if err := <-done; !errors.Is(err, context.Canceled) {
			t.Errorf("Run returned %v", err)
		}
	})
}
