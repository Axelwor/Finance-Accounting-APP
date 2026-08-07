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
  | "purchase-invoice"
  | "purchase-payment"
  | "inventory-items"
  | "stock-movements"
  | "asset-register"
  | "report-trial-balance"
  | "report-profit-loss"
  | "report-balance-sheet"
  | "report-cash-flow";

/** Workbench entry sub-kind identifiers (drive the entry tab dispatch). */
export type EntrySubKind =
  | "money-in"
  | "money-out"
  | "cash-transfer"
  | "sales-invoice"
  | "sales-receipt"
  | "purchase-invoice"
  | "purchase-payment"
  | "inventory-item"
  | "asset-register";

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
