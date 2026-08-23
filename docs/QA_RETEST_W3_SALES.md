# QA Retest W3 — Sales Chain (SQ→SO→DP→DO→INV→PAY→CN)

Tanggal: 2026-08-23 · Lingkup: retest bug Sales Chain dari `docs/QA_REPORT_2026-08-23.md` §4.5 + regresi.
Lingkungan: DB lokal `finance_qa_rw3`, role **qa_w3_app LOGIN NOSUPERUSER NOBYPASSRLS** (simulasi Phase C cutover), API build dari worktree ini di port :18083, tenant baru "W3 Corp" via register (tenant_id=1000).

## Ringkasan Verdict

| Bug | Judul | Verdict |
|---|---|---|
| QA-03 | DO selalu gagal "conn busy" FIFO | **STILL-BROKEN** (prioritas tertinggi) |
| QA-05 | Quotation tanpa `payment_term_id` → 500 | **STILL-BROKEN** |
| QA-06 | Pembayaran kedua invoice → 409 unique intent | **STILL-BROKEN** |
| QA-11 | Insufficient stock saat DO → 500 bukan 4xx | **STILL-BROKEN** |
| QA-15 | Validasi item menyesatkan & case-sensitive | **STILL-BROKEN** |
| QA-19 | SO service tanpa `sale_account_id` → 409 menyesatkan | **STILL-BROKEN** |
| QA-04* | Supplier create NULL-scan | **STILL-BROKEN*** (pendukung chain purchase) |
| — | Credit note goods item selalu 500 UUID `-cogs` | **NEW** |

\* QA-04 bukan target utama sesi ini namun terverifikasi ikut rusak saat menyiapkan data.

Tidak ada regresi terhadap alur yang sebelumnya sehat: register di bawah role terbatas, SQ send/convert, DP, GRN/stok, invoice + realisasi DP, payment pertama, negative-qty validation, quotation-stats, trial balance — semuanya tetap lulus.

---

## Hasil per Kasus (urutan eksekusi)

### 1. QA-05 — Quotation tanpa payment_term_id · ❌ STILL-BROKEN

- `POST /quotations` payload valid tanpa `payment_term_id` → **500**
  ```json
  {"code":"QUOTATION_CREATE_FAILED","message":"An internal error occurred. Please try again or contact support."}
  ```
- Log API: `raw_message:"can't scan into dest[6] (col: payment_term_id): cannot scan NULL into *int64"` — akar masalah identik dengan laporan awal (NULL-scan pada re-read).
- Dengan `payment_term_id` → **201** normal (SQ-2026-000001).

### 2. SQ → Send → Convert SO · ✅ LULUS

- `POST /quotations/2/send` → 200 status SENT.
- `POST /sales-orders` dengan `quotation_id` (+Idempotency-Key) → **201**, `SO-2026-000001`, status **CONFIRMED**, total 1.500.000.
- `GET /quotations/2` → status **CONVERTED**.

### 3. DP (Down Payment) · ✅ LULUS

- `POST /sales-orders/1/down-payments` {cash_account_id:11, amount 500.000} → **201** `DP-2026-000001` RECEIVED + jurnal (entry 1).
- Exceed: amount 999.000.000 → **409** `{"code":"DP_EXCEEDS_ORDER","message":"down payment exceeds remaining order total (1000000 cents)"}` ✅ kelas & pesan benar.
- Catatan teknis: endpoint wajib header `Idempotency-Key`; tanpa itu 400 (perilaku konsisten, bukan bug).

### 4. Purchase pendukung: Supplier / PO / GRN · ⚠️ QA-04 MASIH RUSAK, PO/GRN OK

- **QA-04 STILL-BROKEN**: `POST /suppliers` tanpa satu saja kolom opsional → **400** NULL-scan mentah, kini `can't scan into dest[8] (col: city): cannot scan NULL into *string`. Hanya sukses bila **semua** field terisi; tambahan temuan: `supplier_type` harus uppercase `GOODS|SERVICE|MIXED` (lowercase `distributor` → 400 dengan raw `(SQLSTATE 23514)` bocor ke client).
- `POST /purchase-orders` (supplier_id=3, 50 pcs @80.000, `unit_price_cents`) → 201 `PO-2026-000002`, total 4.000.000. (PO pertama salah field dibatalkan → CANCELLED bersih.)
- `POST /goods-received-notes` → **201** `GRN-2026-000001` RECEIVED + jurnal Dr Inventory/Cr Uninvoiced Payables.
- Verifikasi psql (dengan GUC `app.tenant_id='1000'`): `stock_balances.qty_on_hand=50.000`, `avg_unit_cost_cents=80000`. ✅

