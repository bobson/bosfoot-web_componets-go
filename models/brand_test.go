package models

import (
	"testing"
	"time"
)

func TestBrand(t *testing.T) {
	now := time.Now()
	
	// Create pointers for optional fields
	country := "UK"
	year := 2010
	
	brand := Brand{
		ID:              1,
		Name:            "Test Brand",
		SKU:             "TB-001",
		Slug:            "test-brand",
		CountryOfOrigin: &country,
		YearFounded:     &year,
		IsFeatured:      true,
		SortOrder:       1,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if brand.Name != "Test Brand" {
		t.Errorf("Expected Name 'Test Brand', got %s", brand.Name)
	}

	if brand.CountryOfOrigin == nil || *brand.CountryOfOrigin != "UK" {
		t.Errorf("Expected CountryOfOrigin 'UK', got %v", brand.CountryOfOrigin)
	}
}
