package main

// Catalog editing. data/ stays the source of truth: the admin edits
// data/{brand}/catalog.json (price, sale, mode, published, featured, order,
// brand visibility) and data/{brand}/products/{slug}.json (colours, sizes) in
// place — key order and formatting preserved — then runs cmd/shoeimport for the
// brand (Preview = dry-run, Apply = -commit; NEVER -stock, so live stock stays).
//
// A colour can't reach the site without photos: Apply is blocked while any
// colour of a PUBLISHED model lacks its main photo locally or on the live site.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var slugRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
var hexRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// colorFolder MUST match cmd/shoeimport's colorFolder (image folder per colour).
func colorFolder(name string) string {
	return strings.Trim(nonAlnum.ReplaceAllString(strings.ToLower(name), "-"), "-")
}

type colorInfo struct {
	Name, Hex, Folder string
	Primary           bool
	HasMain           bool   // public/images/…/{folder}/{slug}.webp exists here
	Live              string // "yes" | "no" | "?" (main photo reachable on the live site)
	HasCard           bool   // {slug}-card.webp (listing card) — only matters for the primary
	MainPath          string // where the main photo must go
	Stock             int    // live pairs in the DB for this colour
}

// Ready: safe to show on the site.
func (c colorInfo) Ready() bool { return c.HasMain && c.Live != "no" }

type sizeInfo struct {
	Label    string
	Stock    int
	InChart  bool // has a sizeChart row (only checked when the model has a chart)
	HasChart bool
}

type model struct {
	Brand, Slug, Name      string
	Price, OrigPrice, Sort int
	Mode                   string
	Published, Featured    bool
	Colors                 []colorInfo
	Sizes                  []sizeInfo
	SizesFormat            string // "stock" or "sizes"
	TotalStock             int
	Problems               []string // blockers for Apply (published models only)
	Warnings               []string
}

type brandCat struct {
	Slug     string
	Visible  bool
	Models   []model
	Problems []string
}

func dataPath(parts ...string) string { return filepath.Join(append([]string{"data"}, parts...)...) }

// catalogBrands lists brands that have a data/{brand}/catalog.json.
func catalogBrands() []string {
	files, _ := filepath.Glob(dataPath("*", "catalog.json"))
	var out []string
	for _, f := range files {
		out = append(out, filepath.Base(filepath.Dir(f)))
	}
	sort.Strings(out)
	return out
}

// checkModel validates {brand}/{slug} against the files on disk (no path tricks).
func checkModel(brand, slug string) error {
	if !slugRe.MatchString(brand) || (slug != "" && !slugRe.MatchString(slug)) {
		return errors.New("bad brand or model name")
	}
	if _, err := os.Stat(dataPath(brand, "catalog.json")); err != nil {
		return fmt.Errorf("no catalog for %q", brand)
	}
	if slug != "" {
		if _, err := os.Stat(dataPath(brand, "products", slug+".json")); err != nil {
			return fmt.Errorf("no product file for %s/%s", brand, slug)
		}
	}
	return nil
}

func jsonInt(v any) int {
	if n, ok := v.(json.Number); ok {
		f, _ := n.Float64()
		return int(math.Round(f))
	}
	return 0
}

func jsonBool(v any) bool { b, _ := v.(bool); return b }

func jsonStr(v any) string { s, _ := v.(string); return s }

// liveStock returns colour → size → qty from the DB for one model.
func (a *app) liveStock(brand, slug string) map[string]map[string]int {
	out := map[string]map[string]int{}
	rows, err := a.db.Query(`
		SELECT pc.color, ps.eu_size::float8, st.qty
		FROM product_stock st
		JOIN products p ON p.id = st.product_id
		JOIN brands b ON b.id = p.brand_id
		JOIN product_colors pc ON pc.id = st.color_id
		JOIN product_sizes ps ON ps.id = st.size_id
		WHERE b.slug = $1 AND p.slug = $2`, brand, slug)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var c string
		var s float64
		var q int
		if rows.Scan(&c, &s, &q) == nil {
			if out[c] == nil {
				out[c] = map[string]int{}
			}
			out[c][strconv.FormatFloat(s, 'f', -1, 64)] = q
		}
	}
	return out
}

