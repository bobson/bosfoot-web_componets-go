// Command imgvariants generates small WebP variants of primary product images:
//   - "-card":   ~500px, so listing/home product cards serve it via srcset
//     instead of the full 1000px source.
//
// Colour-picker swatches ({slug}-swatch.webp) are NOT generated here: they are
// the brand's own colour tiles, converted with scripts/prep-images.sh (models
// without one — e.g. Freet — keep the hex dot). The suffix is still skipped
// below so a swatch file is never mistaken for a primary image.
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
	imagesRoot = "public/images"
	quality    = "82"
)

// variant is one derived image built from each primary product image.
type variant struct {
	suffix string
	args   []string // magick args between the input and output paths
}

var variants = []variant{
	// 500px WIDE (the card srcset declares 500w), any height — portrait photos
	// keep their shape; only shrinks, never upscales.
	{"-card", []string{"-resize", "500x>"}},
}

// skipSuffixes are derived files that must never count as a primary image.
var skipSuffixes = []string{"-card", "-swatch"}

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

		for _, v := range variants {
			out := variantPath(path, v.suffix)
			if !*force {
				if _, err := os.Stat(out); err == nil {
					skipped++
					continue
				}
			}
			args := append([]string{path}, v.args...)
			args = append(args, "-quality", quality, out)
			if msg, err := exec.Command("magick", args...).CombinedOutput(); err != nil {
				return fmt.Errorf("magick failed for %s: %v\n%s", path, err, msg)
			}
			fmt.Printf("generated %s\n", out)
			generated++
		}
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
	for _, sfx := range skipSuffixes {
		if strings.HasSuffix(name, sfx) {
			return false
		}
	}
	// Two levels up: .../slug/images/slug.webp
	if name == filepath.Base(filepath.Dir(filepath.Dir(path))) {
		return true
	}
	// Three levels up: .../slug/images/colour/slug.webp
	return name == filepath.Base(filepath.Dir(filepath.Dir(filepath.Dir(path))))
}

// variantPath turns /a/b/x.webp into /a/b/x{suffix}.webp, e.g. x-card.webp.
func variantPath(path, suffix string) string {
	ext := filepath.Ext(path)
	return strings.TrimSuffix(path, ext) + suffix + ext
}
