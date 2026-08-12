import { useEffect, useRef } from "react";
import { useWorkbench } from "./state";
import { EmptyState } from "../components/ui";
import { TabErrorBoundary } from "../App";
import { useKeyboardShortcuts, CLOSE_TAB_EVENT } from "../hooks/useKeyboardShortcuts";
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
import { DueDateReminders } from "../screens/list/DueDateReminders";
import { CustomerStatementScreen } from "../screens/list/CustomerStatement";
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
import { CustomerList } from "../screens/list/CustomerList";
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
import { BOMList } from "../screens/list/BOMList";
import { ProductionJobList } from "../screens/list/ProductionJobList";
import { AssetRegisterList } from "../screens/list/Assets";
import { FixedAssetList } from "../screens/list/FixedAssetList";
import { JournalEntryList } from "../screens/list/JournalEntryList";
import { GeneralLedger } from "../screens/list/GeneralLedger";
import { JournalRegister } from "../screens/list/JournalRegister";
import { AuditLogList } from "../screens/list/AuditLogList";
import { ReportTemplateList } from "../screens/list/ReportTemplateList";
import { FinancialNotesList } from "../screens/list/FinancialNotesList";
import { ARAgingList } from "../screens/list/ARAgingList";
import { APAgingList } from "../screens/list/APAgingList";
import { JournalEntryForm } from "../screens/entry/JournalEntryForm";
import { DemoEntryForm } from "../screens/entry/DemoEntryForm";
import { ChequeList } from "../screens/list/ChequeList";
import { ChequeForm } from "../screens/entry/ChequeForm";
import { QuotationForm } from "../screens/entry/QuotationForm";
import { SalesOrderForm } from "../screens/entry/SalesOrderForm";
import { DeliveryOrderForm } from "../screens/entry/DeliveryOrderForm";
import { InvoiceForm } from "../screens/entry/InvoiceForm";
import { CreditNoteForm } from "../screens/entry/CreditNoteForm";
import { PurchaseOrderForm } from "../screens/entry/PurchaseOrderForm";
import { GRNForm } from "../screens/entry/GRNForm";
import { PurchaseSupplierForm } from "../screens/entry/PurchaseSupplierForm";
import { CustomerForm } from "../screens/entry/CustomerForm";
import { SupplierInvoiceForm } from "../screens/entry/SupplierInvoiceForm";
import { PurchaseReturnForm } from "../screens/entry/PurchaseReturnForm";
import { StockOpnameForm } from "../screens/entry/StockOpnameForm";
import { StockTransferForm } from "../screens/entry/StockTransferForm";
import { ItemForm } from "../screens/entry/ItemForm";
import { BOMForm } from "../screens/entry/BOMForm";
import { ProductionJobForm } from "../screens/entry/ProductionJobForm";
import { BankStatementList } from "../screens/list/BankStatementList";
import { BankStatementImport } from "../screens/entry/BankStatementImport";
import { ReconciliationForm } from "../screens/entry/ReconciliationForm";
import { FixedAssetForm } from "../screens/entry/FixedAssetForm";
import { AssetDepreciateForm } from "../screens/entry/AssetDepreciateForm";
import { AssetDisposeForm } from "../screens/entry/AssetDisposeForm";
import { PPNReconciliation } from "../screens/list/PPNReconciliation";
import { PPhFinalCalculator } from "../screens/list/PPhFinalCalculator";
import { ECLCalculator } from "../screens/list/ECLCalculator";
import { DimensionList } from "../screens/list/DimensionList";
import { BudgetList } from "../screens/list/BudgetList";
import { BudgetVsActual } from "../screens/list/BudgetVsActual";
import { BudgetForm } from "../screens/entry/BudgetForm";
import { LeaseContractList } from "../screens/list/LeaseContractList";
import { ConsolidatedReport } from "../screens/list/ConsolidatedReport";
import { LeaseContractForm } from "../screens/entry/LeaseContractForm";
import { LeasePaymentSchedule } from "../screens/entry/LeasePaymentSchedule";
import { FinancialNoteForm } from "../screens/entry/FinancialNoteForm";
import { ReportTemplateEditor } from "../screens/entry/ReportTemplateEditor";
import { ApprovalRuleList } from "../screens/list/ApprovalRuleList";
import { PendingApprovalRequestList } from "../screens/list/PendingApprovalRequestList";
import { EmailTemplateList } from "../screens/list/EmailTemplateList";
import { EmailQueueList } from "../screens/list/EmailQueueList";
import { ApprovalRuleForm } from "../screens/entry/ApprovalRuleForm";
import { WarehouseList } from "../screens/list/WarehouseList";
import { WarehouseForm } from "../screens/entry/WarehouseForm";
import defaultEntryTitle, { findModule } from "./modules";

