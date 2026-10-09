package postgres

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/database"

	"github.com/akshualy/api.merframe.net/internal/postgres/sqlc"
)

//go:embed migrations/*.sql
var migrations embed.FS

var ErrMissing = errors.New("not in the store")

type Store struct {
	pool *pgxpool.Pool
	q    *sqlc.Queries
}

func Open(ctx context.Context, url string) (*Store, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("opening the pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pinging postgres: %w", err)
	}
	return &Store{pool: pool, q: sqlc.New(pool)}, nil
}

func (s *Store) Close() {
	s.pool.Close()
}

func (s *Store) Migrate(ctx context.Context) error {
	dir, err := fs.Sub(migrations, "migrations")
	if err != nil {
		return err
	}

	db := stdlib.OpenDBFromPool(s.pool)
	defer db.Close()

	provider, err := goose.NewProvider(database.DialectPostgres, db, dir)
	if err != nil {
		return fmt.Errorf("preparing migrations: %w", err)
	}

	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("migrating: %w", err)
	}
	return nil
}

type Listener struct {
	conn *pgxpool.Conn
}

func (s *Store) ListenBlobs(ctx context.Context) (*Listener, error) {
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquiring a listener connection: %w", err)
	}

	if _, err := conn.Exec(ctx, "LISTEN "+blobsChannel); err != nil {
		conn.Release()
		return nil, fmt.Errorf("listening on %s: %w", blobsChannel, err)
	}
	return &Listener{conn: conn}, nil
}

func (l *Listener) Next(ctx context.Context) (string, error) {
	notification, err := l.conn.Conn().WaitForNotification(ctx)
	if err != nil {
		return "", fmt.Errorf("waiting on %s: %w", blobsChannel, err)
	}
	return notification.Payload, nil
}

// Close closes the connection instead of releasing it. The LISTEN is not allowed to reach the pool.
func (l *Listener) Close(ctx context.Context) error {
	return l.conn.Hijack().Close(ctx)
}
