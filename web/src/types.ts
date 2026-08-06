/**
 * Tipe data bersama untuk seluruh aplikasi web.
 *
 * Ini adalah kontrak frontend sementara (M1). Ketika backend tersedia, tipe ini
 * akan diselaraskan dengan kontrak API Go (lihat ARCHITECTURE.md, shared types).
 */

/** Bahasa ramah pengguna untuk setiap jenis catatan (tanpa istilah debet/kredit). */
export type TransactionKind = "uang-masuk" | "uang-keluar" | "pindah-uang";

export type CurrencyCode = "IDR";

export interface Usaha {
  id: string;
  nama: string;
  jenisUsaha: string;
  mataUang: CurrencyCode;
  /** Bulan (1-12) saat tahun buku dimulai. */
  tahunBukuMulai: number;
}

/** Periode pembukuan (buku). */
export interface PeriodeBuku {
  /** Tahun buku, mis. 2026. */
  tahun: number;
  /** Bulan (1-12) saat periode dimulai. */
  mulaiBulan: number;
}

export interface SaldoAwal {
  kas: number;
  bank: number;
  piutang: number;
  hutang: number;
  modal: number;
}

export interface User {
  id: string;
  email: string;
  namaUsaha: string;
}

export interface Category {
  id: string;
  nama: string;
  /** Hanya relevan untuk Uang Masuk / Uang Keluar. */
  jenis: "uang-masuk" | "uang-keluar";
}

export interface Transaction {
  id: string;
  jenis: TransactionKind;
  nominal: number;
  tanggal: string;
  keterangan: string;
  kategoriId?: string;
  kategoriNama?: string;
  /** Sumber dana, mis. "Kas" atau nama rekening bank. */
  dari?: string;
  /** Tujuan dana, mis. "Kas" atau nama rekening bank. */
  ke?: string;
  createdAt: string;
}

export interface DashboardSummary {
  saldoKasBank: number;
  untungRugiBulanIni: number;
  tagihanJatuhTempo: number;
  stokMenipis: number;
  transaksiTerbaru: Transaction[];
}

export interface ApiError {
  code: string;
  message: string;
}

export interface RegisterInput {
  email: string;
  password: string;
  namaUsaha: string;
}

export interface LoginInput {
  email: string;
  password: string;
}

export interface OnboardingInput {
  usaha: {
    nama: string;
    jenisUsaha: string;
    mataUang: CurrencyCode;
  };
  periode: PeriodeBuku;
  saldoAwal: SaldoAwal;
}

export interface TransactionInput {
  jenis: TransactionKind;
  nominal: number;
  tanggal: string;
  keterangan: string;
  kategoriId?: string;
  dari?: string;
  ke?: string;
}

/** Daftar rekening/kas untuk dropdown pada form Pindah Uang. */
export interface AccountItem {
  id: string;
  nama: string;
}

/* ------------------------------------------------------------------ */
/* Tipe kontrak backend (Go JSON di /api/v1) — dipetakan ke tipe UI.  */
/* ------------------------------------------------------------------ */

/** Baris akun dari GET /api/v1/accounts (coa.account struct). */
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

/** Baris kategori dari GET /api/v1/categories (coa.category struct). */
export interface BackendCategory {
  id: number;
  name: string;
  direction: "IN" | "OUT";
  default_debit_account_id: number | null;
  default_credit_account_id: number | null;
  is_active: boolean;
}

/** Respons dari POST /api/v1/tenants. */
export interface BackendTenant {
  id: number;
  name: string;
  slug: string;
}

/** Payload umum perintah keuangan (POST /cash-in, /cash-out). */
export interface CashCommandPayload {
  source_ref: string;
  entry_date: string;
  cash_account_id: number;
  counter_account_id: number;
  amount_cents: number;
  description: string;
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

/** Respons posting jurnal (cash.postingResult). */
export interface BackendJournalResult {
  id: number;
  number: string;
  status: string;
  hash: string;
  prev_hash: string;
  intent_type: string;
  is_reversal: boolean;
}

/** Respons GET /api/v1/reports/profit-loss. */
export interface BackendProfitLoss {
  revenue_cents: number;
  expense_cents: number;
  profit_cents: number;
}

/** Respons GET /api/v1/reports/balance-sheet. */
export interface BackendBalanceSheet {
  asset_cents: number;
  liability_cents: number;
  equity_cents: number;
  balanced: boolean;
}

/** Respons GET /api/v1/reports/cash-flow. */
export interface BackendCashFlow {
  inflow_cents: number;
  outflow_cents: number;
  net_cash_flow_cents: number;
}

/** Baris saldo awal untuk POST /api/v1/opening-balances. */
export interface OpeningBalanceLine {
  account_id: number;
  debit_cents: number;
  credit_cents: number;
}

/** Payload POST /api/v1/opening-balances. */
export interface OpeningBalancePayload {
  source_ref: string;
  entry_date: string;
  equity_account_id: number;
  balances: OpeningBalanceLine[];
  description: string;
}
