// Command reviewinvites sends post-purchase "leave a review" emails. Each
// invite carries a single-use token per purchased product, which is the buyer's
// proof of purchase on the review page (there are no user accounts).
//
// There is no automated fulfilment step, so invites are time-based: an order
// becomes eligible once it is older than -days (default 7) and has not been
// invited yet. Use -order to target one order regardless of age.
//
// DEFAULT IS DRY-RUN: it prints which orders/links it *would* send and writes
// nothing. Pass -commit to generate the tokens and send. Links are printed even
// when SMTP is unconfigured, so you can paste them into whatever you email from.
//
//	go run ./cmd/reviewinvites                     # preview eligible orders (dry-run)
//	go run ./cmd/reviewinvites -days 10            # older-than-10-days threshold
//	go run ./cmd/reviewinvites -order 42           # just order #42
//	go run ./cmd/reviewinvites -commit             # create tokens + send
//	go run ./cmd/reviewinvites -order 42 -resend -commit  # re-email order #42's existing link
//	go run ./cmd/reviewinvites -order 42 -viber -commit   # Viber instead of email (see below)
//
// -viber prepares the invite for Viber instead of emailing it: it prints a
// ready-to-paste message (customer's first name + link), their phone number in
// international form, and two viber:// links — one opens the chat with that
// number, the other opens Viber with the message pre-filled to pick a contact.
// Tokens are still created and the order still marked delivered on -commit.
package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"

	"bosfoot/internal/database"
	"bosfoot/internal/notify"
	"bosfoot/internal/site"
	"bosfoot/logger"
)

func main() {
	days := flag.Int("days", 7, "invite orders older than this many days")
	orderID := flag.Int("order", 0, "target a single order id (ignores -days)")
	resend := flag.Bool("resend", false, "re-email an already-invited order's EXISTING link (requires -order)")
	commit := flag.Bool("commit", false, "actually create tokens and send (default: dry-run)")
	viber := flag.Bool("viber", false, "print a Viber message + viber:// links instead of emailing")
	flag.Parse()

	if *resend && *orderID == 0 {
		log.Fatal("-resend needs -order <id> (which order to re-send)")
	}

	db, err := database.Connect() // also loads .env (godotenv)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Read SITE_URL only after Connect() has loaded .env, so the laptop's .env
	// value is honoured. An inline `SITE_URL=... go run ...` still wins, because
	// godotenv.Load() never overrides an already-set environment variable.
	siteURL := strings.TrimRight(os.Getenv("SITE_URL"), "/")
	if siteURL == "" {
		siteURL = "http://localhost:8080"
	}

	lg, err := logger.NewLogger("bosfoot.log")
	if err != nil {
		log.Fatal(err)
	}
	defer lg.Close()
	mailer := notify.New(lg)

	// Eligible orders: have an email, and either the explicitly requested order or
	// old enough. Normally we skip orders that already have a token; -resend drops
	// that filter so we can re-email an order's EXISTING link (buildLinks reuses
	// the token via ON CONFLICT, so no new token is created).
	invitedFilter := "AND NOT EXISTS (SELECT 1 FROM review_tokens rt WHERE rt.order_id = o.id)"
	if *resend {
		invitedFilter = ""
	}
	rows, err := db.Query(`
		SELECT o.id, o.email, o.first_name || ' ' || o.last_name, o.locale::text, COALESCE(o.phone, '')
		FROM orders o
		WHERE COALESCE(o.email, '') <> ''
		  -- A cancelled order is dead: never invite it (no email, no token, no
		  -- delivered flip). Applies to both -order and the time-based path.
		  AND o.status <> 'cancelled'
		  `+invitedFilter+`
		  AND CASE WHEN $1 <> 0 THEN o.id = $1
		           ELSE o.created_at < now() - ($2 * interval '1 day') END
		ORDER BY o.created_at
	`, *orderID, *days)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	type order struct {
		id          int
		email, name string
		locale      string
		phone       string
	}
	var orders []order
	for rows.Next() {
		var o order
		if err := rows.Scan(&o.id, &o.email, &o.name, &o.locale, &o.phone); err != nil {
			log.Fatal(err)
		}
		orders = append(orders, o)
	}
	if err := rows.Err(); err != nil {
		log.Fatal(err)
	}

	if len(orders) == 0 {
		fmt.Println("No eligible orders to invite.")
		return
	}

	mode := "DRY-RUN (nothing written)"
	if *commit {
		mode = "COMMIT"
	}
	fmt.Printf("Review invites — %s — %d eligible order(s)\n\n", mode, len(orders))

	sent, failed := 0, 0
	for _, o := range orders {
		links, err := buildLinks(db, siteURL, o.id, o.locale, *commit)
		if err != nil {
			log.Printf("order #%d: %v", o.id, err)
			failed++
			continue
		}
		if len(links) == 0 {
			continue
		}

		fmt.Printf("%s  %s  <%s>  [%s]\n", site.OrderNumber(o.id), o.name, o.email, o.locale)
		for _, l := range links {
			fmt.Printf("    %s\n    %s\n", l.ProductName, l.URL)
		}

		if *viber {
			printViber(o.name, o.phone, o.locale, links, *commit)
		}

		if !*commit {
			fmt.Println()
			continue
		}

		if *viber {
			fmt.Println("    ✓ Viber message prepared (not emailed) — send it from Viber")
			sent++
		} else if mailer.Enabled() {
			if err := mailer.ReviewInvite(notify.ReviewInviteData{
				OrderID: o.id, Email: o.email, Name: o.name, Locale: o.locale, Links: links,
			}); err != nil {
				fmt.Printf("    ! send failed: %v (tokens created; links above)\n", err)
				failed++
			} else {
				fmt.Println("    ✓ emailed")
				sent++
			}
		} else {
			fmt.Println("    (SMTP not configured — paste the links above)")
			sent++
		}

		// An invite is only sent after the shoes are delivered, so sending one
		// also marks the order delivered — the invite is the single action, no
		// separate `orders -deliver` step. (Cancelled orders were already
		// excluded from the eligibility query above, so they never reach here.)
		if _, err := db.Exec(
			`UPDATE orders SET status = 'delivered' WHERE id = $1`,
			o.id); err != nil {
			fmt.Printf("    ! could not mark delivered: %v\n", err)
		} else {
			fmt.Println("    ✓ marked delivered")
		}
		fmt.Println()
	}

	if *commit {
		fmt.Printf("Done. %d sent/prepared, %d failed.\n", sent, failed)
	} else {
		fmt.Println("Dry-run only. Re-run with -commit to create tokens and send.")
	}
}

