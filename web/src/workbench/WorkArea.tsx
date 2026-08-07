import { useWorkbench } from "./state";
import { EmptyState } from "../components/ui";
import type { Tab } from "./types";
import { CashEntryList } from "../screens/list/CashEntryList";
import { CashEntryForm } from "../screens/entry/CashEntryForm";
import { TrialBalanceReport, ProfitLossReport, BalanceSheetReport, CashFlowReport } from "../screens/list/Reports";
import { SalesInvoiceList, SalesReceiptList } from "../screens/list/Sales";
import { PurchaseInvoiceList, PurchasePaymentList } from "../screens/list/Purchases";
import { InventoryItemsList, StockMovementsList } from "../screens/list/Inventory";
import { AssetRegisterList } from "../screens/list/Assets";
import { MockEntryForm } from "../screens/entry/MockEntryForm";
import { defaultEntryTitle } from "./modules";

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
      case "sales-invoice":
        return <SalesInvoiceList />;
      case "sales-receipt":
        return <SalesReceiptList />;
      case "purchase-invoice":
        return <PurchaseInvoiceList />;
      case "purchase-payment":
        return <PurchasePaymentList />;
      case "inventory-items":
        return <InventoryItemsList />;
      case "stock-movements":
        return <StockMovementsList />;
      case "asset-register":
        return <AssetRegisterList />;
      // Reports (read-only) — kept from step 4.
      case "report-trial-balance":
        return <TrialBalanceReport />;
      case "report-profit-loss":
        return <ProfitLossReport />;
      case "report-balance-sheet":
        return <BalanceSheetReport />;
      case "report-cash-flow":
        return <CashFlowReport />;
      default: {
        // Exhaustiveness — should be unreachable once all sub-kinds are
        // wired. Cast keeps a friendly fallback while satisfying TS.
        const fallback = tab as Tab;
        return <PlaceholderTab title={fallback.title} sub={`list · ${fallback.subKind}`} />;
      }
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
    case "sales-invoice":
    case "sales-receipt":
    case "purchase-invoice":
    case "purchase-payment":
    case "inventory-item":
    case "asset-register":
      return (
        <MockEntryForm
          tabId={tab.id}
          subKind={tab.subKind}
          title={defaultEntryTitle(tab.subKind)}
          initialTitle={tab.title}
        />
      );
    default: {
      const fallback = tab as Tab;
      return <PlaceholderTab title={fallback.title} sub={`entry · ${fallback.subKind}`} />;
    }
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
