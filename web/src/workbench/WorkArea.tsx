import { useWorkbench } from "./state";
import { EmptyState } from "../components/ui";
import type { EntryTab, ListTab, Tab } from "./types";
import { CashEntryList } from "../screens/list/CashEntryList";
import { CashEntryForm } from "../screens/entry/CashEntryForm";
import {
  TrialBalanceReport,
  ProfitLossReport,
  BalanceSheetReport,
  CashFlowReport,
} from "../screens/list/Reports";
import { DashboardScreen } from "../screens/workbench/DashboardScreen";
import {
  SalesInvoiceList,
  SalesReceiptList,
} from "../screens/list/Sales";
import {
  PurchaseInvoiceList,
  PurchasePaymentList,
} from "../screens/list/Purchases";
import {
  InventoryItemsList,
  StockMovementsList,
} from "../screens/list/Inventory";
import { AssetRegisterList } from "../screens/list/Assets";
import { MockEntryForm } from "../screens/entry/MockEntryForm";
import { defaultEntryTitle } from "./modules";

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
  switch (tab.kind) {
    case "dashboard":
      return <DashboardScreen />;
    case "list":
      return <ListTabContent tab={tab} />;
    case "entry":
      return <EntryTabContent tab={tab} />;
  }
}

function ListTabContent({ tab }: { tab: ListTab }) {
  const subKind = tab.subKind;
  switch (subKind) {
    case "cash-other-receipt":
      return (
        <CashEntryList
          listKind={subKind}
          title="Other Receipt"
          description="Money received from sources other than sales (capital, loans, other income)."
          entrySubKind="money-in"
          fixedKind="money-in"
        />
      );
    case "cash-other-payment":
      return (
        <CashEntryList
          listKind={subKind}
          title="Other Payment"
          description="Money paid out for expenses, assets, or settlements other than purchases."
          entrySubKind="money-out"
          fixedKind="money-out"
        />
      );
    case "cash-transfer":
      return (
        <CashEntryList
          listKind={subKind}
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
    case "report-trial-balance":
      return <TrialBalanceReport />;
    case "report-profit-loss":
      return <ProfitLossReport />;
    case "report-balance-sheet":
      return <BalanceSheetReport />;
    case "report-cash-flow":
      return <CashFlowReport />;
    default:
      return <PlaceholderTab title={tab.title} sub={`list · ${subKind}`} />;
  }
}

function EntryTabContent({ tab }: { tab: EntryTab }) {
  const subKind = tab.subKind;
  switch (subKind) {
    case "money-in":
    case "money-out":
    case "cash-transfer":
      return (
        <CashEntryForm
          tabId={tab.id}
          subKind={subKind}
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
          subKind={subKind}
          title={defaultEntryTitle(subKind)}
          initialTitle={tab.title}
        />
      );
    default:
      return <PlaceholderTab title={tab.title} sub={`entry · ${subKind}`} />;
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
        This tab will be wired in the next step of the workbench rebuild.
      </p>
    </div>
  );
}
