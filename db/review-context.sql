-- Buyer context snapshotted onto a review at submit time: the city, and the exact
-- size + colour that was ordered. Snapshotting (rather than joining to the order
-- at display) means the product page can always show "★★★★★ — Marko, Skopje ·
-- size 43", and it survives the order being deleted.
--
-- Also relaxes reviews.order_id to ON DELETE SET NULL (it was a plain RESTRICT
-- reference), so orders can be pruned without the FK blocking on a review — the
-- snapshot keeps the context. Runs after reviews.sql (which creates order_id).
ALTER TABLE reviews ADD COLUMN IF NOT EXISTS buyer_city TEXT;
ALTER TABLE reviews ADD COLUMN IF NOT EXISTS size       TEXT;
ALTER TABLE reviews ADD COLUMN IF NOT EXISTS color      TEXT;

-- Drop whatever FK currently sits on reviews.order_id (name may vary), then
-- recreate it as ON DELETE SET NULL. Idempotent.
DO $$
DECLARE c text;
BEGIN
  SELECT conname INTO c FROM pg_constraint
   WHERE conrelid = 'reviews'::regclass AND contype = 'f'
     AND conkey = ARRAY[(SELECT attnum FROM pg_attribute
                          WHERE attrelid = 'reviews'::regclass AND attname = 'order_id')];
  IF c IS NOT NULL THEN
    EXECUTE 'ALTER TABLE reviews DROP CONSTRAINT ' || quote_ident(c);
  END IF;
END $$;
ALTER TABLE reviews ADD CONSTRAINT reviews_order_id_fkey
    FOREIGN KEY (order_id) REFERENCES orders(id) ON DELETE SET NULL;
