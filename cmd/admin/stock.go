package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/lib/pq"
)

// Stock lives per (product, size, colour) in product_stock — the DB is the
// source of truth (orders decrement it; shoeimport keeps it unless -stock).
// The admin edits it directly; the JSON files are not touched.

type stockColor struct {
	ID        int
	Name, Hex string
	Qty       []int // per size, same order as stockProduct.Sizes
	Total     int
}

type stockProduct struct {
	ID                     int
	Brand, BrandSlug, Name string
	Slug, Mode             string
	Published              bool
	Sizes                  []stockSize
	Colors                 []stockColor
	Total                  int
}

type stockSize struct {
	ID    int
	Label string // "42", "40.5"
}

func (a *app) stock(w http.ResponseWriter, r *http.Request) {
	brand := r.URL.Query().Get("brand")
	showHidden := r.URL.Query().Get("hidden") == "1"
	prods, brands, err := a.loadStock(brand, showHidden)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	a.render(w, "stock.html", a.page("stock", map[string]any{
		"Products": prods, "Brands": brands, "Brand": brand, "Hidden": showHidden,
	}))
}

func (a *app) loadStock(brand string, showHidden bool) ([]stockProduct, []string, error) {
	var brands []string
	brows, err := a.db.Query(`SELECT DISTINCT b.slug FROM brands b JOIN products p ON p.brand_id = b.id ORDER BY 1`)
	if err != nil {
		return nil, nil, err
	}
	for brows.Next() {
		var s string
		if brows.Scan(&s) == nil {
			brands = append(brands, s)
		}
	}
	brows.Close()

	rows, err := a.db.Query(`
		SELECT p.id, b.name, b.slug, p.name, p.slug, p.fulfilment, p.is_published
		FROM products p JOIN brands b ON b.id = p.brand_id
		WHERE p.is_active AND ($1 = '' OR b.slug = $1) AND ($2 OR p.is_published)
		ORDER BY b.name, p.sort_order, p.name`, brand, showHidden)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var prods []stockProduct
	idx := map[int]int{}
	var ids []int64
	for rows.Next() {
		var p stockProduct
		if err := rows.Scan(&p.ID, &p.Brand, &p.BrandSlug, &p.Name, &p.Slug, &p.Mode, &p.Published); err != nil {
			return nil, nil, err
		}
		idx[p.ID] = len(prods)
		ids = append(ids, int64(p.ID))
		prods = append(prods, p)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	// Sizes, then colours, then the stock cells for all listed products.
	srows, err := a.db.Query(`SELECT product_id, id, eu_size::text FROM product_sizes
		WHERE product_id = ANY($1) ORDER BY product_id, eu_size`, pq.Array(ids))
	if err != nil {
		return nil, nil, err
	}
	sizePos := map[int]int{} // size_id → column
	for srows.Next() {
		var pid int
		var s stockSize
		if srows.Scan(&pid, &s.ID, &s.Label) == nil {
			s.Label = strings.TrimSuffix(strings.TrimSuffix(s.Label, "0"), ".")
			p := &prods[idx[pid]]
			sizePos[s.ID] = len(p.Sizes)
			p.Sizes = append(p.Sizes, s)
		}
	}
	srows.Close()

	crows, err := a.db.Query(`SELECT product_id, id, color, COALESCE(hex, '') FROM product_colors
		WHERE product_id = ANY($1) ORDER BY product_id, id`, pq.Array(ids))
	if err != nil {
		return nil, nil, err
	}
	colorPos := map[int][2]int{} // color_id → (product index, colour row)
	for crows.Next() {
		var pid int
		var c stockColor
		if crows.Scan(&pid, &c.ID, &c.Name, &c.Hex) == nil {
			p := &prods[idx[pid]]
			c.Qty = make([]int, len(p.Sizes))
			colorPos[c.ID] = [2]int{idx[pid], len(p.Colors)}
			p.Colors = append(p.Colors, c)
		}
	}
	crows.Close()

	qrows, err := a.db.Query(`SELECT size_id, color_id, qty FROM product_stock WHERE product_id = ANY($1)`, pq.Array(ids))
	if err != nil {
		return nil, nil, err
	}
	defer qrows.Close()
	for qrows.Next() {
		var sid, cid, q int
		if qrows.Scan(&sid, &cid, &q) != nil {
			continue
		}
		cp, ok := colorPos[cid]
		col, ok2 := sizePos[sid]
		if !ok || !ok2 {
			continue
		}
		c := &prods[cp[0]].Colors[cp[1]]
		if col < len(c.Qty) {
			c.Qty[col] = q
			c.Total += q
			prods[cp[0]].Total += q
		}
	}
	return prods, brands, qrows.Err()
}

// stockSet changes one cell: delta=+1/-1, or set=N. Creates the row if it was
// missing; never goes below 0. Answers JSON {qty} for the page's fetch().
func (a *app) stockSet(w http.ResponseWriter, r *http.Request) {
	pid, e1 := strconv.Atoi(r.FormValue("product"))
	sid, e2 := strconv.Atoi(r.FormValue("size"))
	cid, e3 := strconv.Atoi(r.FormValue("color"))
	if e1 != nil || e2 != nil || e3 != nil {
		jsonErr(w, "bad cell")
		return
	}
	var qty int
	var err error
	if set := r.FormValue("set"); set != "" {
		n, convErr := strconv.Atoi(set)
		if convErr != nil || n < 0 || n > 999 {
			jsonErr(w, "enter a number 0–999")
			return
		}
		err = a.db.QueryRow(`
			INSERT INTO product_stock (product_id, size_id, color_id, qty)
			SELECT $1, s.id, c.id, $4 FROM product_sizes s, product_colors c
			WHERE s.id = $2 AND s.product_id = $1 AND c.id = $3 AND c.product_id = $1
			ON CONFLICT (product_id, size_id, color_id) DO UPDATE SET qty = EXCLUDED.qty
			RETURNING qty`, pid, sid, cid, n).Scan(&qty)
	} else {
		d, convErr := strconv.Atoi(r.FormValue("delta"))
		if convErr != nil || (d != 1 && d != -1) {
			jsonErr(w, "bad delta")
			return
		}
		err = a.db.QueryRow(`
			INSERT INTO product_stock (product_id, size_id, color_id, qty)
			SELECT $1, s.id, c.id, GREATEST($4, 0) FROM product_sizes s, product_colors c
			WHERE s.id = $2 AND s.product_id = $1 AND c.id = $3 AND c.product_id = $1
			ON CONFLICT (product_id, size_id, color_id)
			DO UPDATE SET qty = GREATEST(product_stock.qty + $4, 0)
			RETURNING qty`, pid, sid, cid, d).Scan(&qty)
	}
	if err != nil {
		jsonErr(w, "not saved: "+err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]int{"qty": qty})
}

func jsonErr(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
