package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRealIP(t *testing.T) {
	tests := []struct {
		name       string
		cfHeader   string
		remoteAddr string
		want       string // expected r.RemoteAddr seen by next
	}{
		{"cf header rewrites, keeps port shape", "203.0.113.7", "10.0.0.1:54321", "203.0.113.7:54321"},
		{"cf IPv6 is bracketed", "2001:db8::1", "10.0.0.1:443", "[2001:db8::1]:443"},
		{"no port on original falls back to :0", "203.0.113.7", "10.0.0.1", "203.0.113.7:0"},
		{"no cf header leaves RemoteAddr untouched", "", "10.0.0.1:54321", "10.0.0.1:54321"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got string
			h := RealIP(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got = r.RemoteAddr
			}))

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.cfHeader != "" {
				req.Header.Set("CF-Connecting-IP", tt.cfHeader)
			}
			h.ServeHTTP(httptest.NewRecorder(), req)

			if got != tt.want {
				t.Errorf("RemoteAddr = %q, want %q", got, tt.want)
			}
		})
	}
}
