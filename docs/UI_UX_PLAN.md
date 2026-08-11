# UI/UX Plan — Per-Module Detail & Navigation Redesign

**Audit Date:** 2026-08-11  
**Scope:** Every sidebar module, every sub-item, every entry form, every transaction flow. Navigation patterns. Click-count analysis. Workflow chain gaps.

---

## Part 1: Navigation Architecture — Current Problems

### Current Flow (broken)

```
Sidebar hover → Flyout menu → Click sub-item → LIST opens (tab 1)
                                     ↓
                              Click "+New" → DRAFT ENTRY opens (tab 2)
                                     ↓
                              Fill form → Click Save → Badge changes to POSTED
                                     ↓
                              Stuck on form. List NOT refreshed. No toast. No "next step".
```

### Problems

| # | Problem | Impact |
|---|---|---|
| N-01 | **"+New" button uses wrong entry kind** — `WorkArea.tsx:149-151` uses `module.items[0].openEntry` (first sub-item), not the one matching the currently active list. If you're on "Credit Notes" and click "+", it opens an Invoice draft (items[0]). | Wrong form opens |
| N-02 | **List-first navigation adds a step** — sidebar always opens a list, not the form. Power users want to start typing a transaction immediately. | +1 click per transaction |
| N-03 | **No workflow chain** — no "Convert to SO", "Create DO", "Create Invoice" buttons. User must manually re-enter all line items at every step. | 20-60 redundant clicks per pipeline |
| N-04 | **No auto-fill from parent** — GRN doesn't load PO lines, DO doesn't load SO lines, CN doesn't load invoice lines. | Full manual re-entry every time |
| N-05 | **No inline master creation** — can't create customer/supplier/item from within a form. Must leave, navigate, create, return. | Breaks flow completely |
| N-06 | **No save feedback** — no toast. Badge changes. List doesn't refresh. User unsure if saved. | Confusion, duplicate saves |
| N-07 | **Tab switch destroys unsaved data** — `key` prop remounts form. All `useState` lost. | Silent data loss |
| N-08 | **Can't open 2 entries of same type** — dedup bug checks `draft` not `entryId`. | Can't compare two invoices |
| N-09 | **No "back to list" after save** — entry tab stays open. No auto-close or list-return. | Tab clutter |
| N-10 | **No keyboard shortcuts** — Ctrl+S, Esc, Alt+N all missing. | Mouse-only workflow |
| N-11 | **Journal Entry double-post risk** — Post button not disabled after save. | Duplicate journals |
| N-12 | **No "Save & New" that works** — CashEntryForm has stale-error bug, JournalEntryForm has it permanently disabled. | Can't rapid-enter transactions |

### Proposed Flow (improved)

```
Sidebar click → LIST opens with search + filters
        ↓ (or)                          ↓ (or)
    "+New" button → FORM opens      Quick-add button on sidebar
        ↓                                 ↓
    Auto-fill from parent (if linked)     Form opens immediately
        ↓                                 ↓
    Fill form (with inline master create) ↓
        ↓                                 ↓
    Ctrl+S / Click Save                   ↓
        ↓                                 ↓
    Toast: "✓ Saved INV-2026-00001"  + List refreshes behind the form
        ↓
    "Next Step" button: [Create DO] [Create Invoice] [Print] [Close]
        ↓
    Click next step → new form opens pre-filled from current document
```

---

## Part 2: Per-Module Audit & Improvements

### Module 1: Cash & Bank

| Sub-item | List Screen | Entry Form | Status | Issues |
|---|---|---|---|---|
| Other Receipt | CashEntryList ✅ | CashEntryForm ✅ | Working | 6 clicks. No account search. Balance check dead code. Save&New stale-error bug. |
| Other Payment | CashEntryList ✅ | CashEntryForm ✅ | Working | Same as above |
| Bank Transfer | CashEntryList ✅ | CashEntryForm ✅ | Working | Simpler form, works OK |
| Bank Reconciliation | BankStatementList ✅ | ReconciliationForm ✅ | Working | Import flow works |
| **Cheques/GIRO** | ❌ | ❌ | **Missing** | Backend ready, no frontend |
| **Petty Cash** | ❌ | ❌ | **Missing** | Backend ready, no frontend |

