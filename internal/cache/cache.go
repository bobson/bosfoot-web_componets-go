// Package cache provides a tiny in-memory TTL cache for rendered SSR pages.
//
// The home, listing, and product-detail pages are byte-identical for every
// visitor (the cart is client-side state, not part of the SSR HTML), so the
// whole response can be cached by URL path. A cache hit skips both the database
// queries and the template rendering — it just writes the stored bytes.
//
// Only GET requests that produce a 200 are cached; redirects, 404s, 500s, and
// non-GET requests pass straight through. A short TTL keeps prices/stock fresh
// without any explicit invalidation; Flush is provided for after a data import.
package cache

import (
	"bytes"
	"net/http"
	"sync"
	"time"
)

type entry struct {
	body        []byte
	contentType string
	expires     time.Time
}

// Cache is a concurrency-safe path -> rendered-page map with a TTL.
type Cache struct {
	mu  sync.RWMutex
	ttl time.Duration
	m   map[string]entry
}

// New returns a cache whose entries live for ttl.
func New(ttl time.Duration) *Cache {
	return &Cache{ttl: ttl, m: make(map[string]entry)}
}

// Flush empties the cache (e.g. after cmd/dbimport changes the catalog).
func (c *Cache) Flush() {
	c.mu.Lock()
	c.m = make(map[string]entry)
	c.mu.Unlock()
}

// Wrap returns next wrapped with caching keyed by request path.
func (c *Cache) Wrap(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			next(w, r)
			return
		}

		key := r.URL.Path

		c.mu.RLock()
		e, ok := c.m[key]
		c.mu.RUnlock()
		if ok && time.Now().Before(e.expires) {
			w.Header().Set("Content-Type", e.contentType)
			w.Header().Set("X-Cache", "HIT")
			w.Write(e.body)
			return
		}

		w.Header().Set("X-Cache", "MISS")
		rec := &recorder{ResponseWriter: w, status: http.StatusOK}
		next(rec, r)

		// Only cache a clean, fully-rendered 200.
		if rec.status == http.StatusOK {
			body := make([]byte, rec.buf.Len())
			copy(body, rec.buf.Bytes())
			c.mu.Lock()
			c.m[key] = entry{
				body:        body,
				contentType: rec.Header().Get("Content-Type"),
				expires:     time.Now().Add(c.ttl),
			}
			c.mu.Unlock()
		}
	}
}

// recorder tees the response body into a buffer while passing it through to the
// real ResponseWriter, and remembers the status code.
type recorder struct {
	http.ResponseWriter
	buf         bytes.Buffer
	status      int
	wroteHeader bool
}

func (rec *recorder) WriteHeader(code int) {
	if !rec.wroteHeader {
		rec.status = code
		rec.wroteHeader = true
	}
	rec.ResponseWriter.WriteHeader(code)
}

func (rec *recorder) Write(b []byte) (int, error) {
	if !rec.wroteHeader {
		rec.wroteHeader = true // first write implies a 200
	}
	rec.buf.Write(b)
	return rec.ResponseWriter.Write(b)
}
