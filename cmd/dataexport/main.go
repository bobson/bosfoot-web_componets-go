// Throwaway bootstrap: read each Freet product from the DB (the current source
// of truth) + the euro price from its shoe.json, and write the complete
// data/freet/products/{slug}.json source files. Run once to seed data/, verify,
// then delete. The real importer (cmd/shoeimport) reads these back later.
//
//	go run ./cmd/dataexport
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"bosfoot/internal/database"
)

type statusOut struct {
	Active    bool `json:"active"`
	Published bool `json:"published"`
	Featured  bool `json:"featured"`
}

type colorOut struct {
	Name string `json:"name"`
	Hex  string `json:"hex"`
}

type specOut struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type productOut struct {
	Name         string                       `json:"name"`
	Price        int                          `json:"price"` // EUR
	Gender       string                       `json:"gender"`
	Status       statusOut                    `json:"status"`
	SortOrder    int                          `json:"sortOrder"`
	Activities   []string                     `json:"activities"`
	Highlights   []map[string]string          `json:"highlights"`
	Colors       []colorOut                   `json:"colors"`
	Stock        map[string]map[string]int    `json:"stock"` // color -> size -> qty
	Specs        []specOut                    `json:"specs"`
	Translations map[string]map[string]string `json:"translations"`
}

func fmtSize(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }

func euroPrice(slug string) int {
	raw, err := os.ReadFile(filepath.Join("public/images/freet", slug, "shoe.json"))
	if err != nil {
		log.Fatalf("read shoe.json for %s: %v", slug, err)
	}
	var s struct {
		Price int `json:"price"`
	}
	if err := json.Unmarshal(raw, &s); err != nil {
		log.Fatalf("parse shoe.json for %s: %v", slug, err)
	}
	return s.Price
}

func main() {
	db, err := database.Connect()
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	rows, err := db.Query(`
		SELECT p.id, p.sku, p.name, p.slug, g.value,
		       p.is_active, p.is_published, p.is_featured, p.sort_order
		FROM products p JOIN genders g ON g.id = p.gender_id
		WHERE p.sku LIKE 'FREET-%' ORDER BY p.sort_order`)
	if err != nil {
		log.Fatal(err)
	}
	type prod struct {
		id                              int
		sku, name, slug, gender         string
		active, published, featured     bool
		sortOrder                       int
	}
	var prods []prod
	for rows.Next() {
		var p prod
		if err := rows.Scan(&p.id, &p.sku, &p.name, &p.slug, &p.gender,
			&p.active, &p.published, &p.featured, &p.sortOrder); err != nil {
			log.Fatal(err)
		}
		prods = append(prods, p)
	}
	rows.Close()

	if err := os.MkdirAll("data/freet/products", 0o755); err != nil {
		log.Fatal(err)
	}

	for _, p := range prods {
		out := productOut{
			Name:         p.name,
			Price:        euroPrice(p.slug),
			Gender:       p.gender,
			Status:       statusOut{p.active, p.published, p.featured},
			SortOrder:    p.sortOrder,
			Stock:        map[string]map[string]int{},
			Translations: map[string]map[string]string{},
		}

		// activities
		ar, _ := db.Query(`SELECT activity FROM product_activities WHERE product_id=$1 ORDER BY activity`, p.id)
		for ar.Next() {
			var a string
			ar.Scan(&a)
			out.Activities = append(out.Activities, a)
		}
		ar.Close()

		// highlights
		hr, _ := db.Query(`SELECT mk, sq, en FROM product_highlights WHERE product_id=$1 ORDER BY sort_order`, p.id)
		for hr.Next() {
			var mk, sq, en string
			hr.Scan(&mk, &sq, &en)
			out.Highlights = append(out.Highlights, map[string]string{"mk": mk, "sq": sq, "en": en})
		}
		hr.Close()

		// colors
		cr, _ := db.Query(`SELECT color, COALESCE(hex,'') FROM product_colors WHERE product_id=$1 ORDER BY id`, p.id)
		for cr.Next() {
			var c colorOut
			cr.Scan(&c.Name, &c.Hex)
			out.Colors = append(out.Colors, c)
		}
		cr.Close()

		// stock: color -> size -> qty
		sr, _ := db.Query(`
			SELECT c.color, ps.eu_size, st.qty
			FROM product_stock st
			JOIN product_sizes ps ON ps.id = st.size_id
			JOIN product_colors c ON c.id = st.color_id
			WHERE st.product_id=$1 ORDER BY c.color, ps.eu_size`, p.id)
		for sr.Next() {
			var color string
			var size float64
			var qty int
			sr.Scan(&color, &size, &qty)
			if out.Stock[color] == nil {
				out.Stock[color] = map[string]int{}
			}
			out.Stock[color][fmtSize(size)] = qty
		}
		sr.Close()

		// specs
		spr, _ := db.Query(`SELECT key, value FROM product_specs WHERE product_id=$1 ORDER BY id`, p.id)
		for spr.Next() {
			var k, v string
			spr.Scan(&k, &v)
			out.Specs = append(out.Specs, specOut{k, v})
		}
		spr.Close()

		// translations
		tr, _ := db.Query(`
			SELECT lang, COALESCE(description,''), COALESCE(about,''),
			       COALESCE(size_and_fit,''), COALESCE(product_info,''), COALESCE(sustainability,'')
			FROM product_translations WHERE product_id=$1`, p.id)
		for tr.Next() {
			var lang, desc, about, fit, info, sust string
			tr.Scan(&lang, &desc, &about, &fit, &info, &sust)
			out.Translations[lang] = map[string]string{
				"description": desc, "about": about, "sizeAndFit": fit,
				"productInfo": info, "sustainability": sust,
			}
		}
		tr.Close()

		buf, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			log.Fatal(err)
		}
		path := filepath.Join("data/freet/products", p.slug+".json")
		if err := os.WriteFile(path, append(buf, '\n'), 0o644); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("  wrote %s (€%d, %d colors, %d specs)\n", path, out.Price, len(out.Colors), len(out.Specs))
	}
	fmt.Printf("done: %d products exported to data/freet/products/\n", len(prods))
}
