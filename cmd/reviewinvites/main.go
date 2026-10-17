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
//	go run ./cmd/reviewinvites -order 42           # just order #42 (or 42,43,50)
//	go run ./cmd/reviewinvites -commit             # create tokens + send
//	go run ./cmd/reviewinvites -order 42 -resend -commit  # re-email order #42's existing link
//	go run ./cmd/reviewinvites -order 42 -viber -commit   # Viber instead of email (see below)
//
// -viber prepares the invite for Viber instead of emailing it: it prints a
// ready-to-paste message (customer's first name + link), their phone number in
// international form, and two viber:// links — one opens the chat with that
// number, the other opens Viber with the message pre-filled to pick a contact.
// Tokens are still created and the order still marked delivered on -commit.
// Add -html out.html to also write a local page with one card per customer and
// click-to-send buttons (copy / open chat / send via Viber Desktop).
package main

import (
	"flag"
	"fmt"
	"html/template"
	"log"
	"os"
	"strconv"
	"strings"

	"bosfoot/internal/database"
	"bosfoot/internal/notify"
	"bosfoot/internal/reviewinvite"
	"bosfoot/internal/site"
	"bosfoot/logger"

	"github.com/lib/pq"
)

func main() {
	days := flag.Int("days", 7, "invite orders older than this many days")
	orderFlag := flag.String("order", "", "target order id(s), comma-separated, e.g. 64,65 (ignores -days)")
	resend := flag.Bool("resend", false, "re-email an already-invited order's EXISTING link (requires -order)")
	commit := flag.Bool("commit", false, "actually create tokens and send (default: dry-run)")
	viber := flag.Bool("viber", false, "print a Viber message + viber:// links instead of emailing")
	htmlOut := flag.String("html", "", "with -viber -commit: also write a click-to-send page to this file")
	flag.Parse()

	orderIDs, err := parseIDs(*orderFlag)
	if err != nil {
		log.Fatal(err)
	}
	if *resend && len(orderIDs) == 0 {
		log.Fatal("-resend needs -order <id> (which order(s) to re-send)")
	}
	if *htmlOut != "" && !*viber {
		log.Fatal("-html needs -viber")
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
		  AND CASE WHEN cardinality($1::int[]) > 0 THEN o.id = ANY($1::int[])
		           ELSE o.created_at < now() - ($2 * interval '1 day') END
		ORDER BY o.created_at
	`, pq.Array(orderIDs), *days)
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
	var cards []viberCard
	for _, o := range orders {
		links, err := reviewinvite.BuildLinks(db, siteURL, o.id, o.locale, *commit)
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
			if *commit {
				cards = append(cards, newViberCard(site.OrderNumber(o.id), o.name, o.phone, o.locale, links))
			}
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
		if err := reviewinvite.MarkDelivered(db, o.id); err != nil {
			fmt.Printf("    ! could not mark delivered: %v\n", err)
		} else {
			fmt.Println("    ✓ marked delivered")
		}
		fmt.Println()
	}

	if *htmlOut != "" && len(cards) > 0 {
		if err := writeViberPage(*htmlOut, cards); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Click-to-send page: %s  (open it in your browser)\n", *htmlOut)
	}
	if *commit {
		fmt.Printf("Done. %d sent/prepared, %d failed.\n", sent, failed)
	} else {
		fmt.Println("Dry-run only. Re-run with -commit to create tokens and send.")
	}
}

// printViber prints a ready-to-send Viber message for one order: the text, the
// customer's phone in international form, and viber:// links to open the chat
// or open Viber with the message pre-filled.
func printViber(name, phone, locale string, links []notify.ReviewLink, commit bool) {
	msg := reviewinvite.ViberMessage(strings.Split(name, " ")[0], locale, links)
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
	if p := reviewinvite.IntlPhone(phone); p != "" {
		fmt.Printf("    Phone:        %s\n", p)
		fmt.Printf("    Open chat:    %s\n", reviewinvite.ViberChatURL(p, msg))
	} else {
		fmt.Println("    Phone:        (none on the order)")
	}
	fmt.Printf("    Send message: %s\n", reviewinvite.ViberForwardURL(msg))
}

// viberCard is one customer on the click-to-send page.
type viberCard struct {
	Order, Name, Phone, Products, Message string
	ChatURL, ForwardURL                   template.URL // viber:// — trusted, built here
}

func newViberCard(order, name, phone, locale string, links []notify.ReviewLink) viberCard {
	msg := reviewinvite.ViberMessage(strings.Split(name, " ")[0], locale, links)
	var prods []string
	for _, l := range links {
		prods = append(prods, l.ProductName)
	}
	c := viberCard{Order: order, Name: name, Phone: reviewinvite.IntlPhone(phone), Products: strings.Join(prods, ", "),
		Message: msg, ForwardURL: template.URL(reviewinvite.ViberForwardURL(msg))}
	if c.Phone != "" {
		c.ChatURL = template.URL(reviewinvite.ViberChatURL(c.Phone, msg))
	}
	return c
}

var viberPage = template.Must(template.New("p").Parse(`<!doctype html>
<html lang="mk"><head><meta charset="utf-8"><title>Review invites — Viber</title>
<style>
 body{font:16px/1.5 system-ui,sans-serif;background:#f5f1e9;color:#1a1a1a;margin:0;padding:24px}
 h1{font-size:22px;margin:0 0 4px} p.hint{color:#555;margin:0 0 20px}
 .card{background:#fff;border:1px solid #e3ddd2;border-radius:12px;padding:16px 18px;margin:0 0 14px;max-width:760px}
 .card.done{opacity:.45}
 .who{font-weight:700} .meta{color:#555;font-size:14px}
 pre{white-space:pre-wrap;font:15px/1.5 system-ui,sans-serif;background:#faf8f4;border-radius:8px;padding:10px 12px;margin:10px 0}
 .btns{display:flex;gap:8px;flex-wrap:wrap}
 .btns a,.btns button{font:600 15px system-ui,sans-serif;padding:9px 14px;border-radius:8px;border:0;cursor:pointer;text-decoration:none}
 .copy{background:#e8e2d6;color:#1a1a1a} .chat{background:#7360f2;color:#fff} .fwd{background:#fff;color:#7360f2;border:2px solid #7360f2!important}
 label{margin-left:auto;font-size:14px;color:#555;align-self:center}
</style></head><body>
<h1>Review invites — Viber</h1>
<p class="hint">Per customer: <b>Open chat</b> (opens Viber on their number, message pre-filled if your Viber supports it — otherwise <b>Copy</b> and paste). Or <b>Send via Viber</b> and pick the contact. Tick ✓ when sent.</p>
{{range .}}<div class="card">
 <div class="who">{{.Order}} · {{.Name}}</div>
 <div class="meta">{{.Products}} · {{if .Phone}}{{.Phone}}{{else}}no phone on the order{{end}}</div>
 <pre>{{.Message}}</pre>
 <div class="btns">
  <button class="copy" type="button">📋 Copy</button>
  {{if .ChatURL}}<a class="chat" href="{{.ChatURL}}">💬 Open chat</a>{{end}}
  <a class="fwd" href="{{.ForwardURL}}">↗ Send via Viber</a>
  <label><input type="checkbox" class="sent"> sent</label>
 </div>
</div>
{{end}}
<script>
document.querySelectorAll('.card').forEach(c=>{
 c.querySelector('.copy').onclick=e=>{navigator.clipboard.writeText(c.querySelector('pre').textContent);e.target.textContent='✓ Copied'};
 c.querySelector('.sent').onchange=e=>c.classList.toggle('done',e.target.checked);
});
</script>
</body></html>
`))

// writeViberPage writes the click-to-send page for the prepared invites.
func writeViberPage(path string, cards []viberCard) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return viberPage.Execute(f, cards)
}

// parseIDs reads "-order 64,65 , 67" into [64 65 67]; "" → none.
func parseIDs(s string) ([]int64, error) {
	var ids []int64
	for _, f := range strings.Split(s, ",") {
		if f = strings.TrimSpace(f); f == "" {
			continue
		}
		n, err := strconv.ParseInt(f, 10, 64)
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("-order: %q is not an order id", f)
		}
		ids = append(ids, n)
	}
	return ids, nil
}
