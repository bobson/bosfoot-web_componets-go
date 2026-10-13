-- Per-product fulfilment mode, colour photo swatches, per-line fulfilment record,
-- brand visibility, and the Barebarics brand. Idempotent (in cmd/dbimport's
-- always-run list) and ADD-only, so it is safe against currently-deployed code.
--
-- DEPLOY ORDER (migration-first): run `go run ./cmd/dbimport` against prod, then
-- `go run ./cmd/shoeimport -commit` per brand (writes each model's mode from its
-- catalog.json), THEN merge the code that reads these columns.

-- What happens to sizes NOT in stock:
--   stock       → "get notified" only (in-stock sizes buy + decrement)
--   reservation → orderable, ~1–2 weeks (ordered from the brand after purchase)
--   dropship    → orderable, ~7 days (ordered from the maker after purchase)
-- In-stock sizes always ship fast and decrement, whatever the mode.
ALTER TABLE products ADD COLUMN IF NOT EXISTS fulfilment TEXT NOT NULL DEFAULT 'stock';
DO $$ BEGIN
  ALTER TABLE products ADD CONSTRAINT products_fulfilment_check
    CHECK (fulfilment IN ('stock', 'reservation', 'dropship'));
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- Small shoe photo for the colour picker ({slug}-swatch.webp, set by shoeimport
-- when the file exists). NULL → the hex dot is used.
ALTER TABLE product_colors ADD COLUMN IF NOT EXISTS swatch_url TEXT;

-- How each ordered line will be fulfilled, recorded at order time so the owner
-- knows which pairs to order from the supplier: 'stock' | 'reservation' | 'dropship'.
-- NULL on orders placed before this column existed.
ALTER TABLE order_items ADD COLUMN IF NOT EXISTS fulfilment TEXT;

-- Hidden brands are left off the brands page (no "coming soon" card), so a
-- brand can be prepared before launch. Set from the brand's catalog.json.
ALTER TABLE brands ADD COLUMN IF NOT EXISTS is_visible BOOLEAN NOT NULL DEFAULT TRUE;

-- Barebarics (Be Lenka's sneaker brand) — created hidden; catalog.json reveals it.
INSERT INTO brands (name, sku, slug, country_of_origin, website_url, is_featured, sort_order, is_visible)
VALUES ('Barebarics', 'barebarics', 'barebarics', 'SK', 'https://www.barebarics.com', FALSE, 9, FALSE)
ON CONFLICT (slug) DO NOTHING;

INSERT INTO brand_translations (brand_id, lang, description)
VALUES
  ((SELECT id FROM brands WHERE slug = 'barebarics'), 'mk', 'Урбани барефут патики од Словачка, од креаторите на Be Lenka — модерен изглед со широк простор за прсти и нула дроп.'),
  ((SELECT id FROM brands WHERE slug = 'barebarics'), 'sq', 'Atlete urbane barefoot nga Sllovakia, nga krijuesit e Be Lenka — pamje moderne me hapësirë të gjerë për gishtat dhe zero drop.'),
  ((SELECT id FROM brands WHERE slug = 'barebarics'), 'en', 'Urban barefoot sneakers from Slovakia, by the makers of Be Lenka — modern looks with a wide toe box and zero drop.')
ON CONFLICT DO NOTHING;
