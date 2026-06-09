-- ============================================================
-- Bosfoot — seed data
-- Reference rows that the app expects to exist. Run AFTER schema.sql:
--   psql "$DATABASE_URL" -f import/seed.sql
-- Safe to re-run: ON CONFLICT DO NOTHING keeps it idempotent.
-- ============================================================

-- ---------- Genders ----------
INSERT INTO genders (value) VALUES
    ('male'),
    ('female'),
    ('unisex'),
    ('kids')
ON CONFLICT (value) DO NOTHING;

-- ---------- Categories ----------
-- accessories arrive later but the row is here so filtering works from day one.
INSERT INTO categories (slug, name) VALUES
    ('shoes',       'Shoes'),
    ('insoles',     'Insoles'),
    ('accessories', 'Accessories')
ON CONFLICT (slug) DO NOTHING;
