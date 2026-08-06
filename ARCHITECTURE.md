# Arsitektur Teknis
## Software Pembukuan Mudah dengan Mesin Akuntansi Standar IFRS/PSAK

**Lampiran PRD** — Software Pembukuan Mudah dengan Mesin Akuntansi Standar IFRS/PSAK  
**Versi:** 3.4 — Review  
**Status:** Review — acuan implementasi sementara  
**Tanggal:** 2026-08-06  
**Owner:** Engineering + Product  
**Normative:** Tidak; implementation design yang tunduk pada Engine/Data Model

### Riwayat Revisi
| Versi | Tanggal | Perubahan |
|---|---|---|
| 1.0 | 2026-08-05 | Draft awal (Node/NestJS, React SPA) |
| 2.0 | 2026-08-05 | Migrasi ke **Go (Golang)**; hapus Next.js; frontend SPA statis |
| 2.1 | 2026-08-05 | API surface, ERD, indexing, testing, error handling, env, backup/DR, SLO |
| 3.0 | 2026-08-05 | **Final**: alur end-to-end, contoh kode (Go/sqlc/SQL), ADR, daftar periksa finalisasi |
| 3.1 | 2026-08-06 | Sinkronisasi §6 Database Schema dengan DATA_MODEL v1.5 (tabel baru: user_tokens, bank_reconciliations, leases, write_offs, recurring_instances, dll) |
| 3.2 | 2026-08-06 | Audit lintas dokumen: sinkronisasi DATA_MODEL v1.6, reversal metadata, roadmap fase, status API, dan klasifikasi akun |
| 3.3 | 2026-08-06 | Sinkronisasi DATA_MODEL v1.8; kontrak periode/idempotensi, recurring archive, lifecycle matrix, hash-chain serialization, dan governance dokumen |

## 1. Tujuan & Prinsip

Dokumen ini mendefinisikan arsitektur teknis untuk membangun produk: **web app pembukuan mudah dengan accounting engine IFRS/PSAK di belakang layar**.

### Prinsip Arsitektur
| Prinsip | Penjelasan |
|---|---|
| **Bahasa cepat untuk olah data** | Backend & accounting engine memakai **Go (Golang)** — kompilasi ke native binary, sangat cepat untuk perhitungan akuntansi & agregasi laporan |
| **Mudah di-deploy** | Go menghasilkan **satu binary statis** → image Docker kecil, deploy ke mana saja tanpa runtime tambahan |
| **Mudah dikembangkan** | Struktur package sederhana, satu bahasa di backend, tooling bawaan (go test, gofmt, vet) |
| **Modular Monolith dulu** | Satu aplikasi dengan batas modul jelas; mudah dievolusi ke microservices bila perlu |
| **Accounting Engine = pure package** | Terpisah dari IO (DB/HTTP), agar akurasi double-entry teruji matematis & portabel |
| **Event-driven di titik integrasi** | Perubahan penting (posting, invoice, payment) menerbitkan event → dipakai job queue & integrasi eksternal |
| **Database = source of truth** | PostgreSQL ACID menjamin integritas jurnal; Redis hanya untuk cache/queue |
| **Idempotensi & anti-tamper** | Setiap intent diposting sekali; hash chain jurnal mendeteksi perubahan ilegal |
| **Multi-tenant dengan isolasi data** | Satu basis data, isolasi per entitas via `tenant_id` + RLS (row-level security) |

### Dokumen Terkait
| Dokumen | Isi |
|---|---|
| [PRD.md](PRD.md) | Kebutuhan produk, persona, roadmap |
| [ACCOUNTING_ENGINE.md](ACCOUNTING_ENGINE.md) | Aturan jurnal & COA — sumber kebenaran akuntansi |
| [GAP_ANALYSIS.md](GAP_ANALYSIS.md) | Perbandingan fitur dengan kompetitor |
| [DATA_MODEL.md](DATA_MODEL.md) | Skema tabel, constraint, indeks, dan ERD |
| [GLOSSARY.md](GLOSSARY.md) | Terminologi UI dan akuntansi |
| [USER_STORIES.md](USER_STORIES.md) | Backlog testable dan acceptance criteria |

*(Dokumen ini fokus pada **cara membangun**; aturan akuntansi di ACCOUNTING_ENGINE.md.)*

---

## 2. Diagram Arsitektur

```
┌─────────────────────────────────────────────────────────────────────┐
│                        FRONTEND (React SPA)                         │
│         React + TypeScript (Vite) · Ant Design · TanStack Query     │
│         (SPA statis — tanpa Next.js/SSR, deploy ke CDN/storage)     │
└──────────────────────────────┬──────────────────────────────────────┘
                               │ HTTPS / REST (JSON) + WebSocket
                               ▼
┌─────────────────────────────────────────────────────────────────────┐
│                    BACKEND API (Go / chi + net/http)                │
│                                                                     │
│  ┌──────────────┐  ┌──────────────┐  ┌───────────────────────────┐  │
│  │  Auth Module │  │ API Modules  │  │   Accounting Engine       │  │
│  │ (JWT, OAuth) │  │ (Sales, PO,  │  │   (pure package, tanpa IO)│  │
│  │              │  │  Inventory,  │  │   - double-entry          │  │
│  │              │  │  Report, ...)│  │   - posting, balance check│  │
│  └──────┬───────┘  └──────┬───────┘  └──────────┬────────────────┘  │
│         │                 │                     │                   │
│  ┌──────┴─────────────────┴─────────────────────┴────────────────┐  │
│  │                 Event Bus (Redis pub/sub)                     │  │
│  └──────┬───────────────────────────┬───────────────────────────┘  │
│         ▼                           ▼                              │
│  ┌───────────────┐          ┌────────────────┐                     │
│  │  Job Queue    │          │  Integrations  │                     │
│  │  (asynq+Redis)│          │  (OCR, Bank,   │                     │
│  │               │          │   DJP, e-mail) │                     │
│  └───────────────┘          └────────────────┘                     │
└─────────────────────────────────────────────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────────────┐
│                         DATA LAYER                                  │
│  ┌─────────────────────────────┐  ┌──────────────────────────────┐  │
│  │  PostgreSQL (OLTP, ACID)    │  │  Redis (cache + queue)       │  │
│  │  - jurnal, COA, sub-ledger  │  │  - session cache, rate limit │  │
│  │  - RLS per tenant           │  │  - asynq job queue           │  │
│  └─────────────────────────────┘  └──────────────────────────────┘  │
│  ┌─────────────────────────────┐                                    │
│  │  Object Storage (S3-compat) │  File bukti/attachment + backup   │
│  └─────────────────────────────┘                                    │
└─────────────────────────────────────────────────────────────────────┘

Deployment: 1 binary Go + static frontend · Docker (image ~15MB) · CI/CD GitHub Actions
Monitoring: Sentry · Prometheus+Grafana · Uptime monitor
```

---

## 3. Tech Stack (Finalisasi)

