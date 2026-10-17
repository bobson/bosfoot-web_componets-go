package main

// Invoices (A4) and shipping labels (A6) as PDFs, rendered from HTML by headless
// Chromium (handles Cyrillic + matches the hand-made samples in books/).
//
// An invoice is created once per order with the next gap-free number NNNN/YYYY
// (invoice_series row-locked in the same transaction) and a SNAPSHOT of buyer,
// lines and totals — later order/price edits never change it. The company is NOT
// VAT-registered, so invoices carry no VAT lines (just a note). Labels are made
// fresh from the order each time (plus the invoice number when one exists).
// PDFs go to invoices/ (gitignored: customer data — back it up).

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"

	"bosfoot/internal/site"
)

// Seller details as on the invoice (public company data).
var company = struct {
	Brand, Name, Address, EDB, EMBS, Bank, Phone, Email, Web string
}{
	Brand:   "BOSFOOT",
	Name:    "БОСФУТ ТРЕЈД ДООЕЛ",
	Address: "Ул. Димитар Влахов 49/9, 6000 Охрид",
	EDB:     "4020026558013",
	EMBS:    "7954735",
	Bank:    "240120122652643, Уни Банка",
	Phone:   "+389 70 255 906",
	Email:   "info@bosfoot.com",
	Web:     "www.bosfoot.com",
}

// pairWeightKG is the label's default parcel weight per pair (shoe + box).
const pairWeightKG = 0.4

// invoiceRoot is where PDFs go: invoices/ for production, invoices/sandbox/ for
// the local sandbox DB — so a sandbox "0006/2026" can never be mistaken for (or
// served instead of) the real one.
func invoiceRoot() string {
	if dbLabel() == "PRODUCTION" {
		return "invoices"
	}
	return filepath.Join("invoices", "sandbox")
}

type invBuyer struct {
	Name       string `json:"name"`
	Address    string `json:"address"`
	PostalCode string `json:"postal_code"`
	City       string `json:"city"`
	Country    string `json:"country"`
	Phone      string `json:"phone"`
	Email      string `json:"email"`
}

type invLine struct {
	Desc     string `json:"desc"`
	Qty      int    `json:"qty"`
	UnitMKD  int    `json:"unit_mkd"`
	TotalMKD int    `json:"total_mkd"`
}

type invoice struct {
	Year, Seq   int
	OrderID     int
	IssuedOn    time.Time
	Buyer       invBuyer
	Lines       []invLine
	DeliveryMKD int
	TotalMKD    int
	Payment     string
}

// Number is the printed invoice number, e.g. "0006/2026".
func (iv invoice) Number() string { return fmt.Sprintf("%04d/%d", iv.Seq, iv.Year) }

func (iv invoice) GoodsMKD() int { return iv.TotalMKD - iv.DeliveryMKD }

func (iv invoice) file() string {
	return filepath.Join(invoiceRoot(), strconv.Itoa(iv.Year), fmt.Sprintf("bosfoot_faktura_%04d-%d.pdf", iv.Seq, iv.Year))
}

func paymentText(method string) string {
	if method == "bank_transfer" {
		return "Уплата на трансакциска сметка"
	}
	return "Откуп преку курир (плаќање при испорака)"
}

var countryMK = map[string]string{
	"MK": "Северна Македонија", "AL": "Албанија", "XK": "Косово",
	"RS": "Србија", "BG": "Бугарија", "GR": "Грција",
}

