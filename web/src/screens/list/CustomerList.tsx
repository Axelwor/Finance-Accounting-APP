import { useEffect, useMemo, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { FormError, LoadingState } from "../../components/ui";
import { EmptyState } from "../../components/EmptyState";
import { SortableHeader, type SortState } from "../../components/SortableHeader";
import { StatusBadge } from "../../components/StatusBadge";
import { RowActions, type RowAction } from "../../components/RowActions";
import { api } from "../../api";
import type { Customer } from "../../types";

type SortableColumn = "code" | "name" | "price_level" | "group" | "phone" | "status";

export function CustomerList() {
  const workbench = useWorkbench();
  const [customers, setCustomers] = useState<Customer[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [search, setSearch] = useState("");
  const [sort, setSort] = useState<SortState>({ column: "code", direction: "asc" });

  useEffect(() => {
    api.listCustomers().then(setCustomers).catch(() => setError("Failed to load customers")).finally(() => setLoading(false));
  }, []);

  const onSort = (column: string) =>
    setSort((s) => ({
      column,
      direction: s.column === column && s.direction === "asc" ? "desc" : "asc",
    }));

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase();
    if (!q) return customers;
    return customers.filter((c) =>
      [c.code, c.name, c.customer_group, c.phone, c.price_level].some((field) => field?.toLowerCase().includes(q)),
    );
  }, [customers, search]);

  const sorted = useMemo(() => {
    const dir = sort.direction === "asc" ? 1 : -1;
    const by = sort.column as SortableColumn;
    const cmp = (a: Customer, b: Customer) => {
      let av: string | boolean;
      let bv: string | boolean;
      switch (by) {
        case "code":
          av = a.code ?? "";
          bv = b.code ?? "";
          break;
        case "name":
          av = a.name ?? "";
          bv = b.name ?? "";
          break;
        case "price_level":
          av = a.price_level ?? "";
          bv = b.price_level ?? "";
          break;
        case "group":
          av = a.customer_group ?? "";
          bv = b.customer_group ?? "";
          break;
        case "phone":
          av = a.phone ?? "";
          bv = b.phone ?? "";
          break;
        case "status":
          av = a.is_active;
          bv = b.is_active;
          break;
        default:
          av = a.code ?? "";
          bv = b.code ?? "";
      }
      if (typeof av === "boolean" && typeof bv === "boolean") {
        return (Number(av) - Number(bv)) * dir;
      }
      return String(av).localeCompare(String(bv)) * dir;
    };
    return [...filtered].sort(cmp);
  }, [filtered, sort]);

  if (loading) return <LoadingState label="Loading customers..." />;
  if (error) return <FormError message={error} />;

  const openCustomer = (c: Customer) =>
    workbench.openEntryExisting("customer-entry", c.id, `${c.code} · ${c.name}`, c.is_active ? "ACTIVE" : "INACTIVE");

  return (
    <div className="listtab listtab--accurate">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>Customers</span>
          <small>Customer master data for sales orders and invoices.</small>
        </div>
      </div>
      <div className="listtab__toolbar">
        <div className="listtab__filters" />
        <div className="listtab__actions">
          <input
            type="search"
            className="input listtab__search"
            placeholder="Search customers..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            aria-label="Search customers"
          />
          <button type="button" className="btn btn--primary btn--sm" onClick={() => workbench.openEntryDraft("customer-entry")}>
            + New Customer
          </button>
          <span className="listtab__count">{sorted.length}</span>
        </div>
      </div>
      <div className="listtab__body listtab__body--scroll">
        {sorted.length === 0 ? (
          <EmptyState
            entity="customer"
            filtered={customers.length > 0}
            action={
              <button type="button" className="btn btn--primary" onClick={() => workbench.openEntryDraft("customer-entry")}>
                + New Customer
              </button>
            }
          />
        ) : (
          <table className="ledger-table" aria-label="Customers list">
            <thead>
              <tr>
                <SortableHeader column="code" sort={sort} onSort={onSort}>Code</SortableHeader>
                <SortableHeader column="name" sort={sort} onSort={onSort}>Name</SortableHeader>
                <SortableHeader column="price_level" sort={sort} onSort={onSort}>Price Level</SortableHeader>
                <SortableHeader column="group" sort={sort} onSort={onSort}>Group</SortableHeader>
                <SortableHeader column="phone" sort={sort} onSort={onSort}>Phone</SortableHeader>
                <SortableHeader column="status" sort={sort} onSort={onSort}>Status</SortableHeader>
                <th scope="col" className="right">Aksi</th>
              </tr>
            </thead>
            <tbody>
              {sorted.map((c) => (
                <CustomerRow key={c.id} c={c} onOpen={() => openCustomer(c)} />
              ))}
            </tbody>
          </table>
        )}
      </div>
      <div className="listtab__footer">
        <span className="listtab__footer-count">{sorted.length} customer(s)</span>
      </div>
    </div>
  );
}

function CustomerRow({ c, onOpen }: { c: Customer; onOpen: () => void }) {
  const actions: RowAction[] = [
    { label: "Open", onClick: onOpen },
    { label: c.is_active ? "Deactivate" : "Activate", onClick: onOpen },
    { label: "Delete", onClick: onOpen, destructive: true },
  ];
  return (
    <tr role="button" tabIndex={0} onClick={onOpen} onKeyDown={(e) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); onOpen(); } }} style={{ cursor: "pointer" }}>
      <th scope="row">{c.code}</th>
      <td>{c.name}</td>
      <td>{c.price_level || "—"}</td>
      <td>{c.customer_group || "—"}</td>
      <td>{c.phone || "—"}</td>
      <td className="right">
        <StatusBadge status={c.is_active ? "ACTIVE" : "INACTIVE"} />
        <span onClick={(e) => e.stopPropagation()}>
          <RowActions actions={actions} label={`Actions for ${c.code}`} />
        </span>
      </td>
    </tr>
  );
}
