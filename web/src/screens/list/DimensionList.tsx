import { useEffect, useState } from "react";
import { EmptyState, ErrorState, LoadingState } from "../../components/ui";
import { api } from "../../api";
import type { Dimension } from "../../types";

type DimensionType = Dimension["dimension_type"];

const DIMENSION_TYPE_LABEL: Record<string, string> = {
  branch: "Branch (Cabang)",
  project: "Project (Proyek)",
  department: "Department (Departemen)",
  cost_center: "Cost Center",
};

/**
 * Dimensions list (US-093): master data for cabang / proyek / departemen /
 * cost center. Dimensions tag journal lines so reports can be filtered by
 * dimension and budgets can be scoped to a dimension.
 */
export function DimensionList() {
  const [items, setItems] = useState<Dimension[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [showForm, setShowForm] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  // form fields
  const [code, setCode] = useState("");
  const [name, setName] = useState("");
  const [dimType, setDimType] = useState<DimensionType>("branch");

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await api.listDimensions();
      setItems(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load dimensions.");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void load();
  }, []);

  const resetForm = () => {
    setCode("");
    setName("");
    setDimType("branch");
    setFormError(null);
    setShowForm(false);
  };

  const handleSave = async () => {
    if (!code.trim() || !name.trim()) {
      setFormError("Code and name are required.");
      return;
    }
    setSaving(true);
    setFormError(null);
    try {
      await api.createDimension({
        code: code.trim(),
        name: name.trim(),
        dimension_type: dimType,
      });
      resetForm();
      await load();
    } catch (err) {
      setFormError(err instanceof Error ? err.message : "Failed to create dimension.");
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="listtab listtab--accurate">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>Dimensions</span>
          <small>Cabang / Proyek / Departemen / Cost Center</small>
        </div>
        <div className="listtab__toolbar">
          <button type="button" className="btn btn--secondary btn--sm" onClick={() => void load()}>
            Reload
          </button>
          {!showForm ? (
            <button type="button" className="btn btn--primary btn--sm" onClick={() => setShowForm(true)}>
              + New Dimension
            </button>
          ) : null}
        </div>
      </div>

      <div className="listtab__body">
        {loading ? (
          <LoadingState label="Loading..." />
        ) : error ? (
          <ErrorState message={error} onRetry={() => void load()} />
        ) : showForm ? (
          <div className="card" style={{ padding: "16px", marginBottom: "12px" }}>
            <div className="detail-grid" style={{ gridTemplateColumns: "120px 1fr", gap: "8px 12px", alignItems: "center" }}>
              <label className="field__label">Code</label>
              <input
                className="input"
                value={code}
                onChange={(e) => setCode(e.target.value)}
                placeholder="e.g. BR-01"
                style={{ padding: "6px 10px" }}
              />
              <label className="field__label">Name</label>
              <input
                className="input"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="e.g. Jakarta Branch"
                style={{ padding: "6px 10px" }}
              />
              <label className="field__label">Type</label>
              <select
                className="input"
                value={dimType}
                onChange={(e) => setDimType(e.target.value as DimensionType)}
                style={{ padding: "6px 10px" }}
              >
                <option value="branch">Branch (Cabang)</option>
                <option value="project">Project (Proyek)</option>
                <option value="department">Department (Departemen)</option>
                <option value="cost_center">Cost Center</option>
              </select>
            </div>
            {formError ? (
              <p style={{ color: "var(--neg)", margin: "8px 0 0" }}>{formError}</p>
            ) : null}
            <div style={{ display: "flex", gap: "8px", marginTop: "12px" }}>
              <button type="button" className="btn btn--primary btn--sm" disabled={saving} onClick={() => void handleSave()}>
                {saving ? "Saving..." : "Save"}
              </button>
              <button type="button" className="btn btn--secondary btn--sm" onClick={resetForm}>
                Cancel
              </button>
            </div>
          </div>
        ) : null}

        {!loading && !error ? (
          items.length === 0 && !showForm ? (
            <EmptyState title="No dimensions" message="Create a branch, project, or department to tag journal lines." />
          ) : items.length > 0 ? (
            <table className="data-table">
              <thead>
                <tr>
                  <th>Code</th>
                  <th>Name</th>
                  <th>Type</th>
                  <th>Active</th>
                </tr>
              </thead>
              <tbody>
                {items.map((d) => (
                  <tr key={d.id}>
                    <td style={{ fontFamily: "var(--font-mono)" }}>{d.code}</td>
                    <td>{d.name}</td>
                    <td>{DIMENSION_TYPE_LABEL[d.dimension_type] ?? d.dimension_type}</td>
                    <td>{d.is_active ? "Yes" : "No"}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          ) : null
        ) : null}
      </div>
      <div className="listtab__footer">
        <span className="listtab__footer-count">{items.length} Dimension(s)</span>
      </div>
    </div>
  );
}
