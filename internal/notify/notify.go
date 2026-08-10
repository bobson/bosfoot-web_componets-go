// Package notify sends the shop owner an email when an order is placed. It is
// deliberately best-effort: the email is sent in the background after the order
// is already committed, so a mail outage never blocks or fails an order — the
// worst case is a logged error (and the order is still in the DB + the "Order
// placed" INFO log the handler writes as a durable second channel).
//
// Transport is plain SMTP (stdlib), so any relay works — Brevo, Mailgun, your
// host's SMTP, etc. Configure via env; if unset, notifications stay disabled:
//
//	SMTP_HOST, SMTP_PORT (use 587 / STARTTLS), SMTP_USER, SMTP_PASS,
//	SMTP_FROM, ORDER_NOTIFY_TO (comma-separated for multiple recipients)
package notify

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"os"
	"strconv"
	"strings"
	"time"

	"bosfoot/internal/site"
	"bosfoot/logger"
)

// Item is one line of an order, for the email.
type Item struct {
	Name  string
	Size  string
	Color string
	Qty   int
	Price int // unit price, MKD
}

// Order is the data the notification email needs.
type Order struct {
	ID            int
	Total         int // MKD — grand total (goods + shipping)
	Shipping      int // MKD — delivery fee; 0 when free. Goods subtotal = Total − Shipping.
	PaymentMethod string
	Name          string
	Email         string
	Phone         string
	Address       string
	City          string
	PostalCode    string
	Notes         string
	Locale        string // mk | sq | en
	Items         []Item
}

// Mailer sends order notifications. The zero-value / disabled Mailer is a no-op.
type Mailer struct {
	addr string // host:port
	host string // for STARTTLS ServerName + auth
	auth smtp.Auth
	from string
	to   []string
	log  *logger.Logger
	on   bool
}

// New builds a Mailer from the SMTP_* / ORDER_NOTIFY_TO env vars. If the
// required ones are missing it returns a disabled Mailer (OrderPlaced no-ops),
// so dev and not-yet-configured prod run fine.
func New(log *logger.Logger) *Mailer {
	host := os.Getenv("SMTP_HOST")
	port := os.Getenv("SMTP_PORT")
	from := os.Getenv("SMTP_FROM")
	toRaw := os.Getenv("ORDER_NOTIFY_TO")

	if host == "" || port == "" || from == "" || toRaw == "" {
		log.Info("Order notifications disabled (set SMTP_HOST, SMTP_PORT, SMTP_FROM, ORDER_NOTIFY_TO to enable)")
		return &Mailer{log: log}
	}
	if port == "465" {
		// net/smtp does STARTTLS after a plaintext dial; 465 expects TLS on
		// connect and would fail confusingly. Tell the operator to use 587.
		log.Info("Order notifications disabled: SMTP_PORT 465 (implicit TLS) is unsupported — use 587 (STARTTLS)")
		return &Mailer{log: log}
	}

	var to []string
	for _, t := range strings.Split(toRaw, ",") {
		if t = strings.TrimSpace(t); t != "" {
			to = append(to, t)
		}
	}

	var auth smtp.Auth
	if user := os.Getenv("SMTP_USER"); user != "" {
		auth = smtp.PlainAuth("", user, os.Getenv("SMTP_PASS"), host)
	}

	log.Info("Order notifications enabled", "to", strings.Join(to, ","))
	return &Mailer{
		addr: net.JoinHostPort(host, port),
		host: host,
		auth: auth,
		from: from,
		to:   to,
		log:  log,
		on:   true,
	}
}

// OrderPlaced emails the owner about a new order. It returns immediately; the
// send runs in the background and can only ever log an error.
func (m *Mailer) OrderPlaced(o Order) {
	if m == nil || !m.on {
		return
	}
	go func() {
		defer m.recover("Owner notification", o.ID)
		if err := m.send(m.to, buildMessage(m.from, m.to, o)); err != nil {
			m.log.Error("Order notification failed to send", err, "order_id", o.ID)
			return
		}
		m.log.Info("Order notification sent to owner", "order_id", o.ID)
	}()
}

