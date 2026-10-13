#!/usr/bin/env bash
# Convert supplier product photos into the site's image format, driven by a
# manifest of "src<TAB>dest" lines (blank lines and # comments ignored).
#
#   bash scripts/prep-images.sh data/sources/be-lenka-raw/manifest.tsv
#
# Output: WebP in the photo's ORIGINAL proportions (never cropped or padded to
# a square — lifestyle shots would lose the shoes), shrunk to at most 1000px
# wide; transparency flattened onto white.
# Name the dests the way shoeimport expects:
#   public/images/{brand}/{slug}/images/{colour}/{slug}.webp   (primary)
#   public/images/{brand}/{slug}/images/{colour}/{slug}-N.webp (gallery)
# Then run `go run ./cmd/imgvariants` to build the -card / -swatch variants.
# Needs ImageMagick 7 (`magick`). Overwrites existing dests.
set -euo pipefail

manifest="${1:?usage: prep-images.sh manifest.tsv}"
MAXW=1000

n=0
while IFS=$'\t' read -r src dest; do
  [[ -z "${src// /}" || "$src" == \#* ]] && continue
  mkdir -p "$(dirname "$dest")"
  magick "$src[0]" -background white -alpha remove -alpha off \
    -resize "${MAXW}x>" -quality 82 "$dest"
  n=$((n + 1))
  echo "$dest"
done <"$manifest"
echo "converted $n images"
