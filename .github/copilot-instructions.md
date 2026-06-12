# Copilot instructions for Bosfoot (bosfoot_vanilajs_go)

This file is tuned for future Copilot sessions working on this repository. It focuses on concrete commands, the big-picture architecture, and project-specific conventions that matter when editing code or adding features.

---

## Quick commands

- Dev (live reload):
  - go tool air    # watches .go, .html, .json
- Run (no reload):
  - go run main.go
- Build binary:
  - go build -o bosfoot .
- DB
  - go run ./cmd/dbping       # verify DB connection (must run from repo root)
  - go run ./cmd/dbimport     # idempotent: loads schema, seed, brand SQL files
  - go run ./cmd/articleimport# loads/updates articles from db/articles.json
- Tests
  - Run all: go test ./...
  - Run package: go test ./handlers
  - Run a single test: go test ./handlers -run ^TestName$ -v

Notes: All `cmd/` utilities and server must be run from the project root so .env, ca.pem, public/locales and templates resolve correctly.

---

## High-level architecture (big picture)

- Backend: Go (net/http, no third-party router). main.go wires logger → DB → locale UI → templates → routes.
- Handlers: `handlers/` contains three primary handler structs:
  - ProductHandler (JSON API)
  - OrderHandler (order creation API)
  - PageHandler (SSR pages + sitemap)
- Templates: `internal/tmpl` parses `templates/partials/*.html` then `templates/pages/*.html`. Templates expose helper funcs (t, mkd, eur, showEUR, deref, derefF, json, nl2p, dict).
- Locales: `internal/locale` loads `public/locales/{mk,sq,en}.json` into a UI used by templates and PageHandler.
- DB connection: `internal/database.Connect()` builds a TLS-verified Aiven connection from env vars and pings the DB. It also sets conservative pool limits.
- Caching: `internal/cache` provides a small in-memory TTL cache (pc.Wrap) for GET SSR pages (60s). API and sitemap are not cached.
- Frontend: `public/` contains static assets and Web Components (light DOM, no shadow root). No build step; files served as-is.
- CLI utilities: `cmd/dbimport`, `cmd/dbping`, `cmd/articleimport` are idempotent helpers that expect to run from repo root.
- CI/CD: `.github/workflows/deploy.yml` performs an SSH deploy on push to main (builds binary on droplet and restarts systemd service).

---

## Key repository-specific conventions (essential to follow)

- Run tools from the project root. Relative paths (templates, public/locales, ca.pem) depend on cwd.
- Locale URLs: three fixed codes (`/mk`, `/sq`, `/en`). Use `locale.FromPath()` / `locale.IsValid()` when parsing.
- Translations: UI strings in `public/locales/*.json`. Missing keys fallback to MK then the key itself (visible in UI).
- Template parsing order: partials parsed before pages so sub-templates are available by name.
- Template helpers: use `dict` to pass multiple named values into sub-templates instead of relying on `.` binding.
- Product detail: gathers ~9 related DB queries concurrently using errgroup with SetLimit(4). Do not increase concurrency beyond pool capacity without adjusting DB pool.
- DB pool: Connect() sets MaxOpenConns=8 (chosen for the Aiven plan). Any code that fans out DB work must cap its concurrency.
- Caching: Only GET responses with a full 200 are cached. Use `cache.Flush()` after imports (`cmd/dbimport`). Cache sets `X-Cache: HIT|MISS` header.
- Static web components: server-render their static markup; JS classes only add behavior. Keep SSR HTML crawlable.
- Articles: per-locale article slugs live in `article_translations.slug` (exception to the prefix-swap rule). When adding articles, use `cmd/articleimport`.
- Adding routes/pages: add handler method on `PageHandler`, register route in main.go, wrap in `pc.Wrap` if same for all visitors, and add sitemap entries in `handlers/sitemap_handler.go`.
- SQL imports in `db/` are idempotent (use ON CONFLICT). `cmd/dbimport` runs db/schema.sql → db/seed.sql → db/freet.sql.

---

## Files to consult first

- README.md — developer overview and running instructions
- CLAUDE.md — high-signal, repo-specific guidance (reused here). Treat it as authoritative for architecture/commands.
- internal/tmpl/renderer.go — template helpers and asset hashing logic
- internal/database/connect.go — DB connection & pool tuning
- handlers/page_handler.go and handlers/product_handlers.go — SSR data shapes and query patterns
- .github/workflows/deploy.yml — production deploy steps

---

## AI-assistant / other assistant configs

- CLAUDE.md is present and contains detailed repository guidance. Copilot sessions should read it when starting.
- No other assistant config files (Cursor, Aider, Cline, Windsurf, AGENTS.md) were found in the repo root.

---

If anything in this file needs to be expanded (more examples for single-test runs, linter steps to add, or per-package test suggestions), say which area to expand and a short goal (e.g., "add go vet instructions" or "document common test fixtures").
