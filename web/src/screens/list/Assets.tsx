import { useMemo } from "react";
import { useWorkbench } from "../../workbench/state";
import { MockList, type MockListColumn } from "./MockList";
import { formatIDR } from "../../lib/format";
import { makeAssetRegister, type AssetRegisterRow } from "../../lib/mockData";

function AssetStatusBadge({ status }: { status: AssetRegisterRow["status"] }) {
  const tone =
    status === "ACTIVE"
      ? "is-positive"
      : status === "DISPOSED"
        ? "is-negative"
        : "is-muted";
  const label = status === "FULLY_DEPRECIATED" ? "DEPRECIATED" : status;
  return (
    <span className={`kind-mark ${tone}`} style={{ minWidth: 100, display: "inline-block" }}>
      {label}
    </span>
  );
}

export function AssetRegisterList() {
  const workbench = useWorkbench();
  const rows = useMemo(() => makeAssetRegister(), []);

  const columns: MockListColumn<AssetRegisterRow>[] = [
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
      secondary: (r) => r.category,
    },
    {
      key: "acquiredDate",
      label: "Acquired",
      render: (r) => <span style={{ fontFamily: "var(--font-mono)" }}>{r.acquiredDate}</span>,
    },
    {
      key: "cost",
      label: "Cost",
      align: "right",
      render: (r) => formatIDR(r.cost),
    },
    {
      key: "accumDep",
      label: "Accum. dep.",
      align: "right",
      render: (r) => formatIDR(r.accumDep),
    },
    {
      key: "nbv",
      label: "NBV",
      align: "right",
      tone: (r) => (r.status === "DISPOSED" ? "is-muted" : r.nbv > 0 ? "" : "is-muted"),
      render: (r) => formatIDR(r.nbv),
    },
    {
      key: "status",
      label: "Status",
      render: (r) => <AssetStatusBadge status={r.status} />,
    },
  ];

  return (
    <MockList
      title="Asset Register"
      description="Fixed asset cost, accumulated depreciation, and net book value (mock data — no backend yet)."
      kind="fixed-assets"
      columns={columns}
      rows={rows}
      searchFields={["code", "name", "category", "acquiredDate"]}
      getRowKey={(r) => r.id}
      searchPlaceholder="code, name, category..."
      onAdd={() => workbench.openEntryDraft("asset-register")}
      onOpen={(r) => workbench.openEntryExisting("asset-register", r.id, r.code, r.status)}
    />
  );
}
