# Bosfoot — Commands

Every command runs **from the project root** (`.env`, `ca.pem`, `public/`, `templates/`
are resolved relative to the working directory). Each Go tool can be run two ways:

- **Direct:** `go run ./cmd/<tool> [flags]`
- **npm wrapper:** `npm run <script> -- [flags]` — note the `--`, everything after it is
  passed to the Go tool. `npm run help` prints a short one-line list of every script.

**Environment** (`.env`, gitignored): `DATABASE_URL`, `PG_CA_CERT=ca.pem`, `SITE_URL`.
Set `SITE_URL=http://localhost:8080` for local dev, the live domain in prod. See
`CLAUDE.md` for architecture details.

Tools that write to the shared/live database **default to a dry-run** and print what
they *would* do; pass `-commit` to actually apply.

---

## Run the app

| npm | Direct | What |
|-----|--------|------|
| `npm run dev` | `go tool air` | Live-reload server (watches `.go`, `.html`, `.json`). |
| `npm run serve` | `go run main.go` | Server, no live reload. |
| — | `go build -o bosfoot .` | Build the production binary. |
| — | `go test ./...` | Run all Go tests. |

---

## Database setup & migrations

| npm | Direct | What |
|-----|--------|------|
| `npm run db-ping` | `go run ./cmd/dbping` | Test the DB connection. |
| `npm run db-import` | `go run ./cmd/dbimport` | Load schema + seed + all migrations (idempotent, safe to re-run). Skips `schema.sql`/`seed.sql` once the DB is initialised; always runs the additive migrations. |
| `npm run add-brands` | `go run ./cmd/addbrands` | Seed the "coming soon" brands (Be Lenka, Groundies) + translations. |
| `npm run article-import` | `go run ./cmd/articleimport` | Load foot-health articles from `db/articles.json` (upserts translations). |

> **Migration-first deploys:** for changes where new code reads new columns (e.g. the
> reviews model), run `db-import` against **prod first**, then push the code — otherwise
> pages 500. `db-import` is additive/backward-compatible, so running it early is safe.

---

## Products, prices & images

Product data lives in `data/{brand}/products/*.json` (euro `price` is the source; MKD is
derived via `internal/site`). `data/` is outside `public/` so it isn't web-served.

| npm | Direct | What |
|-----|--------|------|
| `npm run stock-import` | `go run ./cmd/shoeimport` | **Dry-run preview.** Builds each product in a rolled-back tx and prints a summary. |
| `npm run stock-import -- -commit` | `go run ./cmd/shoeimport -commit` | Apply: upsert every product from its JSON. |
| `npm run img-variants` | `go run ./cmd/imgvariants` | Generate ~500px `-card` WebP variants of primary images (idempotent; needs ImageMagick). |
| — | `go run ./cmd/imgvariants -force` | Same, but overwrite existing variants (use after replacing a photo in place). |

> **Pricing deploy order:** `shoeimport -commit` writes the euro-derived `price_mkd`, so
> apply it only **after** the euro-aware code is deployed.

---

## Stock / inventory

There are **two** ways stock changes, and they must not be mixed carelessly:

### `add-stock` — incremental, live-safe (use once selling)

Adds inventory to **existing** variants only; `qty = qty + delta` on the exact
`(product, colour, size)` rows you name. Touches nothing else — safe on a live shop.

```bash
# Preview (dry-run — writes nothing):
go run ./cmd/addstock -product vibe-2 -color Black -add "43:2,45:1,47:1"
npm run add-stock -- -product vibe-2 -color Black -add "43:2,45:1,47:1"

# Apply:
go run ./cmd/addstock -product vibe-2 -color Black -add "43:2,45:1,47:1" -commit
```

- `-add "size:qty,size:qty"` — quantities to add per size (qty may be negative for a
  correction; refused if it would drive a variant below 0).
- `-brand` defaults to `freet`. `-color` must match the stored name exactly (e.g. `Black`).
- Default dry-run; `-commit` applies atomically in one transaction.

### `stock-import` (shoeimport) — full rebuild from JSON

`shoeimport` **`DELETE`s and re-inserts** `product_stock` for every product from its JSON
`stock` block.

> ⚠️ **Pre-launch only for stock.** While the catalogue is in preorder mode nothing
> decrements DB stock, so a rebuild is harmless. **Once you are selling, do NOT use
> `shoeimport` to change stock** — it would wipe the sale decrements and reset quantities
> to the JSON numbers. Use `add-stock` for restocks instead; the **DB becomes the source
> of truth** for stock, and the JSON `stock` block is only the initial seed.

Restock workflow **pre-launch:** edit the JSON `stock`, then `stock-import -- -commit`.
Restock workflow **post-launch:** `add-stock -- ... -commit` (never `stock-import`).

---

## Orders

Order numbers display as `BF-<id+1000>` (e.g. internal id 64 → `BF-1064`); the commands
accept that same number back.

