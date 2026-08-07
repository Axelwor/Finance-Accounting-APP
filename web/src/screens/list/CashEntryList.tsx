import { useEffect, useMemo, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { EmptyState, LoadingState } from "../../components/ui";
import { api } from "../../api";
import { formatIDR } from "../../lib/format";
import type { CashEntryListItem, EntrySubKind, ListSubKind } from "../../types";

interface Props {
  listKind: ListSubKind;
  title: string;
  description: string;
  entrySubKind: EntrySubKind;
  /** Optional pre-applied filter; absent = show all kinds. */
  fixedKind?: "money-in" | "money-out" | "transfer";
}

const KIND_LABEL: Record<NonNullable<Props["fixedKind"]>, string> = {
  "money-in": "RECEIPT",
  "money-out": "PAYMENT",
  transfer: "TRANSFER",
};

const KIND_TONE: Record<NonNullable<Props["fixedKind"]>, string> = {
  "money-in": "kind-mark--money-in",
  "money-out": "kind-mark--money-out",
  transfer: "kind-mark--transfer",
};

/**
 * Cash & bank history list (Accurate-style layout).
 *
 *   ┌──────────────────────────────────────────────────────────┐
 *   │ Tanggal: 07/08/2026 - 07/08/2026 ▾  Kas/Bank: Semua ▾ ▾ │
 *   │                                                        │
 *   │ [+ Tambah] [↻] [⬇] [🖨] [⚙]            [search]    [0] │
 *   │ ┌──────────────────────────────────────────────────┐   │
 *   │ │ Nomor # | Tanggal | Kas/Bank | No Cek # | ...     │   │
 *   │ │ Belum ada data                                     │   │
 *   │ └──────────────────────────────────────────────────┘   │
 *   └──────────────────────────────────────────────────────────┘
 */
export function CashEntryList({ listKind, title, description, entrySubKind, fixedKind }: Props) {
  const workbench = useWorkbench();
  const [items, setItems] = useState<CashEntryListItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [search, setSearch] = useState("");
  const [fromDate, setFromDate] = useState("");
  const [toDate, setToDate] = useState("");

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await api.listCashEntries({
        kind: fixedKind,
        from: fromDate || undefined,
        to: toDate || undefined,
        q: search.trim() || undefined,
        limit: 200,
      });
      setItems(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load the ledger.");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [fixedKind, fromDate, toDate]);

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase();
    if (!q) return items;
    return items.filter((it) =>
      [it.number, it.description, it.reference].some((field) => field?.toLowerCase().includes(q)),
    );
  }, [items, search]);

  const total = useMemo(() => {
    return filtered.reduce((acc, it) => {
      const signed = it.kind === "money-in" ? it.amount_cents : it.kind === "money-out" ? -it.amount_cents : 0;
      return acc + signed;
    }, 0);
  }, [filtered]);

  const openEntry = (item: CashEntryListItem) => {
    workbench.openEntryExisting(entrySubKind, item.id, item.number || `Entry #${item.id}`, item.status);
  };

  const openAdd = () => workbench.openEntryDraft(entrySubKind);

  return (
    <div className="listtab listtab--accurate">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>{title}</span>
          <small>{description}</small>
        </div>
      </div>

      <div className="listtab__toolbar">
        <div className="listtab__filters">
          <div className="filter-pill">
            <span className="filter-pill__label">Tanggal</span>
            <span className="filter-pill__value">Semua</span>
            <span className="filter-pill__caret">▾</span>
          </div>
          <div className="filter-pill">
            <span className="filter-pill__label">Kas/Bank</span>
            <span className="filter-pill__value">Semua</span>
            <span className="filter-pill__caret">▾</span>
          </div>
          <button type="button" className="filter-pill__toggle" aria-label="More filters">
            <span aria-hidden="true">▾</span>
          </button>
        </div>
        <div className="listtab__actions">
          <button type="button" className="btn btn--primary btn--sm" onClick={openAdd}>
            + Tambah
          </button>
          <button type="button" className="btn btn--icon btn--sm" onClick={() => void load()} aria-label="Reload">
            <ReloadIcon />
          </button>
          <button type="button" className="btn btn--icon btn--sm" aria-label="Download" disabled>
            <DownloadIcon />
          </button>
          <button type="button" className="btn btn--icon btn--sm" aria-label="Print" disabled>
            <PrintIcon />
          </button>
          <button type="button" className="btn btn--icon btn--sm" aria-label="Settings" disabled>
            <SettingsIcon />
          </button>
          <input
            type="search"
            className="input listtab__search"
            placeholder="Ketik dan [Enter]"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") void load();
            }}
          />
          <span className="listtab__count">{filtered.length}</span>
        </div>
      </div>

      <div className="listtab__body">
        {loading ? (
          <LoadingState label="Loading entries..." />
        ) : error ? (
          <EmptyState title="Could not load" message={error} />
        ) : filtered.length === 0 ? (
          <div className="ledger-table">
            <div className="ledger-table__head">
              <span>Nomor #</span>
              <span>Tanggal</span>
              <span>Kas/Bank</span>
              <span>No Cek #</span>
              <span>Keterangan</span>
              <span className="right">Nilai</span>
            </div>
            <div className="ledger-table__empty">Belum ada data</div>
          </div>
        ) : (
          <div className="ledger-table">
            <div className="ledger-table__head">
              <span>Nomor #</span>
              <span>Tanggal</span>
              <span>Kas/Bank</span>
              <span>No Cek #</span>
              <span>Keterangan</span>
              <span className="right">Nilai</span>
            </div>
            {filtered.map((it) => (
              <CashRow key={it.id} item={it} fixedKind={fixedKind} onOpen={() => openEntry(it)} />
            ))}
          </div>
        )}
      </div>

      <div className="listtab__footer">
        <span>
          Net{" "}
          <strong className={total >= 0 ? "is-positive" : "is-negative"} style={{ color: total >= 0 ? "var(--pos)" : "var(--neg)" }}>
            {total >= 0 ? "+" : "−"}
            {formatIDR(Math.abs(total))}
          </strong>
        </span>
        <span className="listtab__footer-count">
          {filtered.length} of {items.length}
        </span>
      </div>
    </div>
  );
}

