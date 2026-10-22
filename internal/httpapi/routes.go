package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/akshualy/api.merframe.net/internal/publish"
)

func addRoutes(mux *http.ServeMux, blobs *Blobs, store Store, log *slog.Logger) {
	mux.Handle("GET /healthz", handleHealth())
	mux.Handle("GET /readyz", handleReady(blobs))

	mux.Handle("GET /v1/prices", handleBlob(blobs, publish.PricesBlob, pricesCaching))
	mux.Handle("GET /v1/prices/{slug}", handleItemPrice(blobs))
	mux.Handle("GET /v1/prices/{slug}/history", handleHistory(store, log))

	mux.Handle("GET /v1/items", handleBlob(blobs, publish.CatalogBlob, catalogCaching))

	mux.Handle("GET /v1/riven", handleBlob(blobs, publish.RivenBlob, rivenCaching))
	mux.Handle("GET /v1/riven/auctions/{slug}", handleAuctions(blobs, store, log))

	mux.Handle("/", handleNotFound())
}