| Lapisan | Pilihan | Alternatif | Alasan |
|---|---|---|---|
| **Frontend** | React + TypeScript (Vite), Ant Design, TanStack Query, Zustand | — | SPA statis; **tidak pakai Next.js** — dashboard app tidak butuh SSR; Vite lebih sederhana & mudah deploy (static hosting/CDN) |
| **Backend** | **Go** (chi + net/http) | Gin, Echo, Fiber | Cepat untuk olah data, satu binary, stdlib matang |
| **Accounting Engine** | **Go pure package** (`internal/accounting`) | Rust (jika butuh performa ekstrem) | Deterministik, sangat cepat, portabel; mudah diuji dengan `go test` |
| **DB Access** | **sqlc** (type-safe SQL → Go) + **pgx** | GORM (ORM) | sqlc memberi kendali penuh SQL, type-safe, tanpa overhead ORM |
| **Database** | PostgreSQL 16 | — | ACID, RLS, JSONB, skema kuat |
| **Cache & Queue** | Redis 7 + **asynq** (Go job queue) | RabbitMQ | Ringan, teruji, API Go yang bersih |
| **Object Storage** | S3-compatible (MinIO lokal, AWS S3 / Cloudflare R2 prod) | — | Attachment & backup |
| **Auth** | golang-jwt (access+refresh), OAuth 2.0 (Google), 2FA (TOTP) | — | Login mudah + keamanan enterprise |
| **OCR (fase 3)** | Google Vision API | Tesseract (lokal) | Akurasi tinggi untuk struk |
| **Job/Worker** | asynq worker (process terpisah) | — | Export besar, posting massal, recurring |
| **Deployment** | Docker (image ~15MB) + cloud (Render/Fly.io/AWS ECS), CI/CD GitHub Actions | Kubernetes (jika sudah besar) | Sangat mudah deploy: binary tunggal |
| **Monitoring** | Sentry, Prometheus, Grafana, Uptime Kuma | — | Observability lengkap |

### Mengapa Go (sesuai kebutuhan: cepat olah data, mudah deploy & develop)
| Kebutuhan | Keunggulan Go |
|---|---|
| **Cepat olah data** | Kompilasi native, tanpa runtime/GC berat; ~10× lebih cepat dari Node untuk perhitungan CPU-bound (engine jurnal, agregasi laporan 100k+ transaksi) |
| **Mudah di-deploy** | `go build` → **satu binary statis** (cross-compile GOOS/GOARCH); image Docker `scratch` ~15MB; tanpa Node/runtime dependency |
| **Mudah dikembangkan** | Struktur package sederhana, tooling bawaan (`go test`, `gofmt`, `go vet`), konvensi ketat, hiring relatif mudah |
| **Konkurensi** | Goroutine + channel — ideal untuk posting massal, rekonsiliasi paralel, worker job |

---

## 4. Backend: Modular Monolith (Go)

### 4.1 Struktur Modul
```
backend/
  ├── cmd/
  │   ├── api/                    # entrypoint HTTP server
  │   └── worker/                 # entrypoint asynq worker
  ├── internal/
  │   ├── accounting/             # ENGINE PURE (double-entry) — tanpa IO
  │   │   ├── engine.go           #   ProcessIntent(state, intent) → result
  │   │   ├── journal.go          #   journal entry, posting, balance check
  │   │   ├── hashchain.go        #   anti-tamper hash chain
  │   │   ├── coa.go              #   chart of accounts, tipe akun
  │   │   ├── sales.go            #   intent → jurnal (SQ→SO→DP→DO→INV)
  │   │   ├── purchasing.go       #   PR→PO→GRN→tagihan→bayar
  │   │   ├── inventory.go        #   FIFO, rata-rata, costing, NRV
  │   │   ├── production.go       #   job order costing
  │   │   ├── fixed_assets.go     #   penyusutan, revaluasi, disposisi
  │   │   ├── tax.go              #   PPN, PPh, pajak tangguhan
  │   │   ├── period_close.go     #   tutup buku & laba berjalan otomatis
  │   │   ├── consolidation.go    #   eliminasi antar-entitas
  │   │   └── reporting.go        #   trial balance, GL, laporan
  │   ├── auth/                   # JWT, OAuth, 2FA, RBAC
  │   ├── tenant/                 # entitas, periode akuntansi
  │   ├── coa/                    # service CRUD COA (custom account)
  │   ├── sales/  purchasing/  inventory/  production/
  │   ├── fixedassets/  tax/  bank/  recurring/  dimensions/
  │   ├── masterdata/  budgeting/  attachments/  notifications/
  │   ├── reporting/              # service laporan (panggil engine)
  │   ├── consolidation/
  │   ├── http/                   # router, handler, middleware
  │   │   ├── router.go
  │   │   ├── middleware/         #   auth, tenant, RLS, rate-limit
  │   │   └── handlers/           #   handler per modul
  │   ├── events/                 # publisher & subscriber (Redis)
  │   ├── config/                 # env, konfigurasi
  │   └── db/                     # sqlc generated, migrasi
  ├── migrations/                 # SQL migrations (golang-migrate)
  ├── go.mod
  └── go.sum
```

### 4.2 Alur Permintaan (Request Flow)
```
HTTP Request
  → Middleware (auth JWT, tenant context, RLS set, logging, recovery)
  → Router (chi) → Handler (modul bisnis)
  → Service (validasi bisnis)
  → Accounting Engine (pure, menghasilkan jurnal + sub-ledger effects)
  → DB Transaction (pgx, PostgreSQL) — posting jurnal + sub-ledger ATOMIC
  → Publish Event (Redis pub/sub: audit, notifikasi, integrasi)
  → Response (JSON)
```

### 4.3 Transaction Boundary (Kritis)
- **Setiap posting = satu transaksi DB** (jurnal + sub-ledger + status dokumen).
- Jika salah satu gagal → seluruhnya rollback (`PERIOD_CLOSED`, `STOCK_NEGATIVE`, dll).
- `balanceCheck` dijalankan di dalam transaksi, sebelum commit.
- Engine **pure** → service yang mengelola transaksi DB; engine hanya menghitung.

### 4.4 Folder Layout & Dependency Injection (Konkret)
```
internal/
  ├── config/          # Load env → struct Config (dibuat sekali di main)
  ├── db/              # sqlc generated; Repositori per modul
  ├── accounting/      # PURE — tidak depend pada modul lain
  ├── <modul>/         # service.go (bisnis) + repo.go (sqlc)
  │                     # DI: modul menerima dependency via constructor
  │                     #   func NewSalesService(repo SalesRepo, engine *accounting.Engine, ev *events.Bus) *SalesService
  └── http/            # handlers → panggil service; wiring DI di router.go
```
- **Prinsip dependensi:** `http → service → accounting engine`; service tidak pernah memanggil handler; engine tidak tahu apa-apa tentang service/DB.
- **Wiring manual** (tanpa framework DI) — cukup jelas untuk 25 modul; konstruktor eksplisit memudahkan testing (mock interface repo).

