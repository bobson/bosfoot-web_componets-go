-- ============================================================
-- Articles — additive schema migration (idempotent).
--
-- schema.sql uses bare CREATE TABLE and can't be re-run against an
-- already-populated database, so the columns added to support the
-- foot-health articles live here as ADD COLUMN IF NOT EXISTS. Safe to run
-- against the live DB (metadata-only changes, no table rewrite).
--
-- The article CONTENT is loaded separately by `go run ./cmd/articleimport`
-- (parameterised inserts from db/articles.json) — not here — because the
-- MK/SQ/EN bodies are large and don't belong in hand-escaped SQL.
-- ============================================================

ALTER TABLE articles             ADD COLUMN IF NOT EXISTS is_featured  BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE articles             ADD COLUMN IF NOT EXISTS author       TEXT;
ALTER TABLE articles             ADD COLUMN IF NOT EXISTS published_at TIMESTAMPTZ;
ALTER TABLE article_translations ADD COLUMN IF NOT EXISTS lead         TEXT;
