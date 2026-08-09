import { useWorkbench } from "./state";
import { EmptyState } from "../components/ui";
import { NestedTabStrip } from "./NestedTabStrip";
import type { EntryTab, ListTab, ModuleTab, NestedTab } from "./types";
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
  SalesReceiptList,
  QuotationList,
} from "../screens/list/Sales";
import { SalesOrderList } from "../screens/list/SalesOrderList";
import { DeliveryOrderList } from "../screens/list/DeliveryOrderList";
import { InvoiceList } from "../screens/list/InvoiceList";
import { CreditNoteList } from "../screens/list/CreditNoteList";
import { PurchaseOrderList } from "../screens/list/PurchaseOrderList";
import { GRNList } from "../screens/list/GRNList";
import { PurchaseSupplierList } from "../screens/list/PurchaseSupplierList";
import { PurchaseReturnList } from "../screens/list/PurchaseReturnList";
import {
  PurchaseInvoiceList,
  PurchasePaymentList,
} from "../screens/list/Purchases";
import { SupplierInvoiceList } from "../screens/list/SupplierInvoiceList";
import {
  InventoryItemsList,
  StockMovementsList,
} from "../screens/list/Inventory";
import { StockOpnameList } from "../screens/list/StockOpnameList";
import { StockTransferList } from "../screens/list/StockTransferList";
import { AssetRegisterList } from "../screens/list/Assets";
import { MockEntryForm } from "../screens/entry/MockEntryForm";
import { QuotationForm } from "../screens/entry/QuotationForm";
import { SalesOrderForm } from "../screens/entry/SalesOrderForm";
import { DeliveryOrderForm } from "../screens/entry/DeliveryOrderForm";
import { InvoiceForm } from "../screens/entry/InvoiceForm";
import { CreditNoteForm } from "../screens/entry/CreditNoteForm";
import { PurchaseOrderForm } from "../screens/entry/PurchaseOrderForm";
import { GRNForm } from "../screens/entry/GRNForm";
import { PurchaseSupplierForm } from "../screens/entry/PurchaseSupplierForm";
import { SupplierInvoiceForm } from "../screens/entry/SupplierInvoiceForm";
import { PurchaseReturnForm } from "../screens/entry/PurchaseReturnForm";
import { StockOpnameForm } from "../screens/entry/StockOpnameForm";
import { StockTransferForm } from "../screens/entry/StockTransferForm";
import { defaultEntryTitle, findModule } from "./modules";

export function WorkArea() {
  const workbench = useWorkbench();
  const activeTab = workbench.activeTab;

  if (!activeTab) {
    return (
      <div className="workarea workarea--empty">
        <EmptyState
          title="No tab open"
          message="Pick a module from the sidebar to open its history, or use the +New entry button on any list to rule a new entry."
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

function TabContent({ tab }: { tab: { id: string; kind: string } }) {
  const workbench = useWorkbench();
  const active = workbench.tabs.find((t) => t.id === tab.id);
  if (!active) return null;
  if (active.kind === "dashboard") return <DashboardScreen />;
  if (active.kind === "module") return <ModuleContent parent={active} />;
  return null;
}

function ModuleContent({ parent }: { parent: ModuleTab }) {
  const workbench = useWorkbench();
  const children = workbench.nested[parent.id] ?? [];
  const activeChildId = workbench.activeChildFor(parent.id);
  const activeChild = activeChildId ? children.find((c) => c.id === activeChildId) ?? null : null;

  const module = findModule(parent.moduleId);
  const defaultAddLabel = module?.items[0]?.openEntry ? `New ${defaultEntryTitle(module.items[0].openEntry).toLowerCase()}` : "New entry";

  // If the module has no children yet, auto-open the first list view so the
  // work area is never empty.
  if (children.length === 0 && module && module.items[0]) {
    queueMicrotask(() => workbench.openList(module.items[0].openList));
    return (
      <div className="workarea__placeholder">
        <EmptyState
          title={`No items open in ${module.label}`}
          message={`Open a history tab from the sidebar to see what's there.`}
        />
      </div>
    );
  }

  return (
    <div className="module-content">
      <NestedTabStrip
        parentId={parent.id}
        children={children}
        activeChildId={activeChildId}
        onAdd={
          module?.items[0]?.openEntry
            ? () => workbench.openEntryDraft(module.items[0].openEntry!)
            : undefined
        }
        addLabel={defaultAddLabel}
      />
      <div className="module-content__body" key={activeChild?.id ?? "empty"}>
        {activeChild ? <NestedContent tab={activeChild} /> : (
          <div className="workarea__placeholder">
            <EmptyState
              title="No item selected"
              message="Pick a tab from the sub-strip above, or use the +New entry button to rule a new transaction."
            />
          </div>
        )}
      </div>
    </div>
  );
}

function NestedContent({ tab }: { tab: NestedTab }) {
  if (tab.kind === "list") return <ListTabContent tab={tab} />;
  return <EntryTabContent tab={tab} />;
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
      return <InvoiceList />;
    case "sales-receipt":
      return <SalesReceiptList />;
    case "sales-quotation":
      return <QuotationList />;
    case "sales-order":
      return <SalesOrderList />;
    case "delivery-order":
      return <DeliveryOrderList />;
    case "credit-note":
      return <CreditNoteList />;
    case "purchase-order":
      return <PurchaseOrderList />;
    case "grn":
      return <GRNList />;
    case "purchase-supplier":
      return <PurchaseSupplierList />;
    case "supplier-invoice":
      return <SupplierInvoiceList />;
    case "purchase-return":
      return <PurchaseReturnList />;
    case "purchase-invoice":
      return <PurchaseInvoiceList />;
    case "purchase-payment":
      return <PurchasePaymentList />;
    case "inventory-items":
      return <InventoryItemsList />;
    case "stock-movements":
      return <StockMovementsList />;
    case "stock-opname":
      return <StockOpnameList />;
    case "stock-transfer":
      return <StockTransferList />;
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
      return <InvoiceForm tabId={tab.id} entryId={tab.entryId} initialTitle={tab.title} />;
    case "credit-note-entry":
      return <CreditNoteForm tabId={tab.id} entryId={tab.entryId} initialTitle={tab.title} />;
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
    case "sales-quotation-entry":
      return <QuotationForm tabId={tab.id} entryId={tab.entryId} initialTitle={tab.title} />;
    case "sales-order-entry":
      return <SalesOrderForm tabId={tab.id} entryId={tab.entryId} initialTitle={tab.title} />;
    case "delivery-order-entry":
      return <DeliveryOrderForm tabId={tab.id} entryId={tab.entryId} initialTitle={tab.title} />;
    case "purchase-order-entry":
      return <PurchaseOrderForm tabId={tab.id} entryId={tab.entryId} initialTitle={tab.title} />;
    case "grn-entry":
      return <GRNForm tabId={tab.id} entryId={tab.entryId} initialTitle={tab.title} />;
    case "supplier-invoice-entry":
      return <SupplierInvoiceForm tabId={tab.id} entryId={tab.entryId} initialTitle={tab.title} />;
    case "purchase-return-entry":
      return <PurchaseReturnForm tabId={tab.id} entryId={tab.entryId} initialTitle={tab.title} />;
    case "purchase-supplier-entry":
      return <PurchaseSupplierForm />;
    case "stock-opname-entry":
      return <StockOpnameForm tabId={tab.id} entryId={tab.entryId} initialTitle={tab.title} />;
    case "stock-transfer-entry":
      return <StockTransferForm tabId={tab.id} entryId={tab.entryId} initialTitle={tab.title} />;
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
