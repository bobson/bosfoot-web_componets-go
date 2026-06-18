package models

import (
	"strings"
	"testing"
)

// validOrder returns an order that passes Validate(); each test case mutates a
// single field so exactly one rule trips, and asserts which one.
//
// Reservation mode: name + phone are required; email/address/city are optional.
func validOrder() Order {
	phone := "070123456"
	return Order{
		Email:         "customer@example.com",
		Phone:         &phone,
		FirstName:     "John",
		LastName:      "Doe",
		Address:       "Main St 1",
		City:          "Skopje",
		Country:       "MK",
		PaymentMethod: "cod",
		Items:         []OrderItem{{ProductID: 1, Quantity: 1, UnitPriceMKD: 5000}},
	}
}

func TestOrderValidate(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(*Order)
		errSubstr string // "" → expect no error
	}{
		{"valid cod", func(o *Order) {}, ""},
		{"valid bank_transfer", func(o *Order) { o.PaymentMethod = "bank_transfer" }, ""},
		{"empty email is allowed", func(o *Order) { o.Email = "" }, ""},
		{"empty address/city allowed", func(o *Order) { o.Address = ""; o.City = ""; o.LastName = "" }, ""},
		{"malformed email", func(o *Order) { o.Email = "not-an-email" }, "email"},
		{"missing name", func(o *Order) { o.FirstName = "" }, "name"},
		{"missing phone", func(o *Order) { o.Phone = nil }, "phone"},
		{"blank phone", func(o *Order) { blank := "  "; o.Phone = &blank }, "phone"},
		{"invalid payment method", func(o *Order) { o.PaymentMethod = "paypal" }, "payment method"},
		{"empty payment method", func(o *Order) { o.PaymentMethod = "" }, "payment method"},
		{"no items", func(o *Order) { o.Items = nil }, "item"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := validOrder()
			tt.mutate(&o)
			err := o.Validate()

			if tt.errSubstr == "" {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.errSubstr)
			}
			if !strings.Contains(err.Error(), tt.errSubstr) {
				t.Errorf("expected error containing %q, got %q", tt.errSubstr, err.Error())
			}
		})
	}
}
