/**
 * Shared data types for the whole web app.
 *
 * This is the temporary frontend contract (M1). Once the backend is fully
 * available, these types will be aligned with the Go API contract
 * (see ARCHITECTURE.md, shared types).
 */

/** User-friendly label for each record kind (no debit/credit terms). */
export type TransactionKind = "money-in" | "money-out" | "transfer";

/** Workbench list sub-kind identifiers (drives the sidebar + tab dispatch). */
export type ListSubKind =
  | "cash-other-receipt"
  | "cash-other-payment"
  | "cash-transfer"
  | "sales-invoice"
  | "sales-receipt"
  | "sales-quotation"
  | "sales-order"
  | "delivery-order"
  | "credit-note"
  | "purchase-order"
  | "grn"
  | "purchase-supplier"
  | "purchase-invoice"
  | "supplier-invoice"
  | "purchase-payment"
  | "purchase-return"
  | "bank-reconciliation"
  | "inventory-items"
  | "stock-movements"
  | "stock-opname"
  | "stock-transfer"
  | "asset-register"
  | "journal-entry"
  | "general-ledger"
  | "journal-register"
  | "report-trial-balance"
  | "report-profit-loss"
  | "report-balance-sheet"
  | "report-cash-flow"
  | "financial-notes"
  | "due-date-reminders"
  | "bom"
  | "production-job";

/** Workbench entry sub-kind identifiers (drive the entry tab dispatch). */
export type EntrySubKind =
  | "money-in"
  | "money-out"
  | "cash-transfer"
  | "bank-reconciliation-entry"
  | "sales-invoice"
  | "sales-receipt"
  | "sales-quotation-entry"
  | "sales-order-entry"
  | "delivery-order-entry"
  | "credit-note-entry"
  | "purchase-order-entry"
  | "grn-entry"
  | "purchase-supplier-entry"
  | "purchase-invoice"
  | "supplier-invoice-entry"
  | "purchase-payment"
  | "purchase-return-entry"
  | "inventory-item"
  | "stock-opname-entry"
  | "stock-transfer-entry"
  | "asset-register"
  | "journal-entry"
  | "financial-notes-entry"
  | "bom-entry"
  | "production-job-entry";

export type CurrencyCode = "IDR";

export interface Business {
  id: string;
  name: string;
  businessType: string;
  currency: CurrencyCode;
  /** Month (1-12) when the fiscal year starts. */
  fiscalYearStart: number;
}

/** Accounting book period. */
export interface BookPeriod {
  /** Fiscal year, e.g. 2026. */
  year: number;
  /** Month (1-12) when the period starts. */
  startMonth: number;
}

export interface OpeningBalance {
  cash: number;
  bank: number;
  receivables: number;
  payables: number;
  equity: number;
}

export interface User {
  id: string;
  email: string;
  businessName: string;
}

export interface Category {
  id: string;
  name: string;
  /** Only relevant for Money In / Money Out. */
  kind: "money-in" | "money-out";
}

export interface Transaction {
  id: string;
  kind: TransactionKind;
  amount: number;
  date: string;
  description: string;
  categoryId?: string;
  categoryName?: string;
  /** Source of funds, e.g. "Cash" or a bank account name. */
  from?: string;
  /** Destination of funds, e.g. "Cash" or a bank account name. */
  to?: string;
  createdAt: string;
}

export interface DashboardSummary {
  cashAndBankBalance: number;
  monthlyProfitLoss: number;
  dueBills: number;
  lowStock: number;
  recentTransactions: Transaction[];
}

export interface ApiError {
  code: string;
  message: string;
}

export interface RegisterInput {
  email: string;
  password: string;
  businessName: string;
}

export interface LoginInput {
  email: string;
  password: string;
}

export interface OnboardingInput {
  business: {
    name: string;
    businessType: string;
    currency: CurrencyCode;
  };
  period: BookPeriod;
  openingBalance: OpeningBalance;
}

export interface TransactionInput {
  kind: TransactionKind;
  amount: number;
  date: string;
  description: string;
  categoryId?: string;
  from?: string;
  to?: string;
}

/** List of accounts/cash for the Transfer form dropdown. */
export interface AccountItem {
  id: string;
  name: string;
}

/* ------------------------------------------------------------------ */
/* Backend contract types (Go JSON at /api/v1) — mapped to UI types.  */
/* ------------------------------------------------------------------ */

