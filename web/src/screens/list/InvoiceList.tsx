import { useEffect, useMemo, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { useTabRefresh } from "../../workbench/useTabRefresh";
import { LoadingState } from "../../components/ui";
import { EmptyState } from "../../components/EmptyState";
import { SortableHeader, type SortState } from "../../components/SortableHeader";
import { StatusBadge } from "../../components/StatusBadge";
import { RowActions, type RowAction } from "../../components/RowActions";
import { api } from "../../api";
import { formatIDR } from "../../lib/format";
import { showToast } from "../../lib/toast";
import type { InvoiceListItem } from "../../types";
import { Button, IconButton } from "../../components/m3";

type SortableColumn = "number" | "date" | "customer" | "due" | "status" | "dp" | "receivable";

export function InvoiceList() {
  const workbench = useWorkbench();
  const [items, setItems] = useState<InvoiceListItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [status, setStatus] = useState<"ALL" | InvoiceListItem["status"]>("ALL");
  const [sort, setSort] = useState<SortState>({ column: "date", direction: "desc" });
  const [voidingId, setVoidingId] = useState<number | null>(null);

  const load = async (filter: "ALL" | InvoiceListItem["status"] = status) => {
    setLoading(true);
    const data = await api.listInvoices(filter === "ALL" ? undefined : filter);
    setItems(data);
    setLoading(false);
  };

  useEffect(() => {
    void load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);
  useTabRefresh(load);

  const onSort = (column: string) =>
    setSort((s) => ({
      column,
      direction: s.column === column && s.direction === "asc" ? "desc" : "asc",
    }));

  const sorted = useMemo(() => {
    const dir = sort.direction === "asc" ? 1 : -1;
    const by = sort.column as SortableColumn;
    const cmp = (a: InvoiceListItem, b: InvoiceListItem) => {
      let av: string | number;
      let bv: string | number;
      switch (by) {
        case "number":
          av = a.number ?? "";
          bv = b.number ?? "";
          break;
        case "date":
          av = a.invoice_date ?? "";
          bv = b.invoice_date ?? "";
          break;
        case "customer":
          av = a.customer_name ?? `#${a.customer_id}`;
          bv = b.customer_name ?? `#${b.customer_id}`;
          break;
        case "due":
          av = a.due_date ?? "";
          bv = b.due_date ?? "";
          break;
        case "status":
          av = a.status;
          bv = b.status;
          break;
        case "dp":
          av = a.dp_applied_cents;
          bv = b.dp_applied_cents;
          break;
        case "receivable":
          av = a.receivable_cents;
          bv = b.receivable_cents;
          break;
        default:
          av = a.invoice_date ?? "";
          bv = b.invoice_date ?? "";
      }
      if (typeof av === "number" && typeof bv === "number") return (av - bv) * dir;
      return String(av).localeCompare(String(bv)) * dir;
    };
    return [...items].sort(cmp);
  }, [items, sort]);

  const totalReceivable = useMemo(
    () => sorted.filter((i) => i.status !== "VOID").reduce((acc, it) => acc + it.receivable_cents, 0),
    [sorted],
  );
  const openEntry = (item: InvoiceListItem) =>
    workbench.openEntryExisting("sales-invoice", item.id, item.number, item.status);

  const handleVoid = async (item: InvoiceListItem) => {
    if (
      !window.confirm(
        `Void invoice ${item.number}? A reversal journal will be posted and the invoice cannot be un-voided.`,
      )
    ) {
      return;
    }
    setVoidingId(item.id);
    try {
      await api.voidInvoice(item.id);
      showToast(`✓ Invoice ${item.number} voided.`);
      await load();
    } catch (err) {
      showToast(err instanceof Error ? err.message : "Failed to void invoice.", "error");
    } finally {
      setVoidingId(null);
    }
  };

  return (
    <div className="listtab listtab--accurate">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>Sales Invoices</span>
          <small>Invoices issued to customers (INV). Posts revenue + DP realization.</small>
        </div>
      </div>

      <div className="listtab__toolbar">
        <div className="listtab__filters">
          <label className="filter-pill">
            <span className="filter-pill__label">Status</span>
            <select
              className="filter-pill__input"
              style={{ appearance: "none", width: "auto", paddingRight: 2, cursor: "pointer" }}
              value={status}
              aria-label="Filter invoices by status"
              onChange={(e) => {
                const next = e.target.value as "ALL" | InvoiceListItem["status"];
                setStatus(next);
                void load(next);
              }}
            >
              <option value="ALL">All</option>
              <option value="DRAFT">Draft</option>
              <option value="ISSUED">Issued</option>
              <option value="PARTIALLY_PAID">Partially Paid</option>
              <option value="PAID">Paid</option>
              <option value="VOID">Void</option>
            </select>
            <span className="filter-pill__caret">▾</span>
          </label>
        </div>
        <div className="listtab__actions">
          <Button
            variant="filled"
            size="sm"
            onClick={() => workbench.openEntryDraft("sales-invoice")}
          >
            + New Invoice
          </Button>
          <IconButton
            size="sm"
            onClick={() => void load()}
            label="Reload"
          >
            <ReloadIcon />
          </IconButton>
          <span className="listtab__count">{sorted.length}</span>
        </div>
      </div>

      <div className="listtab__body listtab__body--scroll">
        {loading ? (
          <LoadingState label="Loading invoices..." />
        ) : sorted.length === 0 ? (
          <EmptyState
            entity="invoice"
            filtered={items.length > 0}
            action={
              <Button variant="filled" onClick={() => workbench.openEntryDraft("sales-invoice")}>
                + New Invoice
              </Button>
            }
          />
        ) : (
          <table className="ledger-table" aria-label="Sales invoices list">
            <thead>
              <tr>
                <SortableHeader column="number" sort={sort} onSort={onSort}>Number</SortableHeader>
                <SortableHeader column="date" sort={sort} onSort={onSort}>Date</SortableHeader>
                <SortableHeader column="customer" sort={sort} onSort={onSort}>Customer</SortableHeader>
                <SortableHeader column="due" sort={sort} onSort={onSort}>Due</SortableHeader>
                <SortableHeader column="status" sort={sort} onSort={onSort}>Status</SortableHeader>
                <SortableHeader column="dp" sort={sort} onSort={onSort} align="right">DP Applied</SortableHeader>
                <SortableHeader column="receivable" sort={sort} onSort={onSort} align="right">Receivable</SortableHeader>
                <th scope="col" className="right">Aksi</th>
              </tr>
            </thead>
            <tbody>
              {sorted.map((it) => (
                <InvoiceRow
                  key={it.id}
                  item={it}
                  onOpen={() => openEntry(it)}
                  onVoid={() => void handleVoid(it)}
                  voiding={voidingId === it.id}
                />
              ))}
            </tbody>
          </table>
        )}
      </div>

      <div className="listtab__footer">
        <span>
          Outstanding receivable <strong>{formatIDR(totalReceivable)}</strong>
        </span>
        <span className="listtab__footer-count">{sorted.length} invoice(s)</span>
      </div>
    </div>
  );
}

function InvoiceRow({
  item,
  onOpen,
  onVoid,
  voiding,
}: {
  item: InvoiceListItem;
  onOpen: () => void;
  onVoid: () => void;
  voiding: boolean;
}) {
  const actions: RowAction[] = [
    { label: "Open", onClick: onOpen },
    { label: "Print", onClick: () => window.print(), disabled: item.status === "VOID" },
    {
      label: voiding ? "Voiding…" : "Void",
      onClick: onVoid,
      destructive: true,
      disabled: item.status === "VOID" || voiding,
    },
  ];
  return (
    <tr role="button" tabIndex={0} onClick={onOpen} onKeyDown={(e) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); onOpen(); } }} style={{ cursor: "pointer" }}>
      <th scope="row">{item.number}</th>
      <td>{item.invoice_date}</td>
      <td>{item.customer_name ?? `#${item.customer_id}`}</td>
      <td>{item.due_date ?? "—"}</td>
      <td>
        <StatusBadge status={item.status} />
      </td>
      <td className="right">{item.dp_applied_cents > 0 ? formatIDR(item.dp_applied_cents) : "—"}</td>
      <td className="right">{formatIDR(item.receivable_cents)}</td>
      <td className="right">
        <span onClick={(e) => e.stopPropagation()}>
          <RowActions actions={actions} label={`Actions for ${item.number}`} />
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
