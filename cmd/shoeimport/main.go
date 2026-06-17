// Command shoeimport makes data/ the source of truth for products: it reads
// data/{brand}/products/*.json and upserts every product into the database,
// deriving the gallery + primary image by scanning public/images. Re-running is
// safe and self-healing (ON CONFLICT DO UPDATE for the parent row; child rows
// are rebuilt), so the flow is: edit a data/ JSON in euro → run this → DB matches.
//
//	go run ./cmd/shoeimport
//
// Euro is the source price; MKD is derived via site.MKD (legacy price_mkd kept
// in sync during the transition). Brand row + size_chart still come from the SQL
// seed for now. Run from the project root so data/ and public/ resolve.
package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"bosfoot/internal/database"
	"bosfoot/internal/site"
)

const brandSlug = "freet"

type product struct {
	Name   string `json:"name"`
	Price  int    `json:"price"` // EUR (source)
	Gender string `json:"gender"`
	Status struct {
		Active    bool `json:"active"`
		Published bool `json:"published"`
		Featured  bool `json:"featured"`
	} `json:"status"`
	SortOrder    int                          `json:"sortOrder"`
	Activities   []string                     `json:"activities"`
	Highlights   []map[string]string          `json:"highlights"`
	Colors       []struct{ Name, Hex string } `json:"colors"`
	Stock        map[string]map[string]int    `json:"stock"` // color -> size -> qty
	Specs        []struct{ Key, Value string } `json:"specs"`
	Translations map[string]struct {
		Description    string `json:"description"`
		About          string `json:"about"`
		SizeAndFit     string `json:"sizeAndFit"`
		ProductInfo    string `json:"productInfo"`
		Sustainability string `json:"sustainability"`
	} `json:"translations"`
}

// colorFolder slugifies a colour name to its image folder: "Brown Black" -> "brown-black".
func colorFolder(name string) string {
	return strings.ToLower(strings.ReplaceAll(name, " ", "-"))
}

var gallerySuffix = regexp.MustCompile(`-(\d+)\.webp$`)

// galleryFiles returns the numbered gallery images in dir for slug, sorted by
// their numeric suffix, excluding the bare primary ({slug}.webp) and the -card variant.
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
		if !strings.HasPrefix(name, slug+"-") || strings.HasSuffix(name, "-card.webp") {
			continue
		}
		m := gallerySuffix.FindStringSubmatch(name)
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
	// Default is a dry run (each product is built in a transaction then rolled
	// back) because committing changes price_mkd on the shared/live DB before the
	// euro-aware code deploys. Pass -commit at the coordinated deploy.
	commit := flag.Bool("commit", false, "actually write to the DB (default: dry-run, rolled back)")
	flag.Parse()

	db, err := database.Connect()
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	var brandID, catID int
	if err := db.QueryRow(`SELECT id FROM brands WHERE slug=$1`, brandSlug).Scan(&brandID); err != nil {
		log.Fatalf("brand %q not found: %v", brandSlug, err)
	}
	if err := db.QueryRow(`SELECT id FROM categories WHERE slug='shoes'`).Scan(&catID); err != nil {
		log.Fatalf("category 'shoes' not found: %v", err)
	}

	files, _ := filepath.Glob(filepath.Join("data", brandSlug, "products", "*.json"))
	if len(files) == 0 {
		log.Fatal("no product files under data/" + brandSlug + "/products/")
	}
	sort.Strings(files)

	if !*commit {
		fmt.Println("DRY RUN (no changes written — pass -commit to apply)")
	}
	for _, path := range files {
		slug := strings.TrimSuffix(filepath.Base(path), ".json")
		if err := importProduct(db, brandID, catID, slug, path, *commit); err != nil {
			log.Fatalf("%s: %v", slug, err)
		}
	}
	if *commit {
		fmt.Printf("Committed %d products from data/%s.\n", len(files), brandSlug)
	} else {
		fmt.Printf("Dry run complete: %d products (rolled back).\n", len(files))
	}
}