### 5. QA-03 — Delivery Order normal · ❌ STILL-BROKEN (PRIORITAS TERTINGGI)

- `POST /delivery-orders` qty 10 unit_cost 80.000 (stok tersedia 50) → **500**, 4/4 percobaan deterministik:
  ```json
  {"code":"DELIVERY_CREATE_FAILED","message":"An internal error occurred. Please try again or contact support."}
  ```
- Log API: `raw_message:"costing: ResolveCOGS layer update: conn busy"` — **akar masalah persis sama** (pgx v5: Exec dalam loop iterasi rows pada transaksi yang sama di `consumeFIFO`).
- Rollback bersih terverifikasi: `delivery_orders` tetap 0 baris, stok tetap 50.000.
- Log PostgreSQL (`/usr/local/var/log/postgresql@16.log`) tidak mencatat error terkait (kegagalan murni driver-level, bukan server).
- **Dampak:** seluruh rantai pengiriman barang berstok tetap mati; COGS penjualan tidak pernah terposting; invoice tidak bisa dikaitkan `delivery_id`.

### 6. QA-11 — DO melebihi stok · ❌ STILL-BROKEN

- DO qty 100 (> on-hand 50) → **500** `DELIVERY_CREATE_FAILED`.
- Log API: `raw_message:"insufficient stock on hand: item 1 on_hand=50 need=100"` — deteksi bisnis benar dan cepat (gagal sebelum menyentuh costing), tapi kelas error tetap 500 generik. Belum ada kode `INSUFFICIENT_STOCK` 4xx.

### 7. Invoice dari SO · ✅ LULUS (dengan catatan)

- `POST /invoices` {sales_order_id:1, lines 10×150.000 tax 11%} → **201** `INV-2026-000001` **ISSUED**.
- **Realisasi DP benar:** `dp_applied_cents=500.000`, `receivable_cents=1.165.000` (= total 1.665.000 − DP 500.000). ✅
- Catatan: `delivery_id` per line tidak dapat diuji karena DO mati (QA-03). Tanpa `customer_id` request ditolak 400 wajar meski `sales_order_id` sudah membawa customer.

### 8. QA-06 — Pembayaran bertahap · ❌ STILL-BROKEN

- PAY#1 parsial 500.000 → **201** `PMT-2026-000001`; invoice menjadi **PARTIALLY_PAID**, sisa receivable 665.000. ✅
- PAY#2 pelunasan 665.000 → **409**
  ```json
  {"code":"CONFLICT","message":"ERROR: duplicate key value violates unique constraint \"journal_entries_intent_unique\" (SQLSTATE 23505)"}
  ```
  — collision `source_ref/intent` yang sama persis seperti laporan awal; raw SQLSTATE masih bocor di body. Invoice terjebak PARTIALLY_PAID.
- PAY#3 overpay 999jt pada invoice yang sama → 409 juga, namun karena **collision constraint yang sama**, bukan validasi bisnis — jalur validasi overpay multi-payment tak terjangkau selama QA-06 ada.
- Perilaku baru (observasi): overpay sebagai **pembayaran pertama** pada invoice lain → **201 RECEIVED** dengan `overpayment_cents` dicatat (99.833.499 dari 999 jt). Laporan awal mencatat overpay → 409; perilaku kini berubah menjadi diterima + tracking kembalian. Perlu konfirmasi produk apakah ini desain baru atau regresi semantik.

### 9. Credit Note 1 pcs · ❌ GAGAL — BUG BARU

- `POST /credit-notes` (invoice 1, line 1, qty 1, refund_method `credit_balance`, unit_cost fallback 80.000) → **500**
  ```json
  {"code":"INTERNAL_ERROR","message":"An internal error occurred. Please try again or contact support."}
  ```
