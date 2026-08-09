/**
 * Client API layer (M1, hybrid).
 *
 * Auth (register/login/logout) is connected to the backend:
 *   POST /api/v1/auth/register
 *   POST /api/v1/auth/login
 *   POST /api/v1/auth/logout
 *
 * Accounts, categories, transactions, dashboard, and onboarding are also
 * connected to backend endpoints (see API_CONTRACT.md). Every failed network
 * call falls back to local (mock) data so the app keeps working offline
 * (graceful degradation).
 */

import type {
  AccountItem,
  ApiError,
  BackendAccount,
  BackendBalanceSheet,
  BackendCashFlow,
  BackendCategory,
  BackendTrialBalance,
  BackendJournalResult,
  BackendProfitLoss,
  BackendTenant,
  CashCommandPayload,
  CashEntryListItem,
  Category,
  CurrencyCode,
  DashboardSummary,
  LoginInput,
  OnboardingInput,
  OpeningBalance,
  OpeningBalancePayload,
  PeriodResult,
  RegisterInput,
  Transaction,
  TransactionInput,
  TransferCommandPayload,
  Business,
  Customer,
  Item,
  Quotation,
  QuotationCreateInput,
  QuotationListItem,
  SalesOrder,
  SalesOrderCreateInput,
  SalesOrderListItem,
  CreateDownPaymentInput,
  DownPayment,
  DeliveryOrder,
  DeliveryOrderListItem,
  CreateDeliveryInput,
  Invoice,
  InvoiceListItem,
  CreateInvoiceInput,
  InvoicePayment,
  CreatePaymentInput,
  CreditNote,
  CreditNoteListItem,
  CreateCreditNoteInput,
  Supplier,
  SupplierListItem,
  CreateSupplierInput,
  PurchaseOrder,
  PurchaseOrderListItem,
  CreatePurchaseOrderInput,
  GoodsReceivedNote,
  GoodsReceivedNoteListItem,
  CreateGRNInput,
  SupplierInvoice,
  SupplierInvoiceListItem,
  CreateSupplierInvoiceInput,
  SupplierPayment,
  CreateSupplierPaymentInput,
  PurchaseReturn,
  PurchaseReturnListItem,
  CreatePurchaseReturnInput,
  StockOpname,
  StockOpnameListItem,
  CreateStockOpnameInput,
  StockTransfer,
  StockTransferListItem,
  CreateStockTransferInput,
  JournalEntryListItem,
  JournalEntry,
  ManualJournalInput,
  GeneralLedgerResult,
  JournalRegisterItem,
  BankStatement,
  BankStatementListItem,
  BankStatementLine,
  BankReconciliation,
  BookCandidate,
  CreateBankStatementInput,
  BankStatementLineInput,
  ReconcileMatchInput,
  ReconcileUnmatchInput,
  FinancialNote,
  FinancialNoteInput,
  BOM,
  BOMListItem,
  CreateBOMInput,
  ProductionJob,
  ProductionJobListItem,
  ProductionJobCost,
  CreateProductionJobInput,
  AddProductionJobCostInput,
  CreateProductionJobCostInput,
  CompleteProductionJobInput,
  DueDateReminder,
  FixedAsset,
  FixedAssetListItem,
  RegisterFixedAssetInput,
  DepreciateAssetInput,
  DepreciationResult,
  RevalueAssetInput,
  DisposeAssetInput,
  AssetDisposalResult,
  ImpairAssetInput,
  PPNSummary,
  PPNReconciliationResult,
  PPNReconciliationRecord,
  CreatePPNReconciliationInput,
  PPhFinalResult,
  CalculatePPhFinalInput,
  PayPPhFinalInput,
  CalculateECLResult,
  CalculateECLInput,
  WriteOffInput,
  WriteOffResult,
  CalculateDeferredTaxInput,
  CalculateDeferredTaxResult,
  ReportFrameworkRecord,
  SetFrameworkInput,
  Dimension,
  CreateDimensionInput,
  TagJournalLineInput,
  Budget,
  BudgetListItem,
  BudgetLineInput,
  CreateBudgetInput,
  BudgetVsActualResult,
  Attachment,
  AuditLog,
} from "./types";

/** Reconciliation detail returned by the reconcile endpoints. */
type Reconciliation = BankReconciliation;

const LATENCY_MS = 200;
const STORAGE_KEY = "ledgerly.m1.v1";
const TOKEN_KEY = "ledgerly.tokens";
const API_BASE = "/api/v1";

const MOCK_CATEGORIES: Category[] = [
  { id: "cat-sales", name: "Sales", kind: "money-in" },
  { id: "cat-receivables", name: "Receive receivables", kind: "money-in" },
  { id: "cat-capital", name: "Additional capital", kind: "money-in" },
  { id: "cat-loan", name: "Loan received", kind: "money-in" },
  { id: "cat-purchase", name: "Purchase of goods", kind: "money-out" },
  { id: "cat-materials", name: "Raw materials", kind: "money-out" },
  { id: "cat-rent", name: "Rent", kind: "money-out" },
  { id: "cat-utilities", name: "Electricity and water", kind: "money-out" },
  { id: "cat-salaries", name: "Employee salaries", kind: "money-out" },
  { id: "cat-other", name: "Other expenses", kind: "money-out" },
];

const MOCK_ACCOUNTS: AccountItem[] = [
  { id: "acc-cash", name: "Cash" },
  { id: "acc-bank", name: "BCA Bank" },
];

interface PersistedState {
  user: { id: string; email: string; businessName: string } | null;
  business: Business | null;
  transactions: Transaction[];
}

interface AuthResponse {
  access_token: string;
  refresh_token?: string;
  family_id?: string;
}

const nowIso = () => new Date().toISOString();

const today = () => {
  const d = new Date();
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  return `${d.getFullYear()}-${m}-${day}`;
};

function loadState(): PersistedState {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (raw) return JSON.parse(raw) as PersistedState;
  } catch {
    /* corrupt data -> start fresh */
  }
  return { user: null, business: null, transactions: [] };
}

function saveState(state: PersistedState): void {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(state));
}

function delay<T>(value: T): Promise<T> {
  return new Promise((resolve) => setTimeout(() => resolve(value), LATENCY_MS));
}

function makeError(code: string, message: string): ApiError {
  return { code, message };
}

function fakeId(prefix: string): string {
  return `${prefix}-${Math.random().toString(36).slice(2, 10)}`;
}

function fmtIDR(n: number): string {
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency: "IDR",
    maximumFractionDigits: 0,
  }).format(n);
}

/* ------------------------------------------------------------------ */
/* HTTP helper for the backend API                                     */
/* ------------------------------------------------------------------ */

interface HttpOptions extends RequestInit {
  /** Include Authorization: Bearer <access token>. */
  auth?: boolean;
  /** Include the Idempotency-Key header (UUID) — required for financial commands. */
  idempotencyKey?: string;
}

async function http<T>(path: string, options: HttpOptions = {}): Promise<T> {
  const { auth = false, idempotencyKey, ...rest } = options;
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...(rest.headers as Record<string, string> | undefined),
  };
  if (auth) {
    const token = getAccessToken();
    if (token) headers.Authorization = `Bearer ${token}`;
  }
  if (idempotencyKey) headers["Idempotency-Key"] = idempotencyKey;
  const response = await fetch(`${API_BASE}${path}`, { ...rest, headers });
  const body = await response.json().catch(() => ({}));
  if (!response.ok) {
    const code = (body as ApiError)?.code ?? "REQUEST_FAILED";
    const message = (body as ApiError)?.message ?? "Something went wrong. Please try again.";
    throw makeError(code, message);
  }
  return body as T;
}

/** UUID v4 for the Idempotency-Key header (crypto.randomUUID when available). */
function newIdempotencyKey(): string {
  if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
    return crypto.randomUUID();
  }
  // Fallback: manual UUID v4 so it works in older / non-secure browsers.
  return "xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx".replace(/[xy]/g, (ch) => {
    const random = Math.floor(Math.random() * 16);
    const value = ch === "x" ? random : (random & 0x3) | 0x8;
    return value.toString(16);
  });
}

function storeSession(tokens: AuthResponse): void {
  localStorage.setItem(TOKEN_KEY, JSON.stringify(tokens));
}

function getAccessToken(): string | null {
  try {
    const raw = localStorage.getItem(TOKEN_KEY);
    if (!raw) return null;
    return (JSON.parse(raw) as AuthResponse).access_token ?? null;
  } catch {
    return null;
  }
}

