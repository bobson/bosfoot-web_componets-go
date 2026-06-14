package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestRedirectTrailingSlash(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		wantStatus int
		wantLoc    string // Location on redirect; "" means expect pass-through
	}{
		{"locale home", "/en/", http.StatusMovedPermanently, "/en"},
		{"nested path", "/en/products/", http.StatusMovedPermanently, "/en/products"},
		{"already clean", "/en", http.StatusOK, ""},
		{"root untouched", "/", http.StatusOK, ""},
		{"query preserved", "/en/products/?brand=freet", http.StatusMovedPermanently, "/en/products?brand=freet"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ran bool
			h := RedirectTrailingSlash(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ran = true
				w.WriteHeader(http.StatusOK)
			}))
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rr.Code, tt.wantStatus)
			}
			if tt.wantLoc == "" {
				if !ran {
					t.Error("expected pass-through to next, but it didn't run")
				}
			} else {
				if ran {
					t.Error("expected a redirect, but next ran")
				}
				if loc := rr.Header().Get("Location"); loc != tt.wantLoc {
					t.Errorf("Location = %q, want %q", loc, tt.wantLoc)
				}
			}
		})
	}
}

// A trailing-slash path that maps to a real public/ directory must pass through,
// not redirect — otherwise it loops against http.FileServer's dir → dir/ redirect.
func TestRedirectTrailingSlashSkipsRealDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "public", "css"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	var ran bool
	h := RedirectTrailingSlash(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { ran = true }))
	req := httptest.NewRequest(http.MethodGet, "/css/", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if !ran {
		t.Errorf("expected /css/ to pass through (real dir), but got %d → %q",
			rr.Code, rr.Header().Get("Location"))
	}
}
