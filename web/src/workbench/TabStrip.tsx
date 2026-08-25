import { useWorkbench } from "./state";
import { findModule } from "./modules";
import { Icon } from "../components/m3/Icon";
import type { Tab } from "./types";

/**
 * Top-level browser-style tab strip below the top bar.
 * Provides intuitive tab switching with unsaved changes indicators (dot)
 * and responsive close buttons. Roving tabindex keyboard support mirrors
 * NestedTabStrip: Arrow keys / Home / End move, Enter or Space activates.
 */
export function TabStrip() {
  const workbench = useWorkbench();
  const tabs = workbench.tabs;
  const activeIndex = tabs.findIndex((t) => t.id === workbench.activeId);

  if (tabs.length === 0) return null;

  const handleKeyDown = (event: React.KeyboardEvent, index: number) => {
    let newIndex = index;

    switch (event.key) {
      case "ArrowLeft":
      case "ArrowUp":
        newIndex = index > 0 ? index - 1 : tabs.length - 1;
        event.preventDefault();
        break;
      case "ArrowRight":
      case "ArrowDown":
        newIndex = index < tabs.length - 1 ? index + 1 : 0;
        event.preventDefault();
        break;
      case "Home":
        newIndex = 0;
        event.preventDefault();
        break;
      case "End":
        newIndex = tabs.length - 1;
        event.preventDefault();
        break;
      case "Enter":
      case " ":
        workbench.activate(tabs[index].id);
        event.preventDefault();
        return;
      default:
        return;
    }

    if (newIndex !== index) {
      workbench.activate(tabs[newIndex].id);
      (document.querySelector(`[data-tab-index="${newIndex}"]`) as HTMLElement)?.focus?.();
    }
  };

  return (
    <nav className="tabstrip" aria-label="Open modules">
      <div className="tabstrip__inner" role="tablist" aria-orientation="horizontal">
        {tabs.map((tab, index) => (
          <TabPill
            key={tab.id}
            tab={tab}
            tabIndex={activeIndex === index ? 0 : -1}
            dataIndex={index}
            onKeyDown={(e) => handleKeyDown(e, index)}
          />
        ))}
      </div>
    </nav>
  );
}

function TabPill({
  tab,
  tabIndex,
  dataIndex,
  onKeyDown,
}: {
  tab: Tab;
  tabIndex?: number;
  dataIndex?: number;
  onKeyDown?: (e: React.KeyboardEvent) => void;
}) {
  const workbench = useWorkbench();
  const isActive = tab.id === workbench.activeId;
  const isDashboard = tab.kind === "dashboard";
  const label = isDashboard
    ? "Dashboard"
    : tab.title || (findModule(tab.moduleId)?.label ?? tab.moduleId);

  return (
    <div
      role="tab"
      aria-selected={isActive}
      tabIndex={tabIndex}
      data-tab-index={dataIndex}
      className={`tabpill${isActive ? " is-active" : ""}`}
      onClick={() => workbench.activate(tab.id)}
      onKeyDown={onKeyDown}
    >
      <span className="tabpill__icon">
        <Icon
          name={isDashboard ? "table_chart" : "folder"}
          size={14}
          className={isActive ? "text-brand" : "text-muted"}
        />
      </span>
      <span className="tabpill__title" title={label}>
        {label}
      </span>
      {!isDashboard ? (
        <button
          type="button"
          className="tabpill__close"
          aria-label={`Close ${label}`}
          onClick={(e) => {
            e.stopPropagation();
            workbench.close(tab.id);
          }}
        >
          <Icon name="close" size={12} />
        </button>
      ) : null}
    </div>
  );
}
