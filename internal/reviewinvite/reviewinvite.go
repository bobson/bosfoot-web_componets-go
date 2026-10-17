// Package reviewinvite builds post-purchase review requests: the single-use
// review links (one token per ordered product — the buyer's proof of purchase,
// there are no accounts) and the personal Viber message. Shared by
// cmd/reviewinvites and cmd/admin so both send exactly the same thing.
package reviewinvite

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"

	"bosfoot/internal/notify"
)

// MarkDelivered sets the order delivered. A review request is only sent after
// the shoes arrive, so sending one is also the delivered action.
func MarkDelivered(db *sql.DB, orderID int) error {
	_, err := db.Exec(`UPDATE orders SET status = 'delivered', updated_at = now() WHERE id = $1`, orderID)
	return err
}

// BuildLinks returns the per-product review links for an order. In commit mode
// it inserts a token per (order, product) first; in dry-run it uses a throwaway
// placeholder token so the preview shows realistic URLs without writing.
func BuildLinks(db *sql.DB, siteURL string, orderID int, locale string, commit bool) ([]notify.ReviewLink, error) {
	prodRows, err := db.Query(`
		SELECT DISTINCT p.id, p.name, b.slug, p.slug
		FROM order_items oi
		JOIN products p ON p.id = oi.product_id
		JOIN brands   b ON b.id = p.brand_id
		WHERE oi.order_id = $1
		ORDER BY p.name
	`, orderID)
	if err != nil {
		return nil, err
	}
	defer prodRows.Close()

	type prod struct {
		id              int
		name, brandSlug string
		slug            string
	}
	var prods []prod
	for prodRows.Next() {
		var p prod
		if err := prodRows.Scan(&p.id, &p.name, &p.brandSlug, &p.slug); err != nil {
			return nil, err
		}
		prods = append(prods, p)
	}
	if err := prodRows.Err(); err != nil {
		return nil, err
	}

	var email string
	if err := db.QueryRow(`SELECT email FROM orders WHERE id = $1`, orderID).Scan(&email); err != nil {
		return nil, err
	}

	var links []notify.ReviewLink
	for _, p := range prods {
		// Dry-run uses a visibly-fake placeholder so nobody pastes a preview link
		// to a customer (it would resolve to no token → "invalid"). Commit mode
		// inserts a real single-use token and uses that.
		token := "<preview>"
		if commit {
			token = NewToken()
			// One token per (order, product). ON CONFLICT keeps it idempotent if
			// this somehow re-runs for a partially-invited order.
			if err := db.QueryRow(`
				INSERT INTO review_tokens (token, order_id, product_id, email, lang_code)
				VALUES ($1, $2, $3, $4, $5::lang_code)
				ON CONFLICT (order_id, product_id) DO UPDATE SET order_id = review_tokens.order_id
				RETURNING token
			`, token, orderID, p.id, email, locale).Scan(&token); err != nil {
				return nil, err
			}
		}
		links = append(links, notify.ReviewLink{
			ProductName: p.name,
			URL:         fmt.Sprintf("%s/%s/review/%s", siteURL, locale, token),
		})
	}
	return links, nil
}

// NewToken returns a URL-safe random token.
func NewToken() string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		panic(err) // crypto/rand never fails on supported platforms
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// ViberMessage is the short, personal review request — Macedonian, Cyrillic
// for /mk orders and Latin script otherwise.
func ViberMessage(firstName, locale string, links []notify.ReviewLink) string {
	// The Viber number shows the owner's personal name, so the message says up
	// front that it's Bosfoot and signs off as Bosfoot. Informal "ти", like the
	// site. Always Macedonian: Cyrillic for /mk orders, Latin script for the rest
	// (/sq, /en — e.g. Albanian-speaking customers who read Macedonian in Latin).
	var hello, body, thanks string
	if locale == "mk" {
		hello = "Здраво " + firstName + "! Ова е Bosfoot 👟 Како ти се твоите нови барефут обувки? 🙂"
		body = "Ќе ни значи многу ако оставиш кратка оценка — како ти одговара големината, колку се удобни и каков е квалитетот. Им помага на другите полесно да одлучат, а нам како мала локална продавница ни значи многу за да продолжиме понатаму."
		thanks = "Благодарам! — Bosfoot"
	} else {
		hello = "Zdravo " + firstName + "! Ova e Bosfoot 👟 Kako ti se tvoite novi barefoot obuvki? 🙂"
		body = "Ke ni znachi mnogu ako ostavish kratka ocenka — kako ti odgovara goleminata, kolku se udobni i kakov e kvalitetot. Im pomaga na drugite polesno da odluchat, a nam kako mala lokalna prodavnica ni znachi mnogu za da prodolzhime ponatamu."
		thanks = "Blagodaram! — Bosfoot"
	}
	var b strings.Builder
	b.WriteString(hello + " " + body + " " + thanks)
	for _, l := range links {
		if len(links) > 1 {
			b.WriteString("\n" + l.ProductName + ":")
		}
		b.WriteString("\n" + l.URL)
	}
	return b.String()
}

// IntlPhone turns the phone as typed at checkout into +389… form for Viber:
// "070 377 075" → "+38970377075", "+389 70326290" → "+38970326290",
// "0038970…" → "+38970…". Returns "" when there's nothing usable.
func IntlPhone(raw string) string {
	var d strings.Builder
	for i, r := range raw {
		if r >= '0' && r <= '9' || (r == '+' && i == strings.IndexAny(raw, "+0123456789")) {
			d.WriteRune(r)
		}
	}
	p := d.String()
	switch {
	case p == "" || p == "+":
		return ""
	case strings.HasPrefix(p, "+"):
		return p
	case strings.HasPrefix(p, "00"):
		return "+" + p[2:]
	case strings.HasPrefix(p, "389"):
		return "+" + p
	case strings.HasPrefix(p, "0"):
		return "+389" + p[1:] // Macedonian national format
	}
	return "+" + p
}

// viberEscape query-escapes (so & + = are encoded), then writes spaces as %20 —
// Viber can show a "+" literally.
func viberEscape(s string) string {
	return strings.ReplaceAll(url.QueryEscape(s), "+", "%20")
}

// ViberChatURL opens the chat with this number; draft pre-fills the message on
// Viber versions that support it (others just open the chat — paste instead).
func ViberChatURL(intlNumber, msg string) string {
	return "viber://chat?number=" + viberEscape(intlNumber) + "&draft=" + viberEscape(msg)
}

// ViberForwardURL opens Viber with the message ready; you pick the contact.
func ViberForwardURL(msg string) string {
	return "viber://forward?text=" + viberEscape(msg)
}
