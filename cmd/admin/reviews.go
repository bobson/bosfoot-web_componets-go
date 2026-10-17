package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"bosfoot/internal/reviewmod"
	"bosfoot/internal/site"
)

type review struct {
	ID, Rating, Photos          int
	Fit                         *int
	Product, Author, Body, Lang string
	Size, Color, City, Status   string
	OrderID                     *int
	Created                     time.Time
	PhotosLocal                 bool // files are on this machine (else moderate via SSH)
}

func (r review) Stars() string {
	n := min(max(r.Rating, 0), 5)
	return strings.Repeat("★", n) + strings.Repeat("☆", 5-n)
}

func (r review) FitLabel() string {
	switch {
	case r.Fit == nil:
		return ""
	case *r.Fit < 0:
		return "runs small"
	case *r.Fit > 0:
		return "runs large"
	}
	return "true to size"
}

var reviewTabs = []struct{ Key, Label string }{
	{"pending", "Pending"}, {"approved", "Approved"}, {"rejected", "Rejected"},
}

func (a *app) reviews(w http.ResponseWriter, r *http.Request) {
	tab := r.URL.Query().Get("tab")
	if tab != "approved" && tab != "rejected" {
		tab = "pending"
	}
	counts := map[string]int{}
	crow, err := a.db.Query(`SELECT status, count(*) FROM reviews GROUP BY status`)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	for crow.Next() {
		var s string
		var n int
		if crow.Scan(&s, &n) == nil {
			counts[s] = n
		}
	}
	crow.Close()

	rows, err := a.db.Query(`
		SELECT r.id, r.rating, r.fit, b.name || ' ' || p.name, COALESCE(r.author_name, ''),
		       COALESCE(r.body, ''), r.lang_code::text, COALESCE(r.size, ''), COALESCE(r.color, ''),
		       COALESCE(r.buyer_city, ''), r.status, r.order_id, r.created_at,
		       (SELECT count(*) FROM review_photos rp WHERE rp.review_id = r.id)
		FROM reviews r
		JOIN products p ON p.id = r.product_id
		JOIN brands   b ON b.id = p.brand_id
		WHERE r.status = $1
		ORDER BY r.created_at DESC`, tab)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	var list []review
	for rows.Next() {
		var rv review
		var fit sql.NullInt64
		var oid sql.NullInt64
		if err := rows.Scan(&rv.ID, &rv.Rating, &fit, &rv.Product, &rv.Author, &rv.Body, &rv.Lang,
			&rv.Size, &rv.Color, &rv.City, &rv.Status, &oid, &rv.Created, &rv.Photos); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if fit.Valid {
			f := int(fit.Int64)
			rv.Fit = &f
		}
		if oid.Valid {
			o := int(oid.Int64)
			rv.OrderID = &o
		}
		list = append(list, rv)
	}
	if err := rows.Err(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	for i := range list {
		list[i].PhotosLocal = list[i].Photos == 0 || reviewmod.PhotosLocal(reviewmod.Photos(a.db, list[i].ID))
	}
	a.render(w, "reviews.html", a.page("reviews", map[string]any{
		"Tab": tab, "Tabs": reviewTabs, "Counts": counts, "Reviews": list,
		"Msg": r.URL.Query().Get("msg"), "Err": r.URL.Query().Get("err"),
	}))
}

// moderate approves or rejects one review. Text-only reviews (or photos held on
// this machine) are done here; a review whose photos live on the server is
// moderated THERE over SSH, so its photos are really published/deleted.
func (a *app) moderate(action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			a.reviewsBack(w, r, "", "bad review id")
			return
		}
		names := reviewmod.Photos(a.db, id)
		if len(names) > 0 && !reviewmod.PhotosLocal(names) {
			// The server's cmd/reviews acts on the PRODUCTION database, so only
			// take this path when the admin itself is on production — a sandbox
			// review id must never touch a live review.
			if dbLabel() != "PRODUCTION" {
				a.reviewsBack(w, r, "", fmt.Sprintf("review #%d has photos that aren't on this machine — on the sandbox it can't be moderated over SSH (that would hit production)", id))
				return
			}
			out, err := runOnServer(reviewmod.ServerCommand(action, id))
			if err != nil {
				a.reviewsBack(w, r, "", fmt.Sprintf("on the server: %v — %s", err, out))
				return
			}
			a.reviewsBack(w, r, "Done on the server: "+out, "")
			return
		}

		switch action {
		case "approve":
			res, err := reviewmod.Approve(a.db, a.mailer, id)
			switch {
			case errors.Is(err, reviewmod.ErrNotFound):
				a.reviewsBack(w, r, "", fmt.Sprintf("no review #%d", id))
			case err != nil:
				a.reviewsBack(w, r, "", err.Error())
			case res.AlreadyApproved:
				a.reviewsBack(w, r, fmt.Sprintf("Review #%d was already approved", id), "")
			default:
				msg := fmt.Sprintf("Review #%d approved — live on the product page within a minute", id)
				switch {
				case res.Emailed:
					msg += "; thank-you emailed to " + res.Email
				case res.EmailErr != nil:
					msg += "; thank-you email failed: " + res.EmailErr.Error()
				case res.Email != "" && !a.mailer.Enabled():
					msg += "; thank-you not sent (SMTP not configured)"
				}
				a.reviewsBack(w, r, msg, "")
			}
		case "reject":
			if _, err := reviewmod.Reject(a.db, id); err != nil {
				a.reviewsBack(w, r, "", err.Error())
				return
			}
			a.reviewsBack(w, r, fmt.Sprintf("Review #%d rejected (hidden from the site)", id), "")
		}
	}
}

// runOnServer runs a command on the droplet over SSH (key auth, no prompts) and
// returns its trimmed output. `go run` compiles first, so allow a few minutes.
func runOnServer(cmd string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	out, err := exec.CommandContext(ctx, "ssh", "-o", "BatchMode=yes", "-o", "ConnectTimeout=15",
		reviewmod.ServerSSH, cmd).CombinedOutput()
	return strings.Join(strings.Fields(strings.TrimSpace(string(out))), " "), err
}

func (a *app) reviewsBack(w http.ResponseWriter, r *http.Request, msg, errMsg string) {
	q := url.Values{}
	if t := r.FormValue("tab"); t != "" {
		q.Set("tab", t)
	}
	if msg != "" {
		q.Set("msg", msg)
	}
	if errMsg != "" {
		q.Set("err", errMsg)
	}
	http.Redirect(w, r, "/reviews?"+q.Encode(), http.StatusSeeOther)
}

// orderLabel renders an order id as its BF- number for the template.
func orderLabel(id *int) string {
	if id == nil {
		return "no order"
	}
	return site.OrderNumber(*id)
}
