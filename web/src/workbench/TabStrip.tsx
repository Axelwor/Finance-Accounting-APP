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

  if (workbench.tabs.length === 0) return null;

  return (
    <nav className="tabstrip" aria-label="Open modules">
      <div className="tabstrip__inner">
        {workbench.tabs.map((tab) => (
          <TabPill key={tab.id} tab={tab} />
        ))}
      </div>
    </nav>
  );
}

function TabPill({ tab }: { tab: Tab }) {
  const workbench = useWorkbench();
  const isActive = tab.id === workbench.activeId;
  const isDashboard = tab.kind === "dashboard";
  const label = isDashboard ? "Dashboard" : findModule(tab.moduleId)?.label ?? tab.title;

  return (
    <div
      className={`tabpill${isActive ? " is-active" : ""}`}
      role="tab"
      aria-selected={isActive}
      onClick={() => workbench.activate(tab.id)}
    >
      <span className={`tabpill__kind tabpill__kind--${isDashboard ? "home" : "module"}`}>
        {isDashboard ? "HOME" : "MODULE"}
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
