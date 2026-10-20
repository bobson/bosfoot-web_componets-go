// Command shoeimport makes data/ the source of truth for products: for each brand
// it reads data/{brand}/catalog.json + data/{brand}/products/*.json and upserts
// every product into the database, deriving the gallery, primary image and colour
// swatches by scanning public/images. Re-running is safe and self-healing (ON
// CONFLICT DO UPDATE for the parent row; child rows are rebuilt).
//
//	go run ./cmd/shoeimport                       # dry-run, every brand with a catalog.json
//	go run ./cmd/shoeimport -commit               # apply
//	go run ./cmd/shoeimport -brand be-lenka       # one brand only — never touches others
//	go run ./cmd/shoeimport -stock -commit        # ALSO load stock qty from the JSON
//
// FILES (per brand):
//   - catalog.json — the control panel, the things you change often. Per product:
//     "price" (EUR, the source; MKD derived via site.MKD), optional
//     "originalPrice" (EUR, puts it on sale — remove to end the sale), "mode"
//     (stock | reservation | dropship: what happens to sizes NOT in stock),
//     "published", "featured", "sortOrder". Brand-level "visible" shows/hides the
//     brand on the brands page. Every product file must have an entry, and v.v.
//   - products/{slug}.json — the rarely-changed content: name, gender, colours,
//     sizes ("stock" counts, or a plain "sizes" list for dropship models),
//     specs, activities, highlights, translations, optional "sizeChart" (mm).
//   - costs.json (optional, GITIGNORED — import prices are private): supplier
//     cost per product + customs % + shipping per pair. Only used to print the
//     estimated profit per pair; never written to the DB.
//
// STOCK: the database is the source of truth for live stock (orders decrement
// it). By default the import keeps each product's current DB quantities and only
// takes the JSON stock for a brand-new product (no stock rows yet). Pass -stock
// only when a delivery arrives and the JSON holds the real counts — it then
// overwrites DB stock with the JSON.
//
// Brand row comes from the SQL seed/migrations; size_chart rows are replaced only
// for products whose JSON has a "sizeChart" (Freet's still come from SQL).
// Run from the project root so data/ and public/ resolve.
package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"bosfoot/internal/database"
	"bosfoot/internal/site"
)

// catalogFile is data/{brand}/catalog.json.
type catalogFile struct {
	Brand struct {
		Visible *bool `json:"visible"` // nil = leave the brand's visibility as is
	} `json:"brand"`
	Products map[string]catalogEntry `json:"products"`
}

type catalogEntry struct {
	Price         int    `json:"price"`                   // EUR, shown + charged
	OriginalPrice int    `json:"originalPrice,omitempty"` // EUR; > price = on sale
	Mode          string `json:"mode"`                    // stock | reservation | dropship
	Published     bool   `json:"published"`
	Featured      bool   `json:"featured"`
	SortOrder     int    `json:"sortOrder"`
}

var validModes = map[string]bool{"stock": true, "reservation": true, "dropship": true}

// costsFile is the optional, gitignored data/{brand}/costs.json.
type costsFile struct {
	CustomsPct  float64 `json:"customsPct"`  // default duty+VAT on the import price
	ShippingEUR float64 `json:"shippingEUR"` // default inbound shipping per pair
	Products    map[string]struct {
		Cost       float64  `json:"cost"`       // supplier price per pair, EUR
		CustomsPct *float64 `json:"customsPct"` // override (e.g. 18 = VAT only, EU origin)
	} `json:"products"`
}

type product struct {
	Name         string                        `json:"name"`
	Gender       string                        `json:"gender"`
	Activities   []string                      `json:"activities"`
	Highlights   []map[string]string           `json:"highlights"`
	Colors       []struct{ Name, Hex string }  `json:"colors"`
	Stock        map[string]map[string]int     `json:"stock"`      // color -> size -> qty
	Sizes        []string                      `json:"sizes"`      // alternative to stock: every colour × size at qty 0
	ColorSizes   map[string][]string           `json:"colorSizes"` // with "sizes": colour -> the subset it is made in
	SizeChart    map[string]sizeChartRow       `json:"sizeChart"`
	Specs        []struct{ Key, Value string } `json:"specs"`
	Translations map[string]struct {
		Description    string `json:"description"`
		About          string `json:"about"`
		SizeAndFit     string `json:"sizeAndFit"`
		ProductInfo    string `json:"productInfo"`
		Sustainability string `json:"sustainability"`
	} `json:"translations"`
}

