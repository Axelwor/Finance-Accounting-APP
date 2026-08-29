-- SET-001 down: revert settings/master data/multi-currency.
-- Item legacy text columns are restored best-effort from the master tables.

-- FC columns off documents.
ALTER TABLE invoice_payments
    DROP COLUMN IF EXISTS currency_code,
    DROP COLUMN IF EXISTS exchange_rate;
ALTER TABLE supplier_payments
    DROP COLUMN IF EXISTS currency_code,
    DROP COLUMN IF EXISTS exchange_rate;
ALTER TABLE purchase_returns
    DROP COLUMN IF EXISTS currency_code,
    DROP COLUMN IF EXISTS exchange_rate;
ALTER TABLE supplier_invoices
    DROP COLUMN IF EXISTS currency_code,
    DROP COLUMN IF EXISTS exchange_rate,
    DROP COLUMN IF EXISTS tax_id;
ALTER TABLE purchase_orders
    DROP COLUMN IF EXISTS currency_code,
    DROP COLUMN IF EXISTS exchange_rate;
ALTER TABLE credit_notes
    DROP COLUMN IF EXISTS currency_code,
    DROP COLUMN IF EXISTS exchange_rate;
ALTER TABLE invoices
    DROP COLUMN IF EXISTS currency_code,
    DROP COLUMN IF EXISTS exchange_rate,
    DROP COLUMN IF EXISTS tax_id;
ALTER TABLE sales_down_payments
    DROP COLUMN IF EXISTS currency_code,
    DROP COLUMN IF EXISTS exchange_rate;
ALTER TABLE sales_orders
    DROP COLUMN IF EXISTS currency_code,
    DROP COLUMN IF EXISTS exchange_rate;
ALTER TABLE sales_quotations
    DROP COLUMN IF EXISTS currency_code,
    DROP COLUMN IF EXISTS exchange_rate;

DROP INDEX IF EXISTS idx_exchange_rates_latest;

-- Restore item text columns (best-effort) before dropping masters.
ALTER TABLE items ADD COLUMN IF NOT EXISTS uom TEXT;
ALTER TABLE items ADD COLUMN IF NOT EXISTS secondary_uom TEXT;
ALTER TABLE items ADD COLUMN IF NOT EXISTS sale_uom TEXT;
ALTER TABLE items ADD COLUMN IF NOT EXISTS purchase_uom TEXT;
ALTER TABLE items ADD COLUMN IF NOT EXISTS "category" TEXT;
ALTER TABLE items ADD COLUMN IF NOT EXISTS "brand" TEXT;

UPDATE items i SET uom = u.name FROM units u WHERE u.id = i.unit_id AND i.uom IS NULL;
UPDATE items i SET "category" = c.name FROM item_categories c WHERE c.id = i.category_id AND i."category" IS NULL;
UPDATE items i SET "brand" = b.name FROM item_brands b WHERE b.id = i.brand_id AND i."brand" IS NULL;

ALTER TABLE items
    DROP COLUMN IF EXISTS unit_id,
    DROP COLUMN IF EXISTS category_id,
    DROP COLUMN IF EXISTS brand_id;

DROP TABLE IF EXISTS taxes;
DROP TABLE IF EXISTS item_brands;
DROP TABLE IF EXISTS item_categories;
DROP TABLE IF EXISTS units;
DROP TABLE IF EXISTS tenant_settings;

-- Remove the FX accounts only when they were never used.
DELETE FROM accounts a
WHERE a.code IN ('4904', '5905')
  AND NOT EXISTS (SELECT 1 FROM journal_lines jl WHERE jl.account_id = a.id);

ALTER TABLE tenants
    DROP COLUMN IF EXISTS legal_name,
    DROP COLUMN IF EXISTS address,
    DROP COLUMN IF EXISTS city,
    DROP COLUMN IF EXISTS phone,
    DROP COLUMN IF EXISTS email,
    DROP COLUMN IF EXISTS tax_id;
