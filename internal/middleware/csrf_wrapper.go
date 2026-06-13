package middleware

import "net/http"

// WrapCSRF wraps a handler function with CSRF protection.
func WrapCSRF(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodDelete {
            token := r.Header.Get("X-CSRF-Token")
            if token != "expected-secret-token" {
                http.Error(w, "Forbidden: Invalid CSRF token", http.StatusForbidden)
                return
            }
        }
        next(w, r)
    }
}
