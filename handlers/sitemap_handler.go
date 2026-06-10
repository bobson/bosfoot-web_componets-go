package handlers

import (
	"fmt"
	"net/http"
	"strings"
)

// Sitemap handles GET /sitemap.xml.
// Generates a multilingual sitemap with hreflang annotations so Google indexes
// each language version correctly instead of treating them as duplicate pages.
// Add a route here each time a new SSR page is built and registered in main.go.
func (h *PageHandler) Sitemap(w http.ResponseWriter, r *http.Request) {
	base := h.SiteURL
	locales := []string{"mk", "sq", "en"}

	var b strings.Builder

	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"` + "\n")
	b.WriteString(`        xmlns:xhtml="http://www.w3.org/1999/xhtml">` + "\n")

	// writeURL emits one <url> block. path is the part after the locale prefix,
	// e.g. "/products". defaultPath is the x-default href (usually the same as
	// the mk path).
	writeURL := func(localedPath, changefreq, priority string) {
		// Extract locale and base path from localedPath (e.g. "/mk/products")
		// so we can build all three alternate hrefs.
		// localedPath already contains the locale, e.g. "/mk/products".
		// We need the path-after-locale to build the other locales.
		// Caller passes pre-built full localed path; alternates are derived here.

		b.WriteString("  <url>\n")
		fmt.Fprintf(&b, "    <loc>%s%s</loc>\n", base, localedPath)

		// All alternates — each locale variant of this page.
		// pathSuffix is everything after "/{locale}", e.g. "/products".
		parts := strings.SplitN(strings.TrimPrefix(localedPath, "/"), "/", 2)
		suffix := ""
		if len(parts) == 2 {
			suffix = "/" + parts[1]
		}
		for _, loc := range locales {
			fmt.Fprintf(&b,
				"    <xhtml:link rel=\"alternate\" hreflang=\"%s\" href=\"%s/%s%s\"/>\n",
				loc, base, loc, suffix,
			)
		}
		// x-default points to the primary language (mk).
		fmt.Fprintf(&b,
			"    <xhtml:link rel=\"alternate\" hreflang=\"x-default\" href=\"%s/mk%s\"/>\n",
			base, suffix,
		)

		if changefreq != "" {
			fmt.Fprintf(&b, "    <changefreq>%s</changefreq>\n", changefreq)
		}
		if priority != "" {
			fmt.Fprintf(&b, "    <priority>%s</priority>\n", priority)
		}
		b.WriteString("  </url>\n")
	}

	// Only emit routes that are currently live.
	// Add a route here each time a new SSR page is built and registered in main.go.
	for _, loc := range locales {
		writeURL("/"+loc+"/products", "weekly", "0.9")
	}

	// Product detail pages.
	pRows, err := h.DB.QueryContext(r.Context(), `
		SELECT p.slug, b.slug
		FROM products p JOIN brands b ON b.id = p.brand_id
		WHERE p.is_active = TRUE AND p.is_published = TRUE
		ORDER BY b.sort_order, p.sort_order
	`)
	if err == nil {
		defer pRows.Close()
		for pRows.Next() {
			var pSlug, bSlug string
			if err := pRows.Scan(&pSlug, &bSlug); err == nil {
				writeURL("/mk/products/"+bSlug+"/"+pSlug, "weekly", "0.8")
			}
		}
	}

	b.WriteString("</urlset>\n")

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	fmt.Fprint(w, b.String())
}
