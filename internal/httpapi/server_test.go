package httpapi

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/akshualy/api.merframe.net/internal/blob"
	"github.com/akshualy/api.merframe.net/internal/market"
	"github.com/akshualy/api.merframe.net/internal/postgres"
	"github.com/akshualy/api.merframe.net/internal/publish"
)

type fakeStore struct {
	prices  map[string]market.Price
	history map[string][]market.Day
	err     error
}

func (f *fakeStore) Price(_ context.Context, slug string) (market.Price, error) {
	if f.err != nil {
		return market.Price{}, f.err
	}

	price, ok := f.prices[slug]
	if !ok {
		return market.Price{}, postgres.ErrMissing
	}
	return price, nil
}

func (f *fakeStore) History(_ context.Context, slug string) ([]market.Day, error) {
	return f.history[slug], f.err
}

var at = time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)

func publishedServer(t *testing.T, store Store) (http.Handler, *Blobs) {
	t.Helper()

	blobs := NewBlobs()
	table := []byte(`{"updated_at":1791460800,"count":1,"items":{"forma_blueprint":{"sell_r0":10,"tier":"hot"}}}`)
	if err := blobs.Set(publish.PricesBlob, blob.Build(table)); err != nil {
		t.Fatal(err)
	}

	if err := blobs.Set(publish.RivenBlob, blob.Build([]byte(`{"weapons":[]}`))); err != nil {
		t.Fatal(err)
	}

	if err := blobs.Set(publish.CatalogBlob, blob.Build([]byte(`{"data":[]}`))); err != nil {
		t.Fatal(err)
	}

	if err := blobs.Set(publish.AuctionsBlob("torid"), blob.Build([]byte(`{"slug":"torid","auctions":[]}`))); err != nil {
		t.Fatal(err)
	}
	return NewServer(blobs, store, slog.New(slog.DiscardHandler)), blobs
}

func get(t *testing.T, h http.Handler, path string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, path, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestRoutes(t *testing.T) {
	store := &fakeStore{
		prices:  map[string]market.Price{"forma_blueprint": {UpdatedAt: at}},
		history: map[string][]market.Day{"forma_blueprint": {{Day: at, Volume: 3, Median: 10, Min: 9, Max: 11, WA: 10.2}}},
	}
	h, _ := publishedServer(t, store)
	tests := []struct {
		path         string
		wantStatus   int
		wantCache    string
		wantEdge     string
		wantBodyPart string
	}{
		{"/healthz", http.StatusOK, noStore, "", `"OK"`},
		{"/readyz", http.StatusOK, noStore, "", `"OK"`},
		{"/v1/prices", http.StatusOK, pricesCaching.Client, pricesCaching.Edge, `"forma_blueprint"`},
		{"/v1/prices/forma_blueprint", http.StatusOK, itemCaching.Client, itemCaching.Edge, `{"sell_r0":10,"tier":"hot"}`},
		{"/v1/prices/nothing", http.StatusNotFound, noStore, "", "Item not priced"},
		{"/v1/prices/forma_blueprint/history", http.StatusOK, itemCaching.Client, itemCaching.Edge, `"day":"2026-10-08"`},
		{"/v1/prices/nothing/history", http.StatusNotFound, noStore, "", "Item has no price history"},
		{"/v1/items", http.StatusOK, catalogCaching.Client, catalogCaching.Edge, `"data"`},
		{"/v1/riven", http.StatusOK, rivenCaching.Client, rivenCaching.Edge, `"weapons"`},
		{"/v1/riven/auctions/torid", http.StatusOK, auctionsCaching.Client, auctionsCaching.Edge, `"torid"`},
		{"/v1/riven/auctions/nothing", http.StatusNotFound, noStore, "", "Weapon not crawled"},
		{"/v1/nothing", http.StatusNotFound, noStore, "", "Not Found"},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			rec := get(t, h, tt.path, nil)
			body, _ := io.ReadAll(rec.Body)
			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d (%s)", rec.Code, tt.wantStatus, body)
			}

			if got := rec.Header().Get("Cache-Control"); got != tt.wantCache {
				t.Errorf("Cache-Control = %q, want %q", got, tt.wantCache)
			}

			if got := rec.Header().Get("Cloudflare-CDN-Cache-Control"); got != tt.wantEdge {
				t.Errorf("Cloudflare-CDN-Cache-Control = %q, want %q", got, tt.wantEdge)
			}

			if tt.wantBodyPart != "" && !bytes.Contains(body, []byte(tt.wantBodyPart)) {
				t.Errorf("body = %s, want it to contain %s", body, tt.wantBodyPart)
			}
		})
	}
}

