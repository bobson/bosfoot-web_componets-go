package models

import "time"

// Product maps to the products table plus common joined data returned by the API.
type Product struct {
	ID               int       `json:"id"`
	SKU              string    `json:"sku"`
	Name             string    `json:"name"`
	Slug             string    `json:"slug"`
	BrandID          int       `json:"brand_id"`
	CategoryID       int       `json:"category_id"`
	GenderID         int       `json:"gender_id"`
	PriceMKD         int       `json:"price_mkd"`
	OriginalPriceMKD *int      `json:"original_price_mkd,omitempty"`
	ImageURL         *string   `json:"image_url,omitempty"`
	IsNew            bool      `json:"is_new"`
	IsFeatured       bool      `json:"is_featured"`
	IsOnSale         bool      `json:"is_on_sale"`
	DiscountPct      *float64  `json:"discount_pct,omitempty"`
	IsActive         bool      `json:"is_active"`
	IsPublished      bool      `json:"is_published"`
	SortOrder        int       `json:"sort_order"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`

	// Denormalised brand fields — populated on every product response so the
	// frontend doesn't need a second request to resolve the brand name/slug.
	BrandName   string `json:"brand_name,omitempty"`
	BrandSlug   string `json:"brand_slug,omitempty"`
	GenderValue string `json:"gender_value,omitempty"`

	// Joined fields — populated when the API fetches a full product.
	Translations []ProductTranslation `json:"translations,omitempty"`
	Sizes        []float64            `json:"sizes,omitempty"`
	Colors       []ProductColor       `json:"colors,omitempty"`
	Activities   []string             `json:"activities,omitempty"`
	Gallery      []ProductGallery     `json:"gallery,omitempty"`
	Specs        []ProductSpec        `json:"specs,omitempty"`
	Highlights   []ProductHighlight   `json:"highlights,omitempty"`
	Stock        []ProductStock       `json:"stock,omitempty"`
	SizeChart    []SizeChartEntry     `json:"size_chart,omitempty"`
	Reviews      []Review             `json:"reviews,omitempty"`

	// InStock is true when at least one (size, colour) variant has qty > 0.
	// Populated for SSR pages (listing + detail) to decide whether a product is
	// buyable or shows the "get notified when available" state. When false the
	// product is displayed but not orderable.
	InStock bool `json:"in_stock"`
}

type ProductTranslation struct {
	Lang            string `json:"lang"`
	Description     string `json:"description,omitempty"`
	About           string `json:"about,omitempty"`
	SizeAndFit      string `json:"size_and_fit,omitempty"`
	ProductInfo     string `json:"product_info,omitempty"`
	Sustainability  string `json:"sustainability,omitempty"`
	MetaTitle       string `json:"meta_title,omitempty"`
	MetaDescription string `json:"meta_description,omitempty"`
}

type ProductColor struct {
	Color string  `json:"color"`
	Hex   *string `json:"hex,omitempty"`
}

type ProductGallery struct {
	ImageURL  string `json:"image_url"`
	SortOrder int    `json:"sort_order"`
}

type ProductSpec struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type ProductHighlight struct {
	SortOrder int    `json:"sort_order"`
	MK        string `json:"mk"`
	SQ        string `json:"sq"`
	EN        string `json:"en"`
}

// ProductStock is a flat entry from a JOIN across product_stock, product_sizes,
// and product_colors — easier to work with in handlers and templates than the
// raw three-table join.
type ProductStock struct {
	EUSize float64 `json:"eu_size"`
	Color  string  `json:"color"`
	Qty    int     `json:"qty"`
}

type SizeChartEntry struct {
	EUSize       float64  `json:"eu_size"`
	InsoleLengthMM *float64 `json:"insole_length_mm,omitempty"`
	InsoleWidthMM  *float64 `json:"insole_width_mm,omitempty"`
}
