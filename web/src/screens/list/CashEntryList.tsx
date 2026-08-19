import { useEffect, useMemo, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { LoadingState } from "../../components/ui";
import { EmptyState } from "../../components/EmptyState";
import { SortableHeader, type SortState } from "../../components/SortableHeader";
import { StatusBadge } from "../../components/StatusBadge";
import { RowActions, type RowAction } from "../../components/RowActions";
import { AccountPicker } from "../../components/AccountPicker";
import { api } from "../../api";
import { formatIDR } from "../../lib/format";
import type { AccountItem, CashEntryListItem, EntrySubKind, ListSubKind } from "../../types";
import { Button, IconButton } from "../../components/m3";

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
  const [accountFilter, setAccountFilter] = useState("");
  const [accounts, setAccounts] = useState<AccountItem[]>([]);
  const [showFilters, setShowFilters] = useState(false);
  const [sort, setSort] = useState<SortState>({ column: "date", direction: "desc" });

  // Load cash/bank accounts once for the account filter.
  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const list = await api.listAccounts();
        if (!cancelled) {
          setAccounts(list.filter((a) => a.account_type === "CASH" || a.account_type === "BANK"));
        }
      } catch {
        /* filter stays empty — pill shows "Semua" */
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  const onSort = (column: string) =>
    setSort((s) => ({
      column,
      direction: s.column === column && s.direction === "asc" ? "desc" : "asc",
    }));

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await api.listCashEntries({
        kind: fixedKind,
        from: fromDate || undefined,
        to: toDate || undefined,
        account_id: accountFilter ? Number(accountFilter) : undefined,
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
  }, [fixedKind, fromDate, toDate, accountFilter]);

  const accountFilterActive = fromDate !== "" || toDate !== "" || accountFilter !== "";

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase();
    if (!q) return items;
    return items.filter((it) =>
      [it.number, it.description, it.reference].some((field) => field?.toLowerCase().includes(q)),
    );
  }, [items, search]);

  const sorted = useMemo(() => {
    const dir = sort.direction === "asc" ? 1 : -1;
    const by = sort.column;
    const cmp = (a: CashEntryListItem, b: CashEntryListItem) => {
      let av: string | number;
      let bv: string | number;
      switch (by) {
        case "number":
          av = a.number ?? "";
          bv = b.number ?? "";
          break;
        case "date":
          av = a.entry_date ?? "";
          bv = b.entry_date ?? "";
          break;
        case "account":
          av = a.cash_account_code ?? a.from_account_code ?? "";
          bv = b.cash_account_code ?? b.from_account_code ?? "";
          break;
        case "reference":
          av = a.reference ?? "";
          bv = b.reference ?? "";
          break;
        case "description":
          av = a.description ?? "";
          bv = b.description ?? "";
          break;
        case "amount":
          av = a.amount_cents;
          bv = b.amount_cents;
          break;
        default:
          av = a.entry_date ?? "";
          bv = b.entry_date ?? "";
      }
      if (typeof av === "number" && typeof bv === "number") return (av - bv) * dir;
      return String(av).localeCompare(String(bv)) * dir;
    };
    return [...filtered].sort(cmp);
  }, [filtered, sort]);

  const total = useMemo(() => {
    return sorted.reduce((acc, it) => {
      const signed = it.kind === "money-in" ? it.amount_cents : it.kind === "money-out" ? -it.amount_cents : 0;
      return acc + signed;
    }, 0);
  }, [sorted]);

  const openEntry = (item: CashEntryListItem) => {
    workbench.openEntryExisting(entrySubKind, item.id, item.number || `Entry #${item.id}`, item.status);
  };

  const openAdd = () => workbench.openEntryDraft(entrySubKind);

  /**
   * Duplikat — open a new draft of the same kind. The form does not yet
   * accept a prefill payload for cash entries (PrefillRef has no cash kind),
   * so duplication opens a fresh draft; the user re-enters the amount. This
   * is still faster than navigating and matches the plan's "quick copy"
   * intent until PrefillKind grows a cash-entry variant.
   */
  const duplicateEntry = (item: CashEntryListItem) => {
    workbench.openEntryDraft(entrySubKind);
    void item;
  };

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
          {/* Date-range filter pill — click to expand the range inputs. */}
          <button
            type="button"
            className="filter-pill"
            aria-expanded={showFilters}
            onClick={() => setShowFilters((v) => !v)}
          >
            <span className="filter-pill__label">Tanggal</span>
            <span className="filter-pill__value">
              {fromDate || toDate ? `${fromDate || "…"} – ${toDate || "…"}` : "Semua"}
            </span>
            <span className="filter-pill__caret">▾</span>
          </button>
          {/* Account filter — combobox limited to CASH/BANK accounts. */}
          <div className="filter-pill filter-pill--wide">
            <span className="filter-pill__label">Kas/Bank</span>
            <AccountPicker
              accounts={accounts}
              value={accountFilter || null}
              onChange={(v) => setAccountFilter(v ?? "")}
              placeholder="Semua"
            />
          </div>
          {accountFilterActive && (
            <Button
              variant="outlined"
              size="sm"
              onClick={() => {
                setFromDate("");
                setToDate("");
                setAccountFilter("");
              }}
            >
              Reset
            </Button>
          )}
        </div>
        {showFilters && (
          <div className="listtab__filter-row">
            <label className="field">
              <span className="field__label">Dari</span>
              <input
                type="date"
                className="input"
                value={fromDate}
                onChange={(e) => setFromDate(e.target.value)}
              />
            </label>
            <label className="field">
              <span className="field__label">Sampai</span>
              <input
                type="date"
                className="input"
                value={toDate}
                onChange={(e) => setToDate(e.target.value)}
              />
            </label>
          </div>
        )}
        <div className="listtab__actions">
          <Button
            variant="filled"
            size="sm"
            onClick={openAdd}
          >
            + Tambah
          </Button>
          <IconButton
            size="sm"
            onClick={() => void load()}
            label="Reload"
          >
            <ReloadIcon />
          </IconButton>
          <IconButton
            size="sm"
            label="Download"
            disabled
          >
            <DownloadIcon />
          </IconButton>
          <IconButton
            size="sm"
            label="Print"
            disabled
          >
            <PrintIcon />
          </IconButton>
          <IconButton
            size="sm"
            label="Settings"
            disabled
          >
            <SettingsIcon />
          </IconButton>
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
          <span className="listtab__count">{sorted.length}</span>
        </div>
      </div>

      <div className="listtab__body listtab__body--scroll">
        {loading ? (
          <LoadingState label="Loading entries..." />
        ) : error ? (
          <EmptyState title="Could not load" message={error} />
        ) : sorted.length === 0 ? (
          <EmptyState
            entity="cash entry"
            filtered={items.length > 0}
            action={
              <Button variant="filled" onClick={openAdd}>
                + Tambah
              </Button>
            }
          />
        ) : (
          <table className="ledger-table" aria-label="Cash entries list">
            <thead>
              <tr>
                <SortableHeader column="number" sort={sort} onSort={onSort}>Nomor #</SortableHeader>
                <SortableHeader column="date" sort={sort} onSort={onSort}>Tanggal</SortableHeader>
                <SortableHeader column="account" sort={sort} onSort={onSort}>Kas/Bank</SortableHeader>
                <SortableHeader column="reference" sort={sort} onSort={onSort}>No Cek #</SortableHeader>
                <SortableHeader column="description" sort={sort} onSort={onSort}>Keterangan</SortableHeader>
                <SortableHeader column="amount" sort={sort} onSort={onSort} align="right">Nilai</SortableHeader>
                <th scope="col" className="right">Aksi</th>
              </tr>
            </thead>
            <tbody>
              {sorted.map((it) => (
                <CashRow
                  key={it.id}
                  item={it}
                  fixedKind={fixedKind}
                  onOpen={() => openEntry(it)}
                  onVoid={() => alert(`Void ${it.number}`)}
                  onPrint={() => window.print()}
                  onDuplicate={() => duplicateEntry(it)}
                />
              ))}
            </tbody>
          </table>
        )}
      </div>

      <div className="listtab__footer">
        <span>
          Net{" "}
          <strong className={total >= 0 ? "is-positive" : "is-negative"} style={{ color: total >= 0 ? "var(--md-sys-color-success)" : "var(--md-sys-color-error)" }}>
            {total >= 0 ? "+" : "−"}
            {formatIDR(Math.abs(total))}
          </strong>
        </span>
        <span className="listtab__footer-count">
          {sorted.length} of {items.length}
        </span>
      </div>
    </div>
  );
}

