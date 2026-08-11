import { useEffect, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { api } from "../../api";
import { REPORT_TEMPLATE_TYPES } from "../../types";
import type { CreateReportTemplateInput, ReportTemplateDocumentType } from "../../types";

interface Props {
  tabId: string;
  entryId?: string | number;
  initialTitle?: string;
}

const DT_LABELS = REPORT_TEMPLATE_TYPES;

/**
 * Report template editor (N-07/N-08): YAML source on the left, live HTML
 * preview on the right. Preview calls the render endpoint (format=html) on
 * the saved template and shows the bytes in an iframe via an object URL.
 */
export function ReportTemplateEditor({ tabId, entryId, initialTitle }: Props) {
  const workbench = useWorkbench();

  const [templateId, setTemplateId] = useState<number | null>(entryId != null ? Number(entryId) : null);
  const [code, setCode] = useState("");
  const [name, setName] = useState("");
  const [docType, setDocType] = useState<ReportTemplateDocumentType>("invoice");
  const [yaml, setYaml] = useState("");
  const [loading, setLoading] = useState(templateId != null);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [previewUrl, setPreviewUrl] = useState<string | null>(null);
  const [previewing, setPreviewing] = useState(false);
  const [previewError, setPreviewError] = useState("");

  useEffect(() => {
    if (templateId == null) return;
    let cancelled = false;
    api
      .getReportTemplate(templateId)
      .then((tpl) => {
        if (cancelled) return;
        setCode(tpl.code);
        setName(tpl.name);
        setDocType(tpl.document_type);
        setYaml(tpl.template_yaml ?? "");
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setError(err instanceof Error ? err.message : "Failed to load template.");
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [templateId]);

  useEffect(() => {
    return () => {
      if (previewUrl) URL.revokeObjectURL(previewUrl);
    };
  }, [previewUrl]);

  const touch = () => {
    setNotice("");
    workbench.markUnsaved(tabId, true);
  };

  const refreshPreview = async (id: number) => {
    setPreviewing(true);
    setPreviewError("");
    try {
      const blob = await api.renderReportTemplate(id, "html");
      setPreviewUrl((prev) => {
        if (prev) URL.revokeObjectURL(prev);
        return URL.createObjectURL(blob);
      });
    } catch (err) {
      setPreviewError(err instanceof Error ? err.message : "Failed to render preview.");
    } finally {
      setPreviewing(false);
    }
  };

  useEffect(() => {
    if (templateId != null) void refreshPreview(templateId);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [templateId]);

  const buildInput = (): CreateReportTemplateInput | null => {
    if (!code.trim() || !name.trim()) {
      setError("Code and name are required.");
      return null;
    }
    if (!yaml.trim()) {
      setError("Template YAML content is required.");
      return null;
    }
    setError("");
    return { code: code.trim(), name: name.trim(), document_type: docType, template_yaml: yaml };
  };

  const handleSave = async () => {
    const input = buildInput();
    if (!input) return;
    setSaving(true);
    try {
      if (templateId != null) {
        await api.updateReportTemplate(templateId, input);
        setNotice("Template saved.");
      } else {
        const created = await api.createReportTemplate(input);
        setTemplateId(created.id);
        workbench.replaceDraft(tabId, `${input.code} · ${input.name}`, "SAVED");
        setNotice("Template created.");
      }
      workbench.markUnsaved(tabId, false);
      await refreshPreview(templateId ?? 0).catch(() => undefined);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to save template.");
    } finally {
      setSaving(false);
    }
  };

  const handleSaveAsNew = async () => {
    const input = buildInput();
    if (!input) return;
    setSaving(true);
    try {
      const created = await api.createReportTemplate(input);
      setTemplateId(created.id);
      workbench.replaceDraft(tabId, `${input.code} · ${input.name}`, "SAVED");
      workbench.markUnsaved(tabId, false);
      setNotice("Saved as a new template.");
      await refreshPreview(created.id);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create template.");
    } finally {
      setSaving(false);
    }
  };

  const handleDownloadPdf = async () => {
    if (templateId == null) {
      setError("Save the template before downloading a PDF.");
      return;
    }
    try {
      const blob = await api.renderReportTemplate(templateId, "pdf");
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `${code || "template"}_rendered.pdf`;
      document.body.appendChild(a);
      a.click();
      a.remove();
      URL.revokeObjectURL(url);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to render PDF.");
    }
  };

  const dtOptions = (Object.keys(DT_LABELS) as ReportTemplateDocumentType[]).map((dt) => (
    <option key={dt} value={dt}>
      {DT_LABELS[dt]}
    </option>
  ));

  if (loading) {
    return <div className="entrytab rp-editor"><p className="rp-editor__status">Loading template...</p></div>;
  }

  return (
    <div className="entrytab rp-editor">
      <div className="entrytab__header">
        <div className="entrytab__header-info">
          <div className="entrytab__header-title">{initialTitle || "Report Template"}</div>
          <div className="entrytab__header-number">
            {templateId != null ? `Template #${templateId}` : "New template"}
          </div>
        </div>
        <div className="entrytab__actions">
          <button type="button" className="btn btn--secondary btn--sm" onClick={() => templateId != null && void refreshPreview(templateId)} disabled={previewing || templateId == null}>
            {previewing ? "Rendering..." : "Refresh Preview"}
          </button>
          <button type="button" className="btn btn--secondary btn--sm" onClick={() => void handleDownloadPdf()}>
            Download PDF
          </button>
          <button type="button" className="btn btn--secondary btn--sm" onClick={() => void handleSaveAsNew()} disabled={saving}>
            Save As New
          </button>
          <button type="button" className="btn btn--primary btn--sm" onClick={() => void handleSave()} disabled={saving}>
            {saving ? "Saving..." : "Save"}
          </button>
        </div>
      </div>

      {error ? <p className="field__error">{error}</p> : null}
      {notice ? <p className="field__hint">{notice}</p> : null}

      <div className="entrytab__section rp-editor__fields">
        <label className="field">
          <span className="field__label">Code *</span>
          <input className="input" value={code} onChange={(e) => { setCode(e.target.value); touch(); }} placeholder="e.g. INV-STD" />
        </label>
        <label className="field">
          <span className="field__label">Name *</span>
          <input className="input" value={name} onChange={(e) => { setName(e.target.value); touch(); }} placeholder="e.g. Standard Invoice" />
        </label>
        <label className="field">
          <span className="field__label">Document Type *</span>
          <select className="input" value={docType} onChange={(e) => { setDocType(e.target.value as ReportTemplateDocumentType); touch(); }}>
            {dtOptions}
          </select>
        </label>
      </div>

      <div className="rp-editor__split">
        <div className="rp-editor__pane">
          <div className="rp-editor__pane-title">Template YAML</div>
          <textarea
            className="rp-editor__yaml"
            spellCheck={false}
            placeholder={"# Report template YAML\nheader:\n  title: {{ invoice.number }}"}
            value={yaml}
            onChange={(e) => { setYaml(e.target.value); touch(); }}
          />
        </div>
        <div className="rp-editor__pane">
          <div className="rp-editor__pane-title">
            HTML Preview
            {previewError ? <span className="field__error"> · {previewError}</span> : null}
          </div>
          {previewUrl ? (
            <iframe className="rp-editor__preview" title="Template preview" src={previewUrl} sandbox="" />
          ) : (
            <div className="rp-editor__preview-empty">
              {templateId == null
                ? "Save the template to render a preview."
                : previewing
                  ? "Rendering..."
                  : "No preview yet. Use Refresh Preview."}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
