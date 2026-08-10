-- Migration 000033: Sprint 9 — Giro/Cheque, Cost Center, Budget Variance, Email, Field Expansion
-- F-14: Giro & Cheque Management
-- F-09: Cost/Profit Center
-- F-11: Budget vs Actual (variance report tables)
-- F-15: Email Notification
-- E-01..E-06: ERP field expansion (customer, supplier, item)

-- =====================================================
-- F-14: Giro & Cheque Management
-- =====================================================
CREATE TABLE IF NOT EXISTS cheques (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id),
    cheque_number VARCHAR(50) NOT NULL,
    cheque_type VARCHAR(20) NOT NULL CHECK (cheque_type IN ('CHEQUE', 'GIRO')),
    direction VARCHAR(10) NOT NULL CHECK (direction IN ('RECEIVED', 'ISSUED')),
    bank_name VARCHAR(100),
    bank_account_number VARCHAR(50),
    payee VARCHAR(200),
    drawer VARCHAR(200),
    amount_cents BIGINT NOT NULL CHECK (amount_cents > 0),
    issue_date DATE NOT NULL,
    due_date DATE,
    clearing_date DATE,
    status VARCHAR(20) NOT NULL DEFAULT 'REGISTERED' CHECK (
        status IN ('REGISTERED', 'DEPOSITED', 'CLEARED', 'BOUNCED', 'CANCELLED')
    ),
    bounced_reason TEXT,
    journal_entry_id BIGINT,
    payment_id BIGINT,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(tenant_id, cheque_number)
);

-- F-14: Giro clearing suspense account
INSERT INTO accounts (tenant_id, code, name, report_group, account_type, is_group, is_active, valid_from)
SELECT t.id, '1304', 'Cheques in Transit', 'asset', 'OTHER_CURRENT_ASSET', false, true, '2026-01-01'
FROM tenants t
WHERE NOT EXISTS (
    SELECT 1 FROM accounts a WHERE a.tenant_id = t.id AND a.code = '1304'
);

INSERT INTO accounts (tenant_id, code, name, report_group, account_type, is_group, is_active, valid_from)
SELECT t.id, '2105', 'Cheques Issued Outstanding', 'liability', 'OTHER_CURRENT_LIABILITY', false, true, '2026-01-01'
FROM tenants t
WHERE NOT EXISTS (
    SELECT 1 FROM accounts a WHERE a.tenant_id = t.id AND a.code = '2105'
);

-- =====================================================
-- F-09: Cost/Profit Center
-- =====================================================
CREATE TABLE IF NOT EXISTS cost_centers (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id),
    code VARCHAR(20) NOT NULL,
    name VARCHAR(200) NOT NULL,
    center_type VARCHAR(20) NOT NULL CHECK (center_type IN ('COST', 'PROFIT', 'INVESTMENT')),
    parent_id BIGINT REFERENCES cost_centers(id),
    manager_user_id BIGINT,
    is_active BOOLEAN NOT NULL DEFAULT true,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(tenant_id, code)
);

-- F-09: Cost center allocation rules
CREATE TABLE IF NOT EXISTS cost_center_allocations (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id),
    source_cost_center_id BIGINT NOT NULL REFERENCES cost_centers(id),
    target_cost_center_id BIGINT NOT NULL REFERENCES cost_centers(id),
    allocation_percentage NUMERIC(5,2) NOT NULL CHECK (allocation_percentage > 0 AND allocation_percentage <= 100),
    allocation_basis VARCHAR(50) DEFAULT 'REVENUE' CHECK (allocation_basis IN ('REVENUE', 'HEADCOUNT', 'AREA', 'EQUAL', 'CUSTOM')),
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- F-09: Dimension links to cost centers (existing dimensions table already supports this)
-- journal_lines.dimension_ids already stores cost center dimension IDs

-- =====================================================
-- F-11: Budget vs Actual Variance
-- =====================================================
CREATE TABLE IF NOT EXISTS budget_variance_reports (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id),
    budget_id BIGINT NOT NULL REFERENCES budgets(id),
    period_id BIGINT REFERENCES accounting_periods(id),
    account_id BIGINT NOT NULL REFERENCES accounts(id),
    cost_center_id BIGINT REFERENCES cost_centers(id),
    budgeted_amount_cents BIGINT NOT NULL DEFAULT 0,
    actual_amount_cents BIGINT NOT NULL DEFAULT 0,
    variance_cents BIGINT NOT NULL DEFAULT 0,
    variance_percentage NUMERIC(8,2),
    variance_direction VARCHAR(10) CHECK (variance_direction IN ('FAVORABLE', 'UNFAVORABLE')),
    explanation TEXT,
    report_date DATE NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- =====================================================
