-- M-014: Seed Output VAT account (2202) for PPN posting.
-- When a sales invoice has tax_rate > 0, the invoice journal now posts:
--   Dr 1201 AR (DPP + PPN) / Cr 4101 Revenue (DPP) / Cr 2202 Output VAT (PPN)

INSERT INTO accounts (tenant_id, code, name, report_group, account_type)
SELECT t.id, '2202', 'Output VAT (PPN Keluaran)', 'liability', 'VAT_PAYABLE'
FROM tenants t
WHERE NOT EXISTS (
    SELECT 1 FROM accounts a WHERE a.tenant_id = t.id AND a.code = '2202'
);
