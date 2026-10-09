package httpapi

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/akshualy/api.merframe.net/internal/blob"
	"github.com/akshualy/api.merframe.net/internal/throttle"
)

type Listener interface {
	Next(ctx context.Context) (string, error)
	Close(ctx context.Context) error
}

type BlobSource interface {
	BlobTags(ctx context.Context) (map[string]string, error)
	Blob(ctx context.Context, name string) (*blob.Blob, error)
}

const (
	resyncInterval = time.Minute
	listenRetry    = 5 * time.Second
)

type Watcher struct {
	source BlobSource
	listen func(ctx context.Context) (Listener, error)
	blobs  *Blobs
	log    *slog.Logger
}

func NewWatcher(source BlobSource, listen func(ctx context.Context) (Listener, error), blobs *Blobs, log *slog.Logger) *Watcher {
	return &Watcher{source: source, listen: listen, blobs: blobs, log: log}
}

// Run subscribes before it resyncs, so nothing published in between is missed. Resyncs every minute just to be safe.
func (w *Watcher) Run(ctx context.Context) error {
	for {
		err := w.follow(ctx)

		if ctx.Err() != nil {
			return ctx.Err()
		}

		w.log.Warn("blob listener stopped", "error", err)
		if err := throttle.Sleep(ctx, listenRetry); err != nil {
			return err
		}
	}
}

func (w *Watcher) follow(ctx context.Context) error {
	listener, err := w.listen(ctx)
	if err != nil {
		return err
	}
	defer listener.Close(context.WithoutCancel(ctx))

	if err := w.Resync(ctx); err != nil {
		return err
	}

	names := make(chan string)
	errs := make(chan error, 1)
	go func() {
		for {
			name, err := listener.Next(ctx)
			if err != nil {
				errs <- err
				return
			}

			names <- name
		}
	}()

	ticker := time.NewTicker(resyncInterval)
	defer ticker.Stop()

	for {
		select {
		case err := <-errs:
			return err
		case name := <-names:
			if err := w.load(ctx, name); err != nil {
				w.log.Warn("blob reload failed", "name", name, "error", err)
			}
		case <-ticker.C:
			if err := w.Resync(ctx); err != nil {
				w.log.Warn("blob resync failed", "error", err)
			}
		}
	}
}

func (w *Watcher) Resync(ctx context.Context) error {
	tags, err := w.source.BlobTags(ctx)
	if err != nil {
		return fmt.Errorf("listing blobs: %w", err)
	}

	for name, etag := range tags {
		if current, ok := w.blobs.Get(name); ok && current.ETag == etag {
			continue
		}

		if err := w.load(ctx, name); err != nil {
			return err
		}
	}
	return nil
}

func (w *Watcher) load(ctx context.Context, name string) error {
	b, err := w.source.Blob(ctx, name)
	if err != nil {
		return fmt.Errorf("loading blob %s: %w", name, err)
	}

	if err := w.blobs.Set(name, b); err != nil {
		return err
	}

	w.log.Info("blob loaded", "name", name, "bytes", len(b.Body))
	return nil
}
