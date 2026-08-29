-- SET-001: Settings module + master data (units/categories/brands/taxes) + multi-currency.
-- See plan 1787856396498. All tenant-scoped tables follow the RLS pattern of 000012.

-- ===========================================================================
-- 1. Tenant company profile
-- ===========================================================================
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS legal_name TEXT;
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS address TEXT;
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS city TEXT;
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS phone TEXT;
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS email TEXT;
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS tax_id TEXT;

-- ===========================================================================
-- 2. FX accounts fix.
--    5904 is already "Deferred Tax Expense" (seed.go) — the 000029 seed of
--    5904 "Loss on Foreign Exchange" always lost the WHERE NOT EXISTS race.
--    4904 was only created for tenants that existed before 000029 ran; newer
--    tenants have neither. Seed 4904 (idempotent) and a brand-new 5905.
--    5904 is NEVER used for FX.
-- ===========================================================================
INSERT INTO accounts (tenant_id, code, name, report_group, account_type)
SELECT t.id, v.code, v.val_name, v.report_group, v.account_type
FROM tenants t
CROSS JOIN (VALUES
  ('4904', 'Gain on Foreign Exchange', 'revenue', 'OTHER_INCOME'),
  ('5905', 'Loss on Foreign Exchange', 'expense', 'OTHER_EXPENSE')
) AS v(code, val_name, report_group, account_type)
WHERE NOT EXISTS (
    SELECT 1 FROM accounts a WHERE a.tenant_id = t.id AND a.code = v.code
);

-- ===========================================================================
-- 3. tenant_settings: per-tenant preferences + default account mapping.
--    Default accounts replace the hardcoded codes scattered across
--    tax/helpers.go, cash/handler.go, sales/payments.go.
--    Seed resolves the legacy codes so behaviour is unchanged for tenants
--    that never touch the settings screen.
-- ===========================================================================
CREATE TABLE IF NOT EXISTS tenant_settings (
    tenant_id BIGINT PRIMARY KEY REFERENCES tenants(id) ON DELETE CASCADE,
    date_format TEXT NOT NULL DEFAULT 'DD/MM/YYYY'
        CHECK (date_format IN ('DD/MM/YYYY','MM/DD/YYYY','YYYY-MM-DD')),
    thousand_separator CHAR(1) NOT NULL DEFAULT '.',
    decimal_separator CHAR(1) NOT NULL DEFAULT ',',
    amount_decimal_places SMALLINT NOT NULL DEFAULT 2 CHECK (amount_decimal_places BETWEEN 0 AND 4),
    qty_decimal_places SMALLINT NOT NULL DEFAULT 2 CHECK (qty_decimal_places BETWEEN 0 AND 4),
    default_sales_account_id BIGINT REFERENCES accounts(id),
    default_purchase_account_id BIGINT REFERENCES accounts(id),
    default_cogs_account_id BIGINT REFERENCES accounts(id),
    default_ar_account_id BIGINT REFERENCES accounts(id),
    default_ap_account_id BIGINT REFERENCES accounts(id),
    default_cash_account_id BIGINT REFERENCES accounts(id),
    default_capital_account_id BIGINT REFERENCES accounts(id),
    retained_earnings_account_id BIGINT REFERENCES accounts(id),
    opening_balance_equity_account_id BIGINT REFERENCES accounts(id),
    fx_gain_account_id BIGINT REFERENCES accounts(id),
    fx_loss_account_id BIGINT REFERENCES accounts(id),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE tenant_settings ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
    CREATE POLICY tenant_isolation_tenant_settings ON tenant_settings
        USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::BIGINT);
EXCEPTION WHEN duplicate_object THEN NULL; END $$;
ALTER TABLE tenant_settings FORCE ROW LEVEL SECURITY;

-- Seed one row per existing tenant, resolving the legacy account codes.
INSERT INTO tenant_settings (tenant_id,
    default_sales_account_id, default_purchase_account_id, default_cogs_account_id,
    default_ar_account_id, default_ap_account_id, default_cash_account_id,
    default_capital_account_id, retained_earnings_account_id,
    opening_balance_equity_account_id, fx_gain_account_id, fx_loss_account_id)
SELECT t.id, s.id, p.id, c.id, ar.id, ap.id, ca.id, cap.id, re.id, ob.id, fg.id, fl.id
FROM tenants t
LEFT JOIN accounts s   ON s.tenant_id = t.id   AND s.code = '4101'
LEFT JOIN accounts p   ON p.tenant_id = t.id   AND p.code = '1301'
LEFT JOIN accounts c   ON c.tenant_id = t.id   AND c.code = '5101'
LEFT JOIN accounts ar  ON ar.tenant_id = t.id  AND ar.code = '1201'
LEFT JOIN accounts ap  ON ap.tenant_id = t.id  AND ap.code = '2101'
LEFT JOIN accounts ca  ON ca.tenant_id = t.id  AND ca.code = '1101'
LEFT JOIN accounts cap ON cap.tenant_id = t.id AND cap.code = '3101'
LEFT JOIN accounts re  ON re.tenant_id = t.id  AND re.code = '3201'
LEFT JOIN accounts ob  ON ob.tenant_id = t.id  AND ob.code = '3101'
LEFT JOIN accounts fg  ON fg.tenant_id = t.id  AND fg.code = '4904'
LEFT JOIN accounts fl  ON fl.tenant_id = t.id  AND fl.code = '5905'
WHERE NOT EXISTS (SELECT 1 FROM tenant_settings ts WHERE ts.tenant_id = t.id);