**Improvements needed:**
1. Add searchable account combobox (currently plain `<select>` with all accounts)
2. Fix balance check (dead code — `cashAmountCents === counterTotalCents` always)
3. Fix Save&New stale-error closure
4. Add toast: "✓ Saved {number}" after save
5. Auto-refresh list behind form after save
6. Add "Close & return to list" button after save
7. Add Cheques/GIRO sub-item + screen (backend: `/cheques` CRUD + deposit/clear/bounce)
8. Add Petty Cash sub-item + screens (backend: `/petty-cash/funds` + `/petty-cash/vouchers` + replenish)
9. Ctrl+S keyboard shortcut for save

### Module 2: Sales

| Sub-item | List Screen | Entry Form | Status | Issues |
|---|---|---|---|---|
| Customers | CustomerList ✅ | CustomerForm ❌ | **BROKEN** | Form permanently loading. List rows not clickable (can't edit). No `getCustomer(id)` API. 13 base fields missing. |
| Quotations | QuotationList ✅ | QuotationForm ⚠️ | Partial | No edit mode. Send button disabled. No convert to SO. No saved-state (duplicates). |
| Sales Orders | SalesOrderList ✅ | SalesOrderForm ⚠️ | Partial | No Confirm button → DP unreachable. No convert to DO. markUnsaved missing 6 fields. |
| Delivery Orders | DeliveryOrderList ✅ | DeliveryOrderForm ⚠️ | Partial | Doesn't load SO lines. COGS rounding bug. No convert to Invoice. |
| Sales Invoices | InvoiceList ✅ | InvoiceForm ⚠️ | Partial | PPN hardcoded 0. No tax input. SO filter only CLOSED. No Send/Print/Void after save. |
| Credit Notes | CreditNoteList ✅ | CreditNoteForm ⚠️ | Partial | Doesn't load invoice lines. Stale qty bug. Free-text item input. No saved-state. |
| Sales Receipts | SalesReceiptList ✅ | → InvoiceForm | Working | Redirects to invoice form (intended) |

**Improvements needed:**
1. **Fix CustomerForm** — remove loading gate, add `getCustomer(id)` API, add 13 missing base fields, make list rows clickable
2. **Add "Convert to SO" button** on QuotationForm after save → opens SO form pre-filled with quotation lines
3. **Add "Confirm" button** on SalesOrderForm → changes status to CONFIRMED, unlocks DP panel
4. **Add "Create DO" button** on SalesOrderForm after confirm → opens DO form pre-filled with SO lines
5. **Add "Create Invoice" button** on DeliveryOrderForm after save → opens Invoice form pre-filled with DO items
6. **Add PPN/tax input** to InvoiceForm (dropdown: Non-PKPN / PKPN 11% / Custom rate)
7. **Auto-fill SO lines** in DeliveryOrderForm when SO is selected
8. **Auto-fill invoice lines** in CreditNoteForm when invoice is selected
9. **Add inline "+New Customer"** button next to customer dropdown in all sales forms → opens CustomerForm in a new tab
10. **Add inline "+New Item"** button next to item dropdown → opens ItemForm (must build first)
11. **Fix stale qty bug** in CreditNoteForm (`l.qty > 0` → `qty > 0`)
12. **Add saved-state tracking** to QuotationForm and CreditNoteForm
13. **Wire Send/Cancel buttons** in QuotationForm (`api.sendQuotation`, `api.cancelQuotation`)
14. **Wire Cancel button** in SalesOrderList (`api.cancelSalesOrder`)
15. **Wire Reverse button** in CashEntryList (`api.reverseCash`)

### Module 3: Purchases

| Sub-item | List Screen | Entry Form | Status | Issues |
|---|---|---|---|---|
| Purchase Orders | PurchaseOrderList ✅ | PurchaseOrderForm ⚠️ | Partial | No saved-state (duplicates). No Create GRN. addLine/removeLine don't markUnsaved. No tax. |
| Goods Received | GRNList ✅ | GRNForm ❌ | **Broken** | Doesn't load PO lines (CRITICAL). No saved-state. No over-delivery validation. Race condition loading PO list. |
| Suppliers | PurchaseSupplierList ✅ | PurchaseSupplierForm ⚠️ | Partial | No edit mode. List rows not clickable. Missing province/postal_code. No tabId prop. |
| Supplier Invoices | SupplierInvoiceList ✅ | SupplierInvoiceForm ✅ | Working | Has payment panel. Edit mode works. |
| Purchase Payments | PurchasePaymentList ✅ | → SupplierInvoiceForm | Working | But: stuck-loading bug (no .catch) |
| Purchase Returns | PurchaseReturnList ✅ | PurchaseReturnForm ⚠️ | Partial | Shows "read-only" placeholder for existing. No actual edit. |

**Improvements needed:**
1. **CRITICAL: Auto-fill PO lines in GRNForm** — when PO selected, fetch `api.getPurchaseOrder(poId)` and populate lines with item, qty ordered, unit cost. Add "Receive All" button.
2. **Add "Create GRN" button** on PurchaseOrderForm after save → opens GRN form pre-filled with PO lines
3. **Add "Create Supplier Invoice" button** on GRNForm after save → opens SupplierInvoiceForm pre-filled with GRN items
4. **Add saved-state tracking** to PurchaseOrderForm and GRNForm (prevent duplicates)
5. **Fix race condition** in GRNForm PO list loading (use `Promise.all` not sequential `then`)
6. **Add over-delivery validation** — warn if received qty > ordered qty
7. **Fix PurchaseSupplierForm** — add edit mode, make list rows clickable, add missing fields
8. **Add inline "+New Supplier"** button next to supplier dropdown
9. **Add tax/PPN input** (PPN masukan Dr 1203)

### Module 4: Production

| Sub-item | List Screen | Entry Form | Status | Issues |
|---|---|---|---|---|
| Bill of Materials | BOMList ✅ | BOMForm ✅ | Working | Edit mode works. |
| Production Jobs | ProductionJobList ✅ | ProductionJobForm ✅ | Working | Edit mode, add costs, complete job. Missing overhead variance. |

**Improvements needed:**
1. Add "Start Production" button on BOM → opens ProductionJobForm pre-filled with BOM materials
2. Add overhead variance screen or section (backend: `/overhead-variance`)
3. Add "Post to Inventory" feedback after job completion

### Module 5: Inventory

| Sub-item | List Screen | Entry Form | Status | Issues |
|---|---|---|---|---|
| Item List | InventoryItemsList ✅ | DemoEntryForm ❌ | **Broken** | List calls real API but flagged mockData. Form is non-functional stub. No createItem API. Missing required fields (item_type, uom, costing_method). |
| Stock Movements | StockMovementsList ❌ | N/A | **Stub** | Just an EmptyState placeholder. No API call. |
| Stock Opnames | StockOpnameList ✅ | StockOpnameForm ✅ | Working | Edit mode works. |
| Stock Transfers | StockTransferList ✅ | StockTransferForm ✅ | Working | Edit mode works. |
| **Warehouses** | ❌ | ❌ | **Missing** | Backend ready, no frontend |
| **Cheques/GIRO** | ❌ | ❌ | **Missing** | Should be in Cash & Bank, not Inventory |

**Improvements needed:**
1. **Build real ItemForm** — replace DemoEntryForm stub. Fields: code, name, item_type (goods/service dropdown), uom, costing_method (FIFO/avg/specific), barcode, brand, category, sale_price, purchase_price, reorder_point, reorder_qty, lead_time_days, inventory_account_id, cogs_account_id, revenue_account_id, description. Add `createItem` API method.
2. **Remove `mockData: true`** flag from `in-items` in modules.ts (list is real)
3. **Build Stock Movements screen** — add backend endpoint `GET /inventory/movements` + frontend list with filters (item, warehouse, movement_type, date range)
4. **Add Warehouses sub-item** — backend: `/warehouses` CRUD + `/warehouses/{id}/stock`
5. Add "View Movements" button on Item List row → opens Stock Movements filtered to that item

### Module 6: Fixed Assets

| Sub-item | List Screen | Entry Form | Status | Issues |
|---|---|---|---|---|
| Asset Register | FixedAssetList ✅ | FixedAssetForm ⚠️ | Partial | List works. Form has ×100 bug. Payment account hardcoded 3 options. Has depreciate/dispose/revalue/impair actions. |
| Lease Contracts | LeaseContractList ✅ | LeaseContractForm + LeasePaymentSchedule ✅ | Working | Form + schedule + post payment. Missing: modify, terminate, depreciate UI. |
| **Asset Maintenance** | ❌ | ❌ | **Missing** | Backend ready, no frontend |

**Improvements needed:**
1. **Fix FixedAssetForm ×100 bug** — `parseInt(cost)` should be `× 100`
2. **Replace hardcoded account `<select>`** with searchable COA combobox
3. **Add Lease Modification UI** — button on LeaseContractForm → modify form (re-measure PV, adjust RoU + liability)
4. **Add Lease Termination UI** — button → terminate form (derecognize, gain/loss)
5. **Add Lease Depreciation UI** — button → depreciate form (Dr 5209 / Cr 1702)
6. **Add Asset Maintenance sub-item** — backend: `/asset-maintenance` CRUD + `/asset-maintenance/upcoming`
7. **Delete orphaned Assets.tsx** (100% mock, superseded by FixedAssetList)

### Module 7: Accountant

| Sub-item | List Screen | Entry Form | Status | Issues |
|---|---|---|---|---|
| Journal Entries | JournalEntryList ✅ | JournalEntryForm ⚠️ | Partial | Works but double-post risk (Post not disabled after save). No Save&New. Account dropdown no search. |
| General Ledger | GeneralLedger ✅ | N/A (read-only) | Working | Well-built. Account dropdown needs search. Error masking. |
| Journal Register | JournalRegister ✅ | N/A (read-only) | Working | 90% duplicate of JournalEntryList. |
| Dimensions | DimensionList ✅ | Inline create ✅ | Working | |
| Budgets | BudgetList ✅ | BudgetForm ⚠️ | Partial | entryId not destructured → no edit mode. |
| Audit Trail | AuditLogList ✅ | N/A | Working | |
| Customer Statement | CustomerStatement ✅ | N/A | Working | |
| **Cost Centers** | ❌ | ❌ | **Missing** | Backend ready, no frontend |
| **Approval Workflow** | ❌ | ❌ | **Missing** | Backend ready, no frontend |
| **Recurring** | ❌ | ❌ | **Missing** | Backend ready, no frontend |

**Improvements needed:**
1. **Fix JournalEntryForm double-post** — disable Post button after successful save, transition to read-only
2. **Add searchable account combobox** for journal entry lines (replace plain `<select>`)
3. **Add "Save & New"** that actually works (reset form + new idempotency key)
4. **Fix BudgetForm** — destructure `entryId`, add `api.getBudget(id)` call for edit mode
5. **Merge or differentiate** JournalEntryList vs JournalRegister (consider tabs in one screen)
6. **Add Cost Centers sub-item** — backend: `/cost-centers` CRUD + allocations + P&L
7. **Add Approval Workflow sub-items** — Approval Rules (config) + Approval Requests (pending list with approve/reject)
8. **Add Recurring sub-item** — backend: `/recurring` CRUD + post button
9. **Wire `tagJournalLine`** API method (exists, never called) — add dimension tag UI on journal entry lines

### Module 8: Tax

| Sub-item | List Screen | Entry Form | Status | Issues |
|---|---|---|---|---|
| PPN Reconciliation | PPNReconciliation ✅ | N/A | Working | |
| PPh Final | PPhFinalCalculator ✅ | N/A | Working | |
| ECL | ECLCalculator ✅ | N/A | Working | But: ages from invoice_date not due_date. Write-off doesn't update subledger. |

**Improvements needed:**
1. **Fix ECL aging** — use `due_date` not `invoice_date` (backend fix: `tax/ecl.go:235,262`)
2. **Fix ECL write-off** — update invoice receivable + status + customer_balances (backend fix)
3. **Wire `calculateDeferredTax`** API method (exists, never called) — add Deferred Tax screen or section

### Module 9: Reports

| Sub-item | List Screen | Entry Form | Status | Issues |
|---|---|---|---|---|
| Trial Balance | Reports.tsx ✅ | N/A | Partial | No date range. No export. |
| Profit & Loss | Reports.tsx ✅ | N/A | Partial | Has framework + dimension. No date range. No export. |
| Balance Sheet | Reports.tsx ✅ | N/A | Partial | No date range. No export. No framework. |
| Cash Flow | Reports.tsx ✅ | N/A | Partial | No date range. No export. |
| Financial Notes | FinancialNotesList ✅ | FinancialNoteForm ✅ | Working | |
| Due Date Reminders | DueDateReminders ✅ | N/A | Working | |
| Budget vs Actual | BudgetVsActual ✅ | N/A | Working | |
| Consolidated TB | ConsolidatedReport ✅ | N/A | Working | |
| Report Templates | ReportTemplateList ✅ | ReportTemplateEditor ✅ | Working | |
| **AR Aging** | ❌ | ❌ | **Missing** | Backend ready, no frontend |
| **AP Aging** | ❌ | ❌ | **Missing** | Backend ready, no frontend |

**Improvements needed:**
1. **Add date range pickers** to all 4 report screens (Trial Balance, P&L, Balance Sheet, Cash Flow) — API supports `from_date`/`to_date` already
2. **Add export buttons** (PDF + Excel) — `api.exportReport()` exists, zero UI calls
3. **Add framework selector** to Balance Sheet and Cash Flow (currently only P&L)
4. **Add dimension filter** to Balance Sheet and Cash Flow (currently only P&L)
5. **Add AR Aging sub-item** — backend: `/aging/ar` → AR aging report with buckets
6. **Add AP Aging sub-item** — backend: `/aging/ap` → AP aging report with buckets
7. **Add quick date ranges** (This Month, This Quarter, YTD, Last Month, Last Quarter)

---

## Part 3: Transaction Input Flow — Improvement Designs

### 3.1: Quick-Add from Sidebar (reduce clicks)

**Current:** Sidebar → List → "+New" → Form (3 clicks to form)  
**Proposed:** Sidebar → Right-click/long-press → "New {type}" → Form (2 clicks to form)

Or: Add a `+` button next to each sub-item in the flyout menu:

```
┌─────────────────────────┐
│ Sales                   │
│ ├─ Customers      [+]   │
│ ├─ Quotations      [+]  │
│ ├─ Sales Orders    [+]  │
│ ├─ Delivery Orders [+]  │
│ ├─ Invoices        [+]  │
│ └─ Credit Notes    [+]  │
└─────────────────────────┘
```

Clicking the sub-item opens the list. Clicking `[+]` opens a new entry form directly.

### 3.2: "+New" Button Context-Aware (fix wrong entry kind)

**Current:** `WorkArea.tsx:149-151` — `module.items[0].openEntry` (always first sub-item)  
**Proposed:** Track which list is currently active, use its `openEntry`:

```tsx
const activeListItem = module.items.find(i => i.openList === activeChild?.subKind);
const onAdd = activeListItem?.openEntry 
  ? () => workbench.openEntryDraft(activeListItem.openEntry!)
  : undefined;
const addLabel = activeListItem ? `New ${defaultEntryTitle(activeListItem.openEntry!).toLowerCase()}` : undefined;
```

### 3.3: Auto-Fill from Parent Document (the biggest UX win)

**Pattern:** When a parent document is selected in a form, auto-load its lines:

```tsx
// GRNForm — when PO selected, fetch lines
useEffect(() => {
  if (!poId) return;
  api.getPurchaseOrder(Number(poId)).then(po => {
    if (po.lines) {
      setLines(po.lines.map(l => ({
        id: crypto.randomUUID(),
        itemId: l.item_id,
        itemCode: l.item_code,
        itemName: l.item_name,
        qty: l.qty,
        unitCostCents: l.unit_price_cents,
        lineTotalCents: l.line_total_cents,
      })));
    }
  });
}, [poId]);
```

Apply same pattern to:
- DeliveryOrderForm (load SO lines when SO selected)
- CreditNoteForm (load invoice lines when invoice selected)
- InvoiceForm (load SO/DO lines when SO selected)
- SupplierInvoiceForm (load GRN lines when GRN selected)

### 3.4: Workflow Chain / Next-Step Buttons

**After saving a transaction, show contextual "next step" buttons:**

```
┌──────────────────────────────────────────┐
│ ✓ Saved SQ-2026-00001                    │
│                                          │
│  [Convert to Sales Order]  [Print]  [Close]│
└──────────────────────────────────────────┘
```

| After saving... | Next-step buttons |
|---|---|
| Quotation | Convert to SO, Print, Close |
| Sales Order | Create Delivery Order, Receive DP, Close |
| Delivery Order | Create Invoice, Print DO, Close |
| Invoice | Receive Payment, Print Invoice, Send, Close |
| Purchase Order | Create GRN, Print PO, Close |
| GRN | Create Supplier Invoice, Close |
| Supplier Invoice | Pay Supplier, Close |
| Journal Entry | New Journal, Close |
| Fixed Asset | Depreciate, Dispose, Revalue, Impair, Close |
| Lease Contract | View Schedule, Post Payment, Modify, Terminate, Close |

### 3.5: Inline Master Creation

**When customer/supplier/item dropdown is empty, show "+New" button inline:**

```tsx
<select ...>
  <option value="">Select customer...</option>
  {customers.map(c => <option key={c.id} value={c.id}>{c.name}</option>)}
</select>
<button onClick={() => workbench.openEntryDraft("customer-entry")}>
  + New Customer
</button>
```

Or better: a modal dialog that creates the master without leaving the current form.

### 3.6: Toast/Notification System

```tsx
// After successful save:
toast.success(`✓ Saved ${result.number} · ${formatIDR(result.total_cents)}`);
workbench.refreshList(activeListKind); // refresh the list behind the form
```

After error:
```tsx
toast.error(`Failed to save: ${error.message}`);
```

### 3.7: Tab Data Persistence

**Current:** `key={activeChild?.id}` unmounts form on tab switch → data lost.  
**Proposed:** Use CSS `display: none` to hide inactive tabs:

```tsx
{children.map(child => (
  <div key={child.id} style={{ display: child.id === activeChildId ? 'block' : 'none' }}>
    <NestedContent tab={child} />
  </div>
))}
```

Or: persist form state to `sessionStorage` keyed by tab ID.

### 3.8: Keyboard Shortcuts

| Shortcut | Action |
|---|---|
| `Ctrl/Cmd+S` | Save current form |
| `Esc` | Close active tab (with unsaved confirm) |
| `Ctrl+Enter` | Save & New (if in entry form) |
| `Alt+N` | New entry (from current list) |
| `Ctrl+Tab` | Switch to next tab |
| `Ctrl+Shift+Tab` | Switch to previous tab |
| `Ctrl+1..9` | Switch to module 1-9 |
| `/` | Focus search in list |

---

## Part 4: Sales Pipeline Flow (End-to-End)

### Current (broken — each step is isolated):

```
[Create Customer] → (navigate away, create, navigate back)
[Create Item] → (navigate away, create, navigate back)
[Create Quotation] → (save, stuck, no next step)
  → [Create Sales Order] → (manual re-entry of all lines, save, stuck)
    → [Create Delivery Order] → (manual re-entry of all lines, save, stuck)
      → [Create Invoice] → (manual re-entry of all lines, save, stuck)
        → [Receive Payment] → (inline panel works)
          → [Credit Note] → (manual item ID typing, save)
```

### Proposed (guided workflow):

```
[Create Quotation] — with inline "+New Customer" and "+New Item" buttons
  ↓ Save → Toast "✓ Saved SQ-001"
  ↓ Show: [Convert to Sales Order] [Print] [Send to Customer] [Close]
  ↓ Click "Convert to SO"
[Sales Order] — pre-filled with quotation lines, editable
  ↓ Save → Toast "✓ Saved SO-001"
  ↓ Show: [Receive DP] [Create Delivery Order] [Cancel SO] [Close]
  ↓ Click "Create DO"
[Delivery Order] — pre-filled with SO lines, qty editable, COGS auto-calculated
  ↓ Save → Toast "✓ Saved DO-001 · COGS Rp 500.000"
  ↓ Show: [Create Invoice] [Print DO] [Close]
  ↓ Click "Create Invoice"
[Invoice] — pre-filled with DO items, PPN 11% auto-calculated
  ↓ Save → Toast "✓ Saved INV-001 · Total Rp 1.100.000"
  ↓ Show: [Receive Payment] [Print Invoice] [Send] [Close]
  ↓ Click "Receive Payment"
[Payment] — inline panel, amount = receivable
  ↓ Save → Toast "✓ Payment received · AR settled"
  ↓ Show: [Close]
```

### Purchase Pipeline (same pattern):

```
[Create PO] — with inline "+New Supplier" and "+New Item"
  ↓ Save → [Create GRN] [Print PO]
[GRN] — pre-filled with PO lines, qty received editable
  ↓ Save → [Create Supplier Invoice] [Close]
[Supplier Invoice] — pre-filled with GRN items, PPN masukan auto-calculated
  ↓ Save → [Pay Supplier] [Print] [Close]
[Payment] — inline panel
```

---

## Part 5: Priority Implementation Order

### Sprint 1: Navigation Fixes + Toast (Day 1-3)

| Task | Effort | Impact |
|---|---|---|
| Fix "+New" button to use active list's entry kind (N-01) | 1h | Prevents wrong form opening |
| Add toast notification system (N-06) | 4h | Every save gets feedback |
| Auto-refresh list after save | 2h | List stays current |
| Fix tab switch data loss (N-07, CSS hide) | 3h | No more silent data loss |
| Fix dedup bug — compare entryId (N-08) | 1h | Can open multiple entries |
| Add Ctrl+S + Esc shortcuts (N-10) | 2h | Power user speed |
| Add "Close & return to list" after save (N-09) | 1h | Tab management |

### Sprint 2: Auto-Fill + Workflow Chain (Day 4-8)

| Task | Effort | Impact |
|---|---|---|
| GRNForm: auto-fill PO lines (N-04) | 3h | 60→3 clicks for 20-line PO |
| DeliveryOrderForm: auto-fill SO lines | 3h | No manual re-entry |
| CreditNoteForm: auto-fill invoice lines | 3h | No manual re-entry |
| InvoiceForm: auto-fill from SO/DO | 3h | No manual re-entry |
| Add "Convert to SO" button on QuotationForm | 3h | Guided sales flow |
| Add "Confirm" + "Create DO" on SalesOrderForm | 4h | Guided sales flow |
| Add "Create Invoice" on DeliveryOrderForm | 3h | Guided sales flow |
| Add "Create GRN" on PurchaseOrderForm | 3h | Guided purchase flow |
| Add "Create Supplier Invoice" on GRNForm | 3h | Guided purchase flow |
| Add inline "+New Customer/Supplier/Item" buttons | 4h | No flow breaking |
| Build Combobox/SearchableSelect (N-03 for FK lookups) | 8h | Searchable COA/customer/item |

### Sprint 3: Form Fixes (Day 9-14)

| Task | Effort |
|---|---|
| Fix CustomerForm loading + add 13 fields + getCustomer API | 4h |
| Build real ItemForm + createItem API | 6h |
| Fix CashEntryForm balance check + Save&New | 3h |
| Fix CreditNoteForm stale qty | 30 min |
| Fix DeliveryOrderForm COGS rounding | 30 min |
| Fix FixedAssetForm ×100 | 30 min |
| Add PPN/tax input to 4 forms | 4h |
| Add saved-state tracking to 4 forms | 4h |
| Fix JournalEntryForm double-post | 1h |
| Fix BudgetForm edit mode | 2h |
| Wire Send/Cancel/Reverse buttons (orphaned API methods) | 4h |
| Make CustomerList/SupplierList rows clickable | 2h |

### Sprint 4: Missing Modules (Day 12-20, parallel via Agent Manager)

| Task | Effort |
|---|---|
| Cheques & GIRO (list + form + state actions) | 1 day |
| Petty Cash (funds + vouchers + replenish) | 1 day |
| Warehouses (list + form + stock view) | 0.5 day |
| Asset Maintenance (list + form + upcoming) | 0.5 day |
| Cost Centers (list + form + allocations + P&L) | 1.5 days |
| Approval Workflow (rules + requests + approve/reject) | 1.5 days |
| Recurring Transactions (list + form + post) | 1 day |
| AR Aging (list with buckets + summary) | 0.5 day |
| AP Aging (list with buckets + summary) | 0.5 day |
| Email Templates & Queue | 1 day |
| Stock Movements screen (backend endpoint + frontend) | 1 day |
| Lease modify/terminate/depreciate UI | 1 day |

### Sprint 5: Reports + Polish (Day 18-22)

| Task | Effort |
|---|---|
| Add date range + quick ranges to all 4 reports | 2h |
| Wire export buttons (PDF/Excel) | 2h |
| Add framework/dimension to Balance Sheet + Cash Flow | 2h |
| Delete dead code (TransactionsScreen, Assets.tsx, nginx.conf) | 30 min |
| Merge/differentiate JournalEntryList vs JournalRegister | 4h |
| Remove mockData flags | 5 min |
| Delete 20 stale branches | 5 min |

---

## Total Effort

| Sprint | Focus | Days |
|---|---|---|
| 1 | Navigation fixes + toast | 3 |
| 2 | Auto-fill + workflow chain + combobox | 5 |
| 3 | Form fixes (all bugs) | 6 |
| 4 | Missing modules (12 items, parallel) | 8 |
| 5 | Reports + polish | 1 |
| | **Total (with parallelism)** | **~23 days** |
