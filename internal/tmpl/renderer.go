// Package tmpl parses and caches HTML templates, and exposes a Render helper.
package tmpl

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"bosfoot/internal/locale"
)

// Renderer parses all templates at startup and caches them.
type Renderer struct {
	tmpl   *template.Template
	hashes sync.Map // path -> hash string
}

// NewRenderer parses all templates under dir/partials/*.html and dir/pages/*.html.
// The provided UI is used by the t() template function.
func NewRenderer(dir string, ui *locale.UI) (*Renderer, error) {
	r := &Renderer{}
	funcMap := template.FuncMap{
		// asset appends a content hash to a public asset path: /styles.css -> /styles.css?v=abcdef
		"asset": func(path string) string {
			fullPath := filepath.Join("public", strings.TrimPrefix(path, "/"))
			info, err := os.Stat(fullPath)
			if err != nil {
				log.Printf("Asset not found: %s (path: %s)", fullPath, path)
				return path
			}

			// Use path + modtime + size as a cache key
			key := fmt.Sprintf("%s:%d:%d", path, info.ModTime().Unix(), info.Size())
			if h, ok := r.hashes.Load(key); ok {
				return fmt.Sprintf("%s?v=%s", path, h.(string))
			}

			f, err := os.Open(fullPath)
			if err != nil {
				return path
			}
			defer f.Close()

			hash := sha256.New()
			if _, err := io.Copy(hash, f); err != nil {
				return path
			}
			hStr := hex.EncodeToString(hash.Sum(nil))[:8]
			r.hashes.Store(key, hStr)
			return fmt.Sprintf("%s?v=%s", path, hStr)
		},

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

		// derefF dereferences a *float64 safely, formatting without trailing
		// zeros (230.0 → "230", 40.5 → "40.5"). Returns "" for nil.
		"derefF": func(f *float64) string {
			if f == nil {
				return ""
			}
			return strconv.FormatFloat(*f, 'g', -1, 64)
		},

		// lower lowercases a string: {{lower .Color}}
		"lower": strings.ToLower,

		// list builds a []string from its args, for ranging over literal keys:
		//   {{range list "kneePain" "heelPain"}}...{{end}}
		"list": func(items ...string) []string { return items },

		// json encodes a value as JSON for use in <script type="application/json"> tags.
		"json": func(v any) (template.JS, error) {
			b, err := json.Marshal(v)
			if err != nil {
				return "", err
			}
			return template.JS(b), nil
		},

		// nl2p converts double-newline-separated text into <p> blocks with <br> for single newlines.
		"nl2p": func(s string) template.HTML {
			if s == "" {
				return ""
			}
			var b strings.Builder
			for _, para := range strings.Split(strings.TrimSpace(s), "\n\n") {
				trimmed := strings.TrimSpace(para)
				if trimmed == "" {
					continue
				}
				b.WriteString("<p>")
				lines := strings.Split(trimmed, "\n")
				for i, line := range lines {
					if i > 0 {
						b.WriteString("<br>")
					}
					b.WriteString(template.HTMLEscapeString(line))
				}
				b.WriteString("</p>")
			}
			return template.HTML(b.String())
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
	r.tmpl = tmpl

	// Parse partials first so page templates can reference them by name.
	for _, pattern := range []string{
		filepath.Join(dir, "partials", "*.html"),
		filepath.Join(dir, "pages", "*.html"),
	} {
		if _, err := tmpl.ParseGlob(pattern); err != nil {
			return nil, fmt.Errorf("parse %s: %w", pattern, err)
		}
	}

	return r, nil
}

// Render executes the named template into a buffer first, then writes the
// result to w. Buffering means a template execution error is caught BEFORE any
// bytes reach the client, so we can send a clean 500 instead of a truncated
// 200 with a half-written body (and the "superfluous WriteHeader" that causes).
func (r *Renderer) Render(w http.ResponseWriter, name string, data any) {
	var buf bytes.Buffer
	if err := r.tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		log.Printf("template render %q failed: %v", name, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(buf.Bytes())
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
