-- +goose Up
CREATE TABLE auction_demand (
    slug text PRIMARY KEY,
    requested_at timestamptz NOT NULL,
    crawled_at timestamptz
);

-- +goose Down
DROP TABLE auction_demand;
