-- Invoices for the local admin (cmd/admin). Idempotent (in cmd/dbimport's
-- always-run list) and ADD-only: the live site never reads these tables.
--
-- One invoice per order, numbered NNNN/YYYY, restarting at 0001 each year, with
-- no gaps: the number comes from invoice_series (row-locked +1 in the same
-- transaction that inserts the invoice). The invoice keeps a SNAPSHOT of the
-- buyer, lines and totals, so it never changes when the order or prices do —
-- and the PDF can always be recreated identically.

CREATE TABLE IF NOT EXISTS invoice_series (
    year     INTEGER PRIMARY KEY,
    last_seq INTEGER NOT NULL
);
-- 0001–0005/2026 were issued by hand before the admin existed; continue at 0006.
INSERT INTO invoice_series (year, last_seq) VALUES (2026, 5) ON CONFLICT (year) DO NOTHING;

CREATE TABLE IF NOT EXISTS invoices (
    id           SERIAL PRIMARY KEY,
    year         INTEGER NOT NULL,
    seq          INTEGER NOT NULL,
    -- RESTRICT: an invoiced order can't be deleted (invoices must be kept).
    order_id     INTEGER NOT NULL UNIQUE REFERENCES orders(id) ON DELETE RESTRICT,
    issued_on    DATE    NOT NULL DEFAULT CURRENT_DATE,
    buyer        JSONB   NOT NULL,   -- {name, address, postal_code, city, country, phone, email}
    lines        JSONB   NOT NULL,   -- [{desc, qty, unit_mkd, total_mkd}]
    delivery_mkd INTEGER NOT NULL,
    total_mkd    INTEGER NOT NULL,
    payment      TEXT    NOT NULL,   -- cod | bank_transfer
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (year, seq)
);
