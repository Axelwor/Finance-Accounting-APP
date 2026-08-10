-- Migration 000038: Seed default report templates (global tenant_id = 0)
-- This migration inserts all 19 document_type templates into report_templates
-- with tenant_id = 0 and is_default = true. Use on fresh DBs or when templates
-- need re-seeding after rollback. On multi-tenant systems, tenant 0 templates
-- serve as defaults visible to all tenants via the UI template picker.
-- Note: Must run after tenants table exists; tenant 0 assumed pre-seeded by
-- deployment scripts or manual setup. If needed, add: 
--   INSERT INTO tenants (id, name, slug, currency_code) VALUES (0, 'Global Defaults', 'global-defaults', 'USD');
-- with appropriate primary key sequence adjustments per environment.

-- Ensure tenant 0 exists (idempotent up to PK constraints). If your deploy
-- seeds tenant ids differently, comment out this block and adjust references.
INSERT INTO tenants (id, name, slug, currency_code)
VALUES (0, 'Global Defaults', 'global-defaults', 'USD')
ON CONFLICT (id) DO NOTHING;

ALTER SEQUENCE tenants_id_seq RESTART WITH 1000;

INSERT INTO report_templates (tenant_id, code, name, document_type, template_yaml, is_default, is_active)
VALUES (0, 'INV-STD', 'Standard Invoice', 'invoice',
        $$title: Invoice
subtitle: Tax Invoice / Sales Invoice
header_fields:
  - number: Invoice Number
  - date: Invoice Date
  - due_date: Due Date
  - customer_name: Customer
  - customer_address: Address
  - currency: Currency
sections:
  - title: Line Items
    table: rows
    columns:
      - code: Item Code
      - description: Description
      - qty: Quantity
      - unit: Unit
      - unit_price: Unit Price
      - amount: Amount
  - title: Totals
    table: totals
    columns:
      - label: Description
      - value: Amount
footer_text: Please remit payment by the due date. Thank you for your business.$$),
       (0, 'PO-STD', 'Standard Purchase Order', 'purchase_order',
        $$title: Purchase Order
subtitle: Supplier Purchase Order
header_fields:
  - number: PO Number
  - date: PO Date
  - expected_date: Expected Delivery
  - supplier_name: Supplier
  - supplier_address: Address
  - currency: Currency
sections:
  - title: Order Lines
    table: rows
    columns:
      - code: Item Code
      - description: Description
      - qty: Quantity
      - unit: Unit
      - unit_price: Unit Price
      - amount: Amount
  - title: Totals
    table: totals
    columns:
      - label: Description
      - value: Amount
footer_text: Please confirm this purchase order and quote the PO number on delivery.$$),
       (0, 'DO-STD', 'Delivery Order', 'delivery_order',
        $$title: Delivery Order
subtitle: Goods Delivery / Shipping Document
header_fields:
  - number: Delivery Number
  - date: Delivery Date
  - reference: Reference
  - customer_name: Deliver To
  - customer_address: Address
sections:
  - title: Delivered Items
    table: rows
    columns:
      - code: Item Code
      - description: Description
      - qty: Quantity
      - unit: Unit
      - notes: Notes
  - title: Acknowledgement
    content: "Received by (name / signature / date):"
footer_text: Please sign and return a copy of this delivery order as proof of receipt.$$),
       (0, 'TAX-STD', 'Tax Invoice', 'tax_invoice',
        $$title: Tax Invoice
subtitle: VAT / GST Tax Invoice
header_fields:
  - number: Tax Invoice Number
  - date: Invoice Date
  - tax_id: Seller Tax ID
  - customer_name: Buyer
  - customer_tax_id: Buyer Tax ID
  - currency: Currency
sections:
  - title: Taxable Items
    table: rows
    columns:
      - code: Item Code
      - description: Description
      - qty: Quantity
      - unit_price: Unit Price
      - amount: Amount
      - tax_rate: Tax Rate
      - tax_amount: Tax Amount
  - title: Tax Summary
    table: totals
    columns:
      - label: Description
      - value: Amount
footer_text: This document is an official tax invoice. Retain for tax filing purposes.$$),
       (0, 'WH-STD', 'Withholding Tax Slip', 'withholding_slip',
        $$title: Withholding Tax Slip
subtitle: Withholding Tax Certificate
header_fields:
  - number: Slip Number
  - date: Payment Date
  - payee_name: Payee
  - payee_tax_id: Payee Tax ID
  - tax_rate: Withholding Rate
  - currency: Currency
sections:
  - title: Withheld Amounts
    table: rows
    columns:
      - reference: Reference
      - description: Description
      - gross_amount: Gross Amount
      - tax_amount: Tax Withheld
      - net_amount: Net Amount
  - title: Summary
    table: totals
    columns:
      - label: Description
      - value: Amount
footer_text: This certificate evidences tax withheld at source. Retain for tax filing purposes.$$),
       (0, 'CUS-STD', 'Customer Statement', 'customer_statement',
        $$title: Customer Statement
subtitle: Statement of Account — Customer
header_fields:
  - number: Statement Number
  - date: Statement Date
  - period_start: Period Start
  - period_end: Period End
  - customer_name: Customer
  - currency: Currency
sections:
  - title: Account Activity
    table: rows
    columns:
      - date: Date
      - reference: Reference
      - description: Description
      - debit: Debit
      - credit: Credit
      - balance: Balance
  - title: Summary
    table: totals
    columns:
      - label: Description
      - value: Amount
footer_text: Please contact us if you have questions about any item on this statement.$$),
       (0, 'SUP-STD', 'Supplier Statement', 'supplier_statement',
        $$title: Supplier Statement
subtitle: Statement of Account — Supplier
header_fields:
  - number: Statement Number
  - date: Statement Date
  - period_start: Period Start
  - period_end: Period End
  - supplier_name: Supplier
  - currency: Currency
sections:
  - title: Account Activity
    table: rows
    columns:
      - date: Date
      - reference: Reference
      - description: Description
      - debit: Debit
      - credit: Credit
      - balance: Balance
  - title: Summary
    table: totals
    columns:
      - label: Description
      - value: Amount
footer_text: Please reconcile this statement against your records and report discrepancies.$$),
       (0, 'PV-STD', 'Payment Voucher', 'payment_voucher',
        $$title: Payment Voucher
subtitle: Outgoing Payment Voucher
header_fields:
  - number: Voucher Number
  - date: Payment Date
  - payee_name: Payee
  - payment_method: Payment Method
  - bank_account: Bank Account
  - currency: Currency
sections:
  - title: Payment Details
    table: rows
    columns:
      - reference: Reference
      - description: Description
      - amount: Amount
  - title: Totals
    table: totals
    columns:
      - label: Description
      - value: Amount
  - title: Approval
    content: "Prepared by: ______________   Checked by: ______________   Approved by: ______________"
footer_text: This voucher authorizes the disbursement described above.$$),
       (0, 'RV-STD', 'Receipt Voucher', 'receipt_voucher',
        $$title: Receipt Voucher
subtitle: Incoming Payment Receipt
header_fields:
  - number: Receipt Number
  - date: Receipt Date
  - payer_name: Received From
  - payment_method: Payment Method
  - bank_account: Bank Account
  - currency: Currency
sections:
  - title: Receipt Details
    table: rows
    columns:
      - reference: Reference
      - description: Description
      - amount: Amount
  - title: Totals
    table: totals
    columns:
      - label: Description
      - value: Amount
footer_text: This receipt acknowledges payment received as described above.$$),
       (0, 'JV-STD', 'Journal Voucher', 'journal_voucher',
        $$title: Journal Voucher
subtitle: General Journal Entry Voucher
header_fields:
  - number: Journal Number
  - date: Journal Date
  - reference: Reference
  - description: Memo
  - created_by: Prepared By
sections:
  - title: Journal Lines
    table: rows
    columns:
      - account_code: Account Code
      - account_name: Account Name
      - description: Description
      - debit: Debit
      - credit: Credit
  - title: Totals
    table: totals
    columns:
      - label: Description
      - value: Amount
  - title: Approval
    content: "Prepared by: ______________   Reviewed by: ______________   Posted by: ______________"
footer_text: Journal entries are immutable once posted; corrections use reversal entries.$$),
       (0, 'SC-STD', 'Stock Card', 'stock_card',
        $$title: Stock Card
subtitle: Item Movement History
header_fields:
  - item_code: Item Code
  - item_name: Item Name
  - warehouse: Warehouse
  - location: Location
  - unit: Unit
  - opening_qty: Opening Balance
sections:
  - title: Movements
    table: rows
    columns:
      - date: Date
      - reference: Reference
      - description: Description
      - in_qty: In
      - out_qty: Out
      - balance_qty: Balance
  - title: Summary
    table: totals
    columns:
      - label: Description
      - value: Amount
footer_text: Stock card quantities are derived from posted inventory movements.$$),
       (0, 'TB-STD', 'Trial Balance', 'trial_balance',
        $$title: Trial Balance
subtitle: General Ledger Trial Balance
header_fields:
  - period_name: Period
  - as_of_date: As Of Date
  - company_name: Company
  - currency: Currency
sections:
  - title: Account Balances
    table: rows
    columns:
      - account_code: Account Code
      - account_name: Account Name
      - account_type: Type
      - debit: Debit
      - credit: Credit
  - title: Totals
    table: totals
    columns:
      - label: Description
      - value: Amount
footer_text: Total debits must equal total credits for a balanced trial balance.$$),
       (0, 'PL-STD', 'Profit and Loss', 'profit_loss',
        $$title: Profit and Loss
subtitle: Income Statement
header_fields:
  - period_name: Period
  - company_name: Company
  - currency: Currency
sections:
  - title: Income
    table: rows
    columns:
      - account_code: Account Code
      - account_name: Account
      - amount: Amount
  - title: Expenses
    table: expense_rows
    columns:
      - account_code: Account Code
      - account_name: Account
      - amount: Amount
  - title: Result
    table: totals
    columns:
      - label: Description
      - value: Amount
footer_text: Figures reflect posted journals for the selected period only.$$),
       (0, 'BS-STD', 'Balance Sheet', 'balance_sheet',
        $$title: Balance Sheet
subtitle: Statement of Financial Position
header_fields:
  - as_of_date: As Of Date
  - company_name: Company
  - currency: Currency
sections:
  - title: Assets
    table: asset_rows
    columns:
      - account_code: Account Code
      - account_name: Account
      - amount: Amount
  - title: Liabilities
    table: liability_rows
    columns:
      - account_code: Account Code
      - account_name: Account
      - amount: Amount
  - title: Equity
    table: equity_rows
    columns:
      - account_code: Account Code
      - account_name: Account
      - amount: Amount
  - title: Totals
    table: totals
    columns:
      - label: Description
      - value: Amount
footer_text: Assets must equal liabilities plus equity.$$),
       (0, 'CF-STD', 'Cash Flow Statement', 'cash_flow',
        $$title: Cash Flow Statement
subtitle: Statement of Cash Flows
header_fields:
  - period_name: Period
  - company_name: Company
  - currency: Currency
sections:
  - title: Operating Activities
    table: operating_rows
    columns:
      - description: Description
      - amount: Amount
  - title: Investing Activities
    table: investing_rows
    columns:
      - description: Description
      - amount: Amount
  - title: Financing Activities
    table: financing_rows
    columns:
      - description: Description
      - amount: Amount
  - title: Summary
    table: totals
    columns:
      - label: Description
      - value: Amount
footer_text: Prepared using the indirect method from posted accounting entries.$$),
       (0, 'AR-STD', 'Accounts Receivable Aging', 'ar_aging',
        $$title: Accounts Receivable Aging
subtitle: AR Aging Summary
header_fields:
  - as_of_date: As Of Date
  - company_name: Company
  - currency: Currency
sections:
  - title: Aging by Customer
    table: rows
    columns:
      - customer_name: Customer
      - current: Current
      - days_30: 1-30 Days
      - days_60: 31-60 Days
      - days_90: 61-90 Days
      - over_90: Over 90 Days
      - total: Total
  - title: Totals
    table: totals
    columns:
      - label: Description
      - value: Amount
footer_text: Aging buckets are calculated from invoice due dates as of the report date.$$),
       (0, 'AP-STD', 'Accounts Payable Aging', 'ap_aging',
        $$title: Accounts Payable Aging
subtitle: AP Aging Summary
header_fields:
  - as_of_date: As Of Date
  - company_name: Company
  - currency: Currency
sections:
  - title: Aging by Supplier
    table: rows
    columns:
      - supplier_name: Supplier
      - current: Current
      - days_30: 1-30 Days
      - days_60: 31-60 Days
      - days_90: 61-90 Days
      - over_90: Over 90 Days
      - total: Total
  - title: Totals
    table: totals
    columns:
      - label: Description
      - value: Amount
footer_text: Aging buckets are calculated from bill due dates as of the report date.$$),
       (0, 'ASR-STD', 'Asset Register', 'asset_register',
        $$title: Asset Register
subtitle: Fixed Asset Register
header_fields:
  - as_of_date: As Of Date
  - company_name: Company
  - currency: Currency
sections:
  - title: Assets
    table: rows
    columns:
      - asset_code: Asset Code
      - asset_name: Asset Name
      - category: Category
      - acquired_date: Acquired
      - cost: Cost
      - accumulated_depreciation: Accum. Depreciation
      - net_book_value: Net Book Value
  - title: Totals
    table: totals
    columns:
      - label: Description
      - value: Amount
footer_text: Net book value equals cost less accumulated depreciation to date.$$),
       (0, 'STK-STD', 'Stock Opname', 'stock_opname',
        $$title: Stock Opname
subtitle: Physical Stock Count Sheet
header_fields:
  - number: Opname Number
  - date: Count Date
  - warehouse: Warehouse
  - counted_by: Counted By
sections:
  - title: Count Lines
    table: rows
    columns:
      - item_code: Item Code
      - item_name: Item Name
      - location: Location
      - system_qty: System Qty
      - counted_qty: Counted Qty
      - variance_qty: Variance
      - notes: Notes
  - title: Summary
    table: totals
    columns:
      - label: Description
      - value: Amount
  - title: Approval
    content: "Counted by: ______________   Verified by: ______________   Approved by: ______________"
footer_text: Post adjustment journals for variances only after approval.$$);
