-- F-04/F-05: AR Aging & AP Aging tables.
-- These are computed views, but we store snapshots for fast reporting.

-- AR Aging snapshot: outstanding receivables grouped by customer and aging bucket.
CREATE TABLE IF NOT EXISTS ar_aging_snapshots (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    snapshot_date DATE NOT NULL,
    customer_id BIGINT NOT NULL,
    customer_name TEXT NOT NULL,
    invoice_id BIGINT,
    invoice_number TEXT,
    invoice_date DATE,
    due_date DATE,
    outstanding_cents BIGINT NOT NULL,
    days_overdue INT NOT NULL DEFAULT 0,
    bucket TEXT NOT NULL CHECK (bucket IN ('current', '1_30', '31_60', '61_90', '90_plus')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, customer_id) REFERENCES customers(tenant_id, id) ON DELETE CASCADE
);

ALTER TABLE ar_aging_snapshots ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_ar_aging ON ar_aging_snapshots
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::BIGINT);
ALTER TABLE ar_aging_snapshots FORCE ROW LEVEL SECURITY;

-- AP Aging snapshot: outstanding payables grouped by supplier and aging bucket.
CREATE TABLE IF NOT EXISTS ap_aging_snapshots (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    snapshot_date DATE NOT NULL,
    supplier_id BIGINT,
    supplier_name TEXT NOT NULL,
    invoice_id BIGINT,
    invoice_number TEXT,
    invoice_date DATE,
    due_date DATE,
    outstanding_cents BIGINT NOT NULL,
    days_overdue INT NOT NULL DEFAULT 0,
    bucket TEXT NOT NULL CHECK (bucket IN ('current', '1_30', '31_60', '61_90', '90_plus')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id)
);

ALTER TABLE ap_aging_snapshots ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_ap_aging ON ap_aging_snapshots
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::BIGINT);
ALTER TABLE ap_aging_snapshots FORCE ROW LEVEL SECURITY;

-- F-07: Recurring transactions template.
CREATE TABLE IF NOT EXISTS recurring_transactions (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    code TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    intent_type TEXT NOT NULL, -- CASH_IN, CASH_OUT, TRANSFER, MANUAL_JOURNAL
    frequency TEXT NOT NULL CHECK (frequency IN ('daily', 'weekly', 'monthly', 'quarterly', 'yearly')),
    next_date DATE NOT NULL,
    end_date DATE,
    last_posted_date DATE,
    last_journal_id BIGINT,
    amount_cents BIGINT NOT NULL,
    from_account_id BIGINT,
    to_account_id BIGINT,
    payment_description TEXT,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_by BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, code)
);

ALTER TABLE recurring_transactions ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_recurring ON recurring_transactions
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::BIGINT);
ALTER TABLE recurring_transactions FORCE ROW LEVEL SECURITY;

-- F-08: Petty Cash fund (imprest system).
CREATE TABLE IF NOT EXISTS petty_cash_funds (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    code TEXT NOT NULL,
    name TEXT NOT NULL,
    cash_account_id BIGINT NOT NULL, -- the petty cash account (e.g., 1102)
    imprest_amount_cents BIGINT NOT NULL, -- fixed float amount
    custodian_user_id BIGINT,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, code)
);

ALTER TABLE petty_cash_funds ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_petty_cash ON petty_cash_funds
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::BIGINT);
ALTER TABLE petty_cash_funds FORCE ROW LEVEL SECURITY;

-- Petty cash vouchers (individual expenses paid from petty cash).
CREATE TABLE IF NOT EXISTS petty_cash_vouchers (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    fund_id BIGINT NOT NULL REFERENCES petty_cash_funds(id) ON DELETE CASCADE,
    number TEXT NOT NULL,
    voucher_date DATE NOT NULL,
    amount_cents BIGINT NOT NULL,
    expense_account_id BIGINT NOT NULL, -- the expense account to debit
    description TEXT NOT NULL,
    recipient TEXT,
    status TEXT NOT NULL DEFAULT 'POSTED' CHECK (status IN ('DRAFT', 'POSTED', 'REVERSED')),
    journal_entry_id BIGINT,
    created_by BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, number)
);

ALTER TABLE petty_cash_vouchers ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_petty_vouchers ON petty_cash_vouchers
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::BIGINT);
ALTER TABLE petty_cash_vouchers FORCE ROW LEVEL SECURITY;

-- F-01: Multi-currency support.
CREATE TABLE IF NOT EXISTS currencies (
    code CHAR(3) PRIMARY KEY, -- IDR, USD, EUR
    name TEXT NOT NULL,
    symbol TEXT NOT NULL,
    decimal_places INT NOT NULL DEFAULT 2
);

INSERT INTO currencies (code, name, symbol, decimal_places) VALUES
    ('IDR', 'Indonesian Rupiah', 'Rp', 2),
    ('USD', 'US Dollar', '$', 2),
    ('EUR', 'Euro', '€', 2),
    ('SGD', 'Singapore Dollar', 'S$', 2),
    ('JPY', 'Japanese Yen', '¥', 0),
    ('CNY', 'Chinese Yuan', '¥', 2),
    ('AUD', 'Australian Dollar', 'A$', 2),
    ('GBP', 'British Pound', '£', 2)
ON CONFLICT (code) DO NOTHING;

CREATE TABLE IF NOT EXISTS exchange_rates (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    from_currency CHAR(3) NOT NULL REFERENCES currencies(code),
    to_currency CHAR(3) NOT NULL REFERENCES currencies(code),
    rate NUMERIC(18,8) NOT NULL, -- 1 from_currency = rate to_currency
    effective_date DATE NOT NULL,
    source TEXT NOT NULL DEFAULT 'manual', -- manual, bi_api
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, from_currency, to_currency, effective_date)
);

ALTER TABLE exchange_rates ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_exchange_rates ON exchange_rates
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::BIGINT);
ALTER TABLE exchange_rates FORCE ROW LEVEL SECURITY;

-- Add currency support to journal_entries.
ALTER TABLE journal_entries ADD COLUMN IF NOT EXISTS currency_code CHAR(3) NOT NULL DEFAULT 'IDR';
ALTER TABLE journal_entries ADD COLUMN IF NOT EXISTS exchange_rate NUMERIC(18,8) NOT NULL DEFAULT 1.0;

-- FX gain/loss accounts (seeded per tenant).
INSERT INTO accounts (tenant_id, code, name, report_group, account_type)
SELECT t.id, v.code, v.val_name, v.report_group, v.account_type
FROM tenants t
CROSS JOIN (VALUES
  ('4904', 'Gain on Foreign Exchange', 'revenue', 'OTHER_REVENUE'),
  ('5904', 'Loss on Foreign Exchange', 'expense', 'OTHER_EXPENSE')
) AS v(code, val_name, report_group, account_type)
WHERE NOT EXISTS (
    SELECT 1 FROM accounts a WHERE a.tenant_id = t.id AND a.code = v.code
);