### 4.5 API Surface (REST)
| Area | Endpoint (ringkas) |
|---|---|
| Auth | `POST /auth/register`, `POST /auth/login`, `POST /auth/refresh`, `POST /auth/logout`, `POST /auth/2fa` |
| Tenants | `GET/POST /tenants`, `GET /tenants/:id`, `PUT /tenants/:id` |
| COA | `GET/POST /accounts`, `GET/PUT /accounts/:id`, `POST /accounts/:id/deactivate` |
| Sales | `GET/POST /sales/quotations`, `/sales/orders`, `/sales/down-payments`, `/sales/deliveries`, `/sales/invoices`, `/sales/payments`, `/sales/credit-notes` |
| Purchasing | `GET/POST /purchases/requests`, `/purchases/orders`, `/purchases/receipts`, `/purchases/invoices`, `/purchases/payments` |
| Inventory | `GET/POST /items`, `/inventory/movements`, `/inventory/stock-opname` |
| Production | `GET/POST /production/jobs`, `/production/bom` |
| Fixed Assets | `GET/POST /fixed-assets`, `/fixed-assets/:id/depreciate`, `/fixed-assets/:id/revalue`, `/fixed-assets/:id/dispose` |
| Tax | `GET /tax/ppn`, `POST /tax/ppn/accrue`, `GET /tax/pph`, `POST /tax/pph/accrue` |
| Bank | `POST /bank/import`, `GET /bank/reconciliations`, `POST /bank/reconciliations/:id/close` |
| Recurring | `GET/POST /recurring`, `POST /recurring/:id/pause`, `/resume`, `/archive` | `/archive` hanya menonaktifkan template; instance/jurnal yang sudah posted tidak dihapus |
| Dimensions | `GET/POST /dimensions`, `POST /dimensions/:id` |
| Master Data | `GET/POST /customers`, `/suppliers`, `/items`, `/items/:id/prices` |
| Budgeting | `GET/POST /budgets`, `GET /budgets/:id/variance` |
| Reporting | `GET /reports/trial-balance`, `/reports/balance-sheet`, `/reports/profit-loss`, `/reports/cash-flow`, `/reports/general-ledger`, `/reports/journal` |
| Attachments | `POST /attachments`, `GET /attachments/:id` |
| Notifications | `GET /notifications`, `POST /notifications/:id/read` |
| Consolidation | `GET /consolidation/report`, `POST /consolidation/run` |

*(Semua endpoint tenant-scoped: `tenant_id` dari JWT, bukan dari URL — mencegah akses lintas entitas.)*

---

## 5. Accounting Engine (Pure Go Package)

### 5.1 Karakteristik
- **Tanpa IO**: tidak menyentuh DB/HTTP/file — menerima `state` & `intent`, mengembalikan hasil jurnal.
- **Pure functions**: deterministik, mudah diuji (`go test` + tabel kasus + property-based via `testing/quick`).
- **Satu source of truth** untuk aturan posting — dipakai semua modul.
- **Sangat cepat**: tipe data primitif, tanpa alokasi berlebihan — cocok untuk posting massal & agregasi.

### 5.2 Kontrak (Go)
```go
package accounting

type Intent struct {
    Type     string         // "SALES_INVOICE" | "CASH_IN" | ...
    TenantID string
    Data     map[string]any
}

type EngineResult struct {
    Journal          JournalEntry
    SubLedgerEffects []SubLedgerEffect // stok, piutang, aset, job
    Warnings         []string
}

// Pure function inti
func ProcessIntent(state LedgerState, intent Intent) (EngineResult, error)
```

### 5.3 Posting Pipeline
1. **Validate** — intent, akun (grup/detail, aktif), periode terbuka, saldo cukup, dan `idempotency_key`.
2. **Transform** — intent → baris jurnal (debet/kredit) + efek sub-ledger.
3. **Balance Check** — `totalDebit == totalKredit` (selalu balance).
4. **Serialize** — lock `ledger_chain_heads` untuk tenant, ambil head terbaru, dan tetapkan `journal_number` atomik.
5. **Hash** — hitung hash dari jurnal canonical + head hash sebelumnya.
6. **Return** — hasil dikembalikan untuk dipersist oleh service dalam transaksi DB.

### 5.4 Anti-Tamper (Hash Chain)
- Hash chain dibuat **per tenant**, bukan global.
- `ledger_chain_heads(tenant_id PRIMARY KEY, last_journal_id, last_hash)` menyimpan head dan dikunci `FOR UPDATE` dalam transaksi posting.
- `hash = SHA256(canonical_version + tenant_id + journal_number + entry_date + source_ref + intent_type + sorted_lines + prev_hash)`.
- `sorted_lines` diurutkan deterministik berdasarkan line sequence; payload canonical memakai encoding UTF-8 yang terdokumentasi.
- Jurnal pertama `prev_hash = genesis` per tenant.
- Request retry memakai `idempotency_key` yang sama dan mengembalikan hasil jurnal pertama, bukan membuat jurnal baru.
- Verifikasi rutin (cron/backup) → deteksi perubahan ilegal dan head yang tidak cocok.

### 5.5 Alur End-to-End (Contoh: Buat Invoice + Posting)

**Langkah** (HTTP → handler → service → engine → DB → event):

```go
// internal/sales/service.go — ringkas
func (s *SalesService) CreateInvoice(ctx context.Context, req CreateInvoiceReq) (*Invoice, error) {
    var inv *Invoice
    err := s.db.WithTx(ctx, func(tx pgx.Tx) error {
        // 1. Ambil state (saldo akun, piutang pelanggan) — lock baris
        state, err := s.loadState(tx, req.TenantID)
        // 2. Engine murni: intent → jurnal + efek sub-ledger
        result, err := accounting.ProcessIntent(state, req.ToIntent())
        // 3. Persist jurnal + sub-ledger + status dokumen (ATOMIC)
        inv, err = s.persist(tx, req, result)
        // 4. Balance check di dalam transaksi (defensif)
        return verifyBalance(tx, req.TenantID, result.Journal.ID)
    })
    // 5. Event (setelah commit — outbox pattern lihat §7.4)
    s.events.Publish(ctx, "invoice.created", inv.ID)
    return inv, nil
}
```

**Contoh SQL (sqlc query) — saldo akun untuk balance check:**
```sql
-- query.sql
-- name: AccountBalanceByPeriod :one
SELECT
  COALESCE(SUM(CASE WHEN debit_cents > 0 THEN debit_cents ELSE -credit_cents END), 0)::bigint AS balance_cents
FROM journal_lines jl
JOIN journal_entries je ON je.id = jl.entry_id
WHERE jl.account_id = $1
  AND je.tenant_id  = $2
  AND je.status     = 'POSTED'
  AND je.period_id  = $3;
```

