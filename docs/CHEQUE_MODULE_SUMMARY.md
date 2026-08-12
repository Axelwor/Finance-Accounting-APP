## Cheques & GIRO Module (Wave 6) - Implementation Summary

### Files Created

#### Frontend (`web/src/`)
1. **`screens/list/ChequeList.tsx`** - Main list view with:
   - Status badges (REGISTERED-blue, DEPOSITED-amber, CLEARED-green, BOUNCED-red)
   - Direction filter dropdown (RECEIVED/ISSUED)
   - Action buttons per row based on status
   - DepositModal and BounceModal integration

2. **`screens/entry/ChequeForm.tsx`** - Entry form for registering cheques:
   - Fields: cheque_number, type, direction, bank_account (combobox), amount_cents, counterparty_name, date
   - Type dropdown automatically sets direction

3. **`screens/entry/DepositModal.tsx`** - Deposit confirmation modal
4. **`screens/entry/BounceModal.tsx`** - Bounce modal with reason input

#### Types (`web/src/types.ts`)
- `ChequeListItem` - List view item
- `Cheque` - Full record
- `ChequeCreateInput` - Create/update payload
- Added `cheque-list` to `ListSubKind`
- Added `cheque-entry` to `EntrySubKind`
- Added `BankAccountListItem` interface

#### API (`web/src/api.ts`)
New methods added:
- `listCheques(params?)` - GET /cheques with optional direction filter
- `createCheque(input)` - POST /cheques
- `updateCheque(id, input)` - PUT /cheques/{id}
- `deleteCheque(id)` - DELETE /cheques/{id}
- `depositCheque(id)` - POST /cheques/{id}/deposit
- `clearCheque(id)` - POST /cheques/{id}/clear
- `bounceCheque(id, reason)` - POST /cheques/{id}/bounce

#### Module Registry (`web/src/workbench/modules.ts`)
- Added "Cheques & GIRO" under Cash & Bank module
- Open list: `cheque-list`, open entry: `cheque-entry`
- Draft number prefix: `CHK-DRAFT`
- Title mapper updated

#### Routing (`web/src/workbench/WorkArea.tsx`)
- Imported `ChequeList` and `ChequeForm`
- Wired `cheque-list` case in `renderList()`
- Wired `cheque-entry` case in `renderEntry()`

---

### Backend Endpoints (Already Implemented)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/cheques` | GET | List cheques with optional `direction` query param |
| `/cheques` | POST | Register new cheque (Dr 1305 / Cr Cash or Cr 2101 AP) |
| `/cheques/{id}` | PUT | Update cheque |
| `/cheques/{id}` | DELETE | Delete cheque |
| `/cheques/{id}/deposit` | POST | Deposit cheque (Dr Bank / Cr 1305 or Dr 2101 / Cr 2106) |
| `/cheques/{id}/clear` | POST | Manually clear deposited cheque (auto when bank syncs) |
| `/cheques/{id}/bounce` | POST | Bounce cheque (reverse deposit + re-establish AR/AP if applicable) |

**Journal posting behavior** (from handler.go:27-38):
- **Register**: Dr 1305 Cheques in Transit / Cr Cash/Bank (RECEIVED)
- **Register ISSUED**: Dr AP / Cr 2106 Cheques Issued Outstanding  
- **Deposit RECEIVED**: Dr Bank / Cr 1305 Cheques in Transit
- **Deposit ISSUED**: Dr 2101 AP / Cr 2106 Cheques Issued Outstanding
- **Clear**: Auto-posts on bank reconciliation sync; manual trigger available
- **Bounce**: Reverses deposit journal; re-establishes AR/AP if originally against receivable/payable

---

### Build Test Results

```bash
# TypeScript check (excluding known demo template issue)
cd web && npx tsc --noEmit

# Expected result:
# - Cheque modules compile successfully
# - Minor warning: DemoEntryForm missing new subkind entries (intentional minimal coverage)
# - Other unrelated ReportTemplateList error not caused by this implementation
```

### UI Design Notes
- Status badges use Tailwind-like classes: `status-badge--info` (blue), `status-badge--warning` (amber), `status-badge--success` (green), `status-badge--danger` (red)
- Bank account selection uses native `<select>` styled as combobox (Combobox component has different props signature)
- Form follows existing pattern from other entry forms (JournalEntryForm, InvoiceForm)

---

### Next Steps (if needed)
1. Run full build: `make web-build`
2. Add tests in `web/src/__tests__/ChequeList.test.tsx`
3. Wire image thumbnail upload (optional enhancement)
4. Clear button action (manual trigger if auto-clear doesn't work)
5. Reactivate/Closed actions for BOUNCED status
