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

# Seed the "coming soon" brands (Groundies, Xero, Vivobarefoot) + translations (idempotent)
go run ./cmd/addbrands

# Load foot-health articles from db/articles.json into the DB (idempotent, upserts translations)
go run ./cmd/articleimport

# Import products from data/{brand}/products/*.json into the DB (euro source price;
# MKD derived via site.MKD; gallery/primary image scanned from public/images).
# DEFAULT IS DRY-RUN (builds each product in a tx, rolls back, prints a summary).
# Pass -commit to actually write — only AFTER the euro-aware code is deployed,
# since it sets price_mkd to the euro-derived value that old deployed code reads.
go run ./cmd/shoeimport          # dry-run / preview
go run ./cmd/shoeimport -commit  # apply (post-deploy)

# Generate ~500px "-card" WebP variants of primary product images (idempotent;
# needs ImageMagick). Run after adding a brand's images.
go run ./cmd/imgvariants

# Same, but overwrite existing variants — use only after replacing a product
# photo in place (a plain run would skip the stale variant).
go run ./cmd/imgvariants -force

# Export reservations: writes reservations_to_order.csv (qty per product/size/
# color, i.e. what to order from suppliers) + reservations_contacts.csv (who to
# contact) and prints a "to order" summary. Used during the reservation phase.
go run ./cmd/reservations

# Read orders to the terminal, newest first, with contact + line items.
# -status <s> filters by status, -limit N shows only the N most recent.
go run ./cmd/orders

# Run tests (internal/locale, models, internal/middleware)
go test ./...

# Lint + format browser JS with Biome (the only JS toolchain; no build step).
# Lint is scoped to a narrow rule set (use-before-declaration, redeclare) that
# catches RUNTIME-fatal mistakes a parse check can't see — e.g. a local var
# shadowing an import (TDZ). Biome also parses each module, so it subsumes the
# old scripts/check-js.sh syntax gate (now removed). CI gates deploy on `check`.
# Config in biome.json; widen lint rules / formatter scope there if wanted.
npm ci             # first time / in CI, installs the pinned Biome
npm run check      # lint + verify formatting (what CI runs); read-only
npm run format     # apply formatting in place
npm run lint       # lint only

# Traffic report from Caddy's JSON access log (run ON the droplet; needs sudo + jq).
# Shows total requests, unique real visitors (Cloudflare Cf-Connecting-Ip, minus the
# facebookexternalhit crawler), top pages, views/hour, and visitors by country.
# Optional arg is any journalctl --since value (default "24 hours ago").
sudo bash scripts/traffic.sh            # last 24h
sudo bash scripts/traffic.sh "today"    # since midnight

# Funnel snapshot (run ON the droplet; needs sudo + jq). Real-customer view of
# visitors → product views → add-to-cart → checkout → reservations, with bots AND
# the owner's test device/email filtered out (tweak the filters atop the script).
# Caddy log = page traffic; bosfoot log = add-to-cart beacons + placed orders.
sudo bash scripts/funnel.sh             # last 24h
sudo bash scripts/funnel.sh "7 days ago"
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
- `main.go` — initialises logger → connects DB → loads locale strings → parses templates → registers routes (`internal/routes.Register`) → starts an `http.Server` with read/write/idle timeouts and graceful shutdown (drains in-flight requests on SIGINT/SIGTERM, 15s window). Dependencies are injected into the handler structs (no global DB/logger/renderer); routes are registered on the default `http.ServeMux`.
- `handlers/` — three handler structs:
  - `ProductHandler{DB, Logger}` — JSON API handlers
  - `OrderHandler{DB, Logger}` — order-creation API (`POST /api/orders`)
  - `PageHandler{DB, Logger, Renderer, UI, SiteURL}` — SSR page handlers + sitemap