**Contoh migrasi (golang-migrate):**
```sql
-- 000001_create_journal.up.sql
CREATE TABLE journal_entries (
    id           BIGSERIAL PRIMARY KEY,
    tenant_id    BIGINT NOT NULL REFERENCES tenants(id),
    number       TEXT   NOT NULL,
    entry_date   DATE   NOT NULL,
    period_id    BIGINT NOT NULL REFERENCES accounting_periods(id),
    status       TEXT   NOT NULL DEFAULT 'POSTED', -- POSTED | VOID
    description  TEXT,
    source_ref   TEXT,
    intent_type  TEXT,
    reversal_of_id BIGINT REFERENCES journal_entries(id),
    void_reason  TEXT,
    voided_by    BIGINT REFERENCES users(id),
    voided_at    TIMESTAMPTZ,
    hash         TEXT   NOT NULL,
    prev_hash    TEXT   NOT NULL,
    created_by   BIGINT REFERENCES users(id),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, source_ref, intent_type)   -- idempotensi
);
CREATE INDEX idx_journal_entries_tenant_date ON journal_entries (tenant_id, entry_date);
```

### 5.6 Outbox Pattern (Event Reliability)
- Event **tidak diterbitkan langsung** saat commit — ditulis ke tabel `outbox_events` dalam transaksi yang sama dengan jurnal.
- Worker `outbox:dispatch` membaca & publish ke Redis, lalu menandai `dispatched`.
- Menjamin **tidak ada event yang hilang** bila commit sukses tapi publish gagal.

---

## 6. Database Schema (Ringkasan)

### 6.1 Core Tables
```
tenants (id, name, currency, fiscal_year_start, period_type)
users (id, email, password_hash, is_active)
user_tenants (id, user_id, tenant_id, role[owner|accountant|admin|staff|consultant])
user_tokens (id, user_id, token_type, token_hash, family_id, expires_at, revoked_at)
accounts (id, tenant_id, code, name, report_group, account_type, parent_id,
          is_active, is_group, valid_from, valid_to)
report_mappings (id, tenant_id, account_id, report_type, report_line)  -- akun kustom → baris laporan
accounting_periods (id, tenant_id, period_start, period_end, status[OPEN|CLOSED],
                    closed_by, unlock_requested; one OPEN/no overlap enforced)
ledger_chain_heads (tenant_id, last_journal_id, last_hash)  -- serialisasi per tenant
journal_entries (id, tenant_id, number, entry_date, period_id, status[POSTED|VOID],
                 source_ref, intent_type, idempotency_key, reversal_of_id,
                 void_reason, voided_by, voided_at, hash, prev_hash,
                 created_by, created_at)
journal_lines (id, tenant_id, entry_id, account_id, debit_cents, credit_cents,
               description, dimension_ids JSONB, original_currency,
               original_amount_cents, original_rate)
customers / suppliers / items / item_price_lists (harga multi-level)
revenue_contracts / performance_obligations / revenue_recognition_schedules
categories (kategori UI → akun) / templates (jurnal cepat)
sales_quotations / sales_orders / down_payments / deliveries / invoices / payments
credit_notes / purchase_requests / purchase_orders / grns / supplier_invoices / payments_ap
purchase_returns / payment_allocations / payment_allocations_ap
inventory_movements (id, item_id, qty, unit_cost, method, batch_ref, location_id)
inventory_batches (lapisan FIFO) / stock_opnames (fisik vs sistem)
fixed_assets (id, account_id, cost, accumulated_depr, method, useful_life,
              residual, revaluation_surplus, impairment, parent_asset_id)
asset_depreciation_entries / asset_transactions
jobs / bom / job_costs / job_completions
tax_rates / tax_breakdowns / tax_accruals / tax_payments
ecl_policies / write_offs (penghapusan piutang)
prepayments / accruals
recurring_templates (multi-line lines JSONB) / recurring_instances
bank_statements / bank_statement_lines / bank_reconciliations (batch)
petty_cash_funds / petty_cash_transactions (imprest)
leases / lease_payments (PSAK 73)
consolidation_entities / intercompany_transactions (PSAK 65)
budgets / budget_lines / dimensions / locations
transfers (pindah kas/bank)
attachments (polymorphic owner) / pending_documents (checklist tutup buku)
approvals / notifications / reports_snapshots
audit_logs / outbox_events
```

> Skema lengkap (kolom per tabel, FK, CHECK, indeks): lihat [DATA_MODEL.md](DATA_MODEL.md).

### 6.2 DB Access (sqlc)
- **sqlc** membaca skema SQL → menghasilkan kode Go type-safe (struct + query functions).
- Tidak ada ORM magic — SQL eksplisit, mudah di-review akurasi akuntansi.
- Migrasi dengan **golang-migrate** (SQL versioned, up/down).

### 6.3 Multi-Tenancy & RLS
- Tabel multi-tenant memakai kolom `tenant_id` + **PostgreSQL Row-Level Security (RLS)**.
- Middleware Go set `current_setting('app.tenant_id')` per request — isolasi bahkan di level DB.
- Setiap usaha = satu tenant; cabang = dimensi (bukan tenant terpisah).

### 6.4 Money & Precision
- Seluruh nominal `BIGINT` satuan **sen** (Rupiah × 100).
- Tidak ada `FLOAT` — menghindari error pembulatan.
- Rate/kurs `NUMERIC(18,6)`.

### 6.5 Relasi Inti (ERD Ringkas)
```
tenants 1─∞ user_tenants ∞─1 users
users 1─∞ user_tokens (refresh token, rotasi per family)
tenants 1─∞ accounting_periods
tenants 1─∞ accounts (COA; parent_id self-ref untuk grup→detail)
accounts 1─∞ journal_lines
journal_entries 1─∞ journal_lines
journal_entries ∞─1 accounting_periods (period_id)
accounts 1─∞ report_mappings (pemetaan baris laporan)

customers 1─∞ invoices 1─∞ invoice_lines
invoices 1─∞ payments  (payment allocation)
invoices ∞─1 credit_notes (refund, PPN dibalik)
invoices 1─∞ write_offs (penghapusan piutang / ECL)

suppliers 1─∞ purchase_orders 1─∞ grns
purchase_orders 1─∞ supplier_invoices
supplier_invoices 1─∞ payments_ap (via payment_allocations_ap)

items 1─∞ inventory_movements (batch_ref untuk FIFO)
items 1─∞ invoice_lines / grn_lines (item_id)
items 1─∞ item_price_lists (harga multi-level)
items 1─∞ inventory_batches (lapisan FIFO)
stock_opnames 1─∞ stock_opname_lines (selisih → jurnal 4907/5907)

fixed_assets ∞─1 accounts (account_id; akun FIXED_ASSET)
fixed_assets self-ref (parent_asset_id untuk komponen aset)
jobs 1─∞ job_costs (material, labor, overhead)

recurring_templates 1─∞ recurring_instances 1─∞ journal_entries (source_ref RCR-...)
bank_statements 1─∞ bank_statement_lines ∞─1 journal_entries
bank_reconciliations 1─∞ bank_statement_lines (batch rekonsiliasi)
budgets 1─∞ budget_lines ∞─1 accounts
leases 1─∞ lease_payments (PSAK 73)
consolidation_entities 1─∞ intercompany_transactions (eliminasi PSAK 65)
petty_cash_funds 1─∞ petty_cash_transactions (imprest)
attachments ∞─1 journal_entries (bukti; polymorphic owner)
audit_logs (tidak berelasi — append-only)
```
- **Integritas:** foreign key + `ON DELETE RESTRICT` untuk data akuntansi (tidak pernah cascade-hapus jurnal).
- **Idempotensi:** partial unique indexes `(tenant_id, source_ref, intent_type)` untuk nilai non-null dan `(tenant_id, idempotency_key)`; recurring memakai unique `(template_id, due_date)`.

