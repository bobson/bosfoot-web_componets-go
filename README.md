# Bosfoot

A barefoot shoe shop built for the Balkans. Macedonian by default, Albanian and English too.

I built this because finding quality barefoot shoes in this region meant ordering from abroad, paying high shipping, and hoping the sizing worked out. Bosfoot is the fix — a shop where you can actually talk to someone who wears the shoes they sell.

**Live at:** [bosfoot.com](https://bosfoot.com) *(coming soon)*

---

## What it is

A full-stack e-commerce store. Go on the backend, Vanilla JS on the frontend. No frameworks on either end — just the standard library, a PostgreSQL database on Aiven, and templates rendered server-side so Google can actually find the pages.

The shop sells barefoot shoes from carefully selected European brands. First up is Freet from the UK. More brands coming.

---

## Stack

- **Backend** — Go (`net/http`, standard library only)
- **Frontend** — Vanilla JS, server-side rendered HTML
- **Database** — PostgreSQL on Aiven (Frankfurt)
- **Languages** — Macedonian (default), Albanian, English
- **Hosting** — DigitalOcean (app) + Aiven (DB), both Frankfurt region

---

## Running it locally

You need Go installed and an Aiven PostgreSQL instance (or any Postgres, really — just skip the `ca.pem` part and use `sslmode=disable` locally if needed).

**1. Clone and set up environment**

```bash
git clone <repo>
cd bosfoot_vanilajs_go
cp .env.example .env  # then fill in your values
```

The `.env` needs three things:

```
DATABASE_URL=postgres://user:password@host:port/db?sslmode=require
PG_CA_CERT=ca.pem
SITE_URL=http://localhost:8080
```

If you're using Aiven, download the CA certificate from your service dashboard and put it in the project root as `ca.pem`.

**2. Set up the database**

```bash
go run ./cmd/dbping     # make sure the connection works first
go run ./cmd/dbimport   # loads schema, seed data, and Freet products
```

Both commands must be run from the project root. The import is idempotent — safe to run multiple times.

**3. Run**

```bash
go tool air       # with live reload (watches Go, HTML, JSON files)
# or
go run main.go    # without
```

Open [http://localhost:8080](http://localhost:8080) — it redirects to `/mk/products`.

---

## Project layout

```
cmd/            CLI utilities (dbping, dbimport) — not part of the server
db/             SQL files: schema, seed data, per-brand imports
handlers/       HTTP handlers — one struct per concern
internal/       Shared packages (database connection, locale, template renderer)
models/         Go structs matching DB tables
public/         Static files served directly (images, CSS, locale JSON)
  images/       Product and brand images, organized by brand/product slug
  locales/      UI strings in mk.json, sq.json, en.json
templates/      Server-rendered HTML templates
  partials/     Reusable pieces (nav, footer, product card, head)
  pages/        Full page templates
```

---

## Adding a new brand

1. Create `public/images/{brand-slug}/` with the brand and product images
2. Add a `brand.json` (brand info + size chart) and `shoe.json` per product — follow the Freet files as a reference
3. Write `db/{brand-slug}.sql` following `db/freet.sql` as a template
4. Run `go run ./cmd/dbimport`

---

## SEO notes

The shop is trilingual. Each language gets its own URL (`/mk/products`, `/sq/products`, `/en/products`). Every server-rendered page includes:

- A `canonical` tag pointing to its own URL
- `hreflang` tags linking all three language versions
- An `x-default` pointing to the Macedonian version

This tells Google these are translations of the same page, not duplicates. The sitemap at `/sitemap.xml` is generated dynamically from the database and only lists routes that actually exist.

When the domain goes live, set `SITE_URL=https://bosfoot.com` in the environment and submit the sitemap to Google Search Console.

---

## Author

Slobodan Markoski — building tools for the region I grew up in.