// orderSnapshot reads what an invoice/label needs from the live order.
func (a *app) orderSnapshot(q interface {
	QueryRow(string, ...any) *sql.Row
	Query(string, ...any) (*sql.Rows, error)
}, id int) (invBuyer, []invLine, int, string, error) {
	var b invBuyer
	var total int
	var pay, first, last, postal, country string
	err := q.QueryRow(`SELECT first_name, last_name, address, COALESCE(postal_code, ''), city, country,
		COALESCE(phone, ''), email, total_mkd, payment_method::text FROM orders WHERE id = $1`, id).
		Scan(&first, &last, &b.Address, &postal, &b.City, &country, &b.Phone, &b.Email, &total, &pay)
	if err != nil {
		return b, nil, 0, "", err
	}
	b.Name, b.PostalCode = titleName(first+" "+last), postal
	b.Country = countryMK[strings.TrimSpace(country)]
	if b.Country == "" {
		b.Country = country
	}
	rows, err := q.Query(`SELECT COALESCE(p.name, '?'), COALESCE(oi.size, ''), COALESCE(oi.color, ''),
		oi.quantity, oi.unit_price_mkd FROM order_items oi LEFT JOIN products p ON p.id = oi.product_id
		WHERE oi.order_id = $1 ORDER BY oi.id`, id)
	if err != nil {
		return b, nil, 0, "", err
	}
	defer rows.Close()
	var lines []invLine
	for rows.Next() {
		var name, size, color string
		var l invLine
		if err := rows.Scan(&name, &size, &color, &l.Qty, &l.UnitMKD); err != nil {
			return b, nil, 0, "", err
		}
		l.Desc = name
		var v []string
		if size != "" {
			v = append(v, "бр. "+size)
		}
		if color != "" {
			v = append(v, color)
		}
		if len(v) > 0 {
			l.Desc += " — " + strings.Join(v, ", ")
		}
		l.TotalMKD = l.Qty * l.UnitMKD
		lines = append(lines, l)
	}
	return b, lines, total, pay, rows.Err()
}

func (a *app) loadInvoice(orderID int) (invoice, error) {
	var iv invoice
	var buyer, lines []byte
	err := a.db.QueryRow(`SELECT year, seq, order_id, issued_on, buyer, lines, delivery_mkd, total_mkd, payment
		FROM invoices WHERE order_id = $1`, orderID).
		Scan(&iv.Year, &iv.Seq, &iv.OrderID, &iv.IssuedOn, &buyer, &lines, &iv.DeliveryMKD, &iv.TotalMKD, &iv.Payment)
	if err != nil {
		return iv, err
	}
	if err := json.Unmarshal(buyer, &iv.Buyer); err != nil {
		return iv, err
	}
	return iv, json.Unmarshal(lines, &iv.Lines)
}

