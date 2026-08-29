import { lazy, Suspense, useEffect, useRef } from "react";
import { useWorkbench } from "./state";
import { EmptyState } from "../components/ui";
import { TabErrorBoundary } from "../App";
import { useKeyboardShortcuts, CLOSE_TAB_EVENT } from "../hooks/useKeyboardShortcuts";
import { NestedTabStrip } from "./NestedTabStrip";
import { PaneTabScope } from "./useTabRefresh";
import type { EntryTab, ListTab, ModuleTab, NestedTab } from "./types";
import { defaultEntryTitle, findModule } from "./modules";

const CashEntryList = lazy(() => import("../screens/list/CashEntryList").then((m) => { return { default: m.CashEntryList }; }));
const CashEntryForm = lazy(() => import("../screens/entry/CashEntryForm").then((m) => { return { default: m.CashEntryForm }; }));
const TrialBalanceReport = lazy(() => import("../screens/list/Reports").then((m) => { return { default: m.TrialBalanceReport }; }));
const ProfitLossReport = lazy(() => import("../screens/list/Reports").then((m) => { return { default: m.ProfitLossReport }; }));
const BalanceSheetReport = lazy(() => import("../screens/list/Reports").then((m) => { return { default: m.BalanceSheetReport }; }));
const CashFlowReport = lazy(() => import("../screens/list/Reports").then((m) => { return { default: m.CashFlowReport }; }));
const DueDateReminders = lazy(() => import("../screens/list/DueDateReminders").then((m) => { return { default: m.DueDateReminders }; }));
const CustomerStatementScreen = lazy(() => import("../screens/list/CustomerStatement").then((m) => { return { default: m.CustomerStatementScreen }; }));
const DashboardScreen = lazy(() => import("../screens/workbench/DashboardScreen").then((m) => { return { default: m.DashboardScreen }; }));
const SalesReceiptList = lazy(() => import("../screens/list/Sales").then((m) => { return { default: m.SalesReceiptList }; }));
const QuotationList = lazy(() => import("../screens/list/Sales").then((m) => { return { default: m.QuotationList }; }));
const SalesOrderList = lazy(() => import("../screens/list/SalesOrderList").then((m) => { return { default: m.SalesOrderList }; }));
const DeliveryOrderList = lazy(() => import("../screens/list/DeliveryOrderList").then((m) => { return { default: m.DeliveryOrderList }; }));
const InvoiceList = lazy(() => import("../screens/list/InvoiceList").then((m) => { return { default: m.InvoiceList }; }));
const CreditNoteList = lazy(() => import("../screens/list/CreditNoteList").then((m) => { return { default: m.CreditNoteList }; }));
const PurchaseOrderList = lazy(() => import("../screens/list/PurchaseOrderList").then((m) => { return { default: m.PurchaseOrderList }; }));
const GRNList = lazy(() => import("../screens/list/GRNList").then((m) => { return { default: m.GRNList }; }));
const PurchaseSupplierList = lazy(() => import("../screens/list/PurchaseSupplierList").then((m) => { return { default: m.PurchaseSupplierList }; }));
const CustomerList = lazy(() => import("../screens/list/CustomerList").then((m) => { return { default: m.CustomerList }; }));
const PurchaseReturnList = lazy(() => import("../screens/list/PurchaseReturnList").then((m) => { return { default: m.PurchaseReturnList }; }));
const PurchaseInvoiceList = lazy(() => import("../screens/list/Purchases").then((m) => { return { default: m.PurchaseInvoiceList }; }));
const PurchasePaymentList = lazy(() => import("../screens/list/Purchases").then((m) => { return { default: m.PurchasePaymentList }; }));
const SupplierInvoiceList = lazy(() => import("../screens/list/SupplierInvoiceList").then((m) => { return { default: m.SupplierInvoiceList }; }));
const InventoryItemsList = lazy(() => import("../screens/list/Inventory").then((m) => { return { default: m.InventoryItemsList }; }));
const StockMovementsList = lazy(() => import("../screens/list/Inventory").then((m) => { return { default: m.StockMovementsList }; }));
const StockOpnameList = lazy(() => import("../screens/list/StockOpnameList").then((m) => { return { default: m.StockOpnameList }; }));
const StockTransferList = lazy(() => import("../screens/list/StockTransferList").then((m) => { return { default: m.StockTransferList }; }));
const BOMList = lazy(() => import("../screens/list/BOMList").then((m) => { return { default: m.BOMList }; }));
const ProductionJobList = lazy(() => import("../screens/list/ProductionJobList").then((m) => { return { default: m.ProductionJobList }; }));
const AssetRegisterList = lazy(() => import("../screens/list/Assets").then((m) => { return { default: m.AssetRegisterList }; }));
const FixedAssetList = lazy(() => import("../screens/list/FixedAssetList").then((m) => { return { default: m.FixedAssetList }; }));
const JournalEntryList = lazy(() => import("../screens/list/JournalEntryList").then((m) => { return { default: m.JournalEntryList }; }));
const GeneralLedger = lazy(() => import("../screens/list/GeneralLedger").then((m) => { return { default: m.GeneralLedger }; }));
const JournalRegister = lazy(() => import("../screens/list/JournalRegister").then((m) => { return { default: m.JournalRegister }; }));
const AuditLogList = lazy(() => import("../screens/list/AuditLogList").then((m) => { return { default: m.AuditLogList }; }));
const ReportTemplateList = lazy(() => import("../screens/list/ReportTemplateList").then((m) => { return { default: m.ReportTemplateList }; }));
const FinancialNotesList = lazy(() => import("../screens/list/FinancialNotesList").then((m) => { return { default: m.FinancialNotesList }; }));
const ARAgingList = lazy(() => import("../screens/list/ARAgingList").then((m) => { return { default: m.ARAgingList }; }));
const APAgingList = lazy(() => import("../screens/list/APAgingList").then((m) => { return { default: m.APAgingList }; }));
const JournalEntryForm = lazy(() => import("../screens/entry/JournalEntryForm").then((m) => { return { default: m.JournalEntryForm }; }));
const DemoEntryForm = lazy(() => import("../screens/entry/DemoEntryForm").then((m) => { return { default: m.DemoEntryForm }; }));
const ChequeList = lazy(() => import("../screens/list/ChequeList").then((m) => { return { default: m.ChequeList }; }));
const ChequeForm = lazy(() => import("../screens/entry/ChequeForm").then((m) => { return { default: m.ChequeForm }; }));
const QuotationForm = lazy(() => import("../screens/entry/QuotationForm").then((m) => { return { default: m.QuotationForm }; }));
const SalesOrderForm = lazy(() => import("../screens/entry/SalesOrderForm").then((m) => { return { default: m.SalesOrderForm }; }));
const DeliveryOrderForm = lazy(() => import("../screens/entry/DeliveryOrderForm").then((m) => { return { default: m.DeliveryOrderForm }; }));
const InvoiceForm = lazy(() => import("../screens/entry/InvoiceForm").then((m) => { return { default: m.InvoiceForm }; }));
const CreditNoteForm = lazy(() => import("../screens/entry/CreditNoteForm").then((m) => { return { default: m.CreditNoteForm }; }));
const PurchaseOrderForm = lazy(() => import("../screens/entry/PurchaseOrderForm").then((m) => { return { default: m.PurchaseOrderForm }; }));
const GRNForm = lazy(() => import("../screens/entry/GRNForm").then((m) => { return { default: m.GRNForm }; }));
const PurchaseSupplierForm = lazy(() => import("../screens/entry/PurchaseSupplierForm").then((m) => { return { default: m.PurchaseSupplierForm }; }));
const CustomerForm = lazy(() => import("../screens/entry/CustomerForm").then((m) => { return { default: m.CustomerForm }; }));
const SupplierInvoiceForm = lazy(() => import("../screens/entry/SupplierInvoiceForm").then((m) => { return { default: m.SupplierInvoiceForm }; }));
const PurchaseReturnForm = lazy(() => import("../screens/entry/PurchaseReturnForm").then((m) => { return { default: m.PurchaseReturnForm }; }));
const StockOpnameForm = lazy(() => import("../screens/entry/StockOpnameForm").then((m) => { return { default: m.StockOpnameForm }; }));
const StockTransferForm = lazy(() => import("../screens/entry/StockTransferForm").then((m) => { return { default: m.StockTransferForm }; }));
const ItemForm = lazy(() => import("../screens/entry/ItemForm").then((m) => { return { default: m.ItemForm }; }));
const BOMForm = lazy(() => import("../screens/entry/BOMForm").then((m) => { return { default: m.BOMForm }; }));
const ProductionJobForm = lazy(() => import("../screens/entry/ProductionJobForm").then((m) => { return { default: m.ProductionJobForm }; }));
const BankStatementList = lazy(() => import("../screens/list/BankStatementList").then((m) => { return { default: m.BankStatementList }; }));
const BankStatementImport = lazy(() => import("../screens/entry/BankStatementImport").then((m) => { return { default: m.BankStatementImport }; }));
const ReconciliationForm = lazy(() => import("../screens/entry/ReconciliationForm").then((m) => { return { default: m.ReconciliationForm }; }));
const FixedAssetForm = lazy(() => import("../screens/entry/FixedAssetForm").then((m) => { return { default: m.FixedAssetForm }; }));
const AssetDepreciateForm = lazy(() => import("../screens/entry/AssetDepreciateForm").then((m) => { return { default: m.AssetDepreciateForm }; }));
const AssetDisposeForm = lazy(() => import("../screens/entry/AssetDisposeForm").then((m) => { return { default: m.AssetDisposeForm }; }));
const PPNReconciliation = lazy(() => import("../screens/list/PPNReconciliation").then((m) => { return { default: m.PPNReconciliation }; }));
const PPhFinalCalculator = lazy(() => import("../screens/list/PPhFinalCalculator").then((m) => { return { default: m.PPhFinalCalculator }; }));
const ECLCalculator = lazy(() => import("../screens/list/ECLCalculator").then((m) => { return { default: m.ECLCalculator }; }));
const DimensionList = lazy(() => import("../screens/list/DimensionList").then((m) => { return { default: m.DimensionList }; }));
const BudgetList = lazy(() => import("../screens/list/BudgetList").then((m) => { return { default: m.BudgetList }; }));
const BudgetVsActual = lazy(() => import("../screens/list/BudgetVsActual").then((m) => { return { default: m.BudgetVsActual }; }));
const BudgetForm = lazy(() => import("../screens/entry/BudgetForm").then((m) => { return { default: m.BudgetForm }; }));
const LeaseContractList = lazy(() => import("../screens/list/LeaseContractList").then((m) => { return { default: m.LeaseContractList }; }));
const ConsolidatedReport = lazy(() => import("../screens/list/ConsolidatedReport").then((m) => { return { default: m.ConsolidatedReport }; }));
const LeaseContractForm = lazy(() => import("../screens/entry/LeaseContractForm").then((m) => { return { default: m.LeaseContractForm }; }));
const LeasePaymentSchedule = lazy(() => import("../screens/entry/LeasePaymentSchedule").then((m) => { return { default: m.LeasePaymentSchedule }; }));
const FinancialNoteForm = lazy(() => import("../screens/entry/FinancialNoteForm").then((m) => { return { default: m.FinancialNoteForm }; }));
const ReportTemplateEditor = lazy(() => import("../screens/entry/ReportTemplateEditor").then((m) => { return { default: m.ReportTemplateEditor }; }));
const ApprovalRuleList = lazy(() => import("../screens/list/ApprovalRuleList").then((m) => { return { default: m.ApprovalRuleList }; }));
const PendingApprovalRequestList = lazy(() => import("../screens/list/PendingApprovalRequestList").then((m) => { return { default: m.PendingApprovalRequestList }; }));
const EmailTemplateList = lazy(() => import("../screens/list/EmailTemplateList").then((m) => { return { default: m.EmailTemplateList }; }));
const EmailQueueList = lazy(() => import("../screens/list/EmailQueueList").then((m) => { return { default: m.EmailQueueList }; }));
const ApprovalRuleForm = lazy(() => import("../screens/entry/ApprovalRuleForm").then((m) => { return { default: m.ApprovalRuleForm }; }));
const WarehouseList = lazy(() => import("../screens/list/WarehouseList").then((m) => { return { default: m.WarehouseList }; }));
const UnitList = lazy(() => import("../screens/list/UnitList").then((m) => { return { default: m.UnitList }; }));
const ItemNameMasterList = lazy(() => import("../screens/list/ItemNameMasterList").then((m) => { return { default: m.ItemNameMasterList }; }));
const TaxMasterList = lazy(() => import("../screens/list/TaxMasterList").then((m) => { return { default: m.TaxMasterList }; }));
const SettingsCompanyScreen = lazy(() => import("../screens/list/SettingsCompanyScreen").then((m) => { return { default: m.SettingsCompanyScreen }; }));
const SettingsCurrencyScreen = lazy(() => import("../screens/list/SettingsCurrencyScreen").then((m) => { return { default: m.SettingsCurrencyScreen }; }));
const SettingsDefaultAccountsScreen = lazy(() => import("../screens/list/SettingsDefaultAccountsScreen").then((m) => { return { default: m.SettingsDefaultAccountsScreen }; }));
const SettingsPreferencesScreen = lazy(() => import("../screens/list/SettingsPreferencesScreen").then((m) => { return { default: m.SettingsPreferencesScreen }; }));
const WarehouseForm = lazy(() => import("../screens/entry/WarehouseForm").then((m) => { return { default: m.WarehouseForm }; }));

