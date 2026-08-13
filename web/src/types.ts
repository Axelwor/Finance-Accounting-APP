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
  | "customer-list"
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
  | "fixed-assets"
  | "journal-entry"
  | "general-ledger"
  | "journal-register"
  | "report-trial-balance"
  | "report-profit-loss"
  | "report-balance-sheet"
  | "report-cash-flow"
  | "financial-notes"
  | "due-date-reminders"
  | "customer-statement"
  | "bom"
  | "production-job"
  | "ppn-reconciliation"
  | "pph-final"
  | "ecl-calculator"
  | "dimensions"
  | "budgets"
  | "budget-vs-actual"
  | "audit-logs"
  | "lease-contract"
  | "consolidated-report"
  | "report-templates"
  | "approval-rules"
  | "pending-approval-requests"
  | "cheque-list"
  | "ar-aging"
  | "ap-aging";

  | "warehouse-list";
  | "email-templates"
  | "email-queue";
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
  | "customer-entry"
  | "purchase-invoice"
  | "supplier-invoice-entry"
  | "purchase-payment"
  | "purchase-return-entry"
  | "inventory-item"
  | "stock-opname-entry"
  | "stock-transfer-entry"
  | "asset-register"
  | "fixed-assets-entry"
  | "asset-depreciate"
  | "asset-dispose"
  | "journal-entry"
  | "financial-notes-entry"
  | "bom-entry"
  | "production-job-entry"
  | "budget-entry"
  | "lease-contract-entry"
  | "lease-payment-schedule"
  | "rp-editor"
  | "approval-rule-entry"
  | "cheque-entry"
  | "warehouse-entry";

  | "email-template-entry";

export type CurrencyCode = "IDR";

export interface Business {
  id: string;
  name: string;
  businessType: string;
  currency: CurrencyCode;
  /** Month (1-12) when the fiscal year starts. */
  fiscalYearStart: number;
}

/** A tenant (book) the signed-in user belongs to, from GET /tenants. */
export interface Tenant {
  id: string;
  name: string;
  slug: string;
  role: string;
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
  /** Present when ?framework= is set — same totals, different presentation. */
  framework?: string;
  sections?: ProfitLossSection[];
}