-- F-15: Email Notification
-- =====================================================
CREATE TABLE IF NOT EXISTS email_templates (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id),
    code VARCHAR(50) NOT NULL,
    name VARCHAR(200) NOT NULL,
    subject VARCHAR(500) NOT NULL,
    body_html TEXT NOT NULL,
    body_text TEXT,
    trigger_event VARCHAR(50) NOT NULL CHECK (trigger_event IN (
        'INVOICE_CREATED', 'INVOICE_OVERDUE', 'PAYMENT_RECEIVED',
        'PURCHASE_ORDER_CREATED', 'GOODS_RECEIVED', 'SUPPLIER_INVOICE_DUE',
        'PERIOD_CLOSE', 'BUDGET_EXCEEDED', 'LOW_STOCK', 'CUSTOM'
    )),
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(tenant_id, code)
);

CREATE TABLE IF NOT EXISTS email_queue (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id),
    template_id BIGINT REFERENCES email_templates(id),
    to_email VARCHAR(500) NOT NULL,
    cc_email VARCHAR(500),
    bcc_email VARCHAR(500),
    subject VARCHAR(500) NOT NULL,
    body_html TEXT,
    body_text TEXT,
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING', 'SENT', 'FAILED', 'CANCELLED')),
    retry_count INT NOT NULL DEFAULT 0,
    max_retries INT NOT NULL DEFAULT 3,
    last_error TEXT,
    sent_at TIMESTAMPTZ,
    entity_type VARCHAR(50),
    entity_id BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_email_queue_status ON email_queue(tenant_id, status, created_at);

-- =====================================================
-- E-01: Customer field expansion
-- =====================================================
ALTER TABLE customers ADD COLUMN IF NOT EXISTS billing_address TEXT;
ALTER TABLE customers ADD COLUMN IF NOT EXISTS shipping_address TEXT;
ALTER TABLE customers ADD COLUMN IF NOT EXISTS customer_group VARCHAR(50);
ALTER TABLE customers ADD COLUMN IF NOT EXISTS price_level VARCHAR(20) DEFAULT 'RETAIL' CHECK (price_level IN ('RETAIL', 'WHOLESALE', 'DISTRIBUTOR', 'SPECIAL'));
ALTER TABLE customers ADD COLUMN IF NOT EXISTS currency_code CHAR(3) DEFAULT 'IDR';
ALTER TABLE customers ADD COLUMN IF NOT EXISTS is_pkp BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE customers ADD COLUMN IF NOT EXISTS credit_hold BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE customers ADD COLUMN IF NOT EXISTS website VARCHAR(200);
ALTER TABLE customers ADD COLUMN IF NOT EXISTS fax VARCHAR(50);
ALTER TABLE customers ADD COLUMN IF NOT EXISTS contact_person_2 VARCHAR(200);
ALTER TABLE customers ADD COLUMN IF NOT EXISTS phone_2 VARCHAR(50);
ALTER TABLE customers ADD COLUMN IF NOT EXISTS npwp_name VARCHAR(200);
ALTER TABLE customers ADD COLUMN IF NOT EXISTS opening_balance_cents BIGINT DEFAULT 0;
ALTER TABLE customers ADD COLUMN IF NOT EXISTS opening_balance_date DATE;

-- =====================================================
-- E-02: Supplier field expansion
-- =====================================================
ALTER TABLE suppliers ADD COLUMN IF NOT EXISTS supplier_type VARCHAR(20) DEFAULT 'GOODS' CHECK (supplier_type IN ('GOODS', 'SERVICE', 'MIXED'));
ALTER TABLE suppliers ADD COLUMN IF NOT EXISTS is_pkp BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE suppliers ADD COLUMN IF NOT EXISTS currency_code CHAR(3) DEFAULT 'IDR';
ALTER TABLE suppliers ADD COLUMN IF NOT EXISTS bank_name VARCHAR(100);
ALTER TABLE suppliers ADD COLUMN IF NOT EXISTS bank_account_number VARCHAR(50);
ALTER TABLE suppliers ADD COLUMN IF NOT EXISTS bank_account_name VARCHAR(200);
ALTER TABLE suppliers ADD COLUMN IF NOT EXISTS website VARCHAR(200);
ALTER TABLE suppliers ADD COLUMN IF NOT EXISTS fax VARCHAR(50);
ALTER TABLE suppliers ADD COLUMN IF NOT EXISTS contact_person_2 VARCHAR(200);
ALTER TABLE suppliers ADD COLUMN IF NOT EXISTS phone_2 VARCHAR(50);
ALTER TABLE suppliers ADD COLUMN IF NOT EXISTS opening_balance_cents BIGINT DEFAULT 0;
ALTER TABLE suppliers ADD COLUMN IF NOT EXISTS opening_balance_date DATE;

