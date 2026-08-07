import { useMemo } from "react";
import { useWorkbench } from "../../workbench/state";
import { MockList, type MockListColumn } from "./MockList";
import { formatIDR } from "../../lib/format";
import {
  makeInventoryItems,
  makeStockMovements,
  type InventoryItem,
  type StockMovement,
} from "../../lib/mockData";

function ItemStatusBadge({ status, onHand }: { status: InventoryItem["status"]; onHand: number }) {
  const low = onHand > 0 && onHand < 20;
  const tone = status === "INACTIVE" ? "is-muted" : low ? "is-negative" : "is-positive";
  const label = status === "INACTIVE" ? "INACTIVE" : low ? "LOW" : "ACTIVE";
  return (
    <span className={`kind-mark ${tone}`} style={{ minWidth: 64, display: "inline-block" }}>
      {label}
    </span>
  );
}

function MovementStatusBadge({ status }: { status: StockMovement["status"] }) {
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

export function InventoryItemsList() {
  const workbench = useWorkbench();
  const rows = useMemo(() => makeInventoryItems(), []);

  const columns: MockListColumn<InventoryItem>[] = [
    {
      key: "code",
      label: "Code",
      render: (r) => (
        <span style={{ fontFamily: "var(--font-mono)", color: "var(--ink-secondary)" }}>{r.code}</span>
      ),
    },
    {
      key: "name",
      label: "Name",
      primary: true,
      render: (r) => r.name,
      secondary: (r) => `${r.category} · ${r.unit}`,
    },
    {
      key: "onHand",
      label: "On hand",
      align: "right",
      tone: (r) => (r.onHand === 0 ? "is-muted" : r.onHand < 20 ? "is-negative" : ""),
      render: (r) => new Intl.NumberFormat("en-US").format(r.onHand),
    },
    {
      key: "avgCost",
      label: "Avg cost",
      align: "right",
      render: (r) => formatIDR(r.avgCost),
    },
    {
      key: "status",
      label: "Status",
      render: (r) => <ItemStatusBadge status={r.status} onHand={r.onHand} />,
    },
  ];

  return (
    <MockList
      title="Item List"
      description="Inventory items and average cost (mock data — no backend yet)."
      kind="inventory"
      columns={columns}
      rows={rows}
      searchFields={["code", "name", "category", "unit"]}
      getRowKey={(r) => r.id}
      searchPlaceholder="code, name, category..."
      onAdd={() => workbench.openEntryDraft("inventory-item")}
      onOpen={(r) => workbench.openEntryExisting("inventory-item", r.id, r.code, r.status)}
    />
  );
}

export function StockMovementsList() {
  const workbench = useWorkbench();
  const rows = useMemo(() => makeStockMovements(), []);

  const columns: MockListColumn<StockMovement>[] = [
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
      secondary: (r) => r.item,
    },
    {
      key: "type",
      label: "Type",
      render: (r) => r.type,
    },
    {
      key: "qty",
      label: "Qty",
      align: "right",
      render: (r) => new Intl.NumberFormat("en-US").format(r.qty),
    },
    {
      key: "unitCost",
      label: "Unit cost",
      align: "right",
      render: (r) => formatIDR(r.unitCost),
    },
    {
      key: "total",
      label: "Total",
      align: "right",
      tone: (r) => (r.status === "VOID" ? "is-muted" : ""),
      render: (r) => formatIDR(r.total),
    },
    {
      key: "status",
      label: "Status",
      render: (r) => <MovementStatusBadge status={r.status} />,
    },
  ];

  // The stock-movements list is read-only — no entry form yet.
  return (
    <MockList
      title="Stock Movements"
      description="Receipts, issues, transfers, and adjustments (mock data — no backend yet)."
      kind="inventory"
      columns={columns}
      rows={rows}
      searchFields={["number", "item", "type", "date"]}
      getRowKey={(r) => r.id}
      searchPlaceholder="number, item, type..."
    />
  );
}
