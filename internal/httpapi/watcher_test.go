package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/akshualy/api.merframe.net/internal/blob"
	"github.com/akshualy/api.merframe.net/internal/postgres"
	"github.com/akshualy/api.merframe.net/internal/publish"
)

type fakeSource struct {
	mu    sync.Mutex
	blobs map[string]*blob.Blob
	loads int
}

func (f *fakeSource) publish(name string, b *blob.Blob) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.blobs[name] = b
}

func (f *fakeSource) loaded() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.loads
}

func (f *fakeSource) BlobTags(context.Context) (map[string]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	tags := map[string]string{}
	for name, b := range f.blobs {
		tags[name] = b.ETag
	}
	return tags, nil
}

func (f *fakeSource) Blob(_ context.Context, name string) (*blob.Blob, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.loads++

	b, ok := f.blobs[name]
	if !ok {
		return nil, postgres.ErrMissing
	}
	return b, nil
}

type fakeListener struct {
	names  chan string
	fail   chan error
	closed int
}

func (l *fakeListener) Next(ctx context.Context) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case err := <-l.fail:
		return "", err
	case name := <-l.names:
		return name, nil
	}
}

func (l *fakeListener) Close(context.Context) error {
	l.closed++
	return nil
}

func startWatcher(t *testing.T, source *fakeSource, listener *fakeListener) (*Blobs, func()) {
	t.Helper()
	blobs := NewBlobs()
	opened := 0
	w := NewWatcher(source, func(context.Context) (Listener, error) {
		opened++
		return listener, nil
	}, blobs, slog.New(slog.DiscardHandler))

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- w.Run(ctx) }()
	return blobs, func() {
		cancel()

		if err := <-done; !errors.Is(err, context.Canceled) {
			t.Errorf("Run returned %v", err)
		}

		if listener.closed != opened {
			t.Errorf("opened %d listeners, closed %d", opened, listener.closed)
		}
	}
}

func TestWatcherNotification(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		source := &fakeSource{blobs: map[string]*blob.Blob{publish.RivenBlob: blob.Build([]byte(`{"weapons":[]}`))}}
		listener := &fakeListener{names: make(chan string), fail: make(chan error)}
		blobs, stop := startWatcher(t, source, listener)
		defer stop()
		synctest.Wait()

		if _, ok := blobs.Get(publish.RivenBlob); !ok {
			t.Fatal("the initial resync did not load the riven blob")
		}

		table := blob.Build([]byte(`{"count":1,"items":{"forma_blueprint":{"sell_r0":10}}}`))
		source.publish(publish.PricesBlob, table)
		listener.names <- publish.PricesBlob
		synctest.Wait()

		if got, ok := blobs.Get(publish.PricesBlob); !ok || got.ETag != table.ETag {
			t.Errorf("after the notification prices = %v, want the new table", got)
		}

		if _, ok := blobs.Item("forma_blueprint"); !ok {
			t.Error("the loaded table did not index its items")
		}
	})
}

func TestWatcherResyncTick(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		source := &fakeSource{blobs: map[string]*blob.Blob{}}
		listener := &fakeListener{names: make(chan string), fail: make(chan error)}
		blobs, stop := startWatcher(t, source, listener)
		defer stop()
		synctest.Wait()
		source.publish(publish.AuctionsBlob("torid"), blob.Build([]byte(`{"slug":"torid","auctions":[]}`)))
		time.Sleep(resyncInterval)
		synctest.Wait()

		if _, ok := blobs.Get(publish.AuctionsBlob("torid")); !ok {
			t.Error("a blob saved without a notification was not picked up by the resync")
		}

		loadsBefore := source.loaded()
		time.Sleep(resyncInterval)
		synctest.Wait()

		if got := source.loaded(); got != loadsBefore {
			t.Errorf("an unchanged blob was loaded %d more times by the next resync", got-loadsBefore)
		}
	})
}

func TestWatcherReconnect(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		source := &fakeSource{blobs: map[string]*blob.Blob{}}
		listener := &fakeListener{names: make(chan string), fail: make(chan error)}
		blobs, stop := startWatcher(t, source, listener)
		defer stop()
		synctest.Wait()
		source.publish(publish.RivenBlob, blob.Build([]byte(`{"weapons":[]}`)))
		listener.fail <- errors.New("connection reset")
		time.Sleep(listenRetry)
		synctest.Wait()

		if _, ok := blobs.Get(publish.RivenBlob); !ok {
			t.Error("the resync after the reconnect did not load the blob saved while the listener was down")
		}

		if listener.closed != 1 {
			t.Errorf("closed %d listeners after one failure, want 1", listener.closed)
		}
	})
}
