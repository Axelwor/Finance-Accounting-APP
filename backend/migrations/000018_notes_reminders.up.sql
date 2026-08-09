-- P2-016: Catatan atas Laporan Keuangan (basic) + Pengingat Jatuh Tempo.
--
-- financial_notes stores free-form notes attached to a reporting period
-- (year). These accompany the Laporan Posisi Keuangan (Balance Sheet) and
-- other financial statements as the basic "Catatan atas Laporan Keuangan"
-- disclosure text. Notes are tenant-scoped and ordered by display_order
-- then note_number within a period.
--
-- Due date reminders (Pengingat Jatuh Tempo) are view-only: they query the
-- existing invoices (customer) and supplier_invoices (supplier) tables for
-- rows whose due_date is approaching, so no new table is required here.

CREATE TABLE financial_notes (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    period_year INT NOT NULL,
    note_number TEXT NOT NULL,
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    display_order INT NOT NULL DEFAULT 0,
    created_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, period_year, note_number)
);

CREATE INDEX financial_notes_tenant_year_idx ON financial_notes (tenant_id, period_year, display_order, note_number);

ALTER TABLE financial_notes ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation_financial_notes ON financial_notes
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::BIGINT);

ALTER TABLE financial_notes FORCE ROW LEVEL SECURITY;
