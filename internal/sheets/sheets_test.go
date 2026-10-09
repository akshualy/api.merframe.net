package sheets

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTab(t *testing.T) {
	srv := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("sheet") {
		case "Primary":
			_, _ = w.Write([]byte("\"WEAPON\",\"POSITIVE STATS:\"\n"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	c := New(srv.Client(), "test", "https://merframe.net")
	c.base = srv.URL + "/export?sheet="

	got, err := c.Tab(t.Context(), "Primary")
	if err != nil {
		t.Fatal(err)
	}

	if got != "\"WEAPON\",\"POSITIVE STATS:\"\n" {
		t.Errorf("tab = %q", got)
	}

	if _, err := c.Tab(t.Context(), "Missing"); err == nil {
		t.Error("expected an error for an unknown tab")
	}
}
