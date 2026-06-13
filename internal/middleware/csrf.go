package middleware

import (
	"net/http"
)

// CSRFMiddleware validates a custom CSRF token in the header.
func CSRFMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only protect state-changing methods
		if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodDelete {
			token := r.Header.Get("X-CSRF-Token")
			// In a real app, validate against a session or cookie-based secret.
			// For this prototype, we'll implement a basic check against a server-side secret.
			if !isValidToken(token) {
				http.Error(w, "Forbidden: Invalid CSRF token", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// Placeholder for real token validation
func isValidToken(token string) bool {
    // Should compare against a secure, unique, per-session token.
	return token == "expected-secret-token" 
}
