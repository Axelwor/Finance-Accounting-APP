# FIX F3 — Costing + Schema Drift (Wave 3)

**Worktree:** `fix-wave3-f3-costing-drift` · **Tanggal:** 2026-08-23 · **Sumber temuan:** `QA_RETEST_2026-08-23_CONSOLIDATED.md`, `QA_RETEST_W3_SALES.md` §5, `QA_RETEST_W4_PURCHASE_MISC.md` §5–9

Lingkungan verifikasi: DB `finance_qa_fx3` (57 migrasi diterapkan per-file sebagai superuser; 000030 error mid-file = known issue, di-re-run hingga lengkap), role aplikasi **`qa_fx3_app` NOSUPERUSER NOBYPASSRLS**, server `:18093` (background persistent), tenant **W-F3** (id 1000).

## Ringkasan

| ID | Bug | Status |
|---|---|---|
| QA-03 | DO selalu 500 `conn busy` pada `consumeFIFO` | ✅ FIXED — DO → **201**, COGS terposting, stok 40 |
| QA-12 | GET `/forecast/cash-flow` → 500 `column si.amount_due_cents does not exist` | ✅ FIXED — **200** struktur wajar |
| QA-13 | GET `/production-jobs` → 500 `invalid input syntax for type date: ""` | ✅ FIXED — list & detail **200** dengan `completion_date: null` |
| N3 | POST `/lease-contracts/{id}/depreciate` → 500 `column "rou_cost_cents" does not exist` | ✅ FIXED — **201** + log terisi |
| N4 | POST `/dimensions` → 500 `cannot scan timestamptz into *string` | ✅ FIXED — create **201**, GET menampilkan baris |
| QA-16 | Fixed asset `straight_line` tanpa rate → 400 not-null | ✅ FIXED — **201**, rate otomatis, register NBV benar |

## Detail Perbaikan

### QA-03 (HIGH) — consumeFIFO conn busy — `backend/internal/costing/costing.go`

Akar: `tx.Exec` dijalankan di dalam loop iterasi `rows.Next()` dari `tx.Query` pada koneksi pgx yang sama (dilarang pgx v5 → `conn busy`). Fix:

1. **Materialisasi**: `loadOpenFIFOLayers()` membaca semua layer terbuka `(id, qty_remaining, unit_cost)` ke slice, lalu menutup rows (`rows.Close()` via defer + cek `rows.Err()`) SEBELUM statement lain dieksekusi. Semantik query tetap: filter open layer, `ORDER BY created_at, id`, `FOR UPDATE`.
2. **Plan murni**: `planFIFOConsumption(layers, qty, avgCost)` menghitung COGS FIFO oldest-first, daftar update per-layer (partial → `qty_remaining=$4`; habis → `qty_remaining=0, closed_at=now()`), dan qty tak-tercakup untuk fallback avg cost. UPDATE dijalankan setelah rows tertutup, dalam transaksi yang sama.
3. **Bug laten ikut dibetulkan**: SQL partial-update lama memakai `$3` dua kali dengan 4 argumen (`mismatched param and argument count`) — jalur ini tidak pernah terjangkau sebelumnya karena selalu mati lebih dulu di `conn busy`. Kini `SET qty_remaining = $4`.

Semantik dipertahankan: FIFO oldest-first, pembulatan `math.Round`, fallback qty shortfall × `avg_unit_cost_cents`, reduksi `stock_balances.qty_on_hand`, penolakan stok negatif.

**Test baru:**
- Unit murni (`fifo_plan_test.go`, 10 kasus): konsumsi penuh/parsial, multi-layer oldest-first, fallback, skip layer kosong, berhenti di qty, rounding, qty fraksional, keunikan target update.
- Regression driver-level (`costing_integration_test.go`, guard `TEST_DATABASE_URL`): `TestResolveCOGS_FIFO_NoConnBusy` — seed tenant+item+2 layer GRN (30@80rb, 20@90rb) sebagai role terbatas, `PostGRN` → `ResolveCOGS` qty 10 → COGS 800.000 tanpa `conn busy`; state layer/balance diverifikasi. **PASS** terhadap `finance_qa_fx3`.
- Test replika lama di `costing_test.go` dialihkan dari salinan lokal ke fungsi produksi `planFIFOConsumption`.

