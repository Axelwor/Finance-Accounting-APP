import { useEffect, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { EmptyState, LoadingState } from "../../components/ui";
import { api } from "../../api";
import { formatIDR } from "../../lib/format";
import type { SupplierInvoiceListItem, SupplierPayment } from "../../types";

function StatusBadge({ status }: { status: string }) {
  const cls =
    status === "PAID" || status === "POSTED"
      ? "is-positive"
      : status === "VOID" || status === "REVERSED"
        ? "is-negative"
        : status === "PARTIALLY_PAID"
          ? "is-warning"
          : "is-muted";
  return (
    <span className={`kind-mark ${cls}`} style={{ minWidth: 80, display: "inline-block" }}>
      {status}
    </span>
  );
}

/* ---------------------- Purchase Invoices (Tagihan) ---------------------- */

export function PurchaseInvoiceList() {
  const workbench = useWorkbench();
  const [rows, setRows] = useState<SupplierInvoiceListItem[] | null>(null);

  useEffect(() => {
    api.listSupplierInvoices().then(setRows);
  }, []);

  if (rows === null) return <LoadingState />;
  if (rows.length === 0)
    return <EmptyState title="Purchase Invoices" message="No supplier invoices yet." action={<button className="btn btn--primary" onClick={() => workbench.openEntryDraft("supplier-invoice-entry")}>+ New Invoice</button>} />;

  return (
    <div className="listtab listtab--accurate">
      <div className="listtab__header">
        <div>
          <h2 className="listtab__title">Purchase Invoices</h2>
          <p className="listtab__desc">Supplier invoices / bills (Tagihan)</p>
        </div>
        <button className="btn btn--primary" onClick={() => workbench.openEntryDraft("supplier-invoice-entry")}>
          + New Invoice
        </button>
      </div>
      <div className="listtab__body">
        <div className="ledger-table">
          <div className="ledger-table__row ledger-table__row--header">
            <span>Number</span>
            <span>Supplier</span>
            <span>Date</span>
            <span className="right">Total</span>
            <span className="right">Payable</span>
            <span>Status</span>
          </div>
          {rows.map((r) => (
            <div
              key={r.id}
              className="ledger-table__row ledger-table__row--clickable"
              onClick={() => workbench.openEntryExisting("supplier-invoice-entry", r.id, r.number, r.status)}
            >
              <span style={{ fontFamily: "var(--font-mono)" }}>{r.number}</span>
              <span>{r.supplier_name ?? `Supplier #${r.supplier_id}`}</span>
              <span style={{ fontFamily: "var(--font-mono)" }}>{r.invoice_date}</span>
              <span className="ledger-table__amount right">{formatIDR(r.total_cents)}</span>
              <span className="ledger-table__amount right">{formatIDR(r.payable_cents)}</span>
              <StatusBadge status={r.status} />
            </div>
          ))}
        </div>
      </div>
      <div className="listtab__footer">
        <span>
          Total payable <strong>{formatIDR(rows.reduce((s, r) => s + r.payable_cents, 0))}</strong>
        </span>
        <span className="listtab__footer-count">{rows.length} invoice(s)</span>
      </div>
    </div>
  );
}

/* ---------------------- Purchase Payments (Bayar) ---------------------- */

export function PurchasePaymentList() {
  const workbench = useWorkbench();
  const [rows, setRows] = useState<SupplierInvoiceListItem[] | null>(null);

  useEffect(() => {
    // Show invoices that have been paid (PARTIALLY_PAID or PAID)
    api.listSupplierInvoices("PARTIALLY_PAID").then(async (partial) => {
      const paid = await api.listSupplierInvoices("PAID");
      setRows([...partial, ...paid]);
    });
  }, []);

  if (rows === null) return <LoadingState />;
  if (rows.length === 0)
    return <EmptyState title="Purchase Payments" message="No payments yet. Pay supplier invoices from the Purchase Invoice screen." action={<button className="btn btn--primary" onClick={() => workbench.openEntryDraft("supplier-invoice-entry")}>+ New Payment</button>} />;

  const totalPaid = rows.reduce((s, r) => s + (r.total_cents - r.payable_cents), 0);

  return (
    <div className="listtab listtab--accurate">
      <div className="listtab__header">
        <div>
          <h2 className="listtab__title">Purchase Payments</h2>
          <p className="listtab__desc">Payments made to suppliers (Bayar)</p>
        </div>
        <button className="btn btn--primary" onClick={() => workbench.openEntryDraft("supplier-invoice-entry")}>
          + New Payment
        </button>
      </div>
      <div className="listtab__body">
        <div className="ledger-table">
          <div className="ledger-table__row ledger-table__row--header">
            <span>Invoice #</span>
            <span>Supplier</span>
            <span>Date</span>
            <span className="right">Total</span>
            <span className="right">Paid</span>
            <span>Status</span>
          </div>
          {rows.map((r) => {
            const paidAmount = r.total_cents - r.payable_cents;
            return (
              <div
                key={r.id}
                className="ledger-table__row ledger-table__row--clickable"
                onClick={() => workbench.openEntryExisting("supplier-invoice-entry", r.id, r.number, r.status)}
              >
                <span style={{ fontFamily: "var(--font-mono)" }}>{r.number}</span>
                <span>{r.supplier_name ?? `Supplier #${r.supplier_id}`}</span>
                <span style={{ fontFamily: "var(--font-mono)" }}>{r.invoice_date}</span>
                <span className="ledger-table__amount right">{formatIDR(r.total_cents)}</span>
                <span className="ledger-table__amount right">{formatIDR(paidAmount)}</span>
                <StatusBadge status={r.status} />
              </div>
            );
          })}
        </div>
      </div>
      <div className="listtab__footer">
        <span>
          Total paid <strong>{formatIDR(totalPaid)}</strong>
        </span>
        <span className="listtab__footer-count">{rows.length} payment(s)</span>
      </div>
    </div>
  );
}
