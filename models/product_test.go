package models

import (
	"testing"
	"time"
)

func TestProduct(t *testing.T) {
	now := time.Now()
	price := 5000
	
	product := Product{
		ID:          1,
		SKU:         "PROD-001",
		Name:        "Barefoot Shoe",
		Slug:        "barefoot-shoe",
		BrandID:     1,
		PriceMKD:    price,
		IsActive:    true,
		IsPublished: true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if product.Name != "Barefoot Shoe" {
		t.Errorf("Expected Name 'Barefoot Shoe', got %s", product.Name)
	}

	if product.PriceMKD != 5000 {
		t.Errorf("Expected PriceMKD 5000, got %d", product.PriceMKD)
	}
}

func TestProductStock(t *testing.T) {
	stock := ProductStock{
		EUSize: 42.0,
		Color:  "Black",
		Qty:    5,
	}

	if stock.EUSize != 42.0 {
		t.Errorf("Expected EUSize 42.0, got %f", stock.EUSize)
	}
}