**E2E:** supplier lengkap → PO → **GRN 50 pcs @80.000 (201)** → SO → **POST /delivery-orders qty 10 (201)**, respons `total_cogs_cents=800000`, jurnal `JRN-2026-000002`: Dr 5101 COGS 800.000 / Cr 1301 Persediaan 800.000, psql: `stock=40.000`, layer tersisa `remaining=40`.

### QA-12 — forecast cash-flow kolom lama — `backend/internal/forecast/handler.go`

Skema aktual `supplier_invoices` memakai `payable_cents` (outstanding, dikurangi payment) — bukan `amount_due_cents`. Query AP diganti ke `si.payable_cents`. Sekalian drift status dibetulkan (kedua tabel tidak punya status `POSTED`): AR `invoices.status IN ('ISSUED','PARTIALLY_PAID') AND receivable_cents > 0`; AP `supplier_invoices.status IN ('ISSUED','PARTIALLY_PAID') AND payable_cents > 0` — sesuai transisi status aktual di modul sales/purchase.

**E2E:** `GET /forecast/cash-flow?horizon=7` → **200** `{starting_balance_cents, horizon_days, buckets[7], total_in/out, ending_balance_cents}`.

### QA-13 — cast tanggal palsu — `backend/internal/production/jobs.go`

`COALESCE(pj.completion_date,'')` memaksa cast literal `''` ke tipe date (query selalu gagal). Scan target sudah `pgtype.Date` sehingga NULL ditangani sendiri. Dihilangkan dari **dua** query: list (`ListProductionJobs`) dan detail (`fetchProductionJob`). Tidak ada parameter tanggal pada endpoint, tidak ada yang perlu divalidasi.

**E2E:** BOM `BOM-F3` (201) → job `JOB-2026-000001` (201) → `GET /production-jobs` → **200**, 1 baris `{"status":"OPEN","completion_date":null}`; `GET /production-jobs/1` → **200**.

### N3 — lease depreciate drift skema — `backend/internal/lease/depreciation.go`

Kolom aktual `lease_contracts`: `initial_rou_cents` (bukan `rou_cost_cents`), tidak ada `total_months` (diturunkan dari `total_payments × payment_frequency`: MONTHLY=1×, QUARTERLY=3×, ANNUALLY=12× via helper murni `leaseTermMonths`), dan tidak ada `accum_dep_cents` (UPDATE lama juga pasti gagal).

Perilaku hitung depresiasi RoU dipertahankan: straight-line `initial_rou_cents / total_months`, residual diserap periode terakhir, penolakan bila sudah fully depreciated, jurnal Dr 5209 / Cr 1702. Perubahan:

1. SELECT memakai `initial_rou_cents, total_payments, payment_frequency`.
2. Akumulasi depresiasi dibaca dari `lease_depreciation_log` (`SUM(depreciation_cents)`) — konsisten dengan log yang ditulis di transaksi yang sama.
3. INSERT `lease_depreciation_log (tenant_id, lease_id, period_year, period_month, depreciation_cents, journal_entry_id)` menggantikan UPDATE `accum_dep_cents`; UNIQUE per (lease, tahun, bulan) menjadi guard idempotensi → 409 `DEPRECIATION_ALREADY_POSTED`.
4. Ikut dibetulkan (drift sama, file sama): `ListDepreciationLog` scan `posted_at` timestamptz ke `*string` → diganti `time.Time` + format RFC3339 (sebelumnya log 500 meski ada isi).

**Test:** `depreciation_test.go::TestLeaseTermMonths` (MONTHLY/QUARTERLY/ANNUAL/fallback).

