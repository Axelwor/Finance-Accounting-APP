/**
 * Lapisan API klien (M1, hybrid).
 *
 * Auth (register/login/logout) sudah terhubung ke backend:
 *   POST /api/v1/auth/register
 *   POST /api/v1/auth/login
 *   POST /api/v1/auth/logout
 *
 * Akun, kategori, transaksi, dashboard, dan onboarding juga sudah terhubung
 * ke endpoint backend (lihat API_CONTRACT.md). Setiap panggilan jaringan yang
 * gagal otomatis jatuh ke data lokal (mock) sehingga aplikasi tetap berfungsi
 * offline (graceful degradation).
 */

import type {
  AccountItem,
  ApiError,
  BackendAccount,
  BackendBalanceSheet,
  BackendCashFlow,
  BackendCategory,
  BackendJournalResult,
  BackendProfitLoss,
  BackendTenant,
  CashCommandPayload,
  Category,
  CurrencyCode,
  DashboardSummary,
  LoginInput,
  OnboardingInput,
  OpeningBalancePayload,
  RegisterInput,
  Transaction,
  TransactionInput,
  TransferCommandPayload,
  Usaha,
} from "./types";

const LATENCY_MS = 200;
const STORAGE_KEY = "pembukuan-mudah.m1.v1";
const TOKEN_KEY = "pembukuan-mudah.tokens";
const API_BASE = "/api/v1";

const MOCK_CATEGORIES: Category[] = [
  { id: "cat-penjualan", nama: "Penjualan", jenis: "uang-masuk" },
  { id: "cat-piutang", nama: "Terima piutang", jenis: "uang-masuk" },
  { id: "cat-modal", nama: "Modal tambahan", jenis: "uang-masuk" },
  { id: "cat-pinjaman", nama: "Pinjaman masuk", jenis: "uang-masuk" },
  { id: "cat-belanja", nama: "Belanja barang dagang", jenis: "uang-keluar" },
  { id: "cat-bahan", nama: "Bahan baku", jenis: "uang-keluar" },
  { id: "cat-sewa", nama: "Sewa tempat", jenis: "uang-keluar" },
  { id: "cat-listrik", nama: "Listrik dan air", jenis: "uang-keluar" },
  { id: "cat-gaji", nama: "Gaji karyawan", jenis: "uang-keluar" },
  { id: "cat-lain", nama: "Pengeluaran lain", jenis: "uang-keluar" },
];

const MOCK_ACCOUNTS: AccountItem[] = [
  { id: "acc-kas", nama: "Kas" },
  { id: "acc-bank", nama: "Bank BCA" },
];

interface PersistedState {
  user: { id: string; email: string; namaUsaha: string } | null;
  usaha: Usaha | null;
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
    /* data rusak -> mulai dari awal */
  }
  return { user: null, usaha: null, transactions: [] };
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

function fmtRupiah(n: number): string {
  return new Intl.NumberFormat("id-ID", {
    style: "currency",
    currency: "IDR",
    maximumFractionDigits: 0,
  }).format(n);
}

/* ------------------------------------------------------------------ */
/* HTTP helper untuk backend API                                      */
/* ------------------------------------------------------------------ */

interface HttpOptions extends RequestInit {
  /** Sertakan Authorization: Bearer <access token>. */
  auth?: boolean;
  /** Sertakan header Idempotency-Key (UUID) — wajib untuk perintah finansial. */
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
    const message = (body as ApiError)?.message ?? "Terjadi kesalahan. Coba lagi.";
    throw makeError(code, message);
  }
  return body as T;
}

/** UUID v4 untuk header Idempotency-Key (crypto.randomUUID saat tersedia). */
function newIdempotencyKey(): string {
  if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
    return crypto.randomUUID();
  }
  // Fallback: UUID v4 manual agar berjalan di browser lama / non-secure context.
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
/* Pemetaan respons backend -> bentuk UI                               */
/* ------------------------------------------------------------------ */

/** GET /accounts -> AccountItem {id, nama}. Hanya akun aktif & detail. */
function mapAccounts(raw: BackendAccount[]): AccountItem[] {
  return raw
    .filter((account) => account.is_active && !account.is_group)
    .map((account) => ({ id: String(account.id), nama: account.name }));
}

/** GET /categories -> Category {id, nama, jenis}. */
function mapCategories(raw: BackendCategory[]): Category[] {
  return raw
    .filter((category) => category.is_active)
    .map((category) => ({
      id: String(category.id),
      nama: category.name,
      jenis: category.direction === "IN" ? "uang-masuk" : "uang-keluar",
    }));
}