### 6.6 Indexing Strategy
| Index | Tujuan |
|---|---|
| `journal_lines(account_id, entry_id)` | Buku besar per akun cepat |
| `journal_entries(tenant_id, entry_date)` | Laporan per periode |
| `journal_entries(tenant_id, status)` | Filter posting/void |
| `journal_entries(reversal_of_id)` | Relasi jurnal reversal |
| `ledger_chain_heads(tenant_id)` | Serialisasi hash chain |
| `inventory_movements(item_id, created_at)` | Mutasi stok & FIFO |
| `invoices(tenant_id, due_date, status)` | Aging & reminder |
| `payments(invoice_id)` | Alokasi pembayaran |
| `journal_lines(dimension_ids)` (GIN) | Laporan per dimensi |
| Partial unique `(tenant_id, source_ref, intent_type)` + `(tenant_id, idempotency_key)` | Anti duplikat posting |

### 6.7 Lifecycle Contract Dokumen
| Resource | Status/transisi utama | Efek jurnal | Aturan periode/approval |
|---|---|---|---|
| Sales Quotation | `DRAFT → SENT → CONVERTED/EXPIRED/CANCELLED` | Tidak ada | Customer wajib; edit bebas sebelum konversi |
| Sales Order | `CONFIRMED → PARTIALLY_DELIVERED → DELIVERED → INVOICED` atau `CANCELLED` | Tidak ada | Customer + payment term wajib saat diproses |
| Down Payment Customer | `RECEIVED → APPLIED/REFUNDED` | Kas/Bank ↔ `2201` (+ PPN bila terutang) | Void/reversal mengikuti periode |
| Delivery | `DRAFT → SHIPPED → RETURNED`; `DRAFT → CANCELLED` | HPP sesuai kebijakan pengakuan, stok keluar | Setelah `SHIPPED`, pembatalan memakai return/reversal |
| Invoice | `DRAFT → ISSUED → PARTIALLY_PAID → PAID`; `VOID` | AR, revenue, PPN, HPP sesuai kebijakan | Customer, term, alamat, NPWP/tax rule tervalidasi |
| Credit Note | `DRAFT → APPLIED`; `VOID` | Contra-revenue, PPN reversal, refund/AR | Tidak melebihi saldo invoice; approval sesuai aturan |
| Purchase Order | `CONFIRMED → PARTIALLY_RECEIVED → RECEIVED/CANCELLED` | Tidak ada | Supplier + payment term wajib |
| GRN | `RECEIVED → MATCHED/RETURNED/CANCELLED` | Inventory ↔ `2105` | Over-delivery memerlukan approval |
| Supplier Invoice | `DRAFT → RECEIVED → PARTIALLY_PAID → PAID`; `VOID` | AP, PPN input, DP realization | Supplier, term, tax rule tervalidasi |
| Payment / Payment AP | `RECORDED → ALLOCATED`; `VOID` | Kas/Bank ↔ AR/AP; overpayment customer `2402`, supplier `1204` | Idempotency key wajib untuk posting |
| Recurring Template | `ACTIVE → PAUSED/ARCHIVED`; instances `REMINDED → POSTED/SKIPPED` | Jurnal sistem per instance | Periode CLOSED → tunda; posted instance tidak dihapus |

*(Engine/Data Model adalah sumber normatif untuk detail transisi dan jurnal.)*

---

## 7. Event-Driven & Job Queue

### 7.1 Event yang Diterbitkan (Redis pub/sub)
| Event | Dipakai Untuk |
|---|---|
| `journal.posted` | Audit, notifikasi, update dashboard real-time |
| `invoice.created` / `invoice.paid` | Dunning/reminder, integrasi |
| `payment.received` | Update piutang, bank feeds |
| `stock.moved` | Update stok & reorder |
| `period.closed` | Tutup buku, laporan final |
| `recurring.due` | Auto-post transaksi berulang |

### 7.2 Job Queue (asynq)
| Job | Fungsi |
|---|---|
| `recurring:post` | Post transaksi berulang yang jatuh tempo |
| `report:export` | Generate PDF/Excel besar (async) |
| `bank:import` | Import & auto-match mutasi bank |
| `tax:calc` | Perhitungan pajak bulanan massal |
| `backup` | Backup terjadwal |

- asynq: retry dengan backoff eksponensial, dead-letter queue, unique job (idempotensi).
- Worker = process Go terpisah (`cmd/worker`).

### 7.3 Konkurensi & Konsistensi
| Skenario | Mekanisme |
|---|---|
| **Posting bersamaan** ke akun yang sama | DB row lock (`SELECT ... FOR UPDATE` pada saldo akun) — serialisasi per akun |
| **Nomor dokumen unik** | Sequence PostgreSQL per (jenis, tenant) — no race |
| **Dua request intent sama** | Unique index `(source_ref, intent_type)` + retry → `DUPLICATE_INTENT` |
| **Posting massal** | Worker Go (goroutine) memproses batch; setiap item tetap transaksi DB sendiri |
| **Recurring & cron** | asynq unique job per (template, periode) — tidak ganda |
| **Optimistic concurrency** | Versi kolom pada master data (COA, item) → stale update ditolak |

### 7.4 Outbox Pattern (Event Reliability)
- Event **tidak diterbitkan langsung** saat commit — ditulis ke tabel `outbox_events` dalam transaksi yang sama dengan jurnal.
- Worker `outbox:dispatch` membaca & publish ke Redis, lalu menandai `dispatched`.
- Menjamin **tidak ada event yang hilang** bila commit sukses tapi publish gagal.
- **Tabel:** `outbox_events(id, tenant_id, topic, payload JSONB, created_at, dispatched_at)`.

---

## 8. Auth & Authorization

| Aspek | Implementasi |
|---|---|
| **Registrasi/Login** | Email + password (argon2id), OAuth Google |
| **Session** | JWT access token (15 menit) + refresh token (rotasi, revoke) |
| **2FA** | TOTP (Google Authenticator) — opsional, disarankan untuk role akuntan |
| **RBAC** | Role: Pemilik, Akuntan, Admin, Staf, Konsultan — per tenant |
| **Permission** | Granular per modul/aksi (CRUD) |
| **SoD** | Segregation of duties — mis. pembuat invoice ≠ approver pembayaran |
| **Audit** | Setiap aksi penting ke `audit_logs` (siapa, apa, kapan, IP) |

### 8.1 Token Lifecycle
| Tahap | Detail |
|---|---|
| **Login** | Verifikasi kredensial/OAuth → buat access (15 menit) + refresh (30 hari, rotasi) → simpan hash refresh di DB (bukan token asli) |
| **Access token** | JWT berisi `sub` (user), `tenant_id`, `role`, `exp` — ditandatangani HS256/RS256 |
| **Refresh** | `POST /auth/refresh` dengan refresh token → validasi hash → **rotasi** (terbitkan refresh baru, revoke yang lama) |
| **Logout** | Revoke refresh + blacklist access sampai exp (Redis) |
| **Deteksi abuse** | Refresh token yang dipakai ulang (setelah rotasi) → revoke semua sesi user (family detection) |