-- ===========================================================================
-- 4. Item master data: units, categories, brands + backfill from items text
--    columns, then FK + drop the legacy text columns.
-- ===========================================================================
CREATE TABLE IF NOT EXISTS units (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    code TEXT NOT NULL,
    name TEXT NOT NULL,
    decimal_places SMALLINT NOT NULL DEFAULT 0 CHECK (decimal_places BETWEEN 0 AND 4),
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, name)
);

ALTER TABLE units ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
    CREATE POLICY tenant_isolation_units ON units
        USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::BIGINT);
EXCEPTION WHEN duplicate_object THEN NULL; END $$;
ALTER TABLE units FORCE ROW LEVEL SECURITY;

CREATE TABLE IF NOT EXISTS item_categories (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, name)
);

ALTER TABLE item_categories ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
    CREATE POLICY tenant_isolation_item_categories ON item_categories
        USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::BIGINT);
EXCEPTION WHEN duplicate_object THEN NULL; END $$;
ALTER TABLE item_categories FORCE ROW LEVEL SECURITY;

CREATE TABLE IF NOT EXISTS item_brands (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, name)
);

ALTER TABLE item_brands ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
    CREATE POLICY tenant_isolation_item_brands ON item_brands
        USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::BIGINT);
EXCEPTION WHEN duplicate_object THEN NULL; END $$;
ALTER TABLE item_brands FORCE ROW LEVEL SECURITY;

-- Backfill masters from every distinct non-empty text value (all uom variants
-- feed the units master; matching is case-insensitive on the trimmed name).
INSERT INTO units (tenant_id, code, name)
SELECT DISTINCT ON (tenant_id, lower(btrim(v.val_name))) tenant_id, upper(btrim(v.val_name)), btrim(v.val_name)
FROM (
    SELECT tenant_id, uom AS val_name FROM items WHERE btrim(COALESCE(uom, '')) <> ''
    UNION
    SELECT tenant_id, secondary_uom FROM items WHERE btrim(COALESCE(secondary_uom, '')) <> ''
    UNION
    SELECT tenant_id, sale_uom FROM items WHERE btrim(COALESCE(sale_uom, '')) <> ''
    UNION
    SELECT tenant_id, purchase_uom FROM items WHERE btrim(COALESCE(purchase_uom, '')) <> ''
) v
ON CONFLICT (tenant_id, name) DO NOTHING;

INSERT INTO item_categories (tenant_id, name)
SELECT DISTINCT ON (tenant_id, lower(btrim(category))) tenant_id, btrim(category)
FROM items
WHERE btrim(COALESCE(category, '')) <> ''
ON CONFLICT (tenant_id, name) DO NOTHING;

INSERT INTO item_brands (tenant_id, name)
SELECT DISTINCT ON (tenant_id, lower(btrim(brand))) tenant_id, btrim(brand)
FROM items
WHERE btrim(COALESCE(brand, '')) <> ''
ON CONFLICT (tenant_id, name) DO NOTHING;

-- FK columns + backfill the ids.
ALTER TABLE items ADD COLUMN IF NOT EXISTS unit_id BIGINT REFERENCES units(id);
ALTER TABLE items ADD COLUMN IF NOT EXISTS category_id BIGINT REFERENCES item_categories(id);
ALTER TABLE items ADD COLUMN IF NOT EXISTS brand_id BIGINT REFERENCES item_brands(id);

UPDATE items i SET unit_id = u.id
FROM units u
WHERE u.tenant_id = i.tenant_id AND lower(u.name) = lower(btrim(COALESCE(i.uom, '')));

UPDATE items i SET category_id = c.id
FROM item_categories c
WHERE c.tenant_id = i.tenant_id AND lower(c.name) = lower(btrim(COALESCE(i.category, '')));

UPDATE items i SET brand_id = b.id
FROM item_brands b
WHERE b.tenant_id = i.tenant_id AND lower(b.name) = lower(btrim(COALESCE(i.brand, '')));

-- Drop the legacy free-text columns (single source of truth from now on).
ALTER TABLE items DROP COLUMN IF EXISTS uom;
ALTER TABLE items DROP COLUMN IF EXISTS secondary_uom;
ALTER TABLE items DROP COLUMN IF EXISTS sale_uom;
ALTER TABLE items DROP COLUMN IF EXISTS purchase_uom;
ALTER TABLE items DROP COLUMN IF EXISTS category;
ALTER TABLE items DROP COLUMN IF EXISTS brand;