// ReviewNotice is what the owner "new pending review" email needs.
type ReviewNotice struct {
	ProductID   int
	ProductName string
	OrderID     int
	Rating      int
	AuthorName  string
	Body        string
	Locale      string // mk | sq | en
}

// PendingReview emails the owner that a review is awaiting moderation. Like
// OrderPlaced it returns immediately; the send runs in the background and can
// only ever log an error. Owner-only — the reviewer is not emailed.
func (m *Mailer) PendingReview(n ReviewNotice) {
	if m == nil || !m.on {
		return
	}
	go func() {
		defer m.recover("Pending review notification", n.OrderID)
		if err := m.send(m.to, buildReviewNoticeMessage(m.from, m.to, n)); err != nil {
			m.log.Error("Pending review notification failed to send", err, "product_id", n.ProductID)
			return
		}
		m.log.Info("Pending review notification sent to owner", "product_id", n.ProductID)
	}()
}

// buildReviewNoticeMessage renders the owner-facing "new pending review" email.
// English (owner-only), with a reminder of the moderation command.
func buildReviewNoticeMessage(from string, to []string, n ReviewNotice) []byte {
	var b strings.Builder
	header := func(k, v string) {
		v = strings.NewReplacer("\r", "", "\n", "").Replace(v)
		fmt.Fprintf(&b, "%s: %s\r\n", k, v)
	}
	header("From", from)
	header("To", strings.Join(to, ", "))
	header("Subject", fmt.Sprintf("New pending review: %s (%d/5) — Bosfoot", n.ProductName, n.Rating))
	header("MIME-Version", "1.0")
	header("Content-Type", "text/plain; charset=utf-8")
	b.WriteString("\r\n")

	line := func(format string, args ...any) { fmt.Fprintf(&b, format+"\r\n", args...) }
	line("A new review is awaiting moderation.")
	line("")
	line("Product: %s (#%d)", n.ProductName, n.ProductID)
	line("Order:   %s", site.OrderNumber(n.OrderID))
	line("Rating:  %d/5", n.Rating)
	line("Author:  %s", n.AuthorName)
	line("Lang:    %s", n.Locale)
	if n.Body != "" {
		line("")
		line("%s", n.Body)
	}
	line("")
	line("List pending:  go run ./cmd/reviews")
	line("Approve:       go run ./cmd/reviews -approve <id>")
	line("Reject:        go run ./cmd/reviews -reject <id>")
	return []byte(b.String())
}

// CustomerConfirmation emails the customer a "Thank you" with their order details.
func (m *Mailer) CustomerConfirmation(o Order) {
	if m == nil || !m.on || o.Email == "" {
		return
	}
	go func() {
		defer m.recover("Customer confirmation", o.ID)
		if err := m.send([]string{o.Email}, buildCustomerMessage(m.from, strings.Join(m.to, ", "), o)); err != nil {
			m.log.Error("Customer confirmation failed to send", err, "order_id", o.ID)
			return
		}
		m.log.Info("Customer confirmation sent", "order_id", o.ID, "to", o.Email)
	}()
}

// Enabled reports whether SMTP is configured. cmd/reviewinvites uses this to
// decide whether to actually send or just print the links.
func (m *Mailer) Enabled() bool { return m != nil && m.on }

// ReviewLink is one product's review URL in an invite email.
type ReviewLink struct {
	ProductName string
	URL         string
}

// ReviewInviteData is what a post-purchase review invite needs. One email per
// order, with a link per purchased product.
type ReviewInviteData struct {
	OrderID int
	Email   string
	Name    string
	Locale  string // mk | sq | en
	Links   []ReviewLink
}

