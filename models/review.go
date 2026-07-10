package models

import (
	"errors"
	"strings"
	"time"
)

// Review maps to the reviews table. Reviews are guest-checkout friendly: they
// are tied to an order (OrderID) via a review token rather than a logged-in
// user. Fit is an optional -2 (runs small) … 0 (true to size) … +2 (runs large)
// signal. Status gates visibility (pending → approved/rejected).
type Review struct {
	ID         int       `json:"id"`
	ProductID  int       `json:"product_id"`
	OrderID    *int      `json:"order_id,omitempty"`
	Rating     int       `json:"rating"`        // 1–5, required
	Fit        *int      `json:"fit,omitempty"` // -2..2, optional
	AuthorName string    `json:"author_name"`
	Body       *string   `json:"body,omitempty"`
	Lang       string    `json:"lang"` // mk | sq | en
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
}

// Validate checks a submitted review. Rating is required (1–5); fit, when given,
// must be in range; a display name is required; the body is optional but capped.
func (r *Review) Validate() error {
	if r.Rating < 1 || r.Rating > 5 {
		return errors.New("rating must be between 1 and 5")
	}
	if r.Fit != nil && (*r.Fit < -2 || *r.Fit > 2) {
		return errors.New("invalid fit value")
	}
	if strings.TrimSpace(r.AuthorName) == "" {
		return errors.New("name is required")
	}
	if len([]rune(r.AuthorName)) > 80 {
		return errors.New("name is too long")
	}
	if r.Body != nil && len([]rune(*r.Body)) > 2000 {
		return errors.New("review is too long")
	}
	return nil
}

// ReviewToken maps to the review_tokens table. The token is a random string
// emailed to the buyer after fulfilment; presenting it proves the purchase and
// binds a submitted review to a specific (order, product). UsedAt is set once
// the review is submitted, making the token single-use.
type ReviewToken struct {
	Token     string     `json:"token"`
	OrderID   int        `json:"order_id"`
	ProductID int        `json:"product_id"`
	Email     string     `json:"email"`
	Lang      string     `json:"lang"`
	UsedAt    *time.Time `json:"used_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}