// sizeChartRow is one EU size's insole length/width in millimetres.
type sizeChartRow struct {
	Length float64 `json:"length"`
	Width  float64 `json:"width"`
}

// catalogOnly are fields that live in catalog.json; finding one in a product
// file means a stale second source, so the import refuses to guess.
var catalogOnly = []string{"price", "originalPrice", "status", "sortOrder", "mode"}

// sizeKey normalises an EU size for map lookups so the JSON key ("43", "42.5")
// and the DB numeric (43, 42.5) agree.
func sizeKey(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// colorFolder slugifies a colour name to its image folder: "Brown Black" ->
// "brown-black", "White & Green" -> "white-green". MUST match colorFolder()/the
// colour key in public/components/product-detail.js.
func colorFolder(name string) string {
	return strings.Trim(nonAlnum.ReplaceAllString(strings.ToLower(name), "-"), "-")
}

var gallerySuffix = regexp.MustCompile(`-(\d+)\.webp$`)

// galleryFiles returns the numbered gallery images in dir for slug, sorted by
// their numeric suffix, excluding the bare primary ({slug}.webp) and the
// -card/-swatch variants (no numeric suffix).
func galleryFiles(dir, slug string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	type f struct {
		name string
		n    int
	}
	var fs []f
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, slug+"-") {
			continue
		}
		m := gallerySuffix.FindStringSubmatch(strings.TrimPrefix(name, slug))
		if m == nil {
			continue
		}
		n, _ := strconv.Atoi(m[1])
		fs = append(fs, f{name, n})
	}
	sort.Slice(fs, func(i, j int) bool { return fs[i].n < fs[j].n })
	out := make([]string, len(fs))
	for i, x := range fs {
		out[i] = x.name
	}
	return out
}

func main() {
	// Default is a dry run: each product is built in a transaction then rolled
	// back, so you can read the summary (prices, modes, stock, profit) first.
	commit := flag.Bool("commit", false, "actually write to the DB (default: dry-run, rolled back)")
	brand := flag.String("brand", "", "only this brand slug (default: every data/*/catalog.json)")
	loadStock := flag.Bool("stock", false, "overwrite DB stock with the JSON quantities (new delivery); default keeps live DB stock")
	flag.Parse()

	brands := []string{*brand}
	if *brand == "" {
		cats, _ := filepath.Glob(filepath.Join("data", "*", "catalog.json"))
		brands = brands[:0]
		for _, c := range cats {
			brands = append(brands, filepath.Base(filepath.Dir(c)))
		}
		sort.Strings(brands)
		if len(brands) == 0 {
			log.Fatal("no data/*/catalog.json found (run from the project root)")
		}
	}

	db, err := database.Connect()
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	var catID int
	if err := db.QueryRow(`SELECT id FROM categories WHERE slug='shoes'`).Scan(&catID); err != nil {
		log.Fatalf("category 'shoes' not found: %v", err)
	}

	if !*commit {
		fmt.Println("DRY RUN (no changes written — pass -commit to apply)")
	}
	if *loadStock {
		fmt.Println("STOCK: loading quantities from the JSON (overwrites live DB stock)")
	} else {
		fmt.Println("STOCK: keeping live DB quantities (pass -stock to load from the JSON)")
	}
	total := 0
	for _, b := range brands {
		n, err := importBrand(db, b, catID, *commit, *loadStock)
		if err != nil {
			log.Fatalf("%s: %v", b, err)
		}
		total += n
	}
	if *commit {
		fmt.Printf("Committed %d products (%s).\n", total, strings.Join(brands, ", "))
	} else {
		fmt.Printf("Dry run complete: %d products (rolled back).\n", total)
	}
}

