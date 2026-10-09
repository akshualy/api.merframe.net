package wfm

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/akshualy/api.merframe.net/internal/throttle"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "wfm", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func newClient(t *testing.T, handler http.Handler) *Client {
	t.Helper()
	srv := httptest.NewTestServer(t, handler)
	c := New(srv.Client(), throttle.New(1000), "test", "https://merframe.net")
	c.v1 = srv.URL + "/v1"
	c.v2 = srv.URL + "/v2"
	return c
}

func serveFixtures(t *testing.T) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	routes := map[string]string{
		"GET /v2/items":                               "items.json",
		"GET /v2/orders/item/secura_dual_cestra":      "orders_item.json",
		"GET /v2/orders/item/primed_flow":             "orders_item_ranked.json",
		"GET /v1/items/secura_dual_cestra/statistics": "statistics.json",
		"GET /v2/riven/weapons":                       "riven_weapons.json",
		"GET /v2/riven/attributes":                    "riven_attributes.json",
		"GET /v1/auctions/search":                     "auctions_search.json",
	}
	for pattern, name := range routes {
		body := fixture(t, name)
		mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("User-Agent") != "api.merframe.net test (+https://merframe.net)" {
				t.Errorf("User-Agent = %q", r.Header.Get("User-Agent"))
			}

			_, _ = w.Write(body)
		})
	}
	return mux
}

func TestItems(t *testing.T) {
	catalog, err := newClient(t, serveFixtures(t)).Items(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	if len(catalog.Items) != 7 || len(catalog.Document) == 0 {
		t.Fatalf("%d items, %d raw bytes", len(catalog.Items), len(catalog.Document))
	}

	i := slices.IndexFunc(catalog.Items, func(item Item) bool { return item.Slug == "primed_flow" })
	if i < 0 || catalog.Items[i].MaxRank == nil || *catalog.Items[i].MaxRank != 10 {
		t.Errorf("primed_flow not found with maxRank 10 in %+v", catalog.Items)
	}
}

func TestOrders(t *testing.T) {
	c := newClient(t, serveFixtures(t))

	ranked, err := c.Orders(t.Context(), "primed_flow")
	if err != nil {
		t.Fatal(err)
	}

	if len(ranked) != 10 || ranked[0].Rank == nil || *ranked[0].Rank != 10 {
		t.Errorf("%d ranked orders, first rank %v", len(ranked), ranked[0].Rank)
	}

	unranked, err := c.Orders(t.Context(), "secura_dual_cestra")
	if err != nil {
		t.Fatal(err)
	}

	if unranked[0].Rank != nil || unranked[0].User.Status == "" {
		t.Errorf("unranked order parsed as %+v", unranked[0])
	}
}

func TestStatistics(t *testing.T) {
	closed, err := newClient(t, serveFixtures(t)).Statistics(t.Context(), "secura_dual_cestra")
	if err != nil {
		t.Fatal(err)
	}

	if len(closed.Last48Hours) != 19 || len(closed.Last90Days) != 89 {
		t.Errorf("48h %d, 90d %d", len(closed.Last48Hours), len(closed.Last90Days))
	}
}

func TestRivenLists(t *testing.T) {
	c := newClient(t, serveFixtures(t))

	weapons, err := c.RivenWeapons(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	if len(weapons) != 12 || weapons[0].I18n.En.Name == "" {
		t.Errorf("%d weapons, first %+v", len(weapons), weapons[0])
	}

	attributes, err := c.RivenAttributes(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	if len(attributes) != 8 || attributes[0].GameRef == "" {
		t.Errorf("%d attributes, first %+v", len(attributes), attributes[0])
	}
}

func TestSearchAuctions(t *testing.T) {
	var query string
	base := serveFixtures(t)
	spy := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		base.ServeHTTP(w, r)
	})

	auctions, err := newClient(t, spy).SearchAuctions(t.Context(), AuctionSearch{
		Weapon: "torid", Polarity: "madurai", ReRollsMin: new(5), ReRollsMax: new(14),
	})
	if err != nil {
		t.Fatal(err)
	}

	want := "polarity=madurai&re_rolls_max=14&re_rolls_min=5&sort_by=price_asc&type=riven&weapon_url_name=torid"
	if query != want {
		t.Errorf("query = %q, want %q", query, want)
	}

	if len(auctions) != 5 || auctions[0].Item.Attributes[0].URLName == "" {
		t.Errorf("%d auctions, first %+v", len(auctions), auctions[0])
	}
}

func TestNotFound(t *testing.T) {
	c := newClient(t, http.NotFoundHandler())

	if _, err := c.Orders(t.Context(), "nothing"); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestRetries(t *testing.T) {
	t.Run("5xx", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			var calls atomic.Int32
			body := fixture(t, "riven_attributes.json")
			c := newClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if calls.Add(1) < 3 {
					w.WriteHeader(http.StatusBadGateway)
					return
				}

				_, _ = w.Write(body)
			}))

			start := time.Now()

			if _, err := c.RivenAttributes(t.Context()); err != nil {
				t.Fatal(err)
			}

			if got := time.Since(start); got != 6*time.Second {
				t.Errorf("two retries waited %v, want 2s + 4s", got)
			}
		})
	})

	t.Run("429", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			var calls atomic.Int32
			body := fixture(t, "riven_attributes.json")
			c := newClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if calls.Add(1) == 1 {
					w.Header().Set("Retry-After", "7")
					w.WriteHeader(http.StatusTooManyRequests)
					return
				}

				_, _ = w.Write(body)
			}))

			start := time.Now()

			if _, err := c.RivenAttributes(t.Context()); err != nil {
				t.Fatal(err)
			}

			if got, want := time.Since(start), 7*time.Second+retryAfterSlack; got != want {
				t.Errorf("waited %v, want %v", got, want)
			}
		})
	})

	t.Run("exhausted", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			c := newClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusServiceUnavailable)
			}))

			var status *StatusError
			if _, err := c.RivenAttributes(t.Context()); !errors.As(err, &status) || status.Status != http.StatusServiceUnavailable {
				t.Errorf("err = %v, want a 503 StatusError", err)
			}
		})
	})

	t.Run("4xx", func(t *testing.T) {
		var calls atomic.Int32
		c := newClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			calls.Add(1)
			w.WriteHeader(http.StatusForbidden)
		}))

		if _, err := c.RivenAttributes(t.Context()); err == nil {
			t.Fatal("expected an error")
		}

		if calls.Load() != 1 {
			t.Errorf("%d calls, want 1", calls.Load())
		}
	})
}
