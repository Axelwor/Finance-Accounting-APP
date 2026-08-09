-- US-100: Lampirkan Bukti (Attachments) + US-101: Audit Trail.
--
-- attachments stores proof documents (photo struk, invoice scans, PDFs)
-- attached polymorphically to any ownable entity (journal entry, invoice,
-- payment, etc.). Files are stored on local disk under
-- /data/attachments/{tenant_id}/{uuid}; this table holds the metadata.
--
-- audit_logs is an append-only trail of every mutating action (CREATE,
-- UPDATE, DELETE, POST, VOID, ...). Each row captures before/after JSONB
-- snapshots so the full diff can be reconstructed. Rows are tenant-scoped
-- and filtered by entity or user.

CREATE TABLE attachments (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id),
    owner_type TEXT NOT NULL CHECK (owner_type IN ('journal_entry','invoice','payment','grn','delivery_order','credit_note','supplier_invoice','supplier_payment','purchase_return','fixed_asset')),
    owner_id BIGINT NOT NULL,
    file_name TEXT NOT NULL,
    file_key TEXT NOT NULL,
    mime_type TEXT NOT NULL,
    file_size BIGINT NOT NULL CHECK (file_size > 0),
    ocr_status TEXT NOT NULL DEFAULT 'NONE' CHECK (ocr_status IN ('NONE','PENDING','COMPLETED','FAILED')),
    ocr_result JSONB,
    uploaded_by BIGINT REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id)
);
CREATE INDEX attachments_owner_idx ON attachments (tenant_id, owner_type, owner_id);

-- Audit trail (append-only). No UPDATE/DELETE path is ever issued from the
-- application; the table is a ledger of who-did-what-when.
CREATE TABLE audit_logs (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id),
    user_id BIGINT REFERENCES users(id),
    entity_type TEXT NOT NULL,
    entity_id BIGINT,
    action TEXT NOT NULL CHECK (action IN ('CREATE','UPDATE','DELETE','POST','VOID','CLOSE','UNLOCK','APPROVE','REJECT')),
    before_data JSONB,
    after_data JSONB,
    ip_address TEXT,
    user_agent TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id)
);
CREATE INDEX audit_logs_tenant_entity_idx ON audit_logs (tenant_id, entity_type, entity_id, created_at DESC);
CREATE INDEX audit_logs_tenant_user_idx ON audit_logs (tenant_id, user_id, created_at DESC);

-- Row Level Security: both tables are tenant-scoped. The application sets
-- app.tenant_id at the start of each transaction (see scopeTenant helpers).
ALTER TABLE attachments ENABLE ROW LEVEL SECURITY;
ALTER TABLE audit_logs ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation_attachments ON attachments
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::BIGINT);

CREATE POLICY tenant_isolation_audit_logs ON audit_logs
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::BIGINT);

ALTER TABLE attachments FORCE ROW LEVEL SECURITY;
ALTER TABLE audit_logs FORCE ROW LEVEL SECURITY;
