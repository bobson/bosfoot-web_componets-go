package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

const csrfCookie = "_csrf"

// WithCSRFCookie ensures the _csrf cookie is present on every GET response.
// Apply it outside the page cache so both cache HITs and MISSes issue the
// cookie — the cache only stores the body, not Set-Cookie headers.
func WithCSRFCookie(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			ensureCSRFCookie(w, r)
		}
		next(w, r)
	}
}

// WrapCSRF wraps an http.HandlerFunc, rejecting state-changing requests whose
// X-CSRF-Token header doesn't match the _csrf cookie (double-submit pattern).
func WrapCSRF(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodDelete {
			cookie, err := r.Cookie(csrfCookie)
			if err != nil || cookie.Value == "" {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			if r.Header.Get("X-CSRF-Token") != cookie.Value {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
		}
		next(w, r)
	}
}

func ensureCSRFCookie(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(csrfCookie); err == nil && c.Value != "" {
		return
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookie,
		Value:    hex.EncodeToString(b),
		Path:     "/",
		HttpOnly: false, // JS must read this for the double-submit
		Secure:   r.Header.Get("X-Forwarded-Proto") == "https",
		SameSite: http.SameSiteStrictMode,
	})
}