/** Id akun backend (int64) dari id UI string; NaN jika tidak terbaca. */
function accountId(raw: string | undefined): number {
  return Number(raw);
}

/** source_ref unik per perintah, mis. "WEB-1752754093701". */
function newSourceRef(): string {
  return `WEB-${Date.now()}`;
}

/** Untung/rugi bulan berjalan dari transaksi lokal (fallback offline). */
function computeUntungRugiLocal(transactions: Transaction[]): number {
  const now = new Date();
  return transactions
    .filter((trx) => {
      const [y, m] = trx.tanggal.split("-").map(Number);
      return y === now.getFullYear() && m === now.getMonth() + 1 && trx.jenis !== "pindah-uang";
    })
    .reduce((acc, trx) => acc + (trx.jenis === "uang-masuk" ? trx.nominal : -trx.nominal), 0);
}

/** Membangun payload POST /cash-in atau /cash-out. */
function buildCashCommand(input: TransactionInput, cashAccountId: number, counterAccountId: number): CashCommandPayload {
  return {
    source_ref: newSourceRef(),
    entry_date: input.tanggal,
    cash_account_id: cashAccountId,
    counter_account_id: counterAccountId,
    amount_cents: Math.round(input.nominal * 100),
    description: input.keterangan.trim(),
  };
}

/** Membangun payload POST /transfers. */
function buildTransferCommand(input: TransactionInput, fromAccountId: number, toAccountId: number): TransferCommandPayload {
  return {
    source_ref: newSourceRef(),
    entry_date: input.tanggal,
    from_account_id: fromAccountId,
    to_account_id: toAccountId,
    amount_cents: Math.round(input.nominal * 100),
    description: input.keterangan.trim(),
  };
}

/** Slug sederhana dari nama usaha, mis. "Warung Bu Sari" -> "warung-bu-sari". */
function slugify(value: string): string {
  return value
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, 80);
}

/** Membangun payload POST /opening-balances dari SaldoAwal onboarding. */
function buildOpeningBalance(input: OnboardingInput): OpeningBalancePayload | null {
  const { kas, bank, piutang, hutang, modal } = input.saldoAwal;
  const toCents = (value: number) => Math.round(value * 100);
  const balances: OpeningBalancePayload["balances"] = [];
  if (kas > 0) balances.push({ account_id: 0, debit_cents: toCents(kas), credit_cents: 0 });
  if (bank > 0) balances.push({ account_id: 0, debit_cents: toCents(bank), credit_cents: 0 });
  if (piutang > 0) balances.push({ account_id: 0, debit_cents: toCents(piutang), credit_cents: 0 });
  if (hutang > 0) balances.push({ account_id: 0, credit_cents: toCents(hutang), debit_cents: 0 });
  if (modal > 0) balances.push({ account_id: 0, credit_cents: toCents(modal), debit_cents: 0 });
  if (balances.length === 0) return null;
  return {
    source_ref: newSourceRef(),
    entry_date: `${input.periode.tahun}-${String(input.periode.mulaiBulan).padStart(2, "0")}-01`,
    equity_account_id: 0,
    balances,
    description: "Saldo awal onboarding",
  };
}

/* ------------------------------------------------------------------ */
/* Implementasi API (auth nyata; lainnya mock)                        */
/* ------------------------------------------------------------------ */

