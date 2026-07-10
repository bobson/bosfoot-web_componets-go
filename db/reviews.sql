-- ============================================================
-- Reviews — guest-checkout, order-verified product reviews.
--
-- This file is idempotent and is applied on EVERY `cmd/dbimport` run (it is in
-- the always-run list, not the fresh-DB-only schema.sql block). It migrates the
-- legacy `reviews` stub (which assumed a logged-in user_id) to the real model:
-- reviews are proven by a per-order review token emailed after fulfilment, so
-- there is no login. Runs the same way on a fresh DB (schema.sql creates the
-- legacy shape first) and on prod (which already has the legacy table).
-- ============================================================

-- The store has no user accounts (guest checkout), so a review can't be keyed on
-- a user. Drop the NOT NULL so order-verified reviews can leave it empty.
ALTER TABLE reviews ALTER COLUMN user_id DROP NOT NULL;

-- The order that proves the purchase (nullable so a hand-inserted testimonial is
-- still possible), the fit signal, the display name, the language it was written
-- in, and a moderation status. Every review starts 'pending' and is shown only
-- once approved (see cmd/reviews).
ALTER TABLE reviews ADD COLUMN IF NOT EXISTS order_id    INTEGER REFERENCES orders(id);
ALTER TABLE reviews ADD COLUMN IF NOT EXISTS fit         SMALLINT;
ALTER TABLE reviews ADD COLUMN IF NOT EXISTS author_name TEXT;
ALTER TABLE reviews ADD COLUMN IF NOT EXISTS lang_code   lang_code NOT NULL DEFAULT 'mk';
ALTER TABLE reviews ADD COLUMN IF NOT EXISTS status      TEXT NOT NULL DEFAULT 'pending';

-- fit is optional but, when present, is -2 (runs small) … 0 (true to size) … +2
-- (runs large). Drop-then-add so re-running doesn't error on the existing check.
ALTER TABLE reviews DROP CONSTRAINT IF EXISTS reviews_fit_range;
ALTER TABLE reviews ADD  CONSTRAINT reviews_fit_range CHECK (fit IS NULL OR fit BETWEEN -2 AND 2);

ALTER TABLE reviews DROP CONSTRAINT IF EXISTS reviews_status_valid;
ALTER TABLE reviews ADD  CONSTRAINT reviews_status_valid CHECK (status IN ('pending','approved','rejected'));

-- The product page reads approved reviews per product; index that access path.
CREATE INDEX IF NOT EXISTS idx_reviews_product_status ON reviews(product_id, status);

-- One review token per (order, product) line. The token IS the proof of purchase:
-- it's emailed to the buyer after the order is fulfilled and consumed on submit
-- (used_at set), so the public form can't be spammed. email + lang are snapshotted
-- from the order so the invite email and the review can be localised.
CREATE TABLE IF NOT EXISTS review_tokens (
    token      TEXT PRIMARY KEY,
    order_id   INTEGER NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    product_id INTEGER NOT NULL REFERENCES products(id),
    email      TEXT NOT NULL,
    lang_code  lang_code NOT NULL DEFAULT 'mk',
    used_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (order_id, product_id)
);