// ReviewInvite sends the post-purchase "how are your shoes? leave a review"
// email. Unlike OrderPlaced it is synchronous and returns an error, so the batch
// tool (cmd/reviewinvites) can report per-order success and only mark an order
// invited when the send actually succeeded.
func (m *Mailer) ReviewInvite(d ReviewInviteData) error {
	if !m.Enabled() {
		return fmt.Errorf("mailer disabled")
	}
	if d.Email == "" || len(d.Links) == 0 {
		return fmt.Errorf("nothing to send")
	}
	return m.send([]string{d.Email}, buildReviewInviteMessage(m.from, strings.Join(m.to, ", "), d))
}

// ReviewApprovedData is what the reviewer "your review is live" thank-you needs.
type ReviewApprovedData struct {
	Email       string
	Name        string
	Locale      string // mk | sq | en
	ProductName string
}

// ReviewApproved sends the reviewer a localized "thanks, your review is now
// live" email when the owner approves it. Synchronous + returns an error like
// ReviewInvite, so cmd/reviews can report success. Sent to the reviewer, with
// Reply-To the owner inbox.
func (m *Mailer) ReviewApproved(d ReviewApprovedData) error {
	if !m.Enabled() {
		return fmt.Errorf("mailer disabled")
	}
	if d.Email == "" {
		return fmt.Errorf("no recipient email")
	}
	return m.send([]string{d.Email}, buildReviewApprovedMessage(m.from, strings.Join(m.to, ", "), d))
}

// buildReviewApprovedMessage renders the reviewer thank-you, localised by the
// review's locale (same approach as buildReviewInviteMessage).
func buildReviewApprovedMessage(from, replyTo string, d ReviewApprovedData) []byte {
	var b strings.Builder
	header := func(k, v string) {
		v = strings.NewReplacer("\r", "", "\n", "").Replace(v)
		fmt.Fprintf(&b, "%s: %s\r\n", k, v)
	}

	firstName := strings.Split(d.Name, " ")[0]
	var subject, hello, intro, footer, regards, team string
	switch d.Locale {
	case "mk":
		subject = "Вашата оценка е објавена — Bosfoot"
		hello = "Здраво " + firstName + ","
		intro = "Ви благодариме за вашата оценка за " + d.ProductName + "! Веќе е објавена на нашата страница и им помага на другите да ја изберат вистинската големина."
		footer = "Ви благодариме што сте дел од Bosfoot."
		regards = "Со почит,"
		team = "Тимот на Bosfoot"
	case "sq":
		subject = "Vlerësimi juaj u publikua — Bosfoot"
		hello = "Përshëndetje " + firstName + ","
		intro = "Faleminderit për vlerësimin tuaj për " + d.ProductName + "! Tashmë është publikuar në faqen tonë dhe ndihmon të tjerët të zgjedhin madhësinë e duhur."
		footer = "Faleminderit që jeni pjesë e Bosfoot."
		regards = "Gjithë të mirat,"
		team = "Ekipi i Bosfoot"
	default: // en
		subject = "Your review is live — Bosfoot"
		hello = "Hello " + firstName + ","
		intro = "Thank you for your review of " + d.ProductName + "! It's now published on our site and helps others pick the right size."
		footer = "Thank you for being part of Bosfoot."
		regards = "Best regards,"
		team = "The Bosfoot Team"
	}

	header("From", from)
	header("To", d.Email)
	if replyTo != "" {
		header("Reply-To", replyTo)
	}
	header("Subject", subject)
	header("MIME-Version", "1.0")
	header("Content-Type", "text/plain; charset=utf-8")
	b.WriteString("\r\n")

	line := func(format string, args ...any) { fmt.Fprintf(&b, format+"\r\n", args...) }
	line(hello)
	line("")
	line(intro)
	line("")
	line(footer)
	line("")
	line(regards)
	line(team)
	return []byte(b.String())
}

