-- Down migration: drop P2 Phase 2A tables in dependency order.

DROP TABLE IF EXISTS sales_quotations_lines;
DROP TABLE IF EXISTS sales_quotations;
DROP TABLE IF EXISTS item_price_lists;
DROP TABLE IF EXISTS items;
DROP TABLE IF EXISTS customers;
DROP TABLE IF EXISTS payment_terms;