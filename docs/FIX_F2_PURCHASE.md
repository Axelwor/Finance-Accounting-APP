# FIX F2 — Purchase Module (FIX-WAVE-003)

- Sesi: F2 (`fix-wave3-f2-purchase`), basis HEAD `78053fe`
- Lingkup kode: `backend/internal/purchase/*` + **satu migration baru** (sesuai penugasan; TASK_LEDGER tidak disentuh, tidak ada commit)
- Lingkungan E2E: DB `finance_qa_fx2`, role API `qa_fx2_app` (**NOSUPERUSER NOBYPASSRLS**, diverifikasi `rolsuper=f, rolbypassrls=f`), migrasi di-apply per-file sebagai superuser (tenant 0 di-seed sebelum 000030 sesuai known issue), GRANT USAGE schema + ALL TABLES/SEQUENCES, API utama port **:18092** (tenant **W-F2 Corp**, `tenant_id=1000`), probe port **:18097** (tenant **W-F2 Probe**, `tenant_id=1001`)

## Ringkasan Verdict

| ID | Bug | Verdict |
|---|---|---|
| N2 | Purchase return selalu 500 — `PURCHASE_RETURN` tidak ada di CHECK `inventory_movements_movement_type_check` | **FIXED** |
| QA-04 | Supplier create/update NULL-scan — 9+ kolom opsional wajib-faktual; supplier minimal mustahil dibuat | **FIXED** |
| QA-06 | Payment kedua supplier-invoice → 409 `DUPLICATE_PAYMENT` salah pesan (`PAY-{invoiceID}` statis menabrak `journal_entries_intent_unique`) | **FIXED** |

## 1. N2 — Purchase Return 500 (CHECK constraint) → FIXED

**Perubahan:** migration `backend/migrations/000058_purchase_return_movement.{up,down}.sql` — drop + re-add constraint `inventory_movements_movement_type_check` dengan `'PURCHASE_RETURN'` ditambahkan tepat setelah `'SALES_RETURN'` (simetris dengan sisi penjualan). Down migration memulihkan daftar enum original.

**Bukti:**
- DB: `pg_get_constraintdef` menunjukkan `PURCHASE_RETURN` dalam daftar; INSERT `movement_type='PURCHASE_RETURN'` sebagai role `qa_fx2_app` (dengan GUC `app.tenant_id`) sukses, nilai invalid tetap ditolak CHECK (SQLSTATE 23514).
- HTTP (probe :18097, tenant 1001): `POST /purchase-returns` 10 pcs × 10.000 → **201** `PRET-2026-000001` (`total_cents=100000`, `vat_reversed_cents=11000`); return kedua di invoice yang sama juga **201** `PRET-2026-000002` (lihat Keputusan 3 soal source_ref).
- psql `stock_balances` tenant 1001 item 2: **189 = 100 + 100 (2×GRN) − 10 − 1 (2×return)** — stok benar-benar berkurang.
- psql `inventory_movements`: 2 baris `PURCHASE_RETURN` qty `-10`/`-1`, `source_ref` = nomor PRET masing-masing.
- psql `journal_entries`: jurnal reversal `intent_type='PURCHASE_RETURN'` id 17 & 18 dengan `source_ref` unik per return; drift trial balance tenant = **0**.
- Unit: seluruh `purchase_returns_test.go` tetap hijau (perubahan N2 murni skema).

**Catatan dependensi (penting):** di server utama :18092 (kode worktree murni), return berhenti di **`costing: ResolveCOGS layer update: conn busy`** — itu **QA-03** di `backend/internal/costing/consumeFIFO` (Exec di dalam loop rows terbuka), milik sesi **F3**, di luar scope file saya sehingga tidak disentuh. Error ini terjadi SETELAH jurnal + movement + supplier-balance sukses di-insert (rollback bersih), yang sekaligus membuktikan CHECK constraint sudah tidak lagi menjadi blocker (sebelumnya gagal lebih dulu di 23514). Bukti end-to-end lengkap (201 + stok + jurnal) diambil lewat **probe**: salinan worktree di `/tmp/kilo/fx2-probe` dengan satu patch throwaway minimal pada `consumeFIFO` (buffer layers sebelum Exec) + koreksi param `$3`→`$4` pada query layer update.