- `models/` — plain Go structs with JSON tags, one file per resource. Nullable DB columns use pointer fields (`*string`, `*int`, `*float64`) for clean `omitempty` JSON. `PasswordHash` is deliberately absent from `User`. `Order.Validate()` validates guest-checkout submissions.
- `internal/database/connect.go` — shared Aiven connection (upgrades `sslmode=require` → `verify-ca` using `ca.pem`). Used by server and `cmd/` utilities.
- `internal/locale/locale.go` — at startup loads the flat `public/locales/{mk,sq,en}.json` plus the language-grouped `public/locales/common.json` and `pages/*.json`, merging all into one key namespace. Provides `T(locale, key)` for template translations and `FromPath(segment)` / `IsValid(loc)` for URL parsing. Covered by `locale_test.go` (incl. a key-parity drift guard).
- `internal/routes/routes.go` — `Register(...)` wires every route onto the default `http.ServeMux` (see Routing below). Decoupled from `main.go`.
- `internal/middleware/csrf.go` — CSRF protection for state-changing API endpoints via the double-submit cookie pattern. `WithCSRFCookie` issues a random `_csrf` cookie on the checkout page GET (applied *outside* the page cache so both cache HIT and MISS set it); `WrapCSRF` rejects any POST/PUT/DELETE whose `X-CSRF-Token` header doesn't match the cookie. The checkout JS reads the cookie and sends it as the header.
- `internal/tmpl/renderer.go` — parses `templates/partials/*.html` then `templates/pages/*.html` at startup. `Render` executes into a buffer first so a template error becomes a clean 500 instead of a half-written 200. Template functions: `t` (translation), `mkd` (MKD price formatter), `eur` (EUR converter), `showEUR`, `deref` (*string → string), `derefF` (*float64 → trimmed string), `lower`, `json` (marshal for `<script type="application/json">`), `nl2p` (double-newline text → `<p>`/`<br>`), `dict` (build map for sub-template data passing).
- `internal/cache/cache.go` — tiny concurrency-safe in-memory TTL cache keyed by URL path. `pc.Wrap(handler)` caches only GET responses that produce a 200 (redirects/404/500/non-GET pass through). SSR pages are byte-identical per visitor (cart is client-side), so a hit skips both DB queries and rendering. 60s TTL keeps prices/stock fresh with no explicit invalidation; `Flush()` exists for after `cmd/dbimport`. Sets `X-Cache: HIT|MISS`. Only the body + Content-Type are cached — `Set-Cookie` passes through, which is why `WithCSRFCookie` wraps *outside* the cache.
- `logger/` — structured JSON logging via `log/slog`. Every record (`Info` and `Error`) is written to both stdout (visible via journalctl/docker logs) and `bosfoot.log` through an `io.MultiWriter`. `Close()` is deferred in `main`. Usage: `Logger.Info(msg, keysAndValues...)`, `Logger.Error(msg, err, keysAndValues...)`.

**Security**:
- **SQL injection:** all queries are parameterized (`$1`, `$2`) via `QueryContext` / `ExecContext`. Never concatenate user input into SQL.
- **CSRF:** see `internal/middleware/csrf.go` above — all state-changing endpoints (`POST /api/orders`) require a matching `X-CSRF-Token` header.
- **Input validation:** order submissions are validated server-side via `models.Order.Validate()`; the server also re-prices every order from the DB rather than trusting client totals.

**Database** (Aiven PostgreSQL, Frankfurt region):
- Schema in `db/schema.sql` — run `db/seed.sql` then `db/freet.sql` after.
- Pricing: the **euro price is the source** (round numbers, e.g. €135); MKD is derived and stored as INTEGER `price_mkd = floor(EUR × 61 / 10) × 10` (ends in 0). Rate is `site.MKDtoEUR` (= 61) in `internal/site` — the single source. EUR is shown back as a **whole number** (`round(price_mkd / 61)`, recovers the source €) on `sq`/`en` only, hidden on `mk`. MKD floor helper: `site.FloorDenar` (e.g. 5994 → 5990).
- Product `slug` is bare form (`vibe-2`), unique per brand via `UNIQUE(brand_id, slug)`. URL scheme: `/{locale}/products/{brand-slug}/{product-slug}`.
- Translated text lives in `_translations` tables keyed by `lang_code` enum (`mk`, `sq`, `en`). Product name/SKU, brand name/SKU, colors, specs, activities always EN.
- Stock tracked per `(product_id, size_id, color_id)` in `product_stock`.
- Orders support guest checkout: `user_id` nullable; contact + shipping snapshotted on every order.

