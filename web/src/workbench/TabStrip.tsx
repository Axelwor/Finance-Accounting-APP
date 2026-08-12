import { useWorkbench } from "./state";
import { findModule } from "./modules";
import type { Tab } from "./types";

/**
 * Top-level browser-style tab strip below the top bar. Shows only the
 * top-level tabs: the Dashboard (pinned, no close button) and module
 * parents. A module parent's children are rendered by NestedTabStrip
 * inside the work area.
 */
export function TabStrip() {
  const workbench = useWorkbench();
  const tabs = workbench.tabs;

  if (tabs.length === 0) return null;

  const activeIndex = tabs.findIndex((tab) => tab.id === workbench.activeId);

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
      document
        .querySelector(`[data-tab-index="${newIndex}"]`)
        ?.focus();
    }
  };

  return (
    <nav className="tabstrip" aria-label="Open modules">
      <div className="tabstrip__inner" role="tablist" aria-orientation="horizontal">
        {tabs.map((tab, index) => (
          <TabPill key={tab.id} tab={tab} tabIndex={index} onFocus={() => handleKeyDown({ key: "" as keyof KeyboardEvent, preventDefault: () => {} } as React.KeyboardEvent), index} onKeyDown={(e) => handleKeyDown(e, index)} />
        ))}
      </div>
    </nav>
  );
}

function TabPill({ tab, tabIndex, onFocus, onKeyDown }: { tab: Tab; tabIndex: number; onFocus?: (idx: number) => void; onKeyDown?: (e: React.KeyboardEvent) => void }) {
  const workbench = useWorkbench();
  const isActive = tab.id === workbench.activeId;
  const isDashboard = tab.kind === "dashboard";
  const label = isDashboard ? "Dashboard" : tab.title || (findModule(tab.moduleId)?.label ?? tab.moduleId);

  return (
    <div
      className={`tabpill${isActive ? " is-active" : ""}`}
      role="tab"
      aria-selected={isActive}
      tabIndex={isActive ? 0 : -1}
      data-tab-index={tabIndex}
      onClick={() => workbench.activate(tab.id)}
      onFocus={onFocus}
      onKeyDown={onKeyDown}
    >
      <span className={`tabpill__kind tabpill__kind--${isDashboard ? "home" : "module"}`}>
        {isDashboard ? "HOME" : "MENU"}
      </span>
      <span className="tabpill__title" title={label}>{label}</span>
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
          ×
        </button>
      ) : null}
    </div>
  );
}