var liveCache sync.Map // url → struct{ok string; at time.Time}

type liveEntry struct {
	ok string
	at time.Time
}

// liveCheck HEADs the photo on the live site (cached 60s). "?" when unknown.
func (a *app) liveCheck(path string) string {
	if !strings.HasPrefix(a.siteURL, "https://") {
		return "?"
	}
	u := a.siteURL + path
	if v, ok := liveCache.Load(u); ok && time.Since(v.(liveEntry).at) < time.Minute {
		return v.(liveEntry).ok
	}
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodHead, u, nil)
	res, err := http.DefaultClient.Do(req)
	ok := "?"
	if err == nil {
		res.Body.Close()
		if res.StatusCode == http.StatusOK {
			ok = "yes"
		} else if res.StatusCode == http.StatusNotFound {
			ok = "no"
		}
	}
	liveCache.Store(u, liveEntry{ok, time.Now()})
	return ok
}

// loadModel reads one model from the files + live DB stock + image checks.
func (a *app) loadModel(brand, slug string, cat *ojson) (model, error) {
	m := model{Brand: brand, Slug: slug}
	e := cat.Obj("products").Obj(slug)
	if e == nil {
		return m, fmt.Errorf("%s is not in %s/catalog.json", slug, brand)
	}
	v, _ := e.Get("price")
	m.Price = jsonInt(v)
	v, _ = e.Get("originalPrice")
	m.OrigPrice = jsonInt(v)
	v, _ = e.Get("mode")
	m.Mode = jsonStr(v)
	v, _ = e.Get("published")
	m.Published = jsonBool(v)
	v, _ = e.Get("featured")
	m.Featured = jsonBool(v)
	v, _ = e.Get("sortOrder")
	m.Sort = jsonInt(v)

	p, err := readOJSON(dataPath(brand, "products", slug+".json"))
	if err != nil {
		return m, err
	}
	v, _ = p.Get("name")
	m.Name = jsonStr(v)
	live := a.liveStock(brand, slug)

	cols, _ := p.vals["colors"].([]any)
	for i, cv := range cols {
		co, _ := cv.(*ojson)
		if co == nil {
			continue
		}
		n, _ := co.Get("name")
		h, _ := co.Get("hex")
		c := colorInfo{Name: jsonStr(n), Hex: jsonStr(h), Primary: i == 0}
		c.Folder = colorFolder(c.Name)
		rel := fmt.Sprintf("/images/%s/%s/images/%s/%s.webp", brand, slug, c.Folder, slug)
		c.MainPath = "public" + rel
		_, err := os.Stat(c.MainPath)
		c.HasMain = err == nil
		_, err = os.Stat(filepath.Join(filepath.Dir(c.MainPath), slug+"-card.webp"))
		c.HasCard = err == nil
		if c.HasMain {
			c.Live = a.liveCheck(rel)
		} else {
			c.Live = "no"
		}
		for _, q := range live[c.Name] {
			c.Stock += q
		}
		m.TotalStock += c.Stock
		m.Colors = append(m.Colors, c)
	}

	// Sizes: the "sizes" list, or the union of "stock" size keys.
	set := map[string]bool{}
	if arr, ok := p.vals["sizes"].([]any); ok {
		m.SizesFormat = "sizes"
		for _, s := range arr {
			set[jsonStr(s)] = true
		}
	} else if st := p.Obj("stock"); st != nil {
		m.SizesFormat = "stock"
		for _, ck := range st.keys {
			if bys := st.Obj(ck); bys != nil {
				for _, sk := range bys.keys {
					set[sk] = true
				}
			}
		}
	}
	chart := p.Obj("sizeChart")
	for s := range set {
		si := sizeInfo{Label: s, HasChart: chart != nil}
		if chart != nil {
			_, si.InChart = chart.Get(s)
		}
		for _, bys := range live {
			si.Stock += bys[s]
		}
		m.Sizes = append(m.Sizes, si)
	}
	sort.Slice(m.Sizes, func(i, j int) bool { return atof(m.Sizes[i].Label) < atof(m.Sizes[j].Label) })

	for _, c := range m.Colors {
		switch {
		case !c.HasMain:
			m.Problems = append(m.Problems, fmt.Sprintf("“%s” has no photos: add %s (plus the numbered gallery shots) in that folder", c.Name, c.MainPath))
		case c.Live == "no":
			m.Problems = append(m.Problems, fmt.Sprintf("“%s” photos aren't on the live site yet: commit + push public/images/%s/%s/images/%s/, wait for the deploy", c.Name, brand, slug, c.Folder))
		}
		if c.Primary && c.HasMain && !c.HasCard {
			m.Warnings = append(m.Warnings, fmt.Sprintf("primary colour “%s” has no -card image — run `go run ./cmd/imgvariants` (listing cards load the full photo until then)", c.Name))
		}
		if c.HasMain {
			if _, err := os.Stat(filepath.Join(filepath.Dir(c.MainPath), slug+"-swatch.webp")); err != nil && len(m.Colors) > 1 {
				m.Warnings = append(m.Warnings, fmt.Sprintf("“%s” has no -swatch photo tile — the colour picker shows the hex dot instead", c.Name))
			}
		}
	}
	for _, s := range m.Sizes {
		if s.HasChart && !s.InChart {
			m.Warnings = append(m.Warnings, fmt.Sprintf("size %s has no row in the size chart (sizeChart in the product file)", s.Label))
		}
	}
	if len(m.Colors) == 0 {
		m.Problems = append(m.Problems, "no colours")
	}
	if len(m.Sizes) == 0 {
		m.Problems = append(m.Problems, "no sizes")
	}
	if !m.Published {
		m.Problems = nil // hidden models can be prepared without photos
	}
	return m, nil
}