// buildReviewInviteMessage renders the RFC 5322 review-invite email, localised
// by the order's locale (same approach as buildCustomerMessage).
func buildReviewInviteMessage(from, replyTo string, d ReviewInviteData) []byte {
	var b strings.Builder
	header := func(k, v string) {
		v = strings.NewReplacer("\r", "", "\n", "").Replace(v)
		fmt.Fprintf(&b, "%s: %s\r\n", k, v)
	}

	firstName := strings.Split(d.Name, " ")[0]
	var subject, hello, intro, cta, footer, regards, team string
	switch d.Locale {
	case "mk":
		subject = "Како се чувствуваат вашите нови патики? — Bosfoot"
		hello = "Здраво " + firstName + ","
		intro = "Се надеваме дека уживате во вашите нови барефут патики! Вашето искуство им помага на другите да ја изберат вистинската големина. Ќе ни значи многу ако одвоите минута за да оставите оценка:"
		cta = "Оцени:"
		footer = "Ви благодариме што сте дел од Bosfoot."
		regards = "Со почит,"
		team = "Тимот на Bosfoot"
	case "sq":
		subject = "Si ndihen këpucët tuaja të reja? — Bosfoot"
		hello = "Përshëndetje " + firstName + ","
		intro = "Shpresojmë që po i shijoni këpucët tuaja të reja barefoot! Përvoja juaj i ndihmon të tjerët të zgjedhin madhësinë e duhur. Do të na pëlqente shumë nëse lini një vlerësim:"
		cta = "Vlerëso:"
		footer = "Faleminderit që jeni pjesë e Bosfoot."
		regards = "Gjithë të mirat,"
		team = "Ekipi i Bosfoot"
	default: // en
		subject = "How are your new shoes feeling? — Bosfoot"
		hello = "Hello " + firstName + ","
		intro = "We hope you're enjoying your new barefoot shoes! Your experience helps others pick the right size. We'd love it if you left a quick review:"
		cta = "Review:"
		footer = "Thank you for being part of Bosfoot."
		regards = "Best regards,"
		team = "The Bosfoot Team"
	}

	header("From", from)
	header("To", d.Email)
	if replyTo != "" {
		header("Reply-To", replyTo)
	}
	header("Subject", subject)
	header("MIME-Version", "1.0")
	header("Content-Type", "text/plain; charset=utf-8")
	b.WriteString("\r\n")

	line := func(format string, args ...any) { fmt.Fprintf(&b, format+"\r\n", args...) }
	line(hello)
	line("")
	line(intro)
	line("")
	for _, l := range d.Links {
		line("%s %s", cta, l.ProductName)
		line("  %s", l.URL)
		line("")
	}
	line(footer)
	line("")
	line(regards)
	line(team)
	return []byte(b.String())
}

func (m *Mailer) recover(label string, orderID int) {
	if r := recover(); r != nil {
		m.log.Error(label+" panicked", fmt.Errorf("%v", r), "order_id", orderID)
	}
}

// send delivers one message over SMTP+STARTTLS with a hard deadline.
func (m *Mailer) send(to []string, msg []byte) error {
	conn, err := net.DialTimeout("tcp", m.addr, 10*time.Second)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(20 * time.Second))

	c, err := smtp.NewClient(conn, m.host)
	if err != nil {
		return err
	}
	defer c.Close()

	if ok, _ := c.Extension("STARTTLS"); ok {
		if err := c.StartTLS(&tls.Config{ServerName: m.host}); err != nil {
			return fmt.Errorf("starttls: %w", err)
		}
	}
	if m.auth != nil {
		if err := c.Auth(m.auth); err != nil {
			return fmt.Errorf("auth: %w", err)
		}
	}
	if err := c.Mail(m.from); err != nil {
		return err
	}
	for _, rcpt := range to {
		if err := c.Rcpt(rcpt); err != nil {
			return err
		}
	}
	wc, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := wc.Write(msg); err != nil {
		return err
	}
	if err := wc.Close(); err != nil {
		return err
	}
	return c.Quit()
}

