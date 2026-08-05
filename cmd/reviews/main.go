// Command reviews lists and moderates product reviews from the terminal — the
// approval path for the guest-checkout review system (reviews land 'pending' and
// show on the site only once approved). Saves writing raw SQL.
//
//	go run ./cmd/reviews                  # list pending reviews (newest first)
//	go run ./cmd/reviews -status approved # list a different status
//	go run ./cmd/reviews -approve 12      # approve review #12
//	go run ./cmd/reviews -reject 12       # reject review #12 (hides it, keeps the row)
//	go run ./cmd/reviews -delete 12       # permanently delete review #12
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
	"bosfoot/internal/notify"
	"bosfoot/logger"
)

func main() {
	status := flag.String("status", "pending", "list reviews with this status (pending|approved|rejected)")
	approve := flag.Int("approve", 0, "approve the review with this id")
	reject := flag.Int("reject", 0, "reject the review with this id (keeps the row)")
	del := flag.Int("delete", 0, "permanently delete the review with this id")
	flag.Parse()

	db, err := database.Connect()
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	switch {
	case *approve != 0:
		lg, err := logger.NewLogger("bosfoot.log")
		if err != nil {
			log.Fatal(err)
		}
		defer lg.Close()
		approveReview(db, notify.New(lg), *approve)
		return
	case *reject != 0:
		setStatus(db, *reject, "rejected")
		return
	case *del != 0:
		deleteReview(db, *del)
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

// approveReview approves one review and, on a real pending→approved transition,
// emails the reviewer a localized thank-you. The guarded UPDATE means re-running
// -approve on an already-approved review is a no-op that sends no second email.
func approveReview(db *sql.DB, mailer *notify.Mailer, id int) {
	var productID int
	var orderID sql.NullInt64
	var lang, author string
	err := db.QueryRow(`
		UPDATE reviews SET status = 'approved'
		WHERE id = $1 AND status <> 'approved'
		RETURNING product_id, order_id, lang_code::text, author_name
	`, id).Scan(&productID, &orderID, &lang, &author)
	if err == sql.ErrNoRows {
		// No row updated: either the id doesn't exist or it was already approved.
		var existing string
		if e := db.QueryRow(`SELECT status FROM reviews WHERE id = $1`, id).Scan(&existing); e == sql.ErrNoRows {
			fmt.Printf("No review with id %d.\n", id)
		} else {
			fmt.Printf("Review #%d is already approved (no thank-you re-sent).\n", id)
		}
		return
	}
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Review #%d approved.\n", id)

	// Look up the product name + reviewer email for the thank-you. Best-effort:
	// a missing email or an SMTP outage never un-approves the review.
	var productName, email string
	_ = db.QueryRow(`
		SELECT b.name || ' ' || p.name FROM products p
		JOIN brands b ON b.id = p.brand_id WHERE p.id = $1
	`, productID).Scan(&productName)
	if orderID.Valid {
		_ = db.QueryRow(`SELECT COALESCE(email, '') FROM orders WHERE id = $1`, orderID.Int64).Scan(&email)
	}

	if email == "" {
		fmt.Println("    (no reviewer email on file — thank-you not sent)")
		return
	}
	if !mailer.Enabled() {
		fmt.Printf("    (SMTP not configured — thank-you to %s not sent)\n", email)
		return
	}
	if err := mailer.ReviewApproved(notify.ReviewApprovedData{
		Email: email, Name: author, Locale: lang, ProductName: productName,
	}); err != nil {
		fmt.Printf("    ! thank-you send failed: %v\n", err)
		return
	}
	fmt.Printf("    ✓ thank-you emailed to %s\n", email)
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

// deleteReview permanently removes one review row. Use -reject instead to just
// hide it while keeping the record.
func deleteReview(db *sql.DB, id int) {
	res, err := db.Exec(`DELETE FROM reviews WHERE id = $1`, id)
	if err != nil {
		log.Fatal(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		fmt.Printf("No review with id %d.\n", id)
		return
	}
	fmt.Printf("Review #%d deleted.\n", id)
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
