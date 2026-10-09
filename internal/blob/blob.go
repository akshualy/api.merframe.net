package blob

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"

	"github.com/andybalholm/brotli"
)

// Blob has one strong ETag for the uncompressed body that's valid for all encodings.
type Blob struct {
	ETag   string
	Body   []byte
	Gzip   []byte
	Brotli []byte
}

// Build compresses with gzip and brotli, which takes ~1s per megabyte.
func Build(body []byte) *Blob {
	return &Blob{
		ETag:   ETagOf(body),
		Body:   body,
		Gzip:   gzipped(body),
		Brotli: brotlied(body),
	}
}

func ETagOf(body []byte) string {
	sum := sha256.Sum256(body)
	return strconv.Quote(hex.EncodeToString(sum[:]))
}

func gzipped(body []byte) []byte {
	var buf bytes.Buffer
	w, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	_, _ = w.Write(body)
	_ = w.Close()
	return buf.Bytes()
}

func brotlied(body []byte) []byte {
	var buf bytes.Buffer
	w := brotli.NewWriterLevel(&buf, brotli.BestCompression)
	_, _ = w.Write(body)
	_ = w.Close()
	return buf.Bytes()
}

type Caching struct {
	Edge   string
	Client string
}

func Serve(w http.ResponseWriter, r *http.Request, b *Blob, c Caching) {
	if writeValidators(w, r, b.ETag, c) {
		return
	}

	data, encoding := b.encoded(r.Header.Get("Accept-Encoding"))
	if encoding != "" {
		w.Header().Set("Content-Encoding", encoding)
	}

	writeBody(w, r, data)
}

// ServePlain is for small documents that are not worth compressing but still revalidate.
func ServePlain(w http.ResponseWriter, r *http.Request, body []byte, c Caching) {
	if writeValidators(w, r, ETagOf(body), c) {
		return
	}

	writeBody(w, r, body)
}

func writeValidators(w http.ResponseWriter, r *http.Request, etag string, c Caching) bool {
	h := w.Header()
	h.Set("ETag", etag)
	h.Set("Vary", "Accept-Encoding")
	h.Set("Cache-Control", c.Client)
	h.Set("Cloudflare-CDN-Cache-Control", c.Edge)

	if !matches(r.Header.Get("If-None-Match"), etag) {
		return false
	}

	w.WriteHeader(http.StatusNotModified)
	return true
}

func writeBody(w http.ResponseWriter, r *http.Request, data []byte) {
	h := w.Header()
	h.Set("Content-Type", "application/json")
	h.Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(http.StatusOK)

	if r.Method != http.MethodHead {
		_, _ = w.Write(data)
	}
}

// matches also accepts weak validators from a proxy. The body behind them is the same.
func matches(ifNoneMatch, etag string) bool {
	for tag := range strings.SplitSeq(ifNoneMatch, ",") {
		tag = strings.TrimPrefix(strings.TrimSpace(tag), "W/")
		if tag == "*" || tag == etag {
			return true
		}
	}
	return false
}

func (b *Blob) encoded(acceptEncoding string) ([]byte, string) {
	switch {
	case accepts(acceptEncoding, "br"):
		return b.Brotli, "br"
	case accepts(acceptEncoding, "gzip"):
		return b.Gzip, "gzip"
	default:
		return b.Body, ""
	}
}

func accepts(header, coding string) bool {
	for entry := range strings.SplitSeq(header, ",") {
		name, params, _ := strings.Cut(strings.TrimSpace(entry), ";")
		name = strings.ToLower(strings.TrimSpace(name))
		if name != coding && name != "*" {
			continue
		}
		return quality(params) > 0
	}
	return false
}

func quality(params string) float64 {
	for param := range strings.SplitSeq(params, ";") {
		key, value, _ := strings.Cut(strings.TrimSpace(param), "=")
		if strings.ToLower(key) != "q" {
			continue
		}

		q, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return 0
		}
		return q
	}
	return 1
}