export function WorkArea() {
  const workbench = useWorkbench();
  const activeTab = workbench.activeTab;

  // App-wide keyboard shortcuts (Ctrl/Cmd+S, Esc). Active only while the
  // work area is mounted, i.e. inside the shell.
  useKeyboardShortcuts();

  // Esc dispatches "app:close-tab": close the active nested child, or fall
  // back to the active top-level tab. The dashboard is pinned and ignored.
  const wbRef = useRef(workbench);
  wbRef.current = workbench;
  useEffect(() => {
    function onCloseTab() {
      const wb = wbRef.current;
      if (wb.activeNested) {
        wb.close(wb.activeNested.id);
      } else if (wb.activeTab && wb.activeTab.id !== "tab-dashboard") {
        wb.close(wb.activeTab.id);
      }
    }
    document.addEventListener(CLOSE_TAB_EVENT, onCloseTab);
    return () => document.removeEventListener(CLOSE_TAB_EVENT, onCloseTab);
  }, []);

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
  // The +New button should open a draft of the entry kind that matches the
  // currently active list, not the module's first sub-item (N-01).
  const activeListItem = activeChild?.kind === "list"
    ? module?.items.find((i) => i.openList === activeChild.subKind)
    : module?.items[0];
  const onAdd = activeListItem?.openEntry
    ? () => workbench.openEntryDraft(activeListItem.openEntry!)
    : undefined;
  const addLabel = activeListItem?.openEntry
    ? `New ${defaultEntryTitle(activeListItem.openEntry).toLowerCase()}`
    : "New entry";

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
        onAdd={onAdd}
        addLabel={addLabel}
      />
      <div className="module-content__body">
        {children.map((child) => (
          // Render every child tab but hide inactive ones with CSS so
          // switching tabs preserves unsaved form state (E-02).
          <div
            key={child.id}
            className="module-content__pane"
            style={{ display: child.id === activeChildId ? "block" : "none" }}
          >
            <TabErrorBoundary title={child.title}>
              <NestedContent tab={child} />
            </TabErrorBoundary>
          </div>
        ))}
        {!activeChild && (
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
    case "customer-list":
      return <CustomerList />;
    case "supplier-invoice":
      return <SupplierInvoiceList />;
    case "purchase-return":
      return <PurchaseReturnList />;
    case "bank-reconciliation":
      return <BankStatementList />;
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
    case "bom":
      return <BOMList />;
    case "production-job":
      return <ProductionJobList />;
    case "asset-register":
      return <AssetRegisterList />;
    case "fixed-assets":
      return <FixedAssetList />;
    case "journal-entry":
      return <JournalEntryList />;
    case "general-ledger":
      return <GeneralLedger />;
    case "journal-register":
      return <JournalRegister />;
    case "report-trial-balance":
      return <TrialBalanceReport />;
    case "report-profit-loss":
      return <ProfitLossReport />;
    case "report-balance-sheet":
      return <BalanceSheetReport />;
    case "report-cash-flow":
      return <CashFlowReport />;
    case "ppn-reconciliation":
      return <PPNReconciliation />;
    case "pph-final":
      return <PPhFinalCalculator />;
    case "ecl-calculator":
      return <ECLCalculator />;
    case "dimensions":
      return <DimensionList />;
    case "budgets":
      return <BudgetList />;
    case "budget-vs-actual":
      return <BudgetVsActual />;
    case "audit-logs":
      return <AuditLogList />;
    case "financial-notes":
      return <FinancialNotesList />;
    case "ar-aging":
      return <ARAgingList />;
    case "ap-aging":
      return <APAgingList />;
    case "cheque-list":
      return <ChequeList />;
    case "due-date-reminders":
      return <DueDateReminders />;
    case "customer-statement":
      return <CustomerStatementScreen />;
    case "lease-contract":
      return <LeaseContractList />;
    case "consolidated-report":
      return <ConsolidatedReport />;
    case "report-templates":
      return <ReportTemplateList />;
    case "approval-rules":
      return <ApprovalRuleList />;
    case "pending-approval-requests":
      return <PendingApprovalRequestList />;
<<<<<<< HEAD
    case "warehouse-list":
      return <WarehouseList />;
=======
    case "email-templates":
      return <EmailTemplateList />;
    case "email-queue":
      return <EmailQueueList />;
>>>>>>> fix-email-wave6b
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
      return <InvoiceForm tabId={tab.id} entryId={tab.entryId} initialTitle={tab.title} prefill={tab.prefill} />;
    case "credit-note-entry":
      return <CreditNoteForm tabId={tab.id} entryId={tab.entryId} initialTitle={tab.title} prefill={tab.prefill} />;
    case "sales-receipt":
    case "purchase-invoice":
    case "purchase-payment":
    case "asset-register":
      return (
        <DemoEntryForm
          tabId={tab.id}
          subKind={subKind}
          title={defaultEntryTitle(subKind)}
          initialTitle={tab.title}
        />
      );
    case "inventory-item":
      return <ItemForm tabId={tab.id} entryId={tab.entryId} initialTitle={tab.title} />;
    case "sales-quotation-entry":
      return <QuotationForm tabId={tab.id} entryId={tab.entryId} initialTitle={tab.title} />;
    case "sales-order-entry":
      return <SalesOrderForm tabId={tab.id} entryId={tab.entryId} initialTitle={tab.title} prefill={tab.prefill} />;
    case "delivery-order-entry":
      return <DeliveryOrderForm tabId={tab.id} entryId={tab.entryId} initialTitle={tab.title} prefill={tab.prefill} />;
    case "purchase-order-entry":
      return <PurchaseOrderForm tabId={tab.id} entryId={tab.entryId} initialTitle={tab.title} />;
    case "grn-entry":
      return <GRNForm tabId={tab.id} entryId={tab.entryId} initialTitle={tab.title} prefill={tab.prefill} />;
    case "supplier-invoice-entry":
      return <SupplierInvoiceForm tabId={tab.id} entryId={tab.entryId} initialTitle={tab.title} prefill={tab.prefill} />;
    case "purchase-return-entry":
      return <PurchaseReturnForm tabId={tab.id} entryId={tab.entryId} initialTitle={tab.title} />;
    case "purchase-supplier-entry":
      return <PurchaseSupplierForm tabId={tab.id} entryId={tab.entryId} initialTitle={tab.title} />;
    case "customer-entry":
      return <CustomerForm tabId={tab.id} entryId={tab.entryId} initialTitle={tab.title} />;
    case "stock-opname-entry":
      return <StockOpnameForm tabId={tab.id} entryId={tab.entryId} initialTitle={tab.title} />;
    case "stock-transfer-entry":
      return <StockTransferForm tabId={tab.id} entryId={tab.entryId} initialTitle={tab.title} />;
    case "bom-entry":
      return <BOMForm tabId={tab.id} entryId={tab.entryId} initialTitle={tab.title} />;
    case "production-job-entry":
      return <ProductionJobForm tabId={tab.id} entryId={tab.entryId} initialTitle={tab.title} />;
    case "journal-entry":
      return <JournalEntryForm tabId={tab.id} entryId={tab.entryId} initialTitle={tab.title} />;
    case "bank-reconciliation-entry":
      if (tab.entryId == null) {
        return <BankStatementImport tabId={tab.id} initialTitle={tab.title} />;
      }
      return <ReconciliationForm tabId={tab.id} entryId={tab.entryId} initialTitle={tab.title} />;
    case "fixed-assets-entry":
      return <FixedAssetForm tabId={tab.id} entryId={tab.entryId} initialTitle={tab.title} />;
    case "asset-depreciate":
      return <AssetDepreciateForm tabId={tab.id} assetId={Number(tab.entryId)} initialTitle={tab.title} />;
    case "asset-dispose":
      return <AssetDisposeForm tabId={tab.id} assetId={Number(tab.entryId)} initialTitle={tab.title} />;
    case "budget-entry":
      return <BudgetForm tabId={tab.id} entryId={tab.entryId} initialTitle={tab.title} />;
    case "lease-contract-entry":
      return <LeaseContractForm tabId={tab.id} entryId={tab.entryId} initialTitle={tab.title} />;
    case "lease-payment-schedule":
      return <LeasePaymentSchedule tabId={tab.id} leaseId={tab.entryId} initialTitle={tab.title} />;
    case "financial-notes-entry":
      return <FinancialNoteForm tabId={tab.id} entryId={tab.entryId} initialTitle={tab.title as string} />;
    case "cheque-entry":
      return <ChequeForm {...tab} />;
    case "rp-editor":
      return <ReportTemplateEditor tabId={tab.id} entryId={tab.entryId} initialTitle={tab.title as string} />;
    case "approval-rule-entry":
      return <ApprovalRuleForm tabId={tab.id} title={tab.title as string} />;
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
