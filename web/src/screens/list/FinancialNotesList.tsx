import { useEffect, useMemo, useState } from "react";
import { useTabRefresh } from "../../workbench/useTabRefresh";
import { useWorkbench } from "../../workbench/state";
import { EmptyState, ErrorState, LoadingState } from "../../components/ui";
import { api } from "../../api";
import type { FinancialNote } from "../../types";
import { Button, IconButton } from "../../components/m3";

/**
 * Financial Notes list (Catatan atas Laporan Keuangan).
 *
 * Notes are free-text disclosures attached to a fiscal year, presented
 * alongside the Laporan Posisi Keuangan. Each note carries a number,
 * title, and long-form content. The list groups notes by period_year
 * and supports create / edit / delete.
 */
export function FinancialNotesList() {
  const workbench = useWorkbench();
  const [items, setItems] = useState<FinancialNote[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [deletingId, setDeletingId] = useState<number | null>(null);

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await api.listFinancialNotes();
      setItems(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load financial notes.");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void load();
  }, []);
  useTabRefresh(load);

  const grouped = useMemo(() => {
    const map = new Map<number, FinancialNote[]>();
    for (const it of items) {
      const list = map.get(it.period_year) ?? [];
      list.push(it);
      map.set(it.period_year, list);
    }
    return [...map.entries()].sort((a, b) => b[0] - a[0]);
  }, [items]);

  const handleDelete = async (id: number) => {
    if (!window.confirm("Delete this note? This cannot be undone.")) return;
    setDeletingId(id);
    try {
      await api.deleteFinancialNote(id);
      setItems((prev) => prev.filter((it) => it.id !== id));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to delete note.");
    } finally {
      setDeletingId(null);
    }
  };

  const openEntry = (item: FinancialNote) =>
    workbench.openEntryExisting("financial-notes-entry", item.id, `${item.note_number} · ${item.title}`);

  return (
    <div className="listtab listtab--accurate">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>Financial Notes</span>
          <small>Catatan atas Laporan Keuangan — disclosures attached to each fiscal year.</small>
        </div>
      </div>
      <div className="listtab__toolbar">
        <div className="listtab__filters" />
        <div className="listtab__actions">
          <Button
            variant="filled"
            size="sm"
            onClick={() => workbench.openEntryDraft("financial-notes-entry")}
          >
            + New Note
          </Button>
          <IconButton
            size="sm"
            onClick={() => void load()}
            label="Reload"
          >
            <ReloadIcon />
          </IconButton>
          <span className="listtab__count">{items.length}</span>
        </div>
      </div>
      <div className="listtab__body">
        {loading ? (
          <LoadingState label="Loading financial notes..." />
        ) : error ? (
          <ErrorState message={error} onRetry={() => void load()} />
        ) : items.length === 0 ? (
          <EmptyState
            title="No financial notes yet"
            message="Add a note to disclose accounting policies, breakdowns, or commitments attached to the Laporan Posisi Keuangan."
            action={
              <Button variant="filled" onClick={() => workbench.openEntryDraft("financial-notes-entry")}>
                New Note
              </Button>
            }
          />
        ) : (
          <div className="notes-list">
            {grouped.map(([year, notes]) => (
              <section className="notes-list__group" key={year}>
                <header className="notes-list__group-head">
                  <span className="notes-list__group-year">Fiscal Year {year}</span>
                  <span className="notes-list__group-count">{notes.length} note(s)</span>
                </header>
                <table className="ledger-table" aria-label={`Financial notes for fiscal year ${year}`}>
                  <thead>
                    <tr>
                      <th scope="col">No.</th>
                      <th scope="col">Title</th>
                      <th scope="col">Content</th>
                      <th scope="col" />
                    </tr>
                  </thead>
                  <tbody>
                    {notes.map((it) => (
                      <FRow key={it.id} item={it} onOpen={() => openEntry(it)} onDelete={handleDelete} />
                    ))}
                  </tbody>
                </table>
              </section>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

function FRow({ item, onOpen, onDelete }: { item: FinancialNote; onOpen: () => void; onDelete: (id: number) => void }) {
  return (
    <tr role="button" tabIndex={0} onClick={onOpen} onKeyDown={(e) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); onOpen(); } }} style={{ cursor: "pointer" }}>
      <td><span>{item.note_number}</span></td>
      <td><span>{item.title}</span></td>
      <td><span>{previewContent(item.content)}</span></td>
      <td>
        <span>
          <Button
            variant="outlined"
            size="sm"
            onClick={(e) => { e.stopPropagation(); onOpen(); }}
          >
            Edit
          </Button>
          <Button
            variant="text"
            size="sm"
            onClick={(e) => { e.stopPropagation(); onDelete(item.id); }}
          >
            Delete
          </Button>
        </span>
      </td>
    </tr>
  );
}

function previewContent(content: string): string {
  const trimmed = (content ?? "").trim();
  if (!trimmed) return "—";
  const firstLine = trimmed.split(/\r?\n/)[0];
  return firstLine.length > 80 ? firstLine.slice(0, 80) + "…" : firstLine;
}

function ReloadIcon() {
  return (
    <svg viewBox="0 0 24 24" width="14" height="14" aria-hidden="true">
      <path
        d="M4 12a8 8 0 0 1 14-5l2-2v6h-6l2-2a6 6 0 0 0-10 3M20 12a8 8 0 0 1-14 5l-2 2v-6h6l-2 2a6 6 0 0 0 10-3"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinecap="round"
        strokeLinejoin="round"
        fill="none"
      />
    </svg>
  );
}
