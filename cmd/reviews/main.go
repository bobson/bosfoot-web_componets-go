// Command reviews lists and moderates product reviews from the terminal — the
// approval path for the guest-checkout review system (reviews land 'pending' and
// show on the site only once approved). Saves writing raw SQL.
//
//	go run ./cmd/reviews                  # list pending reviews (newest first)
//	go run ./cmd/reviews -status approved # list a different status
//	go run ./cmd/reviews -approve 12      # approve review #12
//	go run ./cmd/reviews -reject 12       # reject review #12
//
// Approved reviews appear on the product page within the 60s page-cache TTL.
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"bosfoot/internal/database"
)

func main() {
	status := flag.String("status", "pending", "list reviews with this status (pending|approved|rejected)")
	approve := flag.Int("approve", 0, "approve the review with this id")
	reject := flag.Int("reject", 0, "reject the review with this id")
	flag.Parse()

	db, err := database.Connect()
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	switch {
	case *approve != 0:
		setStatus(db, *approve, "approved")
		return
	case *reject != 0:
		setStatus(db, *reject, "rejected")
		return
	}

	rows, err := db.Query(`
		SELECT r.id, b.name || ' ' || p.name AS product,
		       r.rating, r.fit, r.author_name, COALESCE(r.body, ''),
		       r.lang_code::text, r.created_at, COALESCE(r.order_id, 0)
		FROM reviews r
		JOIN products p ON p.id = r.product_id
		JOIN brands   b ON b.id = p.brand_id
		WHERE r.status = $1
		ORDER BY r.created_at DESC
	`, *status)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var id, rating, orderID int
		var fit sql.NullInt64
		var product, author, body, lang string
		var createdAt time.Time
		if err := rows.Scan(&id, &product, &rating, &fit, &author, &body, &lang, &createdAt, &orderID); err != nil {
			log.Fatal(err)
		}
		if count == 0 {
			fmt.Println()
		}
		count++

		fmt.Printf("#%d  %s  %s  (%s, order #%d, %s)\n",
			id, stars(rating), product, lang, orderID, createdAt.Format("2006-01-02 15:04"))
		fmt.Printf("    %s", author)
		if fit.Valid {
			fmt.Printf("  ·  fit: %s", fitLabel(int(fit.Int64)))
		}
		fmt.Println()
		if body != "" {
			fmt.Printf("    %s\n", body)
		}
		fmt.Println()
	}
	if err := rows.Err(); err != nil {
		log.Fatal(err)
	}

	if count == 0 {
		fmt.Printf("No %s reviews.\n", *status)
		return
	}
	fmt.Printf("%d %s review(s)", count, *status)
	if *status == "pending" {
		fmt.Print("  ·  approve with:  go run ./cmd/reviews -approve <id>")
	}
	fmt.Println()
}

// setStatus flips one review's status and reports the result.
func setStatus(db *sql.DB, id int, status string) {
	res, err := db.Exec(`UPDATE reviews SET status = $1 WHERE id = $2`, status, id)
	if err != nil {
		log.Fatal(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		fmt.Printf("No review with id %d.\n", id)
		return
	}
	fmt.Printf("Review #%d %s.\n", id, status)
}

// stars renders a 1–5 rating as filled/empty stars.
func stars(n int) string {
	if n < 0 {
		n = 0
	}
	if n > 5 {
		n = 5
	}
	return strings.Repeat("★", n) + strings.Repeat("☆", 5-n)
}

// fitLabel maps the -2..2 fit signal to a readable label.
func fitLabel(v int) string {
	switch {
	case v < 0:
		return "runs small"
	case v > 0:
		return "runs large"
	default:
		return "true to size"
	}
}
