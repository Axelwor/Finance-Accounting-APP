import { useCallback, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { api } from "../api";
import { useAppState } from "../state";
import { Card, ErrorState, LoadingState } from "../components/ui";
import { TransactionRow } from "../components/transactions";
import { formatRupiah } from "../lib/format";
import type { DashboardSummary } from "../types";

const wordmark = "Pembukuan Mudah";

interface CardKpiProps {
  label: string;
  nilai: string;
  catatan?: string;
}

function CardKpi({ label, nilai, catatan }: CardKpiProps) {
  return (
    <section className="kpi-card">
      <p className="kpi-card__label">{label}</p>
      <p className="kpi-card__nilai">{nilai}</p>
      {catatan ? <p className="kpi-card__catatan">{catatan}</p> : null}
    </section>
  );
}

/** Halaman ringkasan utama setelah login / onboarding selesai. */
export function DashboardScreen() {
  const { user, usaha, transactions } = useAppState();
  const [data, setData] = useState<DashboardSummary | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [retryKey, setRetryKey] = useState(0);

  const muat = useCallback(async () => {
    setError(null);
    try {
      const ringkasan = await api.getDashboard();
      setData(ringkasan);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal memuat ringkasan. Coba lagi.");
    }
  }, []);

  useEffect(() => {
    void muat();
  }, [muat, retryKey, transactions]);

  useEffect(() => {
    document.title = usaha ? `${usaha.nama} - ${wordmark}` : wordmark;
  }, [usaha]);

  const namaUsaha = usaha?.nama || user?.namaUsaha || "Usaha Anda";

  return (
    <div className="dashboard">
      <header className="page-head">
        <div>
          <h1 className="page-title">Halo, {namaUsaha}</h1>
          <p className="page-sub">Ringkasan usaha Anda hari ini.</p>
        </div>
        <div className="page-head__actions">
          <Link className="btn btn--primary" to="/catat/uang-masuk">
            Catat transaksi
          </Link>
        </div>
      </header>

      {error ? (
        <ErrorState message={error} onRetry={() => setRetryKey((k) => k + 1)} />
      ) : !data ? (
        <LoadingState label="Memuat ringkasan..." />
      ) : (
        <>
          <div className="kpi-grid">
            <CardKpi label="Saldo Kas & Bank" nilai={formatRupiah(data.saldoKasBank)} catatan="Gabungan kas dan rekening" />
            <CardKpi
              label="Untung/Rugi bulan ini"
              nilai={formatRupiah(data.untungRugiBulanIni)}
              catatan={data.untungRugiBulanIni >= 0 ? "Selisih uang masuk dikurangi uang keluar" : "Pengeluaran lebih besar dari pemasukan"}
            />
            <CardKpi label="Tagihan jatuh tempo" nilai={String(data.tagihanJatuhTempo)} catatan="Contoh: 2 tagihan menunggu" />
            <CardKpi label="Stok menipis" nilai={String(data.stokMenipis)} catatan="Contoh: 4 barang perlu diisi" />
          </div>

          <Card
            title="Transaksi terbaru"
            description="Lima catatan terakhir dari buku Anda."
          >
            {data.transaksiTerbaru.length === 0 ? (
              <div className="empty-state">
                <h3 className="empty-state__title">Belum ada catatan</h3>
                <p className="empty-state__message">
                  Mulai dengan mencatat uang masuk atau uang keluar pertama Anda.
                </p>
                <Link className="btn btn--primary" to="/catat/uang-masuk">
                  Catat transaksi pertama
                </Link>
              </div>
            ) : (
              <ul className="transaction-list">
                {data.transaksiTerbaru.slice(0, 5).map((t) => (
                  <TransactionRow key={t.id} transaksi={t} />
                ))}
              </ul>
            )}
            {data.transaksiTerbaru.length > 5 ? (
              <div className="card__footer">
                <Link className="link-inline" to="/transaksi">
                  Lihat semua catatan
                </Link>
              </div>
            ) : null}
          </Card>
        </>
      )}

    </div>
  );
}
