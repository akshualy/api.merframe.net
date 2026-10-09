package httpapi

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/akshualy/api.merframe.net/internal/market"
)

type Store interface {
	Price(ctx context.Context, slug string) (market.Price, error)
	History(ctx context.Context, slug string) ([]market.Day, error)
}

func NewServer(blobs *Blobs, store Store, log *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	addRoutes(mux, blobs, store, log)
	return mux
}

func writeJSON(w http.ResponseWriter, status int, body []byte) {
	h := w.Header()
	h.Set("Content-Type", "application/json")
	h.Set("Content-Length", strconv.Itoa(len(body)))
	h.Set("Cache-Control", noStore)
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func writeDetail(w http.ResponseWriter, status int, detail string) {
	writeJSON(w, status, fmt.Appendf(nil, `{"detail":%q}`, detail))
}