func importBrand(db *sql.DB, brandSlug string, catID int, commit, loadStock bool) (int, error) {
	var brandID int
	if err := db.QueryRow(`SELECT id FROM brands WHERE slug=$1`, brandSlug).Scan(&brandID); err != nil {
		return 0, fmt.Errorf("brand %q not in DB (add it via db/ migration first): %w", brandSlug, err)
	}

	var cat catalogFile
	if err := readJSON(filepath.Join("data", brandSlug, "catalog.json"), &cat); err != nil {
		return 0, err
	}
	var costs costsFile
	if err := readJSON(filepath.Join("data", brandSlug, "costs.json"), &costs); err != nil && !errors.Is(err, os.ErrNotExist) {
		return 0, err
	}

	files, _ := filepath.Glob(filepath.Join("data", brandSlug, "products", "*.json"))
	sort.Strings(files)
	seen := map[string]bool{}
	for _, f := range files {
		slug := strings.TrimSuffix(filepath.Base(f), ".json")
		seen[slug] = true
		if _, ok := cat.Products[slug]; !ok {
			return 0, fmt.Errorf("products/%s.json has no entry in catalog.json", slug)
		}
	}
	for slug, e := range cat.Products {
		if !seen[slug] {
			return 0, fmt.Errorf("catalog.json lists %q but products/%s.json is missing", slug, slug)
		}
		if !validModes[e.Mode] {
			return 0, fmt.Errorf("catalog %s: mode %q (want stock, reservation or dropship)", slug, e.Mode)
		}
		if e.Price <= 0 {
			return 0, fmt.Errorf("catalog %s: price must be > 0", slug)
		}
	}

	visible := "unchanged"
	if cat.Brand.Visible != nil {
		visible = strconv.FormatBool(*cat.Brand.Visible)
	}
	fmt.Printf("\n%s — %d products, brand visible: %s\n", brandSlug, len(files), visible)
	if cat.Brand.Visible != nil && commit {
		if _, err := db.Exec(`UPDATE brands SET is_visible=$1, updated_at=now() WHERE id=$2`, *cat.Brand.Visible, brandID); err != nil {
			return 0, fmt.Errorf("brand visibility: %w", err)
		}
	}

	for _, f := range files {
		slug := strings.TrimSuffix(filepath.Base(f), ".json")
		if err := importProduct(db, brandSlug, brandID, catID, slug, f, cat.Products[slug], costs, commit, loadStock); err != nil {
			return 0, fmt.Errorf("%s: %w", slug, err)
		}
	}
	return len(files), nil
}

func readJSON(path string, v any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, v); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

