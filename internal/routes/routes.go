package routes

import (
	"bosfoot/handlers"
	"bosfoot/internal/cache"
	"bosfoot/internal/locale"
	"bosfoot/internal/middleware"
	"net/http"
	"strings"
)

func Register(
	productHandler *handlers.ProductHandler,
	orderHandler *handlers.OrderHandler,
	pageHandler *handlers.PageHandler,
	trackHandler *handlers.TrackHandler,
	reviewHandler *handlers.ReviewHandler,
	pc *cache.Cache,
) {
	// API handlers (JSON, CSR)
	http.HandleFunc("/api/brands", productHandler.GetBrands)
	http.HandleFunc("/api/products", productHandler.GetProducts)
	http.HandleFunc("/api/products/{id}", productHandler.GetProductByID)
	http.HandleFunc("/api/products/{id}/stock", productHandler.GetProductStock)
	http.HandleFunc("/api/products/{id}/stock/stream", productHandler.StreamProductStock)

	http.HandleFunc("/api/orders", middleware.WrapCSRF(orderHandler.CreateOrder))
	http.HandleFunc("/api/reviews", middleware.WrapCSRF(reviewHandler.CreateReview))

	// Funnel beacon for add-to-cart (the only client-side-only funnel step).
	// Not CSRF-wrapped: it changes no state — it only appends to the log.
	http.HandleFunc("/api/track", trackHandler.Track)

	// Page handlers (HTML, SSR)
	for _, loc := range []string{"mk", "sq", "en"} {
		http.HandleFunc("/"+loc, pc.Wrap(pageHandler.Home))
	}
	http.HandleFunc("/{locale}/products", pc.Wrap(pageHandler.ProductListing))
	http.HandleFunc("/{locale}/products/{brand}/{slug}", pc.Wrap(pageHandler.ProductDetail))
	http.HandleFunc("/{locale}/brands", pc.Wrap(pageHandler.Brands))
	http.HandleFunc("/{locale}/size-guide", pc.Wrap(pageHandler.SizeGuide))
	http.HandleFunc("/{locale}/size-finder", pc.Wrap(pageHandler.SizeFinder))
	http.HandleFunc("/{locale}/about", pc.Wrap(pageHandler.About))
	http.HandleFunc("/{locale}/contact", pc.Wrap(pageHandler.Contact))
	http.HandleFunc("/{locale}/checkout", middleware.WithCSRFCookie(pc.Wrap(pageHandler.Checkout)))
	// Review token page: per-token + shows used/invalid state, so it must NOT be
	// cached. WithCSRFCookie seeds the _csrf cookie the review form POST needs.
	http.HandleFunc("/{locale}/review/{token}", middleware.WithCSRFCookie(pageHandler.ReviewPage))
	http.HandleFunc("/{locale}/foot-health", pc.Wrap(pageHandler.FootHealth))
	http.HandleFunc("/{locale}/what-are-barefoot-shoes", pc.Wrap(pageHandler.WhatAreBarefootShoes))
	// Three flat barefoot-trait detail pages, all served by BarefootArticle
	// (it picks the article from the path segment after the locale).
	http.HandleFunc("/{locale}/wide-toe-box", pc.Wrap(pageHandler.BarefootArticle))
	http.HandleFunc("/{locale}/zero-drop", pc.Wrap(pageHandler.BarefootArticle))
	http.HandleFunc("/{locale}/thin-flexible-sole", pc.Wrap(pageHandler.BarefootArticle))
	http.HandleFunc("/{locale}/articles", pc.Wrap(pageHandler.ArticlesListing))
	http.HandleFunc("/{locale}/articles/{slug}", pc.Wrap(pageHandler.ArticleDetail))
	http.HandleFunc("/{locale}/privacy", pc.Wrap(pageHandler.Privacy))
	http.HandleFunc("/{locale}/terms", pc.Wrap(pageHandler.Terms))
	http.HandleFunc("/{locale}/returns", pc.Wrap(pageHandler.Returns))
	http.HandleFunc("/{locale}/shipping", pc.Wrap(pageHandler.Shipping))
	http.HandleFunc("/sitemap.xml", pageHandler.Sitemap)

	// Catch-all: static files from public/ with tiered caching (see
	// setStaticCacheHeaders). Keeps fingerprinted assets cached hard while
	// letting un-fingerprinted files (e.g. component JS modules) revalidate, so
	// code/content updates reach clients without a manual cache purge.
	fileServer := http.FileServer(http.Dir("public"))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			// Send returning visitors to the language they last viewed (set client-
			// side in nav-locale.js), defaulting to mk. The destination now varies by
			// cookie, so forbid any shared cache (Cloudflare/browser) from pinning one
			// visitor's redirect for everyone.
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("Vary", "Cookie")
			http.Redirect(w, r, localeRedirectDest(r), http.StatusFound)
			return
		}
		setStaticCacheHeaders(w, r)
		fileServer.ServeHTTP(w, r)
	})
}

// localeRedirectDest picks where "/" sends a visitor: the language from their
// bosfoot_locale cookie if it's a valid locale, else the mk default. The
// IsValid guard is essential — it prevents a tampered cookie from turning the
// redirect into an open redirect (e.g. "//evil.com").
func localeRedirectDest(r *http.Request) string {
	if c, err := r.Cookie("bosfoot_locale"); err == nil && locale.IsValid(c.Value) {
		return "/" + c.Value
	}
	return "/mk"
}

// setStaticCacheHeaders applies a Cache-Control tier based on the request:
//   - fingerprinted URLs (the asset helper adds ?v=<content-hash>) are immutable
//     and cached for a year — the URL changes whenever the content does;
//   - un-fingerprinted .js (the component ES modules imported by app.js) uses
//     no-store: the Facebook in-app browser ignores no-cache and serves stale
//     JS, which silently breaks the ad audience after a JS-only deploy (the
//     imported modules never bump app.js's hash). no-store forbids storing it,
//     so the fix reaches every visitor immediately. These files are tiny.
//   - images/fonts rarely change and aren't fingerprinted, so cache for a week;
//   - everything else (JSON, HTML) uses no-cache, meaning it is still stored
//     but revalidated (cheap 304s) so updates are picked up at once.
func setStaticCacheHeaders(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Query().Has("v"):
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	case strings.HasSuffix(r.URL.Path, ".js"):
		w.Header().Set("Cache-Control", "no-store")
	case isLongLivedAsset(r.URL.Path):
		w.Header().Set("Cache-Control", "public, max-age=604800")
	default:
		w.Header().Set("Cache-Control", "no-cache")
	}
}

// isLongLivedAsset reports whether a path is an image or font — content that
// changes rarely and is safe to cache for a while even without a fingerprint.
func isLongLivedAsset(path string) bool {
	switch {
	case strings.HasSuffix(path, ".webp"),
		strings.HasSuffix(path, ".avif"),
		strings.HasSuffix(path, ".png"),
		strings.HasSuffix(path, ".jpg"),
		strings.HasSuffix(path, ".jpeg"),
		strings.HasSuffix(path, ".gif"),
		strings.HasSuffix(path, ".svg"),
		strings.HasSuffix(path, ".ico"),
		strings.HasSuffix(path, ".woff"),
		strings.HasSuffix(path, ".woff2"):
		return true
	default:
		return false
	}
}
