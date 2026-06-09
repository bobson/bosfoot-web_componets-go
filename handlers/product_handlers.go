package handlers

import (
	"bosfoot/logger"
	"bosfoot/models"
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/lib/pq"
)

type ProductHandler struct {
	DB     *sql.DB
	Logger *logger.Logger
}

func (h *ProductHandler) writeJSONResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.Logger.Error("failed to encode JSON response", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// GetProducts returns all active, published products with brand info and colors.
// Used for the product listing/shop page.
func (h *ProductHandler) GetProducts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

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
		h.Logger.Error("GetProducts: query failed", err)
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
			h.Logger.Error("GetProducts: scan failed", err)
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
		h.Logger.Error("GetProducts: rows error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if len(ids) == 0 {
		h.writeJSONResponse(w, []models.Product{})
		return
	}

	// Fetch colors for all products in a single query, then map them in Go.
	colorRows, err := h.DB.QueryContext(ctx, `
		SELECT product_id, color, hex
		FROM product_colors
		WHERE product_id = ANY($1)
		ORDER BY product_id, color
	`, pq.Array(ids))
	if err != nil {
		h.Logger.Error("GetProducts: colors query failed", err)
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
			h.Logger.Error("GetProducts: color scan failed", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		c := models.ProductColor{Color: color}
		if hex.Valid {
			c.Hex = &hex.String
		}
		colorMap[productID] = append(colorMap[productID], c)
	}
	if err := colorRows.Err(); err != nil {
		h.Logger.Error("GetProducts: color rows error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	for i := range products {
		products[i].Colors = colorMap[products[i].ID]
	}

	h.writeJSONResponse(w, products)
}

// GetProductByID returns a single product with all related data (full detail page).
func (h *ProductHandler) GetProductByID(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Invalid product ID", http.StatusBadRequest)
		return
	}
	ctx := r.Context()

	// --- base product row ---
	var p models.Product
	var origPrice sql.NullInt64
	var imageURL sql.NullString
	var discountPct sql.NullFloat64

	err = h.DB.QueryRowContext(ctx, `
		SELECT p.id, p.sku, p.name, p.slug,
		       p.brand_id, p.category_id, p.gender_id,
		       p.price_mkd, p.original_price_mkd, p.image_url,
		       p.is_new, p.is_featured, p.is_on_sale, p.discount_pct,
		       p.is_active, p.is_published, p.sort_order,
		       p.created_at, p.updated_at,
		       b.name, b.slug
		FROM products p
		JOIN brands b ON b.id = p.brand_id
		WHERE p.id = $1 AND p.is_active = TRUE
	`, id).Scan(
		&p.ID, &p.SKU, &p.Name, &p.Slug,
		&p.BrandID, &p.CategoryID, &p.GenderID,
		&p.PriceMKD, &origPrice, &imageURL,
		&p.IsNew, &p.IsFeatured, &p.IsOnSale, &discountPct,
		&p.IsActive, &p.IsPublished, &p.SortOrder,
		&p.CreatedAt, &p.UpdatedAt,
		&p.BrandName, &p.BrandSlug,
	)
	if err == sql.ErrNoRows {
		http.Error(w, "Product not found", http.StatusNotFound)
		return
	}
	if err != nil {
		h.Logger.Error("GetProductByID: query failed", err)
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

	// --- translations ---
	tRows, err := h.DB.QueryContext(ctx, `
		SELECT lang, COALESCE(description,''), COALESCE(about,''),
		       COALESCE(size_and_fit,''), COALESCE(product_info,''),
		       COALESCE(sustainability,''), COALESCE(meta_title,''),
		       COALESCE(meta_description,'')
		FROM product_translations
		WHERE product_id = $1
		ORDER BY lang
	`, id)
	if err != nil {
		h.Logger.Error("GetProductByID: translations query failed", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer tRows.Close()
	for tRows.Next() {
		var t models.ProductTranslation
		if err := tRows.Scan(&t.Lang, &t.Description, &t.About, &t.SizeAndFit,
			&t.ProductInfo, &t.Sustainability, &t.MetaTitle, &t.MetaDescription); err != nil {
			h.Logger.Error("GetProductByID: translation scan failed", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		p.Translations = append(p.Translations, t)
	}

	// --- colors ---
	cRows, err := h.DB.QueryContext(ctx, `
		SELECT color, hex FROM product_colors
		WHERE product_id = $1 ORDER BY color
	`, id)
	if err != nil {
		h.Logger.Error("GetProductByID: colors query failed", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer cRows.Close()
	for cRows.Next() {
		var color string
		var hex sql.NullString
		if err := cRows.Scan(&color, &hex); err != nil {
			h.Logger.Error("GetProductByID: color scan failed", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		c := models.ProductColor{Color: color}
		if hex.Valid {
			c.Hex = &hex.String
		}
		p.Colors = append(p.Colors, c)
	}

	// --- sizes ---
	sRows, err := h.DB.QueryContext(ctx, `
		SELECT eu_size FROM product_sizes
		WHERE product_id = $1 ORDER BY eu_size
	`, id)
	if err != nil {
		h.Logger.Error("GetProductByID: sizes query failed", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer sRows.Close()
	for sRows.Next() {
		var size float64
		if err := sRows.Scan(&size); err != nil {
			h.Logger.Error("GetProductByID: size scan failed", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		p.Sizes = append(p.Sizes, size)
	}

	// --- gallery ---
	gRows, err := h.DB.QueryContext(ctx, `
		SELECT image_url, sort_order FROM product_gallery
		WHERE product_id = $1 ORDER BY sort_order
	`, id)
	if err != nil {
		h.Logger.Error("GetProductByID: gallery query failed", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer gRows.Close()
	for gRows.Next() {
		var g models.ProductGallery
		if err := gRows.Scan(&g.ImageURL, &g.SortOrder); err != nil {
			h.Logger.Error("GetProductByID: gallery scan failed", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		p.Gallery = append(p.Gallery, g)
	}

	// --- specs ---
	spRows, err := h.DB.QueryContext(ctx, `
		SELECT key, value FROM product_specs
		WHERE product_id = $1 ORDER BY key
	`, id)
	if err != nil {
		h.Logger.Error("GetProductByID: specs query failed", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer spRows.Close()
	for spRows.Next() {
		var s models.ProductSpec
		if err := spRows.Scan(&s.Key, &s.Value); err != nil {
			h.Logger.Error("GetProductByID: spec scan failed", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		p.Specs = append(p.Specs, s)
	}

	// --- highlights ---
	hRows, err := h.DB.QueryContext(ctx, `
		SELECT sort_order, mk, sq, en FROM product_highlights
		WHERE product_id = $1 ORDER BY sort_order
	`, id)
	if err != nil {
		h.Logger.Error("GetProductByID: highlights query failed", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer hRows.Close()
	for hRows.Next() {
		var hl models.ProductHighlight
		if err := hRows.Scan(&hl.SortOrder, &hl.MK, &hl.SQ, &hl.EN); err != nil {
			h.Logger.Error("GetProductByID: highlight scan failed", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		p.Highlights = append(p.Highlights, hl)
	}

	// --- stock (JOIN sizes + colors to return eu_size + color name + qty) ---
	stRows, err := h.DB.QueryContext(ctx, `
		SELECT ps.eu_size, pc.color, pst.qty
		FROM product_stock pst
		JOIN product_sizes  ps ON ps.id = pst.size_id
		JOIN product_colors pc ON pc.id = pst.color_id
		WHERE pst.product_id = $1
		ORDER BY ps.eu_size, pc.color
	`, id)
	if err != nil {
		h.Logger.Error("GetProductByID: stock query failed", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer stRows.Close()
	for stRows.Next() {
		var s models.ProductStock
		if err := stRows.Scan(&s.EUSize, &s.Color, &s.Qty); err != nil {
			h.Logger.Error("GetProductByID: stock scan failed", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		p.Stock = append(p.Stock, s)
	}

	// --- size chart ---
	scRows, err := h.DB.QueryContext(ctx, `
		SELECT eu_size, foot_length_mm, foot_width_mm
		FROM size_chart
		WHERE product_id = $1 ORDER BY eu_size
	`, id)
	if err != nil {
		h.Logger.Error("GetProductByID: size chart query failed", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer scRows.Close()
	for scRows.Next() {
		var sc models.SizeChartEntry
		var lenMM, widMM sql.NullFloat64
		if err := scRows.Scan(&sc.EUSize, &lenMM, &widMM); err != nil {
			h.Logger.Error("GetProductByID: size chart scan failed", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		if lenMM.Valid {
			sc.FootLengthMM = &lenMM.Float64
		}
		if widMM.Valid {
			sc.FootWidthMM = &widMM.Float64
		}
		p.SizeChart = append(p.SizeChart, sc)
	}

	// --- reviews ---
	rRows, err := h.DB.QueryContext(ctx, `
		SELECT id, user_id, rating, body, created_at
		FROM reviews
		WHERE product_id = $1 ORDER BY created_at DESC
	`, id)
	if err != nil {
		h.Logger.Error("GetProductByID: reviews query failed", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer rRows.Close()
	for rRows.Next() {
		var rev models.Review
		var body sql.NullString
		if err := rRows.Scan(&rev.ID, &rev.UserID, &rev.Rating, &body, &rev.CreatedAt); err != nil {
			h.Logger.Error("GetProductByID: review scan failed", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		rev.ProductID = id
		if body.Valid {
			rev.Body = &body.String
		}
		p.Reviews = append(p.Reviews, rev)
	}

	h.writeJSONResponse(w, p)
}
