package middleware

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// RedirectTrailingSlash 301-redirects URLs that carry a trailing slash to the
// slashless canonical form (/en/ → /en, /en/products/ → /en/products), so the
// old Astro-era URLs Google still has indexed consolidate onto the live routes.
// The query string is preserved.
//
// It leaves "/" alone, and skips any path that maps to a real directory under
// public/: the catch-all http.FileServer 301s a slashless directory path back to
// the trailing-slash form, so stripping the slash there would create a redirect
// loop. Static asset *files* are unaffected — they never carry a trailing slash.
func RedirectTrailingSlash(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if p == "/" || !strings.HasSuffix(p, "/") {
			next.ServeHTTP(w, r)
			return
		}

		trimmed := strings.TrimRight(p, "/")
		if trimmed == "" {
			trimmed = "/"
		}

		// Don't fight FileServer's own dir → dir/ redirect (would loop).
		if info, err := os.Stat(filepath.Join("public", trimmed)); err == nil && info.IsDir() {
			next.ServeHTTP(w, r)
			return
		}

		u := *r.URL
		u.Path = trimmed
		http.Redirect(w, r, u.String(), http.StatusMovedPermanently)
	})
}
