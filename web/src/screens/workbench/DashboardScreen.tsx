import { useCallback, useEffect, useState } from "react";
import { api } from "../../api";
import { useAppState } from "../../state";
import { useWorkbench } from "../../workbench/state";
import { ErrorState, LoadingState } from "../../components/ui";
import { Icon } from "../../components/m3/Icon";
import { formatIDR } from "../../lib/format";
import { QuickRatioGauge, SegmentedAgingBar, TrendPill } from "../../components/analytics";
import type {
  AgingSummary,
  JournalEntryListItem,
  LowStockItem,
  PeriodStatusData,
  PPNSummary,
} from "../../types";

export function DashboardScreen() {
  const workbench = useWorkbench();
  const { user, business } = useAppState();
  const [error, setError] = useState<string | null>(null);
  const [retryKey, setRetryKey] = useState(0);
  const [loading, setLoading] = useState(true);

  // Per-widget data
  const [cashBalance, setCashBalance] = useState<number | null>(null);
  const [profitLoss, setProfitLoss] = useState<number | null>(null);
  const [arAging, setArAging] = useState<AgingSummary | null>(null);
  const [apAging, setApAging] = useState<AgingSummary | null>(null);
  const [recentTxns, setRecentTxns] = useState<JournalEntryListItem[]>([]);
  const [lowStock, setLowStock] = useState<LowStockItem[]>([]);
  const [ppn, setPpn] = useState<PPNSummary | null>(null);
  const [period, setPeriod] = useState<PeriodStatusData | null>(null);

  const load = useCallback(async () => {
    setError(null);
    setLoading(true);
    try {
      const results = await Promise.allSettled([
        api.getDashboard(),
        api.getDashboardARAging(),
        api.getDashboardAPAging(),
        api.listRecentJournalEntries(8),
        api.getLowStockItems(),
        api.getDashboardPPNSummary(),
        api.getPeriodStatus(),
      ]);

      const dashResult = results[0].status === "fulfilled" ? results[0].value : null;
      setCashBalance(dashResult ? dashResult.cashAndBankBalance : null);
      setProfitLoss(dashResult ? dashResult.monthlyProfitLoss : null);

      setArAging(results[1].status === "fulfilled" ? results[1].value : null);
      setApAging(results[2].status === "fulfilled" ? results[2].value : null);
      setRecentTxns(results[3].status === "fulfilled" ? results[3].value : []);
      setLowStock(results[4].status === "fulfilled" ? results[4].value : []);
      setPpn(results[5].status === "fulfilled" ? results[5].value : null);
      setPeriod(results[6].status === "fulfilled" ? results[6].value : null);

      if (results[0].status === "rejected" && results.every((r) => r.status === "rejected")) {
        setError("Gagal memuat ringkasan dashboard. Silakan coba lagi.");
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Gagal memuat dashboard.");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [retryKey, load]);

  const businessName = business?.name || user?.businessName || "Entitas Usaha";

  const arTotal = arAging?.total_cents ?? 0;
  const apTotal = apAging?.total_cents ?? 0;

  // Real derived metrics
  const arOver90Ratio = arTotal > 0 ? ((arAging?.bucket_90_plus_cents ?? 0) / arTotal) * 100 : null;
  const apOver90Ratio = apTotal > 0 ? ((apAging?.bucket_90_plus_cents ?? 0) / apTotal) * 100 : null;

  // Quick Ratio computation: (Cash + AR) / AP; null bila AP = 0 (belum dapat dihitung)
  const quickRatio = apTotal > 0 ? ((cashBalance ?? 0) * 100 + arTotal) / apTotal : null;

  // Status periode aktual dari data widget (OPEN/CLOSED); null bila belum termuat
  const periodStatus = period?.status?.toUpperCase() ?? null;

  return (
    <div className="dashboard-container">
      {/* Executive Command Header */}
      <header className="dashboard-header">
        <div className="dashboard-header__left">
          <div className="flex items-center gap-2">
            <span className="live-pulse" />
            <span className="text-xs font-semibold text-muted uppercase">Pusat Kendali Finansial & Pembukuan</span>
          </div>
          <h1 className="text-2xl font-extrabold text-primary tracking-tight mt-1">
            {businessName}
          </h1>
          <p className="text-xs text-secondary mt-0.5">
            Periode Fiskal: <strong>{period?.period_start ? `${period.period_start} s/d ${period.period_end}` : "Aktif"}</strong> &bull; Status:{" "}
            {periodStatus === null ? (
              <span className="status-badge-inline">…</span>
            ) : periodStatus === "CLOSED" ? (
              <span
                className="status-badge-inline"
                style={{ backgroundColor: "var(--bg-surface-secondary)", color: "var(--text-secondary)", border: "1px solid var(--border-color)" }}
              >
                TERTUTUP (CLOSED)
              </span>
            ) : (
              <span className="status-badge-inline status-open">TERBUKA (OPEN)</span>
            )}
          </p>
        </div>

        <div className="dashboard-header__actions">
          <button
            type="button"
            className="btn-dash-primary"
            onClick={() => workbench.openEntryDraft("money-in")}
          >
            <Icon name="plus" size={16} />
            <span>+ Kas Masuk</span>
          </button>
          <button
            type="button"
            className="btn-dash-secondary"
            onClick={() => workbench.openEntryDraft("money-out")}
          >
            <Icon name="arrow_up_right" size={16} />
            <span>- Kas Keluar</span>
          </button>
          <button
            type="button"
            className="btn-dash-secondary"
            onClick={() => workbench.openEntryDraft("sales-invoice")}
          >
            <Icon name="receipt" size={16} />
            <span>+ Faktur Jual</span>
          </button>
        </div>
      </header>

      {error ? (
        <ErrorState message={error} onRetry={() => setRetryKey((k) => k + 1)} />
      ) : loading ? (
        <LoadingState label="Memuat indikator kesehatan finansial..." />
      ) : (
        <>
          {/* TIER 1: Financial Health & Liquidity Strip (5 KPI Cards) */}
          <section className="dashboard-kpi-row" aria-label="Financial Health Indicators">
            <div className="kpi-card-v2">
              <div className="kpi-card-v2__head">
                <span className="kpi-card-v2__label">Kas & Setara Kas</span>
                <span className="kpi-card-v2__icon bg-blue-50 text-blue-600">
                  <Icon name="wallet" size={16} />
                </span>
              </div>
              <div className="kpi-card-v2__value font-mono">
                {cashBalance !== null ? formatIDR(cashBalance) : "—"}
              </div>
              <div className="kpi-card-v2__sub" style={{ color: "var(--color-success-text)" }}>
                <Icon name="check" size={12} />
                <span>Saldo likuid siap pakai</span>
              </div>
            </div>

            <div className="kpi-card-v2">
              <div className="kpi-card-v2__head">
                <span className="kpi-card-v2__label">Laba Bersih MTD</span>
                <span className={`kpi-card-v2__icon ${profitLoss !== null && profitLoss >= 0 ? "bg-emerald-50 text-emerald-600" : "bg-rose-50 text-rose-600"}`}>
                  <Icon name={profitLoss !== null && profitLoss >= 0 ? "trending_up" : "trending_down"} size={16} />
                </span>
              </div>
              <div className={`kpi-card-v2__value font-mono ${profitLoss !== null && profitLoss >= 0 ? "text-emerald-600" : "text-rose-600"}`}>
                {profitLoss !== null ? formatIDR(profitLoss) : "—"}
              </div>
              <div className="kpi-card-v2__sub text-muted">
                <span>Pendapatan vs Beban Operasional</span>
              </div>
            </div>

            <div className="kpi-card-v2">
              <div className="kpi-card-v2__head">
                <span className="kpi-card-v2__label">Piutang Usaha (AR)</span>
                <span className="kpi-card-v2__icon bg-indigo-50 text-indigo-600">
                  <Icon name="tag" size={16} />
                </span>
              </div>
              <div className="kpi-card-v2__value font-mono text-primary">
                {formatIDR(arTotal)}
              </div>
              <div className="kpi-card-v2__sub text-indigo-600">
                <span>Total tagihan belum tertagih</span>
              </div>
            </div>

            <div className="kpi-card-v2">
              <div className="kpi-card-v2__head">
                <span className="kpi-card-v2__label">Hutang Usaha (AP)</span>
                <span className="kpi-card-v2__icon bg-amber-50 text-amber-600">
                  <Icon name="shopping_cart" size={16} />
                </span>
              </div>
              <div className="kpi-card-v2__value font-mono text-amber-700">
                {formatIDR(apTotal)}
              </div>
              <div className="kpi-card-v2__sub text-amber-700">
                <span>Kewajiban bayar ke supplier</span>
              </div>
            </div>

            <div className="kpi-card-v2">
              <div className="kpi-card-v2__head">
                <span className="kpi-card-v2__label">Quick Ratio Likuiditas</span>
                <span className="kpi-card-v2__icon bg-blue-50 text-blue-600">
                  <Icon name="security" size={16} />
                </span>
              </div>
              <div className="kpi-card-v2__value font-mono" style={{ color: "var(--brand-primary-hover)" }}>
                {quickRatio !== null ? `${quickRatio.toFixed(2)}x` : "—"}
              </div>
              {quickRatio !== null ? (
                <div className="mt-1">
                  <QuickRatioGauge value={quickRatio} />
                </div>
              ) : (
                <div className="kpi-card-v2__sub text-muted">
                  <span>Belum dapat dihitung</span>
                </div>
              )}
            </div>
          </section>

          {/* TIER 2 & 3: Cashflow, Aging Matrix & Live Transaksi */}
          <div className="dashboard-main-grid">
            {/* Left 8 Cols: Recent Posted Journals & Aging */}
            <div className="dashboard-main-left">
              {/* Recent Journals */}
              <div className="dash-card">
                <div className="dash-card__header">
                  <div className="flex items-center gap-2">
                    <Icon name="book_open" size={18} className="text-brand" />
                    <h2 className="dash-card__title">Mutasi Buku Jurnal Terkini (Posted Transactions)</h2>
                  </div>
                  <button
                    type="button"
                    className="dash-link-btn"
                    onClick={() => workbench.openList("journal-entry")}
                  >
                    <span>Buka Buku Besar</span>
                    <Icon name="arrow_forward" size={14} />
                  </button>
                </div>

                <div className="datatable-wrapper">
                  <table className="datatable">
                    <thead>
                      <tr>
                        <th>No. Jurnal</th>
                        <th>Tanggal</th>
                        <th>Keterangan Transaksi</th>
                        <th className="num">Total Debit</th>
                        <th className="num">Total Kredit</th>
                        <th>Status</th>
                      </tr>
                    </thead>
                    <tbody>
                      {recentTxns.length === 0 ? (
                        <tr>
                          <td colSpan={6} className="text-center py-6 text-muted">
                            Belum ada entri transaksi buku jurnal yang tercatat.
                          </td>
                        </tr>
                      ) : (
                        recentTxns.map((tx, i) => (
                          <tr key={i}>
                            <td className="font-mono font-semibold text-brand">{tx.number}</td>
                            <td className="font-mono">{tx.entry_date}</td>
                            <td>{tx.description || "—"}</td>
                            <td className="num font-mono cell-debit font-semibold text-emerald-700">{formatIDR(tx.total_debit_cents)}</td>
                            <td className="num font-mono cell-credit font-semibold text-slate-700">{formatIDR(tx.total_credit_cents)}</td>
                            <td>
                              <span className="status-badge-mini status-posted">POSTED</span>
                            </td>
                          </tr>
                        ))
                      )}
                    </tbody>
                  </table>
                </div>
              </div>

              {/* Aging Matrix (AR & AP) */}
              <div className="grid-2col gap-4 mt-4">
                <div className="dash-card">
                  <div className="dash-card__header">
                    <div className="flex items-center gap-2">
                      <h3 className="dash-card__title">Matrix Umur Piutang (AR Aging)</h3>
                      {arOver90Ratio !== null && (
                        <TrendPill deltaPct={-arOver90Ratio} label=">90 Hari" />
                      )}
                    </div>
                    <span className="font-mono text-xs font-bold text-primary">{formatIDR(arTotal)}</span>
                  </div>
                  <div className="mb-3">
                    <SegmentedAgingBar
                      buckets={{
                        b0_30: arAging?.bucket_1_30_cents ?? 0,
                        b31_60: arAging?.bucket_31_60_cents ?? 0,
                        b61_90: arAging?.bucket_61_90_cents ?? 0,
                        over90: arAging?.bucket_90_plus_cents ?? 0,
                      }}
                      formatCurrency={formatIDR}
                    />
                  </div>
                  <div className="aging-bars">
                    <div className="aging-bar-row">
                      <span className="aging-label">Lancar (0 - 30 Hari)</span>
                      <span className="aging-val font-mono">{formatIDR(arAging?.bucket_1_30_cents ?? 0)}</span>
                    </div>
                    <div className="aging-bar-row">
                      <span className="aging-label">31 - 60 Hari</span>
                      <span className="aging-val font-mono">{formatIDR(arAging?.bucket_31_60_cents ?? 0)}</span>
                    </div>
                    <div className="aging-bar-row">
                      <span className="aging-label">61 - 90 Hari</span>
                      <span className="aging-val font-mono">{formatIDR(arAging?.bucket_61_90_cents ?? 0)}</span>
                    </div>
                    <div className="aging-bar-row">
                      <span className="aging-label text-danger font-semibold">&gt; 90 Hari (Macet)</span>
                      <span className="aging-val font-mono text-danger font-bold">{formatIDR(arAging?.bucket_90_plus_cents ?? 0)}</span>
                    </div>
                  </div>
                </div>

                <div className="dash-card">
                  <div className="dash-card__header">
                    <div className="flex items-center gap-2">
                      <h3 className="dash-card__title">Matrix Umur Hutang (AP Aging)</h3>
                      {apOver90Ratio !== null && (
                        <TrendPill deltaPct={-apOver90Ratio} label=">90 Hari" />
                      )}
                    </div>
                    <span className="font-mono text-xs font-bold text-amber-700">{formatIDR(apTotal)}</span>
                  </div>
                  <div className="mb-3">
                    <SegmentedAgingBar
                      buckets={{
                        b0_30: apAging?.bucket_1_30_cents ?? 0,
                        b31_60: apAging?.bucket_31_60_cents ?? 0,
                        b61_90: apAging?.bucket_61_90_cents ?? 0,
                        over90: apAging?.bucket_90_plus_cents ?? 0,
                      }}
                      formatCurrency={formatIDR}
                    />
                  </div>
                  <div className="aging-bars">
                    <div className="aging-bar-row">
                      <span className="aging-label">Jatuh Tempo 0 - 30 Hari</span>
                      <span className="aging-val font-mono">{formatIDR(apAging?.bucket_1_30_cents ?? 0)}</span>
                    </div>
                    <div className="aging-bar-row">
                      <span className="aging-label">31 - 60 Hari</span>
                      <span className="aging-val font-mono">{formatIDR(apAging?.bucket_31_60_cents ?? 0)}</span>
                    </div>
                    <div className="aging-bar-row">
                      <span className="aging-label">61 - 90 Hari</span>
                      <span className="aging-val font-mono">{formatIDR(apAging?.bucket_61_90_cents ?? 0)}</span>
                    </div>
                    <div className="aging-bar-row">
                      <span className="aging-label text-danger font-semibold">&gt; 90 Hari (Menunggak)</span>
                      <span className="aging-val font-mono text-danger font-bold">{formatIDR(apAging?.bucket_90_plus_cents ?? 0)}</span>
                    </div>
                  </div>
                </div>
              </div>
            </div>

            {/* Right 4 Cols: Quick Dock, Low Stock & Tax Summary */}
            <div className="dashboard-main-right">
              {/* Quick Action Dock */}
              <div className="dash-card bg-surface">
                <div className="dash-card__header">
                  <h3 className="dash-card__title">Pintasan Entri Cepat (Quick Dock)</h3>
                </div>
                <div className="quick-dock-grid">
                  <button
                    type="button"
                    className="quick-dock-btn"
                    onClick={() => workbench.openEntryDraft("money-in")}
                  >
                    <Icon name="arrow_down_left" size={18} className="text-success" />
                    <span>+ Kas Masuk</span>
                  </button>
                  <button
                    type="button"
                    className="quick-dock-btn"
                    onClick={() => workbench.openEntryDraft("money-out")}
                  >
                    <Icon name="arrow_up_right" size={18} className="text-danger" />
                    <span>- Kas Keluar</span>
                  </button>
                  <button
                    type="button"
                    className="quick-dock-btn"
                    onClick={() => workbench.openEntryDraft("cash-transfer")}
                  >
                    <Icon name="refresh" size={18} className="text-brand" />
                    <span>Transfer Bank</span>
                  </button>
                  <button
                    type="button"
                    className="quick-dock-btn"
                    onClick={() => workbench.openEntryDraft("journal-entry")}
                  >
                    <Icon name="book_open" size={18} />
                    <span>Jurnal Manual</span>
                  </button>
                </div>
              </div>

              {/* Tax Summary MTD */}
              <div className="dash-card mt-4">
                <div className="dash-card__header">
                  <h3 className="dash-card__title">Pengawasan Pajak MTD</h3>
                  <span className="status-badge-mini status-open">PPN 11%</span>
                </div>
                <div className="tax-summary-body">
                  <div className="tax-row">
                    <span className="text-xs text-secondary">PPN Kurang/(Lebih) Bayar:</span>
                    <span className="font-mono font-bold text-brand">
                      {formatIDR(ppn?.net_ppn_cents ?? 0)}
                    </span>
                  </div>
                </div>
              </div>

              {/* Low Stock Alert */}
              <div className="dash-card mt-4">
                <div className="dash-card__header">
                  <div className="flex items-center gap-1.5">
                    <Icon name="warning" size={16} className="text-amber-500" />
                    <h3 className="dash-card__title">Peringatan Stok Menipis</h3>
                  </div>
                  <span className="text-xs font-mono font-bold text-muted">{lowStock.length} Item</span>
                </div>
                <div className="low-stock-list">
                  {lowStock.length === 0 ? (
                    <p className="text-xs text-muted py-2">Semua persediaan barang berada di atas batas minimum.</p>
                  ) : (
                    lowStock.slice(0, 4).map((item, idx) => (
                      <div key={idx} className="low-stock-item">
                        <div>
                          <p className="text-xs font-semibold text-primary">{item.name || "Item"}</p>
                          <p className="text-10 text-muted font-mono">{item.code || "—"}</p>
                        </div>
                        <div className="text-right">
                          <span className="text-xs font-mono font-bold text-danger">{item.qty_on_hand ?? 0}</span>
                          <span className="text-10 text-muted"> / min {item.min_stock_qty ?? 0}</span>
                        </div>
                      </div>
                    ))
                  )}
                </div>
              </div>
            </div>
          </div>
        </>
      )}
    </div>
  );
}
