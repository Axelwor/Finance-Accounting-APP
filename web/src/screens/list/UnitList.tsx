import { useEffect, useState } from "react";
import { api } from "../../api";
import type { UnitMaster } from "../../types";
import { EmptyState, LoadingState, FormError } from "../../components/ui";
import { Button } from "../../components/m3";
import { showToast } from "../../lib/toast";

/** Unit (satuan) master list (SET-001). */
export function UnitList() {
  const [rows, setRows] = useState<UnitMaster[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [form, setForm] = useState({ code: "", name: "", decimal_places: 0 });
  const [editingId, setEditingId] = useState<number | null>(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    api
      .listUnits()
      .then(setRows)
      .catch(() => setError("Failed to load units"))
      .finally(() => setLoading(false));
  }, []);

  if (loading) return <LoadingState label="Loading units..." />;
  if (error) return <FormError message={error} />;

  const submit = async () => {
    if (!form.code.trim() || !form.name.trim()) return;
    setSaving(true);
    try {
      const payload = { code: form.code.trim(), name: form.name.trim(), decimal_places: form.decimal_places };
      if (editingId) {
        const updated = await api.updateUnit(editingId, payload);
        setRows((prev) => prev.map((r) => (r.id === updated.id ? updated : r)));
        showToast("Unit updated");
      } else {
        const created = await api.createUnit(payload);
        setRows((prev) => [...prev, created]);
        showToast("Unit created");
      }
      setForm({ code: "", name: "", decimal_places: 0 });
      setEditingId(null);
    } catch (err) {
      showToast(err instanceof Error ? err.message : "Failed to save unit", "error");
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="listtab listtab--accurate">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>Units</span>
          <small>Units of measure used by items and documents.</small>
        </div>
      </div>
      <div className="listtab__toolbar">
        <div className="listtab__filters">
          <label className="filter-pill">
            <span className="filter-pill__label">Code</span>
            <input
              className="filter-pill__input"
              type="text"
              value={form.code}
              placeholder="PCS"
              style={{ width: 90 }}
              onChange={(e) => setForm((f) => ({ ...f, code: e.target.value.toUpperCase() }))}
            />
          </label>
          <label className="filter-pill">
            <span className="filter-pill__label">Name</span>
            <input
              className="filter-pill__input"
              type="text"
              value={form.name}
              placeholder="Pieces"
              onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
            />
          </label>
          <label className="filter-pill">
            <span className="filter-pill__label">Decimals</span>
            <input
              className="filter-pill__input"
              type="number"
              min={0}
              max={4}
              value={form.decimal_places}
              style={{ width: 70 }}
              onChange={(e) => setForm((f) => ({ ...f, decimal_places: Number(e.target.value) }))}
            />
          </label>
          <Button variant="filled" size="sm" disabled={saving || !form.code.trim() || !form.name.trim()} onClick={submit}>
            {editingId ? "Update" : "Add"}
          </Button>
          {editingId && (
            <Button
              variant="text"
              size="sm"
              onClick={() => {
                setEditingId(null);
                setForm({ code: "", name: "", decimal_places: 0 });
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
          <EmptyState title="No units yet" message="Add units of measure (pcs, kg, box) for your items." />
        ) : (
          <div className="ledger-table">
            <div className="ledger-table__head">
              <span>Code</span>
              <span>Name</span>
              <span>Decimals</span>
              <span>Status</span>
              <span>Actions</span>
            </div>
            {rows.map((row) => (
              <div key={row.id} className="ledger-table__row">
                <span className="ledger-table__no">{row.code}</span>
                <span className="ledger-table__cat">{row.name}</span>
                <span className="ledger-table__memo">{row.decimal_places}</span>
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
                      setForm({ code: row.code, name: row.name, decimal_places: row.decimal_places });
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
                      if (!window.confirm(`Deactivate unit "${row.code}"?`)) return;
                      api
                        .deactivateUnit(row.id)
                        .then(() => {
                          setRows((prev) => prev.map((r) => (r.id === row.id ? { ...r, is_active: false } : r)));
                          showToast("Unit deactivated");
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
