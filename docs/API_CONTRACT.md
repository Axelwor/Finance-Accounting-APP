# MVP 1 API Contract

**Version:** 0.2.0  
**Status:** Synced with implementation (150+ endpoints)  
**Owner:** Backend + Frontend

## Rules

- Prefix: `/api/v1`.
- All tenant-scoped requests derive tenant from authenticated context, never from a client-supplied tenant ID.
- Financial commands require `Idempotency-Key` (UUID) and return the original result on an identical retry.
- Reusing a key with a different payload returns `IDEMPOTENCY_KEY_REUSE`.
- Posted journal evidence is immutable; corrections use command endpoints, not generic update/delete.
- Errors return `{code, message, details, request_id}`.

## MVP Endpoints

| Method | Path | Purpose |
|---|---|---|
| POST | `/auth/register` | Create user |
| POST | `/auth/login` | Create access/refresh session |
| POST | `/auth/refresh` | Rotate refresh token |
| POST | `/auth/logout` | Revoke refresh session |
| POST | `/tenants` | Create tenant and owner membership |
| GET | `/me` | Current user and tenant context |
| GET/POST | `/accounts` | List/create COA account |
| POST | `/accounts/{id}/deactivate` | Deactivate account |
| GET/POST | `/categories` | List/create UI category mapping |
| POST | `/opening-balances` | Post opening balance command |
| POST | `/cash-in` | Post cash/bank receipt |
| POST | `/cash-out` | Post cash/bank payment |
| POST | `/transfers` | Transfer between CASH/BANK accounts |
| POST | `/journal-entries/{id}/reverse` | Atomic void + reversal |
| GET | `/reports/profit-loss` | Current-period P&L |
| GET | `/reports/balance-sheet` | Balance sheet |
| GET | `/reports/cash-flow` | Basic cash flow |
| GET | `/reports/trial-balance` | Trial balance validation/report |
| GET | `/healthz` | Service health |
| GET/POST | `/customers` | List/create customer master data |
| GET | `/customers/{id}` | Get one customer |
| POST | `/customers/{id}/deactivate` | Deactivate customer |
| GET/POST | `/payment-terms` | List/create payment terms |
| GET/POST | `/items` | List/create item (goods/service) master data |
| POST | `/items/{id}/deactivate` | Deactivate item |
| GET/POST | `/items/{id}/prices` | List/create item price list |
| POST | `/quotations` | Create sales quotation (SQ) with lines |
| GET | `/quotations` | List quotations (optional `?status=`) |
| GET | `/quotations/{id}` | Get quotation with lines |
| POST | `/quotations/{id}/send` | DRAFT → SENT |
| POST | `/quotations/{id}/cancel` | DRAFT/SENT → CANCELLED |
| POST | `/quotations/{id}/mark-expired` | → EXPIRED |
| POST | `/sales-orders` | Create sales order (SO) with lines |
| GET | `/sales-orders` | List sales orders (optional `?status=`) |
| GET | `/sales-orders/{id}` | Get sales order with lines + down payments |
| POST | `/sales-orders/{id}/cancel` | CONFIRMED → CANCELLED (only when no DP) |
| POST | `/sales-orders/{id}/down-payments` | Receive down payment (DP) — posts journal |
| GET | `/sales-orders/{id}/down-payments` | List down payments for an SO |
| POST | `/down-payments/{id}/refund` | Refund a DP — posts reversal journal |
| POST | `/delivery-orders` | Create delivery order (DO) — posts COGS journal |
| GET | `/delivery-orders` | List delivery orders (optional `?status=`) |
| GET | `/delivery-orders/{id}` | Get delivery order with lines |
| POST | `/invoices` | Create invoice (INV) — posts revenue + DP realization |
| GET | `/invoices` | List invoices (optional `?status=`) |
| GET | `/invoices/{id}` | Get invoice with lines |
| POST | `/invoices/{id}/payments` | Receive customer payment — posts Dr Cash / Cr AR |
| GET | `/invoices/{id}/payments` | List payments for an invoice |
| POST | `/credit-notes` | Create credit note (CN) — posts return + COGS reversal |
| GET | `/credit-notes` | List credit notes (optional `?status=`) |
| GET | `/credit-notes/{id}` | Get credit note with lines |
| POST | `/suppliers` | Create supplier |
| GET | `/suppliers` | List suppliers |
| GET | `/suppliers/{id}` | Get supplier |
| POST | `/suppliers/{id}/deactivate` | Deactivate supplier |
| POST | `/purchase-orders` | Create purchase order (PO) — no journal |
| GET | `/purchase-orders` | List purchase orders (optional `?status=`) |
| GET | `/purchase-orders/{id}` | Get purchase order with lines |
| POST | `/goods-received-notes` | Create GRN — posts Dr Inventory / Cr Accrued Payables |
| GET | `/goods-received-notes` | List GRNs (optional `?status=`) |
| GET | `/goods-received-notes/{id}` | Get GRN with lines |