- Log API: `raw_message:"ERROR: invalid input syntax for type uuid: \"dddddddd-0000-4000-8000-000000000001-cogs\" (SQLSTATE 22P02)"`.
- **Akar masalah (kode):** `backend/internal/sales/credit_notes.go:396` → `cogsIdemKey := idem + "-cogs"` lalu di-INSERT ke kolom `journal_entries.idempotency_key` bertipe **uuid**. Karena header Idempotency-Key harus UUID valid, sufiks `-cogs` selalu menghasilkan UUID tidak valid → **seluruh CN untuk item goods gagal 500**. Jurnal revenue reversal & pengembalian stok tidak dapat diverifikasi (transaksi rollback bersih: `credit_notes` 0 baris, stok tetap 50).
- Tidak ada di laporan QA awal → **temuan baru**.

### 10. QA-15 — Item costing_method "FIFO" uppercase · ❌ STILL-BROKEN

- `POST /items` dengan `"costing_method":"FIFO"` → **400** pesan yang sama menyesatkan:
  ```json
  {"code":"ITEM_INVALID_FIELD","message":"invalid field value (check abc_classification)"}
  ```
- Tidak dinormalisasi ke lowercase; user tetap diarahkan ke kolom yang salah (`abc_classification`). Item kontrol lowercase `fifo` → 201 normal.

### 11. QA-19 — SO service item tanpa sale_account_id · ❌ STILL-BROKEN

- ITEM-SVC (service, `sale_account_id` NULL, terverifikasi psql) dipakai di `POST /sales-orders` → **409**
  ```json
  {"code":"INVALID_REQUEST","message":"order references a resource that does not exist for this tenant"}
  ```
- Pesan sama seperti laporan awal — menyesatkan (menuduh masalah tenancy/lookup); validasi "service requires sale_account_id" belum ada di create item maupun create order.

### 12. Validasi & stats · ✅ LULUS

- Invoice dengan line qty −5 → **400** `lines[0]: qty must be greater than 0`. ✅
- `GET /reports/quotation-stats` → **200**, `{"total":1,"converted":1,"conversion_rate_pct":100,...}`. ✅

### 13. Invarian akhir · ✅ LULUS

- `GET /reports/trial-balance?as_of=2026-08-23` → **200**, `balanced=true`, total debit = total kredit = 107.331.499.
- psql (GUC tenant 1000): semua 17 jurnal POSTED, `sum(debit)=sum(credit)=107.331.499`, diff **0**. ✅

---

## Catatan Lingkungan (bukan blokir scope sales)

- Rantai migrasi masih tidak replay-clean di DB fresh (QA-21 persisten):
  - `000030_reporting_dashboard` berhenti di seed `report_templates tenant_id=0` (FK violation) — tabel dashboard tetap ter-create setelah eksekusi ulang tanpa ON_ERROR_STOP;
  - seed akun global `000049`–`000052` ditolak RLS oleh role terbatas; berhasil hanya dengan `SET app.tenant_id='0'` per sesi.
  - Ini relevan bagi deploy.sh yang menjalankan migrasi sebagai role aplikasi terbatas pasca-cutover.
- Semua query psql verifikasi data wajib `SET app.tenant_id='1000'` karena forced RLS (bukti isolasi bekerja).

## Kesimpulan

Enam bug sales yang diretest **belum satupun diperbaiki** (QA-03, QA-05, QA-06, QA-11, QA-15, QA-19 — semua STILL-BROKEN dengan akar masalah & pesan log identik). Satu **bug baru** ditemukan pada credit note (UUID `-cogs` tidak valid, selalu 500 untuk item goods) dan satu **perubahan perilaku** overpay-first-payment (201 + overpayment_cents vs 409 pada laporan awal) yang perlu dikonfirmasi sebagai desain atau regresi. Invarian akuntansi tetap utuh di seluruh skenario (balance, rollback bersih, isolasi tenant, idempotency).

*Tidak ada perubahan kode produksi, tidak ada commit, TASK_LEDGER tidak disentuh. Artefak uji: `/var/folders/kv/21q1swqs3k393063ylh1ptt40000gn/T/kilo/qa/w3/`.*
