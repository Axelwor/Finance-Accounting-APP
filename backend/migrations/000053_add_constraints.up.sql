-- Add CHECK constraints and missing foreign keys
-- Phase 5: Database Hardening Wave 5

-- =====================================================
-- CHECK CONSTRAINTS: All *_cents columns >= 0
-- =====================================================

-- invoices
ALTER TABLE invoices ADD CONSTRAINT IF NOT EXISTS invoices_total_cents_nonneg CHECK (total_cents >= 0);
ALTER TABLE invoices ADD CONSTRAINT IF NOT EXISTS invoices_dp_applied_cents_nonneg CHECK (dp_applied_cents >= 0);

-- invoice_lines
ALTER TABLE invoice_lines ADD CONSTRAINT IF NOT EXISTS invoice_lines_unit_price_cents_nonneg CHECK (unit_price_cents >= 0);
ALTER TABLE invoice_lines ADD CONSTRAINT IF NOT EXISTS invoice_lines_discount_cents_nonneg CHECK (discount_cents >= 0);
ALTER TABLE invoice_lines ADD CONSTRAINT IF NOT EXISTS invoice_lines_line_total_cents_nonneg CHECK (line_total_cents >= 0);

-- sales_orders
ALTER TABLE sales_orders ADD CONSTRAINT IF NOT EXISTS sales_orders_total_cents_nonneg CHECK (total_cents >= 0);
ALTER TABLE sales_orders ADD CONSTRAINT IF NOT EXISTS sales_orders_dp_received_cents_nonneg CHECK (dp_received_cents >= 0);

-- sales_orders_lines
ALTER TABLE sales_orders_lines ADD CONSTRAINT IF NOT EXISTS sales_orders_lines_unit_price_cents_nonneg CHECK (unit_price_cents >= 0);
ALTER TABLE sales_orders_lines ADD CONSTRAINT IF NOT EXISTS sales_orders_lines_discount_cents_nonneg CHECK (discount_cents >= 0);
ALTER TABLE sales_orders_lines ADD CONSTRAINT IF NOT EXISTS sales_orders_lines_line_total_cents_nonneg CHECK (line_total_cents >= 0);

-- delivery_orders
ALTER TABLE delivery_orders ADD CONSTRAINT IF NOT EXISTS delivery_orders_total_cents_nonneg CHECK (total_cents >= 0);
ALTER TABLE delivery_orders ADD CONSTRAINT IF NOT EXISTS delivery_orders_gross_profit_cents_nonneg CHECK (gross_profit_cents >= 0);

-- delivery_orders_lines
ALTER TABLE delivery_orders_lines ADD CONSTRAINT IF NOT EXISTS delivery_orders_lines_unit_price_cents_nonneg CHECK (unit_price_cents >= 0);
ALTER TABLE delivery_orders_lines ADD CONSTRAINT IF NOT EXISTS delivery_orders_lines_discount_cents_nonneg CHECK (discount_cents >= 0);
ALTER TABLE delivery_orders_lines ADD CONSTRAINT IF NOT EXISTS delivery_orders_lines_line_total_cents_nonneg CHECK (line_total_cents >= 0);
ALTER TABLE delivery_orders_lines ADD CONSTRAINT IF NOT EXISTS delivery_orders_lines_cogs_cents_nonneg CHECK (cogs_cents >= 0);

-- sales_quotations
ALTER TABLE sales_quotations ADD CONSTRAINT IF NOT EXISTS sales_quotations_total_cents_nonneg CHECK (total_cents >= 0);

-- sales_quotations_lines
ALTER TABLE sales_quotations_lines ADD CONSTRAINT IF NOT EXISTS sales_quotations_lines_unit_price_cents_nonneg CHECK (unit_price_cents >= 0);
ALTER TABLE sales_quotations_lines ADD CONSTRAINT IF NOT EXISTS sales_quotations_lines_discount_cents_nonneg CHECK (discount_cents >= 0);
ALTER TABLE sales_quotations_lines ADD CONSTRAINT IF NOT EXISTS sales_quotations_lines_line_total_cents_nonneg CHECK (line_total_cents >= 0);

-- purchase_orders
ALTER TABLE purchase_orders ADD CONSTRAINT IF NOT EXISTS purchase_orders_total_cents_nonneg CHECK (total_cents >= 0);
ALTER TABLE purchase_orders ADD CONSTRAINT IF NOT EXISTS purchase_orders_received_cents_nonneg CHECK (received_cents >= 0);

