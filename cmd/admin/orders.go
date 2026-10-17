package main

import (
	"database/sql"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"bosfoot/internal/notify"
	"bosfoot/internal/reviewinvite"
	"bosfoot/internal/site"

	"github.com/lib/pq"
)

type orderItem struct {
	Product, Brand, Size, Color, Fulfilment string
	Qty, PriceMKD                           int
}

type order struct {
	ID                                  int
	Created                             time.Time
	FirstName, Name, Email, Phone, City string
	Address, Notes, Status, Locale      string
	Payment                             string
	TotalMKD                            int
	IsTest                              bool
	InvitedAt                           *time.Time
	InvoiceNo                           string // "0006/2026" once issued
	Reviewed                            bool
	Items                               []orderItem
}

// Tab sorts an order into the admin's tabs. Finished = delivered AND the review
// request has gone out; delivered without one stays Active as a reminder.
func (o order) Tab() string {
	switch {
	case o.IsTest:
		return "test"
	case o.Status == "cancelled":
		return "cancelled"
	case o.Status == "delivered" && o.InvitedAt != nil:
		return "finished"
	}
	return "active"
}

// NeedsReview flags a delivered order whose review request hasn't been sent.
func (o order) NeedsReview() bool { return o.Status == "delivered" && o.InvitedAt == nil }

// Phone in +389… form for the Viber chat link.
func (o order) IntlPhone() string { return reviewinvite.IntlPhone(o.Phone) }

var tabs = []struct{ Key, Label string }{
	{"active", "Active"}, {"finished", "Finished"}, {"cancelled", "Cancelled"}, {"test", "Test"},
}

func (a *app) orders(w http.ResponseWriter, r *http.Request) {
	tab := r.URL.Query().Get("tab")
	if tab == "" {
		tab = "active"
	}
	all, err := a.loadOrders()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	counts := map[string]int{}
	var shown []order
	for _, o := range all {
		counts[o.Tab()]++
		if o.Tab() == tab {
			shown = append(shown, o)
		}
	}
	a.render(w, "orders.html", a.page("orders", map[string]any{
		"Tab": tab, "Tabs": tabs, "Counts": counts, "Orders": shown,
		"Msg": r.URL.Query().Get("msg"), "Err": r.URL.Query().Get("err"),
		"MailOn": a.mailer.Enabled(),
	}))
}

