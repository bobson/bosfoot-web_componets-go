-- Local admin (cmd/admin) support. Idempotent (in cmd/dbimport's always-run
-- list) and ADD-only: the live site never reads these columns, so it is safe to
-- run before or after any deploy.

-- Owner-marked test orders: shown in their own admin tab and the only orders
-- the admin can permanently delete (real orders can only be cancelled).
ALTER TABLE orders ADD COLUMN IF NOT EXISTS is_test BOOLEAN NOT NULL DEFAULT FALSE;
