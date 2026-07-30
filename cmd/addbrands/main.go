package main

import (
	"bosfoot/internal/database"
	"fmt"
	"os"
)

func main() {
	db, err := database.Connect()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	sql := `
-- Coming-soon brands: Be Lenka + Groundies. Remove the previously seeded
-- Xero Shoes / Vivobarefoot (cascade drops their translations). The DELETE
-- fails safely by FK if a removed brand ever gained products.
DELETE FROM brands WHERE slug IN ('xero-shoes', 'vivobarefoot');

INSERT INTO brands (name, sku, slug, is_featured, sort_order) VALUES
  ('Be Lenka',  'be-lenka',  'be-lenka',  FALSE, 9),
  ('Groundies', 'groundies', 'groundies', FALSE, 10)
ON CONFLICT (slug) DO NOTHING;

-- Add translations for Be Lenka
INSERT INTO brand_translations (brand_id, lang, description)
VALUES
  ((SELECT id FROM brands WHERE slug = 'be-lenka'), 'mk', 'Барефут бренд од Словачка што спојува природно движење со секојдневен стил.'),
  ((SELECT id FROM brands WHERE slug = 'be-lenka'), 'sq', 'Markë barefoot nga Sllovakia që ndërthur lëvizjen natyrale me stilin e përditshëm.'),
  ((SELECT id FROM brands WHERE slug = 'be-lenka'), 'en', 'Slovak barefoot brand blending natural movement with everyday style.')
ON CONFLICT (brand_id, lang) DO NOTHING;

-- Add translations for Groundies
INSERT INTO brand_translations (brand_id, lang, description)
VALUES
  ((SELECT id FROM brands WHERE slug = 'groundies'), 'mk', 'Прави се за боси обувки со минимално минешување.'),
  ((SELECT id FROM brands WHERE slug = 'groundies'), 'sq', 'Specjalizuar në këpucë barefoot me dizajn minimaliste.'),
  ((SELECT id FROM brands WHERE slug = 'groundies'), 'en', 'Specializing in minimalist barefoot shoes with exceptional ground feel.')
ON CONFLICT (brand_id, lang) DO NOTHING;
`

	_, err = db.Exec(sql)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to insert: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Coming-soon brands updated (Be Lenka + Groundies; removed Xero, Vivobarefoot)")

	// Show the resulting brand list + product counts (0 = coming soon on the site).
	rows, err := db.Query(`
		SELECT b.name, COUNT(p.id)
		FROM brands b
		LEFT JOIN products p ON p.brand_id = b.id AND p.is_active AND p.is_published
		GROUP BY b.id, b.name
		ORDER BY b.is_featured DESC, b.sort_order ASC, b.name ASC`)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to list brands: %v\n", err)
		os.Exit(1)
	}
	defer rows.Close()
	fmt.Println("\nBrands now:")
	for rows.Next() {
		var name string
		var count int
		if err := rows.Scan(&name, &count); err != nil {
			fmt.Fprintf(os.Stderr, "scan: %v\n", err)
			os.Exit(1)
		}
		status := fmt.Sprintf("%d products", count)
		if count == 0 {
			status = "coming soon"
		}
		fmt.Printf("  %-14s %s\n", name, status)
	}
}