func (a *app) loadOrders() ([]order, error) {
	rows, err := a.db.Query(`
		SELECT o.id, o.created_at, o.first_name, o.first_name || ' ' || o.last_name,
		       o.email, COALESCE(o.phone, ''), o.city, o.address, COALESCE(o.notes, ''),
		       o.status::text, o.locale::text, o.payment_method::text, o.total_mkd, o.is_test,
		       (SELECT min(rt.created_at) FROM review_tokens rt WHERE rt.order_id = o.id),
		       EXISTS (SELECT 1 FROM review_tokens rt WHERE rt.order_id = o.id AND rt.used_at IS NOT NULL),
		       COALESCE((SELECT lpad(i.seq::text, 4, '0') || '/' || i.year FROM invoices i WHERE i.order_id = o.id), '')
		FROM orders o
		ORDER BY o.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []order
	byID := map[int]*order{}
	var ids []int64
	for rows.Next() {
		var o order
		if err := rows.Scan(&o.ID, &o.Created, &o.FirstName, &o.Name, &o.Email, &o.Phone, &o.City,
			&o.Address, &o.Notes, &o.Status, &o.Locale, &o.Payment, &o.TotalMKD, &o.IsTest,
			&o.InvitedAt, &o.Reviewed, &o.InvoiceNo); err != nil {
			return nil, err
		}
		list = append(list, o)
		ids = append(ids, int64(o.ID))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range list {
		byID[list[i].ID] = &list[i]
	}

	irows, err := a.db.Query(`
		SELECT oi.order_id, COALESCE(p.name, '?'), COALESCE(b.name, ''), COALESCE(oi.size, ''),
		       COALESCE(oi.color, ''), COALESCE(oi.fulfilment, ''), oi.quantity, oi.unit_price_mkd
		FROM order_items oi
		LEFT JOIN products p ON p.id = oi.product_id
		LEFT JOIN brands b ON b.id = p.brand_id
		WHERE oi.order_id = ANY($1)
		ORDER BY oi.id`, pq.Array(ids))
	if err != nil {
		return nil, err
	}
	defer irows.Close()
	for irows.Next() {
		var id int
		var it orderItem
		if err := irows.Scan(&id, &it.Product, &it.Brand, &it.Size, &it.Color, &it.Fulfilment,
			&it.Qty, &it.PriceMKD); err != nil {
			return nil, err
		}
		if o := byID[id]; o != nil {
			o.Items = append(o.Items, it)
		}
	}
	return list, irows.Err()
}

// orderID reads {id} and returns the order's current status + test flag.
func (a *app) orderID(r *http.Request) (id int, status string, isTest bool, err error) {
	id, err = strconv.Atoi(r.PathValue("id"))
	if err != nil {
		return 0, "", false, errors.New("bad order id")
	}
	err = a.db.QueryRow(`SELECT status::text, is_test FROM orders WHERE id = $1`, id).Scan(&status, &isTest)
	if errors.Is(err, sql.ErrNoRows) {
		err = fmt.Errorf("no order %s", site.OrderNumber(id))
	}
	return id, status, isTest, err
}

func (a *app) setStatus(w http.ResponseWriter, r *http.Request) {
	id, cur, _, err := a.orderID(r)
	if err != nil {
		a.back(w, r, "", err.Error())
		return
	}
	to := r.FormValue("to")
	if to != "shipped" && to != "delivered" && to != "cancelled" {
		a.back(w, r, "", "unknown status "+to)
		return
	}
	if cur == "cancelled" {
		// Reviving a cancelled order would need its stock taken off the shelf
		// again — not worth the risk; place a new order instead.
		a.back(w, r, "", site.OrderNumber(id)+" is cancelled — it can't be reopened here")
		return
	}
	tx, err := a.db.Begin()
	if err != nil {
		a.back(w, r, "", err.Error())
		return
	}
	defer tx.Rollback()
	restocked := 0
	if to == "cancelled" {
		if restocked, err = restock(tx, id); err != nil {
			a.back(w, r, "", "restock: "+err.Error())
			return
		}
	}
	if _, err := tx.Exec(`UPDATE orders SET status = $1, updated_at = now() WHERE id = $2`, to, id); err != nil {
		a.back(w, r, "", err.Error())
		return
	}
	if err := tx.Commit(); err != nil {
		a.back(w, r, "", err.Error())
		return
	}
	msg := site.OrderNumber(id) + " marked " + to
	if restocked > 0 {
		msg += fmt.Sprintf(" — %d pair(s) back in stock", restocked)
	}
	if to == "delivered" {
		msg += " — now send the review request"
	}
	if to == "cancelled" {
		var invNo string
		_ = a.db.QueryRow(`SELECT lpad(seq::text, 4, '0') || '/' || year FROM invoices WHERE order_id = $1`, id).Scan(&invNo)
		if invNo != "" {
			msg += " — NOTE: invoice " + invNo + " exists; ask your accountant about a storno (credit note)"
		}
	}
	a.back(w, r, msg, "")
}

func (a *app) toggleTest(w http.ResponseWriter, r *http.Request) {
	id, _, isTest, err := a.orderID(r)
	if err != nil {
		a.back(w, r, "", err.Error())
		return
	}
	if _, err := a.db.Exec(`UPDATE orders SET is_test = $1 WHERE id = $2`, !isTest, id); err != nil {
		a.back(w, r, "", err.Error())
		return
	}
	if isTest {
		a.back(w, r, site.OrderNumber(id)+" is a real order again", "")
	} else {
		a.back(w, r, site.OrderNumber(id)+" marked as TEST (see the Test tab)", "")
	}
}

// deleteTest permanently removes a TEST order (items + review tokens cascade;
// any review keeps existing with order_id NULL). In-stock lines go back on the
// shelf unless the order was already cancelled (cancel restocked them).
func (a *app) deleteTest(w http.ResponseWriter, r *http.Request) {
	id, cur, isTest, err := a.orderID(r)
	if err != nil {
		a.back(w, r, "", err.Error())
		return
	}
	if !isTest {
		a.back(w, r, "", "only TEST orders can be deleted — mark it Test first, or Cancel a real order")
		return
	}
	var invNo string
	_ = a.db.QueryRow(`SELECT lpad(seq::text, 4, '0') || '/' || year FROM invoices WHERE order_id = $1`, id).Scan(&invNo)
	if invNo != "" {
		a.back(w, r, "", "invoice "+invNo+" was issued for this order — it can't be deleted (invoices must be kept)")
		return
	}
	tx, err := a.db.Begin()
	if err != nil {
		a.back(w, r, "", err.Error())
		return
	}
	defer tx.Rollback()
	restocked := 0
	if cur != "cancelled" {
		if restocked, err = restock(tx, id); err != nil {
			a.back(w, r, "", "restock: "+err.Error())
			return
		}
	}
	if _, err := tx.Exec(`DELETE FROM orders WHERE id = $1 AND is_test`, id); err != nil {
		a.back(w, r, "", err.Error())
		return
	}
	if err := tx.Commit(); err != nil {
		a.back(w, r, "", err.Error())
		return
	}
	msg := site.OrderNumber(id) + " deleted"
	if restocked > 0 {
		msg += fmt.Sprintf(" — %d pair(s) back in stock", restocked)
	}
	a.back(w, r, msg, "")
}

// restock puts an order's in-stock lines back on the shelf (lines taken from
// product_stock at checkout have fulfilment 'stock'; reservation/dropship never
// touched stock). Quantities are summed per variant first, because UPDATE…FROM
// updates each stock row only once. Returns the number of pairs restocked.
func restock(tx *sql.Tx, orderID int) (int, error) {
	var n sql.NullInt64
	err := tx.QueryRow(`
		WITH lines AS (
		  SELECT oi.product_id, sz.id AS size_id, pc.id AS color_id, SUM(oi.quantity) AS qty
		  FROM order_items oi
		  JOIN product_sizes  sz ON sz.product_id = oi.product_id AND oi.size ~ '^[0-9]+(\.[0-9]+)?$' AND sz.eu_size = oi.size::numeric
		  JOIN product_colors pc ON pc.product_id = oi.product_id AND pc.color = oi.color
		  WHERE oi.order_id = $1 AND oi.fulfilment = 'stock'
		  GROUP BY 1, 2, 3
		), upd AS (
		  UPDATE product_stock ps SET qty = ps.qty + l.qty
		  FROM lines l
		  WHERE ps.product_id = l.product_id AND ps.size_id = l.size_id AND ps.color_id = l.color_id
		  RETURNING l.qty
		)
		SELECT SUM(qty) FROM upd`, orderID).Scan(&n)
	return int(n.Int64), err
}

// reviewEmail creates (or reuses) the order's review links, emails them and
// marks the order delivered — the same as `reviewinvites -order N -commit`.
func (a *app) reviewEmail(w http.ResponseWriter, r *http.Request) {
	o, links, err := a.prepareReview(r)
	if err != nil {
		a.back(w, r, "", err.Error())
		return
	}
	if !a.mailer.Enabled() {
		a.back(w, r, "", "email isn't configured (SMTP_* in .env) — use Viber")
		return
	}
	if err := a.mailer.ReviewInvite(notify.ReviewInviteData{
		OrderID: o.ID, Email: o.Email, Name: o.Name, Locale: o.Locale, Links: links,
	}); err != nil {
		a.back(w, r, "", "email failed: "+err.Error())
		return
	}
	if err := reviewinvite.MarkDelivered(a.db, o.ID); err != nil {
		a.back(w, r, "", "emailed, but marking delivered failed: "+err.Error())
		return
	}
	a.back(w, r, "Review request emailed to "+o.Email+" — "+site.OrderNumber(o.ID)+" finished", "")
}

// reviewViber creates (or reuses) the links, marks delivered, and shows the
// click-to-send Viber card for this customer.
func (a *app) reviewViber(w http.ResponseWriter, r *http.Request) {
	o, links, err := a.prepareReview(r)
	if err != nil {
		a.back(w, r, "", err.Error())
		return
	}
	if err := reviewinvite.MarkDelivered(a.db, o.ID); err != nil {
		a.back(w, r, "", err.Error())
		return
	}
	msg := reviewinvite.ViberMessage(o.FirstName, o.Locale, links)
	data := a.page("orders", map[string]any{"O": o, "Msg": msg, "Back": r.FormValue("tab"),
		"Forward": viberURL(reviewinvite.ViberForwardURL(msg))})
	if p := o.IntlPhone(); p != "" {
		data["Chat"] = viberURL(reviewinvite.ViberChatURL(p, msg))
	}
	a.render(w, "viber.html", data)
}

// prepareReview loads the order and creates its review tokens (idempotent:
// a resend reuses the existing link). Cancelled orders are refused.
func (a *app) prepareReview(r *http.Request) (order, []notify.ReviewLink, error) {
	id, status, _, err := a.orderID(r)
	if err != nil {
		return order{}, nil, err
	}
	if status == "cancelled" {
		return order{}, nil, errors.New(site.OrderNumber(id) + " is cancelled — no review request")
	}
	var o order
	if err := a.db.QueryRow(`SELECT id, first_name, first_name || ' ' || last_name, email,
		COALESCE(phone, ''), locale::text FROM orders WHERE id = $1`, id).
		Scan(&o.ID, &o.FirstName, &o.Name, &o.Email, &o.Phone, &o.Locale); err != nil {
		return order{}, nil, err
	}
	links, err := reviewinvite.BuildLinks(a.db, a.siteURL, o.ID, o.Locale, true)
	if err != nil {
		return order{}, nil, err
	}
	if len(links) == 0 {
		return order{}, nil, errors.New(site.OrderNumber(id) + " has no products to review")
	}
	return o, links, nil
}

// back redirects to the tab the action came from, with a flash message.
func (a *app) back(w http.ResponseWriter, r *http.Request, msg, errMsg string) {
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
	http.Redirect(w, r, "/orders?"+q.Encode()+"#"+strings.TrimSpace(r.PathValue("id")), http.StatusSeeOther)
}

func (a *app) render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := a.tmpl.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// viberURL marks a viber:// link we built ourselves as safe for href.
func viberURL(s string) template.URL { return template.URL(s) }
