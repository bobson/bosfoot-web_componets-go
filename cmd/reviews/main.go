// Command reviews lists and moderates product reviews from the terminal — the
// approval path for the guest-checkout review system (reviews land 'pending' and
// show on the site only once approved). Saves writing raw SQL.
//
//	go run ./cmd/reviews                  # list pending reviews (newest first)
//	go run ./cmd/reviews -status approved # list a different status
//	go run ./cmd/reviews -photo 12        # where to view review #12's photos first
//	go run ./cmd/reviews -approve 12      # approve #12 (publishes its photos)
//	go run ./cmd/reviews -reject 12       # reject #12 (hides it, deletes its photos)
//	go run ./cmd/reviews -delete 12       # permanently delete #12 (and its photos)
//
// Approved reviews appear on the product page within the 60s page-cache TTL.
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"bosfoot/internal/database"
	"bosfoot/internal/notify"
	"bosfoot/internal/site"
	"bosfoot/internal/uploads"
	"bosfoot/logger"
)

func main() {
	status := flag.String("status", "pending", "list reviews with this status (pending|approved|rejected)")
	approve := flag.Int("approve", 0, "approve the review with this id (publishes its photos)")
	reject := flag.Int("reject", 0, "reject the review with this id (hides it, deletes its photos)")
	del := flag.Int("delete", 0, "permanently delete the review with this id (and its photos)")
	photo := flag.Int("photo", 0, "print the photo URL/paths for review id (view before approving)")
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
		rejectReview(db, *reject)
		return
	case *del != 0:
		deleteReview(db, *del)
		return
	case *photo != 0:
		showPhotos(db, *photo)
		return
	}

	rows, err := db.Query(`
		SELECT r.id, b.name || ' ' || p.name AS product,
		       r.rating, r.fit, r.author_name, COALESCE(r.body, ''),
		       r.lang_code::text, r.created_at, COALESCE(r.order_id, 0),
		       (SELECT count(*) FROM review_photos rp WHERE rp.review_id = r.id) AS photos
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
		var id, rating, orderID, photos int
		var fit sql.NullInt64
		var product, author, body, lang string
		var createdAt time.Time
		if err := rows.Scan(&id, &product, &rating, &fit, &author, &body, &lang, &createdAt, &orderID, &photos); err != nil {
			log.Fatal(err)
		}
		if count == 0 {
			fmt.Println()
		}
		count++

		ord := "no order"
		if orderID != 0 {
			ord = site.OrderNumber(orderID)
		}
		pic := ""
		if photos > 0 {
			pic = fmt.Sprintf("  📷×%d (view: -photo %d)", photos, id)
		}
		fmt.Printf("#%d  %s  %s  (%s, %s, %s)%s\n",
			id, stars(rating), product, lang, ord, createdAt.Format("2006-01-02 15:04"), pic)
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

	// Publish its photos: move each from the unserved pending dir into the served
	// public dir. Best-effort — a missing file doesn't un-approve the review.
	if names := reviewPhotos(db, id); len(names) > 0 {
		ok := 0
		for _, fn := range names {
			if err := uploads.Publish(fn); err != nil {
				fmt.Printf("    ! photo publish failed (%s): %v\n", fn, err)
			} else {
				ok++
			}
		}
		fmt.Printf("    published %d/%d photo(s)\n", ok, len(names))
	}

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

// rejectReview hides a review and deletes its photo files + rows, so
// moderated-out content never lingers on disk (the pending files were never
// web-served). The review row is kept as 'rejected' for the record.
func rejectReview(db *sql.DB, id int) {
	names := reviewPhotos(db, id)
	res, err := db.Exec(`UPDATE reviews SET status = 'rejected' WHERE id = $1`, id)
	if err != nil {
		log.Fatal(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		fmt.Printf("No review with id %d.\n", id)
		return
	}
	for _, fn := range names {
		_ = uploads.Remove(fn)
	}
	if len(names) > 0 {
		_, _ = db.Exec(`DELETE FROM review_photos WHERE review_id = $1`, id)
	}
	fmt.Printf("Review #%d rejected", id)
	if len(names) > 0 {
		fmt.Printf(" (removed %d photo file(s))", len(names))
	}
	fmt.Println(".")
}

// deleteReview permanently removes one review row (cascade drops its photo rows)
// and deletes the photo files too. Use -reject to just hide it.
func deleteReview(db *sql.DB, id int) {
	names := reviewPhotos(db, id)
	res, err := db.Exec(`DELETE FROM reviews WHERE id = $1`, id)
	if err != nil {
		log.Fatal(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		fmt.Printf("No review with id %d.\n", id)
		return
	}
	for _, fn := range names {
		_ = uploads.Remove(fn)
	}
	fmt.Printf("Review #%d deleted", id)
	if len(names) > 0 {
		fmt.Printf(" (removed %d photo file(s))", len(names))
	}
	fmt.Println(".")
}

// reviewPhotos returns a review's photo filenames in display order.
func reviewPhotos(db *sql.DB, id int) []string {
	rows, err := db.Query(`SELECT filename FROM review_photos WHERE review_id = $1 ORDER BY sort_order, id`, id)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var f string
		if err := rows.Scan(&f); err == nil {
			out = append(out, f)
		}
	}
	return out
}

// showPhotos prints where to view a review's photos before approving: a live URL
// once approved, or the on-disk pending path (open it on the droplet) otherwise.
func showPhotos(db *sql.DB, id int) {
	var status string
	switch err := db.QueryRow(`SELECT status FROM reviews WHERE id = $1`, id).Scan(&status); err {
	case nil:
	case sql.ErrNoRows:
		fmt.Printf("No review with id %d.\n", id)
		return
	default:
		log.Fatal(err)
	}
	names := reviewPhotos(db, id)
	if len(names) == 0 {
		fmt.Printf("Review #%d has no photos.\n", id)
		return
	}
	siteURL := strings.TrimRight(os.Getenv("SITE_URL"), "/")
	fmt.Printf("Review #%d (%s) — %d photo(s):\n", id, status, len(names))
	for _, fn := range names {
		if status == "approved" {
			fmt.Printf("  %s%s\n", siteURL, uploads.PublicURL(fn))
		} else {
			fmt.Printf("  (pending, not web-served) open on the server: %s\n", uploads.PendingFsPath(fn))
		}
	}
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
