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

## Financial Command Contract

Request headers:

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
