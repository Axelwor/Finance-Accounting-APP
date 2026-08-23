# QA Retest W4 — Purchase & Modul Pendukung

- Tanggal: 2026-08-23
- Lingkup: Retest bug lama (docs/QA_REPORT_2026-08-23.md bagian Purchase/Misc/Dashboard) + verifikasi migration 000057 (commit db2f04d) pada role DB terbatas
- Lingkungan: DB lokal PostgreSQL, database `finance_qa_rw4`, role API `qa_w4_app` (**NOSUPERUSER NOBYPASSRLS**, diverifikasi `rolsuper=f, rolbypassrls=f` sepanjang pengujian runtime), API build dari worktree ini di port :18084
- Tenant uji: W4 Corp (`tenant_id=1000`, owner@w4.test), semua request memakai header `Idempotency-Key` UUID unik

## Ringkasan Verdict

| ID | Item | Verdict |
|---|---|---|
| QA-04 | Supplier NULL-scan | **STILL-BROKEN** |
| — | PO → GRN → stok → jurnal | PASS |
| — | Supplier invoice (PPN, dpp_cents) | PASS |
| — | Payment parsial | PASS |
| QA-06 | Payment kedua (pelunasan) | **STILL-BROKEN** |
| — | Purchase return (stok + jurnal reversal) | **STILL-BROKEN (varian baru)** |
| QA-12 | GET /forecast/cash-flow | **STILL-BROKEN** |
| QA-13 | GET /production-jobs | **STILL-BROKEN** (POST job/BOM sehat) |
| QA-14 | POST /dashboard/widgets tanpa config_json | **PARTIAL** (bug inti tetap; ada workaround) |
| QA-24 / 000057 | Template global tenant_id=0 terbaca tenant biasa | **FIXED** |
| QA-16 | Fixed asset straight_line tanpa rate | **PARTIAL** (rate tak dihitung otomatis; register 200) |
| — | Lease contract + payment no.1 + depreciation-log | PASS dengan **temuan baru** di depreciate |
| — | Smoke 16 endpoint pendukung | PASS (semua 200) |
| — | Invarian trial-balance | PASS (`balanced=true`) |

---

## 1. QA-04 — Supplier NULL-scan: STILL-BROKEN

Pemetaan bertahap (semua 400, pesan raw scan error):

| Tahap | Payload | Hasil |
|---|---|---|
| S1 | code+name+email | 400 `can't scan into dest[3] (col: npwp): cannot scan NULL into *string` |
| S2 | +npwp | 400 `dest[4] contact_person` |
| S3 | +contact_person | 400 `dest[5] phone` |
| T1 | npwp+cp+phone+payment_term_id | 400 `dest[7] address` |
| T2/T3/T4 | berurutan tanpa city/province/postal_code | 400 `dest[8] city` dst. |
| T5 | semua kecuali credit_limit | 400 `dest[12] credit_limit_cents: cannot scan NULL into *int64` |
| S5c | **SEMUA** kolom opsional terisi + payment_term_id | **201** |

Kesimpulan: supplier minimal **mustahil dibuat**. Rantai wajib-faktual: `npwp → contact_person → phone → address → city → province → postal_code → payment_term_id → credit_limit_cents` — semuanya kolom nullable di DB tapi discan ke `*string`/`*int64` non-null di `backend/internal/purchase/suppliers.go`.

Temuan tambahan (LOW): `supplier_type:"company"` → 400 raw `violates check constraint suppliers_supplier_type_check`; nilai valid hanya `GOODS/SERVICE/MIXED` (dokumentasi/kontrak API tidak jelas).

## 2. Alur PO → GRN → Invoice → Payment

PASS sampai payment pertama:

- PO `PO-2026-000001`: 100 pcs × 10.000 cents, 201.
- GRN `GRN-2026-000001`: 201, `stock_balances` = 100 qty (diverifikasi psql), jurnal #1: Dr 1301 Inventory 1.000.000 / Cr 2105 Uninvoiced Payables 1.000.000. ✔
- Supplier invoice `BIL-2026-000001`: `dpp_cents=1.000.000`, `vat_cents=110.000` (PPN 11%), `total_cents=1.110.000`, jurnal #2: Dr 2105 1.000.000, Dr 1203 Input VAT 110.000, Cr 2101 AP 1.110.000. ✔
- Payment parsial 500.000: 201, `PAY-2026-000001`, AP sub-ledger dan status invoice jadi `PARTIALLY_PAID` (payable 610.000). ✔

## 3. QA-06-family — Payment kedua pelunasan: STILL-BROKEN

```
POST /api/v1/supplier-invoices/1/payments {"amount_cents":610000}  (Idempotency-Key UUID baru)
→ 409 {"code":"DUPLICATE_PAYMENT","message":"payment already exists"}
```

Bukan duplikat sungguhan dan bukan replay idempotency (key selalu baru). Akar masalah: `sourceRef := fmt.Sprintf("PAY-%d", invoiceID)` (`backend/internal/purchase/supplier_payments.go:159`) membuat SEMUA jurnal payment untuk satu invoice memakai `source_ref` sama, sehingga INSERT jurnal kedua menabrak unique index `journal_entries_intent_unique (tenant_id, source_ref, intent_type)`; unique violation dipetakan generik oleh `isUniqueViolation` → 409 DUPLICATE_PAYMENT (pesan salah).

Dampak: pelunasan bertahap tidak mungkin; invoice terjebak `PARTIALLY_PAID` selamanya (terbukti payable=610.000 permanen).

## 4. Purchase Return: STILL-BROKEN (varian baru)

```
POST /api/v1/purchase-returns {"invoice_id":1,"refund_method":"refund","lines":[...]} 
→ 500 PR_CREATE_FAILED
log: ERROR: new row for relation "inventory_movements" violates check constraint 
     "inventory_movements_movement_type_check" (SQLSTATE 23514)
```

Akar: kode meng-insert `movement_type='PURCHASE_RETURN'` (`backend/internal/purchase/purchase_returns.go:326`), padahal CHECK constraint hanya mengizinkan `GRN, SALES_RETURN, DO, PRODUCTION_OUT, PRODUCTION_IN, TRANSFER_IN, TRANSFER_OUT, OPNAME_IN, OPNAME_OUT, ADJUSTMENT`. Setiap purchase return gagal total (transaksi rollback): **stok tidak berkurang dan jurnal reversal tidak pernah tercipta** (diverifikasi: movements tetap 1 baris milik GRN, stock tetap 100). Tidak bisa diuji lebih lanjut karena blocker.

Catatan LOW: enum `refund_method` valid adalah `deduct/refund/credit_balance` — kontrak API perlu didokumentasikan.

## 5. QA-12 — GET /forecast/cash-flow: STILL-BROKEN

```
GET /api/v1/forecast/cash-flow → 500 FORECAST_FAILED
"ERROR: column si.amount_due_cents does not exist (SQLSTATE 42703)"
```

Kolom `supplier_invoices` kini `payable_cents` (dpp/vat/total/dp_applied); referensi `si.amount_due_cents` belum diperbarui.

## 6. QA-13 — GET /production-jobs: STILL-BROKEN

```
GET /api/v1/production-jobs → 500 JOB_LIST_FAILED
log: ERROR: invalid input syntax for type date: "" (SQLSTATE 22007)
```

Akar: `COALESCE(pj.completion_date,'')` di query list (`backend/internal/production/jobs.go:216`) memaksa cast literal `''` ke tipe date → query selalu gagal apa pun datanya.

Sisi tulis sehat: BOM `BOM-001` 201, production-job `JOB-2026-000001` 201 (WIP/FG account resolved). Hanya LIST yang rusak.

## 7. QA-14 — Dashboard widgets/layout: PARTIAL