/** Account row from GET /api/v1/accounts (coa.account struct). */
export interface BackendAccount {
  id: number;
  code: string;
  name: string;
  report_group: string;
  account_type: string;
  parent_id: number | null;
  is_group: boolean;
  is_active: boolean;
  valid_from: string | null;
  valid_to: string | null;
}

/** Category row from GET /api/v1/categories (coa.category struct). */
export interface BackendCategory {
  id: number;
  name: string;
  direction: "IN" | "OUT";
  default_debit_account_id: number | null;
  default_credit_account_id: number | null;
  is_active: boolean;
}

/** Response from POST /api/v1/tenants. */
export interface BackendTenant {
  id: number;
  name: string;
  slug: string;
}

/** A single counter line for multi-line cash commands. */
export interface CounterLinePayload {
  account_id: number;
  amount_cents: number;
  description: string;
}

/** Common financial command payload (POST /cash-in, /cash-out). */
export interface CashCommandPayload {
  source_ref: string;
  entry_date: string;
  cash_account_id: number;
  counter_account_id: number;
  amount_cents: number;
  description: string;
  /** Optional multi-counter: when provided, replaces counter_account_id. */
  counter_lines?: CounterLinePayload[];
}

/** Payload POST /api/v1/transfers. */
export interface TransferCommandPayload {
  source_ref: string;
  entry_date: string;
  from_account_id: number;
  to_account_id: number;
  amount_cents: number;
  description: string;
}

/** Journal posting response (cash.postingResult). */
export interface BackendJournalResult {
  id: number;
  number: string;
  status: string;
  hash: string;
  prev_hash: string;
  intent_type: string;
  is_reversal: boolean;
}

/** Response GET /api/v1/reports/profit-loss. */
export interface BackendProfitLoss {
  revenue_cents: number;
  expense_cents: number;
  profit_cents: number;
}

/** Response GET /api/v1/reports/balance-sheet. */
export interface BackendBalanceSheet {
  asset_cents: number;
  liability_cents: number;
  equity_cents: number;
  balanced: boolean;
}

/** Response GET /api/v1/reports/cash-flow. */
export interface BackendCashFlow {
  inflow_cents: number;
  outflow_cents: number;
  net_cash_flow_cents: number;
}

/** Single row in the trial balance response. */
export interface BackendTrialBalanceRow {
  account_id: number;
  account_code: string;
  account_name: string;
  report_group: string;
  debit_cents: number;
  credit_cents: number;
}

/** Response GET /api/v1/reports/trial-balance. */
export interface BackendTrialBalance {
  rows: BackendTrialBalanceRow[];
  total_debit_cents: number;
  total_credit_cents: number;
  balanced: boolean;
}

/** Opening balance line for POST /api/v1/opening-balances. */
export interface OpeningBalanceLine {
  account_id: number;
  debit_cents: number;
  credit_cents: number;
}

/** Cash & bank history list item returned by GET /api/v1/cash-entries. */
export interface CashEntryListItem {
  id: number;
  number: string;
  kind: "money-in" | "money-out" | "transfer";
  entry_date: string;
  status: string;
  description: string;
  amount_cents: number;
  cash_account_id: number;
  cash_account_code: string;
  cash_account_name: string;
  counter_account_id: number;
  counter_account_code: string;
  counter_account_name: string;
  from_account_id: number;
  from_account_code: string;
  from_account_name: string;
  to_account_id: number;
  to_account_code: string;
  to_account_name: string;
  reference: string;
  reversal_of_id: number;
}

export interface CashEntryListResponse {
  items: CashEntryListItem[];
  limit: number;
  offset: number;
  count: number;
}

export interface ListCashEntriesParams {
  kind?: "money-in" | "money-out" | "transfer";
  from?: string;
  to?: string;
  account_id?: number;
  q?: string;
  limit?: number;
  offset?: number;
}

/** Payload POST /api/v1/opening-balances. */
export interface OpeningBalancePayload {
  source_ref: string;
  entry_date: string;
  equity_account_id: number;
  balances: OpeningBalanceLine[];
  description: string;
}

/** Result of period close/open commands (POST /api/v1/periods/close|unlock). */
export interface PeriodResult {
  period_id: number;
  status: string;
  journal_id: number;
  number: string;
  hash?: string;
}

