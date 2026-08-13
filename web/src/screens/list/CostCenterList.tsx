import { useEffect, useMemo, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { EmptyState, ErrorState, LoadingState } from "../../components/ui";
import { api } from "../../api";
import { formatIDR } from "../../lib/format";
import type { CostCenterListItem } from "../../types";

type TreeCostCenter = CostCenterListItem & { children?: TreeCostCenter[] };

const CENTER_TYPE_LABEL: Record<string, string> = {
  COST: "Cost",
  PROFIT: "Profit",
  INVESTMENT: "Investment",
};

/**
 * Cost Center list (US-094). Renders a tree view of cost centers with hierarchy.
 * Shows code, name, center_type, parent chain, and total_allocated.
 */
export function CostCenterList() {
  const workbench = useWorkbench();
  const [items, setItems] = useState<CostCenterListItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await api.listCostCenters();
      setItems(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load cost centers.");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void load();
  }, []);

  const treeData = useMemo<TreeCostCenter[]>(() => {
    const rootNodes: Map<number, TreeCostCenter> = new Map();
    const allNodes: Map<number, TreeCostCenter> = new Map();

    items.forEach((item) => {
      const node: TreeCostCenter = { ...item, children: [] };
      allNodes.set(item.id, node);
    });

    items.forEach((item) => {
      const node = allNodes.get(item.id)!;
      if (item.parent_id != null && allNodes.has(item.parent_id)) {
        const parent = allNodes.get(item.parent_id)!;
        if (!parent.children) parent.children = [];
        parent.children.push(node);
      } else {
        rootNodes.set(item.id, node);
      }
    });

    return Array.from(rootNodes.values());
  }, [items]);

  return (
    <div className="listtab listtab--accurate">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>Cost Centers</span>
          <small>Hierarki pusat biaya / laba / investasi</small>
        </div>
        <div className="listtab__toolbar">
          <button type="button" className="btn btn--secondary btn--sm" onClick={() => void load()}>
            Reload
          </button>
          <button
            type="button"
            className="btn btn--primary btn--sm"
            onClick={() => workbench.openEntryDraft("cost-center-entry")}
          >
            + New Cost Center
          </button>
        </div>
      </div>

      <div className="listtab__body">
        {loading ? (
          <LoadingState label="Loading cost centers..." />
        ) : error ? (
          <ErrorState message={error} onRetry={() => void load()} />
        ) : treeData.length === 0 ? (
          <EmptyState
            title="No cost centers yet"
            message="Create a cost center to start organizing expenses and allocations."
          />
        ) : (
          <CostCenterTreeView nodes={treeData} allItems={items} level={0} />
        )}
      </div>
      <div className="listtab__footer">
        <span className="listtab__footer-count">{items.length} Cost Center(s)</span>
      </div>
    </div>
  );
}

interface CostCenterTreeViewProps {
  nodes: TreeCostCenter[];
  allItems: CostCenterListItem[];
  level?: number;
}

function CostCenterTreeView({ nodes, allItems, level = 0 }: CostCenterTreeViewProps) {
  return (
    <>
      {nodes.map((node) => (
        <TreeCostCenterRow key={node.id} node={node} allItems={allItems} level={level} />
      ))}
    </>
  );
}

interface TreeCostCenterRowProps {
  node: TreeCostCenter;
  allItems: CostCenterListItem[];
  level: number;
}

function TreeCostCenterRow({ node, allItems, level }: TreeCostCenterRowProps) {
  const workbench = useWorkbench();

  const handleAllocation = () => {
    workbench.openEntryDraft("cost-center-allocation-entry");
  };

  const handlePnL = () => {
    workbench.openList("cost-center-pnl");
  };

  const findParentCode = (parentId: number): string => {
    const found = allItems.find((i) => i.id === parentId);
    return found ? found.code : "-";
  };

  return (
    <>
      <div
        style={{
          display: "flex",
          gap: 8,
          alignItems: "center",
          padding: "8px 12px",
          backgroundColor: node.is_active ? "#fff" : "#f5f5f5",
          borderBottom: "1px solid #eee",
        }}
      >
        <span
          style={{
            width: `${level * 20}px`,
            flexShrink: 0,
          }}
        />
        <span style={{ fontFamily: "var(--font-mono)", fontWeight: 600 }}>{node.code}</span>
        <span style={{ maxWidth: 300, overflow: "hidden", textOverflow: "ellipsis" }}>{node.name}</span>
        <span>{CENTER_TYPE_LABEL[node.center_type] ?? node.center_type}</span>
        <span>{node.parent_id != null ? findParentCode(node.parent_id) : "-"}</span>
        <span style={{ textAlign: "right", fontFamily: "var(--font-mono)" }}>
          {formatIDR(node.total_allocated_cents)}
        </span>
        <div style={{ marginLeft: 8, display: "flex", gap: 4 }}>
          <button
            type="button"
            className="btn btn--secondary btn--xs"
            onClick={handleAllocation}
          >
            Allocate
          </button>
          <button
            type="button"
            className="btn btn--secondary btn--xs"
            onClick={handlePnL}
          >
            P&L
          </button>
        </div>
      </div>
      {node.children && node.children.length > 0 && (
        <CostCenterTreeView nodes={node.children} allItems={allItems} level={level + 1} />
      )}
    </>
  );
}

export default CostCenterList;
