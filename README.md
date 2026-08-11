# Finance Accounting App

Aplikasi akuntansi double-entry lengkap dengan arsitektur modular monolith Go + React/Vite SPA. Semua angka disimpan sebagai integer sen (tidak ada float), semua jurnal immutable dengan hash chain anti-tamper, dan multi-tenant dengan Row-Level Security.

**Live:** https://accounting.tikuma.net/

## Status Proyek

| Metrik | Nilai |
|---|---|
| Backend Go files | 106 source + 50 test files |
| Frontend screens | 73 TSX components |
| Database migrations | 44 (000001 s/d 000044) |
| API endpoints | 150+ (lihat `docs/API_CONTRACT.md`) |
| Test coverage | 35 packages pass, ~500 test functions |
| Audit findings | 128/128 resolved (lihat `audit-report.md`) |

## Fitur Utama

### Akuntansi Double-Entry
- **Journal Engine** — pure accounting engine (`backend/internal/accounting/`), semua angka integer sen, balance-check wajib, hash chain anti-tamper per tenant
- **Cash & Bank** — cash-in, cash-out, transfer antar CASH/BANK, opening balance
- **Manual Journal** — jurnal manual multi-line dengan approval
- **Period Close** — tutup buku dengan closing entries (revenue/expense → retained earnings), unlock dengan reversal
- **Reports** — Trial Balance, P&L, Balance Sheet, Cash Flow (operating/investing/financing) dengan export PDF/XLSX, framework EMKM/ETAP/SAK Umum, filter dimension

### Penjualan (SQ → SO → DP → DO → INV → Pelunasan → CN)
- Quotation → Sales Order → Down Payment → Delivery Order → Invoice → Payment → Credit Note
- Setiap posting menghasilkan jurnal balanced dengan hash chain + idempotency
- PPN 11% otomatis (Cr 2202 VAT Payable) saat invoice
- AR sub-ledger per customer + rekonsiliasi GL

### Pembelian (PO → GRN → SI → SP → Return)
- Purchase Order → Goods Received Note → Supplier Invoice → Supplier Payment → Purchase Return
- GRN posting: Dr 1301 Inventory / Cr 2105 Uninvoiced Payables
- PPN Masukan (Dr 1203) saat supplier invoice

### Persediaan & Produksi
- Stock balance per item per warehouse (multi-warehouse)
- Costing FIFO + Moving Average dengan cost layers
- Stock opname dengan approval, stock transfer antar warehouse
- Bill of Materials, production jobs, overhead variance

### Aset Tetap & Lease
- Akuisisi, depresiasi (straight-line, declining balance, units-of-production), disposal gain/loss, revaluation, impairment
- Asset register dengan NBV + maintenance tracking
- Lease PSAK 73: commencement (Dr 1701 / Cr 2301), amortisasi, RoU depreciation, modification, termination

### Pajak
- PPN keluaran/masukan + rekonsiliasi
- PPh Final UMKM (0.5%/0.75%), PPh 21/22/23/26 withholding
- ECL (Expected Credit Loss) provisioning + write-off
- Deferred tax

### Keuangan & Operasional
- Multi-tenant (satu email bisa punya banyak tenant, switcher di UI)
- 2FA TOTP (RFC 6238)
- RBAC (owner/admin/accountant/manager/staff/viewer)
- Approval workflow untuk invoice di atas threshold
- Giro & Cheque management (deposit/clear/bounce)
- Petty cash (imprest system)
- Recurring transactions (daily/weekly/monthly/quarterly/yearly)
- Cash flow forecasting
- Budget vs actual variance
- Bank reconciliation
- Cost centers + inter-company elimination (SALE/PURCHASE/LOAN/INTEREST/DIVIDEND)
- Email templates + queue

### Reporting & Dashboard
- Report template designer (YAML) + render PDF/HTML via NextReport sidecar
- Dashboard per-user dengan widgets (KPI, charts, aging, alerts)
- Consolidated reports multi-entitas (PSAK 65)

## Tech Stack