-- purchase_orders_lines
ALTER TABLE purchase_orders_lines ADD CONSTRAINT IF NOT EXISTS purchase_orders_lines_unit_price_cents_nonneg CHECK (unit_price_cents >= 0);
ALTER TABLE purchase_orders_lines ADD CONSTRAINT IF NOT EXISTS purchase_orders_lines_discount_cents_nonneg CHECK (discount_cents >= 0);
ALTER TABLE purchase_orders_lines ADD CONSTRAINT IF NOT EXISTS purchase_orders_lines_line_total_cents_nonneg CHECK (line_total_cents >= 0);

-- goods_received_notes
ALTER TABLE goods_received_notes ADD CONSTRAINT IF NOT EXISTS goods_received_notes_total_cents_nonneg CHECK (total_cents >= 0);

-- grn_lines
ALTER TABLE grn_lines ADD CONSTRAINT IF NOT EXISTS grn_lines_unit_cost_cents_nonneg CHECK (unit_cost_cents >= 0);
ALTER TABLE grn_lines ADD CONSTRAINT IF NOT EXISTS grn_lines_amount_cents_nonneg CHECK (amount_cents >= 0);

-- supplier_invoices
ALTER TABLE supplier_invoices ADD CONSTRAINT IF NOT EXISTS supplier_invoices_dpp_cents_nonneg CHECK (dpp_cents >= 0);
ALTER TABLE supplier_invoices ADD CONSTRAINT IF NOT EXISTS supplier_invoices_vat_cents_nonneg CHECK (vat_cents >= 0);
ALTER TABLE supplier_invoices ADD CONSTRAINT IF NOT EXISTS supplier_invoices_total_cents_nonneg CHECK (total_cents >= 0);
ALTER TABLE supplier_invoices ADD CONSTRAINT IF NOT EXISTS supplier_invoices_dp_applied_cents_nonneg CHECK (dp_applied_cents >= 0);
ALTER TABLE supplier_invoices ADD CONSTRAINT IF NOT EXISTS supplier_invoices_payable_cents_nonneg CHECK (payable_cents >= 0);

-- supplier_invoice_lines
ALTER TABLE supplier_invoice_lines ADD CONSTRAINT IF NOT EXISTS supplier_invoice_lines_unit_price_cents_nonneg CHECK (unit_price_cents >= 0);
ALTER TABLE supplier_invoice_lines ADD CONSTRAINT IF NOT EXISTS supplier_invoice_lines_discount_cents_nonneg CHECK (discount_cents >= 0);
ALTER TABLE supplier_invoice_lines ADD CONSTRAINT IF NOT EXISTS supplier_invoice_lines_line_total_cents_nonneg CHECK (line_total_cents >= 0);

-- cheque_payments
ALTER TABLE cheque_payments ADD CONSTRAINT IF NOT EXISTS cheque_payments_amount_cents_nonneg CHECK (amount_cents >= 0);

-- cheque_transfers
ALTER TABLE cheque_transfers ADD CONSTRAINT IF NOT EXISTS cheque_transfers_amount_cents_nonneg CHECK (amount_cents >= 0);

-- petty_cash_funds
ALTER TABLE petty_cash_funds ADD CONSTRAINT IF NOT EXISTS petty_cash_funds_balance_cents_nonneg CHECK (balance_cents >= 0);

-- petty_cash_vouchers
ALTER TABLE petty_cash_vouchers ADD CONSTRAINT IF NOT EXISTS petty_cash_vouchers_amount_cents_nonneg CHECK (amount_cents >= 0);

-- cash_entries
ALTER TABLE cash_entries ADD CONSTRAINT IF NOT EXISTS cash_entries_amount_cents_nonneg CHECK (amount_cents >= 0);

-- journal_adjustments
ALTER TABLE journal_adjustments ADD CONSTRAINT IF NOT EXISTS journal_adjustments_amount_cents_nonneg CHECK (amount_cents >= 0);

-- pph_calculations
ALTER TABLE pph_calculations ADD CONSTRAINT IF NOT EXISTS pph_calculations_dpp_cents_nonneg CHECK (dpp_cents >= 0);
ALTER TABLE pph_calculations ADD CONSTRAINT IF NOT EXISTS pph_calculations_pph_cents_nonneg CHECK (pph_cents >= 0);

-- inventory_movements
ALTER TABLE inventory_movements ADD CONSTRAINT IF NOT EXISTS inventory_movements_qty_cents_nonneg CHECK (qty_cents >= 0);

-- stock_opnames
ALTER TABLE stock_opnames ADD CONSTRAINT IF NOT EXISTS stock_opnames_total_adjustment_cents_nonneg CHECK (total_adjustment_cents >= 0);

