-- The size_chart columns hold Freet's INSOLE measurements (insole length + width
-- of the shoe), not foot measurements — see the Freet size chart / foot-finder.js.
-- Rename them for accuracy. Idempotent: only renames when the old name is still
-- present, so it's a no-op on fresh DBs (schema.sql already uses the new names)
-- and on re-runs. Must run BEFORE the freet* size_chart seeds, which now insert
-- the new column names.
--
-- NOTE: this is an in-place RENAME, so it is NOT backward-compatible with a build
-- that still SELECTs foot_length_mm (the product page reads size_chart). Apply it
-- together with the matching code deploy.
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM information_schema.columns
             WHERE table_name = 'size_chart' AND column_name = 'foot_length_mm') THEN
    ALTER TABLE size_chart RENAME COLUMN foot_length_mm TO insole_length_mm;
  END IF;
  IF EXISTS (SELECT 1 FROM information_schema.columns
             WHERE table_name = 'size_chart' AND column_name = 'foot_width_mm') THEN
    ALTER TABLE size_chart RENAME COLUMN foot_width_mm TO insole_width_mm;
  END IF;
END $$;