### 8.2 RBAC Matrix (Role × Modul)
| Modul | Pemilik | Akuntan | Admin | Staf | Konsultan |
|---|---|---|---|---|---|
| COA / Jurnal | ✓ | ✓ | ✓ | ✗ | ✓ |
| Sales / Purchasing | ✓ | ✓ | ✓ | ✓ (input) | ✓ |
| Approval pembayaran | ✓ | ✓ | ✓ | ✗ | ✗ |
| Pajak | ✓ | ✓ | ✗ | ✗ | ✓ |
| Laporan | ✓ | ✓ | ✓ | ✓ | ✓ |
| Tutup buku | ✓ | ✓ | ✗ | ✗ | ✗ |
| Pengaturan tenant | ✓ | ✗ | ✓ | ✗ | ✗ |
| Audit log | ✓ | ✓ | ✗ | ✗ | ✓ |

*(Matriks awal — dapat dikonfigurasi per tenant.)*

---

## 9. Multi-Tenant & Data Isolation

- Satu tenant = satu entitas usaha (bukan per pengguna).
- Pengguna dapat memiliki beberapa tenant (multi-usaha) → relasi `user_tenants` (role per tenant).
- Konsultan/akuntan diundang ke tenant klien dengan role terbatas.
- Isolasi: RLS di PostgreSQL + filter wajib `tenant_id` di semua query service.
- Cabang = dimensi dalam satu tenant; konsolidasi entitas hukum via modul konsolidasi.

---

## 10. Caching & Performa

| Data | Strategi |
|---|---|
| COA & master data statis | Cache in-memory (sync.Map / bigcache) + invalidasi saat update |
| Session/rate-limit | Redis |
| Dashboard ringkasan | Cache 30 detik (atau materialized view ringan) |
| Laporan berat | Materialized view / tabel agregat di-refresh via job (worker Go cepat) |
| Realtime (notifikasi) | WebSocket (gorilla/websocket) subscribe ke event |

- Target: input < 1 detik, dashboard < 2 detik, laporan 100k transaksi < 5 detik (realistis dicapai Go).

### 10.1 Cache Invalidation
| Cache | Kapan Invalidasi |
|---|---|
| COA & master data | Update/delete akun, item, customer, supplier |
| Dashboard ringkasan | Setiap `journal.posted` event (subscribe) |
| Laporan materialized view | Setelah tutup buku / post massal / void |
| Session blacklist | Saat logout / rotate refresh |

- Strategi **write-through / TTL pendek**: data akuntansi tidak pernah hanya di-cache — selalu baca DB untuk kepastian; cache hanya untuk akselerasi tampilan.

---

## 11. Frontend Architecture (React SPA)

### 11.1 Folder Layout
```
web/src/
  ├── app/                 # router (React Router), layout, providers
  ├── features/            # per fitur (coa, sales, inventory, report, ...)
  │   └── <fitur>/
  │       ├── components/  #   komponen UI khusus fitur
  │       ├── hooks/       #   custom hooks (data fetching)
  │       └── api/         #   client API (fetch wrapper)
  ├── components/          # UI umum (Button, Table, Modal, Form)
  ├── stores/              # Zustand (session, tenant switch, draft)
  ├── lib/                 # utils, format Rupiah, validasi
  ├── types/               # TypeScript types (shared dgn backend contract)
  └── main.tsx
```

### 11.2 State & Data Fetching
| Data | Strategi |
|---|---|
| Server state | **TanStack Query** — cache, refetch, optimistic update, invalidasi setelah mutasi |
| Session & tenant aktif | **Zustand** (persist localStorage untuk refresh token aman? → lebih aman httpOnly cookie untuk refresh) |
| Draft form (belum submit) | Zustand / komponen state |
| Realtime notifikasi | WebSocket (hook `useNotifications`) subscribe event |

### 11.3 Pola UI "Pembukuan Mudah"
- **Input transaksi** = satu form besar dengan bahasa sehari-hari ("Uang Masuk/Uang Keluar"), bukan jurnal manual.
- **Mode Akuntan** = toggle yang menampilkan jurnal, buku besar, trial balance.
- **Invalidasi query otomatis** setelah POST/PUT/PATCH (mis. setelah invoice → dashboard & laporan refresh).
- Error dari API ditampilkan `code` + `message` bahasa Indonesia.

---

## 12. API Versioning & Contoh Payload

### 12.1 Versioning
- URL prefix: `/api/v1/...` — versi mayor eksplisit.
- Versi minor/additive tanpa change version.
- Deprecation: header `Deprecation` + masa transisi 6 bulan.
- Contract TypeScript shared: `packages/shared` (types API) dipakai frontend & backend (gen dari OpenAPI bila perlu).

### 12.2 Contoh Payload — Buat Invoice
```json
POST /api/v1/sales/invoices
{
  "customer_id": "cus_123",
  "date": "2026-08-05",
  "due_date": "2026-09-04",
  "lines": [
    { "item_id": "itm_1", "qty": 2, "unit_price_cents": 5000000, "tax": "ppn_11" }
  ],
  "down_payment_applied_cents": 2500000
}
```
**Respons:**
```json
{
  "id": "inv_2026_000123",
  "number": "INV-2026-000123",
  "status": "ISSUED",
  "total_cents": 11100000,
  "journal": { "id": "jrn_2026_000123", "status": "POSTED" }
}
```

### 12.3 Contoh Payload — Error (sudah di §18.1)

---

## 13. Integrasi Eksternal (Fase 3+)

| Integrasi | Mekanisme |
|---|---|
| **e-Faktur / e-Bupot / e-SPT (DJP)** | API DJP / file upload terstruktur; retry & reconciliation |
| **Bank feeds** | Import CSV/OFX manual dulu; API Open Banking (fase lanjut) |
| **OCR struk** | Upload foto → Google Vision API → pratinjau transaksi |
| **Payment gateway** (Midtrans/Xendit) | Webhook → verifikasi signature → jurnal otomatis (idempoten) |
| **E-commerce** (Shopee/Tokopedia) | API order → draft invoice |
| **Email notifikasi** | SendGrid / Resend (SMTP) |

---

## 14. Deployment & Infrastruktur

### 14.1 Keunggulan Binary Tunggal
- `go build` → **satu binary statis** (`CGO_ENABLED=0`) — tanpa runtime/dependency.
- Cross-compile: `GOOS=linux GOARCH=amd64` dari mesin mana pun.
- Image Docker **scratch/alpine ~15MB** → startup instan, deploy sangat mudah.
- Frontend = file statis hasil `vite build` → upload ke CDN/storage (S3, Cloudflare Pages).

### 14.2 Lingkungan
| Env | Tujuan |
|---|---|
| `development` | Lokal (Docker Compose: api, worker, db, redis) |
| `staging` | UAT, mirror production |
| `production` | Public |

