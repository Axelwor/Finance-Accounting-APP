import { useWorkbench } from "./state";
import { Icon } from "../components/m3/Icon";
import type { NestedTab } from "./types";

/**
 * Sub-tab strip rendered inside the work area when the active top-level
 * tab is a module parent. Lists open children (list views & entry forms).
 */
export function NestedTabStrip({
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
            dataIndex={index}
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
            <Icon name="plus" size={14} />
            <span>{addLabel ?? "Baru"}</span>
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
  dataIndex,
  onKeyDown,
}: {
  tab: NestedTab;
  isActive: boolean;
  tabIndex?: number;
  dataIndex?: number;
  onKeyDown?: (e: React.KeyboardEvent) => void;
}) {
  const workbench = useWorkbench();

  // List tabs: icon-only + clean indicator.
  if (tab.kind === "list") {
    return (
      <button
        type="button"
        className={`nested-icon-tab${isActive ? " is-active" : ""}`}
        role="tab"
        aria-selected={isActive}
        tabIndex={tabIndex}
        data-nested-tab-index={dataIndex ?? -1}
        title={tab.title}
        onClick={() => workbench.activate(tab.id)}
        onKeyDown={onKeyDown}
      >
        <Icon name="table_chart" size={14} />
        <span className="nested-icon-tab__label">{tab.title}</span>
      </button>
    );
  }

  // Entry tabs: title + dirty state dot + close button.
  return (
    <div
      className={`nested-tabpill${isActive ? " is-active" : ""}${tab.unsaved ? " is-unsaved" : ""}`}
      role="tab"
      aria-selected={isActive}
      tabIndex={tabIndex}
      data-nested-tab-index={dataIndex ?? -1}
      onClick={() => workbench.activate(tab.id)}
      onKeyDown={onKeyDown}
    >
      <Icon name="description" size={13} className={isActive ? "text-brand" : "text-muted"} />
      <span className="nested-tabpill__title" title={tab.title}>{tab.title}</span>
      {tab.unsaved ? (
        <span className="nested-tabpill__dot" title="Perubahan belum disimpan" aria-hidden="true" />
      ) : null}
      <button
        type="button"
        className="nested-tabpill__close"
        aria-label={`Close ${tab.title}`}
        onClick={(e) => {
          e.stopPropagation();
          workbench.close(tab.id);
        }}
      >
        <Icon name="close" size={11} />
      </button>
    </div>
  );
}
