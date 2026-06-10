package handlers

import (
	"bosfoot/internal/locale"
	"bosfoot/internal/tmpl"
	"bosfoot/logger"
	"bosfoot/models"
	"database/sql"
	"net/http"
	"sort"
	"strings"

	"github.com/lib/pq"
	"golang.org/x/sync/errgroup"
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
	Products         []models.Product
	Count            int
	FilterBrands     []FilterBrand
	FilterGenders    []FilterGender
	FilterActivities []string
	FilterSizes      []float64
	FilterColors     []FilterColor
}

type FilterBrand struct {
	Slug string
	Name string
}

type FilterGender struct {
	Value     string // DB value, used in data-gender attr
	LocaleKey string // existing locale key for display
}

type FilterColor struct {
	Name string
	Hex  string
}

// genderLocaleKey maps DB gender values to existing locale keys.
var genderLocaleKey = map[string]string{
	"male":   "gender.mens",
	"female": "gender.womens",
	"unisex": "gender.unisex",
	"kids":   "gender.kids",
}

// HomePageData is the template data for /{locale}.
type HomePageData struct {
	PageBase
	Brands   []models.Brand
	Featured []models.Product
}

// ProductDetailData is the template data for /{locale}/products/{brand}/{slug}.
type ProductDetailData struct {
	PageBase
	Product     models.Product
	Translation models.ProductTranslation
	Related     []models.Product
}

