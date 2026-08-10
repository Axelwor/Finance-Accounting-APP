-- 000030: Report templates + Dashboard widget system
-- Fulfils audit items N-01..N-10 (NextReport integration) and D-01..D-02 (dashboard)

-- ==========================================================
-- REPORT TEMPLATES
-- ==========================================================

CREATE TABLE IF NOT EXISTS report_templates (
    id              BIGSERIAL PRIMARY KEY,
    tenant_id       BIGINT      NOT NULL REFERENCES tenants(id),
    code            TEXT        NOT NULL,
    name            TEXT        NOT NULL,
    document_type   TEXT        NOT NULL CHECK (document_type IN (
        'invoice', 'purchase_order', 'delivery_order', 'tax_invoice',
        'withholding_slip', 'customer_statement', 'supplier_statement',
        'payment_voucher', 'receipt_voucher', 'journal_voucher',
        'stock_card', 'trial_balance', 'profit_loss', 'balance_sheet',
        'cash_flow', 'ar_aging', 'ap_aging', 'asset_register', 'stock_opname'
    )),
    template_yaml  TEXT         NOT NULL DEFAULT '',
    data_source_sql TEXT        NOT NULL DEFAULT '',
    output_format   TEXT        NOT NULL DEFAULT 'pdf' CHECK (output_format IN ('pdf', 'html', 'xlsx')),
    is_default      BOOLEAN      NOT NULL DEFAULT FALSE,
    is_active       BOOLEAN      NOT NULL DEFAULT TRUE,
    created_by      BIGINT,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, code)
);

-- RLS
ALTER TABLE report_templates ENABLE ROW LEVEL SECURITY;
CREATE POLICY report_templates_tenant ON report_templates
    USING (tenant_id = current_setting('app.tenant_id', true)::bigint);

-- ==========================================================
-- DASHBOARD LAYOUTS (per-user)
-- ==========================================================

CREATE TABLE IF NOT EXISTS dashboard_layouts (
    id          BIGSERIAL PRIMARY KEY,
    tenant_id   BIGINT      NOT NULL REFERENCES tenants(id),
    user_id     BIGINT      NOT NULL,
    name        TEXT        NOT NULL DEFAULT 'Default',
    layout_json JSONB       NOT NULL DEFAULT '[]'::jsonb,
    is_active   BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, user_id, name)
);

-- RLS
ALTER TABLE dashboard_layouts ENABLE ROW LEVEL SECURITY;
CREATE POLICY dashboard_layouts_tenant ON dashboard_layouts
    USING (tenant_id = current_setting('app.tenant_id', true)::bigint);

-- ==========================================================
-- DASHBOARD WIDGETS (per-user, per-layout)
-- ==========================================================

CREATE TABLE IF NOT EXISTS dashboard_widgets (
    id          BIGSERIAL PRIMARY KEY,
    tenant_id   BIGINT      NOT NULL REFERENCES tenants(id),
    layout_id   BIGINT      NOT NULL REFERENCES dashboard_layouts(id) ON DELETE CASCADE,
    widget_type TEXT        NOT NULL CHECK (widget_type IN (
        'kpi_cash', 'kpi_ar', 'kpi_ap', 'kpi_pl', 'kpi_low_stock',
        'ar_aging_summary', 'ap_aging_summary', 'cash_flow_forecast',
        'pl_snapshot', 'budget_vs_actual', 'revenue_by_customer',
        'top_selling_items', 'low_stock_alert', 'recent_transactions',
        'outstanding_invoices', 'bank_balance', 'tax_summary',
        'production_status', 'asset_summary', 'bank_recon_status',
        'period_status'
    )),
    title       TEXT        NOT NULL,
    config_json JSONB       NOT NULL DEFAULT '{}'::jsonb,
    grid_x      INT         NOT NULL DEFAULT 0,
    grid_y      INT         NOT NULL DEFAULT 0,
    grid_w      INT         NOT NULL DEFAULT 4,
    grid_h      INT         NOT NULL DEFAULT 3,
    sort_order  INT         NOT NULL DEFAULT 0,
    is_visible  BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- RLS
ALTER TABLE dashboard_widgets ENABLE ROW LEVEL SECURITY;
CREATE POLICY dashboard_widgets_tenant ON dashboard_widgets
    USING (tenant_id = current_setting('app.tenant_id', true)::bigint);

-- ==========================================================
-- SEED: Default report templates (YAML skeletons)
-- ==========================================================

INSERT INTO report_templates (tenant_id, code, name, document_type, template_yaml, is_default, is_active)
SELECT 0, 'INV-STD', 'Standard Invoice', 'invoice',
'document:
  title: INVOICE
  size: A4
  margin: 20mm
header:
  - company_name
  - company_address
  - company_phone
body:
  - invoice_number
  - invoice_date
  - due_date
  - customer_name
  - customer_address
  - lines_table: [no, description, qty, unit_price, line_total]
  - subtotal
  - tax_total
  - grand_total
footer:
  - terms_and_conditions
  - signature_block
', TRUE, TRUE
WHERE NOT EXISTS (SELECT 1 FROM report_templates WHERE code = 'INV-STD');

INSERT INTO report_templates (tenant_id, code, name, document_type, template_yaml, is_default, is_active)
SELECT 0, 'PO-STD', 'Standard Purchase Order', 'purchase_order',
'document:
  title: PURCHASE ORDER
  size: A4
  margin: 20mm
header:
  - company_name
  - supplier_name
body:
  - po_number
  - po_date
  - expected_date
  - lines_table: [no, description, qty, unit_price, line_total]
  - subtotal
  - tax_total
  - grand_total
footer:
  - terms_and_conditions
  - authorized_signature
', TRUE, TRUE
WHERE NOT EXISTS (SELECT 1 FROM report_templates WHERE code = 'PO-STD');

INSERT INTO report_templates (tenant_id, code, name, document_type, template_yaml, is_default, is_active)
SELECT 0, 'DO-STD', 'Delivery Order', 'delivery_order',
'document:
  title: SURAT JALAN
  size: A4
header:
  - company_name
  - customer_name
body:
  - do_number
  - delivery_date
  - lines_table: [no, description, qty, unit]
  - received_by
  - received_date
footer:
  - signature_block
', TRUE, TRUE
WHERE NOT EXISTS (SELECT 1 FROM report_templates WHERE code = 'DO-STD');