/** Customer master data (GET/POST /api/v1/customers). */
export interface Customer {
  id: number;
  code: string;
  name: string;
  payment_term_id?: number | null;
  is_active: boolean;
}

/** Item master data (GET /api/v1/items). */
export interface Item {
  id: number;
  code: string;
  name: string;
  item_type: "goods" | "service";
  is_tracked_stock: boolean;
}

/** Sales quotation list row (GET /api/v1/quotations). */
export interface QuotationListItem {
  id: number;
  number: string;
  customer_id: number;
  customer_name?: string;
  quotation_date: string;
  valid_until?: string;
  payment_term_id?: number | null;
  status: "DRAFT" | "SENT" | "CONVERTED" | "EXPIRED" | "CANCELLED";
  total_cents: number;
}

/** A quotation line as returned by GET /api/v1/quotations/{id}. */
export interface QuotationLine {
  id: number;
  item_id?: number | null;
  item_code?: string;
  item_name?: string;
  qty: string;
  unit_price_cents: number;
  discount_cents: number;
  tax_rate: string;
  line_total_cents: number;
  description?: string | null;
}

/** Full quotation with lines (GET /api/v1/quotations/{id}). */
export interface Quotation extends QuotationListItem {
  lines: QuotationLine[];
}

/** Input line for POST /api/v1/quotations. */
export interface QuotationLineInput {
  item_id: number;
  qty: number;
  unit_price_cents: number;
  discount_cents: number;
  tax_rate: number;
  description?: string;
}

/** Payload POST /api/v1/quotations. */
export interface QuotationCreateInput {
  customer_id: number;
  quotation_date: string;
  valid_until?: string;
  payment_term_id?: number;
  notes?: string;
  source_ref?: string;
  lines: QuotationLineInput[];
}

/* ------------------------------------------------------------------ */
/* Sales Order (SO) + Down Payment (DP)                                */
/* ------------------------------------------------------------------ */

/** Sales order list row (GET /api/v1/sales-orders). */
export interface SalesOrderListItem {
  id: number;
  number: string;
  quotation_id?: number;
  customer_id: number;
  customer_name?: string;
  order_date: string;
  payment_term_id?: number;
  notes?: string;
  status: "CONFIRMED" | "CLOSED" | "CANCELLED";
  total_cents: number;
  dp_received_cents: number;
}

/** A sales order line (GET /api/v1/sales-orders/{id}). */
export interface SalesOrderLine {
  id: number;
  item_id: number;
  item_code?: string;
  item_name?: string;
  line_no: number;
  qty: string;
  unit_price_cents: number;
  discount_cents: number;
  tax_rate: string;
  line_total_cents: number;
  description?: string;
}

/** Down payment attached to an order (GET /api/v1/sales-orders/{id}). */
export interface DownPayment {
  id: number;
  number: string;
  order_id: number;
  journal_entry_id?: number;
  amount_cents: number;
  cash_account_id: number;
  dp_date: string;
  description?: string;
  status: "RECEIVED" | "REFUNDED";
}

/** Full sales order with lines and down payments. */
export interface SalesOrder extends SalesOrderListItem {
  lines: SalesOrderLine[];
  down_payments: DownPayment[];
}

/** Input line for POST /api/v1/sales-orders. */
export interface SalesOrderLineInput {
  item_id: number;
  qty: number;
  unit_price_cents: number;
  discount_cents: number;
  tax_rate: number;
  description?: string;
}

/** Payload POST /api/v1/sales-orders. */
export interface SalesOrderCreateInput {
  customer_id: number;
  quotation_id?: number;
  order_date: string;
  payment_term_id?: number;
  notes?: string;
  lines: SalesOrderLineInput[];
}

/** Payload POST /api/v1/sales-orders/{id}/down-payments. */
export interface CreateDownPaymentInput {
  cash_account_id: number;
  amount_cents: number;
  dp_date: string;
  description?: string;
}

/* ------------------------------------------------------------------ */
/* Delivery Order (DO)                                                 */
/* ------------------------------------------------------------------ */

/** Delivery order list row (GET /api/v1/delivery-orders). */
export interface DeliveryOrderListItem {
  id: number;
  number: string;
  sales_order_id: number;
  customer_id: number;
  customer_name?: string;
  delivery_date: string;
  notes?: string;
  status: "SHIPPED" | "RETURNED" | "CANCELLED";
  journal_entry_id?: number;
  total_cogs_cents: number;
}

