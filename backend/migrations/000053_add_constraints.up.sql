-- Add CHECK constraints and missing foreign keys
-- Phase 5: Database Hardening Wave 5

-- =====================================================
-- CHECK CONSTRAINTS: All *_cents columns >= 0
-- =====================================================

-- invoices
DO $$ BEGIN
    ALTER TABLE invoices ADD CONSTRAINT invoices_total_cents_nonneg CHECK (total_cents >= 0);
EXCEPTION WHEN duplicate_object OR undefined_column OR undefined_table THEN NULL; END $$;
DO $$ BEGIN
    ALTER TABLE invoices ADD CONSTRAINT invoices_dp_applied_cents_nonneg CHECK (dp_applied_cents >= 0);
EXCEPTION WHEN duplicate_object OR undefined_column OR undefined_table THEN NULL; END $$;

-- invoice_lines
DO $$ BEGIN
    ALTER TABLE invoice_lines ADD CONSTRAINT invoice_lines_unit_price_cents_nonneg CHECK (unit_price_cents >= 0);
EXCEPTION WHEN duplicate_object OR undefined_column OR undefined_table THEN NULL; END $$;
DO $$ BEGIN
    ALTER TABLE invoice_lines ADD CONSTRAINT invoice_lines_discount_cents_nonneg CHECK (discount_cents >= 0);
EXCEPTION WHEN duplicate_object OR undefined_column OR undefined_table THEN NULL; END $$;
DO $$ BEGIN
    ALTER TABLE invoice_lines ADD CONSTRAINT invoice_lines_line_total_cents_nonneg CHECK (line_total_cents >= 0);
EXCEPTION WHEN duplicate_object OR undefined_column OR undefined_table THEN NULL; END $$;

-- sales_orders
DO $$ BEGIN
    ALTER TABLE sales_orders ADD CONSTRAINT sales_orders_total_cents_nonneg CHECK (total_cents >= 0);
EXCEPTION WHEN duplicate_object OR undefined_column OR undefined_table THEN NULL; END $$;
DO $$ BEGIN
    ALTER TABLE sales_orders ADD CONSTRAINT sales_orders_dp_received_cents_nonneg CHECK (dp_received_cents >= 0);
EXCEPTION WHEN duplicate_object OR undefined_column OR undefined_table THEN NULL; END $$;

-- sales_orders_lines
DO $$ BEGIN
    ALTER TABLE sales_orders_lines ADD CONSTRAINT sales_orders_lines_unit_price_cents_nonneg CHECK (unit_price_cents >= 0);
EXCEPTION WHEN duplicate_object OR undefined_column OR undefined_table THEN NULL; END $$;
DO $$ BEGIN
    ALTER TABLE sales_orders_lines ADD CONSTRAINT sales_orders_lines_discount_cents_nonneg CHECK (discount_cents >= 0);
EXCEPTION WHEN duplicate_object OR undefined_column OR undefined_table THEN NULL; END $$;
DO $$ BEGIN
    ALTER TABLE sales_orders_lines ADD CONSTRAINT sales_orders_lines_line_total_cents_nonneg CHECK (line_total_cents >= 0);
EXCEPTION WHEN duplicate_object OR undefined_column OR undefined_table THEN NULL; END $$;

-- delivery_orders
DO $$ BEGIN
    ALTER TABLE delivery_orders ADD CONSTRAINT delivery_orders_total_cents_nonneg CHECK (total_cents >= 0);
EXCEPTION WHEN duplicate_object OR undefined_column OR undefined_table THEN NULL; END $$;
DO $$ BEGIN
    ALTER TABLE delivery_orders ADD CONSTRAINT delivery_orders_gross_profit_cents_nonneg CHECK (gross_profit_cents >= 0);
EXCEPTION WHEN duplicate_object OR undefined_column OR undefined_table THEN NULL; END $$;

-- delivery_orders_lines
DO $$ BEGIN
    ALTER TABLE delivery_orders_lines ADD CONSTRAINT delivery_orders_lines_unit_price_cents_nonneg CHECK (unit_price_cents >= 0);
EXCEPTION WHEN duplicate_object OR undefined_column OR undefined_table THEN NULL; END $$;
DO $$ BEGIN
    ALTER TABLE delivery_orders_lines ADD CONSTRAINT delivery_orders_lines_discount_cents_nonneg CHECK (discount_cents >= 0);
