/** Utilitas format untuk UI (bahasa Indonesia). */

export function formatRupiah(nilai: number): string {
  return new Intl.NumberFormat("id-ID", {
    style: "currency",
    currency: "IDR",
    maximumFractionDigits: 0,
  }).format(nilai);
}

/** Format "15 Juni 2026". */
export function formatTanggal(tanggal: string): string {
  const [tahun, bulan, hari] = tanggal.split("-").map(Number);
  if (!tahun || !bulan || !hari) return tanggal;
  const d = new Date(tahun, bulan - 1, hari);
  return new Intl.DateTimeFormat("id-ID", { day: "numeric", month: "long", year: "numeric" }).format(d);
}

/** Tanggal hari ini dalam format yyyy-mm-dd (lokal). */
export function todayISO(): string {
  const d = new Date();
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const hari = String(d.getDate()).padStart(2, "0");
  return `${d.getFullYear()}-${m}-${hari}`;
}

/** "Hari ini" / "Kemarin" / "12 Juni 2026" untuk daftar transaksi. */
export function formatTanggalRelatif(tanggal: string): string {
  const [tahun, bulan, hari] = tanggal.split("-").map(Number);
  if (!tahun || !bulan || !hari) return tanggal;
  const d = new Date(tahun, bulan - 1, hari);
  const sekarang = new Date();
  const awalHariIni = new Date(sekarang.getFullYear(), sekarang.getMonth(), sekarang.getDate());
  const selisih = Math.round((awalHariIni.getTime() - d.getTime()) / 86400000);
  if (selisih === 0) return "Hari ini";
  if (selisih === 1) return "Kemarin";
  return new Intl.DateTimeFormat("id-ID", { day: "numeric", month: "short", year: "numeric" }).format(d);
}

/** Parsing input angka dari form menjadi bilangan bulat rupiah (0 bila kosong). */
export function parseRupiahInput(teks: string): number {
  const bersih = teks.replace(/[^\d]/g, "");
  if (!bersih) return 0;
  return parseInt(bersih, 10);
}