function CashRow({
  item,
  fixedKind,
  onOpen,
  onVoid,
  onPrint,
  onDuplicate,
}: {
  item: CashEntryListItem;
  fixedKind: Props["fixedKind"];
  onOpen: () => void;
  onVoid: () => void;
  onPrint: () => void;
  onDuplicate: () => void;
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

  const actions: RowAction[] = [
    { label: "Open", onClick: onOpen },
    { label: "Duplikat", onClick: onDuplicate },
    { label: "Print", onClick: onPrint, disabled: item.status === "VOID" },
    { label: "Void", onClick: onVoid, destructive: true, disabled: item.status === "VOID" },
  ];

  return (
    <tr role="button" tabIndex={0} onClick={onOpen} onKeyDown={(e) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); onOpen(); } }} style={{ cursor: "pointer" }}>
      <th scope="row">{item.number || `Entry #${item.id}`}</th>
      <td>{item.entry_date}</td>
      <td>{accountLine}</td>
      <td>{item.reference || "—"}</td>
      <td>
        <span className={`kind-mark ${kindClass}`}>{kindLabel}</span>
        <span>{item.description || "—"}</span>
      </td>
      <td className={`${toneClass} right`}>{signedAmount}</td>
      <td className="right">
        <StatusBadge status={item.status} />
        <span onClick={(e) => e.stopPropagation()}>
          <RowActions actions={actions} label={`Actions for ${item.number ?? item.id}`} />
        </span>
      </td>
    </tr>
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