EXCEPTION WHEN duplicate_object OR undefined_column OR undefined_table THEN NULL; END $$;
DO $$ BEGIN
    ALTER TABLE delivery_orders_lines ADD CONSTRAINT delivery_orders_lines_line_total_cents_nonneg CHECK (line_total_cents >= 0);
EXCEPTION WHEN duplicate_object OR undefined_column OR undefined_table THEN NULL; END $$;
DO $$ BEGIN
    ALTER TABLE delivery_orders_lines ADD CONSTRAINT delivery_orders_lines_cogs_cents_nonneg CHECK (cogs_cents >= 0);
EXCEPTION WHEN duplicate_object OR undefined_column OR undefined_table THEN NULL; END $$;

-- sales_quotations
DO $$ BEGIN
    ALTER TABLE sales_quotations ADD CONSTRAINT sales_quotations_total_cents_nonneg CHECK (total_cents >= 0);
EXCEPTION WHEN duplicate_object OR undefined_column OR undefined_table THEN NULL; END $$;

-- sales_quotations_lines
DO $$ BEGIN
    ALTER TABLE sales_quotations_lines ADD CONSTRAINT sales_quotations_lines_unit_price_cents_nonneg CHECK (unit_price_cents >= 0);
EXCEPTION WHEN duplicate_object OR undefined_column OR undefined_table THEN NULL; END $$;
DO $$ BEGIN
    ALTER TABLE sales_quotations_lines ADD CONSTRAINT sales_quotations_lines_discount_cents_nonneg CHECK (discount_cents >= 0);
EXCEPTION WHEN duplicate_object OR undefined_column OR undefined_table THEN NULL; END $$;
DO $$ BEGIN
    ALTER TABLE sales_quotations_lines ADD CONSTRAINT sales_quotations_lines_line_total_cents_nonneg CHECK (line_total_cents >= 0);
EXCEPTION WHEN duplicate_object OR undefined_column OR undefined_table THEN NULL; END $$;

-- purchase_orders
DO $$ BEGIN
    ALTER TABLE purchase_orders ADD CONSTRAINT purchase_orders_total_cents_nonneg CHECK (total_cents >= 0);
EXCEPTION WHEN duplicate_object OR undefined_column OR undefined_table THEN NULL; END $$;
DO $$ BEGIN
    ALTER TABLE purchase_orders ADD CONSTRAINT purchase_orders_received_cents_nonneg CHECK (received_cents >= 0);
EXCEPTION WHEN duplicate_object OR undefined_column OR undefined_table THEN NULL; END $$;

-- purchase_orders_lines
DO $$ BEGIN
    ALTER TABLE purchase_orders_lines ADD CONSTRAINT purchase_orders_lines_unit_price_cents_nonneg CHECK (unit_price_cents >= 0);
EXCEPTION WHEN duplicate_object OR undefined_column OR undefined_table THEN NULL; END $$;
DO $$ BEGIN
    ALTER TABLE purchase_orders_lines ADD CONSTRAINT purchase_orders_lines_discount_cents_nonneg CHECK (discount_cents >= 0);
EXCEPTION WHEN duplicate_object OR undefined_column OR undefined_table THEN NULL; END $$;
DO $$ BEGIN
    ALTER TABLE purchase_orders_lines ADD CONSTRAINT purchase_orders_lines_line_total_cents_nonneg CHECK (line_total_cents >= 0);
EXCEPTION WHEN duplicate_object OR undefined_column OR undefined_table THEN NULL; END $$;

-- goods_received_notes
DO $$ BEGIN
    ALTER TABLE goods_received_notes ADD CONSTRAINT goods_received_notes_total_cents_nonneg CHECK (total_cents >= 0);
EXCEPTION WHEN duplicate_object OR undefined_column OR undefined_table THEN NULL; END $$;

-- grn_lines
DO $$ BEGIN
    ALTER TABLE grn_lines ADD CONSTRAINT grn_lines_unit_cost_cents_nonneg CHECK (unit_cost_cents >= 0);
EXCEPTION WHEN duplicate_object OR undefined_column OR undefined_table THEN NULL; END $$;
DO $$ BEGIN
    ALTER TABLE grn_lines ADD CONSTRAINT grn_lines_amount_cents_nonneg CHECK (amount_cents >= 0);