// buildMessage renders the RFC 5322 message for the owner.
func buildMessage(from string, to []string, o Order) []byte {
	var b strings.Builder
	header := func(k, v string) {
		v = strings.NewReplacer("\r", "", "\n", "").Replace(v)
		fmt.Fprintf(&b, "%s: %s\r\n", k, v)
	}
	header("From", from)
	header("To", strings.Join(to, ", "))
	if o.Email != "" {
		header("Reply-To", o.Email)
	}
	header("Subject", fmt.Sprintf("New Bosfoot order %s — %s MKD", site.OrderNumber(o.ID), mkd(o.Total)))
	header("MIME-Version", "1.0")
	header("Content-Type", "text/plain; charset=utf-8")
	b.WriteString("\r\n")

	line := func(format string, args ...any) { fmt.Fprintf(&b, format+"\r\n", args...) }
	line("Order %s", site.OrderNumber(o.ID))
	if o.Shipping > 0 {
		line("Subtotal: %s MKD", mkd(o.Total-o.Shipping))
		line("Shipping: %s MKD", mkd(o.Shipping))
	}
	line("Total: %s MKD", mkd(o.Total))
	line("Payment: %s", paymentLabel(o.PaymentMethod))
	line("")
	line("Customer")
	line("  Name:  %s", o.Name)
	line("  Email: %s", o.Email)
	if o.Phone != "" {
		line("  Phone: %s", o.Phone)
	}
	line("  Ship:  %s, %s %s", o.Address, o.City, o.PostalCode)
	line("")
	line("Items")
	for _, it := range o.Items {
		variant := ""
		if it.Size != "" || it.Color != "" {
			variant = fmt.Sprintf(" — size %s, %s", it.Size, it.Color)
		}
		line("  - %s%s ×%d — %s MKD", it.Name, variant, it.Qty, mkd(it.Price*it.Qty))
	}
	if o.Notes != "" {
		line("")
		line("Notes: %s", o.Notes)
	}
	return []byte(b.String())
}

