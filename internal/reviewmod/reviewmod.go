// Package reviewmod moderates product reviews (reviews land 'pending' and show on
// the site only once approved). Shared by cmd/reviews and cmd/admin.
//
// Review photos live on the server's UPLOADS_DIR: approving moves them from the
// unserved pending dir to the public dir, rejecting deletes them. That only works
// where those files are — so callers on another machine check PhotosLocal first.
package reviewmod

import (
	"database/sql"
	"errors"
	"fmt"
	"os"

	"bosfoot/internal/notify"
	"bosfoot/internal/uploads"
)

// ErrNotFound means there is no review with that id.
var ErrNotFound = errors.New("no such review")

// ApproveResult reports what Approve did beyond flipping the status.
type ApproveResult struct {
	AlreadyApproved bool   // was approved before: nothing done, no second email
	PhotosTotal     int    // photos on the review
	PhotosPublished int    // moved pending → public
	Email           string // reviewer email from the order ("" if none)
	Emailed         bool   // thank-you sent
	EmailErr        error  // thank-you failed (review stays approved)
}

// Approve approves a review and, on a real pending/rejected → approved
// transition, publishes its photos and emails the reviewer a localized
// thank-you. The guarded UPDATE makes re-approving a no-op (no second email).
func Approve(db *sql.DB, mailer *notify.Mailer, id int) (ApproveResult, error) {
	var res ApproveResult
	var productID int
	var orderID sql.NullInt64
	var lang, author string
	err := db.QueryRow(`
		UPDATE reviews SET status = 'approved'
		WHERE id = $1 AND status <> 'approved'
		RETURNING product_id, order_id, lang_code::text, author_name
	`, id).Scan(&productID, &orderID, &lang, &author)
	if errors.Is(err, sql.ErrNoRows) {
		var existing string
		if e := db.QueryRow(`SELECT status FROM reviews WHERE id = $1`, id).Scan(&existing); errors.Is(e, sql.ErrNoRows) {
			return res, ErrNotFound
		}
		res.AlreadyApproved = true
		return res, nil
	}
	if err != nil {
		return res, err
	}

	// Publish its photos. Best-effort — a missing file doesn't un-approve.
	names := Photos(db, id)
	res.PhotosTotal = len(names)
	for _, fn := range names {
		if uploads.Publish(fn) == nil {
			res.PhotosPublished++
		}
	}

	// Thank-you email. Best-effort: no email / SMTP outage never un-approves.
	var productName string
	_ = db.QueryRow(`SELECT b.name || ' ' || p.name FROM products p
		JOIN brands b ON b.id = p.brand_id WHERE p.id = $1`, productID).Scan(&productName)
	if orderID.Valid {
		_ = db.QueryRow(`SELECT COALESCE(email, '') FROM orders WHERE id = $1`, orderID.Int64).Scan(&res.Email)
	}
	if res.Email != "" && mailer.Enabled() {
		if err := mailer.ReviewApproved(notify.ReviewApprovedData{
			Email: res.Email, Name: author, Locale: lang, ProductName: productName,
		}); err != nil {
			res.EmailErr = err
		} else {
			res.Emailed = true
		}
	}
	return res, nil
}

// Reject hides a review and deletes its photo files + rows, so moderated-out
// content never lingers on disk. The row is kept as 'rejected' for the record.
// Returns how many photo files were removed.
func Reject(db *sql.DB, id int) (int, error) {
	names := Photos(db, id)
	r, err := db.Exec(`UPDATE reviews SET status = 'rejected' WHERE id = $1`, id)
	if err != nil {
		return 0, err
	}
	if n, _ := r.RowsAffected(); n == 0 {
		return 0, ErrNotFound
	}
	for _, fn := range names {
		_ = uploads.Remove(fn)
	}
	if len(names) > 0 {
		_, _ = db.Exec(`DELETE FROM review_photos WHERE review_id = $1`, id)
	}
	return len(names), nil
}

// Delete permanently removes a review row (cascade drops its photo rows) and
// its photo files. Returns how many photo files were removed.
func Delete(db *sql.DB, id int) (int, error) {
	names := Photos(db, id)
	r, err := db.Exec(`DELETE FROM reviews WHERE id = $1`, id)
	if err != nil {
		return 0, err
	}
	if n, _ := r.RowsAffected(); n == 0 {
		return 0, ErrNotFound
	}
	for _, fn := range names {
		_ = uploads.Remove(fn)
	}
	return len(names), nil
}

// Photos returns a review's photo filenames in display order.
func Photos(db *sql.DB, id int) []string {
	rows, err := db.Query(`SELECT filename FROM review_photos WHERE review_id = $1 ORDER BY sort_order, id`, id)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var f string
		if rows.Scan(&f) == nil {
			out = append(out, f)
		}
	}
	return out
}

// PhotosLocal reports whether this machine holds the review's photo files (the
// pending or public copy), i.e. whether Approve/Reject can move/delete them
// here. A review without photos is always fine.
func PhotosLocal(names []string) bool {
	for _, fn := range names {
		pub := uploads.PublicRoot() + "/reviews/" + fn
		if _, err := os.Stat(uploads.PendingFsPath(fn)); err != nil {
			if _, err := os.Stat(pub); err != nil {
				return false
			}
		}
	}
	return true
}

// Server is where review photos live (UPLOADS_DIR) and the remote command that
// moderates a photo review there — run by cmd/admin over SSH (key auth), or by
// hand. Uses the droplet's deployed checkout of cmd/reviews.
const (
	ServerSSH = "bobson@206.81.21.49"
	serverDir = "/srv/bosfoot"
	serverGo  = "/usr/local/go/bin/go"
)

// ServerCommand is the remote shell command for `reviews -<action> <id>`.
func ServerCommand(action string, id int) string {
	return fmt.Sprintf("cd %s && %s run ./cmd/reviews -%s %d", serverDir, serverGo, action, id)
}
