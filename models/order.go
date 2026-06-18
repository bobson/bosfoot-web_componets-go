package models

import (
	"errors"
	"net/mail"
	"strings"
	"time"
)

// Order maps to the orders table. UserID is nil for guest checkouts.
type Order struct {
// ... existing fields ...
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
	Locale        string    `json:"locale"` // mk | sq | en
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`

	Items []OrderItem `json:"items,omitempty"`
}

// Validate checks if the order data is structurally valid.
//
// Reservation mode (pre-launch): a reservation only needs a way to reach the
// customer, so we require name + phone and treat email/address/city as
// optional (email is validated only when supplied). To restore full checkout at
// launch, bring back the mandatory email and address/city checks and drop the
// phone requirement.
func (o *Order) Validate() error {
	if o.FirstName == "" {
		return errors.New("name is required")
	}
	if o.Phone == nil || strings.TrimSpace(*o.Phone) == "" {
		return errors.New("phone is required")
	}
	if o.Email != "" {
		if _, err := mail.ParseAddress(o.Email); err != nil {
			return errors.New("invalid email address")
		}
	}
	if o.PaymentMethod != "cod" && o.PaymentMethod != "bank_transfer" {
		return errors.New("invalid payment method")
	}
	if len(o.Items) == 0 {
		return errors.New("order must contain at least one item")
	}
	return nil
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
