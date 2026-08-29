import { useEffect, useState } from "react";
import { api } from "../../api";
import type { ItemNameMaster } from "../../types";
import { EmptyState, LoadingState, FormError } from "../../components/ui";
import { Button } from "../../components/m3";
import { showToast } from "../../lib/toast";

type Kind = "category" | "brand";

const CONFIG: Record<Kind, { title: string; subtitle: string; emptyTitle: string; emptyMessage: string }> = {
  category: {
    title: "Item Categories",
    subtitle: "Master categories used by the item form and inventory reports.",
    emptyTitle: "No categories yet",
    emptyMessage: "Add categories to group your items.",
  },
  brand: {
    title: "Item Brands",
    subtitle: "Master brands used by the item form.",
    emptyTitle: "No brands yet",
    emptyMessage: "Add brands to tag your items.",
  },
};

/** Shared list screen for the name-only item masters (SET-001). */
export function ItemNameMasterList({ kind }: { kind: Kind }) {
  const cfg = CONFIG[kind];
  const [rows, setRows] = useState<ItemNameMaster[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [name, setName] = useState("");
  const [editingId, setEditingId] = useState<number | null>(null);
  const [saving, setSaving] = useState(false);

  const load = () => {
    const promise = kind === "category" ? api.listItemCategories() : api.listItemBrands();
    promise
      .then(setRows)
      .catch(() => setError("Failed to load data"))
      .finally(() => setLoading(false));
  };
  useEffect(load, [kind]);

  if (loading) return <LoadingState label={`Loading ${cfg.title.toLowerCase()}...`} />;
  if (error) return <FormError message={error} />;

  const submit = async () => {
    if (!name.trim()) return;
    setSaving(true);
    try {
      if (editingId) {
        const updated = kind === "category"
          ? await api.updateItemCategory(editingId, name.trim())
          : await api.updateItemBrand(editingId, name.trim());
        setRows((prev) => prev.map((r) => (r.id === updated.id ? updated : r)));
        showToast("Saved");
      } else {
        const created = kind === "category"
          ? await api.createItemCategory(name.trim())
          : await api.createItemBrand(name.trim());
        setRows((prev) => [...prev, created]);
        showToast("Created");
      }
      setName("");
      setEditingId(null);
    } catch (err) {
      showToast(err instanceof Error ? err.message : "Failed to save", "error");
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="listtab listtab--accurate">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>{cfg.title}</span>
          <small>{cfg.subtitle}</small>
        </div>
      </div>
      <div className="listtab__toolbar">
        <div className="listtab__filters">
          <label className="filter-pill">
            <span className="filter-pill__label">{editingId ? "Edit" : "New"}</span>
            <input
              className="filter-pill__input"
              type="text"
              value={name}
              placeholder="Name..."
              onChange={(e) => setName(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") {
                  e.preventDefault();
                  void submit();
                }
              }}
            />
          </label>
          <Button variant="filled" size="sm" disabled={saving || !name.trim()} onClick={submit}>
            {editingId ? "Update" : "Add"}
          </Button>
          {editingId && (
            <Button
              variant="text"
              size="sm"
              onClick={() => {
                setEditingId(null);
                setName("");
              }}
            >
              Cancel
            </Button>
          )}
        </div>
        <span className="listtab__count">{rows.length}</span>
      </div>
      <div className="listtab__body">
        {rows.length === 0 ? (
          <EmptyState title={cfg.emptyTitle} message={cfg.emptyMessage} />
        ) : (
          <div className="ledger-table">
            <div className="ledger-table__head">
              <span>Name</span>
              <span>Status</span>
              <span>Actions</span>
            </div>
            {rows.map((row) => (
              <div key={row.id} className="ledger-table__row">
                <span className="ledger-table__cat">{row.name}</span>
                <span>
                  <span className={`kind-mark ${row.is_active ? "is-positive" : "is-negative"}`}>
                    {row.is_active ? "Active" : "Inactive"}
                  </span>
                </span>
                <span className="ledger-table__action">
                  <Button
                    variant="text"
                    size="xs"
                    onClick={() => {
                      setEditingId(row.id);
                      setName(row.name);
                    }}
                  >
                    Edit
                  </Button>
                  <Button
                    variant="outlined"
                    size="xs"
                    danger
                    disabled={!row.is_active}
                    onClick={() => {
                      if (!window.confirm(`Deactivate "${row.name}"?`)) return;
                      const call = kind === "category" ? api.deactivateItemCategory(row.id) : api.deactivateItemBrand(row.id);
                      call
                        .then(() => {
                          setRows((prev) => prev.map((r) => (r.id === row.id ? { ...r, is_active: false } : r)));
                          showToast("Deactivated");
                        })
                        .catch((err) => showToast(err instanceof Error ? err.message : "Failed to deactivate", "error"));
                    }}
                  >
                    Deactivate
                  </Button>
                </span>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
