import { useMemo, useState, type ReactNode } from "react";
import { EmptyState } from "../../components/ui";

/** Tone class for amount cells. */
export type AmountTone = "is-positive" | "is-negative" | "is-muted" | "";

export interface MockListColumn<T> {
  /** Unique key for the column. Also used as the default React key. */
  key: string;
  /** Header label (rendered uppercase by the existing listtab CSS). */
  label: string;
  /** When true, header and cells are right-aligned. */
  align?: "right";
  /** Optional tone class for amount cells. */
  tone?: (row: T) => AmountTone;
  /** Renderer for a single row. */
  render: (row: T) => ReactNode;
  /** When true, the column is rendered through the rich desc cell (title + meta). */
  primary?: boolean;
  /** When primary, this renders the secondary line under the title. */
  secondary?: (row: T) => ReactNode;
}

interface Props<T> {
  title: string;
  description: string;
  /** Module identifier — purely informational, labels the demo badge. */
  kind: "sales" | "purchases" | "inventory" | "fixed-assets";
  columns: MockListColumn<T>[];
  rows: T[];
  /** Fields used by the client-side search input. */
  searchFields: (keyof T)[];
  /** Optional callback — opens a draft entry. */
  onAdd?: () => void;
  /** Optional callback — opens an existing entry. */
  onOpen?: (row: T) => void;
  /** Optional search placeholder. */
  searchPlaceholder?: string;
  /** Stable id per row (used as the React key). */
  getRowKey: (row: T) => string;
}

/**
 * Generic list view used by Sales, Purchases, Inventory, and Fixed Assets
 * modules while their real backends land. The data is passed in pre-shaped;
 * MockList owns search + sort + the table chrome only.
 */
export function MockList<T>({
  title,
  description,
  kind,
  columns,
  rows,
  searchFields,
  onAdd,
  onOpen,
  searchPlaceholder = "filter rows...",
  getRowKey,
}: Props<T>) {
  const [search, setSearch] = useState("");
  const [reloadKey, setReloadKey] = useState(0);

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase();
    if (!q) return rows;
    return rows.filter((row) =>
      searchFields.some((field) => {
        const value = row[field];
        if (value === null || value === undefined) return false;
        return String(value).toLowerCase().includes(q);
      }),
    );
    // reloadKey is intentionally part of the dependency list so the
    // "Reload" button feels alive even though data is in-memory.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [rows, search, reloadKey]);

  const handleReload = () => setReloadKey((k) => k + 1);

  const primaryColumn = columns.find((c) => c.primary);
  const regularColumns = columns.filter((c) => !c.primary);

  // The base CSS for `.ledger-table__row` is hard-coded to 5 columns.
  // When we have more (or fewer) we override grid-template-columns
  // inline so the cells line up without touching styles/components.css.
  const totalColumns = (primaryColumn ? 1 : 0) + regularColumns.length + 1; // +1 chevron
  const gridTemplateColumns = buildGridTemplateColumns(
    totalColumns,
    primaryColumn !== undefined,
    regularColumns,
  );

  return (
    <div className="listtab">
      <div className="listtab__head">
        <div className="listtab__title">
          <span>{title}</span>
          <span className="listtab__demo" title={`${kind} module — mock data`}>
            Demo data
          </span>
          <small>{filtered.length} {filtered.length === 1 ? "row" : "rows"}</small>
        </div>
        <div className="listtab__toolbar">
          {onAdd ? (
            <button type="button" className="btn btn--ink btn--sm" onClick={onAdd}>
              + Tambah
            </button>
          ) : null}
          <button type="button" className="btn btn--secondary btn--sm" onClick={handleReload}>
            Reload
          </button>
        </div>
      </div>

      <div className="listtab__filters">
        <label className="listtab__filter">
          <span>Search</span>
          <input
            type="search"
            placeholder={searchPlaceholder}
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
        </label>
      </div>

      <div className="listtab__body">
        {filtered.length === 0 ? (
          <EmptyState
            title="No rows match"
            message="Clear the search or rule a new entry with the + Tambah button."
            action={
              onAdd ? (
                <button type="button" className="btn btn--primary btn--sm" onClick={onAdd}>
                  + Tambah
                </button>
              ) : undefined
            }
          />
        ) : (
          <div className="ledger-table">
            <div className="ledger-table__head" style={{ gridTemplateColumns }}>
              {primaryColumn ? (
                <span>{primaryColumn.label}</span>
              ) : null}
              {regularColumns.map((col) => (
                <span key={col.key} className={col.align === "right" ? "right" : undefined}>
                  {col.label}
                </span>
              ))}
              <span aria-hidden="true" />
            </div>
            {filtered.map((row) => (
              <MockListRow
                key={getRowKey(row)}
                row={row}
                primaryColumn={primaryColumn}
                columns={regularColumns}
                gridTemplateColumns={gridTemplateColumns}
                onOpen={onOpen ? () => onOpen(row) : undefined}
              />
            ))}
          </div>
        )}
      </div>

      <div className="listtab__footer">
        <span>
          Showing <strong>{filtered.length}</strong> of {rows.length} &middot;{" "}
          <span style={{ color: "var(--ink-muted)" }}>{description}</span>
        </span>
        <span style={{ color: "var(--ink-muted)" }}>
          Module: <strong style={{ color: "var(--ink-tertiary)" }}>{kind}</strong>
        </span>
      </div>
    </div>
  );
}