// Home handles GET /mk, /sq, /en — the homepage.
func (h *PageHandler) Home(w http.ResponseWriter, r *http.Request) {
	seg := strings.TrimPrefix(r.URL.Path, "/")
	loc := locale.FromPath(seg)
	ctx := r.Context()

	// Brands
	brandRows, err := h.DB.QueryContext(ctx, `
		SELECT id, name, slug, logo_url, is_featured
		FROM brands
		ORDER BY is_featured DESC, sort_order ASC, name ASC
	`)
	if err != nil {
		h.Logger.Error("Home: brands query failed", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer brandRows.Close()

	var brands []models.Brand
	for brandRows.Next() {
		var b models.Brand
		var logo sql.NullString
		if err := brandRows.Scan(&b.ID, &b.Name, &b.Slug, &logo, &b.IsFeatured); err != nil {
			h.Logger.Error("Home: brands scan failed", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		if logo.Valid {
			b.LogoURL = &logo.String
		}
		brands = append(brands, b)
	}

	// Featured products
	featRows, err := h.DB.QueryContext(ctx, `
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
		LIMIT 3
	`)
	if err != nil {
		h.Logger.Error("Home: featured query failed", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer featRows.Close()

	var featured []models.Product
	var ids []int
	for featRows.Next() {
		var p models.Product
		var origPrice sql.NullInt64
		var imageURL sql.NullString
		var discountPct sql.NullFloat64
		if err := featRows.Scan(
			&p.ID, &p.SKU, &p.Name, &p.Slug,
			&p.BrandID, &p.CategoryID, &p.GenderID,
			&p.PriceMKD, &origPrice, &imageURL,
			&p.IsNew, &p.IsFeatured, &p.IsOnSale, &discountPct,
			&p.IsActive, &p.IsPublished, &p.SortOrder,
			&p.CreatedAt, &p.UpdatedAt,
			&p.BrandName, &p.BrandSlug,
		); err != nil {
			h.Logger.Error("Home: featured scan failed", err)
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
		featured = append(featured, p)
		ids = append(ids, p.ID)
	}

	// Colors for featured products
	if len(ids) > 0 {
		colorRows, err := h.DB.QueryContext(ctx, `
			SELECT product_id, color, hex
			FROM product_colors
			WHERE product_id = ANY($1)
			ORDER BY product_id, color
		`, pq.Array(ids))
		if err != nil {
			h.Logger.Error("Home: colors query failed", err)
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
				h.Logger.Error("Home: color scan failed", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			c := models.ProductColor{Color: color}
			if hex.Valid {
				c.Hex = &hex.String
			}
			colorMap[productID] = append(colorMap[productID], c)
		}
		for i := range featured {
			featured[i].Colors = colorMap[featured[i].ID]
		}
	}

	h.Renderer.Render(w, "home", HomePageData{
		PageBase: PageBase{
			Locale:      loc,
			CurrentPath: "",
			SiteURL:     h.SiteURL,
		},
		Brands:   brands,
		Featured: featured,
	})
}

// ProductListing handles GET /{locale}/products.
func (h *PageHandler) ProductListing(w http.ResponseWriter, r *http.Request) {
	loc := locale.FromPath(r.PathValue("locale"))
	if !locale.IsValid(r.PathValue("locale")) {
		http.Redirect(w, r, "/"+locale.Default+"/products", http.StatusFound)
		return
	}
	ctx := r.Context()

	// Fetch published products with brand and gender joined.
	rows, err := h.DB.QueryContext(ctx, `
		SELECT p.id, p.sku, p.name, p.slug,
		       p.brand_id, p.category_id, p.gender_id,
		       p.price_mkd, p.original_price_mkd, p.image_url,
		       p.is_new, p.is_featured, p.is_on_sale, p.discount_pct,
		       p.is_active, p.is_published, p.sort_order,
		       p.created_at, p.updated_at,
		       b.name, b.slug, g.value
		FROM products p
		JOIN brands b ON b.id = p.brand_id
		JOIN genders g ON g.id = p.gender_id
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
			&p.BrandName, &p.BrandSlug, &p.GenderValue,
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

		// Fetch activities for all products.
		actRows, err := h.DB.QueryContext(ctx, `
			SELECT product_id, activity
			FROM product_activities
			WHERE product_id = ANY($1)
			ORDER BY product_id, activity
		`, pq.Array(ids))
		if err != nil {
			h.Logger.Error("ProductListing: activities query failed", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		defer actRows.Close()

		actMap := make(map[int][]string)
		for actRows.Next() {
			var productID int
			var activity string
			if err := actRows.Scan(&productID, &activity); err != nil {
				h.Logger.Error("ProductListing: activity scan failed", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			actMap[productID] = append(actMap[productID], activity)
		}
		for i := range products {
			products[i].Activities = actMap[products[i].ID]
		}

		// Fetch available sizes for all products.
		sizeRows, err := h.DB.QueryContext(ctx, `
			SELECT product_id, eu_size
			FROM product_sizes
			WHERE product_id = ANY($1)
			ORDER BY product_id, eu_size
		`, pq.Array(ids))
		if err != nil {
			h.Logger.Error("ProductListing: sizes query failed", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		defer sizeRows.Close()

		sizeMap := make(map[int][]float64)
		for sizeRows.Next() {
			var productID int
			var euSize float64
			if err := sizeRows.Scan(&productID, &euSize); err != nil {
				h.Logger.Error("ProductListing: size scan failed", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			sizeMap[productID] = append(sizeMap[productID], euSize)
		}
		for i := range products {
			products[i].Sizes = sizeMap[products[i].ID]
		}
	}

	// Collect unique filter options from all products.
	brandSeen, genderSeen, actSeen, sizeSeen, colorSeen := map[string]bool{}, map[string]bool{}, map[string]bool{}, map[float64]bool{}, map[string]bool{}
	var filterBrands []FilterBrand
	var filterGenders []FilterGender
	var filterActivities []string
	var filterSizes []float64
	var filterColors []FilterColor

	for _, p := range products {
		if !brandSeen[p.BrandSlug] {
			brandSeen[p.BrandSlug] = true
			filterBrands = append(filterBrands, FilterBrand{Slug: p.BrandSlug, Name: p.BrandName})
		}
		if !genderSeen[p.GenderValue] {
			genderSeen[p.GenderValue] = true
			key, ok := genderLocaleKey[p.GenderValue]
			if !ok {
				key = "gender." + p.GenderValue
			}
			filterGenders = append(filterGenders, FilterGender{Value: p.GenderValue, LocaleKey: key})
		}
		for _, a := range p.Activities {
			if !actSeen[a] {
				actSeen[a] = true
				filterActivities = append(filterActivities, a)
			}
		}
		for _, s := range p.Sizes {
			if !sizeSeen[s] {
				sizeSeen[s] = true
				filterSizes = append(filterSizes, s)
			}
		}
		for _, c := range p.Colors {
			lname := strings.ToLower(c.Color)
			if !colorSeen[lname] {
				colorSeen[lname] = true
				hex := ""
				if c.Hex != nil {
					hex = *c.Hex
				}
				filterColors = append(filterColors, FilterColor{Name: c.Color, Hex: hex})
			}
		}
	}

	sort.Strings(filterActivities)
	sort.Float64s(filterSizes)
	sort.Slice(filterColors, func(i, j int) bool { return filterColors[i].Name < filterColors[j].Name })

	h.Renderer.Render(w, "products", ProductListingData{
		PageBase: PageBase{
			Locale:      loc,
			CurrentPath: "/products",
			SiteURL:     h.SiteURL,
		},
		Products:         products,
		Count:            len(products),
		FilterBrands:     filterBrands,
		FilterGenders:    filterGenders,
		FilterActivities: filterActivities,
		FilterSizes:      filterSizes,
		FilterColors:     filterColors,
	})
}

// ProductDetail handles GET /{locale}/products/{brand}/{slug}.
func (h *PageHandler) ProductDetail(w http.ResponseWriter, r *http.Request) {
	if !locale.IsValid(r.PathValue("locale")) {
		http.Redirect(w, r, "/"+locale.Default+"/products", http.StatusFound)
		return
	}
	loc := locale.FromPath(r.PathValue("locale"))
	brandSlug := r.PathValue("brand")
	productSlug := r.PathValue("slug")
	ctx := r.Context()

	// Base product row.
	var p models.Product
	var origPrice sql.NullInt64
	var imageURL sql.NullString
	var discountPct sql.NullFloat64

	err := h.DB.QueryRowContext(ctx, `
		SELECT p.id, p.sku, p.name, p.slug,
		       p.brand_id, p.category_id, p.gender_id,
		       p.price_mkd, p.original_price_mkd, p.image_url,
		       p.is_new, p.is_featured, p.is_on_sale, p.discount_pct,
		       p.is_active, p.is_published, p.sort_order,
		       p.created_at, p.updated_at,
		       b.name, b.slug, g.value
		FROM products p
		JOIN brands b ON b.id = p.brand_id
		JOIN genders g ON g.id = p.gender_id
		WHERE b.slug = $1 AND p.slug = $2 AND p.is_active = TRUE
	`, brandSlug, productSlug).Scan(
		&p.ID, &p.SKU, &p.Name, &p.Slug,
		&p.BrandID, &p.CategoryID, &p.GenderID,
		&p.PriceMKD, &origPrice, &imageURL,
		&p.IsNew, &p.IsFeatured, &p.IsOnSale, &discountPct,
		&p.IsActive, &p.IsPublished, &p.SortOrder,
		&p.CreatedAt, &p.UpdatedAt,
		&p.BrandName, &p.BrandSlug, &p.GenderValue,
	)
	if err == sql.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		h.Logger.Error("ProductDetail: query failed", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if origPrice.Valid  { v := int(origPrice.Int64); p.OriginalPriceMKD = &v }
	if imageURL.Valid   { p.ImageURL = &imageURL.String }
	if discountPct.Valid { p.DiscountPct = &discountPct.Float64 }

	// All the related data is fetched concurrently. These queries are
	// independent — each only needs p.ID (or p.BrandID) — so running them in
	// parallel collapses N sequential round-trips to Aiven into roughly one.
	// On a remote DB that network latency is the dominant cost. Each goroutine
	// writes a distinct field of p (or the separate `related` slice), so there
	// is no shared-memory race. errgroup cancels siblings on the first error.
	var related []models.Product
	g, gctx := errgroup.WithContext(ctx)
	// Cap concurrent queries per request so one product page can't claim the
	// whole connection pool (max 8 on this Aiven plan). 4 still cuts the ~9
	// queries into a couple of waves — most of the speedup, half the pool.
	g.SetLimit(4)

	// Translations.
	g.Go(func() error {
		rows, err := h.DB.QueryContext(gctx, `
			SELECT lang, COALESCE(description,''), COALESCE(about,''),
			       COALESCE(size_and_fit,''), COALESCE(product_info,''),
			       COALESCE(sustainability,''), COALESCE(meta_title,''),
			       COALESCE(meta_description,'')
			FROM product_translations WHERE product_id = $1 ORDER BY lang
		`, p.ID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var t models.ProductTranslation
			if err := rows.Scan(&t.Lang, &t.Description, &t.About, &t.SizeAndFit,
				&t.ProductInfo, &t.Sustainability, &t.MetaTitle, &t.MetaDescription); err != nil {
				return err
			}
			p.Translations = append(p.Translations, t)
		}
		return rows.Err()
	})

	// Colors.
	g.Go(func() error {
		rows, err := h.DB.QueryContext(gctx, `
			SELECT color, hex FROM product_colors WHERE product_id = $1 ORDER BY color
		`, p.ID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var color string
			var hex sql.NullString
			if err := rows.Scan(&color, &hex); err != nil {
				return err
			}
			c := models.ProductColor{Color: color}
			if hex.Valid {
				c.Hex = &hex.String
			}
			p.Colors = append(p.Colors, c)
		}
		return rows.Err()
	})

	// Sizes.
	g.Go(func() error {
		rows, err := h.DB.QueryContext(gctx, `
			SELECT eu_size FROM product_sizes WHERE product_id = $1 ORDER BY eu_size
		`, p.ID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var s float64
			if err := rows.Scan(&s); err != nil {
				return err
			}
			p.Sizes = append(p.Sizes, s)
		}
		return rows.Err()
	})

	// Gallery.
	g.Go(func() error {
		rows, err := h.DB.QueryContext(gctx, `
			SELECT image_url, sort_order FROM product_gallery WHERE product_id = $1 ORDER BY sort_order
		`, p.ID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var gal models.ProductGallery
			if err := rows.Scan(&gal.ImageURL, &gal.SortOrder); err != nil {
				return err
			}
			p.Gallery = append(p.Gallery, gal)
		}
		return rows.Err()
	})

	// Specs.
	g.Go(func() error {
		rows, err := h.DB.QueryContext(gctx, `
			SELECT key, value FROM product_specs WHERE product_id = $1 ORDER BY key
		`, p.ID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var s models.ProductSpec
			if err := rows.Scan(&s.Key, &s.Value); err != nil {
				return err
			}
			p.Specs = append(p.Specs, s)
		}
		return rows.Err()
	})

	// Highlights.
	g.Go(func() error {
		rows, err := h.DB.QueryContext(gctx, `
			SELECT sort_order, mk, sq, en FROM product_highlights WHERE product_id = $1 ORDER BY sort_order
		`, p.ID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var hl models.ProductHighlight
			if err := rows.Scan(&hl.SortOrder, &hl.MK, &hl.SQ, &hl.EN); err != nil {
				return err
			}
			p.Highlights = append(p.Highlights, hl)
		}
		return rows.Err()
	})

	// Stock.
	g.Go(func() error {
		rows, err := h.DB.QueryContext(gctx, `
			SELECT ps.eu_size, pc.color, pst.qty
			FROM product_stock pst
			JOIN product_sizes  ps ON ps.id = pst.size_id
			JOIN product_colors pc ON pc.id = pst.color_id
			WHERE pst.product_id = $1
			ORDER BY ps.eu_size, pc.color
		`, p.ID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var s models.ProductStock
			if err := rows.Scan(&s.EUSize, &s.Color, &s.Qty); err != nil {
				return err
			}
			p.Stock = append(p.Stock, s)
		}
		return rows.Err()
	})

	// Size chart.
	g.Go(func() error {
		rows, err := h.DB.QueryContext(gctx, `
			SELECT eu_size, foot_length_mm, foot_width_mm FROM size_chart WHERE product_id = $1 ORDER BY eu_size
		`, p.ID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var sc models.SizeChartEntry
			var lenMM, widMM sql.NullFloat64
			if err := rows.Scan(&sc.EUSize, &lenMM, &widMM); err != nil {
				return err
			}
			if lenMM.Valid {
				sc.FootLengthMM = &lenMM.Float64
			}
			if widMM.Valid {
				sc.FootWidthMM = &widMM.Float64
			}
			p.SizeChart = append(p.SizeChart, sc)
		}
		return rows.Err()
	})

	// Related products from the same brand, plus their colors. These two are
	// sequential with each other (colors need the related IDs) but run in
	// parallel with everything above.
	g.Go(func() error {
		rows, err := h.DB.QueryContext(gctx, `
			SELECT p.id, p.sku, p.name, p.slug,
			       p.brand_id, p.category_id, p.gender_id,
			       p.price_mkd, p.original_price_mkd, p.image_url,
			       p.is_new, p.is_featured, p.is_on_sale, p.discount_pct,
			       p.is_active, p.is_published, p.sort_order,
			       p.created_at, p.updated_at,
			       b.name, b.slug, g.value
			FROM products p
			JOIN brands b ON b.id = p.brand_id
			JOIN genders g ON g.id = p.gender_id
			WHERE p.brand_id = $1 AND p.id != $2
			  AND p.is_active = TRUE AND p.is_published = TRUE
			ORDER BY p.sort_order LIMIT 4
		`, p.BrandID, p.ID)
		if err != nil {
			return err
		}
		defer rows.Close()

		var relIDs []int
		for rows.Next() {
			var rp models.Product
			var rOrigPrice sql.NullInt64
			var rImageURL sql.NullString
			var rDiscountPct sql.NullFloat64
			if err := rows.Scan(
				&rp.ID, &rp.SKU, &rp.Name, &rp.Slug,
				&rp.BrandID, &rp.CategoryID, &rp.GenderID,
				&rp.PriceMKD, &rOrigPrice, &rImageURL,
				&rp.IsNew, &rp.IsFeatured, &rp.IsOnSale, &rDiscountPct,
				&rp.IsActive, &rp.IsPublished, &rp.SortOrder,
				&rp.CreatedAt, &rp.UpdatedAt,
				&rp.BrandName, &rp.BrandSlug, &rp.GenderValue,
			); err != nil {
				return err
			}
			if rOrigPrice.Valid {
				v := int(rOrigPrice.Int64)
				rp.OriginalPriceMKD = &v
			}
			if rImageURL.Valid {
				rp.ImageURL = &rImageURL.String
			}
			if rDiscountPct.Valid {
				rp.DiscountPct = &rDiscountPct.Float64
			}
			related = append(related, rp)
			relIDs = append(relIDs, rp.ID)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		if len(relIDs) == 0 {
			return nil
		}

		// Colors for related products.
		crows, err := h.DB.QueryContext(gctx, `
			SELECT product_id, color, hex FROM product_colors
			WHERE product_id = ANY($1) ORDER BY product_id, color
		`, pq.Array(relIDs))
		if err != nil {
			return err
		}
		defer crows.Close()
		rcMap := make(map[int][]models.ProductColor)
		for crows.Next() {
			var pid int
			var color string
			var hex sql.NullString
			if err := crows.Scan(&pid, &color, &hex); err != nil {
				return err
			}
			c := models.ProductColor{Color: color}
			if hex.Valid {
				c.Hex = &hex.String
			}
			rcMap[pid] = append(rcMap[pid], c)
		}
		if err := crows.Err(); err != nil {
			return err
		}
		for i := range related {
			related[i].Colors = rcMap[related[i].ID]
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		h.Logger.Error("ProductDetail: data fetch failed", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Find locale-specific translation; fall back to mk.
	var translation models.ProductTranslation
	for _, t := range p.Translations {
		if t.Lang == loc { translation = t; break }
	}
	if translation.Lang == "" {
		for _, t := range p.Translations {
			if t.Lang == "mk" { translation = t; break }
		}
	}

	metaDesc := translation.MetaDescription
	if metaDesc == "" { metaDesc = translation.Description }

	currentPath := "/products/" + brandSlug + "/" + productSlug
	h.Renderer.Render(w, "product", ProductDetailData{
		PageBase: PageBase{
			Locale:          loc,
			CurrentPath:     currentPath,
			SiteURL:         h.SiteURL,
			MetaDescription: metaDesc,
		},
		Product:     p,
		Translation: translation,
		Related:     related,
	})
}
