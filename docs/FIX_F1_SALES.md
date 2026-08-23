# FIX F1 — Sales Module Bug Wave (N1 + QA-05/06/11/15/19)

Tanggal: 2026-08-23 · Worktree: `fix-wave3-f1-sales` · Scope file: `backend/internal/sales/*`, `backend/internal/item/*`
Sumber: `docs/QA_RETEST_2026-08-23_CONSOLIDATED.md`, `docs/QA_RETEST_W3_SALES.md`

## Ringkasan Verdict

| Bug | Judul | Status | Bukti Utama |
|---|---|---|---|
| N1 (HIGH) | Credit note item goods selalu 500 (`idem+"-cogs"` bukan UUID) | **FIXED** | E2E CN goods → 201; jurnal COGS_REVERSAL terposting dengan idem key valid `d74691e3-4163-5a3f-a325-b9722f809c27`; retry key sama → replay CN yang sama. Test: `TestCOGSIdempotencyKey` |
| QA-05 (HIGH) | SQ tanpa `payment_term_id` → 500 NULL-scan | **FIXED** | E2E POST /quotations tanpa payment_term_id → 201 SQ-2026-000001 & SQ-2026-000002 (dua kali). Fix: scan via `pgtype.Int8` di `fetchQuotation` dan `List` |
| QA-06 (HIGH) | Pembayaran kedua invoice → 409 unique intent collision | **FIXED** | E2E PAY#1 parsial → PARTIALLY_PAID; PAY#2 pelunasan → 201 PAID (dulu 409); PAY#3 overpay → 409 `PAYMENT_EXCEEDS_RECEIVABLE` bersih. Test: `TestApplyPayment`, `TestPaymentErrorFor` |
| QA-11 (MED) | DO qty > stok → 500 DELIVERY_CREATE_FAILED | **FIXED** | E2E DO qty 100 > on-hand 50 → 422 `{"code":"INSUFFICIENT_STOCK","message":"insufficient stock on hand: item 1 on_hand=50 need=100"}`; rollback bersih (delivery_orders=0, stok tetap 50). Test: `TestDeliveryErrorFor` |
| QA-15 (MED) | Item costing_method case-sensitive & pesan check-violation menyesatkan | **FIXED** | E2E `costing_method:"FIFO"` → 201, DB tersimpan `fifo`. Pesan per-constraint dipetakan (nama constraint diverifikasi langsung dari pg_catalog). Test: `TestNormalizeItemRequest`, `TestItemCheckViolationMessage`, `TestValidateCreateNormalizedCostingMethod` |
| QA-19 (MED) | SO service item tanpa `sale_account_id` → 409 menyesatkan | **FIXED** | E2E POST /items service tanpa sale_account_id → 400 `{"code":"INVALID_REQUEST","message":"item JAS-002: service requires sale_account_id"}` — item invalid tak bisa lagi tercipta sehingga jalur SO-409-menyesatkan tak terjangkau. Test: `TestValidateCreateService/service_missing_sale_account_(QA-19)` |

Gate: `gofmt -l internal/sales internal/item` bersih · `go vet ./...` OK · `go build ./...` OK · `go test ./...` semua PASS.

## File yang Diubah

Kode produksi:
1. `backend/internal/sales/credit_notes.go` — helper `cogsIdempotencyKey()` (UUIDv5 SHA-1, namespace `uuid.NameSpaceOID`, atas string `idem+"-cogs"`); import `github.com/google/uuid`.
2. `backend/internal/sales/sales.go` — `fetchQuotation()` + `List()`: kolom `payment_term_id` discan ke `pgtype.Int8`, di-set ke response bila Valid.
3. `backend/internal/sales/payments.go` — source_ref jurnal = nomor PMT unik (`nextPMTNumber` dialokasikan lebih awal; dulu `PMT-{invoiceID}` statis); validasi overpay via `applyPayment()` + sentinel `errPaymentExceedsReceivable`; mapping `paymentErrorFor` → 409 `PAYMENT_EXCEEDS_RECEIVABLE`.
4. `backend/internal/sales/delivery.go` — `deliveryErrorFor`: `costing.ErrInsufficientStock` → 422 `INSUFFICIENT_STOCK`.
5. `backend/internal/item/handler.go` — `normalizeItemRequest()` (lowercase+trim costing_method sebelum validate/insert); peta `itemCheckMessages` nama-constraint → pesan spesifik + `itemCheckViolationMessage()`; `validateCreate`: service wajib `sale_account_id`.