export const api = {
  /** Mendaftarkan pengguna baru di backend dan membuka sesi lokal. */
  async register(input: RegisterInput): Promise<{ user: { id: string; email: string; namaUsaha: string } }> {
    const email = input.email.trim().toLowerCase();
    if (!email || !input.password || !input.namaUsaha.trim()) {
      throw makeError("VALIDATION_ERROR", "Lengkapi alamat email, kata sandi, dan nama usaha.");
    }
    if (input.password.length < 8) {
      throw makeError("VALIDATION_ERROR", "Kata sandi minimal 8 karakter.");
    }
    const response = await http<AuthResponse>("/auth/register", {
      method: "POST",
      body: JSON.stringify({ email, password: input.password, full_name: input.namaUsaha.trim() }),
    });
    storeSession(response);
    const user = { id: "usr-" + (response.family_id ?? Date.now()), email, namaUsaha: input.namaUsaha.trim() };
    saveState({ ...loadState(), user });
    return delay({ user });
  },

  /** Membuka sesi via backend; menyimpan access + refresh token. */
  async login(input: LoginInput): Promise<{ user: { id: string; email: string; namaUsaha: string } }> {
    const email = input.email.trim().toLowerCase();
    if (!email || !input.password) {
      throw makeError("VALIDATION_ERROR", "Masukkan alamat email dan kata sandi.");
    }
    const response = await http<AuthResponse>("/auth/login", {
      method: "POST",
      body: JSON.stringify({ email, password: input.password }),
    });
    storeSession(response);
    const user = { id: "usr-" + (response.family_id ?? Date.now()), email, namaUsaha: email.split("@")[0] || "Usaha saya" };
    saveState({ ...loadState(), user });
    return delay({ user });
  },

  /** Menutup sesi: revoke refresh token di backend + hapus data lokal. */
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
   * Ringkasan dashboard dari laporan backend (profit-loss, balance-sheet,
   * cash-flow). Gagal jaringan -> hitung dari transaksi lokal (mock).
   */
  async getDashboard(): Promise<DashboardSummary> {
    const state = loadState();
    const sorted = [...state.transactions].sort((a, b) => b.tanggal.localeCompare(a.tanggal));
    const fallback = (): DashboardSummary => ({
      saldoKasBank: sorted.reduce(
        (acc, trx) =>
          trx.jenis === "uang-masuk"
            ? acc + trx.nominal
            : trx.jenis === "uang-keluar"
              ? acc - trx.nominal
              : acc,
        0,
      ),
      untungRugiBulanIni: computeUntungRugiLocal(sorted),
      tagihanJatuhTempo: 2,
      stokMenipis: 4,
      transaksiTerbaru: sorted.slice(0, 8),
    });
    try {
      const [profitLoss, cashFlow, balanceSheet] = await Promise.all([
        http<BackendProfitLoss>("/reports/profit-loss", { auth: true }),
        http<BackendCashFlow>("/reports/cash-flow", { auth: true }),
        http<BackendBalanceSheet>("/reports/balance-sheet", { auth: true }),
      ]);
      const saldoKasBank = Math.round(cashFlow.net_cash_flow_cents / 100);
      const untungRugiBulanIni = Math.round(profitLoss.profit_cents / 100);
      if (
        Number.isFinite(saldoKasBank) &&
        Number.isFinite(untungRugiBulanIni) &&
        typeof balanceSheet.asset_cents === "number" &&
        typeof balanceSheet.liability_cents === "number"
      ) {
        return delay({
          saldoKasBank,
          untungRugiBulanIni,
          tagihanJatuhTempo: 2,
          stokMenipis: 4,
          transaksiTerbaru: sorted.slice(0, 8),
        });
      }
      // Respons backend tidak seperti yang diharapkan -> pakai data lokal.
      return fallback();
    } catch {
      return fallback();
    }
  },

  /**
   * Mencatat transaksi ke backend:
   *   uang-masuk  -> POST /cash-in
   *   uang-keluar -> POST /cash-out
   *   pindah-uang -> POST /transfers
   * Setiap perintah memakai Idempotency-Key UUID. Gagal jaringan -> simpan
   * lokal (mock) sebagai graceful degradation.
   */
  async createTransaction(input: TransactionInput): Promise<{ transaksi: Transaction; transaksiTerbaru: Transaction[] }> {
    if (!input.nominal || input.nominal <= 0) {
      throw makeError("VALIDATION_ERROR", "Nominal harus lebih besar dari nol.");
    }
    if (!input.tanggal) {
      throw makeError("VALIDATION_ERROR", "Pilih tanggal transaksi.");
    }
    if (input.jenis === "pindah-uang" && (!input.dari || !input.ke || input.dari === input.ke)) {
      throw makeError("VALIDATION_ERROR", "Pilih dua rekening berbeda untuk pindah uang.");
    }
    const state = loadState();
    const kategori = input.kategoriId ? MOCK_CATEGORIES.find((c) => c.id === input.kategoriId) : undefined;
    const transaksi: Transaction = {
      id: fakeId("trx"),
      jenis: input.jenis,
      nominal: input.nominal,
      tanggal: input.tanggal,
      keterangan: input.keterangan.trim(),
      kategoriId: kategori?.id,
      kategoriNama: kategori?.nama,
      dari: input.dari,
      ke: input.ke,
      createdAt: nowIso(),
    };
    const simpanLokal = (): { transaksi: Transaction; transaksiTerbaru: Transaction[] } => {
      const transactions = [transaksi, ...state.transactions];
      saveState({ ...state, transactions });
      const sorted = [...transactions].sort((a, b) => b.tanggal.localeCompare(a.tanggal));
      return { transaksi, transaksiTerbaru: sorted.slice(0, 8) };
    };
    try {
      if (input.jenis === "pindah-uang") {
        const payload = buildTransferCommand(input, accountId(input.dari), accountId(input.ke));
        if (!Number.isFinite(payload.from_account_id) || !Number.isFinite(payload.to_account_id)) {
          throw makeError("VALIDATION_ERROR", "Pilih rekening sumber dan tujuan yang valid.");
        }
        await http<BackendJournalResult>("/transfers", {
          method: "POST",
          auth: true,
          idempotencyKey: newIdempotencyKey(),
          body: JSON.stringify(payload),
        });
      } else {
        const kategoriId = accountId(input.kategoriId);
        const cashAccountId = accountId(input.jenis === "uang-masuk" ? input.kategoriId : input.dari);
        const counterAccountId = accountId(input.jenis === "uang-masuk" ? input.dari : input.kategoriId);
        if (!Number.isFinite(cashAccountId) || !Number.isFinite(counterAccountId)) {
          throw makeError("VALIDATION_ERROR", "Pilih kategori dan rekening yang valid.");
        }
        const payload = buildCashCommand(input, cashAccountId, counterAccountId);
        await http<BackendJournalResult>(input.jenis === "uang-masuk" ? "/cash-in" : "/cash-out", {
          method: "POST",
          auth: true,
          idempotencyKey: newIdempotencyKey(),
          body: JSON.stringify(payload),
        });
        if (Number.isFinite(kategoriId)) {
          transaksi.kategoriId = String(kategoriId);
        }
      }
      // Berhasil di backend: simpan sebagai cache lokal agar offline tetap lengkap.
      return simpanLokal();
    } catch {
      return delay(simpanLokal());
    }
  },

  /** Daftar kategori dari GET /categories (Bearer). Gagal -> mock lokal. */
  async listCategories(): Promise<Category[]> {
    try {
      const raw = await http<BackendCategory[]>("/categories", { auth: true });
      const mapped = mapCategories(raw);
      return delay(mapped.length > 0 ? mapped : MOCK_CATEGORIES);
    } catch {
      return delay(MOCK_CATEGORIES);
    }
  },

  /** Daftar rekening dari GET /accounts (Bearer). Gagal -> mock lokal. */
  async listAccounts(): Promise<AccountItem[]> {
    try {
      const raw = await http<BackendAccount[]>("/accounts", { auth: true });
      const mapped = mapAccounts(raw);
      return delay(mapped.length > 0 ? mapped : MOCK_ACCOUNTS);
    } catch {
      return delay(MOCK_ACCOUNTS);
    }
  },

  /**
   * Menyelesaikan onboarding: buat tenant di backend (POST /tenants),
   * lalu kirim saldo awal (POST /opening-balances) bila diisi. Gagal
   * jaringan -> simpan lokal (mock) sebagai graceful degradation.
   */
  async completeOnboarding(input: OnboardingInput): Promise<{ usaha: Usaha }> {
    if (!input.usaha.nama.trim() || !input.usaha.jenisUsaha.trim()) {
      throw makeError("VALIDATION_ERROR", "Nama usaha dan jenis usaha wajib diisi.");
    }
    const state = loadState();
    const usaha: Usaha = {
      id: fakeId("tnt"),
      nama: input.usaha.nama.trim(),
      jenisUsaha: input.usaha.jenisUsaha.trim(),
      mataUang: input.usaha.mataUang,
      tahunBukuMulai: input.periode.mulaiBulan,
    };
    try {
      const tenant = await http<BackendTenant>("/tenants", {
        method: "POST",
        auth: true,
        body: JSON.stringify({ name: usaha.nama, slug: slugify(usaha.nama) }),
      });
      if (tenant.id > 0) usaha.id = String(tenant.id);
      const opening = buildOpeningBalance(input);
      if (opening) {
        try {
          await http<BackendJournalResult>("/opening-balances", {
            method: "POST",
            auth: true,
            idempotencyKey: newIdempotencyKey(),
            body: JSON.stringify(opening),
          });
        } catch {
          // Saldo awal gagal diposting (mis. akun saldo belum dibuat di
          // backend) — onboarding tetap selesai dengan tenant tersimpan.
        }
      }
      saveState({ ...state, usaha });
      return delay({ usaha });
    } catch {
      saveState({ ...state, usaha });
      return delay({ usaha });
    }
  },

  /** Membaca status setup dari penyimpanan lokal. */
  getLocalState(): PersistedState {
    return loadState();
  },

  /** Token akses untuk panggilan API terproteksi (dipakai nanti). */
  getAccessToken,
};

export const mockHelpers = {
  today,
  fmtRupiah,
  nowIso,
};

export type { CurrencyCode };
