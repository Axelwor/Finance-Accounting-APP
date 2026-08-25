import { useEffect, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { api } from "../api";
import { useAppState } from "../state";
import { Icon } from "../components/m3/Icon";
import { formatIDR } from "../lib/format";
import type { CurrencyCode } from "../types";

type BalanceKey = "cash" | "bank" | "receivables" | "payables" | "equity";

const BALANCE_FIELDS: { key: BalanceKey; label: string; hint: string; type: "debit" | "credit" }[] = [
  { key: "cash", label: "Kas Fisik (Cash on Hand)", hint: "Uang tunai di kasir/brankas", type: "debit" },
  { key: "bank", label: "Rekening Bank Utama", hint: "Saldo giro/tabungan operasional", type: "debit" },
  { key: "receivables", label: "Piutang Usaha (AR)", hint: "Tagihan yang belum dibayar pelanggan", type: "debit" },
  { key: "payables", label: "Hutang Usaha (AP)", hint: "Kewajiban tagihan kepada pemasok", type: "credit" },
  { key: "equity", label: "Modal Pemilik / Laba Ditahan", hint: "Ekuitas awal penyeimbang neraca", type: "credit" },
];

const BUSINESS_TYPES = [
  "Perdagangan / Retail / Grosir",
  "Restoran / Cafe / F&B",
  "Manufaktur & Pabrikasi",
  "Jasa Konsultan & Profesional",
  "Kontraktor & Konstruksi",
  "Logistik & Transportasi",
  "Lainnya",
];

const MONTHS = [
  "Januari", "Februari", "Maret", "April", "Mei", "Juni",
  "Juli", "Agustus", "September", "Oktober", "November", "Desember",
];

export function OnboardingScreen() {
  const [step, setStep] = useState(0);
  const [name, setName] = useState("");
  const [businessType, setBusinessType] = useState(BUSINESS_TYPES[0]);
  const [currency] = useState<CurrencyCode>("IDR");
  const [year, setYear] = useState(String(new Date().getFullYear()));
  const [startMonth, setStartMonth] = useState("1");
  const [balance, setBalance] = useState<Record<BalanceKey, string>>({
    cash: "",
    bank: "",
    receivables: "",
    payables: "",
    equity: "",
  });
  const [error, setError] = useState<string | null>(null);
  const [finishing, setFinishing] = useState(false);

  const navigate = useNavigate();
  const { setBusiness } = useAppState();

  // Move keyboard focus to the new step's heading when the step changes
  // (skipped on first mount so initial focus stays in the browser/page).
  const stepHeadingRef = useRef<HTMLHeadingElement>(null);
  const mountedRef = useRef(false);
  useEffect(() => {
    if (mountedRef.current) stepHeadingRef.current?.focus();
    mountedRef.current = true;
  }, [step]);

  const totalDebit =
    (balance.cash ? Number(balance.cash) : 0) +
    (balance.bank ? Number(balance.bank) : 0) +
    (balance.receivables ? Number(balance.receivables) : 0);

  const totalCredit =
    (balance.payables ? Number(balance.payables) : 0) +
    (balance.equity ? Number(balance.equity) : 0);

  const difference = totalDebit - totalCredit;
  const isBalanced = difference === 0 && (totalDebit > 0 || totalCredit === 0);

  const autoBalanceEquity = () => {
    // Solves for equity so that: Assets = Liabilities + Equity (totalDebit = totalCredit)
    const netAssets = (balance.cash ? Number(balance.cash) : 0) +
      (balance.bank ? Number(balance.bank) : 0) +
      (balance.receivables ? Number(balance.receivables) : 0) -
      (balance.payables ? Number(balance.payables) : 0);

    setBalance(prev => ({
      ...prev,
      equity: String(Math.max(0, netAssets))
    }));
  };

  const handleFinish = async () => {
    setError(null);
    // F-16: block unbalanced opening balances instead of silently plugging
    // the difference into the backend.
    if (!isBalanced) {
      setError(
        `Selisih ${formatIDR(Math.abs(difference))} — isi Modal/Kewajiban agar seimbang, atau set semua ke 0.`
      );
      return;
    }
    setFinishing(true);
    try {
      await api.completeOnboarding({
        business: { name: name.trim(), businessType: businessType.trim(), currency },
        period: { year: Number(year), startMonth: Number(startMonth) },
        openingBalance: {
          cash: Number(balance.cash || 0),
          bank: Number(balance.bank || 0),
          receivables: Number(balance.receivables || 0),
          payables: Number(balance.payables || 0),
          equity: Number(balance.equity || 0),
        },
      });
      const state = api.getLocalState();
      setBusiness(state.business);
      navigate("/", { replace: true });
    } catch (err: unknown) {
      const msg = typeof err === "object" && err !== null && "message" in err
        ? (err as { message: string }).message
        : "Gagal menyelesaikan konfigurasi.";
      setError(msg);
    } finally {
      setFinishing(false);
    }
  };

  return (
    <div className="onboarding-page">
      <header className="onboarding-header">
        <div className="onboarding-brand">
          <div className="brand-badge">
            <Icon name="book_open" size={20} className="text-white" />
          </div>
          <span className="brand-name">Ledgerly Setup Wizard</span>
        </div>
        <div className="onboarding-steps-indicator">
          <div className={`step-pill ${step >= 0 ? "is-active" : ""}`} aria-current={step === 0 ? "step" : undefined}>
            <span className="step-num">1</span>
            <span>Profil Usaha</span>
          </div>
          <div className="step-line" />
          <div className={`step-pill ${step >= 1 ? "is-active" : ""}`} aria-current={step === 1 ? "step" : undefined}>
            <span className="step-num">2</span>
            <span>Periode Buku</span>
          </div>
          <div className="step-line" />
          <div className={`step-pill ${step >= 2 ? "is-active" : ""}`} aria-current={step === 2 ? "step" : undefined}>
            <span className="step-num">3</span>
            <span>Neraca Awal</span>
          </div>
        </div>
      </header>

      <main className="onboarding-body">
        <div className="onboarding-card">
          {error && (
            <div className="auth-error-alert" role="alert">
              <Icon name="error" size={16} />
              <span>{error}</span>
            </div>
          )}

          {step === 0 && (
            <div className="wizard-step">
              <div className="wizard-step__header">
                <h2 ref={stepHeadingRef} tabIndex={-1}>Langkah 1: Identitas & Bidang Usaha</h2>
                <p>Tentukan nama entitas resmi dan jenis industri untuk struktur bagan akun otomatis.</p>
              </div>

              <div className="wizard-fields">
                <div className="auth-field">
                  <label htmlFor="company-name">Nama Entitas / Perusahaan *</label>
                  <input
                    id="company-name"
                    type="text"
                    required
                    className="input-base"
                    placeholder="Contoh: PT Surya Niaga Mandiri"
                    value={name}
                    onChange={(e) => setName(e.target.value)}
                  />
                </div>

                <div className="auth-field">
                  <label htmlFor="business-type">Sektor / Bidang Industri</label>
                  <select
                    id="business-type"
                    className="input-base"
                    value={businessType}
                    onChange={(e) => setBusinessType(e.target.value)}
                  >
                    {BUSINESS_TYPES.map((t) => (
                      <option key={t} value={t}>{t}</option>
                    ))}
                  </select>
                </div>
              </div>

              <div className="wizard-actions">
                <div />
                <button
                  type="button"
                  className="btn-wizard-next"
                  disabled={!name.trim()}
                  onClick={() => setStep(1)}
                >
                  <span>Lanjutkan ke Periode Buku</span>
                  <Icon name="arrow_forward" size={16} />
                </button>
              </div>
            </div>
          )}

          {step === 1 && (
            <div className="wizard-step">
              <div className="wizard-step__header">
                <h2 ref={stepHeadingRef} tabIndex={-1}>Langkah 2: Konfigurasi Tahun & Periode Fiskal</h2>
                <p>Tentukan tahun buku aktif dan bulan awal pembukuan akuntansi.</p>
              </div>

              <div className="wizard-fields form-grid-2col">
                <div className="auth-field">
                  <label htmlFor="fiscal-year">Tahun Buku Fiskal *</label>
                  <input
                    id="fiscal-year"
                    type="number"
                    min="2000"
                    max="2100"
                    className="input-base font-mono"
                    value={year}
                    onChange={(e) => setYear(e.target.value)}
                  />
                </div>

                <div className="auth-field">
                  <label htmlFor="start-month">Bulan Mulai Pembukuan</label>
                  <select
                    id="start-month"
                    className="input-base"
                    value={startMonth}
                    onChange={(e) => setStartMonth(e.target.value)}
                  >
                    {MONTHS.map((m, idx) => (
                      <option key={m} value={String(idx + 1)}>{m}</option>
                    ))}
                  </select>
                </div>
              </div>

              <div className="wizard-actions">
                <button
                  type="button"
                  className="btn-wizard-prev"
                  onClick={() => setStep(0)}
                >
                  <Icon name="arrow_back" size={16} />
                  <span>Kembali</span>
                </button>
                <button
                  type="button"
                  className="btn-wizard-next"
                  onClick={() => setStep(2)}
                >
                  <span>Lanjutkan ke Neraca Saldo Awal</span>
                  <Icon name="arrow_forward" size={16} />
                </button>
              </div>
            </div>
          )}

          {step === 2 && (
            <div className="wizard-step">
              <div className="wizard-step__header">
                <div className="flex-between">
                  <div>
                    <h2 ref={stepHeadingRef} tabIndex={-1}>Langkah 3: Neraca Saldo Hari Pertama (Day-One Solver)</h2>
                    <p>Masukkan saldo kas, piutang, dan hutang per tanggal mulai pembukuan.</p>
                  </div>
                  <button
                    type="button"
                    className="btn-auto-balance"
                    onClick={autoBalanceEquity}
                    title="Seimbangkan modal secara otomatis"
                  >
                    <Icon name="refresh" size={14} />
                    <span>Seimbangkan Modal Otomatis</span>
                  </button>
                </div>
              </div>

              <div className="balance-grid">
                {BALANCE_FIELDS.map((f) => (
                  <div key={f.key} className="balance-row">
                    <div className="balance-info">
                      <span className="balance-label">{f.label}</span>
                      <span className="balance-hint">{f.hint}</span>
                    </div>
                    <div className="balance-input-wrapper">
                      <span className="currency-prefix">Rp</span>
                      <input
                        type="number"
                        className="balance-input font-mono"
                        placeholder="0"
                        value={balance[f.key]}
                        onChange={(e) =>
                          setBalance((b) => ({ ...b, [f.key]: e.target.value }))
                        }
                      />
                    </div>
                  </div>
                ))}
              </div>

              <div className="balance-summary-strip">
                <div className="summary-col">
                  <span className="summary-label">Total Aset (Debit):</span>
                  <span className="summary-val font-mono">{formatIDR(totalDebit)}</span>
                </div>
                <div className="summary-col">
                  <span className="summary-label">Total Kewajiban & Modal (Kredit):</span>
                  <span className="summary-val font-mono">{formatIDR(totalCredit)}</span>
                </div>
                <div className="summary-col">
                  <span className="summary-label">Status Keseimbangan:</span>
                  {isBalanced ? (
                    <span className="status-badge status-balanced">
                      <Icon name="check" size={14} /> Seimbang (100%)
                    </span>
                  ) : (
                    <span className="status-badge status-unbalanced">
                      Selisih: {formatIDR(Math.abs(difference))}
                    </span>
                  )}
                </div>
              </div>

              <div className="wizard-actions">
                <button
                  type="button"
                  className="btn-wizard-prev"
                  onClick={() => setStep(1)}
                >
                  <Icon name="arrow_back" size={16} />
                  <span>Kembali</span>
                </button>
                <div className="flex flex-col items-end gap-1">
                  {!isBalanced && (
                    <p
                      role="alert"
                      style={{ margin: 0, fontSize: "0.8rem", color: "var(--md-sys-color-error, #b3261e)", textAlign: "right", maxWidth: 360 }}
                    >
                      Selisih {formatIDR(Math.abs(difference))} — isi Modal/Kewajiban agar seimbang, atau set semua ke 0.
                    </p>
                  )}
                  <button
                    type="button"
                    className="btn-wizard-finish"
                    // F-16: never plug an unbalanced opening balance silently.
                    disabled={!isBalanced || finishing}
                    onClick={handleFinish}
                  >
                    {finishing ? (
                      <span>Menyimpan Konfigurasi...</span>
                    ) : (
                      <>
                        <Icon name="check" size={16} />
                        <span>Selesaikan Setup & Buka Dashboard</span>
                      </>
                    )}
                  </button>
                </div>
              </div>
            </div>
          )}
        </div>
      </main>
    </div>
  );
}
