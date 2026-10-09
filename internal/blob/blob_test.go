package blob

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/andybalholm/brotli"
)

var body = []byte(`{"count":1,"items":{"forma_blueprint":{"sell_r0":10}}}`)

const bodyETag = `"0297aae5532dd3d52bbb481568b4341be67b7de965365d047b14243f720b57de"`

func TestBuild(t *testing.T) {
	b := Build(body)

	if b.ETag != bodyETag {
		t.Errorf("ETag = %s, want %s", b.ETag, bodyETag)
	}

	gz, err := gzip.NewReader(bytes.NewReader(b.Gzip))
	if err != nil {
		t.Fatal(err)
	}

	got, err := io.ReadAll(gz)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(got, body) {
		t.Error("gzip does not round-trip")
	}

	got, err = io.ReadAll(brotli.NewReader(bytes.NewReader(b.Brotli)))
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(got, body) {
		t.Error("brotli does not round-trip")
	}
}

func TestMatches(t *testing.T) {
	b := Build(body)
	tests := []struct {
		header string
		want   bool
	}{
		{b.ETag, true},
		{"W/" + b.ETag, true},
		{`"other", ` + b.ETag, true},
		{"*", true},
		{`"other"`, false},
		{"", false},
	}
	for _, tt := range tests {
		if got := matches(tt.header, b.ETag); got != tt.want {
			t.Errorf("matches(%q) = %v, want %v", tt.header, got, tt.want)
		}
	}
}

func TestEncoded(t *testing.T) {
	b := Build(body)
	tests := []struct {
		header string
		want   string
	}{
		{"br, gzip", "br"},
		{"gzip, deflate", "gzip"},
		{"gzip;q=0, br;q=0.5", "br"},
		{"gzip;q=0.0, br;q=0.000", ""},
		{"br;q=0", ""},
		{"*", "br"},
		{"*;q=0", ""},
		{"", ""},
		{"identity", ""},
	}
	for _, tt := range tests {
		if _, got := b.encoded(tt.header); got != tt.want {
			t.Errorf("encoded(%q) = %q, want %q", tt.header, got, tt.want)
		}
	}
}

func TestServe(t *testing.T) {
	b := Build(body)
	caching := Caching{Edge: "max-age=60", Client: "public, max-age=0"}
	tests := []struct {
		name         string
		method       string
		headers      map[string]string
		wantStatus   int
		wantEncoding string
		wantBody     []byte
	}{
		{"brotli", http.MethodGet, map[string]string{"Accept-Encoding": "br"}, http.StatusOK, "br", b.Brotli},
		{"gzip", http.MethodGet, map[string]string{"Accept-Encoding": "gzip"}, http.StatusOK, "gzip", b.Gzip},
		{"plain", http.MethodGet, nil, http.StatusOK, "", body},
		{"not modified", http.MethodGet, map[string]string{"If-None-Match": b.ETag, "Accept-Encoding": "gzip"}, http.StatusNotModified, "", nil},
		{"head", http.MethodHead, map[string]string{"Accept-Encoding": "br"}, http.StatusOK, "br", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/v1/prices", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			rec := httptest.NewRecorder()
			Serve(rec, req, b, caching)
			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}

			if got := rec.Header().Get("Content-Encoding"); got != tt.wantEncoding {
				t.Errorf("Content-Encoding = %q, want %q", got, tt.wantEncoding)
			}

			if !bytes.Equal(rec.Body.Bytes(), tt.wantBody) {
				t.Errorf("body is %d bytes, want %d", rec.Body.Len(), len(tt.wantBody))
			}

			h := rec.Header()

			if h.Get("ETag") != b.ETag || h.Get("Cache-Control") != caching.Client || h.Get("Cloudflare-CDN-Cache-Control") != caching.Edge || h.Get("Vary") != "Accept-Encoding" {
				t.Errorf("headers = %v", h)
			}
		})
	}
}

func TestServePlain(t *testing.T) {
	caching := Caching{Edge: "max-age=60", Client: "public, max-age=60"}
	etag := ETagOf(body)
	req := httptest.NewRequest(http.MethodGet, "/v1/prices/forma_blueprint", nil)
	req.Header.Set("Accept-Encoding", "br")
	rec := httptest.NewRecorder()
	ServePlain(rec, req, body, caching)
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Encoding") != "" || !bytes.Equal(rec.Body.Bytes(), body) {
		t.Errorf("status %d, encoding %q, body %d bytes, want the plain body", rec.Code, rec.Header().Get("Content-Encoding"), rec.Body.Len())
	}

	if rec.Header().Get("ETag") != etag || rec.Header().Get("Cloudflare-CDN-Cache-Control") != caching.Edge {
		t.Errorf("headers = %v", rec.Header())
	}

	req.Header.Set("If-None-Match", etag)
	rec = httptest.NewRecorder()
	ServePlain(rec, req, body, caching)
	if rec.Code != http.StatusNotModified || rec.Body.Len() != 0 {
		t.Errorf("status %d with %d bytes, want 304 and no body", rec.Code, rec.Body.Len())
	}
}
