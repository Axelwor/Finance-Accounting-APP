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