func importProduct(db *sql.DB, brandID, catID int, slug, path string, commit bool) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var p product
	if err := json.Unmarshal(raw, &p); err != nil {
		return err
	}
	if len(p.Colors) == 0 {
		return fmt.Errorf("no colors")
	}

	sku := "FREET-" + strings.ToUpper(slug)
	var genderID int
	if err := db.QueryRow(`SELECT id FROM genders WHERE value=$1`, p.Gender).Scan(&genderID); err != nil {
		return fmt.Errorf("gender %q: %w", p.Gender, err)
	}
	primaryFolder := colorFolder(p.Colors[0].Name)
	imageURL := fmt.Sprintf("/images/freet/%s/images/%s/%s.webp", slug, primaryFolder, slug)
	priceMKD := site.MKD(p.Price)

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var pid int
	err = tx.QueryRow(`
		INSERT INTO products
			(sku, name, slug, brand_id, category_id, gender_id,
			 price_mkd, price_eur, image_url, is_active, is_published, is_featured, sort_order)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		ON CONFLICT (sku) DO UPDATE SET
			name=EXCLUDED.name, slug=EXCLUDED.slug, gender_id=EXCLUDED.gender_id,
			price_mkd=EXCLUDED.price_mkd, price_eur=EXCLUDED.price_eur,
			image_url=EXCLUDED.image_url, is_active=EXCLUDED.is_active,
			is_published=EXCLUDED.is_published, is_featured=EXCLUDED.is_featured,
			sort_order=EXCLUDED.sort_order, updated_at=now()
		RETURNING id
	`, sku, p.Name, slug, brandID, catID, genderID,
		priceMKD, p.Price, imageURL, p.Status.Active, p.Status.Published, p.Status.Featured, p.SortOrder).Scan(&pid)
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

	// Child lists are rebuilt. Stock first (FK to colors/sizes), then colors/sizes, then the rest.
	for _, table := range []string{"product_stock", "product_colors", "product_sizes",
		"product_gallery", "product_activities", "product_highlights", "product_specs"} {
		if _, err := tx.Exec(`DELETE FROM `+table+` WHERE product_id=$1`, pid); err != nil {
			return fmt.Errorf("clear %s: %w", table, err)
		}
	}

	// Colors -> id map.
	colorID := map[string]int{}
	for _, c := range p.Colors {
		var id int
		if err := tx.QueryRow(`INSERT INTO product_colors (product_id, color, hex) VALUES ($1,$2,$3) RETURNING id`,
			pid, c.Name, nz(c.Hex)).Scan(&id); err != nil {
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

	// Stock per colour × size.
	for color, bySize := range p.Stock {
		cid, ok := colorID[color]
		if !ok {
			return fmt.Errorf("stock references unknown colour %q", color)
		}
		for s, qty := range bySize {
			if _, err := tx.Exec(`INSERT INTO product_stock (product_id, size_id, color_id, qty) VALUES ($1,$2,$3,$4)`,
				pid, sizeID[s], cid, qty); err != nil {
				return fmt.Errorf("stock %s/%s: %w", color, s, err)
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

	// Gallery: scan each colour folder (primary first), numbered files only.
	sortOrder := 0
	for _, c := range p.Colors {
		folder := colorFolder(c.Name)
		dir := filepath.Join("public/images/freet", slug, "images", folder)
		for _, name := range galleryFiles(dir, slug) {
			sortOrder++
			url := fmt.Sprintf("/images/freet/%s/images/%s/%s", slug, folder, name)
			if _, err := tx.Exec(`INSERT INTO product_gallery (product_id, image_url, sort_order) VALUES ($1,$2,$3)`,
				pid, url, sortOrder); err != nil {
				return fmt.Errorf("gallery %s: %w", name, err)
			}
		}
	}

	fmt.Printf("  %-12s €%-4d → %d MKD | %d colours, %d sizes, %d gallery, %d specs | %s\n",
		slug, p.Price, priceMKD, len(p.Colors), len(sizeID), sortOrder, len(p.Specs), imageURL)

	if commit {
		return tx.Commit()
	}
	return tx.Rollback()
}

func atof(s string) float64 { f, _ := strconv.ParseFloat(s, 64); return f }

// nz maps "" to SQL NULL for nullable text columns.
func nz(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