-- stock_opname_lines
ALTER TABLE stock_opname_lines ADD CONSTRAINT IF NOT EXISTS stock_opname_lines_adjustment_cents_nonneg CHECK (adjustment_cents >= 0);
ALTER TABLE stock_opname_lines ADD CONSTRAINT IF NOT EXISTS stock_opname_lines_unit_cost_cents_nonneg CHECK (unit_cost_cents >= 0);

-- stock_transfers
ALTER TABLE stock_transfers ADD CONSTRAINT IF NOT EXISTS stock_transfers_total_cents_nonneg CHECK (total_cents >= 0);

-- budget_allocations
ALTER TABLE budget_allocations ADD CONSTRAINT IF NOT EXISTS budget_allocations_amount_cents_nonneg CHECK (amount_cents >= 0);

-- production_jobs
ALTER TABLE production_jobs ADD CONSTRAINT IF NOT EXISTS production_jobs_target_cost_cents_nonneg CHECK (target_cost_cents >= 0);

-- production_job_costs
ALTER TABLE production_job_costs ADD CONSTRAINT IF NOT EXISTS production_job_costs_amount_cents_nonneg CHECK (amount_cents >= 0);

-- fixed_assets
ALTER TABLE fixed_assets ADD CONSTRAINT IF NOT EXISTS fixed_assets_acquisition_cost_cents_nonneg CHECK (acquisition_cost_cents >= 0);
ALTER TABLE fixed_assets ADD CONSTRAINT IF NOT EXISTS fixed_assets_accumulated_depreciation_cents_nonneg CHECK (accumulated_depreciation_cents >= 0);

-- lease_contracts
ALTER TABLE lease_contracts ADD CONSTRAINT IF NOT EXISTS lease_contracts_monthly_rent_cents_nonneg CHECK (monthly_rent_cents >= 0);

-- recurring_transactions
ALTER TABLE recurring_transactions ADD CONSTRAINT IF NOT EXISTS recurring_transactions_amount_cents_nonneg CHECK (amount_cents >= 0);

-- items
ALTER TABLE items ADD CONSTRAINT IF NOT EXISTS items_purchase_price_cents_nonneg CHECK (purchase_price_cents >= 0);
ALTER TABLE items ADD CONSTRAINT IF NOT EXISTS items_sale_price_cents_nonneg CHECK (sale_price_cents >= 0);

-- customers
ALTER TABLE customers ADD CONSTRAINT IF NOT EXISTS customers_credit_limit_cents_nonneg CHECK (credit_limit_cents >= 0);

-- suppliers
ALTER TABLE suppliers ADD CONSTRAINT IF NOT EXISTS suppliers_credit_limit_cents_nonneg CHECK (credit_limit_cents >= 0);

-- =====================================================
-- CHECK CONSTRAINT: payment_terms.discount_percent
-- =====================================================

ALTER TABLE payment_terms ADD CONSTRAINT IF NOT EXISTS payment_terms_discount_percent_check 
CHECK (discount_percent IS NULL OR (discount_percent >= 0 AND discount_percent <= 100));

-- =====================================================
-- UNIQUE CONSTRAINT: tax_rates(tenant_id, tax_type, effective_from)
-- =====================================================

ALTER TABLE tax_rates ADD CONSTRAINT IF NOT EXISTS tax_rates_unique_tenant_tax_effective 
UNIQUE (tenant_id, tax_type, effective_from);

-- =====================================================
-- CHECK CONSTRAINT: recurring_transactions.intent_type
-- =====================================================

ALTER TABLE recurring_transactions ADD CONSTRAINT IF NOT EXISTS recurring_transactions_intent_type_check 
CHECK (intent_type IN ('CASH_IN', 'CASH_OUT', 'TRANSFER', 'MANUAL_JOURNAL'));

-- =====================================================
-- MISSING FOREIGN KEYS
-- =====================================================

-- cheques.journal_entry_id REFERENCES journal_entries(id)
ALTER TABLE cheques ADD CONSTRAINT IF NOT EXISTS cheques_journal_entry_id_fkey 
FOREIGN KEY (tenant_id, journal_entry_id) REFERENCES journal_entries(tenant_id, id) ON DELETE SET NULL;

-- cheques.payment_id REFERENCES invoice_payments(id) - for cheque clearing against invoice
ALTER TABLE cheques ADD CONSTRAINT IF NOT EXISTS cheques_payment_id_fkey 
FOREIGN KEY (payment_id) REFERENCES invoice_payments(id) ON DELETE SET NULL;

-- cost_centers.parent_id REFERENCES cost_centers(id) - self-referencing for hierarchy
ALTER TABLE cost_centers ADD CONSTRAINT IF NOT EXISTS cost_centers_parent_id_fkey 
FOREIGN KEY (parent_id) REFERENCES cost_centers(id) ON DELETE CASCADE;