// createInvoice issues the order's invoice (or returns the existing one).
func (a *app) createInvoice(orderID int) (invoice, bool, error) {
	if iv, err := a.loadInvoice(orderID); err == nil {
		return iv, false, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return iv, false, err
	}
	tx, err := a.db.Begin()
	if err != nil {
		return invoice{}, false, err
	}
	defer tx.Rollback()
	buyer, lines, total, pay, err := a.orderSnapshot(tx, orderID)
	if err != nil {
		return invoice{}, false, err
	}
	if len(lines) == 0 {
		return invoice{}, false, errors.New("the order has no items")
	}
	goods := 0
	for _, l := range lines {
		goods += l.TotalMKD
	}
	year := time.Now().Year()
	var seq int
	// Gap-free per-year counter: the row lock serialises concurrent issuers, and
	// it only advances if the invoice insert below commits too.
	if err := tx.QueryRow(`INSERT INTO invoice_series (year, last_seq) VALUES ($1, 1)
		ON CONFLICT (year) DO UPDATE SET last_seq = invoice_series.last_seq + 1
		RETURNING last_seq`, year).Scan(&seq); err != nil {
		return invoice{}, false, err
	}
	bj, _ := json.Marshal(buyer)
	lj, _ := json.Marshal(lines)
	if _, err := tx.Exec(`INSERT INTO invoices (year, seq, order_id, buyer, lines, delivery_mkd, total_mkd, payment)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		year, seq, orderID, bj, lj, max(total-goods, 0), total, pay); err != nil {
		return invoice{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return invoice{}, false, err
	}
	iv, err := a.loadInvoice(orderID)
	return iv, true, err
}

// renderPDF fills an HTML template and prints it to PDF with headless Chromium.
// Page size comes from the template's CSS @page rule (A4 / A6).
func (a *app) renderPDF(tmpl string, data any, out string) error {
	var html bytes.Buffer
	if err := a.tmpl.ExecuteTemplate(&html, tmpl, data); err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "bosfoot-pdf-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	src := filepath.Join(dir, "doc.html")
	if err := os.WriteFile(src, html.Bytes(), 0o600); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	abs, _ := filepath.Abs(out)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "chromium", "--headless", "--disable-gpu", "--no-pdf-header-footer",
		"--user-data-dir="+filepath.Join(dir, "profile"), "--print-to-pdf="+abs, "file://"+src)
	if msg, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("chromium: %v: %s", err, strings.TrimSpace(string(msg)))
	}
	if st, err := os.Stat(abs); err != nil || st.Size() == 0 {
		return errors.New("chromium produced no PDF")
	}
	return nil
}

func (a *app) invoiceDoc(iv invoice) map[string]any {
	iv.Buyer.Name = titleName(iv.Buyer.Name)
	return map[string]any{"C": company, "I": iv, "PayText": paymentText(iv.Payment),
		"Issued": iv.IssuedOn.Format("02.01.2006")}
}

// makeInvoice: POST — issue (once) and show the PDF.
func (a *app) makeInvoice(w http.ResponseWriter, r *http.Request) {
	id, status, _, err := a.orderID(r)
	if err != nil {
		a.back(w, r, "", err.Error())
		return
	}
	if status == "cancelled" {
		a.back(w, r, "", site.OrderNumber(id)+" is cancelled — no invoice")
		return
	}
	iv, created, err := a.createInvoice(id)
	if err != nil {
		a.back(w, r, "", "invoice: "+err.Error())
		return
	}
	if err := a.renderPDF("invoice.html", a.invoiceDoc(iv), iv.file()); err != nil {
		a.back(w, r, "", "invoice "+iv.Number()+" saved, but the PDF failed: "+err.Error())
		return
	}
	_ = created // either way: open the (same) PDF
	http.Redirect(w, r, fmt.Sprintf("/orders/%d/invoice.pdf", id), http.StatusSeeOther)
}

// invoicePDF: GET — serve the saved PDF (re-rendered from the snapshot if the
// file is missing, e.g. on another laptop).
func (a *app) invoicePDF(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	iv, err := a.loadInvoice(id)
	if err != nil {
		http.Error(w, "no invoice for this order yet", http.StatusNotFound)
		return
	}
	if _, err := os.Stat(iv.file()); err != nil {
		if err := a.renderPDF("invoice.html", a.invoiceDoc(iv), iv.file()); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `inline; filename="`+filepath.Base(iv.file())+`"`)
	http.ServeFile(w, r, iv.file())
}

// labelPDF: GET — a fresh A6 shipping label for the order.
func (a *app) labelPDF(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	buyer, lines, total, pay, err := a.orderSnapshot(a.db, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	pairs := 0
	for _, l := range lines {
		pairs += l.Qty
	}
	invNo := ""
	if iv, err := a.loadInvoice(id); err == nil {
		invNo = iv.Number()
	}
	data := map[string]any{"C": company, "B": buyer, "Lines": lines, "Total": total,
		"COD": pay != "bank_transfer", "InvNo": invNo, "Order": site.OrderNumber(id),
		"Weight": strconv.FormatFloat(pairWeightKG*float64(max(pairs, 1)), 'f', 1, 64)}
	out := filepath.Join(invoiceRoot(), "labels", fmt.Sprintf("bosfoot_label_%s.pdf", site.OrderNumber(id)))
	if err := a.renderPDF("label.html", data, out); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `inline; filename="`+filepath.Base(out)+`"`)
	http.ServeFile(w, r, out)
}

// den formats denars: den(6200, true) = "6.200,00 ден.", den(6200, false) = "6.200 ден."
func den(n int, decimals bool) string {
	s := fmtMKD(n)
	if decimals {
		return s + ",00 ден."
	}
	return s + " ден."
}

// titleName capitalises each part of a person's name however it was typed:
// "lendita TARAVARI" → "Lendita Taravari", "ана-марија" → "Ана-Марија".
func titleName(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		parts := strings.Split(w, "-")
		for j, p := range parts {
			r := []rune(strings.ToLower(p))
			if len(r) > 0 {
				r[0] = unicode.ToUpper(r[0])
			}
			parts[j] = string(r)
		}
		words[i] = strings.Join(parts, "-")
	}
	return strings.Join(words, " ")
}
