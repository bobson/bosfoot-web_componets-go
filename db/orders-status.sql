-- Adds the 'cancelled' order status for orders that fall through (cash-on-
-- delivery refused, customer unreachable, etc.) so they can be closed without
-- masquerading as delivered. Idempotent: ADD VALUE IF NOT EXISTS is a no-op if
-- the value already exists (fresh DBs already have it from schema.sql).
--
-- PostgreSQL 12+ allows ALTER TYPE ... ADD VALUE inside the implicit transaction
-- dbimport uses, as long as the new value isn't USED in the same transaction —
-- this file only adds it, so that holds. Aiven runs a modern Postgres.
ALTER TYPE order_status ADD VALUE IF NOT EXISTS 'cancelled';
