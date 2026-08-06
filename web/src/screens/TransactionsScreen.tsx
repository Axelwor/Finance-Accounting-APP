import { useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { useAppState } from "../state";
import { Button, EmptyState } from "../components/ui";
import { TransactionList } from "../components/transactions";
import type { TransactionKind } from "../types";

const FILTERS: { value: TransactionKind | "semua"; label: string }[] = [
  { value: "semua", label: "Semua" },
  { value: "uang-masuk", label: "Uang masuk" },
  { value: "uang-keluar", label: "Uang keluar" },
  { value: "pindah-uang", label: "Pindah uang" },
];

/** Daftar seluruh catatan transaksi dengan filter dan penghapusan lokal. */
export function TransactionsScreen() {
  const { transactions, setTransactions } = useAppState();
  const [filter, setFilter] = useState<TransactionKind | "semua">("semua");

  useEffect(() => {
    document.title = "Catatan - Pembukuan Mudah";
  }, []);

  const tampil = useMemo(() => {
    const urut = [...transactions].sort((a, b) => b.tanggal.localeCompare(a.tanggal) || b.createdAt.localeCompare(a.createdAt));
    return filter === "semua" ? urut : urut.filter((t) => t.jenis === filter);
  }, [transactions, filter]);

  const hapus = (id: string) => {
    setTransactions(transactions.filter((t) => t.id !== id));
  };

  return (
    <div className="list-page">
      <header className="page-head">
        <div>
          <h1 className="page-title">Catatan transaksi</h1>
          <p className="page-sub">Semua uang masuk, uang keluar, dan pemindahan.</p>
        </div>
      </header>

      <div className="filter-row" role="group" aria-label="Filter catatan">
        {FILTERS.map((f) => (
          <button
            key={f.value}
            type="button"
            className={`filter-chip${filter === f.value ? " is-active" : ""}`}
            aria-pressed={filter === f.value}
            onClick={() => setFilter(f.value)}
          >
            {f.label}
          </button>
        ))}
      </div>

      <div className="list-card">
        {tampil.length === 0 ? (
          <EmptyState
            title={filter === "semua" ? "Belum ada catatan" : "Tidak ada catatan untuk filter ini"}
            message="Catatan uang masuk, uang keluar, atau pemindahan akan muncul di sini."
            action={
              <Link className="btn btn--primary" to="/catat/uang-masuk">
                Catat transaksi
              </Link>
            }
          />
        ) : (
          <>
            <TransactionList transaksi={tampil} onHapus={hapus} />
            <p className="list-card__footer">Total {tampil.length} catatan</p>
          </>
        )}
      </div>

      <div className="quick-actions">
        <Button to="/catat/uang-masuk">Uang masuk</Button>
        <Button to="/catat/uang-keluar" variant="secondary">
          Uang keluar
        </Button>
        <Button to="/catat/pindah-uang" variant="secondary">
          Pindah uang
        </Button>
      </div>
    </div>
  );
}