// buildLinks returns the per-product review links for an order. In commit mode
// it inserts a token per (order, product) first; in dry-run it uses a throwaway
// placeholder token so the preview shows realistic URLs without writing.
func buildLinks(db *sql.DB, siteURL string, orderID int, locale string, commit bool) ([]notify.ReviewLink, error) {
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
			token = newToken()
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

// newToken returns a URL-safe random token.
func newToken() string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		log.Fatal(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// printViber prints a ready-to-send Viber message for one order: the text, the
// customer's phone in international form, and viber:// links to open the chat
// or open Viber with the message pre-filled.
func printViber(name, phone, locale string, links []notify.ReviewLink, commit bool) {
	msg := viberMessage(strings.Split(name, " ")[0], locale, links)
	fmt.Println()
	fmt.Println("    ── Viber message ─────────────────────────────")
	for _, l := range strings.Split(msg, "\n") {
		fmt.Println("    " + l)
	}
	fmt.Println("    ──────────────────────────────────────────────")
	if !commit {
		fmt.Println("    (dry-run: the link is a placeholder — re-run with -commit before sending)")
		return
	}
	if p := intlPhone(phone); p != "" {
		fmt.Printf("    Phone:        %s\n", p)
		fmt.Printf("    Open chat:    viber://chat?number=%s\n", url.QueryEscape(p))
	} else {
		fmt.Println("    Phone:        (none on the order)")
	}
	// Query-escape (so & + = are encoded), then spaces as %20 — Viber can show a
	// "+" literally.
	fmt.Printf("    Send message: viber://forward?text=%s\n", strings.ReplaceAll(url.QueryEscape(msg), "+", "%20"))
}

// viberMessage is the short, personal review request (mk/sq/en by order locale).
func viberMessage(firstName, locale string, links []notify.ReviewLink) string {
	var hello, body, thanks string
	switch locale {
	case "sq":
		hello = "Përshëndetje " + firstName + "! Si ju duken këpucët e reja? 🙂"
		body = "Do të na pëlqente shumë nëse lini një vlerësim të shkurtër — si ju përshtatet madhësia, sa të rehatshme janë dhe si është cilësia. U ndihmon të tjerëve të vendosin më lehtë, dhe për ne si një dyqan i vogël lokal do të thotë shumë që të vazhdojmë më tej."
		thanks = "Faleminderit!"
	case "en":
		hello = "Hi " + firstName + "! How are you finding your new shoes? 🙂"
		body = "It would mean a lot if you left a short review — how the size fits, how comfortable they are and what you think of the quality. It helps others decide, and for a small local shop like ours it means a lot to keep going."
		thanks = "Thank you!"
	default: // mk
		hello = "Здраво " + firstName + "! Како ви се новите патики? 🙂"
		body = "Ќе ни значи многу ако оставите кратка оценка — како ви одговара големината, колку се удобни и каков е квалитетот. Им помага на другите полесно да одлучат, а нам како мала локална продавница ни значи многу за да продолжиме понатаму."
		thanks = "Благодарам!"
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

// intlPhone turns the phone as typed at checkout into +389… form for Viber:
// "070 377 075" → "+38970377075", "+389 70326290" → "+38970326290",
// "0038970…" → "+38970…". Returns "" when there's nothing usable.
func intlPhone(raw string) string {
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