**Routing** (registered in `internal/routes/routes.go`):
- `GET /api/brands` — JSON brand list
- `GET /api/products` — JSON listing with colors (bulk `ANY($1)` query, no N+1)
- `GET /api/products/{id}` — JSON full detail (sequential queries)
- `POST /api/orders` — guest-checkout order creation (CSRF-protected, validated via `models.Order.Validate()`)
- `GET /{mk,sq,en}` — SSR homepage (registered per-locale, not via `{locale}` wildcard)
- `GET /{locale}/products` — SSR product listing page (with filter facets)
- `GET /{locale}/products/{brand}/{slug}` — SSR product detail page
- `GET /{locale}/brands` — SSR brands listing
- `GET /{locale}/size-guide`, `/{locale}/about` — SSR static content pages (rendered from locale strings)
- `GET /{locale}/checkout` — SSR checkout page (static chrome; order summary rendered client-side from the cart). Wrapped in `WithCSRFCookie` to seed the `_csrf` cookie.
- `GET /{locale}/foot-health` — SSR foot-health hub (links out to article pages)
- `GET /{locale}/articles`, `/{locale}/articles/{slug}` — SSR article listing + detail (per-locale slugs)
- `GET /sitemap.xml` — dynamic multilingual sitemap (only lists live routes)
- `GET /` → redirect to `/mk`; all other paths → `public/` file server
- All SSR page handlers are wrapped in `pc.Wrap` (the 60s page cache); `sitemap.xml` and the JSON API are not cached.

**SSR + SEO** (`templates/`):
- Every SSR page template calls `{{template "head" .}}` which emits canonical + all three hreflang alternates + `x-default` using `.SiteURL` and `.CurrentPath`. This is the fix for Google treating language variants as duplicate pages.
- URL slugs for products, brands, and most static pages are identical across all locales (e.g. `/mk/size-guide`, `/sq/size-guide`, `/en/size-guide`). The `head.html` prefix-swap handles hreflang correctly for all of these automatically.
- **Exception**: article pages have per-language translated slugs (`article_translations.slug`), so the sitemap and hreflang for article pages emit real per-locale alternate URLs (from `PageBase.Alternates`) instead of prefix-swapping.
- Sitemap only lists registered live routes. Add entries to `handlers/sitemap_handler.go` as new SSR routes are registered.

**Templates** (`templates/`):
- Partials: `head.html` (meta/canonical/hreflang/CSS), `nav.html` (language switcher + cart button), `footer.html`, `product_card.html`
- Pages: `home.html`, `products.html` (listing + filter facets), `product.html` (detail), `brands.html`, `size-guide.html`, `about.html`, `checkout.html`, `foot-health.html`, `articles.html`, `article.html`
- Template data structs live in `handlers/page_handler.go`. Every page embeds `PageBase{Locale, CurrentPath, SiteURL, MetaDescription, Alternates}`.
- `ProductDetail` fetches its ~9 related-data queries (translations, colors, sizes, gallery, specs, highlights, stock, size chart, related products) concurrently via `errgroup` with `SetLimit(4)` — capped so one request can't exhaust the 8-connection Aiven pool. Each goroutine writes a distinct field of the product struct, so there's no shared-memory race.
- Sub-templates that need locale + the current item use `dict`: `{{template "product_card" (dict "Product" . "Locale" $.Locale)}}` — necessary because `range` replaces `.` with the loop item.