**Intel untuk F3:** di bawah conn-busy tersembunyi bug kedua — query `UPDATE inventory_cost_layers SET qty_remaining = $3 WHERE tenant_id=$1 AND id=$2 AND warehouse_id=$3` dipanggil dengan **4 argumen** (`tenantID, layerID, warehouseID, newRemaining`): `qty_remaining` dan `warehouse_id` memakai `$3` yang sama → `mismatched param and argument count` segera setelah conn-busy diperbaiki. F3 perlu mengubahnya menjadi `SET qty_remaining = $4`.

## 2. QA-04 — Supplier NULL-scan → FIXED

**Perubahan (`suppliers.go`, `helpers.go`):** fix sistematis, bukan tambal per-kolom:
- `supplierColumns` (konstanta kolom RETURNING/SELECT) + `supplierRow` (semua kolom nullable bertipe `pgtype.Text`/`pgtype.Int8`/`pgtype.Date`) + `supplierScanDest()` + `supplierRow.response()` — dipakai bersama oleh Create, List, Get, dan Update; tidak ada lagi scan NULL ke `*string`/`*int64` non-null.
- `normalizeSupplierType()`: input di-uppercase/trim sebelum insert — lowercase `"goods"` lolos dan tersimpan `GOODS`.
- `validateSupplierRequest`: `supplier_type` wajib salah satu `GOODS/SERVICE/MIXED` (case-insensitive) bila diisi → 400 bersih `supplier_type must be one of GOODS, SERVICE, MIXED`, tanpa menyentuh DB.
- Defense-in-depth: helper baru `isCheckViolation` (SQLSTATE 23514) di `helpers.go`; residu check-violation di create/update dipetakan ke 400 bersih, bukan raw SQLSTATE.

**Bukti:**
- HTTP: `POST /suppliers` hanya `{"code","name"}` → **201** dengan semua opsional `""`/0 (tenant 1000 id 1 & tenant 1001 id 13); `supplier_type:"goods"` → **201** `supplier_type:"GOODS"`; `"company"` → **400** pesan validasi bersih tanpa SQLSTATE.
- Unit: `TestMinimalSupplierRequestIsValid`, `TestValidateSupplierTypeRule`, `TestNormalizeSupplierType`, `TestSupplierRowResponseMapsNullOptionals`, `TestSupplierRowResponseMapsFilledOptionals`, `TestSupplierScanDestMatchesColumnOrder` — semua hijau.

## 3. QA-06 — Payment kedua 409 DUPLICATE_PAYMENT → FIXED

**Perubahan (`supplier_payments.go`):**
- Nomor payment dialokasikan **sebelum** jurnal via `nextPayNumber` (document_numbering, `PAY-YYYY-NNNNNN`) dan dipakai sebagai `source_ref` jurnal + kolom `supplier_payments.source_ref` — setiap payment dapat ref unik, pelunasan bertahap jalan.
- Overpay **ditolak** sesuai instruksi ("tetap ditolak dengan validasi bisnis bersih"): helper murni `splitPaymentAmount` mengembalikan `*paymentExceedsPayableError` bila amount > payable, dipetakan ke **409 `PAYMENT_EXCEEDS_PAYABLE`** dengan pesan bersih (menyebut kedua angka, tanpa SQLSTATE). Jalur booking `overpayment_cents`/akun 1204 dihapus dari flow ini (kolom tetap ada, selalu 0) — sekaligus menjawab N8 OBS: keputusan produk = tolak overpay.
- Status invoice: parsial → `PARTIALLY_PAID`, lunas → `PAID` (logika existing dipertahankan).