func importProduct(db *sql.DB, brandSlug string, brandID, catID int, slug, path string,
	e catalogEntry, costs costsFile, commit, loadStock bool) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return err
	}
	for _, k := range catalogOnly {
		if _, ok := fields[k]; ok {
			return fmt.Errorf("%q belongs in catalog.json, not the product file — remove it there", k)
		}
	}
	var p product
	if err := json.Unmarshal(raw, &p); err != nil {
		return err
	}
	if len(p.Colors) == 0 {
		return fmt.Errorf("no colors")
	}
	// Dropship models list sizes once; expand to every colour × size at qty 0.
	// "colorSizes" narrows a colour to the sizes the brand still makes it in
	// (e.g. Zing White & Beige only 36–41): no row = that size is not offered
	// in that colour, so the page disables it and the order API rejects it.
	if p.Stock == nil && len(p.Sizes) > 0 {
		known := map[string]bool{}
		for _, sz := range p.Sizes {
			known[sz] = true
		}
		colors := map[string]bool{}
		for _, c := range p.Colors {
			colors[c.Name] = true
		}
		for name, list := range p.ColorSizes {
			if !colors[name] {
				return fmt.Errorf("colorSizes: unknown colour %q", name)
			}
			for _, sz := range list {
				if !known[sz] {
					return fmt.Errorf("colorSizes %q: size %s is not in \"sizes\"", name, sz)
				}
			}
		}
		p.Stock = map[string]map[string]int{}
		for _, c := range p.Colors {
			sizes, ok := p.ColorSizes[c.Name]
			if !ok {
				sizes = p.Sizes
			}
			p.Stock[c.Name] = map[string]int{}
			for _, sz := range sizes {
				p.Stock[c.Name][sz] = 0
			}
		}
	} else if len(p.ColorSizes) > 0 {
		return fmt.Errorf(`"colorSizes" only works with "sizes" (with "stock", just leave the size out of that colour)`)
	}
	if len(p.Stock) == 0 {
		return fmt.Errorf(`no sizes: give "stock" (colour → size → qty) or "sizes"`)
	}

	sku := strings.ToUpper(strings.ReplaceAll(brandSlug, "-", "")) + "-" + strings.ToUpper(slug)
	var genderID int
	if err := db.QueryRow(`SELECT id FROM genders WHERE value=$1`, p.Gender).Scan(&genderID); err != nil {
		return fmt.Errorf("gender %q: %w", p.Gender, err)
	}
	primaryFolder := colorFolder(p.Colors[0].Name)
	imageURL := fmt.Sprintf("/images/%s/%s/images/%s/%s.webp", brandSlug, slug, primaryFolder, slug)
	if _, err := os.Stat(filepath.Join("public", imageURL)); err != nil {
		return fmt.Errorf("primary image missing: public%s", imageURL)
	}
	priceMKD := site.MKD(e.Price)
	// Per-product sale columns, all NULL/false unless OriginalPrice > Price.
	var origEUR, origMKD *int
	var discountPct *float64
	onSale := e.OriginalPrice > e.Price
	if onSale {
		oe, om := e.OriginalPrice, site.MKD(e.OriginalPrice)
		pct := math.Round(float64(om-priceMKD)/float64(om)*10000) / 100
		origEUR, origMKD, discountPct = &oe, &om, &pct
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var pid int
	err = tx.QueryRow(`
		INSERT INTO products
			(sku, name, slug, brand_id, category_id, gender_id,
			 price_mkd, price_eur, image_url, is_active, is_published, is_featured, sort_order,
			 original_price_eur, original_price_mkd, is_on_sale, discount_pct, fulfilment)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,TRUE,$10,$11,$12,$13,$14,$15,$16,$17)
		ON CONFLICT (sku) DO UPDATE SET
			name=EXCLUDED.name, slug=EXCLUDED.slug, gender_id=EXCLUDED.gender_id,
			price_mkd=EXCLUDED.price_mkd, price_eur=EXCLUDED.price_eur,
			image_url=EXCLUDED.image_url, is_active=EXCLUDED.is_active,
			is_published=EXCLUDED.is_published, is_featured=EXCLUDED.is_featured,
			sort_order=EXCLUDED.sort_order,
			original_price_eur=EXCLUDED.original_price_eur, original_price_mkd=EXCLUDED.original_price_mkd,
			is_on_sale=EXCLUDED.is_on_sale, discount_pct=EXCLUDED.discount_pct,
			fulfilment=EXCLUDED.fulfilment, updated_at=now()
		RETURNING id
	`, sku, p.Name, slug, brandID, catID, genderID,
		priceMKD, e.Price, imageURL, e.Published, e.Featured, e.SortOrder,
		origEUR, origMKD, onSale, discountPct, e.Mode).Scan(&pid)
	if err != nil {
		return fmt.Errorf("upsert product: %w", err)
	}

	// Translations (upsert per lang).
	for lang, t := range p.Translations {
		if _, err := tx.Exec(`
			INSERT INTO product_translations
				(product_id, lang, description, about, size_and_fit, product_info, sustainability)
			VALUES ($1,$2::lang_code,$3,$4,$5,$6,$7)
			ON CONFLICT (product_id, lang) DO UPDATE SET
				description=EXCLUDED.description, about=EXCLUDED.about,
				size_and_fit=EXCLUDED.size_and_fit, product_info=EXCLUDED.product_info,
				sustainability=EXCLUDED.sustainability
		`, pid, lang, nz(t.Description), nz(t.About), nz(t.SizeAndFit), nz(t.ProductInfo), nz(t.Sustainability)); err != nil {
			return fmt.Errorf("translation %s: %w", lang, err)
		}
	}

	// Snapshot live stock (colour -> size -> qty) before the rebuild wipes it, so
	// it can be restored unless -stock asks for the JSON counts. A product with no
	// stock rows yet is new, so it takes the JSON either way.
	live := map[string]map[string]int{}
	stockRows, err := tx.Query(`
		SELECT pc.color, ps.eu_size, st.qty
		FROM product_stock st
		JOIN product_colors pc ON pc.id = st.color_id
		JOIN product_sizes ps ON ps.id = st.size_id
		WHERE st.product_id = $1`, pid)
	if err != nil {
		return fmt.Errorf("read live stock: %w", err)
	}
	hasLive := false
	for stockRows.Next() {
		var color string
		var size float64
		var qty int
		if err := stockRows.Scan(&color, &size, &qty); err != nil {
			stockRows.Close()
			return fmt.Errorf("scan live stock: %w", err)
		}
		if live[color] == nil {
			live[color] = map[string]int{}
		}
		live[color][sizeKey(size)] = qty
		hasLive = true
	}
	stockRows.Close()
	if err := stockRows.Err(); err != nil {
		return fmt.Errorf("read live stock: %w", err)
	}
	fromJSON := loadStock || !hasLive

	// Child lists are rebuilt. Stock first (FK to colors/sizes), then colors/sizes, then the rest.
	for _, table := range []string{"product_stock", "product_colors", "product_sizes",
		"product_gallery", "product_activities", "product_highlights", "product_specs"} {
		if _, err := tx.Exec(`DELETE FROM `+table+` WHERE product_id=$1`, pid); err != nil {
			return fmt.Errorf("clear %s: %w", table, err)
		}
	}

	// Colors -> id map, with the photo swatch ({slug}-swatch.webp, made by
	// cmd/imgvariants) when it exists — else the hex dot is used.
	colorID := map[string]int{}
	swatches := 0
	for _, c := range p.Colors {
		folder := colorFolder(c.Name)
		var swatch interface{}
		if _, err := os.Stat(filepath.Join("public/images", brandSlug, slug, "images", folder, slug+"-swatch.webp")); err == nil {
			swatch = fmt.Sprintf("/images/%s/%s/images/%s/%s-swatch.webp", brandSlug, slug, folder, slug)
			swatches++
		}
		var id int
		if err := tx.QueryRow(`INSERT INTO product_colors (product_id, color, hex, swatch_url) VALUES ($1,$2,$3,$4) RETURNING id`,
			pid, c.Name, nz(c.Hex), swatch).Scan(&id); err != nil {
			return fmt.Errorf("color %s: %w", c.Name, err)
		}
		colorID[c.Name] = id
	}

	// Sizes = union of stock size keys, sorted numerically -> id map.
	sizeSet := map[string]bool{}
	for _, bySize := range p.Stock {
		for s := range bySize {
			sizeSet[s] = true
		}
	}
	sizes := make([]string, 0, len(sizeSet))
	for s := range sizeSet {
		sizes = append(sizes, s)
	}
	sort.Slice(sizes, func(i, j int) bool { return atof(sizes[i]) < atof(sizes[j]) })
	sizeID := map[string]int{}
	for _, s := range sizes {
		var id int
		if err := tx.QueryRow(`INSERT INTO product_sizes (product_id, eu_size) VALUES ($1,$2) RETURNING id`,
			pid, atof(s)).Scan(&id); err != nil {
			return fmt.Errorf("size %s: %w", s, err)
		}
		sizeID[s] = id
	}

	// Stock per colour × size: the JSON qty with -stock (or for a new product),
	// otherwise the live DB qty snapshotted above (0 for a newly added variant).
	jsonDiffs, inStock := 0, 0
	for color, bySize := range p.Stock {
		cid, ok := colorID[color]
		if !ok {
			return fmt.Errorf("stock references unknown colour %q", color)
		}
		for s, jsonQty := range bySize {
			qty := jsonQty
			if !fromJSON {
				qty = live[color][sizeKey(atof(s))]
				if qty != jsonQty {
					jsonDiffs++
				}
			}
			inStock += qty
			if _, err := tx.Exec(`INSERT INTO product_stock (product_id, size_id, color_id, qty) VALUES ($1,$2,$3,$4)`,
				pid, sizeID[s], cid, qty); err != nil {
				return fmt.Errorf("stock %s/%s: %w", color, s, err)
			}
		}
	}

	// Size chart (insole length/width, mm) — only when the JSON provides one, so
	// products whose chart comes from SQL seeds (Freet) are left untouched.
	if len(p.SizeChart) > 0 {
		if _, err := tx.Exec(`DELETE FROM size_chart WHERE product_id=$1`, pid); err != nil {
			return fmt.Errorf("clear size_chart: %w", err)
		}
		for s, row := range p.SizeChart {
			if _, err := tx.Exec(`INSERT INTO size_chart (product_id, eu_size, insole_length_mm, insole_width_mm) VALUES ($1,$2,$3,$4)`,
				pid, atof(s), row.Length, row.Width); err != nil {
				return fmt.Errorf("size chart %s: %w", s, err)
			}
		}
	}

	// Activities.
	for _, a := range p.Activities {
		if _, err := tx.Exec(`INSERT INTO product_activities (product_id, activity) VALUES ($1,$2)`, pid, a); err != nil {
			return fmt.Errorf("activity %s: %w", a, err)
		}
	}
	// Highlights (ordered).
	for i, h := range p.Highlights {
		if _, err := tx.Exec(`INSERT INTO product_highlights (product_id, sort_order, mk, sq, en) VALUES ($1,$2,$3,$4,$5)`,
			pid, i, h["mk"], h["sq"], h["en"]); err != nil {
			return fmt.Errorf("highlight %d: %w", i, err)
		}
	}
	// Specs (ordered).
	for _, s := range p.Specs {
		if _, err := tx.Exec(`INSERT INTO product_specs (product_id, key, value) VALUES ($1,$2,$3)`, pid, s.Key, s.Value); err != nil {
			return fmt.Errorf("spec %s: %w", s.Key, err)
		}
	}

	// Gallery: scan each colour folder (primary first). The primary colour's bare
	// hero is the page lead image (products.image_url), so it's not repeated here;
	// every OTHER colour leads with its own bare hero ({slug}.webp) so that hero
	// shows when the colour is selected, followed by its numbered shots.
	gallery := 0
	for i, c := range p.Colors {
		folder := colorFolder(c.Name)
		dir := filepath.Join("public/images", brandSlug, slug, "images", folder)
		var names []string
		if i > 0 {
			if _, err := os.Stat(filepath.Join(dir, slug+".webp")); err == nil {
				names = append(names, slug+".webp")
			}
		}
		names = append(names, galleryFiles(dir, slug)...)
		for _, name := range names {
			gallery++
			url := fmt.Sprintf("/images/%s/%s/images/%s/%s", brandSlug, slug, folder, name)
			if _, err := tx.Exec(`INSERT INTO product_gallery (product_id, image_url, sort_order) VALUES ($1,$2,$3)`,
				pid, url, gallery); err != nil {
				return fmt.Errorf("gallery %s: %w", name, err)
			}
		}
	}

	// ── Summary line ──
	stockNote := fmt.Sprintf("%d in stock (live DB)", inStock)
	if fromJSON {
		stockNote = fmt.Sprintf("%d in stock (from JSON)", inStock)
	} else if jsonDiffs > 0 {
		stockNote += fmt.Sprintf(", JSON differs on %d variant(s)", jsonDiffs)
	}
	sale := ""
	if onSale {
		sale = fmt.Sprintf(" SALE was €%d/%d MKD -%.0f%%", *origEUR, *origMKD, *discountPct)
	}
	pub := "published"
	if !e.Published {
		pub = "HIDDEN"
	}
	fmt.Printf("  %-11s €%-4d %5d MKD%s | %-11s | %s | %d col (%d swatch), %d sizes, %d gallery%s | %s%s\n",
		slug, e.Price, priceMKD, sale, e.Mode, pub, len(p.Colors), swatches, len(sizeID), gallery,
		chartNote(p), stockNote, profitNote(slug, e, costs))

	if commit {
		return tx.Commit()
	}
	return tx.Rollback()
}

func chartNote(p product) string {
	if len(p.SizeChart) > 0 {
		return ", size chart"
	}
	return ""
}

// profitNote estimates profit per pair from costs.json: price − (cost × (1 +
// customs%) + shipping). Empty when the product has no cost entry.
func profitNote(slug string, e catalogEntry, c costsFile) string {
	pc, ok := c.Products[slug]
	if !ok || pc.Cost <= 0 {
		return ""
	}
	customs := c.CustomsPct
	if pc.CustomsPct != nil {
		customs = *pc.CustomsPct
	}
	landed := pc.Cost*(1+customs/100) + c.ShippingEUR
	return fmt.Sprintf(" | landed ~€%.0f → profit ~€%.0f/pair", landed, float64(e.Price)-landed)
}

func atof(s string) float64 { f, _ := strconv.ParseFloat(s, 64); return f }

// nz maps "" to SQL NULL for nullable text columns.
func nz(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
