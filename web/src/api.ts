/**
 * Lapisan API klien (M1, mock).
 *
 * Seluruh akses data di aplikasi melewati file ini. Saat ini semua fungsi
 * berjalan di sisi klien (localStorage + delay buatan) sehingga UI dapat
 * dibangun dan diuji tanpa backend. Nanti, fungsi-fungsi di bawah cukup
 * diarahkan ke endpoint REST yang sesuai (lihat ARCHITECTURE.md, API surface):
 *
 *   api.register          -> POST /api/v1/auth/register
 *   api.login             -> POST /api/v1/auth/login
 *   api.logout            -> POST /api/v1/auth/logout
 *   api.getDashboard      -> GET  /api/v1/tenants/:id/dashboard
 *   api.createTransaction -> POST /api/v1/transactions
 *   api.listTransactions  -> GET  /api/v1/transactions
 *   api.listCategories    -> GET  /api/v1/categories
 *   api.listAccounts      -> GET  /api/v1/accounts
 *   api.completeOnboarding-> POST /api/v1/tenants (setup usaha + periode + saldo awal)
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

/** Simulasikan latensi jaringan agar state loading dapat terlihat di UI. */
const LATENCY_MS = 450;

const STORAGE_KEY = "pembukuan-mudah.m1.v1";

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

/** Transaksi contoh agar dashboard tidak kosong saat pertama kali dibuka. */
function seedTransactions(usahaNama: string): Transaction[] {
  const t = (tanggal: string, nominal: number, keterangan: string, kategoriId: string): Transaction => {
    const kategori = MOCK_CATEGORIES.find((c) => c.id === kategoriId);
    const jenis = kategori?.jenis ?? "uang-masuk";
    return {
      id: fakeId("trx"),
      jenis,
      nominal,
      tanggal,
      keterangan,
      kategoriId,
      kategoriNama: kategori?.nama,
      dari: jenis === "uang-keluar" ? "Kas" : undefined,
      ke: jenis === "uang-masuk" ? "Kas" : undefined,
      createdAt: `${tanggal}T09:00:00.000Z`,
    };
  };

  const d = new Date();
  const d30 = new Date(d.getTime() - 30 * 86400000);
  const iso = (x: Date) => {
    const m = String(x.getMonth() + 1).padStart(2, "0");
    const day = String(x.getDate()).padStart(2, "0");
    return `${x.getFullYear()}-${m}-${day}`;
  };

  const cat = (prefix: string) =>
    MOCK_CATEGORIES.find((c) => c.id.startsWith(prefix))?.id ?? "";

  return [
    t(iso(new Date(d.getTime() - 1 * 86400000)), 850000, `Penjualan harian ${usahaNama}`, cat("cat-penjualan")),
    t(iso(new Date(d.getTime() - 2 * 86400000)), 320000, "Belanja bahan baku mingguan", cat("cat-bahan")),
    t(iso(new Date(d.getTime() - 3 * 86400000)), 1500000, "Setoran modal pemilik", cat("cat-modal")),
    t(iso(new Date(d.getTime() - 4 * 86400000)), 275000, "Tagihan listrik dan air", cat("cat-listrik")),
    t(iso(new Date(d.getTime() - 5 * 86400000)), 1250000, "Penjualan pesanan pelanggan", cat("cat-penjualan")),
    t(iso(new Date(d.getTime() - 6 * 86400000)), 600000, "Sewa tempat usaha", cat("cat-sewa")),
    t(iso(new Date(d.getTime() - 7 * 86400000)), 950000, "Penjualan harian", cat("cat-penjualan")),
    t(iso(new Date(d.getTime() - 9 * 86400000)), 1100000, "Penjualan harian", cat("cat-penjualan")),
    t(iso(new Date(d.getTime() - 11 * 86400000)), 250000, "Gaji karyawan", cat("cat-gaji")),
    t(iso(new Date(d.getTime() - 14 * 86400000)), 450000, "Belanja barang dagang", cat("cat-belanja")),
  ].map((x) => ({ ...x, createdAt: `${x.tanggal}T09:00:00.000Z` }));
}

/* ------------------------------------------------------------------ */
/* Implementasi mock API (ganti dengan fetch nyata saat backend siap).  */
/* ------------------------------------------------------------------ */

export const api = {
  /** Mendaftarkan pengguna baru dan langsung membuka sesi. */
  async register(input: RegisterInput): Promise<{ user: { id: string; email: string; namaUsaha: string } }> {
    const email = input.email.trim().toLowerCase();
    if (!email || !input.password || !input.namaUsaha.trim()) {
      throw makeError("VALIDATION_ERROR", "Lengkapi alamat email, kata sandi, dan nama usaha.");
    }
    if (input.password.length < 6) {
      throw makeError("VALIDATION_ERROR", "Kata sandi minimal 6 karakter.");
    }
    const state = loadState();
    if (state.user) {
      throw makeError("ALREADY_LOGGED_IN", "Anda sudah masuk. Keluar dulu untuk mendaftar akun baru.");
    }
    const user = { id: fakeId("usr"), email, namaUsaha: input.namaUsaha.trim() };
    saveState({ ...state, user });
    return delay({ user });
  },

  /** Membuka sesi. Mock: email dan kata sandi apa pun diterima (selama terisi). */
  async login(input: LoginInput): Promise<{ user: { id: string; email: string; namaUsaha: string } }> {
    const email = input.email.trim().toLowerCase();
    if (!email || !input.password) {
      throw makeError("VALIDATION_ERROR", "Masukkan alamat email dan kata sandi.");
    }
    const state = loadState();
    if (state.user) {
      throw makeError("ALREADY_LOGGED_IN", "Anda sudah masuk. Keluar dulu untuk masuk sebagai akun lain.");
    }
    const user = { id: fakeId("usr"), email, namaUsaha: email.split("@")[0] || "Usaha saya" };
    saveState({ ...state, user });
    return delay({ user });
  },

  /** Menutup sesi (data lokal tetap tersimpan). */
  async logout(): Promise<void> {
    const state = loadState();
    saveState({ ...state, user: null });
    return delay(undefined);
  },

  /** Ringkasan dashboard: kartu + transaksi terbaru. */
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

  /** Menyimpan catatan baru dan mengembalikan daftar transaksi terbaru. */
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

  /** Daftar kategori untuk form Uang Masuk / Uang Keluar. */
  async listCategories(): Promise<Category[]> {
    return delay(MOCK_CATEGORIES);
  },

  /** Daftar rekening (Kas & Bank) untuk form Pindah Uang. */
  async listAccounts(): Promise<AccountItem[]> {
    return delay(MOCK_ACCOUNTS);
  },

  /** Menyelesaikan onboarding: data usaha + periode buku + saldo awal ringkas. */
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

  /**
   * Membaca status setup dari penyimpanan lokal.
   * (Pembantu untuk router; bukan bagian dari kontrak backend.)
   */
  getLocalState(): PersistedState {
    return loadState();
  },
};

export const mockHelpers = {
  today,
  fmtRupiah,
  nowIso,
};

export type { CurrencyCode };
