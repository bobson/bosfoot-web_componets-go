-- Official Freet size chart (from freetbarefoot.com), shared across every Freet
-- model — Freet publishes one chart for all styles ("true to size"). This
-- replaces the earlier uniform placeholder that was seeded into size_chart.
--
-- Idempotent: clears the Freet rows and reinserts the official set, so it is
-- safe to re-run (dev + prod). Run after seeding:  psql "$DATABASE_URL" -f db/freet-sizechart.sql
BEGIN;

DELETE FROM size_chart
WHERE product_id IN (
    SELECT p.id FROM products p
    JOIN brands b ON b.id = p.brand_id
    WHERE b.slug = 'freet'
);

INSERT INTO size_chart (product_id, eu_size, foot_length_mm, foot_width_mm)
SELECT p.id, s.eu, s.len, s.wid
FROM products p
JOIN brands b ON b.id = p.brand_id
CROSS JOIN (VALUES
    (37, 236, 88),  (38, 242, 89),  (39, 249, 90),  (40, 257, 92),
    (41, 262, 93),  (42, 268, 95),  (43, 275, 98),  (44, 284, 100),
    (45, 290, 101), (46, 295, 102), (47, 300, 104), (48, 307, 106),
    (49, 313, 107)
) AS s(eu, len, wid)
WHERE b.slug = 'freet';

COMMIT;