-- ===========================================================================
-- 5. Tax master: name, rate, and COA mapping for sales/purchase tax posting.
--    Seeded PPN keeps the legacy behaviour (output 2202 / input 1203).
-- ===========================================================================
CREATE TABLE IF NOT EXISTS taxes (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    code TEXT NOT NULL,
    name TEXT NOT NULL,
    rate NUMERIC(5,2) NOT NULL CHECK (rate >= 0 AND rate <= 100),
    sales_account_id BIGINT REFERENCES accounts(id),
    purchase_account_id BIGINT REFERENCES accounts(id),
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, code)
);

ALTER TABLE taxes ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
    CREATE POLICY tenant_isolation_taxes ON taxes
        USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::BIGINT);
EXCEPTION WHEN duplicate_object THEN NULL; END $$;
ALTER TABLE taxes FORCE ROW LEVEL SECURITY;

INSERT INTO taxes (tenant_id, code, name, rate, sales_account_id, purchase_account_id)
SELECT t.id, 'PPN', 'PPN', 11.00, so.id, pi.id
FROM tenants t
LEFT JOIN accounts so ON so.tenant_id = t.id AND so.code = '2202'
LEFT JOIN accounts pi ON pi.tenant_id = t.id AND pi.code = '1203'
WHERE NOT EXISTS (SELECT 1 FROM taxes x WHERE x.tenant_id = t.id AND x.code = 'PPN');

-- Link the posting documents to the tax master (nullable: legacy rows fall
-- back to the default PPN mapping).
ALTER TABLE invoices ADD COLUMN IF NOT EXISTS tax_id BIGINT REFERENCES taxes(id);
ALTER TABLE supplier_invoices ADD COLUMN IF NOT EXISTS tax_id BIGINT REFERENCES taxes(id);

-- ===========================================================================
-- 6. Multi-currency on commercial documents.
--    Amounts stay in base-currency cents; the header stores the document
--    currency + the rate used at entry time. Defaults keep legacy rows
--    behaving exactly as before (IDR, rate 1).
-- ===========================================================================
ALTER TABLE sales_quotations
    ADD COLUMN IF NOT EXISTS currency_code CHAR(3) NOT NULL DEFAULT 'IDR',
    ADD COLUMN IF NOT EXISTS exchange_rate NUMERIC(18,8) NOT NULL DEFAULT 1.0;
ALTER TABLE sales_orders
    ADD COLUMN IF NOT EXISTS currency_code CHAR(3) NOT NULL DEFAULT 'IDR',
    ADD COLUMN IF NOT EXISTS exchange_rate NUMERIC(18,8) NOT NULL DEFAULT 1.0;
ALTER TABLE sales_down_payments
    ADD COLUMN IF NOT EXISTS currency_code CHAR(3) NOT NULL DEFAULT 'IDR',
    ADD COLUMN IF NOT EXISTS exchange_rate NUMERIC(18,8) NOT NULL DEFAULT 1.0;
ALTER TABLE invoices
    ADD COLUMN IF NOT EXISTS currency_code CHAR(3) NOT NULL DEFAULT 'IDR',
    ADD COLUMN IF NOT EXISTS exchange_rate NUMERIC(18,8) NOT NULL DEFAULT 1.0;
ALTER TABLE credit_notes
    ADD COLUMN IF NOT EXISTS currency_code CHAR(3) NOT NULL DEFAULT 'IDR',
    ADD COLUMN IF NOT EXISTS exchange_rate NUMERIC(18,8) NOT NULL DEFAULT 1.0;
ALTER TABLE purchase_orders
    ADD COLUMN IF NOT EXISTS currency_code CHAR(3) NOT NULL DEFAULT 'IDR',
    ADD COLUMN IF NOT EXISTS exchange_rate NUMERIC(18,8) NOT NULL DEFAULT 1.0;
ALTER TABLE supplier_invoices
    ADD COLUMN IF NOT EXISTS currency_code CHAR(3) NOT NULL DEFAULT 'IDR',
    ADD COLUMN IF NOT EXISTS exchange_rate NUMERIC(18,8) NOT NULL DEFAULT 1.0;
ALTER TABLE purchase_returns
    ADD COLUMN IF NOT EXISTS currency_code CHAR(3) NOT NULL DEFAULT 'IDR',
    ADD COLUMN IF NOT EXISTS exchange_rate NUMERIC(18,8) NOT NULL DEFAULT 1.0;

-- Payments carry the settlement rate (FX gain/loss is computed against the
-- invoice's posting rate).
ALTER TABLE invoice_payments
    ADD COLUMN IF NOT EXISTS currency_code CHAR(3) NOT NULL DEFAULT 'IDR',
    ADD COLUMN IF NOT EXISTS exchange_rate NUMERIC(18,8) NOT NULL DEFAULT 1.0;
ALTER TABLE supplier_payments
    ADD COLUMN IF NOT EXISTS currency_code CHAR(3) NOT NULL DEFAULT 'IDR',
    ADD COLUMN IF NOT EXISTS exchange_rate NUMERIC(18,8) NOT NULL DEFAULT 1.0;

-- Fast latest-rate lookup.
CREATE INDEX IF NOT EXISTS idx_exchange_rates_latest
    ON exchange_rates (tenant_id, from_currency, effective_date DESC);