func atof(s string) float64 { f, _ := strconv.ParseFloat(s, 64); return f }

// loadBrand reads every model of a brand (catalog order by sortOrder).
func (a *app) loadBrand(brand string) (brandCat, error) {
	b := brandCat{Slug: brand, Visible: true}
	cat, err := readOJSON(dataPath(brand, "catalog.json"))
	if err != nil {
		return b, err
	}
	if br := cat.Obj("brand"); br != nil {
		if v, ok := br.Get("visible"); ok {
			b.Visible = jsonBool(v)
		}
	}
	prods := cat.Obj("products")
	if prods == nil {
		return b, nil
	}
	for _, slug := range prods.keys {
		m, err := a.loadModel(brand, slug, cat)
		if err != nil {
			b.Problems = append(b.Problems, err.Error())
			continue
		}
		for _, pr := range m.Problems {
			b.Problems = append(b.Problems, m.Name+": "+pr)
		}
		b.Models = append(b.Models, m)
	}
	sort.SliceStable(b.Models, func(i, j int) bool { return b.Models[i].Sort < b.Models[j].Sort })
	return b, nil
}

// ── Pages ────────────────────────────────────────────────────────────────

func (a *app) catalog(w http.ResponseWriter, r *http.Request) {
	var brands []brandCat
	for _, s := range catalogBrands() {
		b, err := a.loadBrand(s)
		if err != nil {
			b.Problems = append(b.Problems, err.Error())
		}
		brands = append(brands, b)
	}
	a.render(w, "catalog.html", a.page("catalog", map[string]any{
		"Brands": brands, "Msg": r.URL.Query().Get("msg"), "Err": r.URL.Query().Get("err"),
	}))
}

