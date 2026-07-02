package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLocaleRedirectDest(t *testing.T) {
	cases := []struct {
		name   string
		cookie string // "" = no cookie set
		want   string
	}{
		{"no cookie defaults to mk", "", "/mk"},
		{"valid sq", "sq", "/sq"},
		{"valid en", "en", "/en"},
		{"valid mk", "mk", "/mk"},
		{"invalid locale falls back to mk", "xx", "/mk"},
		{"open-redirect attempt is rejected", "/evil.com", "/mk"},
		{"empty value falls back to mk", "%20", "/mk"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			if c.cookie != "" {
				r.AddCookie(&http.Cookie{Name: "bosfoot_locale", Value: c.cookie})
			}
			if got := localeRedirectDest(r); got != c.want {
				t.Errorf("cookie=%q: got %q, want %q", c.cookie, got, c.want)
			}
		})
	}
}