**Frontend** (`public/`, no build step — files served as-is):
- Interactivity is built as **HTML Web Components** (custom elements, light DOM, no shadow root) in `public/components/`: `cart-drawer`, `checkout-form`, `listing-filter`, `nav-drawer`, `nav-locale`, `nav-search`, `product-detail`, `scroll-reveal`. The static chrome is server-rendered inside each element; the JS class only adds behaviour. Because there's no shadow boundary, global CSS styles them normally and SSR HTML stays crawlable.
- `public/app.js` is the single entry point (ES module): it imports and runs each component's `init*` and maintains the cart badge. Each `init*` is page-guarded, so loading them all everywhere is safe — do **not** also add per-component `<script>` tags in templates (they'd be redundant; the module cache means they wouldn't re-run anyway).
- Cart state lives in `localStorage` under `bosfoot_cart`. Client-side price formatting mirrors the server: the EUR rate is **not** hardcoded in JS — it's injected via the `data-eur-rate` attribute (from `site.MKDtoEUR`); the cart/checkout JS also floors to the nearest 10 like `site.FloorDenar`. To change the rate, edit `internal/site` only.
- CSS is split per feature in `public/css/` (`global.css` holds the design-token `:root`, plus `nav`, `home`, `listing`, `product`, `cart`, `search`, `footer`). Use design tokens for all values; only define tokens that are actually used.

**Locale strings** (`public/locales/`):
- Two co-existing layouts, both merged into one flat key namespace at startup (see `locale.go`):
  - **Language-grouped** (preferred): `pages/{page}.json` per page + `common.json` for cross-cutting chrome (nav, footer, cart, search, cta, …). Each key holds all three languages together: `"contact.title": {"mk":…, "sq":…, "en":…}`. Add/remove a page's text in one file.
  - **Flat** `{mk,sq,en}.json` — the original per-language files, now **removed**. The loader still loads them *if present* (optional, used by `internal/locale/testdata`), so all live strings come from the grouped files.
- Rendered via `{{t .Locale "key"}}`; keys stay in one namespace, so which file a key lives in is purely organizational.
- Missing keys fall back to `mk`, then return the key itself — visible in the UI as a signal to add the translation. `locale_test.go` has a drift guard that fails if a key is missing from any language.

**Conventions**:
- New SSR page: add handler method on `PageHandler`, embed `PageBase`, call `h.Renderer.Render(w, "template-name", data)`, register route in `internal/routes/routes.go` (wrap in `pc.Wrap` if the page is the same for every visitor), add to sitemap in `sitemap_handler.go`.
- New Web Component: add `public/components/{name}.js` as a custom element (light DOM, no shadow), server-render its static chrome inside the element, add its `init*` to the array in `public/app.js`, and add matching styles in `public/css/`.
- New JSON API endpoint: add method on `ProductHandler` (or new `XHandler`), register in `internal/routes/routes.go`. Queries go directly in handlers — no repository layer. Wrap state-changing endpoints in `middleware.WrapCSRF`.
- Product data source (Freet): each product lives in `data/{brand}/products/{slug}.json` — **euro `price`** (the source), colours, per-colour stock, specs, activities, highlights, translations. `data/` is **outside `public/`** so it isn't web-served. Edit a price/field there → `go run ./cmd/shoeimport -commit` upserts it into the DB; MKD is derived via `site.MKD`, and the gallery/primary image are scanned from `public/images/{brand}/{slug}/images/{colour}/` (first colour in the JSON = the card image). Brand row + `size_chart` still come from the SQL seed. (`db/freet.sql` is now legacy/seed; `shoeimport` is the live update path.)
- New brand image assets: add `public/images/{brand-slug}/{product}/images/{colour}/` webp files, then `go run ./cmd/imgvariants` to generate the `-card` variants.
- New locale string: add it to the relevant `public/locales/pages/{page}.json` (or `common.json` if shared), with all three languages (`{"mk":…, "sq":…, "en":…}`).
- `db/` SQL files are idempotent (`ON CONFLICT DO NOTHING`) — safe to re-run.
