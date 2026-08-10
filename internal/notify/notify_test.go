package notify

import (
	"strings"
	"testing"
)

func TestBuildMessage(t *testing.T) {
	o := Order{
		ID: 42, Total: 12900, PaymentMethod: "cod",
		Name: "John Doe", Email: "john@example.com", Phone: "+38970123456",
		Address: "Main St 1", City: "Skopje", PostalCode: "1000",
		Notes: "leave at the door",
		Items: []Item{{Name: "Freet Chamois", Size: "42", Color: "Black", Qty: 1, Price: 12900}},
	}
	msg := string(buildMessage("shop@bosfoot.com", []string{"owner@bosfoot.com"}, o))

	wants := []string{
		"Subject: New Bosfoot order BF-1042 — 12 900 MKD",
		"Reply-To: john@example.com",
		"To: owner@bosfoot.com",
		"Content-Type: text/plain; charset=utf-8",
		"\r\n\r\n", // header/body separator
		"Order BF-1042",
		"Payment: Cash on delivery",
		"  - Freet Chamois — size 42, Black ×1 — 12 900 MKD",
		"Notes: leave at the door",
	}
	for _, w := range wants {
		if !strings.Contains(msg, w) {
			t.Errorf("message missing %q", w)
		}
	}
}

// A customer email carrying CRLF must not break out into an extra HEADER line.
// (The plain-text body may echo the raw value — that's harmless content, not a
// header; in production Order.Validate rejects such an email anyway.)
func TestBuildMessageNoHeaderInjection(t *testing.T) {
	o := Order{ID: 1, Email: "evil@x.com\r\nBcc: victim@y.com", Name: "x"}
	msg := string(buildMessage("a@b.com", []string{"c@d.com"}, o))
	headers, _, _ := strings.Cut(msg, "\r\n\r\n")
	if strings.Contains(headers, "\nBcc:") || strings.HasPrefix(headers, "Bcc:") {
		t.Errorf("CRLF in Reply-To injected a header line:\n%s", headers)
	}
}

func TestMKD(t *testing.T) {
	cases := map[int]string{0: "0", 999: "999", 1000: "1 000", 12900: "12 900", 1234567: "1 234 567"}
	for in, want := range cases {
		if got := mkd(in); got != want {
			t.Errorf("mkd(%d) = %q, want %q", in, got, want)
		}
	}
}
