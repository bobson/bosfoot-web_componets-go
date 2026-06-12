package models

import "time"

// Article maps to the articles table. Slug is the EN canonical URL slug;
// translated slugs live in ArticleTranslation.
type Article struct {
	ID          int        `json:"id"`
	Slug        string     `json:"slug"`
	IsPublished bool       `json:"is_published"`
	IsFeatured  bool       `json:"is_featured"`
	Author      *string    `json:"author,omitempty"`
	PublishedAt *time.Time `json:"published_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`

	Translations []ArticleTranslation `json:"translations,omitempty"`
}

type ArticleTranslation struct {
	Lang            string  `json:"lang"`
	Title           string  `json:"title"`
	Lead            *string `json:"lead,omitempty"`
	Body            *string `json:"body,omitempty"` // JSON array of ArticleBlock
	Slug            string  `json:"slug"`
	MetaTitle       *string `json:"meta_title,omitempty"`
	MetaDescription *string `json:"meta_description,omitempty"`
}

// ArticleBlock is one rendered block of an article body. The body column stores
// a JSON array of these; the page handler unmarshals it for the template.
// Style is "normal" (paragraph), "h2", or "h3".
type ArticleBlock struct {
	Style string `json:"style"`
	Text  string `json:"text"`
}
