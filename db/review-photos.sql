-- Photos attached to a review (0..N, capped in the handler). The image files
-- themselves live on disk under UPLOADS_DIR (converted to WebP on upload); this
-- table only records the filename + display order. Cascade-deletes with the
-- review. Idempotent — in cmd/dbimport's always-run list, after reviews.sql
-- (it FKs reviews.id).
CREATE TABLE IF NOT EXISTS review_photos (
    id         SERIAL PRIMARY KEY,
    review_id  INTEGER NOT NULL REFERENCES reviews(id) ON DELETE CASCADE,
    filename   TEXT NOT NULL,          -- e.g. 'Kx9...q.webp'; URL is /uploads/reviews/<filename>
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_review_photos_review ON review_photos(review_id);