### Sales Quotation Notes

- SQ is a **commitment only** — it does **not** post any journal (per `ACCOUNTING_ENGINE.md`). Journals are created only at the following steps (DP / DO / INV).
- `POST /quotations` inserts the quotation header + lines in one transaction and allocates `SQ-{YYYY}-{seq}` from `document_numbering`.
- Line total: `line_total_cents = round(qty * unit_price_cents) − discount_cents`; header `total_cents` is the sum of lines.
- Status machine: `DRAFT → SENT → CONVERTED` (via later SO conversion, not yet exposed) or `EXPIRED` / `CANCELLED`.
- Customer must belong to the tenant; at least one line required; `qty > 0`, `unit_price_cents ≥ 0`, `tax_rate` in 0..100.

### Sales Order + Down Payment Notes

- **SO is a commitment only** — it does **not** post any journal (per `ACCOUNTING_ENGINE.md` §7). Journals are posted only at DP, DO, and INV steps.
- `POST /sales-orders` inserts the order header + lines in one transaction and allocates `SO-{YYYY}-{seq}` from `document_numbering`. When `quotation_id` is provided, the linked quotation is marked `CONVERTED`.
- SO status machine: `CONFIRMED → CANCELLED` (only when `dp_received_cents = 0`) or `CONFIRMED → CLOSED` (via later INV).
- **DP posts a journal**: `Dr Cash/Bank / Cr 2201 Customer Deposit` (intent_type `SALES_DOWN_PAYMENT`). DP requires `Idempotency-Key` header.
- DP validation: `dp_received_cents + amount_cents ≤ total_cents` (rejected with `DP_EXCEEDS_ORDER` when exceeded). Multiple DPs can be allocated to one SO.
- **DP refund** (`POST /down-payments/{id}/refund`): reverses the original DP journal (intent_type `SALES_DP_REFUND`), marks the original journal `VOID`, sets the DP status to `REFUNDED`, and reduces `dp_received_cents`.
- The 2201 account is resolved by code; `seed.go` provisions it for new tenants and migration 000006 seeds it for existing tenants.

### Delivery Order Notes

- **DO posts a COGS journal**: `Dr 5101 COGS / Cr 1301 Inventory` per item delivered (intent_type `SALES_DELIVERY`). Each line carries its own `cogs_account_id` and `inventory_account_id` resolved from the item's account defaults.
- DO requires `Idempotency-Key` header. The journal is posted in the same transaction as the delivery header + lines, with hash-chain and outbox event.
- Only **goods** items can be delivered — services are rejected. Each item must have `inventory_account_id` and `cogs_account_id` set.
- An `inventory_movements` row is recorded per line (movement_type `DO`, qty negative = stock out).
- The linked sales order's status is set to `CLOSED` after delivery.
- `POST /delivery-orders` allocates `DO-{YYYY}-{seq}` from `document_numbering`.
- DO supports partial delivery (multiple DOs per SO); the SO's `delivered_qty` column tracks cumulative delivery per line.

### Invoice Notes

- **INV posts two journals**:
  1. Revenue: `Dr 1201 AR / Cr 4101 Revenue` (intent_type `SALES_INVOICE`)
  2. DP realization (when the linked SO has `dp_received_cents > 0`): `Dr 2201 Customer Deposit / Cr 1201 AR` (intent_type `SALES_DP_REALIZE`)
- INV requires `Idempotency-Key` header. Both journals are posted in the same transaction with hash-chain and outbox events.
- `dp_applied_cents` is clamped to `min(dp_received_cents, total_cents)`; `receivable_cents = total_cents - dp_applied_cents`.
- Accounts 1201 (AR) and 4101 (Revenue) are resolved by code from the seeded COA. 2201 (Customer Deposit) is resolved by code for the DP realization.
- `POST /invoices` allocates `INV-{YYYY}-{seq}` from `document_numbering`.
- When `sales_order_id` is provided, the SO's `dp_received_cents` is read to determine the DP realization amount.