### 14.3 Docker Compose (Lokal)
```yaml
services:
  api:        # Go binary (server HTTP)
    build: ./backend
    ports: ["3000:3000"]
    depends_on: [db, redis]
  worker:     # asynq worker
    build: ./backend
    command: /app/worker
  db:
    image: postgres:16
    environment: { POSTGRES_DB: accounting, POSTGRES_USER: app, POSTGRES_PASSWORD: dev }
  redis:
    image: redis:7
  web:        # frontend statis (Vite preview/dev)
    build: ./web
    ports: ["5173:5173"]
```

### 14.4 Production
- **Render / Fly.io / Railway** di awal (deploy binary Go semudah push + build), atau **AWS ECS/EKS** saat besar.
- 2+ replica API (stateless), worker terpisah, auto-scaling.
- Managed PostgreSQL (RDS/Neon/Supabase) dengan backup + point-in-time recovery.
- Managed Redis (Upstash/ElastiCache).
- Frontend di CDN (Cloudflare Pages / S3+CloudFront) — cepat & murah.

---

## 15. Keamanan

| Area | Implementasi |
|---|---|
| **Transport** | TLS 1.2+ (HTTPS) wajib |
| **At-rest** | Enkripsi disk (DB, storage) |
| **Password** | Argon2id |
| **Secrets** | Env + secret manager (Vault/SSM), tidak di repo |
| **Input** | Validasi DTO (go-playground/validator), rate limiting, SQL injection protection (sqlc parameterized) |
| **XSS/CSRF** | Frontend framework escape, CSRF token untuk cookie session |
| **Anti-tamper** | Hash chain jurnal + audit log |
| **UUD PDP** | Persetujuan data, hak akses & hapus data, enkripsi PII |

---

## 16. Observability

| Alat | Fungsi |
|---|---|
| **Sentry** | Error tracking (frontend & backend) |
| **Prometheus** | Metrik (RPS, latency, error rate, queue depth) |
| **Grafana** | Dashboard metrik |
| **Uptime Kuma / StatusPage** | Monitor ketersediaan |
| **Structured logging** | `slog` (stdlib) / zap → JSON → Loki/Elasticsearch |

**Alert:** error rate > 1%, latency p95 > 2s, queue stuck, disk > 80%.

### 16.1 Distributed Tracing
- **OpenTelemetry** (Go SDK) — trace per request lintas API → worker → DB.
- Export ke **Jaeger / Grafana Tempo** (self-host) atau managed (Datadog/Honeycomb).
- `trace_id` disebar ke log & event → korelasi satu klik antar komponen.

### 16.2 Metrik Aplikasi (custom)
| Metrik | Kegunaan |
|---|---|
| `journal_post_duration` histogram | Performa engine |
| `journal_post_total` counter (by status) | Volume & error posting |
| `reconciled_vs_unmatched` gauge | Kesehatan rekonsiliasi bank |
| `queue_depth` per jenis job | Beban worker |
| `db_connection_pool` | Kesehatan DB |

### 16.3 SLO (Service Level Objectives)
| Metrik | Target |
|---|---|
| Uptime | 99.5% bulanan |
| Latency API p95 | < 1.5s (dashboard) / < 3s (laporan berat) |
| Error rate | < 0.5% request |
| Duplikat posting | 0 (idempotensi wajib) |
| Laporan tidak balance | 0 (invariant engine) |
| Waktu restore | RTO ≤ 1 jam (lihat §20) |

---

## 17. Testing Strategy

| Lapisan | Alat | Cakupan |
|---|---|---|
| **Unit engine** | `go test` + `testing/quick` | Setiap intent → jurnal benar; balance invariant; edge cases (lihat Test Matrix ACCOUNTING_ENGINE §33) |
| **Unit service** | `go test` + mock repo | Logika bisnis tanpa DB |
| **Integration** | `testcontainers-go` (PostgreSQL/Redis riil) | Posting → DB → baca kembali; transaksi rollback; RLS lintas tenant |
| **API/e2e** | `httptest` + seeded DB | Alur lengkap SQ→SO→DP→DO→INV→lunas; auth; RBAC |
| **Frontend** | Vitest + React Testing Library | Komponen & hook; aksi input awam |
| **E2E frontend** | Playwright | Onboarding → input transaksi → lihat laporan |
| **Property-based** | `testing/quick` (Go) | Invariant "selalu balance" untuk arbitrary intents |
| **Load test** | k6 / hey | Input & laporan pada target performa (input <1s, laporan 100k <5s) |

### 17.1 Golden Test Akuntansi
- Simpan **fixture jurnal baku** (dari kasus ACCOUNTING_ENGINE §33) sebagai golden files.
- Setiap perubahan engine → golden test memastikan jurnal tidak berubah tanpa sengaja (regression guard).

---

## 18. Error Handling & Response Convention

### 18.1 Format Error (JSON)
```json
{
  "error": {
    "code": "PERIOD_CLOSED",
    "message": "Periode sudah dikunci",
    "details": {"period": "2026-08"}
  }
}
```
- Kode error standar (lihat ACCOUNTING_ENGINE §30.4) — konsisten frontend & backend.
- `message` dalam bahasa Indonesia (UX) + kode API stabil (untuk integrasi).

### 18.2 Kategori
| HTTP | Kode | Contoh |
|---|---|---|
| 400 | Validasi input | `INVALID_ACCOUNT_CODE`, `STOCK_NEGATIVE` |
| 401 | Unauthorized | token invalid/expired |
| 403 | Forbidden | RBAC/SoD ditolak |
| 404 | Not found | `ACCOUNT_NOT_FOUND` |
| 409 | Konflik bisnis | `PERIOD_CLOSED`, `CN_EXCEEDS_INVOICE`, `DUPLICATE_INTENT` |
| 422 | Invariant gagal | `NOT_BALANCED`, `SUSPENSE_OPEN` |
| 500 | Internal | tidak boleh bocor detail ke client |

### 18.3 Middleware Recovery
- `recover()` di middleware → log stack + trace_id → return 500 generik.
- Panic di job worker → asynq retry (bukan crash seluruh process).

---

## 19. Environment & Configuration

### 19.1 Env (12-factor)
```
APP_ENV=development|staging|production
HTTP_ADDR=:3000
DATABASE_URL=postgres://...
REDIS_URL=redis://...
JWT_SECRET=...            (secret manager di prod)
GOOGLE_CLIENT_ID=...      (OAuth)
S3_ENDPOINT=...  S3_BUCKET=...
SENTRY_DSN=...
OTEL_EXPORTER=...
```
- **Semua konfigurasi via env** (12-factor) — tidak ada config hard-code di repo.
- `config.Load()` di awal main → `*Config` dibagikan via DI.

---

## 20. Backup, Restore & Disaster Recovery

| Aspek | Strategi |
|---|---|
| **Backup DB** | `pg_dump` harian + WAL archiving (PITR) — managed Postgres (RDS/Neon) sudah built-in |
| **Backup file** | S3 versioning untuk attachment |
| **RPO** | ≤ 15 menit (PITR) |
| **RTO** | ≤ 1 jam (restore ke instance baru) |
| **Restore drill** | Uji restore bulanan ke environment staging |
| **DR region** | Replica cross-region (fase besar) |
| **Verifikasi hash chain** | Cron verifikasi jurnal (anti-tamper) setelah restore |