EXCEPTION WHEN duplicate_object OR undefined_column OR undefined_table THEN NULL; END $$;

-- supplier_invoices
DO $$ BEGIN
    ALTER TABLE supplier_invoices ADD CONSTRAINT supplier_invoices_dpp_cents_nonneg CHECK (dpp_cents >= 0);
EXCEPTION WHEN duplicate_object OR undefined_column OR undefined_table THEN NULL; END $$;
DO $$ BEGIN
    ALTER TABLE supplier_invoices ADD CONSTRAINT supplier_invoices_vat_cents_nonneg CHECK (vat_cents >= 0);
EXCEPTION WHEN duplicate_object OR undefined_column OR undefined_table THEN NULL; END $$;
DO $$ BEGIN
    ALTER TABLE supplier_invoices ADD CONSTRAINT supplier_invoices_total_cents_nonneg CHECK (total_cents >= 0);
EXCEPTION WHEN duplicate_object OR undefined_column OR undefined_table THEN NULL; END $$;
DO $$ BEGIN
    ALTER TABLE supplier_invoices ADD CONSTRAINT supplier_invoices_dp_applied_cents_nonneg CHECK (dp_applied_cents >= 0);
EXCEPTION WHEN duplicate_object OR undefined_column OR undefined_table THEN NULL; END $$;
DO $$ BEGIN
    ALTER TABLE supplier_invoices ADD CONSTRAINT supplier_invoices_payable_cents_nonneg CHECK (payable_cents >= 0);
EXCEPTION WHEN duplicate_object OR undefined_column OR undefined_table THEN NULL; END $$;

-- supplier_invoice_lines
DO $$ BEGIN
    ALTER TABLE supplier_invoice_lines ADD CONSTRAINT supplier_invoice_lines_unit_price_cents_nonneg CHECK (unit_price_cents >= 0);
EXCEPTION WHEN duplicate_object OR undefined_column OR undefined_table THEN NULL; END $$;
DO $$ BEGIN
    ALTER TABLE supplier_invoice_lines ADD CONSTRAINT supplier_invoice_lines_discount_cents_nonneg CHECK (discount_cents >= 0);
EXCEPTION WHEN duplicate_object OR undefined_column OR undefined_table THEN NULL; END $$;
DO $$ BEGIN
    ALTER TABLE supplier_invoice_lines ADD CONSTRAINT supplier_invoice_lines_line_total_cents_nonneg CHECK (line_total_cents >= 0);
EXCEPTION WHEN duplicate_object OR undefined_column OR undefined_table THEN NULL; END $$;

-- cheque_payments
DO $$ BEGIN
    ALTER TABLE cheque_payments ADD CONSTRAINT cheque_payments_amount_cents_nonneg CHECK (amount_cents >= 0);
EXCEPTION WHEN duplicate_object OR undefined_column OR undefined_table THEN NULL; END $$;

-- cheque_transfers
DO $$ BEGIN
    ALTER TABLE cheque_transfers ADD CONSTRAINT cheque_transfers_amount_cents_nonneg CHECK (amount_cents >= 0);
EXCEPTION WHEN duplicate_object OR undefined_column OR undefined_table THEN NULL; END $$;

-- petty_cash_funds
DO $$ BEGIN
    ALTER TABLE petty_cash_funds ADD CONSTRAINT petty_cash_funds_balance_cents_nonneg CHECK (balance_cents >= 0);
EXCEPTION WHEN duplicate_object OR undefined_column OR undefined_table THEN NULL; END $$;

-- petty_cash_vouchers
DO $$ BEGIN
    ALTER TABLE petty_cash_vouchers ADD CONSTRAINT petty_cash_vouchers_amount_cents_nonneg CHECK (amount_cents >= 0);
EXCEPTION WHEN duplicate_object OR undefined_column OR undefined_table THEN NULL; END $$;

-- cash_entries
DO $$ BEGIN
    ALTER TABLE cash_entries ADD CONSTRAINT cash_entries_amount_cents_nonneg CHECK (amount_cents >= 0);
EXCEPTION WHEN duplicate_object OR undefined_column OR undefined_table THEN NULL; END $$;

