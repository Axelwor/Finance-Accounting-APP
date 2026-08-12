-- Rollback constraints added in 000053_add_constraints.up.sql

-- =====================================================
-- DROP CHECK CONSTRAINTS: *_cents columns
-- =====================================================

-- invoices
ALTER TABLE invoices DROP CONSTRAINT IF EXISTS invoices_total_cents_nonneg;
ALTER TABLE invoices DROP CONSTRAINT IF EXISTS invoices_dp_applied_cents_nonneg;

-- invoice_lines
ALTER TABLE invoice_lines DROP CONSTRAINT IF EXISTS invoice_lines_unit_price_cents_nonneg;
ALTER TABLE invoice_lines DROP CONSTRAINT IF EXISTS invoice_lines_discount_cents_nonneg;
ALTER TABLE invoice_lines DROP CONSTRAINT IF EXISTS invoice_lines_line_total_cents_nonneg;

-- sales_orders
ALTER TABLE sales_orders DROP CONSTRAINT IF EXISTS sales_orders_total_cents_nonneg;
ALTER TABLE sales_orders DROP CONSTRAINT IF EXISTS sales_orders_dp_received_cents_nonneg;

-- sales_orders_lines
ALTER TABLE sales_orders_lines DROP CONSTRAINT IF EXISTS sales_orders_lines_unit_price_cents_nonneg;
ALTER TABLE sales_orders_lines DROP CONSTRAINT IF EXISTS sales_orders_lines_discount_cents_nonneg;
ALTER TABLE sales_orders_lines DROP CONSTRAINT IF EXISTS sales_orders_lines_line_total_cents_nonneg;

-- delivery_orders
ALTER TABLE delivery_orders DROP CONSTRAINT IF EXISTS delivery_orders_total_cents_nonneg;
ALTER TABLE delivery_orders DROP CONSTRAINT IF EXISTS delivery_orders_gross_profit_cents_nonneg;

-- delivery_orders_lines
ALTER TABLE delivery_orders_lines DROP CONSTRAINT IF EXISTS delivery_orders_lines_unit_price_cents_nonneg;
ALTER TABLE delivery_orders_lines DROP CONSTRAINT IF EXISTS delivery_orders_lines_discount_cents_nonneg;
ALTER TABLE delivery_orders_lines DROP CONSTRAINT IF EXISTS delivery_orders_lines_line_total_cents_nonneg;
ALTER TABLE delivery_orders_lines DROP CONSTRAINT IF EXISTS delivery_orders_lines_cogs_cents_nonneg;

-- sales_quotations
ALTER TABLE sales_quotations DROP CONSTRAINT IF EXISTS sales_quotations_total_cents_nonneg;

-- sales_quotations_lines
ALTER TABLE sales_quotations_lines DROP CONSTRAINT IF EXISTS sales_quotations_lines_unit_price_cents_nonneg;
ALTER TABLE sales_quotations_lines DROP CONSTRAINT IF EXISTS sales_quotations_lines_discount_cents_nonneg;
ALTER TABLE sales_quotations_lines DROP CONSTRAINT IF EXISTS sales_quotations_lines_line_total_cents_nonneg;

-- purchase_orders
ALTER TABLE purchase_orders DROP CONSTRAINT IF EXISTS purchase_orders_total_cents_nonneg;
ALTER TABLE purchase_orders DROP CONSTRAINT IF EXISTS purchase_orders_received_cents_nonneg;

-- purchase_orders_lines
ALTER TABLE purchase_orders_lines DROP CONSTRAINT IF EXISTS purchase_orders_lines_unit_price_cents_nonneg;
ALTER TABLE purchase_orders_lines DROP CONSTRAINT IF EXISTS purchase_orders_lines_discount_cents_nonneg;
ALTER TABLE purchase_orders_lines DROP CONSTRAINT IF EXISTS purchase_orders_lines_line_total_cents_nonneg;

-- goods_received_notes
ALTER TABLE goods_received_notes DROP CONSTRAINT IF EXISTS goods_received_notes_total_cents_nonneg;

-- grn_lines
ALTER TABLE grn_lines DROP CONSTRAINT IF EXISTS grn_lines_unit_cost_cents_nonneg;
ALTER TABLE grn_lines DROP CONSTRAINT IF EXISTS grn_lines_amount_cents_nonneg;

-- supplier_invoices
ALTER TABLE supplier_invoices DROP CONSTRAINT IF EXISTS supplier_invoices_dpp_cents_nonneg;
ALTER TABLE supplier_invoices DROP CONSTRAINT IF EXISTS supplier_invoices_vat_cents_nonneg;
ALTER TABLE supplier_invoices DROP CONSTRAINT IF EXISTS supplier_invoices_total_cents_nonneg;
ALTER TABLE supplier_invoices DROP CONSTRAINT IF EXISTS supplier_invoices_dp_applied_cents_nonneg;
ALTER TABLE supplier_invoices DROP CONSTRAINT IF EXISTS supplier_invoices_payable_cents_nonneg;