| Layer | Technology |
|---|---|
| Backend | Go 1.25, chi router, pgx/v5, sqlc, golang-jwt, bcrypt |
| Database | PostgreSQL 16 (RLS FORCE per tenant, deferred balance constraint) |
| Frontend | React 19, TypeScript, Vite/Rolldown |
| Report Rendering | NextReport sidecar (zero-dep Node, zero-dep PDF writer) |
| Reverse Proxy | Caddy 2 (gzip, Let's Encrypt) |
| Deploy | Docker Compose (5 containers) di VPS, Cloudflare proxy |

## Development

### Prerequisites
- Go 1.25+
- Node.js 20+
- PostgreSQL 16 (untuk integration test)

### Setup & Run
```bash
make backend-build          # build backend
make backend-test           # run backend tests
make web-install            # npm install frontend
make web-build              # build frontend production
make test                   # all backend tests
make test-integration       # integration tests (butuh DATABASE_URL)
make db-migrate             # apply migrations (butuh DATABASE_URL)
```

### Run Full Stack
```bash
docker compose up -d --build
# API: http://localhost:8080 (via caddy: http://localhost)
# Frontend: http://localhost
```

### Verification Gate
```bash
make fmt && make lint && make test && make web-build
```

## Struktur Proyek

```
├── backend/
│   ├── cmd/api/main.go          # Route registration (150+ endpoints)
│   ├── internal/
│   │   ├── accounting/          # Pure accounting engine (no IO)
│   │   ├── auth/                # Auth, JWT, RBAC, 2FA TOTP
│   │   ├── approval/            # Approval workflow engine + gate
│   │   ├── budget/              # Budget vs actual, dimensions, frameworks
│   │   ├── cash/                # Cash-in/out/transfer journal posting
│   │   ├── coa/                 # Chart of accounts, categories, CSV export
│   │   ├── costing/             # FIFO + moving average cost layers
│   │   ├── customer/            # Customer master + AR sub-ledger
│   │   ├── db/                  # pgx helpers, sqlc generated code
│   │   ├── forecast/            # Cash flow forecasting
│   │   ├── httperr/             # Unified error format
│   │   ├── inventory/           # Stock opname, transfer, movements
│   │   ├── lease/               # PSAK 73 lease + consolidation
│   │   ├── middleware/          # Recover, logger, CORS, timeout, rate limit
│   │   ├── period/              # Period close/unlock + closing entries
│   │   ├── production/          # BOM, production jobs, overhead variance
│   │   ├── purchase/            # PO, GRN, supplier invoice/payment/return
│   │   ├── recurring/           # Recurring transactions
│   │   ├── reporting/           # Reports + export PDF/XLSX
│   │   ├── reports/             # Report template CRUD + NextReport render
│   │   ├── sales/               # SQ/SO/DP/DO/INV/Payment/CN
│   │   ├── tax/                 # PPN, PPh, ECL, deferred tax
│   │   └── warehouse/           # Multi-warehouse master
│   ├── migrations/              # 44 migrations (golang-migrate format)
│   └── queries/                 # sqlc SQL queries
├── web/
│   ├── src/
│   │   ├── screens/list/        # 35 list screens
│   │   ├── screens/entry/       # 27 form screens
│   │   ├── workbench/           # Workbench shell, modules, routing
│   │   ├── styles/              # 6 modular CSS files
│   │   └── api.ts + types.ts    # API client + TypeScript types
│   └── Dockerfile
├── nextreport/                  # Report rendering sidecar
├── docker-compose.yml           # 5 services orchestration
├── Caddyfile                    # Reverse proxy config
├── deploy.sh                    # One-command deploy script
├── audit-report.md              # Audit report (128 findings, all resolved)
├── implementation-tracker.md    # Implementation tracker
└── docs/
    ├── API_CONTRACT.md          # API contract (150+ endpoints)
    ├── CHANGELOG.md
    ├── TASK_LEDGER.md
    ├── DEPLOYMENT.md            # Deployment guide
    └── UI_CONTRACT.md
```

## Deployment

Lihat panduan lengkap di `docs/DEPLOYMENT.md`.

### Quick Deploy
```bash
# Deploy code baru (dari lokal, butuh git push dulu)
./deploy.sh

# Deploy + apply migration
./deploy.sh 000044

# Manual
ssh finance-accounting-vps 'cd ~/Finance-Accounting-APP && git pull origin main && docker compose up -d --build api web'
```

### Containers
| Container | Fungsi |
|---|---|
| finance-db | PostgreSQL 16 |
| finance-api | Go API (:8080) |
| finance-web | React SPA |
| finance-nextreport | Report renderer (:3100) |
| finance-caddy | Reverse proxy (:80/:443) |

## Security

- JWT HS256 dengan secret 32+ karakter (fail-fast jika kosong)
- 2FA TOTP (RFC 6238) per user
- RBAC: owner/admin/accountant/manager/staff/viewer
- Row-Level Security FORCE di semua tabel tenant-scoped
- Rate limiting login (5 req/menit per IP)
- Idempotency-Key wajib untuk semua financial commands (409 jika payload berbeda)
- Posted journals immutable (DB trigger), koreksi hanya via reversal
- Hash chain anti-tamper per tenant (SHA-256)
- CORS + recover + timeout middleware

## Specifications

- `PRD.md` — Product scope & roadmap
- `ACCOUNTING_ENGINE.md` — Accounting invariants & posting rules (source of truth)
- `DATA_MODEL.md` — Persistence constraints
- `ARCHITECTURE.md` — Technical architecture & RBAC
- `USER_STORIES.md` — Acceptance criteria per story
- `GLOSSARY.md` — Terminology
- `GAP_ANALYSIS.md` — Competitive analysis
- `docs/API_CONTRACT.md` — API contract (150+ endpoints)
- `audit-report.md` — Audit report (128 findings, all resolved)

## AI Agent Workflow

Baca `AGENTS.md`, claim task di `docs/TASK_LEDGER.md`, kerjakan hanya owned paths, tambahkan tests, update `docs/CHANGELOG.md`, dan jalankan verification gate sebelum menyelesaikan task.
