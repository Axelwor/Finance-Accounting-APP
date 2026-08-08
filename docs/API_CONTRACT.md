# MVP 1 API Contract

**Version:** 0.1.0  
**Status:** Draft  
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

## Non-MVP Endpoint Rules

Sales, purchasing, inventory, tax, fixed assets, recurring, approval, attachment, and integration endpoints are not part of this contract until their feature task is claimed and its contract is approved.
