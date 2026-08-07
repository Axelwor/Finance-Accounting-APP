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
 * Generic cash & bank history list. Used by all three sub-modules
 * (Other Receipt, Other Payment, Bank Transfer). Filters and columns
 * are baked in to keep the toolbar compact.
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
    <div className="listtab">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>{title}</span>
          <small>{filtered.length} entries</small>
        </div>
        <div className="listtab__toolbar">
          <button type="button" className="btn btn--ink btn--sm" onClick={openAdd}>
            + Tambah
          </button>
          <button type="button" className="btn btn--secondary btn--sm" onClick={() => void load()}>
            Reload
          </button>
        </div>
      </div>

      <div className="listtab__filters">
        <label className="listtab__filter">
          <span>Search</span>
          <input
            type="search"
            placeholder="number, memo, reference"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") void load();
            }}
          />
        </label>
        <label className="listtab__filter">
          <span>From</span>
          <input type="date" value={fromDate} onChange={(e) => setFromDate(e.target.value)} />
        </label>
        <label className="listtab__filter">
          <span>To</span>
          <input type="date" value={toDate} onChange={(e) => setToDate(e.target.value)} />
        </label>
      </div>

      <div className="listtab__body">
        {loading ? (
          <LoadingState label="Loading entries..." />
        ) : error ? (
          <EmptyState title="Could not load" message={error} />
        ) : filtered.length === 0 ? (
          <EmptyState
            title="No entries in this view"
            message="Rule your first entry with the + Tambah button, or clear the filters."
            action={
              <button type="button" className="btn btn--primary btn--sm" onClick={openAdd}>
                + Tambah
              </button>
            }
          />
        ) : (
          <div className="ledger-table">
            <div className="ledger-table__head">
              <span>Date</span>
              <span>Number / Memo</span>
              <span>Account</span>
              <span className="right">Amount</span>
              <span aria-hidden="true" />
            </div>
            {filtered.map((it) => (
              <CashRow key={it.id} item={it} fixedKind={fixedKind} onOpen={() => openEntry(it)} />
            ))}
          </div>
        )}
      </div>

      <div className="listtab__footer">
        <span>
          Showing <strong>{filtered.length}</strong> of {items.length} &middot; net{" "}
          <strong className={total >= 0 ? "is-positive" : "is-negative"} style={{ color: total >= 0 ? "var(--pos)" : "var(--neg)" }}>
            {total >= 0 ? "+" : "−"}
            {formatIDR(Math.abs(total))}
          </strong>
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
      <span className="ledger-table__date">{item.entry_date}</span>
      <div className="ledger-table__desc">
        <span className={`kind-mark ${kindClass}`}>{kindLabel}</span>
        <div className="ledger-table__desc-text">
          <span className="ledger-table__desc-title">{item.number || `Entry #${item.id}`}</span>
          <span className="ledger-table__desc-meta">{item.description || item.reference || "—"}</span>
        </div>
      </div>
      <span className="ledger-table__cat">{accountLine}</span>
      <span className={`ledger-table__amount ${toneClass}`}>{signedAmount}</span>
      <span className="ledger-table__delete" aria-hidden="true">›</span>
    </div>
  );
}
