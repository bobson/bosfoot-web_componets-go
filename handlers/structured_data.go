package handlers

import (
	"database/sql"
	"strconv"
	"strings"

	"bosfoot/models"
)

// defaultSocialImage is the site-relative fallback image used for social cards
// and Article structured data when a page has no image of its own. Keep in sync
// with the og:image default in templates/partials/head.html.
const defaultSocialImage = "/images/freet/images/hero.webp"

// absURL turns a site-relative path into an absolute URL. Pass-through if the
// path is empty or already absolute.
func absURL(siteURL, p string) string {
	if p == "" || strings.HasPrefix(p, "http") {
		return p
	}
	return siteURL + p
}

// homeStructuredData returns Organization + WebSite JSON-LD for the homepage,
// establishing the brand entity for search engines.
func homeStructuredData(siteURL string) []any {
	return []any{
		map[string]any{
			"@context":    "https://schema.org",
			"@type":       "Organization",
			"name":        "Bosfoot",
			"url":         siteURL,
			"description": "Barefoot and minimalist shoes from carefully chosen European brands.",
		},
		map[string]any{
			"@context": "https://schema.org",
			"@type":    "WebSite",
			"name":     "Bosfoot",
			"url":      siteURL,
		},
	}
}

// productStructuredData returns a schema.org Product (with an Offer) for a
// product detail page. price is emitted as a string and availability is derived
// from live stock, so Google can show price/availability rich results.
func productStructuredData(siteURL, productURL, desc string, p models.Product) map[string]any {
	availability := "https://schema.org/OutOfStock"
	for _, s := range p.Stock {
		if s.Qty > 0 {
			availability = "https://schema.org/InStock"
			break
		}
	}

	product := map[string]any{
		"@context":    "https://schema.org",
		"@type":       "Product",
		"name":        p.BrandName + " " + p.Name,
		"sku":         p.SKU,
		"description": desc,
		"brand":       map[string]any{"@type": "Brand", "name": p.BrandName},
		"offers": map[string]any{
			"@type":         "Offer",
			"url":           productURL,
			"priceCurrency": "MKD",
			"price":         strconv.Itoa(p.PriceMKD),
			"availability":  availability,
			"itemCondition": "https://schema.org/NewCondition",
		},
	}
	if p.ImageURL != nil {
		product["image"] = absURL(siteURL, *p.ImageURL)
	}
	return product
}

// articleStructuredData returns a schema.org Article for an article detail page.
func articleStructuredData(articleURL, title, lead, author, image string, published sql.NullTime) map[string]any {
	m := map[string]any{
		"@context":         "https://schema.org",
		"@type":            "Article",
		"headline":         title,
		"mainEntityOfPage": articleURL,
		"publisher":        map[string]any{"@type": "Organization", "name": "Bosfoot"},
	}
	if lead != "" {
		m["description"] = lead
	}
	if author != "" {
		m["author"] = map[string]any{"@type": "Person", "name": author}
	}
	if image != "" {
		m["image"] = image
	}
	if published.Valid {
		m["datePublished"] = published.Time.Format("2006-01-02")
	}
	return m
}

// breadcrumbLD builds a schema.org BreadcrumbList from ordered {name, url} pairs.
func breadcrumbLD(items ...[2]string) map[string]any {
	list := make([]any, 0, len(items))
	for i, it := range items {
		list = append(list, map[string]any{
			"@type":    "ListItem",
			"position": i + 1,
			"name":     it[0],
			"item":     it[1],
		})
	}
	return map[string]any{
		"@context":        "https://schema.org",
		"@type":           "BreadcrumbList",
		"itemListElement": list,
	}
}