function CashRow({
  item,
  fixedKind,
  onOpen,
}: {
  item: CashEntryListItem;
  fixedKind: Props["fixedKind"];
  onOpen: () => void;
}) {
  const amount = formatIDR(item.amount_cents);
  const signedAmount =
    item.kind === "money-in"
      ? `+ ${amount}`
      : item.kind === "money-out"
        ? `− ${amount}`
        : amount;
  const toneClass =
    item.kind === "money-in"
      ? "is-positive"
      : item.kind === "money-out"
        ? "is-negative"
        : "is-muted";

  const kindLabel = fixedKind ? KIND_LABEL[fixedKind] : item.kind.toUpperCase();
  const kindClass = fixedKind ? KIND_TONE[fixedKind] : KIND_TONE[item.kind];

  const accountLine =
    item.kind === "transfer"
      ? `${item.from_account_code || "—"} → ${item.to_account_code || "—"}`
      : `${item.cash_account_code || "—"} · ${item.counter_account_name || "—"}`;

  return (
    <div
      className="ledger-table__row"
      role="button"
      tabIndex={0}
      onClick={onOpen}
      onKeyDown={(e) => {
        if (e.key === "Enter" || e.key === " ") {
          e.preventDefault();
          onOpen();
        }
      }}
      style={{ cursor: "pointer" }}
    >
      <span className="ledger-table__no">{item.number || `Entry #${item.id}`}</span>
      <span className="ledger-table__date">{item.entry_date}</span>
      <span className="ledger-table__cat">{accountLine}</span>
      <span className="ledger-table__memo">{item.reference || "—"}</span>
      <span className="ledger-table__desc">
        <span className={`kind-mark ${kindClass}`}>{kindLabel}</span>
        <span>{item.description || "—"}</span>
      </span>
      <span className={`ledger-table__amount ${toneClass}`}>{signedAmount}</span>
    </div>
  );
}

function ReloadIcon() {
  return (
    <svg viewBox="0 0 24 24" width="14" height="14" aria-hidden="true">
      <path d="M4 12a8 8 0 0 1 14-5l2-2v6h-6l2-2a6 6 0 0 0-10 3M20 12a8 8 0 0 1-14 5l-2 2v-6h6l-2 2a6 6 0 0 0 10-3" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" fill="none" />
    </svg>
  );
}
function DownloadIcon() {
  return (
    <svg viewBox="0 0 24 24" width="14" height="14" aria-hidden="true">
      <path d="M12 4v12m-5-5l5 5 5-5M4 20h16" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" fill="none" />
    </svg>
  );
}
function PrintIcon() {
  return (
    <svg viewBox="0 0 24 24" width="14" height="14" aria-hidden="true">
      <path d="M6 9V4h12v5M6 18H4a1 1 0 0 1-1-1v-6a1 1 0 0 1 1-1h16a1 1 0 0 1 1 1v6a1 1 0 0 1-1 1h-2M6 14h12v6H6z" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" fill="none" />
    </svg>
  );
}
function SettingsIcon() {
  return (
    <svg viewBox="0 0 24 24" width="14" height="14" aria-hidden="true">
      <circle cx="12" cy="12" r="3" stroke="currentColor" strokeWidth="1.5" fill="none" />
      <path d="M19 12a7 7 0 0 0-.1-1.2l2-1.5-2-3.5-2.4 1a7 7 0 0 0-2-1.2l-.4-2.6h-4l-.4 2.6a7 7 0 0 0-2 1.2l-2.4-1-2 3.5 2 1.5a7 7 0 0 0 0 2.4l-2 1.5 2 3.5 2.4-1a7 7 0 0 0 2 1.2l.4 2.6h4l.4-2.6a7 7 0 0 0 2-1.2l2.4 1 2-3.5-2-1.5c.1-.4.1-.8.1-1.2z" stroke="currentColor" strokeWidth="1.5" fill="none" />
    </svg>
  );
}
