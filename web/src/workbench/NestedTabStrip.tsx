import { useWorkbench } from "./state";
import type { NestedTab } from "./types";

/**
 * Sub-tab strip rendered inside the work area when the active top-level
 * tab is a module parent. Lists the module's open children (list views
 * and entry forms) so the user can switch between them without leaving
 * the module.
 *
 * Accurate-style layout: the list tab is an icon-only button (green when
 * active); entry tabs show their title with a close button; the + button
 * adds a new entry draft.
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
  const sortedChildren = [...children].sort((a, b) => {
    if (a.kind === "list" && b.kind !== "list") return -1;
    if (a.kind !== "list" && b.kind === "list") return 1;
    return a.createdAt - b.createdAt;
  });

  if (sortedChildren.length === 0 && !onAdd) return null;
  void parentId;

  const activeIndex = sortedChildren.findIndex((c) => c.id === activeChildId);

  const handleKeyDown = (event: React.KeyboardEvent, index: number) => {
    let newIndex = index;

    switch (event.key) {
      case "ArrowLeft":
      case "ArrowUp":
        newIndex = index > 0 ? index - 1 : sortedChildren.length - 1;
        event.preventDefault();
        break;
      case "ArrowRight":
      case "ArrowDown":
        newIndex = index < sortedChildren.length - 1 ? index + 1 : 0;
        event.preventDefault();
        break;
      case "Home":
        newIndex = 0;
        event.preventDefault();
        break;
      case "End":
        newIndex = sortedChildren.length - 1;
        event.preventDefault();
        break;
      case "Enter":
      case " ":
        workbench.activate(sortedChildren[index].id);
        event.preventDefault();
        return;
      default:
        return;
    }

    if (newIndex !== index) {
      workbench.activate(sortedChildren[newIndex].id);
      (document.querySelector(`[data-nested-tab-index="${newIndex}"]`) as HTMLElement)?.focus?.();
    }
  };

  return (
    <nav className="nested-tabstrip" aria-label="Open items in this module">
      <div className="nested-tabstrip__inner" role="tablist" aria-orientation="horizontal">
        {sortedChildren.map((child, index) => (
          <NestedTabPill
            key={child.id}
            tab={child}
            isActive={child.id === activeChildId}
            tabIndex={activeIndex === index ? 0 : -1}
            onKeyDown={(e) => handleKeyDown(e, index)}
          />
        ))}
        {onAdd ? (
          <button
            type="button"
            className="nested-tabstrip__add"
            onClick={onAdd}
            aria-label={addLabel ?? "Add entry"}
            title={addLabel ?? "New entry"}
          >
            +
          </button>
        ) : null}
      </div>
    </nav>
  );
}

function NestedTabPill({
  tab,
  isActive,
  tabIndex,
  onKeyDown,
}: {
  tab: NestedTab;
  isActive: boolean;
  tabIndex?: number;
  onKeyDown?: (e: React.KeyboardEvent) => void;
}) {
  const workbench = useWorkbench();

  // List tabs: icon-only, no text, no close button.
  if (tab.kind === "list") {
    return (
      <button
        type="button"
        className={`nested-icon-tab${isActive ? " is-active" : ""}`}
        role="tab"
        aria-selected={isActive}
        tabIndex={tabIndex}
        data-nested-tab-index={tabIndex ?? -1}
        title={tab.title}
        onClick={() => workbench.activate(tab.id)}
        onKeyDown={onKeyDown}
      >
        <ListIcon />
      </button>
    );
  }

  // Entry tabs: title + close button.
  return (
    <div
      className={`nested-tabpill${isActive ? " is-active" : ""}${tab.unsaved ? " is-unsaved" : ""}`}
      role="tab"
      aria-selected={isActive}
      tabIndex={tabIndex}
      data-nested-tab-index={tabIndex ?? -1}
      onClick={() => workbench.activate(tab.id)}
      onKeyDown={onKeyDown}
    >
      <span className="nested-tabpill__title" title={tab.title}>{tab.title}</span>
      {tab.unsaved ? <span className="nested-tabpill__dot" aria-hidden="true" /> : null}
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
    </div>
  );
}

function ListIcon() {
  return (
    <svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true">
      <path d="M4 6h16M4 12h16M4 18h16" stroke="currentColor" strokeWidth="2" strokeLinecap="round" fill="none" />
    </svg>
  );
}