function MockListRow<T>({
  row,
  primaryColumn,
  columns,
  gridTemplateColumns,
  onOpen,
}: {
  row: T;
  primaryColumn?: MockListColumn<T>;
  columns: MockListColumn<T>[];
  gridTemplateColumns: string;
  onOpen?: () => void;
}) {
  // When the consumer supplies an onOpen, the whole row becomes a button.
  const interactive = Boolean(onOpen);
  const interactiveProps = interactive
    ? {
        role: "button",
        tabIndex: 0,
        onClick: onOpen,
        onKeyDown: (e: React.KeyboardEvent) => {
          if (e.key === "Enter" || e.key === " ") {
            e.preventDefault();
            onOpen?.();
          }
        },
        style: { cursor: "pointer" },
      }
    : {};

  return (
    <div className="ledger-table__row" style={{ gridTemplateColumns }} {...interactiveProps}>
      {primaryColumn ? (
        <div className="ledger-table__desc">
          <div className="ledger-table__desc-text">
            <span className="ledger-table__desc-title">{primaryColumn.render(row) as ReactNode}</span>
            {primaryColumn.secondary ? (
              <span className="ledger-table__desc-meta">{primaryColumn.secondary(row) as ReactNode}</span>
            ) : null}
          </div>
        </div>
      ) : null}
      {columns.map((col) => {
        const tone = col.tone ? col.tone(row) : "";
        if (col.align === "right") {
          return (
            <span key={col.key} className={`ledger-table__amount ${tone}`}>
              {col.render(row)}
            </span>
          );
        }
        return <span key={col.key}>{col.render(row)}</span>;
      })}
      <span className="ledger-table__delete" aria-hidden="true">
        {interactive ? "›" : ""}
      </span>
    </div>
  );
}

/** Build a grid-template-columns string for the table row. */
function buildGridTemplateColumns<T>(
  totalColumns: number,
  hasPrimary: boolean,
  regularColumns: MockListColumn<T>[],
): string {
  // 5-column default (matches styles.css `.ledger-table__row`):
  // 96px date · 1fr desc · 168px cat · 168px amount · 36px chevron.
  if (totalColumns === 5 && hasPrimary) {
    return "96px 1fr 168px 168px 36px";
  }
  // Synthesize columns: first regular column gets a 96px slot for dates,
  // right-aligned columns get 168px, the primary desc takes 1fr, the
  // chevron takes 36px, and everything else stretches.
  const parts: string[] = [];
  let firstSeen = false;
  for (const col of regularColumns) {
    if (col.align === "right") {
      parts.push("168px");
    } else if (!firstSeen && hasPrimary) {
      parts.push("96px");
      firstSeen = true;
    } else {
      parts.push("120px");
    }
  }
  if (hasPrimary) parts.unshift("1fr");
  parts.push("36px");
  return parts.join(" ");
}
