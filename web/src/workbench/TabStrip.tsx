import { useWorkbench } from "./state";
import { findSubItemByList } from "./modules";
import type { Tab } from "./types";

/**
 * Browser-style tab strip below the top bar. Tabs are clickable,
 * closeable, and the active tab carries a ledger-green left rule.
 */
export function TabStrip() {
  const workbench = useWorkbench();

  if (workbench.tabs.length === 0) return null;

  return (
    <nav className="tabstrip" aria-label="Open tabs">
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
  const listMeta = tab.kind === "list" ? findSubItemByList(tab.subKind) : undefined;
  const kind = tab.kind === "list" ? "LIST" : tab.draft ? "NEW" : "ENTRY";
  const status = tab.status ?? (tab.kind === "list" ? "OPEN" : "EDIT");

  return (
    <div
      className={`tabpill${isActive ? " is-active" : ""}${tab.unsaved ? " is-unsaved" : ""}`}
      role="tab"
      aria-selected={isActive}
      onClick={() => workbench.activate(tab.id)}
    >
      <span className={`tabpill__kind tabpill__kind--${tab.kind === "list" ? "list" : tab.draft ? "draft" : "entry"}`}>
        {kind}
      </span>
      <span className="tabpill__title" title={tab.title}>{tab.title}</span>
      <span className="tabpill__status">{status}</span>
      <button
        type="button"
        className="tabpill__close"
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
