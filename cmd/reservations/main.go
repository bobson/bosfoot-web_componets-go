// Command reservations exports the current reservations (orders) so you can see
// exactly what to order from suppliers and who to contact.
//
//	go run ./cmd/reservations
//
// It writes two CSV files in the current directory:
//   - reservations_to_order.csv — total quantity reserved per product/size/color
//   - reservations_contacts.csv — one row per reservation with contact details
//
// and prints a readable "to order" summary to the terminal.
package main

import (
	"database/sql"
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"bosfoot/internal/database"
)

func main() {
	db, err := database.Connect()
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := exportToOrder(db); err != nil {
		log.Fatal(err)
	}
	if err := exportContacts(db); err != nil {
		log.Fatal(err)
	}
}

// exportToOrder aggregates reserved quantities per product/size/color — the list
// you place with the supplier.
func exportToOrder(db *sql.DB) error {
	rows, err := db.Query(`
		SELECT p.sku, p.name,
		       COALESCE(NULLIF(oi.size,  ''), '-') AS size,
		       COALESCE(NULLIF(oi.color, ''), '-') AS color,
		       SUM(oi.quantity)::int AS qty
		FROM order_items oi
		JOIN products p ON p.id = oi.product_id
		GROUP BY p.sku, p.name, oi.size, oi.color
		ORDER BY p.name, size, color
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	f, err := os.Create("reservations_to_order.csv")
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()
	if err := w.Write([]string{"sku", "product", "size", "color", "qty"}); err != nil {
		return err
	}

	fmt.Println("\n── TO ORDER (reserved quantities) ──────────────────────────────")
	fmt.Printf("%-18s %-22s %-6s %-12s %4s\n", "SKU", "PRODUCT", "SIZE", "COLOR", "QTY")
	total := 0
	for rows.Next() {
		var sku, name, size, color string
		var qty int
		if err := rows.Scan(&sku, &name, &size, &color, &qty); err != nil {
			return err
		}
		total += qty
		fmt.Printf("%-18s %-22s %-6s %-12s %4d\n", sku, trunc(name, 22), size, color, qty)
		if err := w.Write([]string{sku, name, size, color, strconv.Itoa(qty)}); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	fmt.Printf("%-18s %-22s %-6s %-12s %4d\n", "", "", "", "TOTAL", total)
	fmt.Println("→ wrote reservations_to_order.csv")
	return nil
}

// exportContacts lists each reservation with contact details and the items
// reserved — the list you use to reach out to customers.
func exportContacts(db *sql.DB) error {
	rows, err := db.Query(`
		SELECT o.id, o.created_at,
		       o.first_name || ' ' || o.last_name AS name,
		       o.email, COALESCE(o.phone, '') AS phone, o.city,
		       o.total_mkd, o.status::text,
		       string_agg(
		         p.name || ' [' || COALESCE(NULLIF(oi.size, ''), '-') || '/' ||
		         COALESCE(NULLIF(oi.color, ''), '-') || '] x' || oi.quantity, '; '
		         ORDER BY p.name
		       ) AS items
		FROM orders o
		JOIN order_items oi ON oi.order_id = o.id
		JOIN products p ON p.id = oi.product_id
		GROUP BY o.id
		ORDER BY o.created_at DESC
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	f, err := os.Create("reservations_contacts.csv")
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()
	if err := w.Write([]string{"id", "date", "name", "email", "phone", "city", "total_mkd", "status", "items"}); err != nil {
		return err
	}

	count := 0
	for rows.Next() {
		var id, totalMKD int
		var createdAt time.Time
		var name, email, phone, city, status, items string
		if err := rows.Scan(&id, &createdAt, &name, &email, &phone, &city, &totalMKD, &status, &items); err != nil {
			return err
		}
		count++
		if err := w.Write([]string{
			strconv.Itoa(id),
			createdAt.Format("2006-01-02 15:04"),
			name, email, phone, city,
			strconv.Itoa(totalMKD), status, items,
		}); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	fmt.Printf("\n%d reservation(s) total\n", count)
	fmt.Println("→ wrote reservations_contacts.csv")
	return nil
}

// trunc shortens s to n runes for tidy terminal columns (CSV keeps the full value).
func trunc(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