export function WorkArea() {
  const workbench = useWorkbench();
  const activeTab = workbench.activeTab;

  // App-wide keyboard shortcuts (Ctrl/Cmd+S, Esc). Active only while the
  // work area is mounted, i.e. inside the shell.
  useKeyboardShortcuts({
    Escape: () => document.dispatchEvent(new CustomEvent(CLOSE_TAB_EVENT)),
  });

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
  // The +New button should follow the ACTIVE child tab: its matching list
  // when a list is active, or the entry's own kind when an entry form is
  // active — falling back to the module's first sub-item (N-01).
  const activeListItem = !activeChild
    ? undefined
    : activeChild.kind === "list"
      ? module?.items.find((i) => i.openList === activeChild.subKind)
      : module?.items.find((i) => i.openEntry === activeChild.subKind) ?? module?.items[0];
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
              <PaneTabScope tab={child}>
                <NestedContent tab={child} />
              </PaneTabScope>
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
  const content =
    tab.kind === "list" ? <ListTabContent tab={tab} /> : <EntryTabContent tab={tab} />;
  return (
    <Suspense
      fallback={
        <div className="workarea">
          <p className="loading-state" role="status">
            <span className="loading-state__spinner" aria-hidden="true" />
            <span>Memuat layar...</span>
          </p>
        </div>
      }
    >
      {content}
    </Suspense>
  );
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
    case "warehouse-list":
      return <WarehouseList />;
    case "unit-list":
      return <UnitList />;
    case "item-category-list":
      return <ItemNameMasterList kind="category" />;
    case "item-brand-list":
      return <ItemNameMasterList kind="brand" />;
    case "tax-master-list":
      return <TaxMasterList />;
    case "settings-company":
      return <SettingsCompanyScreen />;
    case "settings-currency":
      return <SettingsCurrencyScreen />;
    case "settings-default-accounts":
      return <SettingsDefaultAccountsScreen />;
    case "settings-preferences":
      return <SettingsPreferencesScreen />;

    case "email-templates":
      return <EmailTemplateList />;
    case "email-queue":
      return <EmailQueueList />;
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
