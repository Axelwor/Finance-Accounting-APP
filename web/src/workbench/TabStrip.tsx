import { useWorkbench } from "./state";
import { findModule } from "./modules";
import { Icon } from "../components/m3/Icon";
import type { Tab } from "./types";

/**
 * Top-level browser-style tab strip below the top bar.
 * Provides intuitive tab switching with unsaved changes indicators (dot)
 * and responsive close buttons.
 */
export function TabStrip() {
  const workbench = useWorkbench();
  const tabs = workbench.tabs;

  if (tabs.length === 0) return null;

  return (
    <nav className="tabstrip" aria-label="Open modules">
      <div className="tabstrip__inner" role="tablist">
        {tabs.map((tab) => (
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
  const label = isDashboard
    ? "Dashboard"
    : tab.title || (findModule(tab.moduleId)?.label ?? tab.moduleId);

  return (
    <div
      role="tab"
      aria-selected={isActive}
      className={`tabpill${isActive ? " is-active" : ""}`}
      onClick={() => workbench.activate(tab.id)}
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
