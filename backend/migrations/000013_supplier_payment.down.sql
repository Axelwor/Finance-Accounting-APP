-- US-034: Bayar Supplier (Supplier Payment) — rollback.
-- Drops the supplier_payments table. The 1204 Other Receivables account is
-- intentionally left in place (it is a generic asset account that may already
-- carry balances from other flows).

DROP TABLE IF EXISTS supplier_payments;
