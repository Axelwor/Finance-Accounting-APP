-- Rollback 000033
DROP TABLE IF EXISTS email_queue;
DROP TABLE IF EXISTS email_templates;
DROP TABLE IF EXISTS budget_variance_reports;
DROP TABLE IF EXISTS cost_center_allocations;
DROP TABLE IF EXISTS cost_centers;
DROP TABLE IF EXISTS cheques;

-- Remove cheque accounts
DELETE FROM accounts WHERE code IN ('1304', '2105');

-- Customer fields
ALTER TABLE customers DROP COLUMN IF EXISTS billing_address;
ALTER TABLE customers DROP COLUMN IF EXISTS shipping_address;
ALTER TABLE customers DROP COLUMN IF EXISTS customer_group;
ALTER TABLE customers DROP COLUMN IF EXISTS price_level;
ALTER TABLE customers DROP COLUMN IF EXISTS currency_code;
ALTER TABLE customers DROP COLUMN IF EXISTS is_pkp;
ALTER TABLE customers DROP COLUMN IF EXISTS credit_hold;
ALTER TABLE customers DROP COLUMN IF EXISTS website;
ALTER TABLE customers DROP COLUMN IF EXISTS fax;
ALTER TABLE customers DROP COLUMN IF EXISTS contact_person_2;
ALTER TABLE customers DROP COLUMN IF EXISTS phone_2;
ALTER TABLE customers DROP COLUMN IF EXISTS npwp_name;
ALTER TABLE customers DROP COLUMN IF EXISTS opening_balance_cents;
ALTER TABLE customers DROP COLUMN IF EXISTS opening_balance_date;

-- Supplier fields
ALTER TABLE suppliers DROP COLUMN IF EXISTS supplier_type;
ALTER TABLE suppliers DROP COLUMN IF EXISTS is_pkp;
ALTER TABLE suppliers DROP COLUMN IF EXISTS currency_code;
ALTER TABLE suppliers DROP COLUMN IF EXISTS bank_name;
ALTER TABLE suppliers DROP COLUMN IF EXISTS bank_account_number;
ALTER TABLE suppliers DROP COLUMN IF EXISTS bank_account_name;
ALTER TABLE suppliers DROP COLUMN IF EXISTS website;
ALTER TABLE suppliers DROP COLUMN IF EXISTS fax;
ALTER TABLE suppliers DROP COLUMN IF EXISTS contact_person_2;
ALTER TABLE suppliers DROP COLUMN IF EXISTS phone_2;
ALTER TABLE suppliers DROP COLUMN IF EXISTS opening_balance_cents;
ALTER TABLE suppliers DROP COLUMN IF EXISTS opening_balance_date;

-- Item fields
ALTER TABLE items DROP COLUMN IF EXISTS barcode;
ALTER TABLE items DROP COLUMN IF EXISTS secondary_uom;
ALTER TABLE items DROP COLUMN IF EXISTS uom_conversion_factor;
ALTER TABLE items DROP COLUMN IF EXISTS brand;
ALTER TABLE items DROP COLUMN IF EXISTS category;
ALTER TABLE items DROP COLUMN IF EXISTS weight_grams;
ALTER TABLE items DROP COLUMN IF EXISTS volume_cc;
ALTER TABLE items DROP COLUMN IF EXISTS description_long;
ALTER TABLE items DROP COLUMN IF EXISTS image_url;
ALTER TABLE items DROP COLUMN IF EXISTS reorder_point;
ALTER TABLE items DROP COLUMN IF EXISTS reorder_qty;
ALTER TABLE items DROP COLUMN IF EXISTS lead_time_days;
ALTER TABLE items DROP COLUMN IF EXISTS preferred_supplier_id;
ALTER TABLE items DROP COLUMN IF EXISTS abc_classification;
ALTER TABLE items DROP COLUMN IF EXISTS sale_uom;
ALTER TABLE items DROP COLUMN IF EXISTS purchase_uom;

-- SO fields
ALTER TABLE sales_orders DROP COLUMN IF EXISTS customer_po_number;
ALTER TABLE sales_orders DROP COLUMN IF EXISTS customer_po_date;
ALTER TABLE sales_orders DROP COLUMN IF EXISTS requested_delivery_date;
ALTER TABLE sales_orders DROP COLUMN IF EXISTS salesperson_id;
ALTER TABLE sales_orders DROP COLUMN IF EXISTS ship_to_address;
ALTER TABLE sales_orders DROP COLUMN IF EXISTS shipping_terms;

-- Invoice fields
ALTER TABLE invoices DROP COLUMN IF EXISTS tax_invoice_number;
ALTER TABLE invoices DROP COLUMN IF EXISTS sub_total_cents;
ALTER TABLE invoices DROP COLUMN IF EXISTS discount_total_cents;
ALTER TABLE invoices DROP COLUMN IF EXISTS tax_total_cents;
ALTER TABLE invoices DROP COLUMN IF EXISTS shipping_fee_cents;
ALTER TABLE invoices DROP COLUMN IF EXISTS other_charges_cents;
ALTER TABLE invoices DROP COLUMN IF EXISTS rounding_cents;
ALTER TABLE invoices DROP COLUMN IF EXISTS salesperson_id;

-- PO fields
ALTER TABLE purchase_orders DROP COLUMN IF EXISTS supplier_quote_number;
ALTER TABLE purchase_orders DROP COLUMN IF EXISTS supplier_quote_date;
ALTER TABLE purchase_orders DROP COLUMN IF EXISTS buyer_id;