Test:
6. `backend/internal/sales/credit_notes_test.go` — `TestCOGSIdempotencyKey` (valid/deterministik/distinct).
7. `backend/internal/sales/payments_test.go` — ganti test aritmetika overpay lama dengan `TestApplyPayment` + `TestPaymentErrorFor` (tabel-driven).
8. `backend/internal/sales/delivery_test.go` — `TestDeliveryErrorFor` (tabel-driven).
9. `backend/internal/item/item_test.go` — `TestNormalizeItemRequest`, `TestItemCheckViolationMessage`, `TestValidateCreateNormalizedCostingMethod`, kasus baru di `TestValidateCreateService`.

Laporan: `docs/FIX_F1_SALES.md` (file ini). Tidak ada file lain; TASK_LEDGER tidak disentuh (dikoordinasikan terpusat).

## Bukti E2E (DB terisolasi, meniru prod C3)

Lingkungan: DB `finance_qa_fx1` (migrasi per-file sebagai superuser; hanya 000030 gagal mid-file = known issue QA-21), role API **qa_fx1_app LOGIN NOSUPERUSER NOBYPASSRLS** + GRANT USAGE/ALL TABLES/ALL SEQUENCES/ALTER DEFAULT PRIVILEGES, server build worktree ini di `:18091`, tenant **W-F1** (tenant_id=1000) via register. Semua request sebagai role terbatas.