function readRefreshToken(): string | null {
  try {
    const raw = localStorage.getItem(TOKEN_KEY);
    if (!raw) return null;
    return (JSON.parse(raw) as AuthResponse).refresh_token ?? null;
  } catch {
    return null;
  }
}

/* ------------------------------------------------------------------ */
/* Backend response -> UI shape mapping                                */
/* ------------------------------------------------------------------ */

/** GET /accounts -> AccountItem {id, name}. Only active, detail accounts. */
function mapAccounts(raw: BackendAccount[]): AccountItem[] {
  return raw
    .filter((account) => account.is_active && !account.is_group)
    .map((account) => ({ id: String(account.id), name: account.name }));
}

/** GET /categories -> Category {id, name, kind}. */
function mapCategories(raw: BackendCategory[]): Category[] {
  return raw
    .filter((category) => category.is_active)
    .map((category) => ({
      id: String(category.id),
      name: category.name,
      kind: category.direction === "IN" ? "money-in" : "money-out",
    }));
}

/**
 * Backend account id (int64) from a UI string id; NaN when unreadable.
 * Default account codes come from the backend COA seed (see auth/seed.go).
 */
function accountId(raw: string | undefined): number {
  return Number(raw);
}

/** Backend COA seed account code for each onboarding balance type. */
const ACCOUNT_CODE_BY_BALANCE: Record<keyof OpeningBalance, string> = {
  cash: "1101",
  bank: "1102",
  receivables: "1201",
  payables: "2101",
  equity: "3101",
};

/** Equity account code used as the opening balance plug (Capital, 3101). */
const EQUITY_ACCOUNT_CODE = "3101";

/** Unique source_ref per command, e.g. "WEB-1752754093701". */
function newSourceRef(): string {
  return `WEB-${Date.now()}`;
}

/** Current month profit/loss from local transactions (offline fallback). */
function computeMonthlyProfitLossLocal(transactions: Transaction[]): number {
  const now = new Date();
  return transactions
    .filter((trx) => {
      const [y, m] = trx.date.split("-").map(Number);
      return y === now.getFullYear() && m === now.getMonth() + 1 && trx.kind !== "transfer";
    })
    .reduce((acc, trx) => acc + (trx.kind === "money-in" ? trx.amount : -trx.amount), 0);
}

/** Builds the POST /cash-in or /cash-out payload. */
function buildCashCommand(input: TransactionInput, cashAccountId: number, counterAccountId: number): CashCommandPayload {
  return {
    source_ref: newSourceRef(),
    entry_date: input.date,
    cash_account_id: cashAccountId,
    counter_account_id: counterAccountId,
    amount_cents: Math.round(input.amount * 100),
    description: input.description.trim(),
  };
}

/** Builds the POST /transfers payload. */
function buildTransferCommand(input: TransactionInput, fromAccountId: number, toAccountId: number): TransferCommandPayload {
  return {
    source_ref: newSourceRef(),
    entry_date: input.date,
    from_account_id: fromAccountId,
    to_account_id: toAccountId,
    amount_cents: Math.round(input.amount * 100),
    description: input.description.trim(),
  };
}

/** Simple slug from a business name, e.g. "Warung Bu Sari" -> "warung-bu-sari". */
function slugify(value: string): string {
  return value
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, 80);
}

/** Builds the POST /opening-balances payload from onboarding opening balance. */
async function buildOpeningBalance(input: OnboardingInput): Promise<OpeningBalancePayload | null> {
  const { cash, bank, receivables, payables, equity } = input.openingBalance;
  const toCents = (value: number) => Math.round(value * 100);
  const balances: OpeningBalancePayload["balances"] = [];
  if (cash > 0) balances.push({ account_id: 0, debit_cents: toCents(cash), credit_cents: 0 });
  if (bank > 0) balances.push({ account_id: 0, debit_cents: toCents(bank), credit_cents: 0 });
  if (receivables > 0) balances.push({ account_id: 0, debit_cents: toCents(receivables), credit_cents: 0 });
  if (payables > 0) balances.push({ account_id: 0, credit_cents: toCents(payables), debit_cents: 0 });
  if (equity > 0) balances.push({ account_id: 0, credit_cents: toCents(equity), debit_cents: 0 });
  if (balances.length === 0) return null;

  // Account ids are not known during onboarding (payload is built before the
  // tenant exists), so look them up from the backend COA seed
  // (codes 1101/1102/1201/2101/3101).
  let accounts: BackendAccount[] = [];
  try {
    accounts = await http<BackendAccount[]>("/accounts", { auth: true });
  } catch {
    // Network failure -> onboarding still finishes; opening balance is skipped
    // (completeOnboarding has its own fallback for posting failures).
  }
  const idByCode = new Map(accounts.map((account) => [account.code, account.id]));
  const idOf = (code: string): number => idByCode.get(code) ?? 0;

  for (const line of balances) {
    // Balance lines are ordered: cash, bank, receivables, payables, equity.
    const codes = Object.values(ACCOUNT_CODE_BY_BALANCE);
    const idx = balances.indexOf(line);
    line.account_id = idOf(codes[idx] ?? "");
  }

  return {
    source_ref: newSourceRef(),
    entry_date: `${input.period.year}-${String(input.period.startMonth).padStart(2, "0")}-01`,
    equity_account_id: idOf(EQUITY_ACCOUNT_CODE),
    balances,
    description: "Onboarding opening balance",
  };
}

/* ------------------------------------------------------------------ */
/* API implementation (real auth; other endpoints fall back to mock)   */
/* ------------------------------------------------------------------ */

function buildReportQuery(fromDate?: string, toDate?: string): string {
  const params = new URLSearchParams();
  if (fromDate) params.set("from", fromDate);
  if (toDate) params.set("to", toDate);
  const qs = params.toString();
  return qs ? `?${qs}` : "";
}