-- supplier_invoice_lines
ALTER TABLE supplier_invoice_lines DROP CONSTRAINT IF EXISTS supplier_invoice_lines_unit_price_cents_nonneg;
ALTER TABLE supplier_invoice_lines DROP CONSTRAINT IF EXISTS supplier_invoice_lines_discount_cents_nonneg;
ALTER TABLE supplier_invoice_lines DROP CONSTRAINT IF EXISTS supplier_invoice_lines_line_total_cents_nonneg;

-- cheque_payments
ALTER TABLE cheque_payments DROP CONSTRAINT IF EXISTS cheque_payments_amount_cents_nonneg;

-- cheque_transfers
ALTER TABLE cheque_transfers DROP CONSTRAINT IF EXISTS cheque_transfers_amount_cents_nonneg;

-- petty_cash_funds
ALTER TABLE petty_cash_funds DROP CONSTRAINT IF EXISTS petty_cash_funds_balance_cents_nonneg;

-- petty_cash_vouchers
ALTER TABLE petty_cash_vouchers DROP CONSTRAINT IF EXISTS petty_cash_vouchers_amount_cents_nonneg;

-- cash_entries
ALTER TABLE cash_entries DROP CONSTRAINT IF EXISTS cash_entries_amount_cents_nonneg;

-- journal_adjustments
ALTER TABLE journal_adjustments DROP CONSTRAINT IF EXISTS journal_adjustments_amount_cents_nonneg;

-- pph_calculations
ALTER TABLE pph_calculations DROP CONSTRAINT IF EXISTS pph_calculations_dpp_cents_nonneg;
ALTER TABLE pph_calculations DROP CONSTRAINT IF EXISTS pph_calculations_pph_cents_nonneg;

-- inventory_movements
ALTER TABLE inventory_movements DROP CONSTRAINT IF EXISTS inventory_movements_qty_cents_nonneg;

-- stock_opnames
ALTER TABLE stock_opnames DROP CONSTRAINT IF EXISTS stock_opnames_total_adjustment_cents_nonneg;

-- stock_opname_lines
ALTER TABLE stock_opname_lines DROP CONSTRAINT IF EXISTS stock_opname_lines_adjustment_cents_nonneg;
ALTER TABLE stock_opname_lines DROP CONSTRAINT IF EXISTS stock_opname_lines_unit_cost_cents_nonneg;

-- stock_transfers
ALTER TABLE stock_transfers DROP CONSTRAINT IF EXISTS stock_transfers_total_cents_nonneg;

-- budget_allocations
ALTER TABLE budget_allocations DROP CONSTRAINT IF EXISTS budget_allocations_amount_cents_nonneg;

-- production_jobs
ALTER TABLE production_jobs DROP CONSTRAINT IF EXISTS production_jobs_target_cost_cents_nonneg;

-- production_job_costs
ALTER TABLE production_job_costs DROP CONSTRAINT IF EXISTS production_job_costs_amount_cents_nonneg;

-- fixed_assets
ALTER TABLE fixed_assets DROP CONSTRAINT IF EXISTS fixed_assets_acquisition_cost_cents_nonneg;
ALTER TABLE fixed_assets DROP CONSTRAINT IF EXISTS fixed_assets_accumulated_depreciation_cents_nonneg;

-- lease_contracts
ALTER TABLE lease_contracts DROP CONSTRAINT IF EXISTS lease_contracts_monthly_rent_cents_nonneg;

-- recurring_transactions
ALTER TABLE recurring_transactions DROP CONSTRAINT IF EXISTS recurring_transactions_amount_cents_nonneg;

-- items
ALTER TABLE items DROP CONSTRAINT IF EXISTS items_purchase_price_cents_nonneg;
ALTER TABLE items DROP CONSTRAINT IF EXISTS items_sale_price_cents_nonneg;

-- customers
ALTER TABLE customers DROP CONSTRAINT IF EXISTS customers_credit_limit_cents_nonneg;

-- suppliers
ALTER TABLE suppliers DROP CONSTRAINT IF EXISTS suppliers_credit_limit_cents_nonneg;

-- =====================================================
-- DROP CONSTRAINT: payment_terms.discount_percent
-- =====================================================

ALTER TABLE payment_terms DROP CONSTRAINT IF EXISTS payment_terms_discount_percent_check;

-- =====================================================
-- DROP UNIQUE: tax_rates(tenant_id, tax_type, effective_from)
-- =====================================================

ALTER TABLE tax_rates DROP CONSTRAINT IF EXISTS tax_rates_unique_tenant_tax_effective;

-- =====================================================
-- DROP CHECK: recurring_transactions.intent_type
-- =====================================================

ALTER TABLE recurring_transactions DROP CONSTRAINT IF EXISTS recurring_transactions_intent_type_check;

-- =====================================================
-- DROP FOREIGN KEYS
-- =====================================================

-- cheques.journal_entry_id
ALTER TABLE cheques DROP CONSTRAINT IF EXISTS cheques_journal_entry_id_fkey;

-- cheques.payment_id
ALTER TABLE cheques DROP CONSTRAINT IF EXISTS cheques_payment_id_fkey;

-- cost_centers.parent_id
ALTER TABLE cost_centers DROP CONSTRAINT IF EXISTS cost_centers_parent_id_fkey;