/** A delivery order line (GET /api/v1/delivery-orders/{id}). */
export interface DeliveryOrderLine {
  id: number;
  item_id: number;
  item_code?: string;
  item_name?: string;
  line_no: number;
  qty: string;
  unit_cost_cents: number;
  cogs_cents: number;
  inventory_account_id: number;
  cogs_account_id: number;
  description?: string;
}

/** Full delivery order with lines. */
export interface DeliveryOrder extends DeliveryOrderListItem {
  lines: DeliveryOrderLine[];
}

/** Input line for POST /api/v1/delivery-orders. */
export interface DeliveryLineInput {
  item_id: number;
  qty: number;
  unit_cost_cents: number;
  description?: string;
}

/** Payload POST /api/v1/delivery-orders. */
export interface CreateDeliveryInput {
  sales_order_id: number;
  delivery_date: string;
  notes?: string;
  lines: DeliveryLineInput[];
}

/* ------------------------------------------------------------------ */
/* Invoice (INV)                                                       */
/* ------------------------------------------------------------------ */

/** Invoice list row (GET /api/v1/invoices). */
export interface InvoiceListItem {
  id: number;
  number: string;
  sales_order_id?: number;
  customer_id: number;
  customer_name?: string;
  invoice_date: string;
  due_date?: string;
  payment_term_id?: number;
  notes?: string;
  status: "DRAFT" | "ISSUED" | "PARTIALLY_PAID" | "PAID" | "VOID";
  total_cents: number;
  dp_applied_cents: number;
  receivable_cents: number;
}

/** An invoice line (GET /api/v1/invoices/{id}). */
export interface InvoiceLine {
  id: number;
  item_id: number;
  item_code?: string;
  item_name?: string;
  delivery_id?: number;
  line_no: number;
  qty: string;
  unit_price_cents: number;
  discount_cents: number;
  tax_rate: string;
  line_total_cents: number;
  description?: string;
}

/** Full invoice with lines. */
export interface Invoice extends InvoiceListItem {
  lines: InvoiceLine[];
}

/** Input line for POST /api/v1/invoices. */
export interface InvoiceLineInput {
  item_id: number;
  delivery_id?: number;
  qty: number;
  unit_price_cents: number;
  discount_cents: number;
  tax_rate: number;
  description?: string;
}

/** Payload POST /api/v1/invoices. */
export interface CreateInvoiceInput {
  sales_order_id?: number;
  customer_id: number;
  invoice_date: string;
  due_date?: string;
  payment_term_id?: number;
  notes?: string;
  lines: InvoiceLineInput[];
}

/* ------------------------------------------------------------------ */
/* Invoice Payment (Pelunasan)                                         */
/* ------------------------------------------------------------------ */

/** Payment record (POST /invoices/{id}/payments response). */
export interface InvoicePayment {
  id: number;
  number: string;
  invoice_id: number;
  customer_id: number;
  journal_entry_id?: number;
  amount_cents: number;
  ar_applied_cents: number;
  overpayment_cents: number;
  cash_account_id: number;
  payment_date: string;
  description?: string;
  status: "RECEIVED" | "REVERSED";
}

/** Payload POST /api/v1/invoices/{id}/payments. */
export interface CreatePaymentInput {
  cash_account_id: number;
  amount_cents: number;
  payment_date: string;
  description?: string;
}

/* ------------------------------------------------------------------ */
/* Credit Note (CN / Sales Return)                                     */
/* ------------------------------------------------------------------ */

/** Credit note list row (GET /api/v1/credit-notes). */
export interface CreditNoteListItem {
  id: number;
  number: string;
  invoice_id: number;
  customer_id: number;
  customer_name?: string;
  cn_date: string;
  refund_method: "deduct" | "refund" | "credit_balance";
  reason?: string;
  status: "DRAFT" | "APPLIED" | "VOID";
  total_cents: number;
  ar_deducted_cents: number;
  cogs_reversed_cents: number;
}

