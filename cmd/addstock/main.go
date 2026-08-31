// Command addstock adds inventory to existing product variants WITHOUT touching
// anything else — the safe, incremental counterpart to cmd/shoeimport (which
// DELETEs and re-inserts product_stock from JSON, wiping live sale decrements).
// Use this for restocks once the shop is live and selling.
//
//	go run ./cmd/addstock -product vibe-2 -color Black -add "43:2,45:1,47:1"
//	go run ./cmd/addstock -product vibe-2 -color Black -add "43:2" -commit
//
// DEFAULT IS DRY-RUN: it prints old→new qty per variant inside a rolled-back
// transaction and writes nothing. Pass -commit to apply. It only ever does
// qty = qty + delta on the targeted (product, colour, size) rows; every other
// product and variant is left alone. A negative delta is allowed (for
// corrections) but rejected if it would drive a variant below 0.
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"strconv"
	"strings"

	"bosfoot/internal/database"
)

func main() {
	brand := flag.String("brand", "freet", "brand slug")
	product := flag.String("product", "", "product slug (e.g. vibe-2)")
	color := flag.String("color", "", "colour name exactly as stored (e.g. Black)")
	add := flag.String("add", "", `size:qty pairs to add, e.g. "43:2,45:1,47:1" (qty may be negative)`)
	commit := flag.Bool("commit", false, "apply the change (default: dry-run)")
	flag.Parse()

	if *product == "" || *color == "" || *add == "" {
		log.Fatal(`usage: addstock -product <slug> -color <name> -add "43:2,45:1" [-commit]`)
	}

	deltas, err := parseAdd(*add)
	if err != nil {
		log.Fatal(err)
	}

	db, err := database.Connect() // loads .env
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Resolve the product and colour up front, with clear errors.
	var productID, colorID int
	switch err := db.QueryRow(`
		SELECT p.id FROM products p JOIN brands b ON b.id = p.brand_id
		WHERE b.slug = $1 AND p.slug = $2`, *brand, *product).Scan(&productID); err {
	case nil:
	case sql.ErrNoRows:
		log.Fatalf("no product %q for brand %q", *product, *brand)
	default:
		log.Fatal(err)
	}
	switch err := db.QueryRow(`SELECT id FROM product_colors WHERE product_id = $1 AND color = $2`,
		productID, *color).Scan(&colorID); err {
	case nil:
	case sql.ErrNoRows:
		log.Fatalf("product %q has no colour %q (check the exact name)", *product, *color)
	default:
		log.Fatal(err)
	}

	mode := "DRY-RUN (nothing written)"
	if *commit {
		mode = "COMMIT"
	}
	fmt.Printf("addstock — %s — %s / %s\n\n", mode, *product, *color)

	// One transaction: dry-run rolls it back, commit applies it atomically.
	tx, err := db.Begin()
	if err != nil {
		log.Fatal(err)
	}
	defer tx.Rollback() // no-op after Commit

	net := 0
	for _, d := range deltas {
		var sizeID int
		switch err := tx.QueryRow(`SELECT id FROM product_sizes WHERE product_id = $1 AND eu_size = $2`,
			productID, d.size).Scan(&sizeID); err {
		case nil:
		case sql.ErrNoRows:
			log.Fatalf("product %q has no size %s", *product, d.sizeStr)
		default:
			log.Fatal(err)
		}

		// Current qty — 0 (and no row) if this variant has no stock row yet.
		var cur int
		exists := true
		switch err := tx.QueryRow(`
			SELECT qty FROM product_stock WHERE product_id=$1 AND size_id=$2 AND color_id=$3`,
			productID, sizeID, colorID).Scan(&cur); err {
		case nil:
		case sql.ErrNoRows:
			exists = false
		default:
			log.Fatal(err)
		}

		next := cur + d.qty
		if next < 0 {
			log.Fatalf("size %s: %d + (%d) = %d would be negative — refusing", d.sizeStr, cur, d.qty, next)
		}

		// Bump an existing row, or create it at the delta. We branch instead of
		// using INSERT … ON CONFLICT because Postgres validates the qty>=0 CHECK
		// on the *speculative* insert tuple (the raw delta) BEFORE resolving the
		// conflict to DO UPDATE — so a negative delta on an existing row would
		// wrongly fail even though the final value is >= 0. A plain UPDATE only
		// checks the final row; a fresh-row INSERT only ever carries a non-negative
		// delta here (cur=0, so the next<0 guard above already caught negatives).
		if exists {
			if _, err := tx.Exec(`
				UPDATE product_stock SET qty = qty + $4
				WHERE product_id=$1 AND size_id=$2 AND color_id=$3`,
				productID, sizeID, colorID, d.qty); err != nil {
				log.Fatal(err)
			}
		} else {
			if _, err := tx.Exec(`
				INSERT INTO product_stock (product_id, size_id, color_id, qty) VALUES ($1,$2,$3,$4)`,
				productID, sizeID, colorID, d.qty); err != nil {
				log.Fatal(err)
			}
		}

		fmt.Printf("  size %-4s  %d → %d  (%+d)\n", d.sizeStr, cur, next, d.qty)
		net += d.qty
	}

	if *commit {
		if err := tx.Commit(); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("\nDone. Net %+d across %d size(s).\n", net, len(deltas))
	} else {
		fmt.Printf("\nDry-run only (rolled back). Net %+d across %d size(s). Re-run with -commit to apply.\n",
			net, len(deltas))
	}
}

type delta struct {
	size    float64 // eu_size numeric value
	sizeStr string  // as typed, for messages
	qty     int     // amount to add (may be negative)
}

// parseAdd turns "43:2,45:1,47:1" into per-size deltas.
func parseAdd(s string) ([]delta, error) {
	var out []delta
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, ":", 2)
		if len(kv) != 2 {
			return nil, fmt.Errorf("bad -add entry %q (want size:qty, e.g. 43:2)", part)
		}
		sz := strings.TrimSpace(kv[0])
		f, err := strconv.ParseFloat(sz, 64)
		if err != nil {
			return nil, fmt.Errorf("bad size %q: %w", sz, err)
		}
		q, err := strconv.Atoi(strings.TrimSpace(kv[1]))
		if err != nil {
			return nil, fmt.Errorf("bad qty in %q: %w", part, err)
		}
		if q == 0 {
			return nil, fmt.Errorf("qty 0 in %q — nothing to add", part)
		}
		out = append(out, delta{size: f, sizeStr: sz, qty: q})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no size:qty pairs in -add")
	}
	return out, nil
}
