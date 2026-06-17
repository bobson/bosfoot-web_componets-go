-- Euro-as-source: the euro price is authoritative; MKD is derived at render via
-- site.MKD (euro × 61, floored to end in 0). Additive + idempotent — safe to run
-- on the live DB anytime: the deployed code still reads price_mkd (untouched
-- here), so nothing changes live until the euro-aware code deploys.

ALTER TABLE products ADD COLUMN IF NOT EXISTS price_eur          INTEGER;
ALTER TABLE products ADD COLUMN IF NOT EXISTS original_price_eur INTEGER;

-- Fallback backfill so fresh installs (and any unset row) get a sane euro from
-- the existing MKD. The explicit prices below override these for Freet.
UPDATE products SET price_eur = ROUND(price_mkd / 61.0)
  WHERE price_eur IS NULL;
UPDATE products SET original_price_eur = ROUND(original_price_mkd / 61.0)
  WHERE original_price_mkd IS NOT NULL AND original_price_eur IS NULL;

-- Authoritative euro source prices (these are what you edit going forward).
UPDATE products SET price_eur = 100 WHERE sku = 'FREET-VIBE-2';
UPDATE products SET price_eur = 90  WHERE sku = 'FREET-KELD-3';
UPDATE products SET price_eur = 135 WHERE sku = 'FREET-YORK-2';
UPDATE products SET price_eur = 140 WHERE sku = 'FREET-RICHMOND-2';
UPDATE products SET price_eur = 105 WHERE sku = 'FREET-PACE';
UPDATE products SET price_eur = 100 WHERE sku = 'FREET-TANGA-3';
UPDATE products SET price_eur = 100 WHERE sku = 'FREET-FELDOM-3';
UPDATE products SET price_eur = 210 WHERE sku = 'FREET-CHAMOIS';