/** A credit note line (GET /api/v1/credit-notes/{id}). */
export interface CreditNoteLine {
  id: number;
  item_id: number;
  item_code?: string;
  item_name?: string;
  invoice_line_id?: number;
  line_no: number;
  qty: string;
  unit_price_cents: number;
  unit_cost_cents: number;
  line_total_cents: number;
  cogs_reversed_cents: number;
  description?: string;
}

/** Full credit note with lines. */
export interface CreditNote extends CreditNoteListItem {
  lines: CreditNoteLine[];
}

/** Input line for POST /api/v1/credit-notes. */
export interface CreditNoteLineInput {
  item_id: number;
  invoice_line_id?: number;
  qty: number;
  unit_price_cents: number;
  unit_cost_cents: number;
  description?: string;
}

/** Payload POST /api/v1/credit-notes. */
export interface CreateCreditNoteInput {
  invoice_id: number;
  customer_id: number;
  cn_date: string;
  refund_method?: string;
  reason?: string;
  lines: CreditNoteLineInput[];
}

/* ------------------------------------------------------------------ */
/* Suppliers                                                           */
/* ------------------------------------------------------------------ */

export interface SupplierListItem {
  id: number;
  code: string;
  name: string;
  npwp?: string;
  contact_person?: string;
  phone?: string;
  email?: string;
  address?: string;
  city?: string;
  is_active: boolean;
}

export interface Supplier extends SupplierListItem {
  province?: string;
  postal_code?: string;
  payment_term_id?: number;
  credit_limit_cents?: number;
}

export interface CreateSupplierInput {
  code: string;
  name: string;
  npwp?: string;
  contact_person?: string;
  phone?: string;
  email?: string;
  address?: string;
  city?: string;
}

/* ------------------------------------------------------------------ */
/* Purchase Order (PO) — commitment only, no journal                    */
/* ------------------------------------------------------------------ */

export interface PurchaseOrderListItem {
  id: number;
  number: string;
  supplier_id: number;
  supplier_name?: string;
  order_date: string;
  payment_term_id?: number;
  notes?: string;
  status: "CONFIRMED" | "PARTIALLY_RECEIVED" | "RECEIVED" | "CANCELLED";
  total_cents: number;
  received_cents: number;
}

export interface PurchaseOrderLine {
  id: number;
  item_id: number;
  item_code?: string;
  item_name?: string;
  line_no: number;
  qty: string;
  unit_price_cents: number;
  discount_cents: number;
  tax_rate: string;
  line_total_cents: number;
  received_qty: string;
  description?: string;
}

export interface PurchaseOrder extends PurchaseOrderListItem {
  lines: PurchaseOrderLine[];
}

export interface PurchaseOrderLineInput {
  item_id: number;
  qty: number;
  unit_price_cents: number;
  discount_cents: number;
  tax_rate: number;
  description?: string;
}

export interface CreatePurchaseOrderInput {
  supplier_id: number;
  order_date: string;
  payment_term_id?: number;
  notes?: string;
  lines: PurchaseOrderLineInput[];
}

/* ------------------------------------------------------------------ */
/* Goods Received Note (GRN) — posts Dr Inventory / Cr Accrued Payable */
/* ------------------------------------------------------------------ */

export interface GoodsReceivedNoteListItem {
  id: number;
  number: string;
  purchase_order_id: number;
  supplier_id: number;
  supplier_name?: string;
  grn_date: string;
  notes?: string;
  status: "RECEIVED" | "RETURNED" | "CANCELLED";
  journal_entry_id?: number;
  total_cents: number;
}

export interface GRNLine {
  id: number;
  item_id: number;
  item_code?: string;
  item_name?: string;
  line_no: number;
  qty: string;
  unit_cost_cents: number;
  line_total_cents: number;
  description?: string;
}

export interface GoodsReceivedNote extends GoodsReceivedNoteListItem {
  lines: GRNLine[];
}

export interface GRNLineInput {
  item_id: number;
  po_line_id?: number;
  qty: number;
  unit_cost_cents: number;
  description?: string;
}

export interface CreateGRNInput {
  purchase_order_id: number;
  grn_date: string;
  notes?: string;
  lines: GRNLineInput[];
}

