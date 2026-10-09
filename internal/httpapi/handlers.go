package httpapi

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/akshualy/api.merframe.net/internal/blob"
	"github.com/akshualy/api.merframe.net/internal/postgres"
	"github.com/akshualy/api.merframe.net/internal/publish"
)

const unpublishedRetryAfter = "60"

func handleHealth() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, []byte(`"OK"`))
	})
}

func handleReady(blobs *Blobs) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if !blobs.Ready() {
			writeDetail(w, http.StatusServiceUnavailable, "No published data yet")
			return
		}

		writeJSON(w, http.StatusOK, []byte(`"OK"`))
	})
}

func handleNotFound() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeDetail(w, http.StatusNotFound, "Not Found")
	})
}

func handleBlob(blobs *Blobs, name string, caching blob.Caching) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, ok := blobs.Get(name)
		if !ok {
			w.Header().Set("Retry-After", unpublishedRetryAfter)
			writeDetail(w, http.StatusServiceUnavailable, "Not published yet")
			return
		}

		blob.Serve(w, r, b, caching)
	})
}

func handleItemPrice(blobs *Blobs) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		item, ok := blobs.Item(r.PathValue("slug"))
		if !ok {
			writeDetail(w, http.StatusNotFound, "Item not priced")
			return
		}

		blob.ServePlain(w, r, item, itemCaching)
	})
}

func handleHistory(store Store, log *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slug := r.PathValue("slug")
		price, err := store.Price(r.Context(), slug)
		if errors.Is(err, postgres.ErrMissing) {
			writeDetail(w, http.StatusNotFound, "Item has no price history")
			return
		}

		if err != nil {
			log.Error("reading a price failed", "slug", slug, "error", err)
			writeDetail(w, http.StatusInternalServerError, "History could not be read")
			return
		}

		days, err := store.History(r.Context(), slug)
		if err != nil {
			log.Error("reading a history failed", "slug", slug, "error", err)
			writeDetail(w, http.StatusInternalServerError, "History could not be read")
			return
		}

		if len(days) == 0 {
			writeDetail(w, http.StatusNotFound, "Item has no price history")
			return
		}

		body, err := publish.PriceHistory(slug, price, days)
		if err != nil {
			log.Error("encoding a history failed", "slug", slug, "error", err)
			writeDetail(w, http.StatusInternalServerError, "History could not be read")
			return
		}

		blob.ServePlain(w, r, body, itemCaching)
	})
}

func handleAuctions(blobs *Blobs) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, ok := blobs.Get(publish.AuctionsBlob(r.PathValue("slug")))
		if !ok {
			writeDetail(w, http.StatusNotFound, "Weapon not crawled")
			return
		}

		blob.Serve(w, r, b, auctionsCaching)
	})
}
