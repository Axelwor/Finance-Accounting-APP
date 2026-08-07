import { useWorkbench } from "./state";
import { EmptyState } from "../components/ui";
import type { Tab } from "./types";

/**
 * Work area: renders the active tab's content, or an empty state when
 * no tabs are open. Each tab kind (list | entry) dispatches to the right
 * screen component by sub-kind.
 */
export function WorkArea() {
  const workbench = useWorkbench();
  const activeTab = workbench.activeTab;

  if (!activeTab) {
    return (
      <div className="workarea workarea--empty">
        <EmptyState
          title="No tab open"
          message="Pick a module from the sidebar to open its history, or use the +Tambah button on any list to rule a new entry."
        />
      </div>
    );
  }

  return (
    <div className="workarea" key={activeTab.id}>
      <TabContent tab={activeTab} />
    </div>
  );
}

function TabContent({ tab }: { tab: Tab }) {
  if (tab.kind === "list") {
    // Lazy import would be nicer for bundle, but we ship all list screens inline
    // so a single switch keeps the dispatch explicit.
    switch (tab.subKind) {
      // Step 3 fills these in. For Step 1 we render a placeholder so the
      // shell can be visually verified end-to-end.
      default:
        return <PlaceholderTab title={tab.title} sub="list" />;
    }
  }
  return <PlaceholderTab title={tab.title} sub={tab.draft ? "entry-draft" : "entry"} />;
}

function PlaceholderTab({ title, sub }: { title: string; sub: string }) {
  return (
    <div className="tab-placeholder">
      <p className="tab-placeholder__title">{title}</p>
      <p className="tab-placeholder__sub">
        Step 1 scaffold · <code>{sub}</code> view
      </p>
      <p className="tab-placeholder__hint">
        The list/entry component for this tab will be wired in step 3 (Cash & Bank), step 4 (Reports), and step 5 (Sales, Purchases, Inventory, Fixed Assets).
      </p>
    </div>
  );
}
