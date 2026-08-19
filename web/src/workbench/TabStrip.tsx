import { useWorkbench } from "./state";
import { findModule } from "./modules";
import { Tabs } from "../components/m3/Tabs";
import type { Tab } from "./types";

/**
 * Top-level browser-style tab strip below the top bar, rendered with M3
 * `md-tabs` + `md-primary-tab`. Shows only the top-level tabs: the
 * Dashboard (pinned, no close button) and module parents. A module
 * parent's children are rendered by NestedTabStrip inside the work area.
 */
export function TabStrip() {
  const workbench = useWorkbench();
  const tabs = workbench.tabs;

  if (tabs.length === 0) return null;

  const activeIndex = tabs.findIndex((tab) => tab.id === workbench.activeId);

  const handleChange = (e: Event) => {
    const index = (e.target as HTMLElement & { activeTabIndex: number }).activeTabIndex;
    const tab = tabs[index];
    if (tab) workbench.activate(tab.id);
  };

  return (
    <nav className="tabstrip" aria-label="Open modules">
      <Tabs
        className="tabstrip__inner"
        activeTabIndex={activeIndex >= 0 ? activeIndex : 0}
        onChange={handleChange}
      >
        {tabs.map((tab) => (
          <TabPill key={tab.id} tab={tab} />
        ))}
      </Tabs>
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
    <md-primary-tab active={isActive} className={`tabpill${isActive ? " is-active" : ""}`}>
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
          ×
        </button>
      ) : null}
    </md-primary-tab>
  );
}