| Langkah | Hasil HTTP |
|---|---|
| Register tenant W-F1 | 201, tenant_id 1000 |
| Customer CUST-001 / Payment term NET30 | 201 / 201 |
| Item goods fifo (BRG-001), item service dengan sale_account (JAS-001) | 201 / 201 |
| **QA-19**: item service tanpa sale_account_id (JAS-002) | **400** `item JAS-002: service requires sale_account_id` |
| **QA-15**: item goods `"costing_method":"FIFO"` (BRG-FIFO) | **201**, DB: `SELECT costing_method FROM items` → `fifo` |
| Supplier lengkap → PO 50 pcs @80rb → GRN | 201/201/201; stock_balances qty_on_hand=50.000 |
| **QA-05**: SQ tanpa payment_term_id | **201** SQ-2026-000001 (dan SQ#2 201) |
| Send SQ → convert SO | 200 / 201 SO-2026-000001 CONFIRMED |
| **QA-11**: DO qty 100 (> on-hand 50) | **422** INSUFFICIENT_STOCK; rollback: delivery_orders=0, stok tetap 50 |
| Invoice dari SO (10×150rb, PPN 11%) | 201 INV-2026-000001 ISSUED, receivable 1.665.000 |
| **QA-06**: PAY#1 parsial 500rb | **201** PMT-2026-000001 → invoice PARTIALLY_PAID (receivable 1.165.000) |
| **QA-06**: PAY#2 pelunasan 1.165rb | **201** PMT-2026-000002 (dulu 409 collision) → invoice **PAID**, receivable 0 |
| **QA-06**: PAY#3 overpay 999jt | **409** `PAYMENT_EXCEEDS_RECEIVABLE` "payment exceeds outstanding receivable: amount 999000000 exceeds receivable 0" (bukan raw SQLSTATE) |
| **N1**: CN 1 pcs goods pada INV#2 (refund credit_balance, cost fallback 80rb) | **201** CN-2026-000001 APPLIED, cogs_reversed_cents 80.000 |
| Verifikasi jurnal CN (psql GUC app.tenant_id='1000') | SALES_RETURN `CN-REV-2`: Dr 4201/Cr 2402 @150rb; COGS_REVERSAL `CN-COGS-2`: Dr 1301/Cr 5101 @80rb; idempotency_key COGS = `d74691e3-4163-5a3f-a325-b9722f809c27` (**UUID valid hasil turunan**) |
| Retry CN dengan Idempotency-Key sama | Replay CN id 1 CN-2026-000001 → tidak ada jurnal duplikat |
| Invarian akhir | Trial balance `balanced=true` 9.225.000=9.225.000; 16/16 jurnal POSTED, Σdebit=Σkredit |

## Keputusan Teknis

1. **N1 — UUID kedua deterministik**: `uuid.NewSHA1(uuid.NameSpaceOID, []byte(idem+"-cogs")).String()` (UUIDv5). Input sama → UUID sama, sehingga retry tetap idempoten; berbeda dari key primer; kolom uuid menerima nilai valid. `github.com/google/uuid` sudah ada di go.mod (sebelumnya indirect) — tidak ada dependensi baru.
2. **QA-05 — minimal**: tetap scan ke field `PaymentTermID int64` pada response (JSON selalu menyertakan field; NULL → 0), hanya mekanisme scan yang diganti `pgtype.Int8`. Pola ini identik dengan `fetchOrder` di orders.go.
3. **QA-06 — dua sisi**: (a) source_ref jurnal kini memakai nomor `PMT-{year}-{seq}` unik per pembayaran (pola document_numbering yang sama dengan modul lain), menghilangkan tabrakan `journal_entries_intent_unique (tenant_id, source_ref, intent_type)`; (b) **overpay ditolak 409 bersih** sesuai arahan tugas (menyelesaikan N8 OBS dari laporan konsolidasi: perilaku 201+overpayment_cents dinilai regresi semantik, bukan desain). Kolom `overpayment_cents` tetap ada di schema/response (selalu 0) agar kompatibel; cabang jurnal overpayment dihapus.
4. **QA-11 — 422 dipilih** (Unprocessable Entity, semantik bisnis "permintaan valid tapi state tak memungkinkan"); deteksi tetap berasal dari `costing.ErrInsufficientStock` sebelum journal/costing dieksekusi, jadi rollback selalu bersih.
5. **QA-15 — peta constraint statis**: nama CHECK diverifikasi langsung dari pg_catalog database E2E (`items_item_type_check`, `items_costing_method_check`, `items_revenue_recognition_method_check`, `items_abc_classification_check`, `items_check`, `items_check1`); constraint tak-dikenal jatuh ke pesan generik yang menyebut nama constraintnya.
6. **QA-19 — validasi di create item dipilih** (paling kecil, daripada resolusi default saat posting): service tanpa `sale_account_id` ditolak 400 dengan pesan menyebut item & field, sehingga item bermasalah tak pernah bisa tercipta dan 409 menyesatkan di SO tak lagi terjangkau.
7. **Catatan observasi (di luar scope, tidak diubah)**: guard M-008 "credit note total %d exceeds outstanding receivable %d" masih diklasifikasi 500 INTERNAL_ERROR oleh `cnErrorFor` — kelas masalah yang sama dengan QA-11 namun di jalur CN; layak masuk wave error-classification berikutnya (httperr.Classify menyeluruh, prioritas #4 laporan konsolidasi).
8. QA-04 supplier NULL-scan (STILL-BROKEN, scope W4/purchase) sengaja tidak disentuh; E2E memakai supplier dengan semua field terisi untuk menyiapkan stok.

## Reproduksi

```text
psql -h localhost -d postgres -c "CREATE DATABASE finance_qa_fx1;"
for f in backend/migrations/*.up.sql; do psql -h localhost -d finance_qa_fx1 -v ON_ERROR_STOP=1 -q -f "$f"; done   # 000030 gagal = known issue
psql -h localhost -d finance_qa_fx1 -c "CREATE ROLE qa_fx1_app LOGIN PASSWORD 'qapass123' NOSUPERUSER NOBYPASSRLS; GRANT USAGE ON SCHEMA public TO qa_fx1_app; GRANT ALL ON ALL TABLES IN SCHEMA public TO qa_fx1_app; GRANT ALL ON ALL SEQUENCES IN SCHEMA public TO qa_fx1_app;"
DATABASE_URL=postgres://qa_fx1_app:qapass123@localhost:5432/finance_qa_fx1?sslmode=disable \
JWT_SECRET="fx1-secret-key-32chars-minimum-xxxxx" HTTP_ADDR=":18091" go run ./cmd/api   # workdir backend
```
