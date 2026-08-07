import { useState } from "react";
import { api } from "../api";
import { Button, Card, FormError } from "./ui";
import type { PeriodResult } from "../types";

type AksiPeriode = "tutup" | "buka";

const KONFIRMASI: Record<AksiPeriode, string> = {
  tutup:
    "Tutup buku periode berjalan? Setelah ditutup, periode tidak dapat menerima transaksi baru sampai dibuka kembali.",
  buka:
    "Buka kembali periode yang sudah ditutup? Jurnal penutup akan dibatalkan otomatis oleh sistem.",
};

/** Kartu kecil "Periode buku": menutup / membuka periode dari dashboard. */
export function PeriodCard() {
  const [sibuk, setSibuk] = useState<AksiPeriode | null>(null);
  const [hasil, setHasil] = useState<PeriodResult | null>(null);
  const [error, setError] = useState<string | null>(null);

  const jalankan = async (aksi: AksiPeriode) => {
    if (!window.confirm(KONFIRMASI[aksi])) return;
    setError(null);
    setSibuk(aksi);
    try {
      const result = aksi === "tutup" ? await api.closePeriod() : await api.unlockPeriod();
      setHasil(result);
    } catch (err) {
      setHasil(null);
      setError(err instanceof Error ? err.message : "Gagal memproses periode. Coba lagi.");
    } finally {
      setSibuk(null);
    }
  };

  const sedangProses = sibuk !== null;

  return (
    <Card
      className="period-card"
      title="Periode buku"
      description="Tutup buku saat periode selesai, atau buka kembali bila ada koreksi."
    >
      <div className="quick-actions">
        <Button variant="primary" disabled={sedangProses} onClick={() => void jalankan("tutup")}>
          {sibuk === "tutup" ? "Menutup buku..." : "Tutup Buku"}
        </Button>
        <Button variant="secondary" disabled={sedangProses} onClick={() => void jalankan("buka")}>
          {sibuk === "buka" ? "Membuka periode..." : "Buka Periode"}
        </Button>
      </div>

      {hasil ? (
        <p className="period-card__status" role="status">
          Periode #{hasil.period_id} {hasil.status === "CLOSED" ? "ditutup" : "dibuka kembali"} — jurnal{" "}
          <code>{hasil.number}</code> tercatat.
        </p>
      ) : null}
      <FormError message={error} />
    </Card>
  );
}
