# api.merframe.net

The data service behind [Merframe](https://github.com/akshualy/Merframe).

A price table for every tradable warframe.market item, the riven weapon and attribute lists with the community's good rolls, and the open riven auctions per weapon.

One collector updates the data at a reasonable rate, one api serves it with ETags and cache headers.

## Build and test

Go 1.27 and a Postgres to test against.

```
make lint
make test            # the postgres tests need TEST_DATABASE_URL (skipped if unset)
make generate        # after editing internal/postgres/queries or migrations
make build           # bin/collector and bin/api
```

## Run

Both binaries read `MERFRAME_*` variables. `MERFRAME_DATABASE_URL` has no default.

The collector runs the migrations on start. The api only reads the database. So please start the collector first.

```
MERFRAME_DATABASE_URL=postgres://merframe:merframe@localhost/merframe bin/collector
MERFRAME_DATABASE_URL=postgres://merframe:merframe@localhost/merframe MERFRAME_LISTEN_ADDR=127.0.0.1:8080 bin/api
```

The api listens on `MERFRAME_LISTEN_ADDR` (127.0.0.1:8080).

Collector settings are all optional:
- `MERFRAME_MARKET_RATE` (requests per second to warframe.market, 1)
- `MERFRAME_PRIME_INTERVAL` (30m)
- `MERFRAME_HOT_INTERVAL` (4h)
- `MERFRAME_COLD_INTERVAL` (24h)
- `MERFRAME_STATISTICS_INTERVAL` (24h)
- `MERFRAME_CATALOG_INTERVAL` (24h)
- `MERFRAME_PUBLISH_INTERVAL` (15m)
- `MERFRAME_RIVEN_INTERVAL` (24h)
- `MERFRAME_AUCTION_PASS_INTERVAL` (24h)
- `MERFRAME_AUCTION_SPACING` (6s)
- `MERFRAME_CONTACT` (the address in the User-Agent)

## Deploy

`deploy/` has a multi-stage Dockerfile and a compose file with Postgres 18, the collector, the api and `cloudflared`.

Copy `deploy/.env.example` to `deploy/.env`, set the database password and the tunnel token, then `make up`.

Without the `edge` profile the stack runs without the tunnel. Use this to try out things locally.

Every push to `main` that passes CI deploys. The `deploy` workflow logs into the VPS over SSH, resets the checkout in `~/merframe` to the CI's commit and runs `make up`.
It needs the secrets `VPS_HOST`, `VPS_USER`, `VPS_HOST_KEY` (the `known_hosts` line) and `VPS_SSH_KEY`.

The api is not published on any port. `cloudflared` reaches it as `http://api:8080` on the compose network and the tunnel's public hostname `api.merframe.net` points there.
The collector runs with the host's network stack so its outbound requests can use IPv6. It reaches Postgres on `127.0.0.1:5432`, which is the only port the stack opens on the host.

On the Cloudflare side, set a cache rule for `api.merframe.net/v1/*` (eligible for cache, edge TTL respecting origin, strong ETags respected), turn on Smart Tiered Cache, and add a rate limiting rule of 10 requests per 10 seconds per IP on `/v1/*`.

## Endpoints

| Path | What | Edge TTL |
|---|---|---|
| `GET /v1/prices` | All prices | 1 min, stale for 5 min while refreshing, a day if the origin is down |
| `GET /v1/prices/{slug}` | Price of a single item | 1 min |
| `GET /v1/prices/{slug}/history` | One item's daily closed prices of the last 90 days | 1 min |
| `GET /v1/items` | The warframe.market item list as fetched | 1 h |
| `GET /v1/riven` | Riven weapons, attributes and the community good rolls | 1 h |
| `GET /v1/riven/auctions/{slug}` | Open riven auctions of one weapon | 5 min |
| `GET /healthz`, `GET /readyz` | live & readiness checks | not cached |

Bulk documents carry a strong `ETag` for the uncompressed body. Send it back as `If-None-Match` to get `304 Not Modified`.

Bodies are precompressed, so ask for `br` or `gzip` in `Accept-Encoding`.

Before the first publish the bulk routes respond with a `503` and a `Retry-After: 60` header. Missing items, history or weapon respond with a `404` and a `detail` string.

### Price fields

| Field | Type | Description |
|---|---|---|
| `sell_r0` | int or null | Lowest visible unranked sell price from a seller who is in-game |
| `sell_max` | int or null | Same as above, just for fully ranked items, null when the item has no ranks |
| `sell_r0_online`, `sell_max_online` | int or null | Same, counting sellers who are in-game or online on the site |
| `buy_r0`, `buy_max` | int or null | Highest visible unranked buy price, and at max rank, from a buyer who is in-game or online |
| `sell_count`, `buy_count` | int | Visible orders from in-game or online users, all ranks |
| `volume_48h` | int | Closed trades in the last 48 hours, all ranks |
| `median_48h` | float or null | Median unranked closed price of the newest closed hour |
| `volume_7d`, `volume_30d`, `volume_90d` | int | Closed unranked trades in that window |
| `median_7d`, `median_30d`, `median_90d` | float or null | Median of the daily medians over the days with trades in that window |
| `min_90d`, `max_90d` | float or null | Lowest and highest unranked closed price in the last 90 days |
| `days_traded_90d` | int | Days with at least one closed trade in the last 90 days |
| `volume_prev_48h`, `volume_prev_7d`, `volume_prev_30d` | int | Closed unranked trades in the window of the same length before the current one |
| `median_prev_48h`, `median_prev_7d`, `median_prev_30d` | float or null | Same as above, but for the medians |
| `tier` | `prime`, `hot` or `cold` | Which refresh interval the item follows |
| `updated_at` | int | Unix seconds when the item was last priced; the table's own `updated_at` is the newest of these |
| `statistics_at` | int | Unix seconds when the closed statistics and the history were last refreshed |

### Schedule

Primes (items tagged `prime`) are repriced every 30 minutes. Items with a visible order from an in-game or online user, or a closed trade in 48 hours (hot items) are updated every 4 hours. Everything else updates daily, including statistics and history.

With the riven auction crawl, which is also daily and spaces out requests to one per 6 seconds, this API does about 55,000 requests a day against warframe.market and never more than one per second. Since the point of this is to reduce load on the market, this does not change with the number of users Merframe has.
