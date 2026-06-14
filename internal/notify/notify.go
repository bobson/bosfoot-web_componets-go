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
	Total         int // MKD
	PaymentMethod string
	Name          string
	Email         string
	Phone         string
	Address       string
	City          string
	PostalCode    string
	Notes         string
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
// send runs in the background and can only ever log an error — never block or
// fail the order. The order is passed by value (no shared/request state).
func (m *Mailer) OrderPlaced(o Order) {
	if m == nil || !m.on {
		return
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				m.log.Error("Order notification panicked", fmt.Errorf("%v", r), "order_id", o.ID)
			}
		}()
		if err := m.send(o); err != nil {
			m.log.Error("Order notification failed to send", err, "order_id", o.ID)
			return
		}
		m.log.Info("Order notification sent", "order_id", o.ID)
	}()
}

// send delivers one message over SMTP+STARTTLS with a hard deadline so a hung
// server can't leak the goroutine.
func (m *Mailer) send(o Order) error {
	msg := buildMessage(m.from, m.to, o)

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
	for _, rcpt := range m.to {
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

// buildMessage renders the RFC 5322 message. All header values are CRLF-stripped
// so no interpolated field (notably Reply-To, which carries the customer email)
// can inject extra headers.
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
	header("Subject", fmt.Sprintf("New Bosfoot order #%d — %s MKD", o.ID, mkd(o.Total)))
	header("MIME-Version", "1.0")
	header("Content-Type", "text/plain; charset=utf-8")
	b.WriteString("\r\n") // end of headers

	line := func(format string, args ...any) { fmt.Fprintf(&b, format+"\r\n", args...) }
	line("Order #%d", o.ID)
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
