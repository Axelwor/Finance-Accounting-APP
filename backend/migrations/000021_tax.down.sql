-- Reverse US-080..083: Tax (PPN, PPh, ECL).
-- Drop the tax tables. The seeded accounts (2203, 5208, 1202, 5205, 4906,
-- 1206, 5904) are left in place: journal lines may already reference them and
-- removing accounts is destructive. Dropping the tables is safe and reversible
-- (re-running the up migration recreates them empty).
DROP TABLE IF EXISTS ppn_reconciliations;
DROP TABLE IF EXISTS tax_rates;
