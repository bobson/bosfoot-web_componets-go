package models

import "time"

// Brand maps to the brands table.
type Brand struct {
	ID              int       `json:"id"`
	Name            string    `json:"name"`
	SKU             string    `json:"sku"`
	Slug            string    `json:"slug"`
	CountryOfOrigin *string   `json:"country_of_origin,omitempty"`
	YearFounded     *int      `json:"year_founded,omitempty"`
	WebsiteURL      *string   `json:"website_url,omitempty"`
	SizingGuideURL  *string   `json:"sizing_guide_url,omitempty"`
	LogoURL         *string   `json:"logo_url,omitempty"`
	IsFeatured      bool      `json:"is_featured"`
	SortOrder       int       `json:"sort_order"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`

	// Joined fields
	Translations []BrandTranslation `json:"translations,omitempty"`
	Products     []Product          `json:"products,omitempty"`
}

type BrandTranslation struct {
	Lang        string `json:"lang"`
	Description string `json:"description,omitempty"`
}