-- =====================================================
-- E-03: Item field expansion
-- =====================================================
ALTER TABLE items ADD COLUMN IF NOT EXISTS barcode VARCHAR(100);
ALTER TABLE items ADD COLUMN IF NOT EXISTS secondary_uom VARCHAR(20);
ALTER TABLE items ADD COLUMN IF NOT EXISTS uom_conversion_factor NUMERIC(18,6) DEFAULT 1;
ALTER TABLE items ADD COLUMN IF NOT EXISTS brand VARCHAR(100);
ALTER TABLE items ADD COLUMN IF NOT EXISTS category VARCHAR(100);
ALTER TABLE items ADD COLUMN IF NOT EXISTS weight_grams NUMERIC(18,3);
ALTER TABLE items ADD COLUMN IF NOT EXISTS volume_cc NUMERIC(18,3);
ALTER TABLE items ADD COLUMN IF NOT EXISTS description_long TEXT;
ALTER TABLE items ADD COLUMN IF NOT EXISTS image_url VARCHAR(500);
ALTER TABLE items ADD COLUMN IF NOT EXISTS reorder_point NUMERIC(18,3) DEFAULT 0;
ALTER TABLE items ADD COLUMN IF NOT EXISTS reorder_qty NUMERIC(18,3) DEFAULT 0;
ALTER TABLE items ADD COLUMN IF NOT EXISTS lead_time_days INT DEFAULT 0;
ALTER TABLE items ADD COLUMN IF NOT EXISTS preferred_supplier_id BIGINT;
ALTER TABLE items ADD COLUMN IF NOT EXISTS abc_classification CHAR(1) CHECK (abc_classification IN ('A', 'B', 'C'));
ALTER TABLE items ADD COLUMN IF NOT EXISTS sale_uom VARCHAR(20);
ALTER TABLE items ADD COLUMN IF NOT EXISTS purchase_uom VARCHAR(20);

-- E-04: Sales order customer PO fields
ALTER TABLE sales_orders ADD COLUMN IF NOT EXISTS customer_po_number VARCHAR(100);
ALTER TABLE sales_orders ADD COLUMN IF NOT EXISTS customer_po_date DATE;
ALTER TABLE sales_orders ADD COLUMN IF NOT EXISTS requested_delivery_date DATE;
ALTER TABLE sales_orders ADD COLUMN IF NOT EXISTS salesperson_id BIGINT;
ALTER TABLE sales_orders ADD COLUMN IF NOT EXISTS ship_to_address TEXT;
ALTER TABLE sales_orders ADD COLUMN IF NOT EXISTS shipping_terms VARCHAR(20) DEFAULT 'FOB' CHECK (shipping_terms IN ('FOB', 'CIF', 'EXW', 'CFR', 'DAP'));

-- E-05: Invoice tax invoice number + multi-charge
ALTER TABLE invoices ADD COLUMN IF NOT EXISTS tax_invoice_number VARCHAR(100);
ALTER TABLE invoices ADD COLUMN IF NOT EXISTS sub_total_cents BIGINT DEFAULT 0;
ALTER TABLE invoices ADD COLUMN IF NOT EXISTS discount_total_cents BIGINT DEFAULT 0;
ALTER TABLE invoices ADD COLUMN IF NOT EXISTS tax_total_cents BIGINT DEFAULT 0;
ALTER TABLE invoices ADD COLUMN IF NOT EXISTS shipping_fee_cents BIGINT DEFAULT 0;
ALTER TABLE invoices ADD COLUMN IF NOT EXISTS other_charges_cents BIGINT DEFAULT 0;
ALTER TABLE invoices ADD COLUMN IF NOT EXISTS rounding_cents BIGINT DEFAULT 0;
ALTER TABLE invoices ADD COLUMN IF NOT EXISTS salesperson_id BIGINT;

-- E-06: Purchase order expected date + supplier quote
ALTER TABLE purchase_orders ADD COLUMN IF NOT EXISTS supplier_quote_number VARCHAR(100);
ALTER TABLE purchase_orders ADD COLUMN IF NOT EXISTS supplier_quote_date DATE;
ALTER TABLE purchase_orders ADD COLUMN IF NOT EXISTS buyer_id BIGINT;

-- =====================================================
-- RLS policies for new tables
-- =====================================================
ALTER TABLE cheques ENABLE ROW LEVEL SECURITY;
CREATE POLICY cheques_tenant_isolation ON cheques USING (tenant_id = current_setting('app.tenant_id', true)::bigint);

ALTER TABLE cost_centers ENABLE ROW LEVEL SECURITY;
CREATE POLICY cost_centers_tenant_isolation ON cost_centers USING (tenant_id = current_setting('app.tenant_id', true)::bigint);

ALTER TABLE cost_center_allocations ENABLE ROW LEVEL SECURITY;
CREATE POLICY cost_center_allocations_tenant_isolation ON cost_center_allocations USING (tenant_id = current_setting('app.tenant_id', true)::bigint);

ALTER TABLE budget_variance_reports ENABLE ROW LEVEL SECURITY;
CREATE POLICY budget_variance_tenant_isolation ON budget_variance_reports USING (tenant_id = current_setting('app.tenant_id', true)::bigint);

ALTER TABLE email_templates ENABLE ROW LEVEL SECURITY;
CREATE POLICY email_templates_tenant_isolation ON email_templates USING (tenant_id = current_setting('app.tenant_id', true)::bigint);

ALTER TABLE email_queue ENABLE ROW LEVEL SECURITY;
CREATE POLICY email_queue_tenant_isolation ON email_queue USING (tenant_id = current_setting('app.tenant_id', true)::bigint);