/* Supplier Invoice (Tagihan) */
export interface SupplierInvoiceListItem {
  id: number; number: string; supplier_id: number; supplier_name?: string;
  grn_id?: number; invoice_date: string; due_date?: string;
  supplier_invoice_number?: string; dpp_cents: number; vat_cents: number;
  total_cents: number; dp_applied_cents: number; payable_cents: number;
  status: "ISSUED"|"PARTIALLY_PAID"|"PAID"|"VOID"; journal_entry_id?: number;
}
export interface SupplierInvoiceLine {
  id: number; item_id: number; item_code?: string; item_name?: string;
  line_no: number; qty: string; unit_price_cents: number; discount_cents: number;
  tax_rate: number; line_total_cents: number; description?: string;
}
export interface SupplierInvoice extends SupplierInvoiceListItem { notes?: string; lines: SupplierInvoiceLine[]; }
export interface SupplierInvoiceLineInput {
  item_id: number; qty: number; unit_price_cents: number; discount_cents: number; tax_rate: number; description?: string;
}
export interface CreateSupplierInvoiceInput {
  supplier_id: number; grn_id?: number; invoice_date: string; due_date?: string;
  supplier_invoice_number?: string; notes?: string; lines: SupplierInvoiceLineInput[];
}

/* Supplier Payment (Bayar) */
export interface SupplierPayment {
  id: number;
  number: string;
  supplier_id: number;
  invoice_id: number;
  journal_entry_id?: number;
  amount_cents: number;
  ap_applied_cents: number;
  overpayment_cents: number;
  cash_account_id: number;
  payment_date: string;
  description?: string;
  status: "PAID" | "REVERSED";
}

export interface CreateSupplierPaymentInput {
  cash_account_id: number;
  amount_cents: number;
  payment_date: string;
  description?: string;
}

/* Purchase Return (Retur Pembelian) */
export interface PurchaseReturnListItem {
  id: number; number: string; supplier_id: number; supplier_name?: string;
  invoice_id: number; return_date: string; refund_method: "deduct"|"refund"|"credit_balance";
  reason?: string; status: "APPLIED"|"VOID"; total_cents: number;
  vat_reversed_cents: number; ap_deducted_cents: number;
}
export interface PurchaseReturnLine {
  id: number; item_id: number; item_code?: string; item_name?: string;
  invoice_line_id?: number; line_no: number; qty: string;
  unit_price_cents: number; line_total_cents: number; description?: string;
}
export interface PurchaseReturn extends PurchaseReturnListItem { lines: PurchaseReturnLine[]; }
export interface PurchaseReturnLineInput {
  item_id: number; invoice_line_id?: number; qty: number;
  unit_price_cents: number; description?: string;
}
export interface CreatePurchaseReturnInput {
  invoice_id: number; supplier_id: number; return_date: string;
  refund_method?: string; reason?: string; lines: PurchaseReturnLineInput[];
}

/* ------------------------------------------------------------------ */
/* Stock Opname (US-043) — physical count adjustment                  */
/*   surplus  (diff > 0): Dr Inventory / Cr Inventory Adjustment Gain */
/*   shortage (diff < 0): Dr Inventory Adjustment Loss / Cr Inventory */
/* ------------------------------------------------------------------ */

export interface StockOpnameListItem {
  id: number;
  number: string;
  opname_date: string;
  notes?: string;
  status: "DRAFT" | "COUNTED" | "APPROVED" | "VOID";
  journal_entry_id?: number;
  total_adjustment_cents: number;
}

export interface StockOpnameLine {
  id: number;
  item_id: number;
  item_code?: string;
  item_name?: string;
  line_no: number;
  system_qty: string;
  counted_qty: string;
  diff_qty: string;
  unit_cost_cents: number;
  adjustment_cents: number;
  inventory_account_id: number;
  reason?: string;
}

export interface StockOpname extends StockOpnameListItem {
  lines: StockOpnameLine[];
}

export interface StockOpnameLineInput {
  item_id: number;
  counted_qty: number;
  unit_cost_cents: number;
  reason?: string;
}

export interface CreateStockOpnameInput {
  opname_date: string;
  notes?: string;
  lines: StockOpnameLineInput[];
}

/* ------------------------------------------------------------------ */
/* Stock Transfer (US-042) — stock movement, no journal posted        */
/*   (same inventory account, no value change)                        */
/* ------------------------------------------------------------------ */

export interface StockTransferListItem {
  id: number;
  number: string;
  transfer_date: string;
  notes?: string;
  status: "COMPLETED" | "VOID";
}