### Invoice Payment (Pelunasan) Notes

- **Payment posts a journal**: `Dr Cash/Bank / Cr 1201 AR` (intent_type `SALES_RECEIPT`). When the payment exceeds the remaining `receivable_cents`, the excess is credited to `2402 Customer Overpayment` in the same journal.
- Payment requires `Idempotency-Key` header. The journal carries hash-chain, outbox event, and idempotency.
- `ar_applied_cents` is clamped to `min(amount_cents, receivable_cents)`; `overpayment_cents = amount_cents - ar_applied_cents`.
- After payment, the invoice's `receivable_cents` is reduced and `status` becomes `PARTIALLY_PAID` or `PAID`.
- Accounts 1201 (AR) and 2402 (Customer Overpayment) are resolved by code from the seeded COA. `seed.go` provisions 2402 for new tenants; migration 000009 seeds it for existing tenants.

### Credit Note Notes

- **CN posts two journals** in one transaction:
  1. Revenue reversal: `Dr 4201 Sales Returns / Cr 1201 AR` (intent_type `SALES_RETURN`). When `refund_method=refund`, the credit is to Cash/Bank instead of AR.
  2. COGS reversal: `Dr 1301 Inventory / Cr 5101 COGS` (intent_type `COGS_REVERSAL`) per returned item.
- `refund_method` controls how the customer is compensated: `deduct` (reduce AR on the linked invoice), `refund` (cash refund), or `credit_balance` (hold as customer credit).
- After CN with `deduct`, the invoice's `receivable_cents` is increased and `status` may revert from `PAID` to `PARTIALLY_PAID`.
- Inventory movements (movement_type `SALES_RETURN`, qty positive = stock in) are recorded per line.
- Accounts 4201/1201/1301/5101 are resolved by code from the seeded COA. `seed.go` provisions 4201 for new tenants; migration 000010 seeds it for existing tenants.
- `POST /credit-notes` allocates `CN-{YYYY}-{seq}` from `document_numbering`. Requires `Idempotency-Key` header.

## Financial Command Contract

```text
Authorization: Bearer <access-token>
Idempotency-Key: <uuid>
```

Example `POST /cash-in`:

```json
{
  "account_id": 1101,
  "category_id": 1,
  "amount_cents": 500000,
  "entry_date": "2026-08-06",
  "description": "Penjualan tunai"
}
```

Response:

```json
{
  "id": "jrn_123",
  "number": "JRN-2026-000001",
  "status": "POSTED",
  "entry_date": "2026-08-06",
  "total_debit_cents": 500000,
  "total_credit_cents": 500000,
  "replayed": false,
  "request_id": "req_123"
}
```

## Extended Endpoints (Sprint 5–7)

These endpoints were implemented after the MVP contract and are documented here
to keep the contract in sync with the code (m-019). All are under `/api/v1`,
tenant-scoped via the authenticated context, and follow the rules above.
Financial commands require `Idempotency-Key`.

### Auth — Multi-Tenant & 2FA

| Method | Path | Purpose |
|---|---|---|
| POST | `/auth/switch-tenant` | Rotate tokens to another tenant the user belongs to |
| POST | `/auth/2fa/setup` | Generate TOTP secret + provisioning URI (authenticated) |
| POST | `/auth/2fa/verify` | Enable 2FA after proving possession of the secret |
| POST | `/auth/2fa/disable` | Disable 2FA (requires a valid code) |
| GET | `/tenants` | List tenants the user belongs to |
| POST | `/tenants/new` | Create an additional tenant for the user |
| GET | `/tenants/me` | Current active tenant (404 when onboarding pending) |

### Customers & AR Sub-Ledger

| Method | Path | Purpose |
|---|---|---|
| GET | `/customers/ar-balances` | Per-customer outstanding AR + GL reconciliation (1201) |
| GET | `/customers/{id}/ar-balance` | Single customer outstanding AR |
| GET | `/customers/{id}/statement` | Customer AR statement over a date range |
| GET | `/aging/ar` | AR aging buckets (current / 1-30 / 31-60 / 61-90 / 90+) |
| GET | `/aging/ap` | AP aging buckets |
| GET | `/accounts/export` | COA export as CSV |

### Purchase (full flow)

