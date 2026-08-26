package handlers

import (
	"database/sql"

	"bosfoot/internal/site"
	"bosfoot/models"
)

// applyPricing fills a product's display price fields from its euro source price.
// It is the single place the site-wide clearance (site.SalePrice) is applied, so
// every product query — the SSR pages and the JSON API — prices identically.
//
// During a clearance the full (pre-discount) price is kept as OriginalPriceMKD
// for the struck-through display and IsOnSale is forced on; the order handler
// applies the same site.SalePrice, so the charged total matches what's shown.
// With no clearance running the full price stands and any per-product
// original_price_eur (origEUR) is honoured as the "was" price.
func applyPricing(p *models.Product, euro int, origEUR sql.NullInt64) {
	full := site.MKD(euro)
	if site.SaleActive() {
		p.PriceMKD = site.SalePrice(full)
		p.OriginalPriceMKD = &full
		p.IsOnSale = true
		return
	}
	p.PriceMKD = full
	if origEUR.Valid {
		v := site.MKD(int(origEUR.Int64))
		p.OriginalPriceMKD = &v
	}
}
