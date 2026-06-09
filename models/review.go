package models

import "time"

// Review maps to the reviews table.
type Review struct {
	ID        int       `json:"id"`
	ProductID int       `json:"product_id"`
	UserID    int       `json:"user_id"`
	Rating    int       `json:"rating"` // 1–5
	Body      *string   `json:"body,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
