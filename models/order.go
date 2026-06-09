package models

import "time"

// Order maps to the orders table. UserID is nil for guest checkouts.
type Order struct {
	ID            int       `json:"id"`
	UserID        *int      `json:"user_id,omitempty"`
	Email         string    `json:"email"`
	Phone         *string   `json:"phone,omitempty"`
	FirstName     string    `json:"first_name"`
	LastName      string    `json:"last_name"`
	Address       string    `json:"address"`
	City          string    `json:"city"`
	PostalCode    *string   `json:"postal_code,omitempty"`
	Country       string    `json:"country"` // ISO2: MK, AL, XK, RS, BG, GR
	Notes         *string   `json:"notes,omitempty"`
	PaymentMethod string    `json:"payment_method"` // cod | bank_transfer
	Status        string    `json:"status"`         // pending | shipped | delivered
	TotalMKD      int       `json:"total_mkd"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`

	Items []OrderItem `json:"items,omitempty"`
}

// OrderItem maps to the order_items table. Size, Color, and price are
// snapshotted at order time so historical orders stay accurate if the
// product changes later.
type OrderItem struct {
	ID           int     `json:"id"`
	OrderID      int     `json:"order_id"`
	ProductID    int     `json:"product_id"`
	Size         *string `json:"size,omitempty"`
	Color        *string `json:"color,omitempty"`
	Quantity     int     `json:"quantity"`
	UnitPriceMKD int     `json:"unit_price_mkd"`
}
