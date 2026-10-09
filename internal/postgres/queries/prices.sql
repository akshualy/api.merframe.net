-- name: UpsertPrice :exec
INSERT INTO prices (
    slug, sell_r0, sell_max, sell_r0_online, sell_max_online, buy_r0, buy_max,
    sell_count, buy_count,
    volume_48h, median_48h, volume_7d, volume_30d, volume_90d,
    median_7d, median_30d, median_90d, min_90d, max_90d, days_traded_90d,
    volume_prev_48h, median_prev_48h, volume_prev_7d, median_prev_7d,
    volume_prev_30d, median_prev_30d,
    tier, updated_at, statistics_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7,
    $8, $9,
    $10, $11, $12, $13, $14,
    $15, $16, $17, $18, $19, $20,
    $21, $22, $23, $24,
    $25, $26,
    $27, $28, $29
)
ON CONFLICT (slug) DO UPDATE SET
    sell_r0 = EXCLUDED.sell_r0,
    sell_max = EXCLUDED.sell_max,
    sell_r0_online = EXCLUDED.sell_r0_online,
    sell_max_online = EXCLUDED.sell_max_online,
    buy_r0 = EXCLUDED.buy_r0,
    buy_max = EXCLUDED.buy_max,
    sell_count = EXCLUDED.sell_count,
    buy_count = EXCLUDED.buy_count,
    volume_48h = EXCLUDED.volume_48h,
    median_48h = EXCLUDED.median_48h,
    volume_7d = EXCLUDED.volume_7d,
    volume_30d = EXCLUDED.volume_30d,
    volume_90d = EXCLUDED.volume_90d,
    median_7d = EXCLUDED.median_7d,
    median_30d = EXCLUDED.median_30d,
    median_90d = EXCLUDED.median_90d,
    min_90d = EXCLUDED.min_90d,
    max_90d = EXCLUDED.max_90d,
    days_traded_90d = EXCLUDED.days_traded_90d,
    volume_prev_48h = EXCLUDED.volume_prev_48h,
    median_prev_48h = EXCLUDED.median_prev_48h,
    volume_prev_7d = EXCLUDED.volume_prev_7d,
    median_prev_7d = EXCLUDED.median_prev_7d,
    volume_prev_30d = EXCLUDED.volume_prev_30d,
    median_prev_30d = EXCLUDED.median_prev_30d,
    tier = EXCLUDED.tier,
    updated_at = EXCLUDED.updated_at,
    statistics_at = EXCLUDED.statistics_at;

-- name: GetPrice :one
SELECT * FROM prices WHERE slug = $1;

-- name: ListPrices :many
SELECT * FROM prices ORDER BY slug;

-- name: DeletePricesExcept :execrows
DELETE FROM prices WHERE slug <> ALL($1::text[]);

-- name: DeleteHistory :exec
DELETE FROM history WHERE slug = $1;

-- name: InsertHistory :copyfrom
INSERT INTO history (slug, day, volume, median, min, max, wa)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: ListHistory :many
SELECT * FROM history WHERE slug = $1 ORDER BY day;
