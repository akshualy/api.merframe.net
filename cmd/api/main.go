package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/akshualy/api.merframe.net/internal/config"
	"github.com/akshualy/api.merframe.net/internal/httpapi"
	"github.com/akshualy/api.merframe.net/internal/postgres"
)

const (
	readHeaderTimeout = 5 * time.Second
	idleTimeout       = 2 * time.Minute
	shutdownTimeout   = 10 * time.Second
)

func main() {
	if err := run(context.Background(), os.Getenv, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, getenv func(string) string, stdout io.Writer) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.LoadAPI(getenv)
	if err != nil {
		return err
	}

	log := slog.New(slog.NewJSONHandler(stdout, nil))

	db, err := postgres.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	blobs := httpapi.NewBlobs()
	listen := func(ctx context.Context) (httpapi.Listener, error) { return db.ListenBlobs(ctx) }
	watcher := httpapi.NewWatcher(db, listen, blobs, log.With("loop", "watcher"))
	if err := watcher.Resync(ctx); err != nil {
		return err
	}

	listener, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", cfg.ListenAddr, err)
	}

	server := &http.Server{
		Handler:           httpapi.NewServer(blobs, db, log),
		ReadHeaderTimeout: readHeaderTimeout,
		IdleTimeout:       idleTimeout,
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelWarn),
	}
	log.Info("api started", "addr", listener.Addr().String(), "ready", blobs.Ready())
	group, gctx := errgroup.WithContext(ctx)
	group.Go(func() error { return watcher.Run(gctx) })
	group.Go(func() error {
		if err := server.Serve(listener); !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	})

	group.Go(func() error {
		<-gctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(gctx), shutdownTimeout)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	})

	if err := group.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}

	log.Info("api stopped")
	return nil
}