- `POST /dashboard/widgets` **tanpa config_json** → tetap 500: `null value in column "config_json" ... violates not-null constraint (SQLSTATE 23502)`. Handler membaca field `config` (bukan `config_json`) dan saat kosong mengirim `[]byte(nil)` sebagai NULL eksplisit yang menimpa default kolom `'{}'::jsonb`.
- Workaround: kirim `"config":{}` → 201. ✔
- Layout: GET 200 (default widgets ter-seed), PUT save 200. ✔
- Widget data untuk instance valid: `GET /dashboard/widgets/{id}/data` → 200 (contoh `kpi_ar` → `{"buckets":[],"total_cents":0,...}`). ✔

Verdict PARTIAL: alur utama dashboard jalan bila payload benar, tetapi bug inti QA-14 (payload minimalis → 500 not-null) belum diperbaiki.

## 8. QA-24 / Migration 000057 — Template global: FIXED ✔

Sebagai role terbatas `qa_w4_app` (tanpa BYPASSRLS):

- `GET /reports/templates` dari tenant biasa → **19 template global (tenant_id=0)** terlihat semua. ✔
- Policy 000057 aktif: `report_templates_select/insert/update/delete` (per-command; SELECT = own + global, INSERT/UPDATE/DELETE = own saja). ✔
- Create template sendiri → 201, tersimpan dengan `tenant_id=1000`. ✔
- Render PDF: `POST /reports/templates/9/render` → **502 RENDER_FAILED** ("NextReport engine unreachable: dial tcp [::1]:3100 connection refused") — **expected** di lingkungan lokal tanpa sidecar NextReport; dicatat sesuai instruksi.

Temuan LOW (baru, bukan regresi keamanan): `PUT/DELETE /reports/templates/{id}` terhadap ID global mengembalikan 200 "updated/deleted" padahal 0 baris terpengaruh (handler tidak cek RowsAffected). Data global terverifikasi utuh di DB — dilindungi ganda ( klausa `WHERE tenant_id=$1` di handler + policy RLS 000057).

## 9. QA-16 — Fixed assets: PARTIAL

- `POST /fixed-assets` straight_line **tanpa rate** → tetap 400: `null value in column "rate" of relation "fixed_assets" violates not-null constraint` — rate TIDAK dihitung otomatis dari useful_life_months. (STILL-BROKEN untuk bagian auto-rate.)
- Dengan rate eksplisit `"0.0277778"` → 201. ✔
- Asset register `GET /assets/register` → 200 (book value, akumulasi depresiasi terhitung). ✔

## 10. Lease (PSAK 73)

- Create contract: 201. Validasi ketat: `lessee_name` wajib; `payment_frequency` harus `MONTHLY/QUARTERLY/ANNUALLY`; `discount_rate` desimal 0–1 (`0.10`, bukan persen — catatan UX: instruksi menyebut `annual_discount_rate_pct` tetap API pakai fraksi). PV benar: ROU = Liability = **34.068.459** (annuity 12×5.000.000 @10%/thn), jurnal LEASE_INITIAL #8. ✔
- Post payment no.1: 200, jurnal **LEASE_PAYMENT** #9 (`LEASE-PAY-1-1`); interest 3.406.845 ≈ 34.068.459×(10%/12) ✔; liability sisa 32.475.304. ✔
- `GET /lease-contracts/1/depreciation-log` → 200 (kosong sebelum depresiasi). ✔
- **TEMUAN BARU**: `POST /lease-contracts/1/depreciate` → 500: `column "rou_cost_cents" does not exist (SQLSTATE 42703)` — drift skema antara handler depresiasi dan tabel lease.

## 11. Smoke 16 endpoint pendukung — PASS

Semua GET 200: petty-cash/funds, cheques, warehouses, cost-centers, approval-workflows, pph, aging/ar, aging/ap, bank-statements, financial-notes, recurring, lease-contracts, entity-hierarchy, dimensions, budgets, report-frameworks.