| Method | Path | Purpose |
|---|---|---|
| GET/POST | `/suppliers` | List/create supplier master data |
| GET | `/suppliers/{id}` | Get one supplier |
| POST | `/suppliers/{id}/deactivate` | Deactivate supplier |
| GET/POST | `/purchase-orders` | List/create purchase order (PO) |
| GET | `/purchase-orders/{id}` | Get PO with lines |
| GET/POST | `/goods-received-notes` | List/create GRN — posts inventory + accrual journal |
| GET | `/goods-received-notes/{id}` | Get GRN with lines |
| GET/POST | `/purchase-returns` | List/create purchase return — reversal journal |
| GET | `/purchase-returns/{id}` | Get purchase return |

### Inventory

| Method | Path | Purpose |
|---|---|---|
| GET/POST | `/stock-opnames` | List/create stock opname |
| GET | `/stock-opnames/{id}` | Get stock opname |
| POST | `/stock-opnames/{id}/approve` | Approve opname — posts adjustment journal |
| GET/POST | `/stock-transfers` | List/create stock transfer (multi-warehouse) |
| GET | `/stock-transfers/{id}` | Get stock transfer |
| GET/POST | `/bill-of-materials` | List/create BOM |
| GET | `/bill-of-materials/{id}` | Get BOM with lines |
| GET/POST | `/production-jobs` | List/create production job |
| GET | `/production-jobs/{id}` | Get production job |
| POST | `/production-jobs/{id}/costs` | Add job cost (material/labor/overhead) |
| POST | `/production-jobs/{id}/complete` | Complete job — posts finished goods + variance |
| POST | `/overhead-variance` | Reconcile applied overhead (4902) at period close |

### Tax

| Method | Path | Purpose |
|---|---|---|
| GET | `/ppn/summary` | PPN output vs input summary |
| GET | `/ppn/reconciliation` | PPN reconciliation detail |
| POST | `/ppn/reconcile` | Mark PPN reconciled |
| POST | `/pph-final/calculate` | Calculate PPh Final UMKM |
| POST | `/pph-final/pay` | Pay PPh Final |
| POST | `/ecl/calculate` | Calculate ECL provisioning — posts journal |
| POST | `/ecl/write-off` | Write off a receivable |
| POST | `/deferred-tax/calculate` | Calculate deferred tax |
| POST | `/tax/withholding/*` | PPh 21/22/23/26 withholding (via pph module) |

### Fixed Assets & Maintenance

| Method | Path | Purpose |
|---|---|---|
| GET/POST | `/fixed-assets` | List/register fixed asset |
| GET | `/fixed-assets/{id}` | Get asset |
| GET | `/assets/register` | Asset register with NBV + totals |
| POST | `/fixed-assets/{id}/depreciate` | Post depreciation |
| POST | `/fixed-assets/{id}/revalue` | Revalue asset |
| POST | `/fixed-assets/{id}/dispose` | Dispose asset (gain/loss) |
| POST | `/fixed-assets/{id}/impair` | Impair asset |
| GET/POST | `/asset-maintenance` | List/create maintenance record |
| GET | `/asset-maintenance/upcoming` | Upcoming maintenance (horizon days) |

### Lease (PSAK 73)

| Method | Path | Purpose |
|---|---|---|
| GET/POST | `/lease-contracts` | List/create lease contract — posts commencement |
| GET | `/lease-contracts/{id}` | Get lease with schedule |
| POST | `/lease-contracts/{id}/payments/{payment_no}/post` | Post lease payment |
| POST | `/lease-contracts/{id}/depreciate` | Post RoU depreciation |
| GET | `/lease-contracts/{id}/depreciation-log` | Depreciation history |
| POST | `/lease-contracts/{id}/modify` | Re-measure lease to new PV (m-014) |
| POST | `/lease-contracts/{id}/terminate` | Derecognise lease (m-014) |
| GET/POST | `/entity-hierarchy` | List/create consolidation hierarchy |
| GET | `/consolidated-reports/trial-balance` | Consolidated trial balance |
| GET | `/consolidated-reports/profit-loss` | Consolidated P&L |

### Banking, Recurring, Budget, Notes

