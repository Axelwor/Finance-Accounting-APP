import { useCallback, useEffect, useRef, useState } from "react";
import { FormError } from "./ui";
import { api } from "../api";
import type { Attachment } from "../types";
import { Button } from "./m3";

interface Props {
  ownerType: string;
  ownerId: number;
}

/**
 * AttachmentPanel (US-100 — Lampirkan Bukti).
 *
 * Reusable strip rendered at the bottom of entry forms. Loads and shows the
 * list of proof files attached to a given owner (journal entry, invoice,
 * etc.), with an upload button (file picker), a download link, and a delete
 * button. Self-contained: it fetches its own data and reloads after
 * upload/delete. When ownerId <= 0 (unsaved draft) upload is disabled.
 */
export function AttachmentPanel({ ownerType, ownerId }: Props) {
  const fileInput = useRef<HTMLInputElement>(null);
  const [items, setItems] = useState<Attachment[]>([]);
  const [uploading, setUploading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [deletingId, setDeletingId] = useState<number | null>(null);

  const load = useCallback(async () => {
    if (ownerId <= 0) {
      setItems([]);
      return;
    }
    try {
      const data = await api.listAttachments(ownerType, ownerId);
      setItems(data);
    } catch {
      setItems([]);
    }
  }, [ownerType, ownerId]);

  useEffect(() => {
    void load();
  }, [load]);

  const handlePick = useCallback(() => {
    fileInput.current?.click();
  }, []);

  const handleFile = useCallback(
    async (e: React.ChangeEvent<HTMLInputElement>) => {
      const file = e.target.files?.[0];
      if (!file) return;
      setError(null);
      if (file.size > 10 * 1024 * 1024) {
        setError("File is larger than 10MB.");
        e.target.value = "";
        return;
      }
      setUploading(true);
      try {
        await api.uploadAttachment(file, ownerType, ownerId);
        await load();
      } catch (err) {
        setError(err instanceof Error ? err.message : "Upload failed.");
      } finally {
        setUploading(false);
        e.target.value = "";
      }
    },
    [ownerType, ownerId, load],
  );

  const handleDownload = useCallback(async (item: Attachment) => {
    try {
      const blob = await api.downloadAttachment(item.id, item.file_name);
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = item.file_name;
      document.body.appendChild(a);
      a.click();
      a.remove();
      URL.revokeObjectURL(url);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Download failed.");
    }
  }, []);

  const handleDelete = useCallback(
    async (id: number) => {
      if (!window.confirm("Delete this attachment? This cannot be undone.")) return;
      setDeletingId(id);
      setError(null);
      try {
        await api.deleteAttachment(id);
        await load();
      } catch (err) {
        setError(err instanceof Error ? err.message : "Delete failed.");
      } finally {
        setDeletingId(null);
      }
    },
    [load],
  );

  return (
    <div className="attachment-panel">
      <div className="attachment-panel__head">
        <span className="attachment-panel__title">Attachments</span>
        <Button
          variant="outlined"
          size="sm"
          onClick={handlePick}
          disabled={uploading || ownerId <= 0}
          title="Upload a photo or PDF as proof"
        >
          {uploading ? "Uploading..." : "+ Add file"}
        </Button>
        <input
          ref={fileInput}
          type="file"
          accept="image/jpeg,image/png,application/pdf"
          onChange={(e) => void handleFile(e)}
          style={{ display: "none" }}
        />
      </div>

      {items.length > 0 ? (
        <div className="ledger-table">
          <div className="ledger-table__head">
            <span>File</span>
            <span>Type</span>
            <span className="right">Size</span>
            <span>OCR</span>
            <span />
          </div>
          {items.map((item) => (
            <div className="ledger-table__row" key={item.id}>
              <span className="ledger-table__party" onClick={() => void handleDownload(item)} role="button">
                {item.file_name}
              </span>
              <span className="ledger-table__cat">{iconFor(item.mime_type)}</span>
              <span className="ledger-table__amount right">{formatSize(item.file_size)}</span>
              <span>
                <span className={`kind-mark ${item.ocr_status === "COMPLETED" ? "is-positive" : ""}`}>{item.ocr_status}</span>
              </span>
              <span className="ledger-table__actions">
                <Button
                  variant="outlined"
                  size="sm"
                  onClick={() => void handleDownload(item)}
                >
                  Download
                </Button>
                <Button
                  variant="text"
                  size="sm"
                  onClick={() => void handleDelete(item.id)}
                  disabled={deletingId === item.id}
                >
                  {deletingId === item.id ? "Deleting..." : "Delete"}
                </Button>
              </span>
            </div>
          ))}
        </div>
      ) : (
        <p className="attachment-panel__empty">No attachments yet. Upload a photo struk or invoice scan as proof.</p>
      )}
      <FormError message={error} />
    </div>
  );
}

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function iconFor(mime: string): string {
  if (mime.startsWith("image/")) return "Image";
  if (mime === "application/pdf") return "PDF";
  return "File";
}