aging/ap menampilkan outstanding 610.000 (Supplier Lengkap, SI-SUP-001, bucket current) — konsisten dengan ledger. ✔

**Temuan BARU (keluarga QA-04)**: `POST /dimensions` (code terisi) → 500: `can't scan into dest[5] (col: created_at): cannot scan timestamptz ... into *string` (`backend/internal/budget/dimensions.go`). Create dimensions rusak meski GET-nya 200.

## 12. Invarian — Trial Balance: PASS

```
GET /reports/trial-balance?as_of_date=2026-08-23
balanced = true ; total_debit_cents = 56.678.459 ; total_credit_cents = 56.678.459
```

Seluruh 9 jurnal (GRN, invoice, payment, lease initial, lease payment, fixed asset) balanced.

---

## Catatan Setup / Observasi Infrastruktur (non-produksi)

Rantai migrasi TIDAK dapat dijalankan end-to-end oleh role NOSUPERUSER NOBYPASSRLS pada DB fresh:

1. `000030` gagal FK: seed template global `tenant_id=0` butuh baris `tenants.id=0` yang baru dibuat `000038`.
2. `000033` & `000050` gagal RLS: tabel `accounts` ber-FORCE ROW LEVEL SECURITY sejak 000001, sehingga owner pun tunduk RLS saat migrasi mengisi baris tanpa GUC `app.tenant_id`.

Workaround QA (mencerminkan praktik prod di mana migrasi jalan sebagai superuser): BYPASSRLS dimatikan-pun... tepatnya: BYPASSRLS **diberikan sementara hanya untuk fase migrasi** lalu **dicabut** sebelum seluruh pengujian runtime; tenant id=0 diseed manual. Ini konsisten dengan insiden 000030 historis; rekomendasi sebelumnya (golang-migrate + role migrasi khusus) tetap relevan.

## Daftar Bug — Prioritas Perbaikan

| # | Bug | Severity | Lokasi |
|---|---|---|---|
| 1 | Payment kedua invoice → 409 DUPLICATE_PAYMENT (source_ref statis per invoice menabrak `journal_entries_intent_unique`) | HIGH | backend/internal/purchase/supplier_payments.go:159 |
| 2 | Purchase return 500 (`PURCHASE_RETURN` tidak ada di CHECK `inventory_movements_movement_type_check`) — return mustahil | HIGH | backend/internal/purchase/purchase_returns.go:326 + constraint |
| 3 | QA-04 supplier NULL-scan (9 kolom opsional wajib-faktual) | HIGH | backend/internal/purchase/suppliers.go |
| 4 | `/forecast/cash-flow` 500 kolom `si.amount_due_cents` | MEDIUM | forecasting handler (query lama) |
| 5 | `/production-jobs` GET 500 `COALESCE(completion_date,'')` cast date | MEDIUM | backend/internal/production/jobs.go:216 |
| 6 | Widget tanpa `config` → 500 not-null (NULL eksplisit menimpa default) | MEDIUM | backend/internal/dashboard/handler.go:384 |
| 7 | Fixed asset rate tidak auto-hitung (not-null rate) | MEDIUM | backend/internal/assets/assets.go |
| 8 | Lease depreciate 500 `rou_cost_cents` missing | MEDIUM | lease depreciation handler vs schema |
| 9 | Dimensions create 500 (created_at timestamptz → *string) | LOW-MED | backend/internal/budget/dimensions.go |
| 10 | Update/delete template lintas-tenant → 200 false-success (RowsAffected tak dicek) | LOW | backend/internal/reports/templates.go:149,175 |
| 11 | Pesan error item check-violation generik menyebut abc_classification untuk pelanggaran costing_method | LOW | backend/internal/item/handler.go:273 |

— Disusun oleh QA Engineer W4; hanya file ini yang ditambahkan pada worktree.
