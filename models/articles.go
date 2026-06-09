package models

import "time"

// Article maps to the articles table. Slug is the EN canonical URL slug;
// translated slugs live in ArticleTranslation.
type Article struct {
	ID          int       `json:"id"`
	Slug        string    `json:"slug"`
	IsPublished bool      `json:"is_published"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	Translations []ArticleTranslation `json:"translations,omitempty"`
}

type ArticleTranslation struct {
	Lang            string  `json:"lang"`
	Title           string  `json:"title"`
	Body            *string `json:"body,omitempty"`
	Slug            string  `json:"slug"`
	MetaTitle       *string `json:"meta_title,omitempty"`
	MetaDescription *string `json:"meta_description,omitempty"`
}
