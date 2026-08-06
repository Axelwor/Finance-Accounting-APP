/**
 * Lapisan API klien (M1, hybrid).
 *
 * Auth (register/login/logout) sudah terhubung ke backend:
 *   POST /api/v1/auth/register
 *   POST /api/v1/auth/login
 *   POST /api/v1/auth/logout
 *
 * Dashboard/transaksi/kategori/onboarding masih memakai data lokal (mock)
 * sampai endpoint transaksi & onboarding backend tersedia.
 */

import type {
  AccountItem,
  ApiError,
  Category,
  CurrencyCode,
  DashboardSummary,
  LoginInput,
  OnboardingInput,
  RegisterInput,
  Transaction,
  TransactionInput,
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

async function http<T>(path: string, options: RequestInit = {}): Promise<T> {
  const response = await fetch(`${API_BASE}${path}`, {
    headers: { "Content-Type": "application/json", ...(options.headers ?? {}) },
    ...options,
  });
  const body = await response.json().catch(() => ({}));
  if (!response.ok) {
    const code = (body as ApiError)?.code ?? "REQUEST_FAILED";
    const message = (body as ApiError)?.message ?? "Terjadi kesalahan. Coba lagi.";
    throw makeError(code, message);
  }
  return body as T;
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

  /** Ringkasan dashboard dari data lokal (mock sampai endpoint tersedia). */
  async getDashboard(): Promise<DashboardSummary> {
    const state = loadState();
    const sorted = [...state.transactions].sort((a, b) => b.tanggal.localeCompare(a.tanggal));
    const saldoKasBank = sorted.reduce(
      (acc, trx) =>
        trx.jenis === "uang-masuk"
          ? acc + trx.nominal
          : trx.jenis === "uang-keluar"
            ? acc - trx.nominal
            : acc,
      0,
    );
    const now = new Date();
    const isBulanIni = (tanggal: string) => {
      const [y, m] = tanggal.split("-").map(Number);
      return y === now.getFullYear() && m === now.getMonth() + 1;
    };
    const untungRugiBulanIni = sorted
      .filter((trx) => isBulanIni(trx.tanggal) && trx.jenis !== "pindah-uang")
      .reduce((acc, trx) => acc + (trx.jenis === "uang-masuk" ? trx.nominal : -trx.nominal), 0);
    return delay({
      saldoKasBank,
      untungRugiBulanIni,
      tagihanJatuhTempo: 2,
      stokMenipis: 4,
      transaksiTerbaru: sorted.slice(0, 8),
    });
  },

  /** Menyimpan catatan baru di lokal (mock sampai backend transaksi tersedia). */
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
    const kategori = input.kategoriId
      ? MOCK_CATEGORIES.find((c) => c.id === input.kategoriId)
      : undefined;
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
    const transactions = [transaksi, ...state.transactions];
    saveState({ ...state, transactions });
    const sorted = [...transactions].sort((a, b) => b.tanggal.localeCompare(a.tanggal));
    return delay({ transaksi, transaksiTerbaru: sorted.slice(0, 8) });
  },

  /** Daftar kategori (mock sampai backend kategori terhubung UI). */
  async listCategories(): Promise<Category[]> {
    return delay(MOCK_CATEGORIES);
  },

  /** Daftar rekening (mock). */
  async listAccounts(): Promise<AccountItem[]> {
    return delay(MOCK_ACCOUNTS);
  },

  /** Menyelesaikan onboarding: data usaha + periode (mock, backend belum lengkap). */
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
    saveState({ ...state, usaha });
    return delay({ usaha });
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