export interface StockTransferLine {
  id: number;
  item_id: number;
  item_code?: string;
  item_name?: string;
  line_no: number;
  qty: string;
  unit_cost_cents: number;
  inventory_account_id: number;
  description?: string;
}

export interface StockTransfer extends StockTransferListItem {
  lines: StockTransferLine[];
}

export interface StockTransferLineInput {
  item_id: number;
  qty: number;
  unit_cost_cents: number;
  description?: string;
}

export interface CreateStockTransferInput {
  transfer_date: string;
  notes?: string;
  lines: StockTransferLineInput[];
}

/* ================================================================== */
/* Accountant Mode (manual journals, general ledger, journal register) */
/* ================================================================== */

/* Manual Journal Entry */
export interface JournalEntryListItem {
  id: number; number: string; entry_date: string; description: string;
  intent_type: string; status: string; total_debit_cents: number; total_credit_cents: number;
}
export interface JournalEntryLine {
  account_id: number; account_code: string; account_name: string;
  debit_cents: number; credit_cents: number; description?: string;
}
export interface JournalEntry extends JournalEntryListItem {
  source_ref: string; lines: JournalEntryLine[];
}
export interface ManualJournalInput {
  entry_date: string; description: string;
  lines: { account_id: number; debit_cents: number; credit_cents: number; description?: string }[];
}

/* General Ledger (Buku Besar) */
export interface GeneralLedgerEntry {
  entry_number: string; entry_date: string; description: string;
  debit_cents: number; credit_cents: number; running_balance_cents: number;
}
export interface GeneralLedgerResult {
  account_id: number; account_code: string; account_name: string;
  opening_balance_cents: number; entries: GeneralLedgerEntry[]; closing_balance_cents: number;
}

/* Journal Register */
export interface JournalRegisterItem {
  id: number; number: string; entry_date: string; description: string;
  intent_type: string; total_debit_cents: number; total_credit_cents: number;
}

/* ------------------------------------------------------------------ */
/* Bank Reconciliation (US-050)                                       */
/* ------------------------------------------------------------------ */

export interface BankStatementLineInput {
  tx_date: string; description?: string; reference?: string; amount_cents: number;
}
export interface CreateBankStatementInput {
  bank_account_id: number; statement_date: string;
  opening_balance_cents: number; closing_balance_cents: number;
  notes?: string; lines: BankStatementLineInput[];
}

export interface BankStatementLine {
  id: number; line_no: number; tx_date: string;
  description?: string; reference?: string; amount_cents: number;
  matched_journal_line_id?: number | null;
  match_status: "UNMATCHED" | "MATCHED" | "MANUAL" | "ADJUSTMENT";
  journal_entry_number?: string | null;
  journal_entry_date?: string | null;
  journal_description?: string | null;
}
export interface BankStatementListItem {
  id: number; bank_account_id: number; bank_account_name?: string;
  bank_account_code?: string; statement_date: string;
  opening_balance_cents: number; closing_balance_cents: number;
  status: "IMPORTED" | "RECONCILING" | "RECONCILED" | "VOID"; line_count: number;
}
export interface BankStatement extends BankStatementListItem {
  notes?: string; lines: BankStatementLine[];
}

export interface BookCandidate {
  journal_line_id: number; entry_id: number;
  entry_number: string; entry_date: string;
  amount_cents: number; direction: "DEBIT" | "CREDIT";
  description?: string | null; is_matched: boolean;
}
export interface BankReconciliationListItem {
  id: number; statement_id: number; bank_account_id: number;
  bank_account_name?: string; recon_date: string;
  book_balance_cents: number; statement_balance_cents: number;
  adjusted_book_cents: number; adjusted_statement_cents: number; diff_cents: number;
  status: "DRAFT" | "RECONCILED" | "VOID"; notes?: string;
  summary?: {
    statement_balance_cents: number; book_balance_cents: number;
    adjusted_book_cents: number; adjusted_statement_cents: number; diff_cents: number;
    matched_count: number; unmatched_count: number; total_lines: number;
  };
}
export interface BankReconciliation extends BankReconciliationListItem {
  statement_lines: BankStatementLine[];
  book_candidates: BookCandidate[];
  summary: {
    statement_balance_cents: number; book_balance_cents: number;
    adjusted_book_cents: number; adjusted_statement_cents: number; diff_cents: number;
    matched_count: number; unmatched_count: number; total_lines: number;
  };
}

