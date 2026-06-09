package models

import "time"

// User maps to the users table.
// PasswordHash is intentionally excluded — it must never be sent in API responses.
type User struct {
	ID        int       `json:"id"`
	Email     string    `json:"email"`
	Name      *string   `json:"name,omitempty"`
	Address   *string   `json:"address,omitempty"`
	Lang      string    `json:"lang"` // mk | sq | en
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