-- journal_adjustments
DO $$ BEGIN
    ALTER TABLE journal_adjustments ADD CONSTRAINT journal_adjustments_amount_cents_nonneg CHECK (amount_cents >= 0);
EXCEPTION WHEN duplicate_object OR undefined_column OR undefined_table THEN NULL; END $$;

-- pph_calculations
DO $$ BEGIN
    ALTER TABLE pph_calculations ADD CONSTRAINT pph_calculations_dpp_cents_nonneg CHECK (dpp_cents >= 0);
EXCEPTION WHEN duplicate_object OR undefined_column OR undefined_table THEN NULL; END $$;
DO $$ BEGIN
    ALTER TABLE pph_calculations ADD CONSTRAINT pph_calculations_pph_cents_nonneg CHECK (pph_cents >= 0);
EXCEPTION WHEN duplicate_object OR undefined_column OR undefined_table THEN NULL; END $$;

-- inventory_movements
DO $$ BEGIN
    ALTER TABLE inventory_movements ADD CONSTRAINT inventory_movements_qty_cents_nonneg CHECK (qty_cents >= 0);
EXCEPTION WHEN duplicate_object OR undefined_column OR undefined_table THEN NULL; END $$;

-- stock_opnames
DO $$ BEGIN
    ALTER TABLE stock_opnames ADD CONSTRAINT stock_opnames_total_adjustment_cents_nonneg CHECK (total_adjustment_cents >= 0);
EXCEPTION WHEN duplicate_object OR undefined_column OR undefined_table THEN NULL; END $$;

-- stock_opname_lines
DO $$ BEGIN
    ALTER TABLE stock_opname_lines ADD CONSTRAINT stock_opname_lines_adjustment_cents_nonneg CHECK (adjustment_cents >= 0);
EXCEPTION WHEN duplicate_object OR undefined_column OR undefined_table THEN NULL; END $$;
DO $$ BEGIN
    ALTER TABLE stock_opname_lines ADD CONSTRAINT stock_opname_lines_unit_cost_cents_nonneg CHECK (unit_cost_cents >= 0);
EXCEPTION WHEN duplicate_object OR undefined_column OR undefined_table THEN NULL; END $$;

-- stock_transfers
DO $$ BEGIN
    ALTER TABLE stock_transfers ADD CONSTRAINT stock_transfers_total_cents_nonneg CHECK (total_cents >= 0);
EXCEPTION WHEN duplicate_object OR undefined_column OR undefined_table THEN NULL; END $$;

-- budget_allocations
DO $$ BEGIN
    ALTER TABLE budget_allocations ADD CONSTRAINT budget_allocations_amount_cents_nonneg CHECK (amount_cents >= 0);
EXCEPTION WHEN duplicate_object OR undefined_column OR undefined_table THEN NULL; END $$;

-- production_jobs
DO $$ BEGIN
    ALTER TABLE production_jobs ADD CONSTRAINT production_jobs_target_cost_cents_nonneg CHECK (target_cost_cents >= 0);
EXCEPTION WHEN duplicate_object OR undefined_column OR undefined_table THEN NULL; END $$;

-- production_job_costs
DO $$ BEGIN
    ALTER TABLE production_job_costs ADD CONSTRAINT production_job_costs_amount_cents_nonneg CHECK (amount_cents >= 0);
EXCEPTION WHEN duplicate_object OR undefined_column OR undefined_table THEN NULL; END $$;

-- fixed_assets
DO $$ BEGIN
    ALTER TABLE fixed_assets ADD CONSTRAINT fixed_assets_acquisition_cost_cents_nonneg CHECK (acquisition_cost_cents >= 0);
EXCEPTION WHEN duplicate_object OR undefined_column OR undefined_table THEN NULL; END $$;
DO $$ BEGIN
    ALTER TABLE fixed_assets ADD CONSTRAINT fixed_assets_accumulated_depreciation_cents_nonneg CHECK (accumulated_depreciation_cents >= 0);
EXCEPTION WHEN duplicate_object OR undefined_column OR undefined_table THEN NULL; END $$;

-- lease_contracts
DO $$ BEGIN
    ALTER TABLE lease_contracts ADD CONSTRAINT lease_contracts_monthly_rent_cents_nonneg CHECK (monthly_rent_cents >= 0);
EXCEPTION WHEN duplicate_object OR undefined_column OR undefined_table THEN NULL; END $$;

