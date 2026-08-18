// Command synctorder pushes ONLY the products.sort_order column from the
// data/freet/products/*.json files to the DB — the safe way to change display
// order without cmd/shoeimport, which rebuilds ALL product_stock from JSON and
// would wipe live inventory. Touches nothing but sort_order.
//
// DEFAULT IS DRY-RUN: prints old→new per product inside a rolled-back tx.
// Pass -commit to apply.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"bosfoot/internal/database"
)

const brandSlug = "freet"

func main() {
	commit := flag.Bool("commit", false, "apply the change (default: dry-run)")
	flag.Parse()

	db, err := database.Connect()
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	var brandID int
	if err := db.QueryRow(`SELECT id FROM brands WHERE slug=$1`, brandSlug).Scan(&brandID); err != nil {
		log.Fatalf("brand %q not found: %v", brandSlug, err)
	}

	files, _ := filepath.Glob(filepath.Join("data", brandSlug, "products", "*.json"))
	if len(files) == 0 {
		log.Fatal("no product files under data/" + brandSlug + "/products/")
	}

	tx, err := db.Begin()
	if err != nil {
		log.Fatal(err)
	}
	defer tx.Rollback()

	mode := "DRY-RUN (nothing written)"
	if *commit {
		mode = "COMMIT"
	}
	fmt.Printf("synctorder — %s — brand %s\n\n", mode, brandSlug)

	changed := 0
	for _, path := range files {
		slug := strings.TrimSuffix(filepath.Base(path), ".json")
		raw, err := os.ReadFile(path)
		if err != nil {
			log.Fatal(err)
		}
		var p struct {
			SortOrder int `json:"sortOrder"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			log.Fatalf("%s: %v", slug, err)
		}

		var cur int
		if err := tx.QueryRow(`SELECT sort_order FROM products WHERE brand_id=$1 AND slug=$2`,
			brandID, slug).Scan(&cur); err != nil {
			log.Fatalf("product %q: %v", slug, err)
		}
		if cur == p.SortOrder {
			continue
		}
		if _, err := tx.Exec(`UPDATE products SET sort_order=$1, updated_at=now() WHERE brand_id=$2 AND slug=$3`,
			p.SortOrder, brandID, slug); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("  %-14s %d → %d\n", slug, cur, p.SortOrder)
		changed++
	}

	if *commit {
		if err := tx.Commit(); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("\nDone. %d product(s) reordered.\n", changed)
	} else {
		fmt.Printf("\nDry-run only (rolled back). %d product(s) would change. Re-run with -commit to apply.\n", changed)
	}
}
