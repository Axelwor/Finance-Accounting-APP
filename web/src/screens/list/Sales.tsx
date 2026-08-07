import { useMemo } from "react";
import { useWorkbench } from "../../workbench/state";
import { MockList, type MockListColumn } from "./MockList";
import { formatIDR } from "../../lib/format";
import { makeSalesInvoices, makeSalesReceipts, type SalesInvoice, type SalesReceipt } from "../../lib/mockData";

/* ---------------------- Sales Invoice ---------------------- */

function StatusBadge({ status }: { status: SalesInvoice["status"] }) {
  const cls =
    status === "POSTED"
      ? "is-positive"
      : status === "VOID"
        ? "is-negative"
        : "is-muted";
  return (
    <span className={`kind-mark ${cls}`} style={{ minWidth: 64, display: "inline-block" }}>
      {status}
    </span>
  );
}

export function SalesInvoiceList() {
  const workbench = useWorkbench();
  const rows = useMemo(() => makeSalesInvoices(), []);

  const columns: MockListColumn<SalesInvoice>[] = [
    {
      key: "date",
      label: "Date",
      render: (r) => <span style={{ fontFamily: "var(--font-mono)" }}>{r.date}</span>,
    },
    {
      key: "number",
      label: "Number",
      primary: true,
      render: (r) => r.number,
      secondary: (r) => `Customer: ${r.customer}`,
    },
    {
      key: "dueDate",
      label: "Due",
      render: (r) => <span style={{ fontFamily: "var(--font-mono)", color: "var(--ink-secondary)" }}>{r.dueDate}</span>,
    },
    {
      key: "amount",
      label: "Amount",
      align: "right",
      tone: (r) => (r.status === "VOID" ? "is-muted" : ""),
      render: (r) => formatIDR(r.amount),
    },
    {
      key: "status",
      label: "Status",
      render: (r) => <StatusBadge status={r.status} />,
    },
  ];

  return (
    <MockList
      title="Sales Invoices"
      description="Invoices issued to customers (mock data — no backend yet)."
      kind="sales"
      columns={columns}
      rows={rows}
      searchFields={["number", "customer", "date", "dueDate"]}
      getRowKey={(r) => r.id}
      searchPlaceholder="number, customer, date..."
      onAdd={() => workbench.openEntryDraft("sales-invoice")}
      onOpen={(r) => workbench.openEntryExisting("sales-invoice", r.id, r.number, r.status)}
    />
  );
}

/* ---------------------- Sales Receipt ---------------------- */

export function SalesReceiptList() {
  const workbench = useWorkbench();
  const rows = useMemo(() => makeSalesReceipts(), []);

  const columns: MockListColumn<SalesReceipt>[] = [
    {
      key: "date",
      label: "Date",
      render: (r) => <span style={{ fontFamily: "var(--font-mono)" }}>{r.date}</span>,
    },
    {
      key: "number",
      label: "Number",
      primary: true,
      render: (r) => r.number,
      secondary: (r) => `Customer: ${r.customer} · ${r.payer}`,
    },
    {
      key: "payer",
      label: "Payer",
      render: (r) => r.payer,
    },
    {
      key: "amount",
      label: "Amount",
      align: "right",
      tone: (r) => (r.status === "VOID" ? "is-muted" : "is-positive"),
      render: (r) => formatIDR(r.amount),
    },
    {
      key: "status",
      label: "Status",
      render: (r) => <StatusBadge status={r.status} />,
    },
  ];

  return (
    <MockList
      title="Sales Receipts"
      description="Receipts collected from customers (mock data — no backend yet)."
      kind="sales"
      columns={columns}
      rows={rows}
      searchFields={["number", "customer", "payer", "date"]}
      getRowKey={(r) => r.id}
      searchPlaceholder="number, customer, payer..."
      onAdd={() => workbench.openEntryDraft("sales-receipt")}
      onOpen={(r) => workbench.openEntryExisting("sales-receipt", r.id, r.number, r.status)}
    />
  );
}