-- recurring_transactions
DO $$ BEGIN
    ALTER TABLE recurring_transactions ADD CONSTRAINT recurring_transactions_amount_cents_nonneg CHECK (amount_cents >= 0);
EXCEPTION WHEN duplicate_object OR undefined_column OR undefined_table THEN NULL; END $$;

-- items
DO $$ BEGIN
    ALTER TABLE items ADD CONSTRAINT items_purchase_price_cents_nonneg CHECK (purchase_price_cents >= 0);
EXCEPTION WHEN duplicate_object OR undefined_column OR undefined_table THEN NULL; END $$;
DO $$ BEGIN
    ALTER TABLE items ADD CONSTRAINT items_sale_price_cents_nonneg CHECK (sale_price_cents >= 0);
EXCEPTION WHEN duplicate_object OR undefined_column OR undefined_table THEN NULL; END $$;

-- customers
DO $$ BEGIN
    ALTER TABLE customers ADD CONSTRAINT customers_credit_limit_cents_nonneg CHECK (credit_limit_cents >= 0);
EXCEPTION WHEN duplicate_object OR undefined_column OR undefined_table THEN NULL; END $$;

-- suppliers
DO $$ BEGIN
    ALTER TABLE suppliers ADD CONSTRAINT suppliers_credit_limit_cents_nonneg CHECK (credit_limit_cents >= 0);
EXCEPTION WHEN duplicate_object OR undefined_column OR undefined_table THEN NULL; END $$;

-- =====================================================
-- CHECK CONSTRAINT: payment_terms.discount_percent
-- =====================================================

DO $$ BEGIN
    ALTER TABLE payment_terms ADD CONSTRAINT payment_terms_discount_percent_check CHECK (discount_percent IS NULL OR (discount_percent >= 0 AND discount_percent <= 100));
EXCEPTION WHEN duplicate_object OR undefined_column OR undefined_table THEN NULL; END $$;

-- =====================================================
-- UNIQUE CONSTRAINT: tax_rates(tenant_id, tax_type, effective_from)
-- =====================================================

DO $$ BEGIN
    ALTER TABLE tax_rates ADD CONSTRAINT tax_rates_unique_tenant_tax_effective UNIQUE (tenant_id, tax_type, effective_from);
EXCEPTION WHEN duplicate_object OR undefined_column OR undefined_table THEN NULL; END $$;

-- =====================================================
-- CHECK CONSTRAINT: recurring_transactions.intent_type
-- =====================================================

DO $$ BEGIN
    ALTER TABLE recurring_transactions ADD CONSTRAINT recurring_transactions_intent_type_check CHECK (intent_type IN ('CASH_IN', 'CASH_OUT', 'TRANSFER', 'MANUAL_JOURNAL'));
EXCEPTION WHEN duplicate_object OR undefined_column OR undefined_table THEN NULL; END $$;

-- =====================================================
-- MISSING FOREIGN KEYS
-- =====================================================

-- cheques.journal_entry_id REFERENCES journal_entries(id)
DO $$ BEGIN
    ALTER TABLE cheques ADD CONSTRAINT cheques_journal_entry_id_fkey FOREIGN KEY (tenant_id, journal_entry_id) REFERENCES journal_entries(tenant_id, id) ON DELETE SET NULL;
EXCEPTION WHEN duplicate_object OR undefined_column OR undefined_table THEN NULL; END $$;

-- cheques.payment_id REFERENCES invoice_payments(id) - for cheque clearing against invoice
DO $$ BEGIN
    ALTER TABLE cheques ADD CONSTRAINT cheques_payment_id_fkey FOREIGN KEY (payment_id) REFERENCES invoice_payments(id) ON DELETE SET NULL;
EXCEPTION WHEN duplicate_object OR undefined_column OR undefined_table THEN NULL; END $$;

-- cost_centers.parent_id REFERENCES cost_centers(id) - self-referencing for hierarchy
DO $$ BEGIN
    ALTER TABLE cost_centers ADD CONSTRAINT cost_centers_parent_id_fkey FOREIGN KEY (parent_id) REFERENCES cost_centers(id) ON DELETE CASCADE;
EXCEPTION WHEN duplicate_object OR undefined_column OR undefined_table THEN NULL; END $$;
