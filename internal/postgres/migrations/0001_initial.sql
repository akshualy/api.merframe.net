-- +goose Up
CREATE TABLE prices (
    slug text PRIMARY KEY,
    sell_r0 integer,
    sell_max integer,
    sell_r0_online integer,
    sell_max_online integer,
    buy_r0 integer,
    buy_max integer,
    sell_count integer NOT NULL,
    buy_count integer NOT NULL,
    volume_48h integer NOT NULL,
    median_48h double precision,
    volume_7d integer NOT NULL,
    volume_30d integer NOT NULL,
    volume_90d integer NOT NULL,
    median_7d double precision,
    median_30d double precision,
    median_90d double precision,
    min_90d double precision,
    max_90d double precision,
    days_traded_90d integer NOT NULL,
    volume_prev_48h integer NOT NULL,
    median_prev_48h double precision,
    volume_prev_7d integer NOT NULL,
    median_prev_7d double precision,
    volume_prev_30d integer NOT NULL,
    median_prev_30d double precision,
    tier text NOT NULL,
    updated_at timestamptz NOT NULL,
    statistics_at timestamptz NOT NULL
);

CREATE TABLE history (
    slug text NOT NULL REFERENCES prices (slug) ON DELETE CASCADE,
    day date NOT NULL,
    volume integer NOT NULL,
    median double precision NOT NULL,
    min double precision NOT NULL,
    max double precision NOT NULL,
    wa double precision NOT NULL,
    PRIMARY KEY (slug, day)
);

CREATE TABLE catalog (
    id integer PRIMARY KEY CHECK (id = 1),
    refreshed_at timestamptz NOT NULL,
    items json NOT NULL
);

CREATE TABLE riven (
    id integer PRIMARY KEY CHECK (id = 1),
    weapons_updated_at timestamptz,
    good_rolls_updated_at timestamptz,
    weapons jsonb NOT NULL,
    attributes jsonb NOT NULL,
    good_rolls jsonb NOT NULL,
    unmapped text[]
);

CREATE TABLE blobs (
    name text PRIMARY KEY,
    etag text NOT NULL,
    body bytea NOT NULL,
    gzip bytea NOT NULL,
    brotli bytea NOT NULL,
    published_at timestamptz NOT NULL
);

-- +goose Down
DROP TABLE blobs;
DROP TABLE riven;
DROP TABLE catalog;
DROP TABLE history;
DROP TABLE prices;
