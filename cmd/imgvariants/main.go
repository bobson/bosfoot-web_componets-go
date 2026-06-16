// Command imgvariants generates small "-card" WebP variants of primary product
// images so the listing/home product cards can serve ~500px images via srcset
// instead of the full 1000px source.
//
// A primary product image is the file whose name matches its product directory,
// e.g. public/images/freet/chamois/images/chamois.webp → chamois-card.webp.
// Brand-level assets (logos, heroes) and gallery shots are left untouched.
//
// Idempotent: a variant that already exists is skipped, so it's safe to re-run
// after adding a new brand's images (mirrors the db/ import flow). Pass -force to
// regenerate existing variants too — needed when a source image is replaced in
// place. Requires ImageMagick (`magick`) on PATH.
//
//	go run ./cmd/imgvariants          # generate only missing variants
//	go run ./cmd/imgvariants -force   # also overwrite existing variants
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	imagesRoot   = "public/images"
	variantWidth = 500
	variantSfx   = "-card"
	quality      = "82"
)

func main() {
	force := flag.Bool("force", false, "regenerate variants even if they already exist")
	flag.Parse()

	if _, err := exec.LookPath("magick"); err != nil {
		fmt.Fprintln(os.Stderr, "error: ImageMagick (`magick`) not found on PATH")
		os.Exit(1)
	}

	var generated, skipped int
	err := filepath.Walk(imagesRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !isPrimaryProductImage(path) {
			return nil
		}

		variant := variantPath(path)
		if !*force {
			if _, err := os.Stat(variant); err == nil {
				skipped++
				return nil
			}
		}

		// -resize WxH> only shrinks (never upscales) and preserves aspect ratio.
		cmd := exec.Command("magick", path,
			"-resize", fmt.Sprintf("%dx%d>", variantWidth, variantWidth),
			"-quality", quality, variant)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("magick failed for %s: %v\n%s", path, err, out)
		}
		fmt.Printf("generated %s\n", variant)
		generated++
		return nil
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	fmt.Printf("done: %d generated, %d already present\n", generated, skipped)
}

// isPrimaryProductImage reports whether path is a product's primary image — a
// .webp whose basename equals the product slug. Handles both layouts:
//   - .../slug/images/slug.webp          (flat, no colour subfolder)
//   - .../slug/images/colour/slug.webp   (colour subfolder per variant)
func isPrimaryProductImage(path string) bool {
	if strings.ToLower(filepath.Ext(path)) != ".webp" {
		return false
	}
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if strings.HasSuffix(name, variantSfx) {
		return false
	}
	// Two levels up: .../slug/images/slug.webp
	if name == filepath.Base(filepath.Dir(filepath.Dir(path))) {
		return true
	}
	// Three levels up: .../slug/images/colour/slug.webp
	return name == filepath.Base(filepath.Dir(filepath.Dir(filepath.Dir(path))))
}

// variantPath turns /a/b/x.webp into /a/b/x-card.webp.
func variantPath(path string) string {
	ext := filepath.Ext(path)
	return strings.TrimSuffix(path, ext) + variantSfx + ext
}
