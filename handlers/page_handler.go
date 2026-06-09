package handlers

import (
	"bosfoot/internal/locale"
	"bosfoot/internal/tmpl"
	"bosfoot/logger"
	"bosfoot/models"
	"database/sql"
	"net/http"

	"github.com/lib/pq"
)

// PageHandler serves SSR pages. Each method renders a full HTML page.
type PageHandler struct {
	DB       *sql.DB
	Logger   *logger.Logger
	Renderer *tmpl.Renderer
	SiteURL  string // e.g. "https://bosfoot.com" — no trailing slash
}

// PageBase holds the fields every page template needs.
type PageBase struct {
	Locale          string
	CurrentPath     string // path after the locale prefix, used by the language switcher
	SiteURL         string // used for canonical and hreflang tags
	MetaDescription string
}

// ProductListingData is the template data for /{locale}/products.
type ProductListingData struct {
	PageBase
	Products []models.Product
	Count    int
}

// ProductListing handles GET /{locale}/products.
func (h *PageHandler) ProductListing(w http.ResponseWriter, r *http.Request) {
	loc := locale.FromPath(r.PathValue("locale"))
	if !locale.IsValid(r.PathValue("locale")) {
		http.Redirect(w, r, "/"+locale.Default+"/products", http.StatusFound)
		return
	}
	ctx := r.Context()

	// Fetch published products + brand info.
	rows, err := h.DB.QueryContext(ctx, `
		SELECT p.id, p.sku, p.name, p.slug,
		       p.brand_id, p.category_id, p.gender_id,
		       p.price_mkd, p.original_price_mkd, p.image_url,
		       p.is_new, p.is_featured, p.is_on_sale, p.discount_pct,
		       p.is_active, p.is_published, p.sort_order,
		       p.created_at, p.updated_at,
		       b.name, b.slug
		FROM products p
		JOIN brands b ON b.id = p.brand_id
		WHERE p.is_active = TRUE AND p.is_published = TRUE
		ORDER BY p.sort_order ASC, p.created_at DESC
	`)
	if err != nil {
		h.Logger.Error("ProductListing: query failed", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var products []models.Product
	var ids []int

	for rows.Next() {
		var p models.Product
		var origPrice sql.NullInt64
		var imageURL sql.NullString
		var discountPct sql.NullFloat64

		if err := rows.Scan(
			&p.ID, &p.SKU, &p.Name, &p.Slug,
			&p.BrandID, &p.CategoryID, &p.GenderID,
			&p.PriceMKD, &origPrice, &imageURL,
			&p.IsNew, &p.IsFeatured, &p.IsOnSale, &discountPct,
			&p.IsActive, &p.IsPublished, &p.SortOrder,
			&p.CreatedAt, &p.UpdatedAt,
			&p.BrandName, &p.BrandSlug,
		); err != nil {
			h.Logger.Error("ProductListing: scan failed", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		if origPrice.Valid {
			v := int(origPrice.Int64)
			p.OriginalPriceMKD = &v
		}
		if imageURL.Valid {
			p.ImageURL = &imageURL.String
		}
		if discountPct.Valid {
			p.DiscountPct = &discountPct.Float64
		}
		products = append(products, p)
		ids = append(ids, p.ID)
	}
	if err := rows.Err(); err != nil {
		h.Logger.Error("ProductListing: rows error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Fetch colors for all products in one query.
	if len(ids) > 0 {
		colorRows, err := h.DB.QueryContext(ctx, `
			SELECT product_id, color, hex
			FROM product_colors
			WHERE product_id = ANY($1)
			ORDER BY product_id, color
		`, pq.Array(ids))
		if err != nil {
			h.Logger.Error("ProductListing: colors query failed", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		defer colorRows.Close()

		colorMap := make(map[int][]models.ProductColor)
		for colorRows.Next() {
			var productID int
			var color string
			var hex sql.NullString
			if err := colorRows.Scan(&productID, &color, &hex); err != nil {
				h.Logger.Error("ProductListing: color scan failed", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			c := models.ProductColor{Color: color}
			if hex.Valid {
				c.Hex = &hex.String
			}
			colorMap[productID] = append(colorMap[productID], c)
		}
		for i := range products {
			products[i].Colors = colorMap[products[i].ID]
		}
	}

	h.Renderer.Render(w, "products", ProductListingData{
		PageBase: PageBase{
			Locale:      loc,
			CurrentPath: "/products",
			SiteURL:     h.SiteURL,
		},
		Products: products,
		Count:    len(products),
	})
}
