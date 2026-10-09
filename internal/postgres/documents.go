package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/akshualy/api.merframe.net/internal/blob"
	"github.com/akshualy/api.merframe.net/internal/market"
	"github.com/akshualy/api.merframe.net/internal/postgres/sqlc"
	"github.com/akshualy/api.merframe.net/internal/riven"
)

// ReplaceCatalog drops the prices of items the new list doesn't have anymore.
func (s *Store) ReplaceCatalog(ctx context.Context, c market.Catalog) (int64, error) {
	keep := make([]string, len(c.Items))
	for i, item := range c.Items {
		keep[i] = item.Slug
	}

	var dropped int64
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)

		if err := q.SetCatalog(ctx, sqlc.SetCatalogParams{RefreshedAt: c.RefreshedAt, Items: c.Document}); err != nil {
			return err
		}

		var err error
		dropped, err = q.DeletePricesExcept(ctx, keep)
		return err
	})
	return dropped, err
}

func (s *Store) Catalog(ctx context.Context) (market.Catalog, error) {
	row, err := s.q.GetCatalog(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return market.Catalog{}, nil
	}

	if err != nil {
		return market.Catalog{}, err
	}

	c := market.Catalog{RefreshedAt: row.RefreshedAt}
	c.Document = row.Items
	if err := json.Unmarshal(row.Items, &c.Items); err != nil {
		return market.Catalog{}, fmt.Errorf("decoding the stored item list: %w", err)
	}
	return c, nil
}

func (s *Store) SaveRiven(ctx context.Context, d riven.Document) error {
	weapons, err := json.Marshal(d.Weapons)
	if err != nil {
		return fmt.Errorf("encoding the weapons: %w", err)
	}

	attributes, err := json.Marshal(d.Attributes)
	if err != nil {
		return fmt.Errorf("encoding the attributes: %w", err)
	}

	rolls, err := json.Marshal(d.GoodRolls)
	if err != nil {
		return fmt.Errorf("encoding the good rolls: %w", err)
	}
	return s.q.SetRiven(ctx, sqlc.SetRivenParams{
		WeaponsUpdatedAt:   nullableTime(d.WeaponsUpdatedAt),
		GoodRollsUpdatedAt: nullableTime(d.GoodRollsUpdatedAt),
		Weapons:            weapons,
		Attributes:         attributes,
		GoodRolls:          rolls,
		Unmapped:           d.Unmapped,
	})
}

func (s *Store) Riven(ctx context.Context) (riven.Document, error) {
	row, err := s.q.GetRiven(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return riven.Document{}, nil
	}

	if err != nil {
		return riven.Document{}, err
	}

	d := riven.Document{Unmapped: row.Unmapped}
	if err := json.Unmarshal(row.Weapons, &d.Weapons); err != nil {
		return riven.Document{}, fmt.Errorf("decoding the stored weapons: %w", err)
	}

	if err := json.Unmarshal(row.Attributes, &d.Attributes); err != nil {
		return riven.Document{}, fmt.Errorf("decoding the stored attributes: %w", err)
	}

	if err := json.Unmarshal(row.GoodRolls, &d.GoodRolls); err != nil {
		return riven.Document{}, fmt.Errorf("decoding the stored good rolls: %w", err)
	}

	if row.WeaponsUpdatedAt != nil {
		d.WeaponsUpdatedAt = *row.WeaponsUpdatedAt
	}

	if row.GoodRollsUpdatedAt != nil {
		d.GoodRollsUpdatedAt = *row.GoodRollsUpdatedAt
	}
	return d, nil
}

const blobsChannel = "blobs"

func (s *Store) SaveBlob(ctx context.Context, name string, b *blob.Blob, publishedAt time.Time) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		err := q.UpsertBlob(ctx, sqlc.UpsertBlobParams{
			Name: name, Etag: b.ETag, Body: b.Body, Gzip: b.Gzip, Brotli: b.Brotli, PublishedAt: publishedAt,
		})
		if err != nil {
			return fmt.Errorf("saving blob %s: %w", name, err)
		}
		return q.NotifyBlob(ctx, sqlc.NotifyBlobParams{Channel: blobsChannel, Name: name})
	})
}

func (s *Store) Blob(ctx context.Context, name string) (*blob.Blob, error) {
	row, err := s.q.GetBlob(ctx, name)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrMissing
	}

	if err != nil {
		return nil, err
	}
	return &blob.Blob{ETag: row.Etag, Body: row.Body, Gzip: row.Gzip, Brotli: row.Brotli}, nil
}

func (s *Store) BlobTags(ctx context.Context) (map[string]string, error) {
	rows, err := s.q.ListBlobTags(ctx)
	if err != nil {
		return nil, err
	}

	tags := make(map[string]string, len(rows))
	for _, row := range rows {
		tags[row.Name] = row.Etag
	}
	return tags, nil
}

func nullableTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}