// buildCustomerMessage renders the RFC 5322 message for the customer. replyTo is
// the monitored order inbox, so a customer's reply ("reply to this email") lands
// somewhere a human reads rather than the unattended From address.
func buildCustomerMessage(from, replyTo string, o Order) []byte {
	var b strings.Builder
	header := func(k, v string) {
		v = strings.NewReplacer("\r", "", "\n", "").Replace(v)
		fmt.Fprintf(&b, "%s: %s\r\n", k, v)
	}

	// Localized content
	var (
		subject string
		hello   string
		thanks  string
		details string
		ordNum  string
		subtotL string
		shipL   string
		totalL  string
		paymntL string
		shipAdd string
		itemsL  string
		footer  string
		regards string
		team    string
	)

	firstName := strings.Split(o.Name, " ")[0]

	switch o.Locale {
	case "mk":
		subject = fmt.Sprintf("Потврда за нарачка %s — Bosfoot", site.OrderNumber(o.ID))
		hello = "Здраво " + firstName + ","
		thanks = "Ви благодариме за вашата нарачка во Bosfoot! Ја примивме и ќе ве контактираме за да ја потврдиме. Плаќате при испорака."
		details = "Детали за нарачката"
		ordNum = "Број на нарачка:"
		subtotL = "Меѓузбир:"
		shipL = "Достава:"
		totalL = "Вкупно:"
		paymntL = "Начин на плаќање:"
		shipAdd = "Адреса за испорака:"
		itemsL = "Производи:"
		footer = "Ако имате било какви прашања, едноставно одговорете на овој е-маил или контактирајте нè на info@bosfoot.com."
		regards = "Со почит,"
		team = "Тимот на Bosfoot"
	case "sq":
		subject = fmt.Sprintf("Konfirmimi i porosisë %s — Bosfoot", site.OrderNumber(o.ID))
		hello = "Përshëndetje " + firstName + ","
		thanks = "Faleminderit për porosinë tuaj në Bosfoot! E morëm dhe do t'ju kontaktojmë për ta konfirmuar. Paguani me dorëzim."
		details = "Detajet e porosisë"
		ordNum = "Numri i porosisë:"
		subtotL = "Nëntotali:"
		shipL = "Dërgesa:"
		totalL = "Total:"
		paymntL = "Metoda e pagesës:"
		shipAdd = "Adresa e dërgesës:"
		itemsL = "Produktet:"
		footer = "Nëse keni ndonjë pyetje, thjesht përgjigjuni këtij emaili ose na kontaktoni në info@bosfoot.com."
		regards = "Gjithë të mirat,"
		team = "Ekipi i Bosfoot"
	default: // en
		subject = fmt.Sprintf("Order Confirmation %s — Bosfoot", site.OrderNumber(o.ID))
		hello = "Hello " + firstName + ","
		thanks = "Thank you for your order at Bosfoot! We've received it and will contact you to confirm. You pay on delivery."
		details = "Order Details"
		ordNum = "Order Number:"
		subtotL = "Subtotal:"
		shipL = "Shipping:"
		totalL = "Total:"
		paymntL = "Payment:"
		shipAdd = "Shipping Address:"
		itemsL = "Items:"
		footer = "If you have any questions, simply reply to this email or contact us at info@bosfoot.com."
		regards = "Best regards,"
		team = "The Bosfoot Team"
	}

	header("From", from)
	header("To", o.Email)
	if replyTo != "" {
		header("Reply-To", replyTo)
	}
	header("Subject", subject)
	header("MIME-Version", "1.0")
	header("Content-Type", "text/plain; charset=utf-8")
	b.WriteString("\r\n")

	line := func(format string, args ...any) { fmt.Fprintf(&b, format+"\r\n", args...) }
	line(hello)
	line("")
	line(thanks)
	line("")
	line(details)
	line(strings.Repeat("-", len(details)))
	line("%-15s %s", ordNum, site.OrderNumber(o.ID))
	if o.Shipping > 0 {
		line("%-15s %s MKD", subtotL, mkd(o.Total-o.Shipping))
		line("%-15s %s MKD", shipL, mkd(o.Shipping))
	}
	line("%-15s %s MKD", totalL, mkd(o.Total))
	line("%-15s %s", paymntL, paymentLabelLoc(o.PaymentMethod, o.Locale))
	line("")
	line(shipAdd)
	line("%s", o.Name)
	line("%s", o.Address)
	line("%s %s", o.City, o.PostalCode)
	line("")
	line(itemsL)
	for _, it := range o.Items {
		variant := ""
		if it.Size != "" || it.Color != "" {
			switch o.Locale {
			case "mk":
				variant = fmt.Sprintf(" (големина: %s, боја: %s)", it.Size, it.Color)
			case "sq":
				variant = fmt.Sprintf(" (madhësia: %s, ngjyra: %s)", it.Size, it.Color)
			default:
				variant = fmt.Sprintf(" (size: %s, color: %s)", it.Size, it.Color)
			}
		}
		line("- %s%s x%d", it.Name, variant, it.Qty)
	}
	line("")
	line(footer)
	line("")
	line(regards)
	line(team)
	return []byte(b.String())
}

func paymentLabelLoc(m, locale string) string {
	switch locale {
	case "mk":
		if m == "cod" {
			return "Плаќање при преземање"
		}
		return "Банкарски трансфер"
	case "sq":
		if m == "cod" {
			return "Pagesa pas pranimit"
		}
		return "Transfer bankar"
	default:
		if m == "cod" {
			return "Cash on delivery"
		}
		return "Bank transfer"
	}
}

func paymentLabel(m string) string {
	switch m {
	case "cod":
		return "Cash on delivery"
	case "bank_transfer":
		return "Bank transfer"
	default:
		return m
	}
}

// mkd formats an integer with space thousands separators, matching the site.
func mkd(n int) string {
	s := strconv.Itoa(n)
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	var out strings.Builder
	for i := 0; i < len(s); i++ {
		if i > 0 && (len(s)-i)%3 == 0 {
			out.WriteByte(' ')
		}
		out.WriteByte(s[i])
	}
	if neg {
		return "-" + out.String()
	}
	return out.String()
}