func TestRevalidation(t *testing.T) {
	h, blobs := publishedServer(t, &fakeStore{})
	b, _ := blobs.Get(publish.PricesBlob)
	rec := get(t, h, "/v1/prices", map[string]string{"If-None-Match": b.ETag, "Accept-Encoding": "br"})
	if rec.Code != http.StatusNotModified || rec.Header().Get("ETag") != b.ETag {
		t.Errorf("status %d, ETag %q, want 304 with the ETag", rec.Code, rec.Header().Get("ETag"))
	}

	rec = get(t, h, "/v1/prices", map[string]string{"Accept-Encoding": "br"})
	if rec.Header().Get("Content-Encoding") != "br" || rec.Body.Len() != len(b.Brotli) || rec.Header().Get("Vary") != "Accept-Encoding" {
		t.Errorf("encoding %q with %d bytes, want brotli of %d", rec.Header().Get("Content-Encoding"), rec.Body.Len(), len(b.Brotli))
	}

	rec = get(t, h, "/v1/riven/auctions/torid", map[string]string{"Accept-Encoding": "gzip"})
	auctions, _ := blobs.Get(publish.AuctionsBlob("torid"))
	if rec.Header().Get("Content-Encoding") != "gzip" || rec.Body.Len() != len(auctions.Gzip) {
		t.Errorf("auctions encoding %q with %d bytes, want gzip of %d", rec.Header().Get("Content-Encoding"), rec.Body.Len(), len(auctions.Gzip))
	}

	rec = get(t, h, "/v1/riven/auctions/torid", map[string]string{"If-None-Match": auctions.ETag})
	if rec.Code != http.StatusNotModified {
		t.Errorf("auctions revalidation = %d, want 304", rec.Code)
	}

	item, _ := blobs.Item("forma_blueprint")
	rec = get(t, h, "/v1/prices/forma_blueprint", map[string]string{"If-None-Match": blob.ETagOf(item)})
	if rec.Code != http.StatusNotModified {
		t.Errorf("item revalidation = %d, want 304", rec.Code)
	}

	req := httptest.NewRequest(http.MethodHead, "/v1/prices", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Body.Len() != 0 || rec.Header().Get("ETag") != b.ETag {
		t.Errorf("HEAD = %d with %d bytes and ETag %q, want 200, no body and the ETag", rec.Code, rec.Body.Len(), rec.Header().Get("ETag"))
	}
}

func TestUnpublished(t *testing.T) {
	h := NewServer(NewBlobs(), &fakeStore{}, slog.New(slog.DiscardHandler))

	rec := get(t, h, "/v1/prices", nil)
	if rec.Code != http.StatusServiceUnavailable || rec.Header().Get("Retry-After") != unpublishedRetryAfter {
		t.Errorf("status %d, Retry-After %q, want 503 and %s", rec.Code, rec.Header().Get("Retry-After"), unpublishedRetryAfter)
	}

	if rec := get(t, h, "/readyz", nil); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("readyz = %d, want 503 before the first publish", rec.Code)
	}
}

func TestStoreFailure(t *testing.T) {
	h, _ := publishedServer(t, &fakeStore{err: errors.New("connection refused")})

	if rec := get(t, h, "/v1/prices/forma_blueprint/history", nil); rec.Code != http.StatusInternalServerError {
		t.Errorf("history = %d, want 500 when the store fails", rec.Code)
	}

	if rec := get(t, h, "/v1/prices/forma_blueprint", nil); rec.Code != http.StatusOK {
		t.Errorf("item = %d, want 200 from memory while the store fails", rec.Code)
	}
}
