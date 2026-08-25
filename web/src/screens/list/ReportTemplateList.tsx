import { useEffect, useState } from "react";
import { useTabRefresh } from "../../workbench/useTabRefresh";
import { EmptyState, ErrorState, LoadingState } from "../../components/ui";
import { api } from "../../api";
import { useWorkbench } from "../../workbench/state";
import { REPORT_TEMPLATE_TYPES } from "../../types";
import type { ReportTemplate, CreateReportTemplateInput, ReportTemplateDocumentType } from "../../types";
import { Button } from "../../components/m3";

const DT_LABELS = REPORT_TEMPLATE_TYPES;

export function ReportTemplateList() {
  const workbench = useWorkbench();
  const [items, setItems] = useState<ReportTemplate[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [showForm, setShowForm] = useState(false);
  const [editingId, setEditingId] = useState<number | null>(null);
  const [formError, setFormError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [deletingId, setDeletingId] = useState<number | null>(null);

  // form fields
  const [code, setCode] = useState("");
  const [name, setName] = useState("");
  const [docType, setDocType] = useState<ReportTemplateDocumentType>("invoice");
  const [yaml, setYaml] = useState("");
  const [isDefault, setIsDefault] = useState(false);

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await api.listReportTemplates();
      setItems(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load report templates.");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void load();
  }, []);
  useTabRefresh(load);

  const resetForm = () => {
    setCode("");
    setName("");
    setDocType("invoice");
    setYaml("");
    setIsDefault(false);
    setFormError(null);
    setShowForm(false);
    setEditingId(null);
  };

  const openCreate = () => {
    setEditingId(null);
    setCode("");
    setName("");
    setDocType("invoice");
    setYaml("");
    setIsDefault(false);
    setFormError(null);
    setShowForm(true);
  };

  const openEdit = async (item: ReportTemplate) => {
    // The list endpoint omits template_yaml; fetch the full record.
    try {
      const full = await api.getReportTemplate(item.id);
      setEditingId(item.id);
      setCode(full.code);
      setName(full.name);
      setDocType(full.document_type);
      setYaml(full.template_yaml ?? "");
      setIsDefault(full.is_default);
      setFormError(null);
      setShowForm(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load template.");
    }
  };

  const handleSave = async () => {
    if (!code.trim() || !name.trim()) {
      setFormError("Code and name are required.");
      return;
    }
    if (!yaml.trim()) {
      setFormError("Template YAML content is required.");
      return;
    }
    setSaving(true);
    setFormError(null);
    try {
      const input: CreateReportTemplateInput = {
        code: code.trim(),
        name: name.trim(),
        document_type: docType,
        template_yaml: yaml,
      };
      if (editingId) {
        await api.updateReportTemplate(editingId, input);
      } else {
        await api.createReportTemplate(input);
      }
      await load();
      resetForm();
    } catch (err) {
      setFormError(err instanceof Error ? err.message : "Failed to save template.");
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async (id: number) => {
    const item = items.find((i) => i.id === id);
    if (!item) return;
    if (item.is_default) {
      alert("Cannot delete the default template for this document type.");
      return;
    }
    if (!window.confirm(`Delete template "${item.name}"? This cannot be undone.`)) return;
    setDeletingId(id);
    try {
      await api.deleteReportTemplate(id);
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to delete template.");
    } finally {
      setDeletingId(null);
    }
  };

  const renderPDF = async (item: ReportTemplate) => {
    try {
      const blob = await api.renderReportTemplate(item.id, "pdf");
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `${item.code}_rendered.pdf`;
      document.body.appendChild(a);
      a.click();
      a.remove();
      URL.revokeObjectURL(url);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to render PDF.");
    }
  };

  const renderHTML = async (item: ReportTemplate) => {
    try {
      const blob = await api.renderReportTemplate(item.id, "html");
      const url = URL.createObjectURL(blob);
      const win = window.open(url, "_blank");
      if (!win) {
        URL.revokeObjectURL(url);
        setError("Popup blocked — allow popups to preview rendered HTML.");
        return;
      }
      window.setTimeout(() => URL.revokeObjectURL(url), 60_000);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to render HTML.");
    }
  };

  const openEditor = (item: ReportTemplate) => {
    workbench.openEntryExisting("rp-editor", item.id, `Template · ${item.code}`, item.is_active ? "ACTIVE" : "INACTIVE");
  };

  const dtOptions = (Object.keys(DT_LABELS) as ReportTemplateDocumentType[]).map((dt) => (
    <option key={dt} value={dt}>{DT_LABELS[dt]}</option>
  ));

  return (
    <div className="listtab listtab--accurate">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>Report Templates</span>
          <small>YAML templates for invoices, reports, statements, and more.</small>
        </div>
        <div className="listtab__toolbar">
          {!showForm && items.length > 0 && (
            <Button
              variant="outlined"
              size="sm"
              onClick={() => void load()}
              disabled={loading}
            >
              Reload
            </Button>
          )}
          {showForm && editingId && (
            <Button
              variant="outlined"
              size="sm"
              onClick={() => { setShowForm(false); setEditingId(null); setFormError(null); }}
            >
              Cancel
            </Button>
          )}
          {showForm && !editingId && (
            <Button
              variant="outlined"
              size="sm"
              onClick={resetForm}
            >
              Cancel
            </Button>
          )}
        </div>
      </div>

      <div className="listtab__body">
        {loading ? (
          <LoadingState label="Loading templates..." />
        ) : error ? (
          <ErrorState message={error} onRetry={() => void load()} />
        ) : showForm ? (
          <div className="card" style={{ padding: "16px", marginBottom: "12px" }}>
            <div className="detail-grid" style={{ gridTemplateColumns: "140px 1fr", gap: "8px 12px", alignItems: "start" }}>
              <label className="field__label">Code *</label>
              <input
                className="input"
                value={code}
                onChange={(e) => setCode(e.target.value)}
                placeholder="e.g. INV-STD"
              />

              <label className="field__label">Name *</label>
              <input
                className="input"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="e.g. Standard Invoice"
              />

              <label className="field__label">Document Type *</label>
              <select className="input" value={docType} onChange={(e) => setDocType(e.target.value as ReportTemplateDocumentType)}>
                {dtOptions}
              </select>

              <label className="field__label">Template YAML *</label>
              <textarea
                className="input"
                rows={12}
                style={{ fontFamily: "var(--md-ref-typeface-plain)", fontSize: "12px" }}
                placeholder="# NextReport template YAML\nheader:\n  title: {{ invoice.number }}"
                value={yaml}
                onChange={(e) => setYaml(e.target.value)}
              />

              <label className="field__label">&nbsp;</label>
              <label className="field" style={{ display: "flex", alignItems: "center", gap: "8px" }}>
                <input type="checkbox" checked={isDefault} onChange={(e) => setIsDefault(e.target.checked)} />
                <span className="field__label">Set as default template</span>
              </label>
            </div>
            {formError ? <p style={{ color: "var(--md-sys-color-error)", margin: "8px 0 0" }}>{formError}</p> : null}
            <div style={{ marginTop: "12px" }}>
              <Button
                variant="filled"
                size="sm"
                onClick={handleSave}
                disabled={saving}
              >
                {saving ? "Saving..." : editingId ? "Update Template" : "Save Template"}
              </Button>
            </div>
          </div>
        ) : items.length === 0 ? (
          <EmptyState
            title="No templates yet"
            message="Add templates for invoices, reports, and statements."
            action={
              <Button variant="filled" onClick={openCreate}>
                + New Template
              </Button>
            }
          />
        ) : (
          <table className="data-table">
            <thead>
              <tr>
                <th>Code</th>
                <th>Name</th>
                <th>Document Type</th>
                <th>Default</th>
                <th>Active</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {items.map((item) => (
                <tr key={item.id}>
                  <td style={{ fontFamily: "var(--md-ref-typeface-plain)" }}>{item.code}</td>
                  <td>{item.name}</td>
                  <td>
                    <span className={`kind-mark ${item.is_active ? "is-positive" : "is-negative"}`}>
                      {DT_LABELS[item.document_type] || item.document_type}
                    </span>
                  </td>
                  <td>{item.is_default ? "Yes" : "—"}</td>
                  <td>{item.is_active ? "Yes" : "No"}</td>
                  <td>
                    <Button
                      variant="outlined"
                      size="xs"
                      onClick={() => openEdit(item)}
                    >
                      Edit
                    </Button>{" "}
                    <Button
                      variant="outlined"
                      size="xs"
                      onClick={() => openEditor(item)}
                    >
                      Editor
                    </Button>{" "}
                    <Button
                      variant="outlined"
                      size="xs"
                      onClick={() => renderPDF(item)}
                    >
                      PDF
                    </Button>{" "}
                    <Button
                      variant="outlined"
                      size="xs"
                      onClick={() => renderHTML(item)}
                    >
                      HTML
                    </Button>{" "}
                    {!item.is_default && (
                      <Button
                        variant="outlined"
                        size="xs"
                        danger
                        onClick={() => handleDelete(item.id)}
                        disabled={deletingId === item.id}
                      >
                        {deletingId === item.id ? "Deleting..." : "Del"}
                      </Button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      {!showForm && items.length > 0 && (
        <div className="listtab__footer">
          <span className="listtab__footer-count">{items.length} template(s)</span>
          <Button
            variant="filled"
            size="sm"
            onClick={openCreate}
          >
            + New Template
          </Button>
        </div>
      )}
    </div>
  );
}
