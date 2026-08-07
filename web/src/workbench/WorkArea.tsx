import { useWorkbench } from "./state";
import { EmptyState } from "../components/ui";
import type { Tab } from "./types";
import { CashEntryList } from "../screens/list/CashEntryList";
import { CashEntryForm } from "../screens/entry/CashEntryForm";

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
    switch (tab.subKind) {
      case "cash-other-receipt":
        return (
          <CashEntryList
            listKind={tab.subKind}
            title="Other Receipt"
            description="Money received from sources other than sales (capital, loans, other income)."
            entrySubKind="money-in"
            fixedKind="money-in"
          />
        );
      case "cash-other-payment":
        return (
          <CashEntryList
            listKind={tab.subKind}
            title="Other Payment"
            description="Money paid out for expenses, assets, or settlements other than purchases."
            entrySubKind="money-out"
            fixedKind="money-out"
          />
        );
      case "cash-transfer":
        return (
          <CashEntryList
            listKind={tab.subKind}
            title="Bank Transfer"
            description="Move money between cash and bank accounts."
            entrySubKind="cash-transfer"
            fixedKind="transfer"
          />
        );
      // Reports (read-only) and mocked modules get placeholder for now.
      default:
        return <PlaceholderTab title={tab.title} sub={`list · ${tab.subKind}`} />;
    }
  }

  // Entry tabs
  switch (tab.subKind) {
    case "money-in":
    case "money-out":
    case "cash-transfer":
      return (
        <CashEntryForm
          tabId={tab.id}
          subKind={tab.subKind}
          entryId={tab.entryId}
          initialTitle={tab.title}
        />
      );
    default:
      return <PlaceholderTab title={tab.title} sub={`entry · ${tab.subKind}`} />;
  }
}

function PlaceholderTab({ title, sub }: { title: string; sub: string }) {
  return (
    <div className="tab-placeholder">
      <p className="tab-placeholder__title">{title}</p>
      <p className="tab-placeholder__sub">
        Coming next &middot; <code>{sub}</code>
      </p>
      <p className="tab-placeholder__hint">
        This tab will be wired in the next step of the workbench rebuild (reports in step 4, sales/purchases/inventory/fixed-assets in step 5).
      </p>
    </div>
  );
}
