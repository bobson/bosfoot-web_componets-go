# Abandoned-cart email — design plan (NOT built)

Status: **plan only, awaiting decision.** Written 2026-09-04.

## Why this is a real feature, not a tweak

Today the cart is **client-side only** (`localStorage: bosfoot_cart`). Nothing about
cart contents reaches the server until an order is placed at `POST /api/orders`,
where email is collected — and email there is now **optional**. So an abandoned
cart (added, never checked out) leaves **no email and no cart on the server** to
send anything to. Three missing pieces, each a real change:

1. **Capture email before checkout.** Add an opt-in email field somewhere earlier
   than the checkout form (cart drawer is the natural spot). Must be an explicit,
   unticked marketing opt-in — see consent gate below.
2. **Persist the cart server-side, keyed to that email.** A new table, upserted on
   opt-in and on cart changes. This is the only place cart contents would ever
   live in Postgres.
3. **Abandonment detection + send job.** A `cmd/` batch (like `reviewinvites`):
   find rows with `email set + cart non-empty + no matching order + older than N
   hours + not already emailed`, send one reminder, mark it sent.

## The consent gate (owner's decision — legal, not technical)

Emailing someone who added to cart but never bought is **marketing email**, not
transactional. Under GDPR (and the project's existing consent-sensitive posture —
see the pixel/consent memories) this needs a **lawful basis**: either explicit
opt-in consent, or a defensible soft-opt-in. Practical rule:

- **Explicit opt-in** (unticked "email me about my cart / offers" checkbox) is the
  clean path. Store consent timestamp + text alongside the email.
- Every reminder needs a working **unsubscribe** link and sender identity.
- This is distinct from the add-to-cart **beacon**, which stays anonymous
  (no identifier) — do not conflate the two.

**Nothing here should ship until the owner confirms the consent approach.**

## Proposed shape (if approved)

### DB — one new table
```sql
CREATE TABLE saved_carts (
    id           BIGSERIAL PRIMARY KEY,
    email        TEXT NOT NULL,
    cart_json    JSONB NOT NULL,          -- snapshot of bosfoot_cart line items
    locale       lang_code,
    consent_at   TIMESTAMPTZ NOT NULL,    -- when + that they opted in
    consent_text TEXT NOT NULL,           -- exact wording shown
    reminded_at  TIMESTAMPTZ,             -- NULL until a reminder is sent (single send)
    ordered_at   TIMESTAMPTZ,             -- set when a matching order lands → suppress
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX ON saved_carts (lower(email));
```
Migration goes in `db/` (idempotent `IF NOT EXISTS`) and into `cmd/dbimport`'s
always-run list, same pattern as `reviews.sql`. **Migration-first deploy** (run
`dbimport` on prod before the code that reads the table).

### API — one new endpoint
`POST /api/save-cart` (CSRF-wrapped, like `/api/orders`): body = `{email, consent,
cart}`. Rejects if `consent` isn't true. Upserts `saved_carts` by email; refreshes
`cart_json`/`updated_at`. Reuses `models.Order`-style server-side validation for
the email.

### Frontend — opt-in in the cart drawer
In `public/components/cart-drawer.js`: an optional "Email me my cart" field +
explicit consent checkbox. On submit → `POST /api/save-cart`. Keep it unobtrusive;
never pre-checked. No new component needed.

### Send job — `cmd/cartreminders`
Mirror `cmd/reviewinvites` exactly:
- **Dry-run by default**, `-commit` to actually send + set `reminded_at`.
- Eligibility: `reminded_at IS NULL AND ordered_at IS NULL AND updated_at < now()
  - INTERVAL 'N hours'` (default e.g. 4–24h).
- Suppress once an order with the same email exists (set `ordered_at`); a
  post-order hook or a join in the query.
- Reuse the existing SMTP path from `reviewinvites`. Include unsubscribe +
  the saved cart's product/size/colour lines and a link back.
- One reminder per cart, ever (no drip) unless we later decide otherwise.

### Order-side hook
On successful `POST /api/orders`, mark any `saved_carts` row with the same email
as `ordered_at = now()` so it can't be reminded after purchase.

## Effort estimate
- Migration + endpoint + validation: ~half a day.
- Cart-drawer opt-in UI + wiring: ~half a day.
- `cmd/cartreminders` (clone of reviewinvites): ~half a day.
- Email template (3 locales) + unsubscribe: ~half a day.
Roughly **2 focused days**, gated entirely on the consent decision.

## Open questions for the owner
1. Consent model: explicit opt-in checkbox (recommended) vs. something lighter?
2. Reminder timing (N hours) and whether it's one email or a short sequence.
3. Discount in the reminder? (Interacts with `site.ClearancePct` — a coupon layer
   doesn't exist yet; a flat "your cart" nudge with no code is simplest.)
4. Volume: at current traffic, is the return worth the build + the ongoing
   marketing-compliance surface? (Revisit-post-launch is a legitimate answer.)