**E2E:** kontrak 12×5.000.000 @10% → RoU 34.068.459 (201, ACTIVE) → `POST /lease-contracts/1/depreciate?year=2026&month=8` → **201** `depreciation_cents=2839038` (=34.068.459/12), `journal_entry_id=4`; `GET .../depreciation-log` → **200**, entries berisi periode 2026-08; psql `deplog_rows=1`.

### N4 — dimensions timestamptz scan — `backend/internal/budget/dimensions.go`

`created_at` (timestamptz) di-scan ke field `string`. Diperbaiki di kedua jalur (CreateDimension RETURNING dan ListDimensions): scan ke `time.Time` lokal, diformat RFC3339 ke response. Kolom tetap dikembalikan (kontrak API tidak berubah).

**E2E:** `POST /dimensions {code,name,dimension_type}` → **201** `created_at:"2026-08-23T15:38:57+07:00"`; `GET /dimensions` → 200 menampilkan baris tersebut.

### QA-16 — auto-rate straight_line — `backend/internal/assets/assets.go`

`fixed_assets.rate` NOT NULL, sedangkan handler hanya mengisi rate bila client mengirimnya. Kini bila method `straight_line` dan rate kosong: rate diturunkan otomatis `1/useful_life_months` (helper murni `autoStraightLineRate`, format 6 desimal sesuai `NUMERIC(9,6)`) — semantik cocok dengan schedule existing yang membebankan `(cost − salvage)/useful_life_months` per periode. Response `rate` menampilkan nilai efektif. Method lain tidak berubah (`declining_balance` tetap wajib rate eksplisit).

**Test:** `auto_rate_test.go::TestAutoStraightLineRate` (12→0.083333, 36→0.027778, dst.), `TestAutoStraightLineRate_ScannableAsNumeric`, `TestValidateRegisterRequest_StraightLineStillAllowsEmptyRate`.

**E2E:** `POST /fixed-assets` straight_line, useful_life 36 bulan, **tanpa rate** → **201** `rate:"0.027778"`; `GET /assets/register` → 200, NBV = cost 24.000.000, accum 0.

## Gate

```
gofmt -l .        → bersih
go vet ./...      → OK
go build ./...    → OK
go test ./...     → 37 paket ok, 0 FAIL
```
(+ integration test costing PASS dengan `TEST_DATABASE_URL` menuju `finance_qa_fx3` sebagai `qa_fx3_app`)

## File Diubah

| File | Perubahan |
|---|---|
| `backend/internal/costing/costing.go` | Materialisasi layer + plan murni + fix param SQL (QA-03) |
| `backend/internal/costing/costing_test.go` | Test replika dialihkan ke fungsi produksi |
| `backend/internal/costing/fifo_plan_test.go` | BARU — unit test planFIFOConsumption |
| `backend/internal/costing/costing_integration_test.go` | BARU — regression conn-busy (guard TEST_DATABASE_URL) |
| `backend/internal/forecast/handler.go` | payable_cents + filter status outstanding (QA-12) |
| `backend/internal/production/jobs.go` | Hilangkan COALESCE completion_date di list + detail (QA-13) |
| `backend/internal/lease/depreciation.go` | initial_rou_cents, term dari frekuensi, log table, posted_at scan (N3) |
| `backend/internal/lease/depreciation_test.go` | BARU — TestLeaseTermMonths |
| `backend/internal/budget/dimensions.go` | created_at time.Time di create + list (N4) |
| `backend/internal/assets/assets.go` | Auto-rate straight_line + effectiveRate di response (QA-16) |
| `backend/internal/assets/auto_rate_test.go` | BARU — unit test auto-rate |
| `docs/FIX_F3_COSTING_DRIFT.md` | BARU — laporan ini |

Tidak ada perubahan pada sales/purchase/cash/period/auth/dashboard, migrasi, TASK_LEDGER, atau file bersama lain. Tidak ada commit.
