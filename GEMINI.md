# Bosfoot: Barefoot Shoes E-commerce

This project is a high-performance e-commerce store built with Go and Vanilla JS. It prioritizes SEO, speed, and simplicity.

## Architecture Overview

- **Backend:** Go (standard library `net/http`).
- **Frontend:** Vanilla JS with Server-Side Rendering (SSR) using Go's `html/template`.
- **Database:** PostgreSQL.
- **Localization:** Triple-locale support (MK, SQ, EN) managed via URL paths (e.g., `/en/products`).

## Core Conventions

### 1. Hybrid Rendering Pattern
- **SSR First:** All initial page loads must be server-side rendered for SEO and performance.
- **Hydration:** Data required by JS (e.g., stock levels) is embedded via `<script type="application/json">` or `data-*` attributes.
- **CSR for Interaction:** Use the `/api/` endpoints for dynamic actions (filtering, adding to cart, checkout).

### 2. Frontend Organization
- **No Build Step:** Use standard ES6 modules in `public/components/`.
- **Component Isolation:** Use IIFEs or modules to prevent global scope pollution.
- **Event-Driven:** Components communicate via custom DOM events (e.g., `cart:updated`, `cart:open`).

### 3. Backend Structure
- **Handlers:** Located in `/handlers`. Separate `PageHandler` (HTML) from `ProductHandler/OrderHandler` (JSON).
- **Internal Packages:** Business logic, DB connections, and rendering helpers live in `/internal`.
- **Models:** Centralized in `/models` to map DB tables to Go structs.

### 4. Database & Localization
- **Currency:** MKD is the base currency (integer). EUR is calculated at runtime in the template renderer.
- **Translations:** Lookup strings are in `public/locales/*.json`. Product translations are stored in the database.

## Key Entry Points
- `main.go`: Server initialization and routing.
- `handlers/page_handler.go`: SSR page logic.
- `internal/tmpl/renderer.go`: Template engine and custom functions.
- `public/app.js`: Global frontend state.

## Engineering Standards
- **Surgical Edits:** Favor targeted changes over full-file rewrites.
- **Validation:** Always verify changes with manual or automated checks.
- **Testing (Target):** New features should include unit tests in `*_test.go` files (to be established).
