import { useState, useMemo, type ReactNode } from "react";
import { Icon } from "./m3/Icon";

export interface Column<T> {
  key: string;
  header: string;
  render?: (row: T) => ReactNode;
  align?: "left" | "right" | "center";
  sortable?: boolean;
  width?: string | number;
}

export interface DataTableProps<T> {
  title?: string;
  subtitle?: string;
  columns: Column<T>[];
  data: T[];
  loading?: boolean;
  searchPlaceholder?: string;
  searchFilter?: (row: T, query: string) => boolean;
  onAdd?: () => void;
  addLabel?: string;
  onRowClick?: (row: T) => void;
  footerSummary?: ReactNode;
}

export function DataTable<T extends { id?: string | number }>({
  title,
  subtitle,
  columns,
  data,
  loading,
  searchPlaceholder = "Cari data...",
  searchFilter,
  onAdd,
  addLabel = "+ Tambah Data",
  onRowClick,
  footerSummary,
}: DataTableProps<T>) {
  const [search, setSearch] = useState("");
  const [sortKey, setSortKey] = useState<string | null>(null);
  const [sortDir, setSortDir] = useState<"asc" | "desc">("asc");

  const handleSort = (key: string) => {
    if (sortKey === key) {
      setSortDir((prev) => (prev === "asc" ? "desc" : "asc"));
    } else {
      setSortKey(key);
      setSortDir("asc");
    }
  };

  const filteredData = useMemo(() => {
    let result = data;
    if (search.trim() && searchFilter) {
      result = result.filter((row) => searchFilter(row, search.toLowerCase().trim()));
    }
    if (sortKey) {
      result = [...result].sort((a: any, b: any) => {
        const valA = a[sortKey];
        const valB = b[sortKey];
        if (typeof valA === "number" && typeof valB === "number") {
          return sortDir === "asc" ? valA - valB : valB - valA;
        }
        return sortDir === "asc"
          ? String(valA ?? "").localeCompare(String(valB ?? ""))
          : String(valB ?? "").localeCompare(String(valA ?? ""));
      });
    }
    return result;
  }, [data, search, searchFilter, sortKey, sortDir]);

  return (
    <div className="datatable-wrapper">
      <div className="datatable-toolbar">
        <div>
          {title && <h2 className="text-sm font-bold text-primary">{title}</h2>}
          {subtitle && <p className="text-xs text-muted">{subtitle}</p>}
        </div>

        <div className="flex items-center gap-3">
          <div className="auth-input-box" style={{ height: "32px", width: "240px" }}>
            <Icon name="search" size={14} className="auth-input-icon" />
            <input
              type="text"
              placeholder={searchPlaceholder}
              aria-label={searchPlaceholder}
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              style={{ fontSize: "12px" }}
            />
          </div>

          {onAdd && (
            <button
              type="button"
              className="btn-dash-primary text-xs"
              onClick={onAdd}
            >
              <Icon name="plus" size={14} />
              <span>{addLabel}</span>
            </button>
          )}
        </div>
      </div>

      <div style={{ overflowX: "auto" }}>
        <table className="datatable">
          {title && (
            <caption
              style={{
                position: "absolute",
                width: 1,
                height: 1,
                padding: 0,
                margin: -1,
                overflow: "hidden",
                clip: "rect(0, 0, 0, 0)",
                whiteSpace: "nowrap",
                border: 0,
              }}
            >
              {title}
            </caption>
          )}
          <thead>
            <tr>
              {columns.map((col) => {
                const isSorted = sortKey === col.key;
                const ariaSort = !col.sortable
                  ? undefined
                  : isSorted
                    ? sortDir === "asc"
                      ? "ascending"
                      : "descending"
                    : "none";
                return (
                  <th
                    key={col.key}
                    scope="col"
                    style={{ width: col.width }}
                    className={col.align === "right" ? "num" : col.align === "center" ? "text-center" : ""}
                    aria-sort={ariaSort}
                  >
                    {col.sortable ? (
                      <button
                        type="button"
                        onClick={() => handleSort(col.key)}
                        className={`flex items-center gap-1.5 ${
                          col.align === "right" ? "justify-end" : col.align === "center" ? "justify-center" : ""
                        } cursor-pointer select-none hover:text-primary`}
                        style={{
                          background: "none",
                          border: "none",
                          padding: 0,
                          font: "inherit",
                          color: "inherit",
                          width: "100%",
                        }}
                        title={`Sort by ${col.header}`}
                      >
                        <span>{col.header}</span>
                        {isSorted && (
                          <Icon
                            name={sortDir === "asc" ? "chevron_up" : "chevron_down"}
                            size={12}
                            className="text-brand"
                          />
                        )}
                      </button>
                    ) : (
                      <div
                        className={`flex items-center gap-1.5 ${
                          col.align === "right" ? "justify-end" : col.align === "center" ? "justify-center" : ""
                        }`}
                      >
                        <span>{col.header}</span>
                      </div>
                    )}
                  </th>
                );
              })}
            </tr>
          </thead>
          <tbody>
            {loading ? (
              <tr>
                <td colSpan={columns.length} className="text-center py-8 text-muted">
                  Memuat data...
                </td>
              </tr>
            ) : filteredData.length === 0 ? (
              <tr>
                <td colSpan={columns.length} className="text-center py-8 text-muted">
                  {search ? "Tidak ada data yang sesuai dengan pencarian." : "Belum ada rekaman data."}
                </td>
              </tr>
            ) : (
              filteredData.map((row, idx) => (
                <tr
                  key={row.id ?? idx}
                  className={onRowClick ? "cursor-pointer" : ""}
                  onClick={() => onRowClick && onRowClick(row)}
                >
                  {columns.map((col) => (
                    <td
                      key={col.key}
                      className={col.align === "right" ? "num" : col.align === "center" ? "text-center" : ""}
                    >
                      {col.render ? col.render(row) : (row as any)[col.key]}
                    </td>
                  ))}
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      <div className="datatable-footer">
        <div>
          Total: <strong className="font-mono">{filteredData.length}</strong> baris data
        </div>
        {footerSummary && <div>{footerSummary}</div>}
      </div>
    </div>
  );
}