export interface ReconcileMatchInput {
  statement_line_id: number; journal_line_id: number;
}
export interface ReconcileUnmatchInput {
  statement_line_id: number;
}

/* ------------------------------------------------------------------ */
/* Financial Notes (Catatan atas Laporan Keuangan — dasar)             */
/* ------------------------------------------------------------------ */

export interface FinancialNote {
  id: number;
  period_year: number;
  note_number: string;
  title: string;
  content: string;
  display_order: number;
}

export interface FinancialNoteInput {
  period_year: number;
  note_number: string;
  title: string;
  content: string;
  display_order?: number;
}

/* ------------------------------------------------------------------ */
/* Due Date Reminders (Pengingat Jatuh Tempo)                          */
/* ------------------------------------------------------------------ */

export type DueDateDirection = "customer" | "supplier";

export interface DueDateReminder {
  id: number;
  number: string;
  party_name: string;
  direction: DueDateDirection;
  invoice_date: string;
  due_date: string;
  amount_cents: number;
  status: string;
  days_overdue: number;
}

/* ------------------------------------------------------------------ */
/* Production / Job Order Costing (US-070..072)                        */
/*   BOM, production jobs, job costs, job completion.                  */
/*   Material cost: Dr 1303 WIP / Cr 1301 Inventory                   */
/*   Labor/Overhead: Dr 1303 WIP / Cr 1101 Cash                       */
/*   Complete: Dr 1304 Finished Goods / Cr 1303 WIP                   */
/*   Variance loss: Dr 5901 / Cr 1303; gain: Dr 1303 / Cr 4902        */
/* ------------------------------------------------------------------ */

export type ProductionCostType = "material" | "labor" | "overhead";

export interface BOMLine {
  id: number;
  item_id: number;
  item_code?: string;
  item_name?: string;
  line_no: number;
  qty: number;
  unit_cost_cents: number;
  line_total_cents: number;
  cost_type: ProductionCostType;
  description?: string;
}

export interface BOM {
  id: number;
  code: string;
  name: string;
  finished_good_item_id: number;
  finished_good_code?: string;
  finished_good_name?: string;
  output_qty: number;
  status: "ACTIVE" | "VOID";
  lines?: BOMLine[];
}

/** BOM list item (header only, no lines). */
export type BOMListItem = BOM;

export interface BOMLineInput {
  item_id: number;
  qty: number;
  unit_cost_cents: number;
  cost_type: ProductionCostType;
  description?: string;
}

export interface CreateBOMInput {
  code: string;
  name: string;
  finished_good_item_id: number;
  output_qty: number;
  lines: BOMLineInput[];
}

export interface ProductionJobCost {
  id: number;
  cost_type: ProductionCostType;
  item_id?: number;
  item_code?: string;
  item_name?: string;
  description?: string;
  qty?: number;
  unit_cost_cents: number;
  total_cents: number;
  journal_entry_id?: number;
  posted_at?: string;
}

export interface ProductionJob {
  id: number;
  number: string;
  bom_id?: number;
  finished_good_item_id: number;
  finished_good_code?: string;
  finished_good_name?: string;
  target_qty: number;
  completed_qty: number;
  start_date: string;
  completion_date?: string;
  status: "OPEN" | "IN_PROGRESS" | "COMPLETED" | "CANCELLED";
  wip_account_id: number;
  finished_good_account_id: number;
  total_material_cents: number;
  total_labor_cents: number;
  total_overhead_cents: number;
  total_cost_cents: number;
  variance_cents: number;
  journal_entry_id?: number;
  costs?: ProductionJobCost[];
}

/** Production job list item (header only, no costs). */
export type ProductionJobListItem = ProductionJob;

export interface CreateProductionJobInput {
  bom_id?: number;
  finished_good_item_id: number;
  target_qty: number;
  start_date: string;
}

export interface AddProductionJobCostInput {
  cost_type: ProductionCostType;
  item_id?: number;
  description?: string;
  qty: number;
  unit_cost_cents: number;
}

/** Alias for the cost-creation request body (same shape). */
export type CreateProductionJobCostInput = AddProductionJobCostInput;

export interface CompleteProductionJobInput {
  /** Optional completed quantity (defaults to target_qty when omitted). */
  completed_qty?: number;
}
