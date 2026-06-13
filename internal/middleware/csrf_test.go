package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// recordingNext returns a handler that flips *ran true and writes 200, so tests
// can assert whether the wrapped handler was actually reached.
func recordingNext(ran *bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		*ran = true
		w.WriteHeader(http.StatusOK)
	}
}

func getCSRFCookie(cookies []*http.Cookie) *http.Cookie {
	for _, c := range cookies {
		if c.Name == csrfCookie {
			return c
		}
	}
	return nil
}

func TestWrapCSRF(t *testing.T) {
	const token = "deadbeefcafe"
	tests := []struct {
		name       string
		method     string
		setCookie  bool
		cookie     string
		header     string
		wantStatus int
		wantNext   bool
	}{
		{"GET passes unchecked", http.MethodGet, false, "", "", http.StatusOK, true},
		{"HEAD passes unchecked", http.MethodHead, false, "", "", http.StatusOK, true},
		{"POST matching token passes", http.MethodPost, true, token, token, http.StatusOK, true},
		{"PUT matching token passes", http.MethodPut, true, token, token, http.StatusOK, true},
		{"DELETE matching token passes", http.MethodDelete, true, token, token, http.StatusOK, true},
		{"POST mismatched token rejected", http.MethodPost, true, token, "wrong", http.StatusForbidden, false},
		{"POST missing cookie rejected", http.MethodPost, false, "", token, http.StatusForbidden, false},
		{"POST empty cookie rejected", http.MethodPost, true, "", "", http.StatusForbidden, false},
		{"POST missing header rejected", http.MethodPost, true, token, "", http.StatusForbidden, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ran bool
			h := WrapCSRF(recordingNext(&ran))

			req := httptest.NewRequest(tt.method, "/api/orders", nil)
			if tt.setCookie {
				req.AddCookie(&http.Cookie{Name: csrfCookie, Value: tt.cookie})
			}
			if tt.header != "" {
				req.Header.Set("X-CSRF-Token", tt.header)
			}
			rr := httptest.NewRecorder()
			h(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rr.Code, tt.wantStatus)
			}
			if ran != tt.wantNext {
				t.Errorf("next ran = %v, want %v", ran, tt.wantNext)
			}
		})
	}
}

func TestWithCSRFCookieSetsCookieOnGet(t *testing.T) {
	var ran bool
	h := WithCSRFCookie(recordingNext(&ran))
	req := httptest.NewRequest(http.MethodGet, "/mk/checkout", nil)
	rr := httptest.NewRecorder()
	h(rr, req)

	if !ran {
		t.Fatal("next did not run")
	}
	c := getCSRFCookie(rr.Result().Cookies())
	if c == nil {
		t.Fatal("expected _csrf cookie to be set")
	}
	if len(c.Value) != 64 { // 32 random bytes, hex-encoded
		t.Errorf("token length = %d, want 64", len(c.Value))
	}
	if c.HttpOnly {
		t.Error("cookie must not be HttpOnly — the JS reads it for double-submit")
	}
	if c.SameSite != http.SameSiteStrictMode {
		t.Errorf("SameSite = %v, want Strict", c.SameSite)
	}
	if c.Path != "/" {
		t.Errorf("Path = %q, want /", c.Path)
	}
}

func TestWithCSRFCookiePreservesExisting(t *testing.T) {
	var ran bool
	h := WithCSRFCookie(recordingNext(&ran))
	req := httptest.NewRequest(http.MethodGet, "/mk/checkout", nil)
	req.AddCookie(&http.Cookie{Name: csrfCookie, Value: "existing-token"})
	rr := httptest.NewRecorder()
	h(rr, req)

	if c := getCSRFCookie(rr.Result().Cookies()); c != nil {
		t.Errorf("expected no new cookie when one already exists, got %q", c.Value)
	}
}

func TestWithCSRFCookieIgnoresNonGet(t *testing.T) {
	var ran bool
	h := WithCSRFCookie(recordingNext(&ran))
	req := httptest.NewRequest(http.MethodPost, "/api/orders", nil)
	rr := httptest.NewRecorder()
	h(rr, req)

	if !ran {
		t.Fatal("next did not run")
	}
	if c := getCSRFCookie(rr.Result().Cookies()); c != nil {
		t.Error("POST should not set a _csrf cookie")
	}
}

func TestWithCSRFCookieSecureFlag(t *testing.T) {
	tests := []struct {
		name       string
		proto      string
		wantSecure bool
	}{
		{"https sets Secure", "https", true},
		{"plain http not Secure", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ran bool
			h := WithCSRFCookie(recordingNext(&ran))
			req := httptest.NewRequest(http.MethodGet, "/mk/checkout", nil)
			if tt.proto != "" {
				req.Header.Set("X-Forwarded-Proto", tt.proto)
			}
			rr := httptest.NewRecorder()
			h(rr, req)

			c := getCSRFCookie(rr.Result().Cookies())
			if c == nil {
				t.Fatal("expected _csrf cookie")
			}
			if c.Secure != tt.wantSecure {
				t.Errorf("Secure = %v, want %v", c.Secure, tt.wantSecure)
			}
		})
	}
}

// TestCSRFDoubleSubmitRoundTrip ties both halves together: the cookie issued by
// WithCSRFCookie on a GET must satisfy WrapCSRF on a POST that echoes it as the
// X-CSRF-Token header.
func TestCSRFDoubleSubmitRoundTrip(t *testing.T) {
	getH := WithCSRFCookie(func(w http.ResponseWriter, r *http.Request) {})
	getReq := httptest.NewRequest(http.MethodGet, "/mk/checkout", nil)
	getRR := httptest.NewRecorder()
	getH(getRR, getReq)

	c := getCSRFCookie(getRR.Result().Cookies())
	if c == nil {
		t.Fatal("no _csrf cookie issued on GET")
	}

	var ran bool
	postH := WrapCSRF(recordingNext(&ran))
	postReq := httptest.NewRequest(http.MethodPost, "/api/orders", nil)
	postReq.AddCookie(&http.Cookie{Name: csrfCookie, Value: c.Value})
	postReq.Header.Set("X-CSRF-Token", c.Value)
	postRR := httptest.NewRecorder()
	postH(postRR, postReq)

	if postRR.Code != http.StatusOK {
		t.Errorf("round-trip status = %d, want 200", postRR.Code)
	}
	if !ran {
		t.Error("next did not run on a valid double-submit")
	}
}
