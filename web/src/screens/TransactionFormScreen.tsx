import { useCallback, useEffect, useState, type FormEvent } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { api } from "../api";
import { useAppState } from "../state";
import {
  Button,
  DateField,
  ErrorState,
  FormError,
  LoadingState,
  RupiahField,
  SelectField,
  TextareaField,
} from "../components/ui";
import { todayISO } from "../lib/format";
import type { AccountItem, Category, TransactionKind } from "../types";

const JENIS_META: Record<
  TransactionKind,
  { title: string; sub: string; icon: string; submit: string }
> = {
  "uang-masuk": {
    title: "Catat uang masuk",
    sub: "Uang yang diterima usaha, misalnya hasil penjualan.",
    icon: "+",
    submit: "Simpan uang masuk",
  },
  "uang-keluar": {
    title: "Catat uang keluar",
    sub: "Uang yang dikeluarkan usaha, misalnya belanja atau sewa.",
    icon: "-",
    submit: "Simpan uang keluar",
  },
  "pindah-uang": {
    title: "Pindah uang",
    sub: "Memindahkan uang antar rekening atau kas, tanpa untung maupun rugi.",
    icon: "⇄",
    submit: "Simpan pemindahan",
  },
};

/** Form input transaksi: uang masuk / uang keluar / pindah uang. */
export function TransactionFormScreen() {
  const { jenisParam } = useParams<{ jenisParam: string }>();
  const jenis: TransactionKind =
    jenisParam === "uang-keluar"
      ? "uang-keluar"
      : jenisParam === "pindah-uang"
        ? "pindah-uang"
        : "uang-masuk";
  const meta = JENIS_META[jenis];

  const [nominal, setNominal] = useState("");
  const [tanggal, setTanggal] = useState(todayISO());
  const [keterangan, setKeterangan] = useState("");
  const [kategoriId, setKategoriId] = useState("");
  const [dari, setDari] = useState("");
  const [ke, setKe] = useState("");

  const [categories, setCategories] = useState<Category[]>([]);
  const [accounts, setAccounts] = useState<AccountItem[]>([]);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);

  const navigate = useNavigate();
  const { setTransactions } = useAppState();

  useEffect(() => {
    document.title = `${meta.title} - Pembukuan Mudah`;
  }, [meta.title]);

  const muatMaster = useCallback(async () => {
    setLoadError(null);
    try {
      const [kategori, rekening] = await Promise.all([
        api.listCategories(),
        api.listAccounts(),
      ]);
      setCategories(kategori);
      setAccounts(rekening);
    } catch (err) {
      setLoadError(err instanceof Error ? err.message : "Gagal memuat data. Coba lagi.");
    }
  }, []);

  useEffect(() => {
    void muatMaster();
  }, [muatMaster, jenis]);

  const kategoriOptions =
    jenis === "pindah-uang"
      ? []
      : categories.filter((c) => c.jenis === jenis).map((c) => ({ value: c.id, label: c.nama }));

  const accountOptions = accounts.map((a) => ({ value: a.id, label: a.nama }));

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setError(null);
    setSaving(true);
    setSaved(false);
    try {
      const { transaksiTerbaru } = await api.createTransaction({
        jenis,
        nominal: Number(nominal || 0),
        tanggal,
        keterangan,
        kategoriId: jenis === "pindah-uang" ? undefined : kategoriId || undefined,
        dari: jenis === "pindah-uang" ? dari || undefined : undefined,
        ke: jenis === "pindah-uang" ? ke || undefined : undefined,
      });
      setTransactions(transaksiTerbaru);
      setSaved(true);
      setNominal("");
      setKeterangan("");
      setKategoriId("");
      setDari("");
      setKe("");
      setTimeout(() => navigate("/dashboard"), 1100);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Terjadi kesalahan. Coba lagi.");
      setSaving(false);
    }
  };

  return (
    <div className="form-page">
      <header className="page-head">
        <div>
          <h1 className="page-title">{meta.title}</h1>
          <p className="page-sub">{meta.sub}</p>
        </div>
      </header>

      {loadError ? (
        <ErrorState message={loadError} onRetry={() => void muatMaster()} />
      ) : categories.length === 0 ? (
        <LoadingState label="Menyiapkan form..." />
      ) : (
        <form className="form-card" onSubmit={handleSubmit} noValidate>
          <RupiahField label="Nominal (Rp)" value={nominal} onChange={setNominal} placeholder="0" />

          <DateField label="Tanggal" value={tanggal} onChange={setTanggal} max={todayISO()} />

          {jenis === "pindah-uang" ? (
            <div className="field-row">
              <SelectField label="Dari rekening" value={dari} onChange={setDari} options={accountOptions} placeholder="Pilih rekening" />
              <SelectField label="Ke rekening" value={ke} onChange={setKe} options={accountOptions} placeholder="Pilih rekening" />
            </div>
          ) : (
            <SelectField
              label="Kategori"
              value={kategoriId}
              onChange={setKategoriId}
              options={kategoriOptions}
              placeholder="Pilih kategori"
            />
          )}

          <TextareaField
            label="Keterangan"
            value={keterangan}
            onChange={setKeterangan}
            placeholder="mis. Penjualan tunai hari ini"
            hint="Catatan singkat agar mudah dikenali nanti."
          />

          <FormError message={error} />

          <div className="form-card__actions">
            <Button variant="secondary" to="/dashboard">
              Batal
            </Button>
            <Button type="submit" variant="primary" disabled={saving || saved}>
              {saving ? "Menyimpan..." : saved ? "Tersimpan ✓" : meta.submit}
            </Button>
          </div>

          {saved ? (
            <p className="form-card__success" role="status">
              Tersimpan. Mengarahkan ke ringkasan...
            </p>
          ) : null}
        </form>
      )}
    </div>
  );
}
