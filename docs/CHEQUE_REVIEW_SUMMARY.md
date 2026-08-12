# Cheques & GIRO Module (Wave 6) - Code Review Summary

## ✅ Files Created (4 new)

| File | Lines | Purpose |
|------|-------|---------|
| `web/src/screens/list/ChequeList.tsx` | 183 | List view with status badges, direction filter, action buttons per row |
| `web/src/screens/entry/ChequeForm.tsx` | 173 | Entry form for register/edit cheque |
| `web/src/screens/entry/DepositModal.tsx` | 54 | Deposit confirmation modal |
| `web/src/screens/entry/BounceModal.tsx` | 69 | Bounce modal with reason input |

## ✅ Types Added (`web/src/types.ts`)

```typescript
interface ChequeListItem {
  id: number;
  cheque_number: string;
  type: "RECEIVED" | "ISSUED";
  direction: "INBOUND" | "OUTBOUND";
  bank_name: string;
  amount_cents: number;
  counterparty_name: string;
  date: string;
  status: "REGISTERED" | "DEPOSITED" | "CLEARED" | "BOUNCED";
}

interface Cheque extends ChequeListItem {
  bank_account_id?: number | null;
  image_url?: string | null;
  notes?: string | null;
}

interface ChequeCreateInput { ... }

interface BankAccountListItem {
  id: number;
  account_name: string;
  code: string;
}
```

## ✅ API Methods Added (`web/src/api.ts`)

| Method | Endpoint | Description |
|--------|----------|-------------|
| `listCheques(params?)` | GET /cheques | List cheques with optional type filter |
| `createCheque(input)` | POST /cheques | Register new cheque |
| `updateCheque(id, input)` | PUT /cheques/{id} | Update cheque |
| `deleteCheque(id)` | DELETE /cheques/{id} | Delete cheque |
| `depositCheque(id)` | POST /cheques/{id}/deposit | Deposit cheque (Dr Bank / Cr 1305) |
| `clearCheque(id)` | POST /cheques/{id}/clear | Manually clear deposited cheque |
| `bounceCheque(id, reason)` | POST /cheques/{id}/bounce | Bounce cheque (reverse deposit + re-establish AR/AP) |

## ✅ Module Registry (`web/src/workbench/modules.ts`)

- Added `"Cheques & GIRO"` entry under Cash & Bank module
- Open list: `cheque-list`, open entry: `cheque-entry`
- Draft number prefix: `CHK-DRAFT`
- Title mappers updated for list and entry views

## ✅ Routing Wiring (`web/src/workbench/WorkArea.tsx`)

- Imported `ChequeList` and `ChequeForm` components
- Added case `"cheque-list"` in `renderList()` → `<ChequeList />`
- Added case `"cheque-entry"` in `renderEntry()` → `<ChequeForm id={tab.id} ... />`

## 🏗️ Build Test Results

```bash
$ cd web && node_modules/.bin/tsc --noEmit
# Result: No Cheque-related TypeScript errors ✅

# Unrelated pre-existing errors (not caused by this implementation):
# - DemoEntryForm.tsx: Missing minimal demo coverage for Wave 4 modules
# - ReportTemplateList.tsx: is_default property mismatch (pre-existing issue)
```

## 🎨 UI Design Compliance

✅ **Status Badges** (per requirement):
- REGISTERED → blue (`status-badge--info`)
- DEPOSITED → amber (`status-badge--warning`)
- CLEARED → green (`status-badge--success`)
- BOUNCED → red (`status-badge--danger`)

✅ **Direction Filter**: Dropdown with options "" (All), "RECEIVED", "ISSUED"

✅ **Action Buttons Per Row**:
- REGISTERED: [Deposit] button → opens DepositModal
- DEPOSITED: [Bounce] button → opens BounceModal  
- CLEARED: N/A
- BOUNCED: [View] button → opens entry in edit mode

✅ **Bank Account Selection**: Native `<select>` styled as combobox with list of bank accounts from backend

✅ **Modals**: 
- DepositModal: Shows cheque details, confirmation button
- BounceModal: Shows cheque details, required reason textarea, confirmation button

## 🔗 Backend Integration

All endpoints already implemented in `backend/internal/cheque/handler.go`:

| Event | Journal Entry (from handler.go:27-38) |
|-------|---------------------------------------|
| **Register RECEIVED** | Dr 1305 Cheques in Transit / Cr Cash/Bank |
| **Register ISSUED** | Dr 2101 AP / Cr 2106 Cheques Issued Outstanding |
| **Deposit RECEIVED** | Dr Bank / Cr 1305 Cheques in Transit |
| **Deposit ISSUED** | Dr 2101 AP / Cr 2106 Cheques Issued Outstanding |
| **Clear** | Auto-posts on bank reconciliation sync |
| **Bounce** | Reverses deposit journal; re-establishes AR/AP if originally against receivable/payable |

## ✅ Acceptance Criteria Met

- ✅ List cheques with status badge (REGISTERED/DEPOSITED/CLEARED/BOUNCED)
- ✅ Direction dropdown filter (RECEIVED/ISSUED)
- ✅ Register cheque with cheque_number, bank_account combobox, amount_cents, counterparty name, date, type dropdown
- ✅ Action buttons per row based on status
- ✅ DepositModal component for deposit confirmation
- ✅ BounceModal component with reason input and automatic journal reversal
- ✅ Cheques & GIRO module added under "Cash & Bank" in sidebar
- ✅ Wired to WorkArea routing (both list and entry views)
- ✅ TypeScript compilation successful (no cheque-related errors)

## 📝 Notes

- Uses existing `Combobox` component pattern but simplified with native `<select>` due to different props signature
- Form follows same pattern as other entry forms (JournalEntryForm, InvoiceForm)
- Modals follow existing modal styling conventions
- Error handling consistent with rest of codebase (try/catch with user-friendly error messages)

---

**STATUS: READY FOR TESTING**
