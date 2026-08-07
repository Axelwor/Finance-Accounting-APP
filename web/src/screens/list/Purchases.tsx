import { useMemo } from "react";
import { useWorkbench } from "../../workbench/state";
import { MockList, type MockListColumn } from "./MockList";
import { formatIDR } from "../../lib/format";
import {
  makePurchaseInvoices,
  makePurchasePayments,
  type PurchaseInvoice,
  type PurchasePayment,
} from "../../lib/mockData";

function StatusBadge({ status }: { status: PurchaseInvoice["status"] | PurchasePayment["status"] }) {
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

export function PurchaseInvoiceList() {
  const workbench = useWorkbench();
  const rows = useMemo(() => makePurchaseInvoices(), []);

  const columns: MockListColumn<PurchaseInvoice>[] = [
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
      secondary: (r) => `Supplier: ${r.supplier}`,
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
      title="Purchase Invoices"
      description="Bills received from suppliers (mock data — no backend yet)."
      kind="purchases"
      columns={columns}
      rows={rows}
      searchFields={["number", "supplier", "date", "dueDate"]}
      getRowKey={(r) => r.id}
      searchPlaceholder="number, supplier, date..."
      onAdd={() => workbench.openEntryDraft("purchase-invoice")}
      onOpen={(r) => workbench.openEntryExisting("purchase-invoice", r.id, r.number, r.status)}
    />
  );
}

export function PurchasePaymentList() {
  const workbench = useWorkbench();
  const rows = useMemo(() => makePurchasePayments(), []);

  const columns: MockListColumn<PurchasePayment>[] = [
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
      secondary: (r) => `Supplier: ${r.supplier}`,
    },
    {
      key: "payMethod",
      label: "Method",
      render: (r) => r.payMethod,
    },
    {
      key: "amount",
      label: "Amount",
      align: "right",
      tone: (r) => (r.status === "VOID" ? "is-muted" : "is-negative"),
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
      title="Purchase Payments"
      description="Payments made to suppliers (mock data — no backend yet)."
      kind="purchases"
      columns={columns}
      rows={rows}
      searchFields={["number", "supplier", "payMethod", "date"]}
      getRowKey={(r) => r.id}
      searchPlaceholder="number, supplier, method..."
      onAdd={() => workbench.openEntryDraft("purchase-payment")}
      onOpen={(r) => workbench.openEntryExisting("purchase-payment", r.id, r.number, r.status)}
    />
  );
}
