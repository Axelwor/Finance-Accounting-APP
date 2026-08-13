import { type ReactNode } from "react";

export type SortDirection = "asc" | "desc";

export interface SortState {
  column: string;
  direction: SortDirection;
}

interface SortableHeaderProps {
  column: string;
  sort: SortState;
  onSort: (column: string) => void;
  className?: string;
  align?: "left" | "right";
  children: ReactNode;
}

export function SortableHeader({
  column,
  sort,
  onSort,
  className,
  align = "left",
  children,
}: SortableHeaderProps) {
  const isActive = sort.column === column;
  const arrow = isActive ? (sort.direction === "asc" ? "↑" : "↓") : "";
  const classes = [
    "sort-th",
    align === "right" ? "right" : "",
    isActive ? "is-active" : "",
    className ?? "",
  ]
    .filter(Boolean)
    .join(" ");
  const nextDirection: SortDirection = isActive && sort.direction === "asc" ? "desc" : "asc";
  return (
    <th
      scope="col"
      className={classes}
      aria-sort={isActive ? (sort.direction === "asc" ? "ascending" : "descending") : "none"}
    >
      <button
        type="button"
        className="sort-th__button"
        onClick={() => onSort(column)}
        title={isActive ? `Sorted ${sort.direction === "asc" ? "ascending" : "descending"} — click to ${nextDirection === "asc" ? "ascend" : "descend"}` : "Click to sort"}
      >
        <span className="sort-th__label">{children}</span>
        <span className="sort-th__arrow" aria-hidden="true">{arrow}</span>
      </button>
    </th>
  );
}
