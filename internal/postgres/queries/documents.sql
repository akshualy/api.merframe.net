-- name: SetCatalog :exec
INSERT INTO catalog (id, refreshed_at, items)
VALUES (1, $1, $2)
ON CONFLICT (id) DO UPDATE SET refreshed_at = EXCLUDED.refreshed_at, items = EXCLUDED.items;

-- name: GetCatalog :one
SELECT refreshed_at, items FROM catalog WHERE id = 1;

-- name: SetRiven :exec
INSERT INTO riven (id, weapons_updated_at, good_rolls_updated_at, weapons, attributes, good_rolls, unmapped)
VALUES (1, $1, $2, $3, $4, $5, $6)
ON CONFLICT (id) DO UPDATE SET
    weapons_updated_at = EXCLUDED.weapons_updated_at,
    good_rolls_updated_at = EXCLUDED.good_rolls_updated_at,
    weapons = EXCLUDED.weapons,
    attributes = EXCLUDED.attributes,
    good_rolls = EXCLUDED.good_rolls,
    unmapped = EXCLUDED.unmapped;

-- name: GetRiven :one
SELECT weapons_updated_at, good_rolls_updated_at, weapons, attributes, good_rolls, unmapped
FROM riven WHERE id = 1;

-- name: UpsertBlob :exec
INSERT INTO blobs (name, etag, body, gzip, brotli, published_at)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (name) DO UPDATE SET
    etag = EXCLUDED.etag,
    body = EXCLUDED.body,
    gzip = EXCLUDED.gzip,
    brotli = EXCLUDED.brotli,
    published_at = EXCLUDED.published_at;

-- name: NotifyBlob :exec
SELECT pg_notify(sqlc.arg(channel)::text, sqlc.arg(name)::text);

-- name: GetBlob :one
SELECT * FROM blobs WHERE name = $1;

-- name: ListBlobTags :many
SELECT name, etag FROM blobs ORDER BY name;
