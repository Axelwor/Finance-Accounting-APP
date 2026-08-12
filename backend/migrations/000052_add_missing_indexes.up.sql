-- Add missing indexes for query optimization on frequently filtered columns
-- Phase 5: Database Hardening

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_invoices_customer_status_so 
ON invoices (customer_id, status, sales_order_id);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_sales_orders_customer_status 
ON sales_orders (customer_id, status);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_cheques_status_direction 
ON cheques (status, direction);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_recurring_transactions_next_date_active 
ON recurring_transactions (next_date, is_active);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_inventory_movements_type_warehouse 
ON inventory_movements (movement_type, warehouse_id);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_journal_lines_tenant_entry 
ON journal_lines (tenant_id, entry_id);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_supplier_invoices_supplier 
ON supplier_invoices (supplier_id);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_grn_lines_po_line 
ON grn_lines (po_line_id);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_delivery_orders_lines_item_source 
ON delivery_orders_lines (item_id, source_order_line_id);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_purchase_orders_supplier 
ON purchase_orders (supplier_id);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_approval_requests_entity 
ON approval_requests (entity_id, entity_type);
