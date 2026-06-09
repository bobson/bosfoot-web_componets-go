# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

Bosfoot — a barefoot shoes e-commerce store. Go backend (standard library only, no framework) + Vanilla JS frontend. Three languages: MK (Macedonian, default), SQ (Albanian), EN (English). ~30 products, Freet brand is live with 5 products.

## Commands

```bash
# Run with live reload (watches .go, .html, .json)
go tool air

# Run without live reload — must be run from project root
go run main.go

# Build
go build -o bosfoot .

# Test DB connection
go run ./cmd/dbping

# Load schema + seed + brand data into Aiven (idempotent, safe to re-run)
go run ./cmd/dbimport

# Run tests
go test ./...
go test ./handlers/...
```

## Environment

`.env` in the project root (gitignored). All three must be set:
```
DATABASE_URL=postgres://avnadmin:PASSWORD@host:port/defaultdb?sslmode=require
PG_CA_CERT=ca.pem
SITE_URL=https://bosfoot.com   # use http://localhost:8080 for local dev
```
`SITE_URL` is used for canonical and hreflang tags in every page's `<head>`. Must be set to the live domain before launch — all SEO URLs update automatically.

Both the server and `cmd/` utilities must be run from the project root — `.env`, `ca.pem`, `public/locales/`, and `templates/` are resolved relative to cwd. Server exits on startup if DB ping fails or locale/template files can't be loaded.

## Architecture

**Backend** (`net/http`, no router library, Go 1.22+ path parameters):
- `main.go` — initialises logger → connects DB → loads locale strings → parses templates → registers routes → starts server. All dependencies injected; no globals.
- `handlers/` — two handler structs:
  - `ProductHandler{DB, Logger}` — JSON API handlers
  - `PageHandler{DB, Logger, Renderer, SiteURL}` — SSR page handlers + sitemap
- `models/` — plain Go structs with JSON tags, one file per resource. Nullable DB columns use pointer fields (`*string`, `*int`, `*float64`) for clean `omitempty` JSON. `PasswordHash` is deliberately absent from `User`.
- `internal/database/connect.go` — shared Aiven connection (upgrades `sslmode=require` → `verify-ca` using `ca.pem`). Used by server and `cmd/` utilities.
- `internal/locale/locale.go` — loads `public/locales/{mk,sq,en}.json` at startup. Provides `T(locale, key)` for template translations and `FromPath(segment)` / `IsValid(loc)` for URL parsing.
- `internal/tmpl/renderer.go` — parses `templates/partials/*.html` then `templates/pages/*.html` at startup. Registers template functions: `t` (translation), `mkd` (MKD price formatter), `eur` (EUR converter), `showEUR`, `deref` (*string → string), `dict` (build map for sub-template data passing).
- `logger/` — dual-output: INFO → stdout, ERROR → `bosfoot.log`. `Close()` deferred in `main`, not in `initializeLogger`.

**Database** (Aiven PostgreSQL, Frankfurt region):
- Schema in `db/schema.sql` — run `db/seed.sql` then `db/freet.sql` after.
- Prices stored as INTEGER MKD (no decimals). EUR = `price_mkd / 61.5`, shown with 2 decimals on `sq`/`en` locales only, hidden on `mk`.
- Product `slug` is bare form (`vibe-2`), unique per brand via `UNIQUE(brand_id, slug)`. URL scheme: `/{locale}/products/{brand-slug}/{product-slug}`.
- Translated text lives in `_translations` tables keyed by `lang_code` enum (`mk`, `sq`, `en`). Product name/SKU, brand name/SKU, colors, specs, activities always EN.
- Stock tracked per `(product_id, size_id, color_id)` in `product_stock`.
- Orders support guest checkout: `user_id` nullable; contact + shipping snapshotted on every order.

**Routing** (registered in `main.go`):
- `GET /api/products` — JSON listing with colors (bulk `ANY($1)` query, no N+1)
- `GET /api/products/{id}` — JSON full detail (10 sequential queries)
- `GET /{locale}/products` — SSR product listing page
- `GET /sitemap.xml` — dynamic multilingual sitemap (only lists live routes)
- `GET /` → redirect to `/mk/products`; all other paths → `public/` file server

**SSR + SEO** (`templates/`):
- Every SSR page template calls `{{template "head" .}}` which emits canonical + all three hreflang alternates + `x-default` using `.SiteURL` and `.CurrentPath`. This is the fix for Google treating language variants as duplicate pages.
- URL slugs for products, brands, and most static pages are identical across all locales (e.g. `/mk/size-guide`, `/sq/size-guide`, `/en/size-guide`). The `head.html` prefix-swap handles hreflang correctly for all of these automatically.
- **Exception**: article pages have per-language translated slugs (`article_translations.slug`). When articles are built, the sitemap and hreflang for article pages need real per-locale alternate URLs, not prefix-swapping.
- Sitemap only lists registered live routes. Add entries to `handlers/sitemap_handler.go` as new SSR routes are registered.

**Templates** (`templates/`):
- Partials: `head.html` (meta/canonical/hreflang/CSS), `nav.html` (language switcher + cart button), `footer.html`, `product_card.html`
- Pages: `products.html` (product listing)
- Template data structs live in `handlers/page_handler.go`. Every page embeds `PageBase{Locale, CurrentPath, SiteURL, MetaDescription}`.
- Sub-templates that need locale + the current item use `dict`: `{{template "product_card" (dict "Product" . "Locale" $.Locale)}}` — necessary because `range` replaces `.` with the loop item.

**Locale strings** (`public/locales/{mk,sq,en}.json`):
- Loaded server-side for template rendering via `{{t .Locale "key"}}`.
- Also served statically at `/locales/mk.json` etc. for future CSR use (cart, filters).
- Missing keys fall back to `mk`, then return the key itself — visible in the UI as a signal to add the translation.

**Conventions**:
- New SSR page: add handler method on `PageHandler`, embed `PageBase`, call `h.Renderer.Render(w, "template-name", data)`, register route in `main.go`, add to sitemap in `sitemap_handler.go`.
- New JSON API endpoint: add method on `ProductHandler` (or new `XHandler`), register in `main.go`. Queries go directly in handlers — no repository layer.
- New brand: add `public/images/{brand-slug}/` with `brand.json` + per-product `shoe.json`, write `db/{brand-slug}.sql` following `db/freet.sql` as template, run `go run ./cmd/dbimport`.
- New locale string: add to all three `public/locales/*.json` files.
- `db/` SQL files are idempotent (`ON CONFLICT DO NOTHING`) — safe to re-run.
