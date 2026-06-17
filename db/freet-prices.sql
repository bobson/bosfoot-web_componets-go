-- Re-price all Freet products from their euro source price.
-- Rule: price_mkd = floor(EUR * 61 / 10) * 10  (rate site.MKDtoEUR = 61; ends in 0).
-- The shown EUR (round(price_mkd / 61)) then recovers the exact euro figure.
--
-- Run this against the live DB to apply: dbimport's product INSERTs use
-- ON CONFLICT DO NOTHING and will NOT update existing rows, so prices change
-- only via these UPDATEs. Idempotent — safe to re-run.
--
--   psql "$DATABASE_URL" -f db/freet-prices.sql      (or go run ./cmd/dbimport won't apply it)

UPDATE products SET price_mkd = 6100,  updated_at = now() WHERE sku = 'FREET-VIBE-2';      -- €100
UPDATE products SET price_mkd = 5490,  updated_at = now() WHERE sku = 'FREET-KELD-3';      -- €90
UPDATE products SET price_mkd = 8230,  updated_at = now() WHERE sku = 'FREET-YORK-2';      -- €135 (8235 → floored)
UPDATE products SET price_mkd = 8540,  updated_at = now() WHERE sku = 'FREET-RICHMOND-2';  -- €140
UPDATE products SET price_mkd = 6400,  updated_at = now() WHERE sku = 'FREET-PACE';        -- €105 (6405 → floored)
UPDATE products SET price_mkd = 6100,  updated_at = now() WHERE sku = 'FREET-TANGA-3';     -- €100
UPDATE products SET price_mkd = 6100,  updated_at = now() WHERE sku = 'FREET-FELDOM-3';    -- €100
UPDATE products SET price_mkd = 12810, updated_at = now() WHERE sku = 'FREET-CHAMOIS';     -- €210