func (a *app) modelPage(w http.ResponseWriter, r *http.Request) {
	brand, slug := r.PathValue("brand"), r.PathValue("slug")
	if err := checkModel(brand, slug); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	cat, err := readOJSON(dataPath(brand, "catalog.json"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	m, err := a.loadModel(brand, slug, cat)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	a.render(w, "model.html", a.page("catalog", map[string]any{
		"M": m, "Msg": r.URL.Query().Get("msg"), "Err": r.URL.Query().Get("err"),
		"Modes": []string{"stock", "reservation", "dropship"},
	}))
}

// ── Edits (each writes the JSON, then back to the page) ─────────────────

func (a *app) toggleBrand(w http.ResponseWriter, r *http.Request) {
	brand := r.PathValue("brand")
	if err := checkModel(brand, ""); err != nil {
		a.catBack(w, r, "", "", err.Error())
		return
	}
	path := dataPath(brand, "catalog.json")
	cat, err := readOJSON(path)
	if err != nil {
		a.catBack(w, r, "", "", err.Error())
		return
	}
	br := cat.Obj("brand")
	if br == nil {
		br = newOJSON()
		cat.vals["brand"] = br
		cat.keys = append([]string{"brand"}, cat.keys...)
	}
	cur := true // no "visible" key = shoeimport leaves it as is; treat as shown
	if v, ok := br.Get("visible"); ok {
		cur = jsonBool(v)
	}
	now := !cur
	br.Set("visible", now)
	if err := writeOJSON(path, cat); err != nil {
		a.catBack(w, r, "", "", err.Error())
		return
	}
	state := "hidden from"
	if now {
		state = "shown on"
	}
	a.catBack(w, r, "", fmt.Sprintf("%s will be %s the brands page — click Apply to site", brand, state), "")
}

func (a *app) saveSettings(w http.ResponseWriter, r *http.Request) {
	a.editCatalog(w, r, func(e *ojson) (string, error) {
		price, err := strconv.Atoi(strings.TrimSpace(r.FormValue("price")))
		if err != nil || price <= 0 || price > 1000 {
			return "", errors.New("price must be a whole number of euros (1–1000)")
		}
		orig := 0
		if s := strings.TrimSpace(r.FormValue("originalPrice")); s != "" {
			if orig, err = strconv.Atoi(s); err != nil || orig < 0 {
				return "", errors.New("sale “was” price must be a whole number of euros, or empty")
			}
			if orig != 0 && orig <= price {
				return "", errors.New("the “was” price must be higher than the price (or empty to end the sale)")
			}
		}
		mode := r.FormValue("mode")
		if mode != "stock" && mode != "reservation" && mode != "dropship" {
			return "", errors.New("unknown mode")
		}
		sortOrder, err := strconv.Atoi(strings.TrimSpace(r.FormValue("sortOrder")))
		if err != nil {
			return "", errors.New("order must be a whole number")
		}
		e.Set("price", json.Number(strconv.Itoa(price)))
		if orig > 0 {
			insertAfter(e, "price", "originalPrice", json.Number(strconv.Itoa(orig)))
		} else {
			e.Delete("originalPrice")
		}
		e.Set("mode", mode)
		e.Set("published", r.FormValue("published") == "on")
		e.Set("featured", r.FormValue("featured") == "on")
		e.Set("sortOrder", json.Number(strconv.Itoa(sortOrder)))
		return "Settings saved", nil
	})
}

// insertAfter sets k right after `after` (keeps the files' usual key order).
func insertAfter(o *ojson, after, k string, v any) {
	if _, ok := o.vals[k]; ok {
		o.vals[k] = v
		return
	}
	o.vals[k] = v
	for i, x := range o.keys {
		if x == after {
			o.keys = append(o.keys[:i+1], append([]string{k}, o.keys[i+1:]...)...)
			return
		}
	}
	o.keys = append(o.keys, k)
}

// editCatalog loads catalog.json, lets fn change this model's entry, saves.
func (a *app) editCatalog(w http.ResponseWriter, r *http.Request, fn func(e *ojson) (string, error)) {
	brand, slug := r.PathValue("brand"), r.PathValue("slug")
	if err := checkModel(brand, slug); err != nil {
		a.catBack(w, r, slug, "", err.Error())
		return
	}
	path := dataPath(brand, "catalog.json")
	cat, err := readOJSON(path)
	if err != nil {
		a.catBack(w, r, slug, "", err.Error())
		return
	}
	e := cat.Obj("products").Obj(slug)
	if e == nil {
		a.catBack(w, r, slug, "", slug+" is not in catalog.json")
		return
	}
	msg, err := fn(e)
	if err != nil {
		a.catBack(w, r, slug, "", err.Error())
		return
	}
	if err := writeOJSON(path, cat); err != nil {
		a.catBack(w, r, slug, "", err.Error())
		return
	}
	a.catBack(w, r, slug, msg+" — click Apply to site to make it live", "")
}

// editProduct loads the product file, lets fn change it, saves.
func (a *app) editProduct(w http.ResponseWriter, r *http.Request, fn func(p *ojson) (string, error)) {
	brand, slug := r.PathValue("brand"), r.PathValue("slug")
	if err := checkModel(brand, slug); err != nil {
		a.catBack(w, r, slug, "", err.Error())
		return
	}
	path := dataPath(brand, "products", slug+".json")
	p, err := readOJSON(path)
	if err != nil {
		a.catBack(w, r, slug, "", err.Error())
		return
	}
	msg, err := fn(p)
	if err != nil {
		a.catBack(w, r, slug, "", err.Error())
		return
	}
	if err := writeOJSON(path, p); err != nil {
		a.catBack(w, r, slug, "", err.Error())
		return
	}
	a.catBack(w, r, slug, msg+" — click Apply to site to make it live", "")
}

func colorList(p *ojson) []any { c, _ := p.vals["colors"].([]any); return c }

func colorIndex(p *ojson, name string) int {
	for i, cv := range colorList(p) {
		if co, _ := cv.(*ojson); co != nil {
			if n, _ := co.Get("name"); strings.EqualFold(jsonStr(n), name) {
				return i
			}
		}
	}
	return -1
}

func (a *app) addColor(w http.ResponseWriter, r *http.Request) {
	a.editProduct(w, r, func(p *ojson) (string, error) {
		name := strings.Join(strings.Fields(r.FormValue("name")), " ")
		hex := strings.ToLower(strings.TrimSpace(r.FormValue("hex")))
		if name == "" || len(name) > 40 || colorFolder(name) == "" {
			return "", errors.New("enter a colour name (letters/numbers), e.g. “Forest Green”")
		}
		if !hexRe.MatchString(hex) {
			return "", errors.New("pick the colour dot (hex like #2a3350)")
		}
		if colorIndex(p, name) >= 0 {
			return "", fmt.Errorf("“%s” already exists", name)
		}
		for _, cv := range colorList(p) {
			if co, _ := cv.(*ojson); co != nil {
				if n, _ := co.Get("name"); colorFolder(jsonStr(n)) == colorFolder(name) {
					return "", fmt.Errorf("“%s” would share the image folder of “%s”", name, jsonStr(n))
				}
			}
		}
		c := newOJSON()
		c.Set("name", name)
		c.Set("hex", hex)
		p.Set("colors", append(colorList(p), c))
		// "stock" format: give the new colour every size at 0 (live stock starts at 0).
		if st := p.Obj("stock"); st != nil {
			sizes := newOJSON()
			seen := map[string]bool{}
			var all []string
			for _, ck := range st.keys {
				if bys := st.Obj(ck); bys != nil {
					for _, sk := range bys.keys {
						if !seen[sk] {
							seen[sk] = true
							all = append(all, sk)
						}
					}
				}
			}
			sort.Slice(all, func(i, j int) bool { return atof(all[i]) < atof(all[j]) })
			for _, s := range all {
				sizes.Set(s, json.Number("0"))
			}
			st.Set(name, sizes)
		}
		return fmt.Sprintf("Colour “%s” added — now add its photos to public/images/%s/%s/images/%s/", name,
			r.PathValue("brand"), r.PathValue("slug"), colorFolder(name)), nil
	})
}

func (a *app) removeColor(w http.ResponseWriter, r *http.Request) {
	a.editProduct(w, r, func(p *ojson) (string, error) {
		name := r.FormValue("name")
		i := colorIndex(p, name)
		if i < 0 {
			return "", fmt.Errorf("no colour “%s”", name)
		}
		cols := colorList(p)
		if len(cols) == 1 {
			return "", errors.New("a model needs at least one colour")
		}
		p.Set("colors", append(cols[:i:i], cols[i+1:]...))
		if st := p.Obj("stock"); st != nil {
			st.Delete(name)
		}
		if cs := p.Obj("colorSizes"); cs != nil {
			cs.Delete(name)
			if len(cs.keys) == 0 {
				p.Delete("colorSizes")
			}
		}
		return fmt.Sprintf("Colour “%s” removed (its photos stay in the folder)", name), nil
	})
}

func (a *app) primaryColor(w http.ResponseWriter, r *http.Request) {
	a.editProduct(w, r, func(p *ojson) (string, error) {
		name := r.FormValue("name")
		i := colorIndex(p, name)
		if i < 0 {
			return "", fmt.Errorf("no colour “%s”", name)
		}
		cols := colorList(p)
		c := cols[i]
		rest := append(cols[:i:i], cols[i+1:]...)
		p.Set("colors", append([]any{c}, rest...))
		return fmt.Sprintf("“%s” is now the primary colour (card + first photo)", name), nil
	})
}

func normSize(s string) (string, error) {
	f, err := strconv.ParseFloat(strings.TrimSpace(strings.ReplaceAll(s, ",", ".")), 64)
	if err != nil || f < 15 || f > 50 || math.Mod(f*2, 1) != 0 {
		return "", errors.New("size must be an EU size like 42 or 40.5")
	}
	return strconv.FormatFloat(f, 'f', -1, 64), nil
}

func (a *app) addSize(w http.ResponseWriter, r *http.Request) {
	a.editProduct(w, r, func(p *ojson) (string, error) {
		s, err := normSize(r.FormValue("size"))
		if err != nil {
			return "", err
		}
		if arr, ok := p.vals["sizes"].([]any); ok {
			for _, x := range arr {
				if jsonStr(x) == s {
					return "", fmt.Errorf("size %s already exists", s)
				}
			}
			arr = append(arr, s)
			sort.SliceStable(arr, func(i, j int) bool { return atof(jsonStr(arr[i])) < atof(jsonStr(arr[j])) })
			p.Set("sizes", arr)
			return "Size " + s + " added", nil
		}
		st := p.Obj("stock")
		if st == nil {
			return "", errors.New(`this product file has neither "sizes" nor "stock"`)
		}
		for _, ck := range st.keys {
			bys := st.Obj(ck)
			if bys == nil {
				continue
			}
			if _, ok := bys.Get(s); ok {
				return "", fmt.Errorf("size %s already exists", s)
			}
			bys.Set(s, json.Number("0"))
			sort.SliceStable(bys.keys, func(i, j int) bool { return atof(bys.keys[i]) < atof(bys.keys[j]) })
		}
		return "Size " + s + " added (0 in stock)", nil
	})
}

func (a *app) removeSize(w http.ResponseWriter, r *http.Request) {
	a.editProduct(w, r, func(p *ojson) (string, error) {
		s, err := normSize(r.FormValue("size"))
		if err != nil {
			return "", err
		}
		if arr, ok := p.vals["sizes"].([]any); ok {
			out := arr[:0:0]
			for _, x := range arr {
				if jsonStr(x) != s {
					out = append(out, x)
				}
			}
			if len(out) == len(arr) {
				return "", fmt.Errorf("no size %s", s)
			}
			if len(out) == 0 {
				return "", errors.New("a model needs at least one size")
			}
			p.Set("sizes", out)
			// Also drop it from any per-colour subset ("colorSizes").
			if cs := p.Obj("colorSizes"); cs != nil {
				for _, ck := range cs.keys {
					if list, ok := cs.vals[ck].([]any); ok {
						kept := list[:0:0]
						for _, x := range list {
							if jsonStr(x) != s {
								kept = append(kept, x)
							}
						}
						if len(kept) == 0 {
							return "", fmt.Errorf("“%s” is only made in size %s — remove the colour instead", ck, s)
						}
						cs.Set(ck, kept)
					}
				}
			}
			return "Size " + s + " removed", nil
		}
		st := p.Obj("stock")
		if st == nil {
			return "", errors.New(`this product file has neither "sizes" nor "stock"`)
		}
		found := false
		for _, ck := range st.keys {
			if bys := st.Obj(ck); bys != nil {
				if _, ok := bys.Get(s); ok {
					found = true
					bys.Delete(s)
					if len(bys.keys) == 0 {
						return "", errors.New("a model needs at least one size")
					}
				}
			}
		}
		if !found {
			return "", fmt.Errorf("no size %s", s)
		}
		return "Size " + s + " removed", nil
	})
}

// ── Apply = run cmd/shoeimport for the brand ────────────────────────────

func (a *app) apply(w http.ResponseWriter, r *http.Request) {
	brand := r.PathValue("brand")
	if err := checkModel(brand, ""); err != nil {
		a.catBack(w, r, "", "", err.Error())
		return
	}
	commit := r.FormValue("commit") == "1"
	if commit {
		b, err := a.loadBrand(brand)
		if err != nil {
			a.catBack(w, r, r.FormValue("slug"), "", err.Error())
			return
		}
		if len(b.Problems) > 0 {
			a.catBack(w, r, r.FormValue("slug"), "", "Not applied — fix first: "+strings.Join(b.Problems, " · "))
			return
		}
	}
	args := []string{"run", "./cmd/shoeimport", "-brand", brand} // never -stock: live stock stays
	if commit {
		args = append(args, "-commit")
	}
	ctx, cancel := context.WithTimeout(r.Context(), 4*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Env = os.Environ() // same DATABASE_URL as the admin (sandbox or production)
	out, err := cmd.CombinedOutput()
	title := "Preview (nothing changed)"
	if commit {
		title = "Applied to the site"
	}
	if err != nil {
		title = "Import failed — nothing was changed"
	}
	a.render(w, "apply.html", a.page("catalog", map[string]any{
		"Title": title, "Out": strings.TrimSpace(string(out)), "Failed": err != nil,
		"Brand": brand, "Slug": r.FormValue("slug"), "Commit": commit,
	}))
}

// catBack returns to the model page (or the catalog) with a flash message.
func (a *app) catBack(w http.ResponseWriter, r *http.Request, slug, msg, errMsg string) {
	q := url.Values{}
	if msg != "" {
		q.Set("msg", msg)
	}
	if errMsg != "" {
		q.Set("err", errMsg)
	}
	dest := "/catalog"
	if slug != "" && slugRe.MatchString(slug) && slugRe.MatchString(r.PathValue("brand")) {
		dest = "/catalog/" + r.PathValue("brand") + "/" + slug
	}
	http.Redirect(w, r, dest+"?"+q.Encode(), http.StatusSeeOther)
}