| Method | Path | Purpose |
|---|---|---|
| GET/POST | `/bank-statements` | List/upload bank statement |
| GET | `/bank-statements/{id}` | Get statement |
| POST | `/bank-statements/{id}/reconcile` | Reconcile statement |
| GET | `/bank-reconciliations/{id}` | Get reconciliation |
| POST | `/bank-reconciliations/{id}/match` | Match a line |
| POST | `/bank-reconciliations/{id}/unmatch` | Unmatch a line |
| POST | `/bank-reconciliations/{id}/complete` | Complete reconciliation |
| GET/POST | `/recurring` | List/create recurring transaction |
| GET | `/recurring/{id}` | Get recurring |
| PUT | `/recurring/{id}` | Update recurring |
| DELETE | `/recurring/{id}` | Deactivate recurring |
| POST | `/recurring/{id}/post` | Post a recurring occurrence |
| GET/POST | `/budgets` | List/create budget |
| GET | `/budgets/{id}` | Get budget |
| GET | `/budgets/{id}/vs-actual` | Budget vs actual variance |
| GET/POST | `/dimensions` | List/create dimension |
| POST | `/journal-lines/{id}/dimensions` | Tag a journal line with dimensions |
| GET/POST | `/report-frameworks` | List/create report framework |
| GET/POST | `/financial-notes` | List/create financial note |
| GET | `/financial-notes/{id}` | Get note |
| PUT | `/financial-notes/{id}` | Update note |
| DELETE | `/financial-notes/{id}` | Delete note |
| GET | `/reminders/due-dates` | Due-date reminders |

### Petty Cash, Cheques, Approval, Cost Center, Email

| Method | Path | Purpose |
|---|---|---|
| GET/POST | `/petty-cash/funds` | List/create petty cash fund |
| POST | `/petty-cash/funds/{id}/replenish` | Replenish fund to imprest amount |
| GET/POST | `/petty-cash/vouchers` | List/create petty cash voucher |
| GET/POST | `/cheques` | List/create cheque/GIRO |
| GET | `/cheques/{id}` | Get cheque |
| POST | `/cheques/{id}/deposit` | Deposit a received cheque |
| POST | `/cheques/{id}/clear` | Mark cheque cleared |
| POST | `/cheques/{id}/bounce` | Mark cheque bounced |
| GET/POST | `/approval-workflows` | List/create approval workflow rule |
| DELETE | `/approval-workflows/{id}` | Deactivate workflow rule |
| GET/POST | `/approval-requests` | List/submit approval request |
| GET | `/approval-requests/{id}` | Get approval request |
| POST | `/approval-requests/{id}/approve` | Approve |
| POST | `/approval-requests/{id}/reject` | Reject |
| GET/POST | `/cost-centers` | List/create cost center |
| GET | `/cost-centers/{id}` | Get cost center |
| POST | `/cost-centers/{id}/allocations` | Create allocation rule |
| GET | `/cost-centers/{id}/allocations` | List allocations |
| GET/POST | `/email-templates` | List/create email template |
| GET/POST | `/email-queue` | List/queue email |
| POST | `/email-queue/{id}/send` | Send a queued email |

### Cash Flow Forecast

| Method | Path | Purpose |
|---|---|---|
| GET | `/forecast/cash-flow` | Daily cash flow forecast (horizon) |

### Reporting & Dashboard

| Method | Path | Purpose |
|---|---|---|
| GET | `/reports/quotation-stats` | Quotation funnel / conversion rate |
| GET/POST | `/reports/templates` | List/create report template |
| GET | `/reports/templates/{id}` | Get template |
| PUT | `/reports/templates/{id}` | Update template |
| DELETE | `/reports/templates/{id}` | Delete template |
| POST | `/reports/templates/{id}/render` | Render template (`?format=html\|pdf`) |
| GET | `/reports/trial-balance/export` | Export report (`?format=pdf\|xlsx`) |
| GET | `/reports/profit-loss/export` | Export report |
| GET | `/reports/balance-sheet/export` | Export report |
| GET | `/reports/cash-flow/export` | Export report |
| GET/PUT | `/dashboard/layout` | Get/save per-user dashboard layout |
| GET/POST | `/dashboard/widgets` | List available widgets / add widget |
| PUT/DELETE | `/dashboard/widgets/{id}` | Update/remove widget |
| GET | `/dashboard/widgets/{id}/data` | Fetch widget data |
| GET | `/healthz/detail` | Component health (DB + NextReport) |

## Non-MVP Endpoint Rules

All feature endpoints are now implemented and documented above. Any new
endpoint must be added to this contract before or alongside its implementation
per AGENTS.md (shared API contract requires a coordination task).
