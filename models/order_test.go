package models

import (
	"testing"
	"time"
)

func TestOrder(t *testing.T) {
	now := time.Now()
	
	order := Order{
		ID:            1,
		Email:         "customer@example.com",
		FirstName:     "John",
		LastName:      "Doe",
		Address:       "Main St 1",
		City:          "Skopje",
		Country:       "MK",
		PaymentMethod: "cod",
		Status:        "pending",
		TotalMKD:      5000,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if order.Email != "customer@example.com" {
		t.Errorf("Expected Email 'customer@example.com', got %s", order.Email)
	}

	if order.Status != "pending" {
		t.Errorf("Expected Status 'pending', got %s", order.Status)
	}
}