**Bukti:**
- HTTP (tenant 1001, invoice BIL-2026-000002 payable 1.110.000): PAY#1 500.000 → 201 `PAY-2026-000003`, invoice `PARTIALLY_PAID` payable 610.000; PAY#2 610.000 → **201 `PAY-2026-000004`** (sebelumnya 409 DUPLICATE_PAYMENT), invoice **`PAID`** payable 0; PAY#3 100.000 → 409 `PAYMENT_EXCEEDS_PAYABLE` pesan bersih. Di tenant 1000 (:18092) alur sama: PAY#1 `PAY-2026-000001`, PAY#2 `PAY-2026-000002`, invoice PAID.
- psql: 4 jurnal `SUPPLIER_PAYMENT` dengan `source_ref` unik `PAY-2026-000001..000004`; `supplier_payments.source_ref` = nomor payment.
- Unit: `TestSplitPaymentAmount` (parsial/lunas/overpay + pesan + tanpa SQLSTATE), `TestSupplierInvoiceStatusAfterPayment` (termasuk skenario pelunasan tahap kedua 610.000), `TestSupplierPaymentSourceRefIsUniquePerPayment` — semua hijau.

## File Diubah

| File | Perubahan |
|---|---|
| `backend/migrations/000058_purchase_return_movement.up.sql` | **BARU** — enum `PURCHASE_RETURN` di CHECK |
| `backend/migrations/000058_purchase_return_movement.down.sql` | **BARU** — restore constraint original |
| `backend/internal/purchase/suppliers.go` | QA-04: `supplierRow`/`supplierScanDest`/`response()`, `normalizeSupplierType`, validasi type, mapping check-violation |
| `backend/internal/purchase/suppliers_test.go` | **BARU** — 6 unit test QA-04 |
| `backend/internal/purchase/supplier_payments.go` | QA-06: nomor payment → source_ref, `splitPaymentAmount`, 409 `PAYMENT_EXCEEDS_PAYABLE` |
| `backend/internal/purchase/supplier_payments_test.go` | Test baru/ulang untuk QA-06 |
| `backend/internal/purchase/purchase_returns.go` | source_ref PRET statis → nomor PRET unik (pola QA-06 yang sama, lihat Keputusan 3) |
| `backend/internal/purchase/helpers.go` | `isCheckViolation` (23514) |
| `docs/FIX_F2_PURCHASE.md` | Laporan ini |

## Keputusan

1. **Scan strategy satu jalur**: daripada menambal scan per-handler, semua akses supplier (create/list/get/update) melewati `supplierRow` + `pgtype.*` + mapper `response()` — NULL → `""`/0 konsisten di semua endpoint, dan kolom baru tinggal tambah di satu konstanta.
2. **`supplier_type` kosong → NULL** (bukan dipaksa `GOODS`): menghormati semantik "tidak diisi"; default DB `'GOODS'` tidak dipaksa dari handler.
3. **PRET source_ref ikut diperbaiki**: `purchase_returns.go` memiliki bug keluarga yang sama persis dengan QA-06 (`PRET-{invoiceID}` statis → return kedua di invoice yang sama pasti menabrak `journal_entries_intent_unique`). Diperbaiki dengan pola yang sama (nomor PRET sebagai source_ref jurnal + movement), dibuktikan return kedua 201. Masuk scope file purchase.
4. **Overpay ditolak, bukan dibukukan** (409 `PAYMENT_EXCEEDS_PAYABLE`) sesuai instruksi penugasan; menuntut N8 OBS dengan keputusan produk eksplisit.
5. **QA-03 tidak disentuh**: blocker return di server murni ada di `internal/costing` (F3); bukti end-to-end memakai probe terpisah di luar repo dengan patch throwaway, tanpa mengubah diff branch ini.

## Gate & Verifikasi

- `gofmt -l .` bersih; `go vet ./...`, `go build ./...`, `go test ./...` — **semua paket lulus, 0 gagal** (paket purchase: 157 subtest hijau).
- E2E probe :18097: **21/21 PASS** (script `/tmp/kilo/fx2-e2e-final.py`); server utama :18092: 19/23 (4 gagal hanya di leg return karena QA-03 costing, lihat §1).
- Server berjalan via background_process persistent: utama `bgp_02dab5aec001v13UR71gtOKde1` (:18092), probe `bgp_02db46a40001M6ahnwKDkAwjNs` (:18097).