export const api = {
  /** Registers a new user on the backend and opens a local session. */
  async register(input: RegisterInput): Promise<{ user: { id: string; email: string; businessName: string } }> {
    const email = input.email.trim().toLowerCase();
    if (!email || !input.password || !input.businessName.trim()) {
      throw makeError("VALIDATION_ERROR", "Please fill in email, password, and business name.");
    }
    if (input.password.length < 8) {
      throw makeError("VALIDATION_ERROR", "Password must be at least 8 characters.");
    }
    const response = await http<AuthResponse>("/auth/register", {
      method: "POST",
      body: JSON.stringify({ email, password: input.password, full_name: input.businessName.trim() }),
    });
    storeSession(response);
    const user = { id: "usr-" + (response.family_id ?? Date.now()), email, businessName: input.businessName.trim() };
    saveState({ ...loadState(), user });
    return delay({ user });
  },

  /** Opens a session via the backend; stores access + refresh tokens. */
  async login(input: LoginInput): Promise<{ user: { id: string; email: string; businessName: string } }> {
    const email = input.email.trim().toLowerCase();
    if (!email || !input.password) {
      throw makeError("VALIDATION_ERROR", "Please enter your email and password.");
    }
    const response = await http<AuthResponse>("/auth/login", {
      method: "POST",
      body: JSON.stringify({ email, password: input.password }),
    });
    storeSession(response);
    const user = {
      id: "usr-" + (response.family_id ?? Date.now()),
      email,
      businessName: email.split("@")[0] || "My business",
    };
    saveState({ ...loadState(), user });
    return delay({ user });
  },

  /** Closes the session: revokes the refresh token on the backend + clears local data. */
  async logout(): Promise<void> {
    const refreshToken = readRefreshToken();
    if (refreshToken) {
      await fetch(`${API_BASE}/auth/logout`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ refresh_token: refreshToken }),
      }).catch(() => undefined);
      localStorage.removeItem(TOKEN_KEY);
    }
    const state = loadState();
    saveState({ ...state, user: null });
    return delay(undefined);
  },

  /**
   * Dashboard summary from backend reports (profit-loss, balance-sheet,
   * cash-flow). Network failure -> computed from local transactions (mock).
   */
  async getDashboard(): Promise<DashboardSummary> {
    const state = loadState();
    const sorted = [...state.transactions].sort((a, b) => b.date.localeCompare(a.date));
    const fallback = (): DashboardSummary => ({
      cashAndBankBalance: sorted.reduce(
        (acc, trx) =>
          trx.kind === "money-in"
            ? acc + trx.amount
            : trx.kind === "money-out"
              ? acc - trx.amount
              : acc,
        0,
      ),
      monthlyProfitLoss: computeMonthlyProfitLossLocal(sorted),
      dueBills: 2,
      lowStock: 4,
      recentTransactions: sorted.slice(0, 8),
    });
    try {
      const [profitLoss, cashFlow, balanceSheet] = await Promise.all([
        http<BackendProfitLoss>("/reports/profit-loss", { auth: true }),
        http<BackendCashFlow>("/reports/cash-flow", { auth: true }),
        http<BackendBalanceSheet>("/reports/balance-sheet", { auth: true }),
      ]);
      const cashAndBankBalance = Math.round(cashFlow.net_cash_flow_cents / 100);
      const monthlyProfitLoss = Math.round(profitLoss.profit_cents / 100);
      if (
        Number.isFinite(cashAndBankBalance) &&
        Number.isFinite(monthlyProfitLoss) &&
        typeof balanceSheet.asset_cents === "number" &&
        typeof balanceSheet.liability_cents === "number"
      ) {
        return delay({
          cashAndBankBalance,
          monthlyProfitLoss,
          dueBills: 2,
          lowStock: 4,
          recentTransactions: sorted.slice(0, 8),
        });
      }
      // Backend response not as expected -> use local data.
      return fallback();
    } catch {
      return fallback();
    }
  },

  /**
   * Closes the current book period (POST /periods/close).
   * Explicit command: failures are thrown as ApiError, no mock fallback.
   */
  async closePeriod(): Promise<PeriodResult> {
    return http<PeriodResult>("/periods/close", {
      method: "POST",
      auth: true,
      idempotencyKey: newIdempotencyKey(),
      body: JSON.stringify({}),
    });
  },

  /**
   * Reopens a closed period (POST /periods/unlock).
   * Explicit command: failures are thrown as ApiError, no mock fallback.
   */
  async unlockPeriod(): Promise<PeriodResult> {
    return http<PeriodResult>("/periods/unlock", {
      method: "POST",
      auth: true,
      idempotencyKey: newIdempotencyKey(),
      body: JSON.stringify({}),
    });
  },

  /**
   * Records a transaction on the backend:
   *   money-in  -> POST /cash-in
   *   money-out -> POST /cash-out
   *   transfer  -> POST /transfers
   * Each command uses an Idempotency-Key UUID. Network failure -> save
   * locally (mock) as graceful degradation.
   */
  async createTransaction(input: TransactionInput): Promise<{ transaction: Transaction; recentTransactions: Transaction[] }> {
    if (!input.amount || input.amount <= 0) {
      throw makeError("VALIDATION_ERROR", "Amount must be greater than zero.");
    }
    if (!input.date) {
      throw makeError("VALIDATION_ERROR", "Please pick a transaction date.");
    }
    if (input.kind === "transfer" && (!input.from || !input.to || input.from === input.to)) {
      throw makeError("VALIDATION_ERROR", "Pick two different accounts to transfer money.");
    }
    const state = loadState();
    const category = input.categoryId ? MOCK_CATEGORIES.find((c) => c.id === input.categoryId) : undefined;
    const transaction: Transaction = {
      id: fakeId("trx"),
      kind: input.kind,
      amount: input.amount,
      date: input.date,
      description: input.description.trim(),
      categoryId: category?.id,
      categoryName: category?.name,
      from: input.from,
      to: input.to,
      createdAt: nowIso(),
    };
    const saveLocal = (): { transaction: Transaction; recentTransactions: Transaction[] } => {
      const transactions = [transaction, ...state.transactions];
      saveState({ ...state, transactions });
      const sorted = [...transactions].sort((a, b) => b.date.localeCompare(a.date));
      return { transaction, recentTransactions: sorted.slice(0, 8) };
    };
    try {
      if (input.kind === "transfer") {
        const payload = buildTransferCommand(input, accountId(input.from), accountId(input.to));
        if (!Number.isFinite(payload.from_account_id) || !Number.isFinite(payload.to_account_id)) {
          throw makeError("VALIDATION_ERROR", "Pick a valid source and destination account.");
        }
        await http<BackendJournalResult>("/transfers", {
          method: "POST",
          auth: true,
          idempotencyKey: newIdempotencyKey(),
          body: JSON.stringify(payload),
        });
      } else {
        const categoryId = accountId(input.categoryId);
        const cashAccountId = accountId(input.kind === "money-in" ? input.categoryId : input.from);
        const counterAccountId = accountId(input.kind === "money-in" ? input.from : input.categoryId);
        if (!Number.isFinite(cashAccountId) || !Number.isFinite(counterAccountId)) {
          throw makeError("VALIDATION_ERROR", "Pick a valid category and account.");
        }
        const payload = buildCashCommand(input, cashAccountId, counterAccountId);
        await http<BackendJournalResult>(input.kind === "money-in" ? "/cash-in" : "/cash-out", {
          method: "POST",
          auth: true,
          idempotencyKey: newIdempotencyKey(),
          body: JSON.stringify(payload),
        });
        if (Number.isFinite(categoryId)) {
          transaction.categoryId = String(categoryId);
        }
      }
      // Saved on the backend: keep as local cache so offline stays complete.
      return saveLocal();
    } catch {
      return delay(saveLocal());
    }
  },

  /**
   * Posts a CASH_IN journal via the backend. Used by the workbench
   * entry form when the user presses Post on an Other Receipt draft.
   * When `counter_lines` is provided the backend splits the credit side
   * across the listed accounts; otherwise `counter_account_id` is used.
   */
  async postCashIn(payload: {
    entry_date: string;
    description: string;
    cash_account_id: number;
    counter_account_id: number;
    amount_cents: number;
    counter_lines?: import("./types").CounterLinePayload[];
  }): Promise<BackendJournalResult> {
    const body: Record<string, unknown> = {
      source_ref: `WEB-${Date.now()}`,
      entry_date: payload.entry_date,
      cash_account_id: payload.cash_account_id,
      amount_cents: payload.amount_cents,
      description: payload.description,
    };
    if (payload.counter_lines && payload.counter_lines.length > 0) {
      body.counter_lines = payload.counter_lines;
    } else {
      body.counter_account_id = payload.counter_account_id;
    }
    return http<BackendJournalResult>("/cash-in", {
      method: "POST",
      auth: true,
      idempotencyKey: newIdempotencyKey(),
      body: JSON.stringify(body),
    });
  },

  /** Posts a CASH_OUT journal via the backend. */
  async postCashOut(payload: {
    entry_date: string;
    description: string;
    cash_account_id: number;
    counter_account_id: number;
    amount_cents: number;
    counter_lines?: import("./types").CounterLinePayload[];
  }): Promise<BackendJournalResult> {
    const body: Record<string, unknown> = {
      source_ref: `WEB-${Date.now()}`,
      entry_date: payload.entry_date,
      cash_account_id: payload.cash_account_id,
      amount_cents: payload.amount_cents,
      description: payload.description,
    };
    if (payload.counter_lines && payload.counter_lines.length > 0) {
      body.counter_lines = payload.counter_lines;
    } else {
      body.counter_account_id = payload.counter_account_id;
    }
    return http<BackendJournalResult>("/cash-out", {
      method: "POST",
      auth: true,
      idempotencyKey: newIdempotencyKey(),
      body: JSON.stringify(body),
    });
  },

  /** Posts a TRANSFER journal via the backend. */
  async postTransfer(payload: {
    entry_date: string;
    description: string;
    from_account_id: number;
    to_account_id: number;
    amount_cents: number;
  }): Promise<BackendJournalResult> {
    return http<BackendJournalResult>("/transfers", {
      method: "POST",
      auth: true,
      idempotencyKey: newIdempotencyKey(),
      body: JSON.stringify({
        source_ref: `WEB-${Date.now()}`,
        entry_date: payload.entry_date,
        from_account_id: payload.from_account_id,
        to_account_id: payload.to_account_id,
        amount_cents: payload.amount_cents,
        description: payload.description,
      }),
    });
  },

  /** Reverses a posted journal via POST /journal-entries/{id}/reverse. */
  async reverseCash(journalId: number): Promise<BackendJournalResult> {
    return http<BackendJournalResult>(`/journal-entries/${journalId}/reverse`, {
      method: "POST",
      auth: true,
      idempotencyKey: newIdempotencyKey(),
      body: JSON.stringify({
        source_ref: `REV-${Date.now()}`,
        entry_date: mockHelpers.today(),
      }),
    });
  },

  /** Category list from GET /categories (Bearer). Failure -> local mock. */
  async listCategories(): Promise<Category[]> {
    try {
      const raw = await http<BackendCategory[]>("/categories", { auth: true });
      const mapped = mapCategories(raw);
      return delay(mapped.length > 0 ? mapped : MOCK_CATEGORIES);
    } catch {
      return delay(MOCK_CATEGORIES);
    }
  },

  /** Account list from GET /accounts (Bearer). Failure -> local mock. */
  async listAccounts(): Promise<AccountItem[]> {
    try {
      const raw = await http<BackendAccount[]>("/accounts", { auth: true });
      const mapped = mapAccounts(raw);
      return delay(mapped.length > 0 ? mapped : MOCK_ACCOUNTS);
    } catch {
      return delay(MOCK_ACCOUNTS);
    }
  },

  /** Active non-group revenue/expense accounts, for budget line pickers. */
  async listBudgetAccounts(): Promise<BackendAccount[]> {
    try {
      const raw = await http<BackendAccount[]>("/accounts", { auth: true });
      return raw.filter((a) => !a.is_group && a.is_active && (a.report_group === "revenue" || a.report_group === "expense"));
    } catch {
      return [];
    }
  },

  /**
   * Cash & bank history list from GET /api/v1/cash-entries (Bearer).
   * Returns the unified list of CASH_IN, CASH_OUT, and TRANSFER journals
   * with resolved account names. Failure -> empty list (graceful degradation).
   */
  async listCashEntries(params: import("./types").ListCashEntriesParams = {}): Promise<CashEntryListItem[]> {
    const search = new URLSearchParams();
    if (params.kind) search.set("kind", params.kind);
    if (params.from) search.set("from", params.from);
    if (params.to) search.set("to", params.to);
    if (params.account_id) search.set("account_id", String(params.account_id));
    if (params.q) search.set("q", params.q);
    if (params.limit) search.set("limit", String(params.limit));
    if (params.offset) search.set("offset", String(params.offset));
    const query = search.toString();
    try {
      const response = await http<import("./types").CashEntryListResponse>(
        `/cash-entries${query ? `?${query}` : ""}`,
        { auth: true },
      );
      return delay(response.items ?? []);
    } catch {
      return delay([]);
    }
  },

  /* ----------------------- Accountant Mode v1 ----------------------- */

  /**
   * Lists raw backend accounts (GET /accounts) for the account pickers used
   * by the manual journal and general ledger screens. Falls back to an empty
   * list on failure so the picker still renders.
   */
  async listBackendAccounts(): Promise<BackendAccount[]> {
    try {
      return await http<BackendAccount[]>("/accounts", { auth: true });
    } catch {
      return [];
    }
  },

  /** Creates a manual journal entry (POST /journal-entries). */
  async createManualJournal(payload: ManualJournalInput): Promise<BackendJournalResult> {
    return http<BackendJournalResult>("/journal-entries", {
      method: "POST",
      auth: true,
      idempotencyKey: newIdempotencyKey(),
      body: JSON.stringify(payload),
    });
  },

  /** Lists journal entries (GET /journal-entries). */
  async listJournalEntries(params: {
    from_date?: string;
    to_date?: string;
    account_id?: number;
  } = {}): Promise<JournalEntryListItem[]> {
    const search = new URLSearchParams();
    if (params.from_date) search.set("from_date", params.from_date);
    if (params.to_date) search.set("to_date", params.to_date);
    if (params.account_id) search.set("account_id", String(params.account_id));
    const query = search.toString();
    try {
      const response = await http<{ items: JournalEntryListItem[] }>(
        `/journal-entries${query ? `?${query}` : ""}`,
        { auth: true },
      );
      return response.items ?? [];
    } catch {
      return [];
    }
  },

  /** Gets one journal entry with its lines (GET /journal-entries/{id}). */
  async getJournalEntry(id: number): Promise<JournalEntry | null> {
    try {
      return await http<JournalEntry>(`/journal-entries/${id}`, { auth: true });
    } catch {
      return null;
    }
  },

  /** General ledger for one account (GET /general-ledger?account_id=...). */
  async getGeneralLedger(params: {
    account_id: number;
    from_date?: string;
    to_date?: string;
  }): Promise<GeneralLedgerResult | null> {
    const search = new URLSearchParams();
    search.set("account_id", String(params.account_id));
    if (params.from_date) search.set("from_date", params.from_date);
    if (params.to_date) search.set("to_date", params.to_date);
    try {
      return await http<GeneralLedgerResult>(`/general-ledger?${search.toString()}`, { auth: true });
    } catch {
      return null;
    }
  },

  /** Journal register (GET /journal-register). */
  async getJournalRegister(params: {
    from_date?: string;
    to_date?: string;
    intent_type?: string;
  } = {}): Promise<JournalRegisterItem[]> {
    const search = new URLSearchParams();
    if (params.from_date) search.set("from_date", params.from_date);
    if (params.to_date) search.set("to_date", params.to_date);
    if (params.intent_type) search.set("intent_type", params.intent_type);
    const query = search.toString();
    try {
      return await http<JournalRegisterItem[]>(
        `/journal-register${query ? `?${query}` : ""}`,
        { auth: true },
      );
    } catch {
      return [];
    }
  },

  /** Trial balance report (GET /reports/trial-balance). */
  async getTrialBalance(fromDate?: string, toDate?: string): Promise<BackendTrialBalance> {
    const qs = buildReportQuery(fromDate, toDate);
    return http<BackendTrialBalance>(`/reports/trial-balance${qs}`, { auth: true });
  },

  /** Profit & Loss report (GET /reports/profit-loss). */
  async getProfitLoss(
    fromDate?: string,
    toDate?: string,
    framework?: string,
    dimensionId?: number,
  ): Promise<BackendProfitLoss> {
    let qs = buildReportQuery(fromDate, toDate);
    if (framework) qs += (qs ? "&" : "?") + `framework=${encodeURIComponent(framework)}`;
    if (dimensionId && dimensionId > 0) qs += (qs ? "&" : "?") + `dimension_id=${dimensionId}`;
    return http<BackendProfitLoss>(`/reports/profit-loss${qs}`, { auth: true });
  },

  /** Balance Sheet report (GET /reports/balance-sheet). */
  async getBalanceSheet(fromDate?: string, toDate?: string): Promise<BackendBalanceSheet> {
    const qs = buildReportQuery(fromDate, toDate);
    return http<BackendBalanceSheet>(`/reports/balance-sheet${qs}`, { auth: true });
  },

  /** Cash Flow report (GET /reports/cash-flow). */
  async getCashFlow(fromDate?: string, toDate?: string): Promise<BackendCashFlow> {
    const qs = buildReportQuery(fromDate, toDate);
    return http<BackendCashFlow>(`/reports/cash-flow${qs}`, { auth: true });
  },

  /**
   * Export a report as PDF or Excel (GET /reports/{reportType}/export).
   * Returns a Blob suitable for download. Uses a raw fetch (not the shared
   * http() helper) because http() parses JSON; exports are binary.
   */
  async exportReport(reportType: string, format: "pdf" | "xlsx", fromDate?: string, toDate?: string): Promise<Blob> {
    const qs = new URLSearchParams();
    qs.set("format", format);
    if (fromDate) qs.set("from_date", fromDate);
    if (toDate) qs.set("to_date", toDate);
    const response = await fetch(`${API_BASE}/reports/${reportType}/export?${qs}`, {
      headers: { Authorization: `Bearer ${getAccessToken()}` },
    });
    if (!response.ok) {
      throw makeError("EXPORT_FAILED", `Export failed (${response.status})`);
    }
    return response.blob();
  },

  /* ----------------------- Financial Notes (P2-016) ----------------------- */

  /** Lists financial notes (GET /financial-notes?period_year=). Failure -> empty array. */
  async listFinancialNotes(periodYear?: number): Promise<FinancialNote[]> {
    const query = periodYear ? `?period_year=${periodYear}` : "";
    try {
      return await http<FinancialNote[]>(`/financial-notes${query}`, { auth: true });
    } catch {
      return [];
    }
  },

  /** Gets one financial note (GET /financial-notes/{id}). */
  async getFinancialNote(id: number): Promise<FinancialNote> {
    return http<FinancialNote>(`/financial-notes/${id}`, { auth: true });
  },

  /** Creates a financial note (POST /financial-notes). */
  async createFinancialNote(input: FinancialNoteInput): Promise<FinancialNote> {
    return http<FinancialNote>("/financial-notes", {
      method: "POST",
      auth: true,
      idempotencyKey: newIdempotencyKey(),
      body: JSON.stringify(input),
    });
  },

  /** Updates a financial note (PUT /financial-notes/{id}). */
  async updateFinancialNote(id: number, input: FinancialNoteInput): Promise<FinancialNote> {
    return http<FinancialNote>(`/financial-notes/${id}`, {
      method: "PUT",
      auth: true,
      body: JSON.stringify(input),
    });
  },

  /** Deletes a financial note (DELETE /financial-notes/{id}). */
  async deleteFinancialNote(id: number): Promise<{ id: number; deleted: boolean }> {
    return http<{ id: number; deleted: boolean }>(`/financial-notes/${id}`, {
      method: "DELETE",
      auth: true,
    });
  },

  /* ----------------------- Due Date Reminders (P2-016) ----------------------- */

  /** Lists customer + supplier invoices due within days_ahead days (GET /reminders/due-dates). */
  async listDueDateReminders(daysAhead: number = 7): Promise<DueDateReminder[]> {
    try {
      return await http<DueDateReminder[]>(`/reminders/due-dates?days_ahead=${daysAhead}`, { auth: true });
    } catch {
      return [];
    }
  },

  /* ----------------------- Fixed Assets (US-060..063) ----------------------- */

  /** Lists the asset register (GET /fixed-assets). Failure -> empty array. */
  async listFixedAssets(): Promise<FixedAssetListItem[]> {
    try {
      return await http<FixedAssetListItem[]>("/fixed-assets", { auth: true });
    } catch {
      return [];
    }
  },

  /* ----------------------------- Tax (US-080..083) ----------------------------- */

  /** PPN summary across a date range (GET /ppn/summary). */
  async getPPNSummary(fromDate?: string, toDate?: string): Promise<PPNSummary | null> {
    const params = new URLSearchParams();
    if (fromDate) params.set("from_date", fromDate);
    if (toDate) params.set("to_date", toDate);
    const query = params.toString() ? `?${params}` : "";
    try {
      return await http<PPNSummary>(`/ppn/summary${query}`, { auth: true });
    } catch {
      return null;
    }
  },

  /** Detailed PPN reconciliation for a calendar month (GET /ppn/reconciliation). */
  async getPPNReconciliation(periodYear: number, periodMonth: number): Promise<PPNReconciliationResult | null> {
    try {
      return await http<PPNReconciliationResult>(
        `/ppn/reconciliation?period_year=${periodYear}&period_month=${periodMonth}`,
        { auth: true },
      );
    } catch {
      return null;
    }
  },

  /** Files (or re-files) the PPN reconciliation for a month (POST /ppn/reconcile). */
  async createPPNReconciliation(input: CreatePPNReconciliationInput): Promise<PPNReconciliationRecord> {
    return http<PPNReconciliationRecord>("/ppn/reconcile", {
      method: "POST",
      auth: true,
      idempotencyKey: newIdempotencyKey(),
      body: JSON.stringify(input),
    });
  },

  /** Calculates and posts the PPh Final UMKM provision (POST /pph-final/calculate). */
  async calculatePPhFinal(input: CalculatePPhFinalInput): Promise<PPhFinalResult> {
    return http<PPhFinalResult>("/pph-final/calculate", {
      method: "POST",
      auth: true,
      idempotencyKey: newIdempotencyKey(),
      body: JSON.stringify(input),
    });
  },

  /** Records the PPh Final tax payment (POST /pph-final/pay). */
  async payPPhFinal(input: PayPPhFinalInput): Promise<PPhFinalResult> {
    return http<PPhFinalResult>("/pph-final/pay", {
      method: "POST",
      auth: true,
      idempotencyKey: newIdempotencyKey(),
      body: JSON.stringify(input),
    });
  },

  /** Lists cash & bank accounts for the tax-payment account picker. */
  async listCashAccounts(): Promise<{ id: number; code: string; name: string; account_type: string }[]> {
    try {
      const raw = await http<BackendAccount[]>("/accounts", { auth: true });
      return raw
        .filter((a) => !a.is_group && a.account_type && (a.account_type === "CASH" || a.account_type === "BANK"))
        .map((a) => ({ id: a.id, code: a.code, name: a.name, account_type: a.account_type }));
    } catch {
      return [];
    }
  },

  /** Gets one fixed asset with schedule + transactions (GET /fixed-assets/{id}). */
  async getFixedAsset(id: number): Promise<FixedAsset> {
    return http<FixedAsset>(`/fixed-assets/${id}`, { auth: true });
  },

  /** Registers a new fixed asset (POST /fixed-assets). Posts Dr 1401 / Cr Cash. */
  async registerFixedAsset(input: RegisterFixedAssetInput): Promise<FixedAsset> {
    return http<FixedAsset>("/fixed-assets", {
      method: "POST",
      auth: true,
      body: JSON.stringify(input),
    });
  },

  /** Calculates and posts the ECL provision (POST /ecl/calculate). */
  async calculateECL(input: CalculateECLInput): Promise<CalculateECLResult> {
    return http<CalculateECLResult>("/ecl/calculate", {
      method: "POST",
      auth: true,
      idempotencyKey: newIdempotencyKey(),
      body: JSON.stringify(input),
    });
  },

  /** Posts depreciation for a period (POST /fixed-assets/{id}/depreciate). Dr 5206 / Cr 1402. */
  async depreciateAsset(id: number, input: DepreciateAssetInput): Promise<DepreciationResult> {
    return http<DepreciationResult>(`/fixed-assets/${id}/depreciate`, {
      method: "POST",
      auth: true,
      idempotencyKey: newIdempotencyKey(),
      body: JSON.stringify(input),
    });
  },

  /** Writes off a specific receivable (POST /ecl/write-off). */
  async writeOffReceivable(input: WriteOffInput): Promise<WriteOffResult> {
    return http<WriteOffResult>("/ecl/write-off", {
      method: "POST",
      auth: true,
      idempotencyKey: newIdempotencyKey(),
      body: JSON.stringify(input),
    });
  },

  /** Revalues an asset to OCI (POST /fixed-assets/{id}/revalue). Dr/Cr 1401 vs 3401. */
  async revalueAsset(id: number, input: RevalueAssetInput): Promise<{ journal_entry_id?: number; adjustment_cents: number; direction: string }> {
    return http(`/fixed-assets/${id}/revalue`, {
      method: "POST",
      auth: true,
      idempotencyKey: newIdempotencyKey(),
      body: JSON.stringify(input),
    });
  },

  /** Disposes/sells an asset (POST /fixed-assets/{id}/dispose). Sets status DISPOSED. */
  async disposeAsset(id: number, input: DisposeAssetInput): Promise<AssetDisposalResult> {
    return http<AssetDisposalResult>(`/fixed-assets/${id}/dispose`, {
      method: "POST",
      auth: true,
      idempotencyKey: newIdempotencyKey(),
      body: JSON.stringify(input),
    });
  },

  /** Records an impairment (POST /fixed-assets/{id}/impair). Dr 5207 / Cr 1401. */
  async impairAsset(id: number, input: ImpairAssetInput): Promise<{ journal_entry_id?: number; impairment_loss_cents: number; new_book_value_cents: number }> {
    return http(`/fixed-assets/${id}/impair`, {
      method: "POST",
      auth: true,
      idempotencyKey: newIdempotencyKey(),
      body: JSON.stringify(input),
    });
  },

  /** Calculates and posts the deferred tax movement (POST /deferred-tax/calculate). */
  async calculateDeferredTax(input: CalculateDeferredTaxInput): Promise<CalculateDeferredTaxResult> {
    return http<CalculateDeferredTaxResult>("/deferred-tax/calculate", {
      method: "POST",
      auth: true,
      idempotencyKey: newIdempotencyKey(),
      body: JSON.stringify(input),
    });
  },

  /* ----------------------- Attachments (US-100) ----------------------- */

  /**
   * Uploads a proof attachment (POST /attachments, multipart). owner_type +
   * owner_id identify the entity the file belongs to. Returns metadata.
   */
  async uploadAttachment(file: File, ownerType: string, ownerId: number): Promise<Attachment> {
    const form = new FormData();
    form.append("file", file);
    form.append("owner_type", ownerType);
    form.append("owner_id", String(ownerId));
    const response = await fetch(`${API_BASE}/attachments`, {
      method: "POST",
      headers: { Authorization: `Bearer ${getAccessToken()}` },
      body: form,
    });
    const body = await response.json().catch(() => ({}));
    if (!response.ok) {
      throw makeError((body as ApiError)?.code ?? "UPLOAD_FAILED", (body as ApiError)?.message ?? "Upload failed.");
    }
    return body as Attachment;
  },

  /** Lists attachments for an entity (GET /attachments?owner_type=&owner_id=). Failure -> empty array. */
  async listAttachments(ownerType: string, ownerId: number): Promise<Attachment[]> {
    try {
      return await http<Attachment[]>(`/attachments?owner_type=${encodeURIComponent(ownerType)}&owner_id=${ownerId}`, { auth: true });
    } catch {
      return [];
    }
  },

  /** Returns the attachment file as a Blob (GET /attachments/{id}/download, Bearer auth). */
  async downloadAttachment(id: number, fallbackFileName?: string): Promise<Blob> {
    const response = await fetch(`${API_BASE}/attachments/${id}/download`, {
      headers: { Authorization: `Bearer ${getAccessToken()}` },
    });
    if (!response.ok) {
      throw makeError("DOWNLOAD_FAILED", `Download failed (${response.status})`);
    }
    return response.blob();
  },

  /** Deletes an attachment (DELETE /attachments/{id}). Records an audit log. */
  async deleteAttachment(id: number): Promise<{ id: number; deleted: boolean }> {
    return http<{ id: number; deleted: boolean }>(`/attachments/${id}`, {
      method: "DELETE",
      auth: true,
    });
  },

  /* ----------------------- Audit Trail (US-101) ----------------------- */

  /**
   * Lists audit logs (GET /audit-logs) with optional filters:
   * entity_type, entity_id, user_id, from_date, to_date. Failure -> empty array.
   */
  async listAuditLogs(params: {
    entity_type?: string;
    entity_id?: number;
    user_id?: number;
    from_date?: string;
    to_date?: string;
  } = {}): Promise<AuditLog[]> {
    const search = new URLSearchParams();
    if (params.entity_type) search.set("entity_type", params.entity_type);
    if (params.entity_id) search.set("entity_id", String(params.entity_id));
    if (params.user_id) search.set("user_id", String(params.user_id));
    if (params.from_date) search.set("from_date", params.from_date);
    if (params.to_date) search.set("to_date", params.to_date);
    const query = search.toString();
    try {
      return await http<AuditLog[]>(`/audit-logs${query ? `?${query}` : ""}`, { auth: true });
    } catch {
      return [];
    }
  },

  /** Customer list (GET /customers). Failure -> empty array. */
  async listCustomers(): Promise<Customer[]> {
    try {
      return await http<Customer[]>("/customers", { auth: true });
    } catch {
      return [];
    }
  },

  /** Item list (GET /items). Failure -> empty array. */
  async listItems(): Promise<Item[]> {
    try {
      return await http<Item[]>("/items", { auth: true });
    } catch {
      return [];
    }
  },

  /** Quotation list (GET /quotations). Optional status filter. Failure -> empty array. */
  async listQuotations(status?: QuotationListItem["status"]): Promise<QuotationListItem[]> {
    const query = status ? `?status=${status}` : "";
    try {
      return await http<QuotationListItem[]>(`/quotations${query}`, { auth: true });
    } catch {
      return [];
    }
  },

  /** Get one quotation with lines (GET /quotations/{id}). */
  async getQuotation(id: number): Promise<Quotation> {
    return http<Quotation>(`/quotations/${id}`, { auth: true });
  },

  /** Create a quotation (POST /quotations). SQ posts no journal. */
  async createQuotation(input: QuotationCreateInput): Promise<Quotation & { id: number; number: string }> {
    return http<Quotation & { id: number; number: string }>("/quotations", {
      method: "POST",
      auth: true,
      idempotencyKey: newIdempotencyKey(),
      body: JSON.stringify({
        ...input,
        source_ref: input.source_ref ?? `WEB-${Date.now()}`,
        quotation_date: input.quotation_date || mockHelpers.today(),
      }),
    });
  },

  /** Send a quotation (DRAFT -> SENT). */
  async sendQuotation(id: number): Promise<{ id: number; status: string }> {
    return http<{ id: number; status: string }>(`/quotations/${id}/send`, { method: "POST", auth: true });
  },

  /** Cancel a quotation (DRAFT/SENT -> CANCELLED). */
  async cancelQuotation(id: number): Promise<{ id: number; status: string }> {
    return http<{ id: number; status: string }>(`/quotations/${id}/cancel`, { method: "POST", auth: true });
  },

  /** Sales order list (GET /sales-orders). Optional status filter. */
  async listSalesOrders(status?: SalesOrderListItem["status"]): Promise<SalesOrderListItem[]> {
    const query = status ? `?status=${status}` : "";
    try {
      return await http<SalesOrderListItem[]>(`/sales-orders${query}`, { auth: true });
    } catch {
      return [];
    }
  },

  /** Get one sales order with lines and down payments. */
  async getSalesOrder(id: number): Promise<SalesOrder> {
    return http<SalesOrder>(`/sales-orders/${id}`, { auth: true });
  },

  /** Create a sales order (POST /sales-orders). SO posts no journal. */
  async createSalesOrder(input: SalesOrderCreateInput): Promise<SalesOrder> {
    return http<SalesOrder>("/sales-orders", {
      method: "POST",
      auth: true,
      idempotencyKey: newIdempotencyKey(),
      body: JSON.stringify(input),
    });
  },

  /** Cancel a sales order (CONFIRMED -> CANCELLED, only when no DP). */
  async cancelSalesOrder(id: number): Promise<{ id: number; status: string }> {
    return http<{ id: number; status: string }>(`/sales-orders/${id}/cancel`, { method: "POST", auth: true });
  },

  /** Create a down payment for a sales order (POST /sales-orders/{id}/down-payments). */
  async createDownPayment(orderId: number, input: CreateDownPaymentInput): Promise<DownPayment> {
    return http<DownPayment>(`/sales-orders/${orderId}/down-payments`, {
      method: "POST",
      auth: true,
      idempotencyKey: newIdempotencyKey(),
      body: JSON.stringify(input),
    });
  },

  /** List down payments for a sales order. */
  async listDownPayments(orderId: number): Promise<DownPayment[]> {
    try {
      return await http<DownPayment[]>(`/sales-orders/${orderId}/down-payments`, { auth: true });
    } catch {
      return [];
    }
  },

  /** Refund a down payment (POST /down-payments/{id}/refund). */
  async refundDownPayment(dpId: number): Promise<{ dp_id: number; status: string }> {
    return http<{ dp_id: number; status: string }>(`/down-payments/${dpId}/refund`, {
      method: "POST",
      auth: true,
      idempotencyKey: newIdempotencyKey(),
    });
  },

  /** Delivery order list (GET /delivery-orders). Optional status filter. */
  async listDeliveryOrders(status?: DeliveryOrderListItem["status"]): Promise<DeliveryOrderListItem[]> {
    const query = status ? `?status=${status}` : "";
    try {
      return await http<DeliveryOrderListItem[]>(`/delivery-orders${query}`, { auth: true });
    } catch {
      return [];
    }
  },

  /** Get one delivery order with lines. */
  async getDeliveryOrder(id: number): Promise<DeliveryOrder> {
    return http<DeliveryOrder>(`/delivery-orders/${id}`, { auth: true });
  },

  /** Create a delivery order (POST /delivery-orders). DO posts a COGS journal. */
  async createDeliveryOrder(input: CreateDeliveryInput): Promise<DeliveryOrder> {
    return http<DeliveryOrder>("/delivery-orders", {
      method: "POST",
      auth: true,
      idempotencyKey: newIdempotencyKey(),
      body: JSON.stringify(input),
    });
  },

  /** Invoice list (GET /invoices). Optional status filter. */
  async listInvoices(status?: InvoiceListItem["status"]): Promise<InvoiceListItem[]> {
    const query = status ? `?status=${status}` : "";
    try {
      return await http<InvoiceListItem[]>(`/invoices${query}`, { auth: true });
    } catch {
      return [];
    }
  },

  /** Get one invoice with lines. */
  async getInvoice(id: number): Promise<Invoice> {
    return http<Invoice>(`/invoices/${id}`, { auth: true });
  },

  /** Create an invoice (POST /invoices). Posts revenue + DP realization journals. */
  async createInvoice(input: CreateInvoiceInput): Promise<Invoice> {
    return http<Invoice>("/invoices", {
      method: "POST",
      auth: true,
      idempotencyKey: newIdempotencyKey(),
      body: JSON.stringify(input),
    });
  },

  /** Receive a customer payment against an invoice (POST /invoices/{id}/payments). */
  async createInvoicePayment(invoiceId: number, input: CreatePaymentInput): Promise<InvoicePayment> {
    return http<InvoicePayment>(`/invoices/${invoiceId}/payments`, {
      method: "POST",
      auth: true,
      idempotencyKey: newIdempotencyKey(),
      body: JSON.stringify(input),
    });
  },

  /** List payments for an invoice. */
  async listInvoicePayments(invoiceId: number): Promise<InvoicePayment[]> {
    try {
      return await http<InvoicePayment[]>(`/invoices/${invoiceId}/payments`, { auth: true });
    } catch {
      return [];
    }
  },

  /** Credit note list (GET /credit-notes). Optional status filter. */
  async listCreditNotes(status?: CreditNoteListItem["status"]): Promise<CreditNoteListItem[]> {
    const query = status ? `?status=${status}` : "";
    try {
      return await http<CreditNoteListItem[]>(`/credit-notes${query}`, { auth: true });
    } catch {
      return [];
    }
  },

  /** Get one credit note with lines. */
  async getCreditNote(id: number): Promise<CreditNote> {
    return http<CreditNote>(`/credit-notes/${id}`, { auth: true });
  },

  /** Create a credit note (POST /credit-notes). Posts return + COGS reversal journals. */
  async createCreditNote(input: CreateCreditNoteInput): Promise<CreditNote> {
    return http<CreditNote>("/credit-notes", {
      method: "POST",
      auth: true,
      idempotencyKey: newIdempotencyKey(),
      body: JSON.stringify(input),
    });
  },

  // -- Suppliers --

  async listSuppliers(): Promise<SupplierListItem[]> {
    try {
      return await http<SupplierListItem[]>("/suppliers", { auth: true });
    } catch {
      return [];
    }
  },

  async getSupplier(id: number): Promise<Supplier> {
    return http<Supplier>(`/suppliers/${id}`, { auth: true });
  },

  async createSupplier(input: CreateSupplierInput): Promise<Supplier> {
    return http<Supplier>("/suppliers", {
      method: "POST",
      auth: true,
      body: JSON.stringify(input),
    });
  },

  // -- Purchase Orders --

  async listPurchaseOrders(status?: PurchaseOrderListItem["status"]): Promise<PurchaseOrderListItem[]> {
    const query = status ? `?status=${status}` : "";
    try {
      return await http<PurchaseOrderListItem[]>(`/purchase-orders${query}`, { auth: true });
    } catch {
      return [];
    }
  },

  async getPurchaseOrder(id: number): Promise<PurchaseOrder> {
    return http<PurchaseOrder>(`/purchase-orders/${id}`, { auth: true });
  },

  async createPurchaseOrder(input: CreatePurchaseOrderInput): Promise<PurchaseOrder> {
    return http<PurchaseOrder>("/purchase-orders", {
      method: "POST",
      auth: true,
      body: JSON.stringify(input),
    });
  },

  // -- Goods Received Notes --

  async listGRNs(status?: GoodsReceivedNoteListItem["status"]): Promise<GoodsReceivedNoteListItem[]> {
    const query = status ? `?status=${status}` : "";
    try {
      return await http<GoodsReceivedNoteListItem[]>(`/goods-received-notes${query}`, { auth: true });
    } catch {
      return [];
    }
  },

  async getGRN(id: number): Promise<GoodsReceivedNote> {
    return http<GoodsReceivedNote>(`/goods-received-notes/${id}`, { auth: true });
  },

  async createGRN(input: CreateGRNInput): Promise<GoodsReceivedNote> {
    return http<GoodsReceivedNote>("/goods-received-notes", {
      method: "POST",
      auth: true,
      idempotencyKey: newIdempotencyKey(),
      body: JSON.stringify(input),
    });
  },

  // -- Stock Opnames (US-043) --

  async listStockOpnames(): Promise<StockOpnameListItem[]> {
    try {
      return await http<StockOpnameListItem[]>("/stock-opnames", { auth: true });
    } catch {
      return [];
    }
  },

  async getStockOpname(id: number): Promise<StockOpname> {
    return http<StockOpname>(`/stock-opnames/${id}`, { auth: true });
  },

  async createStockOpname(input: CreateStockOpnameInput): Promise<StockOpname> {
    return http<StockOpname>("/stock-opnames", {
      method: "POST",
      auth: true,
      idempotencyKey: newIdempotencyKey(),
      body: JSON.stringify(input),
    });
  },

  async approveStockOpname(id: number): Promise<StockOpname> {
    return http<StockOpname>(`/stock-opnames/${id}/approve`, {
      method: "POST",
      auth: true,
      idempotencyKey: newIdempotencyKey(),
    });
  },

  // -- Stock Transfers (US-042) --

  async listStockTransfers(): Promise<StockTransferListItem[]> {
    try {
      return await http<StockTransferListItem[]>("/stock-transfers", { auth: true });
    } catch {
      return [];
    }
  },

  async getStockTransfer(id: number): Promise<StockTransfer> {
    return http<StockTransfer>(`/stock-transfers/${id}`, { auth: true });
  },

  async createStockTransfer(input: CreateStockTransferInput): Promise<StockTransfer> {
    return http<StockTransfer>("/stock-transfers", {
      method: "POST",
      auth: true,
      idempotencyKey: newIdempotencyKey(),
      body: JSON.stringify(input),
    });
  },

  // -- Supplier Invoices (Tagihan) --

  /** Supplier invoice list (GET /supplier-invoices). Optional status filter. */
  async listSupplierInvoices(status?: string): Promise<SupplierInvoiceListItem[]> {
    const qs = status ? `?status=${encodeURIComponent(status)}` : "";
    try {
      return await http<SupplierInvoiceListItem[]>(`/supplier-invoices${qs}`, { auth: true });
    } catch {
      return [];
    }
  },

  /** Get one supplier invoice with lines. */
  async getSupplierInvoice(id: number): Promise<SupplierInvoice> {
    return http<SupplierInvoice>(`/supplier-invoices/${id}`, { auth: true });
  },

  /** Create a supplier invoice (POST /supplier-invoices). Posts reclassification journal. */
  async createSupplierInvoice(input: CreateSupplierInvoiceInput): Promise<SupplierInvoice> {
    return http<SupplierInvoice>("/supplier-invoices", {
      method: "POST",
      auth: true,
      idempotencyKey: newIdempotencyKey(),
      body: JSON.stringify(input),
    });
  },

  // -- Supplier Payments (Bayar) --

  /** Receive a supplier payment against a supplier invoice (POST /supplier-invoices/{id}/payments). */
  async createSupplierPayment(invoiceId: number, input: CreateSupplierPaymentInput): Promise<SupplierPayment> {
    return http<SupplierPayment>(`/supplier-invoices/${invoiceId}/payments`, {
      method: "POST",
      auth: true,
      idempotencyKey: newIdempotencyKey(),
      body: JSON.stringify(input),
    });
  },

  /** List payments for a supplier invoice. */
  async listSupplierPayments(invoiceId: number): Promise<SupplierPayment[]> {
    try {
      return await http<SupplierPayment[]>(`/supplier-invoices/${invoiceId}/payments`, { auth: true });
    } catch {
      return [];
    }
  },

  // -- Purchase Returns (Retur Pembelian) --

  async listPurchaseReturns(status?: string): Promise<PurchaseReturnListItem[]> {
    const qs = status ? `?status=${encodeURIComponent(status)}` : "";
    try {
      return await http<PurchaseReturnListItem[]>(`/purchase-returns${qs}`, { auth: true });
    } catch {
      return [];
    }
  },

  async getPurchaseReturn(id: number): Promise<PurchaseReturn> {
    return http<PurchaseReturn>(`/purchase-returns/${id}`, { auth: true });
  },

  async createPurchaseReturn(input: CreatePurchaseReturnInput): Promise<PurchaseReturn> {
    return http<PurchaseReturn>("/purchase-returns", {
      method: "POST",
      auth: true,
      idempotencyKey: newIdempotencyKey(),
      body: JSON.stringify(input),
    });
  },

  // -- Bank Reconciliation --

  /** Bank accounts only (account_type = BANK), from GET /accounts. Failure -> empty array. */
  async listBankAccounts(): Promise<BackendAccount[]> {
    try {
      const all = await http<BackendAccount[]>("/accounts", { auth: true });
      return all.filter((a) => a.is_active && !a.is_group && a.account_type === "BANK");
    } catch {
      return [];
    }
  },

  /** List bank statements (GET /bank-statements). Optional status filter. */
  async listBankStatements(status?: string): Promise<BankStatementListItem[]> {
    const qs = status ? `?status=${encodeURIComponent(status)}` : "";
    try {
      return await http<BankStatementListItem[]>(`/bank-statements${qs}`, { auth: true });
    } catch {
      return [];
    }
  },

  /** Get one bank statement with its lines (GET /bank-statements/{id}). */
  async getBankStatement(id: number): Promise<BankStatement> {
    return http<BankStatement>(`/bank-statements/${id}`, { auth: true });
  },

  /** Import a bank statement (POST /bank-statements). CSV is parsed client-side; lines sent as JSON. */
  async importBankStatement(input: CreateBankStatementInput): Promise<BankStatement> {
    return http<BankStatement>("/bank-statements", {
      method: "POST",
      auth: true,
      idempotencyKey: newIdempotencyKey(),
      body: JSON.stringify(input),
    });
  },

  /** Start reconciliation: auto-match statement lines to book transactions (POST /bank-statements/{id}/reconcile). */
  async startReconciliation(statementId: number): Promise<Reconciliation> {
    return http<Reconciliation>(`/bank-statements/${statementId}/reconcile`, {
      method: "POST",
      auth: true,
    });
  },

  /** Get one reconciliation with its detail (GET /bank-reconciliations/{id}). */
  async getReconciliation(id: number): Promise<Reconciliation> {
    return http<Reconciliation>(`/bank-reconciliations/${id}`, { auth: true });
  },

  /** Manual match a statement line to a journal line (POST /bank-reconciliations/{id}/match). */
  async matchReconciliationLine(reconId: number, input: ReconcileMatchInput): Promise<Reconciliation> {
    return http<Reconciliation>(`/bank-reconciliations/${reconId}/match`, {
      method: "POST",
      auth: true,
      body: JSON.stringify(input),
    });
  },

  /** Unmatch a statement line (POST /bank-reconciliations/{id}/unmatch). */
  async unmatchReconciliationLine(reconId: number, input: ReconcileUnmatchInput): Promise<Reconciliation> {
    return http<Reconciliation>(`/bank-reconciliations/${reconId}/unmatch`, {
      method: "POST",
      auth: true,
      body: JSON.stringify(input),
    });
  },

  /** Complete reconciliation (POST /bank-reconciliations/{id}/complete). Requires diff_cents == 0. */
  async completeReconciliation(reconId: number): Promise<Reconciliation> {
    return http<Reconciliation>(`/bank-reconciliations/${reconId}/complete`, {
      method: "POST",
      auth: true,
      idempotencyKey: newIdempotencyKey(),
    });
  },

  // -- Bill of Materials (US-070) --

  async listBOMs(): Promise<BOMListItem[]> {
    try {
      return await http<BOMListItem[]>("/bill-of-materials", { auth: true });
    } catch {
      return [];
    }
  },

  async getBOM(id: number): Promise<BOM> {
    return http<BOM>(`/bill-of-materials/${id}`, { auth: true });
  },

  async createBOM(input: CreateBOMInput): Promise<BOM> {
    return http<BOM>("/bill-of-materials", {
      method: "POST",
      auth: true,
      idempotencyKey: newIdempotencyKey(),
      body: JSON.stringify(input),
    });
  },

  // -- Production Jobs (US-070..072) --

  async listProductionJobs(): Promise<ProductionJobListItem[]> {
    try {
      return await http<ProductionJobListItem[]>("/production-jobs", { auth: true });
    } catch {
      return [];
    }
  },

  async getProductionJob(id: number): Promise<ProductionJob> {
    return http<ProductionJob>(`/production-jobs/${id}`, { auth: true });
  },

  async createProductionJob(input: CreateProductionJobInput): Promise<ProductionJob> {
    return http<ProductionJob>("/production-jobs", {
      method: "POST",
      auth: true,
      idempotencyKey: newIdempotencyKey(),
      body: JSON.stringify(input),
    });
  },

  async addProductionJobCost(jobId: number, input: CreateProductionJobCostInput): Promise<ProductionJobCost> {
    return http<ProductionJobCost>(`/production-jobs/${jobId}/costs`, {
      method: "POST",
      auth: true,
      idempotencyKey: newIdempotencyKey(),
      body: JSON.stringify(input),
    });
  },

  async completeProductionJob(jobId: number, input: CompleteProductionJobInput): Promise<ProductionJob> {
    return http<ProductionJob>(`/production-jobs/${jobId}/complete`, {
      method: "POST",
      auth: true,
      idempotencyKey: newIdempotencyKey(),
      body: JSON.stringify(input),
    });
  },

  // -- Report Frameworks (US-090A) --

  /** List the tenant's configured report frameworks (GET /report-frameworks). */
  async listReportFrameworks(): Promise<ReportFrameworkRecord[]> {
    try {
      return await http<ReportFrameworkRecord[]>("/report-frameworks", { auth: true });
    } catch {
      return [];
    }
  },

  /** Set or upsert a report framework for the tenant (POST /report-frameworks). */
  async setReportFramework(input: SetFrameworkInput): Promise<ReportFrameworkRecord> {
    return http<ReportFrameworkRecord>("/report-frameworks", {
      method: "POST",
      auth: true,
      body: JSON.stringify(input),
    });
  },

  // -- Dimensions (US-093) --

  /** List dimensions (GET /dimensions?dimension_type=&is_active=). */
  async listDimensions(dimensionType?: string): Promise<Dimension[]> {
    const qs = dimensionType ? `?dimension_type=${encodeURIComponent(dimensionType)}` : "";
    try {
      return await http<Dimension[]>(`/dimensions${qs}`, { auth: true });
    } catch {
      return [];
    }
  },

  /** Create a dimension (POST /dimensions). */
  async createDimension(input: CreateDimensionInput): Promise<Dimension> {
    return http<Dimension>("/dimensions", {
      method: "POST",
      auth: true,
      body: JSON.stringify(input),
    });
  },

  /** Tag a journal line with dimensions (POST /journal-lines/{id}/dimensions). */
  async tagJournalLine(lineId: number, input: TagJournalLineInput): Promise<{ tagged: boolean }> {
    return http(`/journal-lines/${lineId}/dimensions`, {
      method: "POST",
      auth: true,
      body: JSON.stringify(input),
    });
  },

  // -- Budgets (US-093) --

  /** List budgets (GET /budgets?fiscal_year=&status=). */
  async listBudgets(fiscalYear?: number, status?: string): Promise<BudgetListItem[]> {
    const params = new URLSearchParams();
    if (fiscalYear) params.set("fiscal_year", String(fiscalYear));
    if (status) params.set("status", status);
    const qs = params.toString();
    try {
      return await http<BudgetListItem[]>(`/budgets${qs ? `?${qs}` : ""}`, { auth: true });
    } catch {
      return [];
    }
  },

  /** Get one budget with its lines (GET /budgets/{id}). */
  async getBudget(id: number): Promise<Budget> {
    return http<Budget>(`/budgets/${id}`, { auth: true });
  },

  /** Create a budget with monthly lines (POST /budgets). */
  async createBudget(input: CreateBudgetInput): Promise<Budget> {
    return http<Budget>("/budgets", {
      method: "POST",
      auth: true,
      idempotencyKey: newIdempotencyKey(),
      body: JSON.stringify(input),
    });
  },

  /** Budget vs actual report (GET /budgets/{id}/vs-actual). */
  async getBudgetVsActual(id: number): Promise<BudgetVsActualResult> {
    return http<BudgetVsActualResult>(`/budgets/${id}/vs-actual`, { auth: true });
  },

  /**
   * Completes onboarding: creates the tenant on the backend (POST /tenants),
   * then posts the opening balance (POST /opening-balances) when filled in.
   * Network failure -> save locally (mock) as graceful degradation.
   */
  async completeOnboarding(input: OnboardingInput): Promise<{ business: Business }> {
    if (!input.business.name.trim() || !input.business.businessType.trim()) {
      throw makeError("VALIDATION_ERROR", "Business name and business type are required.");
    }
    const state = loadState();
    const business: Business = {
      id: fakeId("tnt"),
      name: input.business.name.trim(),
      businessType: input.business.businessType.trim(),
      currency: input.business.currency,
      fiscalYearStart: input.period.startMonth,
    };
    try {
      const tenant = await http<BackendTenant>("/tenants", {
        method: "POST",
        auth: true,
        body: JSON.stringify({ name: business.name, slug: slugify(business.name) }),
      });
      if (tenant.id > 0) business.id = String(tenant.id);
      const opening = await buildOpeningBalance(input);
      if (opening) {
        try {
          await http<BackendJournalResult>("/opening-balances", {
            method: "POST",
            auth: true,
            idempotencyKey: newIdempotencyKey(),
            body: JSON.stringify(opening),
          });
        } catch {
          // Opening balance failed to post (e.g. balance accounts not yet
          // created on the backend) — onboarding still finishes with the
          // tenant saved.
        }
      }
      saveState({ ...state, business });
      return delay({ business });
    } catch {
      saveState({ ...state, business });
      return delay({ business });
    }
  },

  /** Reads the setup status from local storage. */
  getLocalState(): PersistedState {
    return loadState();
  },

  /** Access token for protected API calls (used later). */
  getAccessToken,
};

export const mockHelpers = {
  today,
  fmtIDR,
  nowIso,
};

export type { CurrencyCode };
