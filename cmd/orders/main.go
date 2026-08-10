// Command orders prints all orders to the terminal, newest first, with contact
// details and line items — a quick read of who ordered what — and moves an order
// through its lifecycle (pending → shipped → delivered, or cancelled).
//
//	go run ./cmd/orders                  # all orders, newest first
//	go run ./cmd/orders -status pending  # your to-ship queue
//	go run ./cmd/orders -status delivered# past (completed) orders
//	go run ./cmd/orders -limit 10        # only the N most recent
//	go run ./cmd/orders -ship BF-1064    # mark order shipped
//	go run ./cmd/orders -deliver BF-1064 # mark order delivered (skips shipped if you like)
//	go run ./cmd/orders -cancel BF-1064  # mark order cancelled
//
// Transitions are unconditional: you can jump straight to delivered without ever
// marking shipped. For supplier/contact CSV exports use cmd/reservations instead.
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"bosfoot/internal/database"
	"bosfoot/internal/site"
)

// itemSep is an unlikely delimiter used to string_agg line items into one column
// so the whole listing is a single query (no per-order N+1).
const itemSep = "\x1f"

func main() {
	status := flag.String("status", "", "filter by order status (pending|shipped|delivered|cancelled)")
	limit := flag.Int("limit", 0, "show only the N most recent orders (0 = all)")
	ship := flag.String("ship", "", "mark this order (e.g. BF-1064) as shipped")
	deliver := flag.String("deliver", "", "mark this order (e.g. BF-1064) as delivered")
	cancel := flag.String("cancel", "", "mark this order (e.g. BF-1064) as cancelled")
	flag.Parse()

	db, err := database.Connect()
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	switch {
	case *ship != "":
		setStatus(db, *ship, "shipped")
		return
	case *deliver != "":
		setStatus(db, *deliver, "delivered")
		return
	case *cancel != "":
		setStatus(db, *cancel, "cancelled")
		return
	}

	// $1 = status filter ('' means no filter); $2 = limit (0 means no limit).
	rows, err := db.Query(`
		SELECT o.id, o.created_at,
		       o.first_name || ' ' || o.last_name AS name,
		       o.email, COALESCE(o.phone, '') AS phone, o.city,
		       o.payment_method::text, o.status::text, o.total_mkd, o.locale::text,
		       COALESCE(o.notes, '') AS notes,
		       string_agg(
		         COALESCE(p.name, '?') || itemcols.variant ||
		         '  x' || oi.quantity || '  @ ' || oi.unit_price_mkd || ' MKD',
		         $3 ORDER BY oi.id
		       ) AS items
		FROM orders o
		JOIN order_items oi ON oi.order_id = o.id
		LEFT JOIN products p ON p.id = oi.product_id
		CROSS JOIN LATERAL (
		  SELECT CASE
		           WHEN COALESCE(NULLIF(oi.size, ''), '') = '' AND COALESCE(NULLIF(oi.color, ''), '') = ''
		             THEN ''
		           ELSE '  ' || NULLIF(oi.size, '') ||
		                CASE WHEN NULLIF(oi.size, '') IS NOT NULL AND NULLIF(oi.color, '') IS NOT NULL
		                     THEN ' / ' ELSE '' END ||
		                NULLIF(oi.color, '')
		         END AS variant
		) itemcols
		WHERE ($1 = '' OR o.status::text = $1)
		GROUP BY o.id
		ORDER BY o.created_at DESC
		LIMIT NULLIF($2, 0)
	`, *status, *limit, itemSep)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	count, grand := 0, 0
	for rows.Next() {
		var id, totalMKD int
		var createdAt time.Time
		var name, email, phone, city, pay, st, locale, notes, items string
		if err := rows.Scan(&id, &createdAt, &name, &email, &phone, &city,
			&pay, &st, &totalMKD, &locale, &notes, &items); err != nil {
			log.Fatal(err)
		}
		if count == 0 {
			fmt.Println()
		}
		count++
		grand += totalMKD

		fmt.Printf("%s  %s  [%s]  %d MKD  (%s, %s)\n",
			site.OrderNumber(id), createdAt.Format("2006-01-02 15:04"), st, totalMKD, pay, locale)
		fmt.Printf("    %s  <%s>  %s\n", name, email, phone)
		fmt.Printf("    %s\n", city)
		if notes != "" {
			fmt.Printf("    notes: %s\n", notes)
		}
		for _, it := range strings.Split(items, itemSep) {
			fmt.Printf("      - %s\n", it)
		}
		fmt.Println()
	}
	if err := rows.Err(); err != nil {
		log.Fatal(err)
	}

	if count == 0 {
		if *status != "" {
			fmt.Printf("No orders with status %q.\n", *status)
		} else {
			fmt.Println("No orders yet.")
		}
		return
	}
	fmt.Printf("%d order(s)", count)
	if *status != "" {
		fmt.Printf(" with status %q", *status)
	}
	fmt.Printf("  ·  %d MKD total\n", grand)
}

// setStatus moves one order (given by its BF- number) to the given status and
// reports the result. The transition is unconditional — any status can be set
// from any status — so you can go straight to delivered without ever marking
// shipped.
func setStatus(db *sql.DB, orderNum, status string) {
	id, err := site.ParseOrderNumber(orderNum)
	if err != nil {
		fmt.Println(err)
		return
	}
	res, err := db.Exec(`UPDATE orders SET status = $1 WHERE id = $2`, status, id)
	if err != nil {
		log.Fatal(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		fmt.Printf("No order %s.\n", site.OrderNumber(id))
		return
	}
	fmt.Printf("Order %s marked %s.\n", site.OrderNumber(id), status)
}
