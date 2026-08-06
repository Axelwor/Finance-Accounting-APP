import type { Transaction } from "../types";
import { formatRupiah, formatTanggalRelatif } from "../lib/format";

const JENIS_LABEL: Record<Transaction["jenis"], string> = {
  "uang-masuk": "Uang masuk",
  "uang-keluar": "Uang keluar",
  "pindah-uang": "Pindah uang",
};

interface TransactionRowProps {
  transaksi: Transaction;
  /** Elemen aksi opsional di sisi kanan baris (mis. tombol hapus). */
  action?: React.ReactNode;
}

/** Satu baris dalam daftar transaksi. */
export function TransactionRow({ transaksi, action }: TransactionRowProps) {
  const positif = transaksi.jenis === "uang-masuk";
  const netral = transaksi.jenis === "pindah-uang";

  return (
    <li className="transaction-row">
      <span className={`transaction-row__badge transaction-row__badge--${transaksi.jenis}`}>
        {transaksi.jenis === "uang-masuk"
          ? "Masuk"
          : transaksi.jenis === "uang-keluar"
            ? "Keluar"
            : "Pindah"}
      </span>
      <div className="transaction-row__body">
        <p className="transaction-row__keterangan">
          {transaksi.keterangan || JENIS_LABEL[transaksi.jenis]}
        </p>
        <p className="transaction-row__meta">
          {transaksi.kategoriNama ?? JENIS_LABEL[transaksi.jenis]}
          <span aria-hidden="true"> · </span>
          {formatTanggalRelatif(transaksi.tanggal)}
        </p>
      </div>
      <div className="transaction-row__amount">
        <p className={`transaction-row__nominal${positif ? " is-positif" : netral ? " is-netral" : ""}`}>
          {positif ? "+" : netral ? "" : "-"}
          {formatRupiah(transaksi.nominal)}
        </p>
        {transaksi.dari && transaksi.ke ? (
          <p className="transaction-row__meta">
            {transaksi.dari} ke {transaksi.ke}
          </p>
        ) : null}
      </div>
      {action}
    </li>
  );
}

/** Daftar transaksi lengkap (digunakan pada halaman Catatan). */
export function TransactionList({ transaksi, onHapus }: { transaksi: Transaction[]; onHapus?: (id: string) => void }) {
  return (
    <ul className="transaction-list">
      {transaksi.map((t) => (
        <TransactionRow
          key={t.id}
          transaksi={t}
          action={
            onHapus ? (
              <button
                type="button"
                className="transaction-row__hapus"
                aria-label={`Hapus catatan ${t.keterangan || "tanpa keterangan"}`}
                onClick={() => onHapus(t.id)}
              >
                Hapus
              </button>
            ) : undefined
          }
        />
      ))}
    </ul>
  );
}