---

## 21. CI/CD Pipeline

```
Git push → GitHub Actions
  ├── Lint (golangci-lint, ESLint)
  ├── Test (go test — engine + integration; vitest)
  ├── Build (go build; vite build)
  ├── Docker image build & push (~15MB)
  ├── Deploy staging (auto)
  └── Deploy production (manual approval / tag)
```

---

## 22. Struktur Repositori

```
finance-accounting-app/
  ├── backend/                 # Go monolith
  │   ├── cmd/
  │   │   ├── api/             #   HTTP server entrypoint
  │   │   └── worker/          #   asynq worker entrypoint
  │   ├── internal/
  │   │   ├── accounting/      #   ENGINE PURE (double-entry, tanpa IO)
  │   │   ├── auth/  tenant/  coa/  sales/  purchasing/  inventory/
  │   │   ├── production/  fixedassets/  tax/  bank/  recurring/
  │   │   ├── dimensions/  masterdata/  budgeting/  attachments/
  │   │   ├── reporting/  consolidation/  notifications/
  │   │   ├── http/            #   router + handlers + middleware
  │   │   ├── events/  config/  db/
  │   │   └── ...
  │   ├── migrations/          # SQL migrations
  │   ├── Dockerfile           # multi-stage → image ~15MB
  │   └── go.mod
  ├── web/                     # React + Vite (SPA statis)
  │   ├── src/
  │   ├── Dockerfile (nginx/static) atau deploy ke CDN
  │   └── package.json
  ├── infra/
  │   ├── docker/              # compose
  │   └── k8s/                 # (fase lanjut)
  ├── docs/
  │   ├── PRD.md
  │   ├── ACCOUNTING_ENGINE.md
  │   ├── GAP_ANALYSIS.md
  │   └── ARCHITECTURE.md      # (ini)
  └── README.md
```

---

## 23. Peta Implementasi (Sesuai Roadmap PRD)

| Fase | Fokus Teknis |
|---|---|
| **Fase 1 (MVP)** | Monorepo, engine Go dasar (double-entry, COA, saldo awal), auth, kas/bank, kategori → jurnal, laporan dasar |
| **Fase 2** | Sales & purchasing flow, inventory, **Trial Balance & GL reports**, recurring, master data, rekonsiliasi bank, dimensi dasar, tutup buku otomatis, nomor dokumen |
| **Fase 3** | Produksi, aset tetap, ECL, pajak dasar dan lanjutan, budget, attachment, dimensi lanjutan |
| **Fase 3+** | Pajak lengkap + integrasi DJP, bank feeds lanjutan, multi-mata uang, sewa, konsolidasi, OCR |

---

## 24. Risiko Teknis & Mitigasi

| Risiko | Mitigasi |
|---|---|
| Engine double-entry salah → laporan tidak balance | Pure package + unit test ekstensif + property-based testing + invariant runtime |
| Skala besar laporan lambat | Go cepat + materialized view + agregasi + async export + indexing |
| Multi-tenant bocor data | RLS + integrasi test akses lintas tenant |
| Redis down mengganggu core | Redis hanya cache/queue — database tetap jalan; graceful degradation |
| Regulasi pajak berubah | Konfigurasi tarif berbasis data (tidak hard-code); update terjadwal |
| Monolith membesar | Batas modul ketat, engine terpisah — jalur ke microservices terbuka |
| Tim tidak familiar Go | Tooling sederhana & konvensi ketat; onboarding cepat (dibanding C++/Rust) |

---

## 25. ADR (Architecture Decision Records) Ringkas

| # | Keputusan | Opsi yang Ditolak | Alasan |
|---|---|---|---|
| ADR-001 | **Go (Golang)** untuk backend & engine | Node/NestJS, .NET | Cepat olah data, binary tunggal, mudah deploy & develop |
| ADR-002 | **React SPA statis** (tanpa Next.js) | Next.js/SSR | Dashboard app tidak butuh SSR; deploy CDN lebih sederhana |
| ADR-003 | **Modular Monolith** | Microservices | Mulai sederhana; batas modul jelas; jalur migrasi terbuka |
| ADR-004 | **PostgreSQL** | MySQL, MongoDB | ACID wajib untuk double-entry; RLS, JSONB, ekosistem matang |
| ADR-005 | **sqlc + pgx** (SQL eksplisit) | GORM/ORM | Kendali penuh SQL, type-safe, tanpa overhead ORM |
| ADR-006 | **Redis + asynq** untuk queue | RabbitMQ | Ringan, teruji, API Go bersih; RabbitMQ untuk kebutuhan pub/sub besar |
| ADR-007 | **RLS multi-tenant** satu DB | DB terpisah per tenant | Skala awal murah; isolasi kuat di level DB |
| ADR-008 | **BIGINT sen** (bukan FLOAT/NUMERIC untuk jurnal) | FLOAT, NUMERIC | Presisi mutlak; menghindari error pembulatan |
| ADR-009 | **Hash chain anti-tamper** | Tanpa hash | Deteksi perubahan ilegal jurnal |
| ADR-010 | **Outbox pattern** untuk event | Publish langsung saat commit | Tidak ada event hilang; konsistensi transaksional |

---

## 26. Daftar Periksa Finalisasi (Checklist Implementasi)

### Fondasi
- [ ] Monorepo: `backend/`, `web/`, `infra/`, `docs/`
- [ ] CI/CD GitHub Actions jalan (lint, test, build, docker)
- [ ] Docker Compose lokal: api + worker + db + redis
- [ ] Migrasi golang-migrate + sqlc generate
- [ ] Config 12-factor via env + `.env.example`

### Engine & Data
- [ ] `internal/accounting` pure: double-entry, balance check, hash chain
- [ ] Golden test akuntansi (fixture dari ACCOUNTING_ENGINE §33)
- [ ] RLS diaktifkan + test akses lintas tenant
- [ ] Unique index idempotensi `(tenant_id, source_ref, intent_type)`

### Fitur Inti (Fase 1)
- [ ] Auth: register/login/refresh/logout/2FA + RBAC
- [ ] Tenant & periode akuntansi (open/close)
- [ ] COA inti + tipe akun + custom account
- [ ] Kategori UI → jurnal otomatis (Uang Masuk/Keluar)
- [ ] Laporan dasar: Laba Rugi, Neraca, Arus Kas, Trial Balance

### Kualitas
- [ ] Observability: Sentry + Prometheus + Grafana + tracing OTel
- [ ] Error convention JSON (kode standar) diterapkan seluruh API
- [ ] Backup/DR: PITR aktif + restore drill
- [ ] Load test: input <1s, laporan 100k <5s
- [ ] SLO dimonitor (uptime, latency p95, error rate)

---

*Dokumen ini referensi untuk tim engineering. Keputusan akhir stack diverifikasi saat implementasi dimulai.*
