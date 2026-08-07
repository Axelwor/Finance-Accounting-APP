import { useWorkbench } from "./state";
import type { NestedTab } from "./types";

/**
 * Sub-tab strip rendered inside the work area when the active top-level
 * tab is a module parent. Lists the module's open children (list views
 * and entry forms) so the user can switch between them without leaving
 * the module.
 *
 * Plus, an entry form's "Add" button can request a new draft by clicking
 * the sub-strip's `+` affordance (handled by the parent screen).
 */
export function NestedTabStrip({
  parentId,
  children,
  activeChildId,
  onAdd,
  addLabel,
}: {
  parentId: string;
  children: NestedTab[];
  activeChildId: string | null;
  onAdd?: () => void;
  addLabel?: string;
}) {
  const workbench = useWorkbench();

  if (children.length === 0 && !onAdd) return null;

  return (
    <nav className="nested-tabstrip" aria-label="Open items in this module">
      <div className="nested-tabstrip__inner">
        {children.map((child) => (
          <NestedTabPill
            key={child.id}
            parentId={parentId}
            tab={child}
            isActive={child.id === activeChildId}
          />
        ))}
        {onAdd ? (
          <button type="button" className="nested-tabstrip__add" onClick={onAdd} aria-label={addLabel ?? "Add entry"}>
            + {addLabel ?? "New entry"}
          </button>
        ) : null}
      </div>
    </nav>
  );
}

function NestedTabPill({
  parentId,
  tab,
  isActive,
}: {
  parentId: string;
  tab: NestedTab;
  isActive: boolean;
}) {
  const workbench = useWorkbench();
  const kindLabel = tab.kind === "list" ? "LIST" : tab.draft ? "NEW" : "ENTRY";
  const status =
    tab.kind === "list"
      ? "OPEN"
      : tab.status ?? (tab.draft ? "EDIT" : "POSTED");

  return (
    <div
      className={`nested-tabpill${isActive ? " is-active" : ""}${tab.unsaved ? " is-unsaved" : ""}`}
      role="tab"
      aria-selected={isActive}
      onClick={() => workbench.activate(tab.id)}
    >
      <span className={`nested-tabpill__kind nested-tabpill__kind--${tab.kind === "list" ? "list" : tab.draft ? "draft" : "entry"}`}>
        {kindLabel}
      </span>
      <span className="nested-tabpill__title" title={tab.title}>{tab.title}</span>
      <span className="nested-tabpill__status">{status}</span>
      {tab.kind === "entry" ? (
        <button
          type="button"
          className="nested-tabpill__close"
          aria-label={`Close ${tab.title}`}
          onClick={(e) => {
            e.stopPropagation();
            workbench.close(tab.id);
          }}
        >
          ×
        </button>
      ) : null}
    </div>
  );
}
