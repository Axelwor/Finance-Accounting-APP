-- Drop indexes added in 000052_add_missing_indexes.up.sql

DROP INDEX CONCURRENTLY IF EXISTS idx_invoices_customer_status_so;
DROP INDEX CONCURRENTLY IF EXISTS idx_sales_orders_customer_status;
DROP INDEX CONCURRENTLY IF EXISTS idx_cheques_status_direction;
DROP INDEX CONCURRENTLY IF EXISTS idx_recurring_transactions_next_date_active;
DROP INDEX CONCURRENTLY IF EXISTS idx_inventory_movements_type_warehouse;
DROP INDEX CONCURRENTLY IF EXISTS idx_journal_lines_tenant_entry;
DROP INDEX CONCURRENTLY IF EXISTS idx_supplier_invoices_supplier;
DROP INDEX CONCURRENTLY IF EXISTS idx_grn_lines_po_line;
DROP INDEX CONCURRENTLY IF EXISTS idx_delivery_orders_lines_item_source;
DROP INDEX CONCURRENTLY IF EXISTS idx_purchase_orders_supplier;
DROP INDEX CONCURRENTLY IF EXISTS idx_approval_requests_entity;