/** One grouped line in a framework P&L (EMKM/ETAP/SAK Umum). */
export interface ProfitLossSection {
  code: string;
  label: string;
  amount_cents: number;
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

/** One aging bucket in an AR/AP aging report. */
export interface AgingBucket {
  bucket: "0-30" | "31-60" | "61-90" | ">90";
  amount_cents: number;
  count: number;
}

/** Response GET /api/v1/aging/ar and /api/v1/aging/ap. */
export interface AgingReport {
  asOf: string;
  buckets: AgingBucket[];
  summary: {
    total_invoices: number;
    total_receivable_cents: number;
    overdue_amount_cents: number;
  };
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
  npwp?: string | null;
  contact_person?: string | null;
  phone?: string | null;
  email?: string | null;
  address?: string | null;
  city?: string | null;
  province?: string | null;
  postal_code?: string | null;
  payment_term_id?: number | null;
  credit_limit_cents?: number | null;
  default_revenue_account_id?: number | null;
  default_receivable_account_id?: number | null;
  is_active: boolean;
  billing_address?: string | null;
  shipping_address?: string | null;
  customer_group?: string | null;
  price_level?: "RETAIL" | "WHOLESALE" | "DISTRIBUTOR" | "SPECIAL";
  currency_code?: string;
  is_pkp: boolean;
  credit_hold: boolean;
  website?: string | null;
  fax?: string | null;
  contact_person_2?: string | null;
  phone_2?: string | null;
  npwp_name?: string | null;
  opening_balance_cents: number;
  opening_balance_date?: string | null;
}

/** Customer price levels (migration 000033). */
export type PriceLevel = "RETAIL" | "WHOLESALE" | "DISTRIBUTOR" | "SPECIAL";

/** Payment term master data (GET/POST /api/v1/payment-terms). */
export interface PaymentTerm {
  id: number;
  code: string;
  name: string;
  due_days: number;
  discount_days?: number | null;
  discount_percent?: string | null;
  is_active: boolean;
}

/** Payload POST /api/v1/customers (also reused for PUT /api/v1/customers/{id}). */
export interface CreateCustomerInput {
  code: string;
  name: string;
  npwp?: string;
  contact_person?: string;
  phone?: string;
  email?: string;
  address?: string;
  city?: string;
  province?: string;
  postal_code?: string;
  payment_term_id?: number;
  credit_limit_cents?: number;
  default_revenue_account_id?: number;
  default_receivable_account_id?: number;
  is_active?: boolean;
  billing_address?: string;
  shipping_address?: string;
  customer_group?: string;
  price_level?: PriceLevel;
  currency_code?: string;
  is_pkp?: boolean;
  credit_hold?: boolean;
  website?: string;
  fax?: string;
  contact_person_2?: string;
  phone_2?: string;
  npwp_name?: string;
  opening_balance_cents?: number;
  opening_balance_date?: string;
}

/** Statement line for customer AR statement. */
export interface CustomerStatementLine {
  date: string;
  type: "invoice" | "payment";
  reference: string;
  description: string;
  debit_cents: number;
  credit_cents: number;
  running_balance_cents: number;
}

/** Customer statement response (GET /customers/{id}/statement). */
export interface CustomerStatement {
  customer_id: number;
  code: string;
  name: string;
  from_date: string;
  to_date: string;
  opening_balance_cents: number;
  lines: CustomerStatementLine[];
  invoiced_cents: number;
  paid_cents: number;
  closing_balance_cents: number;
}

/** Item costing methods accepted by the backend (items.costing_method). */
export type ItemCostingMethod = "fifo" | "moving_average" | "specific";

/** Item master data (GET /api/v1/items). */
export interface Item {
  id: number;
  code: string;
  name: string;
  item_type: "goods" | "service";
  is_tracked_stock: boolean;
  is_active: boolean;
  /** Base unit of measure (backend json: `uom`). */
  uom?: string;
  /** Legacy alias for uom kept for older list screens. */
  unit?: string;
  costing_method?: ItemCostingMethod | null;
  sale_account_id?: number | null;
  cogs_account_id?: number | null;
  inventory_account_id?: number | null;
  min_stock_qty?: number | null;
  sale_price_cents: number;
  qty_on_hand?: number | null;
  description?: string;
  barcode?: string | null;
  secondary_uom?: string | null;
  uom_conversion_factor?: number | null;
  brand?: string | null;
  category?: string | null;
  weight_grams?: number | null;
  volume_cc?: number | null;
  description_long?: string | null;
  image_url?: string | null;
  reorder_point?: number | null;
  reorder_qty?: number | null;
  lead_time_days?: number | null;
  preferred_supplier_id?: number | null;
  abc_classification?: "A" | "B" | "C" | null;
  sale_uom?: string | null;
  purchase_uom?: string | null;
}

/**
 * Payload for POST /api/v1/items (api.createItem). ERP columns follow
 * migrations 000005 + 000033. Fields without a backend column yet
 * (purchase price, opening balances) are accepted for forward
 * compatibility; see api.createItem for what is persisted today.
 */
export interface CreateItemInput {
  code: string;
  name: string;
  item_type: "goods" | "service";
  /** Base unit of measure. */
  uom: string;
  costing_method?: ItemCostingMethod;
  inventory_account_id?: number;
  cogs_account_id?: number;
  /** Revenue account FK (items.sale_account_id). */
  sale_account_id?: number;
  sale_uom?: string;
  purchase_uom?: string;
  barcode?: string;
  brand?: string;
  category?: string;
  /** Short description; used as description_long when the long one is empty. */
  description?: string;
  description_long?: string;
  weight_grams?: number;
  volume_cc?: number;
  /** Default sale price; persisted to the "Umum" item price list. */
  sale_price_cents?: number;
  /** Reference purchase price (no backend column yet). */
  purchase_price_cents?: number;
  reorder_point?: number;
  reorder_qty?: number;
  lead_time_days?: number;
  preferred_supplier_id?: number;
  abc_classification?: "A" | "B" | "C";
  /** Opening stock qty (no backend column yet — use Stock Opname to post). */
  opening_balance_qty?: number;
  /** Opening stock cost in cents (no backend column yet). */
  opening_balance_cost_cents?: number;
}

/** One item price-list entry (GET/POST /api/v1/items/{id}/prices). */
export interface ItemPriceEntry {
  id: number;
  item_id: number;
  price_list_name: string;
  customer_group?: string | null;
  customer_id?: number | null;
  unit_price_cents: number;
  currency_code: string;
  effective_from?: string | null;
  effective_to?: string | null;
  is_active: boolean;
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
  customer_po_number?: string | null;
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
  customer_po_date?: string | null;
  requested_delivery_date?: string | null;
  salesperson_id?: number | null;
  ship_to_address?: string | null;
  shipping_terms?: "FOB" | "CIF" | "EXW" | "CFR" | "DAP";
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
  customer_po_number?: string;
  customer_po_date?: string;
  requested_delivery_date?: string;
  salesperson_id?: number;
  ship_to_address?: string;
  shipping_terms?: "FOB" | "CIF" | "EXW" | "CFR" | "DAP";
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
  tax_invoice_number?: string | null;
  sub_total_cents?: number;
  discount_total_cents?: number;
  tax_total_cents?: number;
  shipping_fee_cents?: number;
  other_charges_cents?: number;
  rounding_cents?: number;
  salesperson_id?: number | null;
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
  tax_invoice_number?: string;
  shipping_fee_cents?: number;
  other_charges_cents?: number;
  rounding_cents?: number;
  salesperson_id?: number;
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
  /** Original delivery order — lets the backend resolve the true cost. */
  delivery_order_id?: number;
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
  supplier_type?: "GOODS" | "SERVICE" | "MIXED";
  is_pkp?: boolean;
  currency_code?: string;
  bank_name?: string | null;
  bank_account_number?: string | null;
  bank_account_name?: string | null;
  website?: string | null;
  fax?: string | null;
  contact_person_2?: string | null;
  phone_2?: string | null;
  opening_balance_cents?: number;
  opening_balance_date?: string | null;
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
  province?: string;
  postal_code?: string;
  payment_term_id?: number;
  credit_limit_cents?: number;
  supplier_type?: "GOODS" | "SERVICE" | "MIXED";
  is_pkp?: boolean;
  currency_code?: string;
  bank_name?: string;
  bank_account_number?: string;
  bank_account_name?: string;
  website?: string;
  fax?: string;
  contact_person_2?: string;
  phone_2?: string;
  opening_balance_cents?: number;
  opening_balance_date?: string;
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
  supplier_quote_number?: string | null;
  supplier_quote_date?: string | null;
  buyer_id?: number | null;
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
  supplier_quote_number?: string;
  supplier_quote_date?: string;
  buyer_id?: number;
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
/* Fixed Assets (Aset Tetap) — US-060..063                             */
/* ------------------------------------------------------------------ */

export type DepreciationMethod = "straight_line" | "declining_balance" | "units_of_production";
export type AssetStatus = "ACTIVE" | "DISPOSED" | "IMPAIRED";
export type AssetTxType = "ACQUISITION" | "DEPRECIATION" | "REVALUATION" | "DISPOSAL" | "IMPAIRMENT";

/** Row in the asset register list. */
export interface FixedAssetListItem {
  id: number;
  code: string;
  name: string;
  acquisition_date: string;
  acquisition_cost_cents: number;
  salvage_value_cents: number;
  useful_life_months: number;
  depreciation_method: DepreciationMethod;
  rate: string;
  status: AssetStatus;
  book_value_cents: number;
  accum_dep_cents: number;
}

export interface AssetDepreciationScheduleItem {
  id: number;
  asset_id: number;
  period_year: number;
  period_month: number;
  depreciation_cents: number;
  journal_entry_id?: number;
  posted: boolean;
  posted_at?: string;
}

export interface AssetTransactionItem {
  id: number;
  asset_id: number;
  tx_type: AssetTxType;
  tx_date: string;
  amount_cents: number;
  journal_entry_id?: number;
  description?: string;
  created_at: string;
}

/** Full asset detail (GET /fixed-assets/{id}). */
export interface FixedAsset {
  id: number;
  code: string;
  name: string;
  asset_account_id: number;
  accum_dep_account_id: number;
  dep_expense_account_id: number;
  impairment_account_id?: number;
  acquisition_date: string;
  acquisition_cost_cents: number;
  salvage_value_cents: number;
  useful_life_months: number;
  depreciation_method: DepreciationMethod;
  rate: string;
  units_total?: number;
  units_used: number;
  status: AssetStatus;
  book_value_cents: number;
  accum_dep_cents: number;
  journal_entry_id?: number;
  created_at?: string;
  updated_at?: string;
  schedule?: AssetDepreciationScheduleItem[];
  transactions?: AssetTransactionItem[];
}

export interface RegisterFixedAssetInput {
  code: string;
  name: string;
  acquisition_date: string;
  acquisition_cost_cents: number;
  salvage_value_cents: number;
  useful_life_months: number;
  depreciation_method: DepreciationMethod;
  rate?: string;
  units_total?: number;
  payment_account_code?: string;
  description?: string;
}

export interface DepreciateAssetInput {
  period_year: number;
  period_month: number;
  entry_date: string;
  description?: string;
}

export interface DepreciationResult {
  asset_id: number;
  period_year: number;
  period_month: number;
  depreciation_cents: number;
  journal_entry_id?: number;
  schedule_id?: number;
  book_value_cents: number;
  accum_dep_cents: number;
  already_posted?: boolean;
  status: string;
}

export interface RevalueAssetInput {
  new_value_cents: number;
  entry_date: string;
  description?: string;
}

export interface DisposeAssetInput {
  disposal_date: string;
  proceeds_cents: number;
  cash_account_code?: string;
  description?: string;
}

export interface AssetDisposalResult {
  asset_id: number;
  proceeds_cents: number;
  book_value_cents: number;
  gain_loss_cents: number;
  journal_entry_id?: number;
  status: string;
}

export interface ImpairAssetInput {
  entry_date: string;
  impaired_value_cents: number;
  description?: string;
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

/* ------------------------------------------------------------------ */
/* Tax — PPN, PPh Final UMKM, ECL, Deferred Tax (US-080..083)         */
/* ------------------------------------------------------------------ */

/** PPN summary (GET /ppn/summary). */
export interface PPNSummary {
  from_date: string;
  to_date: string;
  ppn_keluaran_cents: number;
  ppn_masukan_cents: number;
  net_ppn_cents: number;
}

/** One VAT movement in the detailed PPN reconciliation. */
export interface PPNReconciliationLine {
  entry_id: number;
  entry_number: string;
  entry_date: string;
  description: string;
  intent_type: string;
  source_ref: string;
  account_code: string;
  account_name: string;
  direction: "KELUARAN" | "MASUKAN";
  debit_cents: number;
  credit_cents: number;
}

/** PPN reconciliation report (GET /ppn/reconciliation). */
export interface PPNReconciliationResult {
  period_year: number;
  period_month: number;
  ppn_keluaran_cents: number;
  ppn_masukan_cents: number;
  net_ppn_cents: number;
  lines: PPNReconciliationLine[];
}

/** Filed PPN reconciliation record (POST /ppn/reconcile). */
export interface PPNReconciliationRecord {
  id: number;
  period_year: number;
  period_month: number;
  ppn_keluaran_cents: number;
  ppn_masukan_cents: number;
  net_ppn_cents: number;
  status: string;
  notes: string;
  created_at: string;
}

export interface CreatePPNReconciliationInput {
  period_year: number;
  period_month: number;
  notes?: string;
}

/** PPh Final UMKM calculation result. */
export interface PPhFinalResult {
  journal_entry_id: number;
  number: string;
  intent_type: string;
  entry_date: string;
  description: string;
  revenue_cents: number;
  tax_rate: string;
  tax_cents: number;
  payable_balance_cents: number;
}

export interface CalculatePPhFinalInput {
  period_year: number;
  period_month: number;
  entry_date: string;
  notes?: string;
}

export interface PayPPhFinalInput {
  entry_date: string;
  cash_account_id: number;
  amount_cents: number;
  notes?: string;
}

/** One ECL aging bucket. */
export interface ECLBucket {
  label: string;
  min_days: number;
  max_days: number;
  rate_pct: number;
  balance_cents: number;
  provision_cents: number;
}

/** ECL calculation result. */
export interface CalculateECLResult {
  journal_entry_id: number;
  number: string;
  intent_type: string;
  entry_date: string;
  description: string;
  as_of_date: string;
  buckets: ECLBucket[];
  target_allowance_cents: number;
  current_allowance_cents: number;
  adjustment_cents: number;
}

export interface CalculateECLInput {
  as_of_date: string;
  entry_date: string;
  notes?: string;
  rates?: Record<string, number>;
}

export interface WriteOffInput {
  entry_date: string;
  invoice_id?: number;
  amount_cents: number;
  notes?: string;
}

export interface WriteOffResult {
  journal_entry_id: number;
  number: string;
  intent_type: string;
  entry_date: string;
  description: string;
  invoice_id: number;
  amount_cents: number;
}

/** Deferred tax calculation (US-083). */
export interface CalculateDeferredTaxInput {
  temporary_differences_cents: number;
  tax_rate: number;
  entry_date: string;
  notes?: string;
}

export interface CalculateDeferredTaxResult {
  journal_entry_id: number;
  number: string;
  intent_type: string;
  entry_date: string;
  description: string;
  temporary_differences_cents: number;
  tax_rate: string;
  deferred_tax_cents: number;
  direction: "ASSET" | "REVERSAL";
}

/* ------------------------------------------------------------------ */
/* Report Frameworks (US-090A) + Dimensions + Budgets (US-093)       */
/* ------------------------------------------------------------------ */

export type ReportFramework = "EMKM" | "ETAP" | "SAK_UMUM";

/** Row in the report_frameworks table (GET /report-frameworks). */
export interface ReportFrameworkRecord {
  id: number;
  framework: ReportFramework;
  is_default: boolean;
  tenant_id: number;
  created_at: string;
}

/** Payload POST /report-frameworks. */
export interface SetFrameworkInput {
  framework: ReportFramework;
  is_default?: boolean;
}

/** Dimension master (GET/POST /dimensions). */
export interface Dimension {
  id: number;
  code: string;
  name: string;
  dimension_type: "branch" | "project" | "department" | "cost_center";
  is_active: boolean;
  created_at: string;
}

export interface CreateDimensionInput {
  code: string;
  name: string;
  dimension_type: Dimension["dimension_type"];
}

/** Payload POST /journal-lines/{id}/dimensions. */
export interface TagJournalLineInput {
  dimension_ids: number[];
}

/** Budget line input (POST /budgets). */
export interface BudgetLineInput {
  account_id: number;
  dimension_id?: number;
  month: number;
  amount_cents: number;
}

/** Budget line as returned by GET /budgets/{id}. */
export interface BudgetLine {
  id: number;
  budget_id: number;
  account_id: number;
  dimension_id: number;
  month: number;
  amount_cents: number;
}

/** Full budget with lines (GET /budgets/{id}, POST /budgets). */
export interface Budget {
  id: number;
  name: string;
  fiscal_year: number;
  dimension_id: number;
  status: "DRAFT" | "APPROVED" | "CLOSED";
  created_at: string;
  lines?: BudgetLine[];
}

/** Budget list row (GET /budgets). */
export interface BudgetListItem {
  id: number;
  name: string;
  fiscal_year: number;
  dimension_id: number;
  status: "DRAFT" | "APPROVED" | "CLOSED";
  created_at: string;
  line_count: number;
  total_cents: number;
}

export interface CreateBudgetInput {
  name: string;
  fiscal_year: number;
  dimension_id?: number;
  lines: BudgetLineInput[];
}

/** One row in the budget vs actual report. */
export interface BudgetVsActualRow {
  account_id: number;
  account_code: string;
  account_name: string;
  month: number;
  budget_cents: number;
  actual_cents: number;
  variance_cents: number;
}

/** Response GET /budgets/{id}/vs-actual. */
export interface BudgetVsActualResult {
  budget_id: number;
  name: string;
  fiscal_year: number;
  dimension_id: number;
  rows: BudgetVsActualRow[];
  total_budget_cents: number;
  total_actual_cents: number;
  total_variance_cents: number;
}
/** US-100: Attachment metadata returned by the backend. */
export interface Attachment {
  id: number;
  tenant_id: number;
  owner_type: string;
  owner_id: number;
  file_name: string;
  file_key: string;
  mime_type: string;
  file_size: number;
  ocr_status: "NONE" | "PENDING" | "COMPLETED" | "FAILED";
  created_at: string;
  uploaded_by: number;
}

/** US-101: Audit log entry returned by GET /audit-logs. */
export interface AuditLog {
  id: number;
  tenant_id: number;
  user_id: number;
  user_name: string;
  entity_type: string;
  entity_id: number;
  action: "CREATE" | "UPDATE" | "DELETE" | "POST" | "VOID" | "CLOSE" | "UNLOCK" | "APPROVE" | "REJECT";
  before_data: Record<string, unknown> | null;
  after_data: Record<string, unknown> | null;
  created_at: string;
}

/** Lease contract list item (US-111, PSAK 73). */
export interface LeaseContractListItem {
  id: number;
  number: string;
  lessee_name: string;
  lessor_name?: string;
  start_date: string;
  end_date: string;
  payment_amount_cents: number;
  payment_frequency: string;
  total_payments: number;
  discount_rate: string;
  status: string;
  initial_rou_cents: number;
  initial_liability_cents: number;
  journal_entry_id?: number;
}

/** Lease contract with full detail + payment schedule. */
export interface LeasePaymentScheduleItem {
  payment_no: number;
  payment_date: string;
  payment_amount_cents: number;
  principal_cents: number;
  interest_cents: number;
  remaining_liability_cents: number;
  journal_entry_id?: number;
  posted: boolean;
}

export interface LeaseContract extends LeaseContractListItem {
  rou_asset_account_id: number;
  lease_liability_account_id: number;
  interest_expense_account_id: number;
  schedule?: LeasePaymentScheduleItem[];
}

export interface CreateLeaseContractInput {
  lessee_name: string;
  lessor_name?: string;
  start_date: string;
  end_date: string;
  payment_amount_cents: number;
  payment_frequency: string;
  total_payments: number;
  discount_rate: string;
  payment_account_code?: string;
  description?: string;
}

export interface LeasePaymentResult {
  lease_id: number;
  payment_no: number;
  payment_date: string;
  payment_amount_cents: number;
  principal_cents: number;
  interest_cents: number;
  remaining_liability_cents: number;
  journal_entry_id?: number;
  posted: boolean;
}

/** Payload POST /api/v1/lease-contracts/{id}/modify (m-014). */
export interface ModifyLeaseContractInput {
  new_payment_amount_cents: number;
  new_total_payments: number;
  effective_date: string;
  description?: string;
}

/** Result of POST /api/v1/lease-contracts/{id}/modify. */
export interface ModifyLeaseResult {
  lease_id: number;
  number: string;
  journal_entry_id?: number;
  journal_number?: string;
  delta_cents: number;
  new_pv_cents: number;
  new_payment_cents: number;
  new_total_payments: number;
}

/** Payload POST /api/v1/lease-contracts/{id}/terminate (m-014). */
export interface TerminateLeaseContractInput {
  termination_date: string;
  description?: string;
}

/** Result of POST /api/v1/lease-contracts/{id}/terminate. */
export interface TerminateLeaseResult {
  lease_id: number;
  number: string;
  status: string;
  journal_entry_id?: number;
  journal_number?: string;
}

/** Entity hierarchy (US-110, PSAK 65). */
export interface EntityHierarchyItem {
  id: number;
  tenant_id: number;
  parent_tenant_id: number;
  relationship: string;
  consolidation_pct: number;
  tenant_name?: string;
  parent_tenant_name?: string;
  created_at?: string;
}

export interface CreateEntityHierarchyInput {
  child_tenant_id: number;
  parent_tenant_id: number;
  consolidation_pct?: number;
}

/** Consolidated trial balance (US-110). */
export interface ConsolidatedTrialBalanceRow {
  account_id: number;
  account_code: string;
  account_name: string;
  report_group: string;
  debit_cents: number;
  credit_cents: number;
}

export interface ConsolidatedTrialBalanceResult {
  rows: ConsolidatedTrialBalanceRow[];
  total_debit_cents: number;
  total_credit_cents: number;
  elimination_cents: number;
  balanced: boolean;
  consolidated_tenant_ids: number[];
}

/** Consolidated P&L (US-110). */
export interface ConsolidatedProfitLossResult {
  revenue_cents: number;
  expense_cents: number;
  profit_cents: number;
  elimination_cents: number;
  consolidated_tenant_ids: number[];
}

/* ------------------------------------------------------------------ */
/* Report Templates (US-090A)                                         */
/* ------------------------------------------------------------------ */

export type ReportTemplateDocumentType =
  | "invoice"
  | "purchase_order"
  | "delivery_order"
  | "tax_invoice"
  | "withholding_slip"
  | "customer_statement"
  | "supplier_statement"
  | "payment_voucher"
  | "receipt_voucher"
  | "journal_voucher"
  | "stock_card"
  | "trial_balance"
  | "profit_loss"
  | "balance_sheet"
  | "cash_flow"
  | "ar_aging"
  | "ap_aging"
  | "asset_register"
  | "stock_opname";

export interface ReportTemplate {
  id: number;
  tenant_id?: number;
  code: string;
  name: string;
  document_type: ReportTemplateDocumentType;
  /** Present only on GET /reports/templates/{id}; the list endpoint omits it. */
  template_yaml?: string;
  is_default: boolean;
  is_active: boolean;
  created_at?: string;
  updated_at?: string;
}

export interface CreateReportTemplateInput {
  code: string;
  name: string;
  document_type: ReportTemplateDocumentType;
  template_yaml: string;
}

/** Approval workflow types. */
export interface ApprovalRule {
  id: number;
  tenant_id: number;
  entity_type: string;
  min_amount_cents: number;
  approver_account_id: number;
  created_at: string;
  updated_at: string;
}

export interface CreateApprovalRuleInput {
  entity_type: string;
  min_amount_cents: number;
  approver_account_id: number;
}

export interface ApprovalRequest {
  id: number;
  tenant_id: number;
  entity_type: string;
  entity_id: number;
  amount_cents: number;
  status: "PENDING" | "APPROVED" | "REJECTED";
  requested_by: string;
  requested_at: string;
  approved_by?: string;
  approved_at?: string;
  rejected_by?: string;
  rejected_at?: string;
  rejection_reason?: string;
  /** Array of previous approve/reject actions for audit trail. */
  approval_history?: Array<{ action: "approved" | "rejected"; by: string; at: string }>;
}

export interface SubmitApprovalRequestInput {
  entity_type: string;
  entity_id: number;
  amount_cents: number;
}

export interface ApproveApprovalRequestInput {
  reason?: string;
}

/** Cheque list item for GET /api/v1/cheques. */
export interface ChequeListItem {
  id: number;
  cheque_number: string;
  type: "RECEIVED" | "ISSUED";
  direction: "INBOUND" | "OUTBOUND";
  bank_name: string;
  amount_cents: number;
  counterparty_name: string;
  date: string;
  status: "REGISTERED" | "DEPOSITED" | "CLEARED" | "BOUNCED";
}

/** Cheque full record with all fields. */
export interface Cheque extends ChequeListItem {
  bank_account_id?: number | null;
  image_url?: string | null;
  notes?: string | null;
}

/** Input for POST /api/v1/cheques and PUT /api/v1/cheques/{id}. */
export interface ChequeCreateInput {
  cheque_number: string;
  type: "RECEIVED" | "ISSUED";
  direction: "INBOUND" | "OUTBOUND";
  bank_account_id?: number | null;
  amount_cents: number;
  counterparty_name: string;
  date: string;
  bank_name?: string | null;
  image_url?: string | null;
  notes?: string | null;
}

/** Backend account for bank account combobox (from accounts table). */
export interface BankAccountListItem {
  id: number;
  account_name: string;
  code: string;
}

/** Warehouse master data (GET /warehouses). */
export interface Warehouse {
  id: number;
  code: string;
  name: string;
  address?: string;
  city?: string;
  is_active: boolean;
  created_at: string;
  updated_at?: string;
}

export interface CreateWarehouseInput {
  code: string;
  name: string;
  address?: string;
  city?: string;
  is_active: boolean;
}

export type UpdateWarehouseInput = CreateWarehouseInput;

/** Per-item stock balance in one warehouse (GET /warehouses/{id}/stock). */
export interface WarehouseStockItem {
  item_id: number;
  item_code: string;
  item_name: string;
  qty_on_hand: number;
  avg_unit_cost_cents: number;
}

/** End of file. Added approval workflow types 2026-08-12 Wave 4. */

/** Document types with labels for dropdowns. */
export const REPORT_TEMPLATE_TYPES: Record<ReportTemplateDocumentType, string> = {
  invoice: "Invoice",
  purchase_order: "Purchase Order",
  delivery_order: "Delivery Order",
  tax_invoice: "Tax Invoice",
  withholding_slip: "Withholding Slip",
  customer_statement: "Customer Statement",
  supplier_statement: "Supplier Statement",
  payment_voucher: "Payment Voucher",
  receipt_voucher: "Receipt Voucher",
  journal_voucher: "Journal Voucher",
  stock_card: "Stock Card",
  trial_balance: "Trial Balance",
  profit_loss: "Profit & Loss",
  balance_sheet: "Balance Sheet",
  cash_flow: "Cash Flow",
  ar_aging: "AR Aging",
  ap_aging: "AP Aging",
  asset_register: "Asset Register",
  stock_opname: "Stock Opname",
};

/* ------------------------------------------------------------------ */
/* Email Templates & Queue (Wave 6b)                                  */
/* ------------------------------------------------------------------ */

/** Email template trigger events. */
export type EmailTriggerEvent =
  | "INVOICE_SENT"
  | "PAYMENT_RECEIVED"
  | "CREDIT_NOTE_ISSUED"
  | "QUOTATION_ACCEPTED"
  | "DELIVERY_ORDER_COMPLETED";

/** Email template for automated notifications. */
export interface EmailTemplate {
  id: number;
  tenant_id: number;
  subject: string;
  body_html: string;
  body_text: string;
  trigger_event: EmailTriggerEvent;
  is_active: boolean;
  created_at: string;
  updated_at?: string;
}

/** Input for creating/updating email templates. */
export interface CreateEmailTemplateInput {
  subject: string;
  body_html: string;
  body_text?: string;
  trigger_event: EmailTriggerEvent;
  is_active?: boolean;
}

/** Email queue item - queued or sent email notification. */
export interface EmailQueueItem {
  id: number;
  tenant_id: number;
  template_id: number;
  recipient_email: string;
  subject: string;
  status: "PENDING" | "SENT" | "FAILED";
  trigger_event: EmailTriggerEvent;
  sent_at?: string;
  error_message?: string;
  created_at: string;
}

/** End of file. Added email templates types 2026-08-12 Wave 6b. */
