import { useEffect, useState } from "react";
import { useWorkbench } from "../../workbench/state";
import { FormError } from "../../components/ui";
import { api } from "../../api";
import { draftNumber } from "../../workbench/modules";
import type { FinancialNote } from "../../types";

interface Props {
  tabId: string;
  entryId?: string | number;
  initialTitle?: string;
}

/**
 * Financial Note form (create / edit).
 *
 * A note is a free-text disclosure attached to a fiscal year (period_year).
 * Create posts a new note; editing an existing note loads it by id and
 * updates via PUT. There is no journal posting — notes are report-only.
 */
export function FinancialNoteForm({ tabId, entryId, initialTitle }: Props) {
  const workbench = useWorkbench();
  const isExisting = entryId !== undefined;

  const [periodYear, setPeriodYear] = useState(currentYear());
  const [noteNumber, setNoteNumber] = useState("");
  const [title, setTitle] = useState("");
  const [content, setContent] = useState("");
  const [displayOrder, setDisplayOrder] = useState(0);
  const [existing, setExisting] = useState<FinancialNote | null>(null);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!entryId) return;
    const id = Number(entryId);
    if (!Number.isFinite(id)) return;
    api
      .getFinancialNote(id)
      .then((note) => {
        setExisting(note);
        setPeriodYear(note.period_year);
        setNoteNumber(note.note_number);
        setTitle(note.title);
        setContent(note.content);
        setDisplayOrder(note.display_order);
      })
      .catch(() => {
        /* leave form blank if load fails */
      });
  }, [entryId]);

  function handleChange(field: "periodYear" | "noteNumber" | "title" | "content" | "displayOrder", value: string | number) {
    if (field === "periodYear") setPeriodYear(Number(value));
    else if (field === "displayOrder") setDisplayOrder(Number(value));
    else if (field === "noteNumber") setNoteNumber(String(value));
    else if (field === "title") setTitle(String(value));
    else setContent(String(value));
    if (!isExisting) workbench.markUnsaved(tabId, true);
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    if (!periodYear || periodYear <= 0) {
      setError("Period year is required.");
      return;
    }
    if (!noteNumber.trim()) {
      setError("Note number is required.");
      return;
    }
    if (!title.trim()) {
      setError("Title is required.");
      return;
    }
    if (!content.trim()) {
      setError("Content is required.");
      return;
    }

    const payload = {
      period_year: periodYear,
      note_number: noteNumber.trim(),
      title: title.trim(),
      content,
      display_order: displayOrder,
    };
    setSaving(true);
    try {
      if (isExisting && existing) {
        const updated = await api.updateFinancialNote(existing.id, payload);
        setExisting(updated);
        workbench.replaceDraft(tabId, `${updated.note_number} · ${updated.title}`, "SAVED");
        workbench.markUnsaved(tabId, false);
      } else {
        const created = await api.createFinancialNote(payload);
        setExisting(created);
        workbench.replaceDraft(tabId, `${created.note_number} · ${created.title}`, "SAVED");
        workbench.markUnsaved(tabId, false);
      }
    } catch (err: any) {
      setError(err?.message || "Failed to save the note.");
    } finally {
      setSaving(false);
    }
  }

  return (
    <form className="entrytab" onSubmit={handleSubmit}>
      <div className="entrytab__header">
        <div className="entrytab__header-info">
          <div className="entrytab__header-title">{initialTitle || "Financial Note"}</div>
          <div className="entrytab__header-number">
            {isExisting ? `${existing?.note_number ?? ""} · ${existing?.title ?? ""}` : draftNumber("financial-notes-entry")}
          </div>
        </div>
      </div>
      <div className="entrytab__body">
        <div className="entrytab__detail">
          <div className="entrytab__detail-grid">
            <label className="field">
              <span className="field__label">Period Year *</span>
              <input
                className="input"
                type="number"
                min="1900"
                max="2999"
                value={periodYear}
                onChange={(e) => handleChange("periodYear", e.target.value)}
              />
            </label>
            <label className="field">
              <span className="field__label">Note Number *</span>
              <input
                className="input"
                type="text"
                placeholder="e.g. 1, 2A, 3.1"
                value={noteNumber}
                onChange={(e) => handleChange("noteNumber", e.target.value)}
              />
            </label>
            <label className="field">
              <span className="field__label">Title *</span>
              <input
                className="input"
                type="text"
                placeholder="e.g. Significant Accounting Policies"
                value={title}
                onChange={(e) => handleChange("title", e.target.value)}
              />
            </label>
            <label className="field">
              <span className="field__label">Display Order</span>
              <input
                className="input"
                type="number"
                min="0"
                value={displayOrder}
                onChange={(e) => handleChange("displayOrder", e.target.value)}
              />
            </label>
          </div>

          <label className="field">
            <span className="field__label">Content *</span>
            <textarea
              className="input"
              rows={14}
              placeholder="Disclosure text — policies, breakdowns, commitments, contingencies..."
              value={content}
              onChange={(e) => handleChange("content", e.target.value)}
            />
          </label>
        </div>

        <aside className="action-rail" aria-label="Form actions">
          <button type="submit" className="action-rail__btn action-rail__btn--primary" disabled={saving}>
            <span>{saving ? "Saving..." : isExisting ? "Update" : "Save"}</span>
          </button>
        </aside>

        <FormError message={error} />
      </div>
    </form>
  );
}

function currentYear(): number {
  return new Date().getFullYear();
}
