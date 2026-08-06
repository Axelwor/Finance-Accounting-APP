import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { api } from "../api";
import { useAppState } from "../state";
import { Button, FormError, RupiahField, SelectField, TextField } from "../components/ui";
import type { CurrencyCode } from "../types";
import { formatRupiah } from "../lib/format";

type SaldoKey = "kas" | "bank" | "piutang" | "hutang" | "modal";

const SALDO_FIELDS: { key: SaldoKey; label: string; hint?: string }[] = [
  { key: "kas", label: "Uang tunai di kas", hint: "Uang fisik yang ada di tangan." },
  { key: "bank", label: "Saldo rekening bank" },
  { key: "piutang", label: "Piutang (uang yang akan diterima)" },
  { key: "hutang", label: "Hutang (uang yang harus dibayar)" },
  { key: "modal", label: "Modal awal usaha" },
];

const JENIS_USAHA = [
  "Warung / toko kelontong",
  "Kafe / rumah makan",
  "Online shop",
  "Jasa",
  "Bengkel",
  "Lainnya",
];

const MATA_UANG: { value: CurrencyCode; label: string }[] = [{ value: "IDR", label: "Rupiah (Rp)" }];

/** Alur onboarding 3 langkah: data usaha, periode buku, saldo awal ringkas. */
export function OnboardingScreen() {
  const [step, setStep] = useState(0);
  const [nama, setNama] = useState("");
  const [jenisUsaha, setJenisUsaha] = useState("");
  const [mataUang, setMataUang] = useState<CurrencyCode>("IDR");
  const [tahun, setTahun] = useState(String(new Date().getFullYear()));
  const [bulanMulai, setBulanMulai] = useState("1");
  const [saldo, setSaldo] = useState<Record<SaldoKey, string>>({
    kas: "",
    bank: "",
    piutang: "",
    hutang: "",
    modal: "",
  });
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [finishing, setFinishing] = useState(false);

  const navigate = useNavigate();
  const { setUsaha } = useAppState();

  const jumlahHarta = (saldo.kas ? Number(saldo.kas) : 0) + (saldo.bank ? Number(saldo.bank) : 0) + (saldo.piutang ? Number(saldo.piutang) : 0);
  const jumlahKewajibanModal = (saldo.hutang ? Number(saldo.hutang) : 0) + (saldo.modal ? Number(saldo.modal) : 0);

  const bulanLabels = [
    "Januari",
    "Februari",
    "Maret",
    "April",
    "Mei",
    "Juni",
    "Juli",
    "Agustus",
    "September",
    "Oktober",
    "November",
    "Desember",
  ];

  const stepValid = (): boolean => {
    if (step === 0) return nama.trim().length > 0 && jenisUsaha.trim().length > 0;
    if (step === 1) {
      const t = Number(tahun);
      return t >= 2000 && t <= 2100 && bulanMulai !== "";
    }
    return true;
  };

  const lanjut = () => {
    setError(null);
    if (step === 0 && !stepValid()) {
      setError("Lengkapi nama usaha dan jenis usaha untuk melanjutkan.");
      return;
    }
    if (step < 2) setStep((s) => s + 1);
  };

  const handleFinish = async () => {
    setError(null);
    setFinishing(true);
    try {
      await api.completeOnboarding({
        usaha: { nama: nama.trim(), jenisUsaha: jenisUsaha.trim(), mataUang },
        periode: { tahun: Number(tahun), mulaiBulan: Number(bulanMulai) },
        saldoAwal: {
          kas: Number(saldo.kas || 0),
          bank: Number(saldo.bank || 0),
          piutang: Number(saldo.piutang || 0),
          hutang: Number(saldo.hutang || 0),
          modal: Number(saldo.modal || 0),
        },
      });
      const state = api.getLocalState();
      setUsaha(state.usaha);
      setLoading(true);
      navigate("/dashboard", { replace: true });
    } catch (err) {
      setError(err instanceof Error ? err.message : "Terjadi kesalahan. Coba lagi.");
      setFinishing(false);
    }
  };

  const setSaldoField = (key: SaldoKey, raw: string) => {
    setSaldo((s) => ({ ...s, [key]: raw.replace(/[^\d]/g, "").slice(0, 15) }));
  };

  const renderStep = () => {
    if (step === 0) {
      return (
        <div className="form-stack">
          <TextField label="Nama usaha" value={nama} onChange={setNama} placeholder="mis. Warung Bu Sari" />
          <SelectField
            label="Jenis usaha"
            value={jenisUsaha}
            onChange={setJenisUsaha}
            options={JENIS_USAHA.map((j) => ({ value: j, label: j }))}
            placeholder="Pilih jenis usaha"
          />
          <SelectField
            label="Mata uang"
            value={mataUang}
            onChange={(v) => setMataUang(v as CurrencyCode)}
            options={MATA_UANG}
          />
        </div>
      );
    }
    if (step === 1) {
      return (
        <div className="form-stack">
          <TextField
            label="Tahun buku"
            value={tahun}
            onChange={(v) => setTahun(v.replace(/[^\d]/g, "").slice(0, 4))}
            inputMode="numeric"
            hint="Tahun saat pembukuan dimulai."
          />
          <SelectField
            label="Periode buku dimulai pada bulan"
            value={bulanMulai}
            onChange={setBulanMulai}
            options={bulanLabels.map((b, i) => ({ value: String(i + 1), label: b }))}
          />
          <p className="field-note">
            Periode buku: {bulanLabels[Number(bulanMulai) - 1]} {tahun} sampai{" "}
            {bulanLabels[(Number(bulanMulai) + 10) % 12]} {Number(bulanMulai) === 1 ? Number(tahun) : Number(tahun) + 1}.
          </p>
        </div>
      );
    }
    return (
      <div className="form-stack">
        {SALDO_FIELDS.map((f) => (
          <RupiahField
            key={f.key}
            label={f.label}
            value={saldo[f.key]}
            onChange={(raw) => setSaldoField(f.key, raw)}
            hint={f.hint}
          />
        ))}
        <div className="saldo-ringkasan">
          <p className="saldo-ringkasan__label">Ringkasan</p>
          <div className="saldo-ringkasan__row">
            <span>Total harta (kas + bank + piutang)</span>
            <strong>{formatRupiah(jumlahHarta)}</strong>
          </div>
          <div className="saldo-ringkasan__row">
            <span>Hutang + modal</span>
            <strong>{formatRupiah(jumlahKewajibanModal)}</strong>
          </div>
          <p className="saldo-ringkasan__note">
            Perbedaan kecil antara kedua angka akan diseimbangkan otomatis oleh sistem.
          </p>
        </div>
      </div>
    );
  };

  const stepMeta = [
    { label: "Data usaha", desc: "Nama, jenis, dan mata uang." },
    { label: "Periode buku", desc: "Kapan tahun buku dimulai." },
    { label: "Saldo awal", desc: "Ringkasan posisi keuangan hari pertama." },
  ];

  return (
    <div className="onboarding">
      <div className="onboarding__head">
        <p className="onboarding__brand">
          <span className="brand__mark" aria-hidden="true" />
          <span className="brand__name">Pembukuan Mudah</span>
        </p>
        <h1 className="onboarding__title">Siapkan buku usaha Anda</h1>
        <p className="onboarding__sub">Tiga langkah singkat, sekitar 2 menit.</p>
      </div>

      <ol className="stepper" aria-label="Langkah onboarding">
        {stepMeta.map((s, i) => (
          <li key={s.label} className={`stepper__item${i === step ? " is-active" : i < step ? " is-done" : ""}`}>
            <span className="stepper__num" aria-hidden="true">
              {i < step ? "✓" : i + 1}
            </span>
            <span className="stepper__label">{s.label}</span>
          </li>
        ))}
      </ol>

      <form
        className="onboarding-card"
        onSubmit={(e) => {
          e.preventDefault();
          if (step === 2) void handleFinish();
          else lanjut();
        }}
        noValidate
      >
        <div className="onboarding-card__head">
          <h2 className="onboarding-card__title">{stepMeta[step].label}</h2>
          <p className="onboarding-card__desc">{stepMeta[step].desc}</p>
        </div>

        {renderStep()}
        <FormError message={error} />

        <div className="onboarding-card__actions">
          {step > 0 ? (
            <Button variant="secondary" onClick={() => setStep((s) => s - 1)}>
              Kembali
            </Button>
          ) : (
            <span />
          )}
          <Button type="submit" variant="primary" disabled={loading || finishing}>
            {step < 2 ? "Lanjut" : finishing ? "Menyimpan..." : "Selesai, buka dashboard"}
          </Button>
        </div>
      </form>
    </div>
  );
}
