// Package tmpl parses and caches HTML templates, and exposes a Render helper.
package tmpl

import (
	"fmt"
	"html/template"
	"math"
	"net/http"
	"path/filepath"
	"strings"

	"bosfoot/internal/locale"
)

// Renderer parses all templates at startup and caches them.
type Renderer struct {
	tmpl *template.Template
}

// NewRenderer parses all templates under dir/partials/*.html and dir/pages/*.html.
// The provided UI is used by the t() template function.
func NewRenderer(dir string, ui *locale.UI) (*Renderer, error) {
	funcMap := template.FuncMap{
		// t translates a key for the given locale: {{t .Locale "nav.products"}}
		"t": ui.T,

		// mkd formats an integer MKD price with space thousands separator: 6200 → "6 200"
		"mkd": formatMKD,

		// eur converts MKD to EUR and formats with 2 decimals: 6200 → "100.81"
		"eur": func(amount int) string {
			return fmt.Sprintf("%.2f", math.Round(float64(amount)/61.5*100)/100)
		},

		// showEUR returns true for locales that display EUR alongside MKD.
		"showEUR": func(loc string) bool {
			return loc == locale.SQ || loc == locale.EN
		},

		// deref dereferences a *string safely, returning "" for nil.
		"deref": func(s *string) string {
			if s == nil {
				return ""
			}
			return *s
		},

		// dict builds a map from alternating key/value pairs.
		// Used to pass multiple values into a sub-template:
		//   {{template "product_card" (dict "Product" . "Locale" $.Locale)}}
		"dict": func(pairs ...any) (map[string]any, error) {
			if len(pairs)%2 != 0 {
				return nil, fmt.Errorf("dict requires an even number of arguments")
			}
			m := make(map[string]any, len(pairs)/2)
			for i := 0; i < len(pairs); i += 2 {
				key, ok := pairs[i].(string)
				if !ok {
					return nil, fmt.Errorf("dict keys must be strings")
				}
				m[key] = pairs[i+1]
			}
			return m, nil
		},
	}

	tmpl := template.New("").Funcs(funcMap)

	// Parse partials first so page templates can reference them by name.
	for _, pattern := range []string{
		filepath.Join(dir, "partials", "*.html"),
		filepath.Join(dir, "pages", "*.html"),
	} {
		if _, err := tmpl.ParseGlob(pattern); err != nil {
			return nil, fmt.Errorf("parse %s: %w", pattern, err)
		}
	}

	return &Renderer{tmpl: tmpl}, nil
}

// Render executes the named template and writes to w.
// On error it returns a 500 — the caller should not write anything after this.
func (r *Renderer) Render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := r.tmpl.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func formatMKD(amount int) string {
	s := fmt.Sprintf("%d", amount)
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(' ')
		}
		b.WriteRune(c)
	}
	return b.String()
}