```bash
npm run orders                              # all orders, newest first
npm run orders -- -status pending           # to-ship queue
npm run orders -- -status delivered         # past / completed orders
npm run orders -- -limit 10                 # only the 10 most recent
npm run orders -- -ship BF-1064             # mark shipped
npm run orders -- -deliver BF-1064          # mark delivered (can skip shipped)
npm run orders -- -cancel BF-1064           # mark cancelled
```

Direct form: `go run ./cmd/orders [flags]`. Transitions are unconditional (jump straight
to delivered if you like). Requires the `cancelled` status migration — run `db-import` on
prod once before the first `-cancel`.

---

## Reviews

Guest-checkout reviews are verified by a single-use per-order **review token** emailed
after purchase; there are no accounts.

```bash
# Send post-purchase "leave a review" invites (DRY-RUN by default):
npm run review-invites                      # preview eligible orders
npm run review-invites -- -order 42         # just order 42, any age
npm run review-invites -- -commit           # create tokens + send

# Re-email an order's EXISTING link (e.g. the buyer lost the email). Requires
# -order; reuses the same token (no new link), so only useful if they haven't
# reviewed yet. Note: -order takes the raw id, not the BF- number (BF-1064 -> 64).
npm run review-invites -- -order 64 -resend -commit

# Moderate (reviews land 'pending', show only once approved):
npm run reviews                             # list pending
npm run reviews -- -approve 12              # approve #12 (emails the reviewer a thank-you)
npm run reviews -- -reject 12               # hide #12, keep the row
npm run reviews -- -delete 12               # permanently delete #12
```

Direct forms: `go run ./cmd/reviewinvites [flags]`, `go run ./cmd/reviews [flags]`.

> ⚠️ **Moderate reviews with photos ON THE DROPLET.** `-approve`/`-reject`/`-delete`
> move or delete the uploaded photo **files**, which live on the droplet under
> `UPLOADS_DIR`. Run them from `/srv/bosfoot` (so `.env`/`UPLOADS_DIR` load and the
> files are local):
> ```bash
> cd /srv/bosfoot && go run ./cmd/reviews -approve <id>
> ```
> Running them from your laptop still flips the DB status, but can't touch the
> files — you'll see `published 0/1 photo(s)` and the photo won't appear. Listing
> (`go run ./cmd/reviews`, `-status`, `-photo`) is DB-only and works anywhere.

---

## Reservations / supplier export

```bash
npm run reservations                        # go run ./cmd/reservations
```

Writes `reservations_to_order.csv` (qty per product/size/colour — what to order from
suppliers) + `reservations_contacts.csv` (who to contact), and prints a "to order" summary.

---

## Analytics — run **on the droplet** (needs `sudo` + `jq`)

These read the server's logs, so they run on the box, not locally:

```bash
sudo bash scripts/traffic.sh                # last 24h traffic (visitors, top pages, countries)
sudo bash scripts/traffic.sh "today"        # any journalctl --since value
sudo bash scripts/funnel.sh                 # last 24h funnel: visitors → views → cart → checkout → orders
sudo bash scripts/funnel.sh "7 days ago"
```

---

## Frontend JS toolchain (Biome — no build step)

```bash
npm ci                                      # install the pinned Biome (first time / CI)
npm run check                               # lint + verify formatting (what CI gates on)
npm run format                              # apply formatting in place
npm run lint                                # lint only
```

---

## Droplet / deploy notes

- **Caddy runs `/etc/caddy/Caddyfile`, not the repo copy.** The repo's `Caddyfile`
  is deployed to `/srv/bosfoot/Caddyfile` by `git reset --hard`, but Caddy loads
  `/etc/caddy/Caddyfile`. Keep them in sync with a symlink (one-time), then a
  reload picks up any repo change:
  ```bash
  sudo ln -sf /srv/bosfoot/Caddyfile /etc/caddy/Caddyfile
  sudo systemctl reload caddy
  ```
- **Review photo uploads need ImageMagick + `UPLOADS_DIR`** on the droplet:
  `apt install imagemagick`, `UPLOADS_DIR=/srv/bosfoot/uploads` in `.env`, and the
  dir writable by the app user. Photos are served by the Caddy `/uploads` block.
- **Cloudflare caches `.webp` at the edge.** If a photo 404s publicly but the
  origin serves it (`curl --resolve bosfoot.com:443:127.0.0.1 …` returns 200), it's
  a stale cached 404 — purge the Cloudflare cache.

## Quick reference

- `npm run help` — one-line list of every script.
- Pass tool flags after `--`: `npm run reviews -- -approve 12`.
- Write-tools default to **dry-run**; add `-commit` to apply.
- Post-launch stock changes: **`add-stock`**, never `stock-import`.
- Review-photo moderation (`-approve`/`-reject`/`-delete`): **on the droplet**.
