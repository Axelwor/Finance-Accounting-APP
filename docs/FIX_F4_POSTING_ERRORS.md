# FIX F4 — Error-Classification & Validasi Alur Posting Inti

Worktree: `fix-wave3-f4-posting-errors` · Tanggal: 2026-08-23 · Scope:
`backend/internal/cash`, `backend/internal/period`, `backend/internal/accounting`
(handler manual journal), `backend/internal/coa`. Prinsip: hanya klasifikasi
error dan validasi input — perilaku posting/invarian engine tidak diubah.

Sumber temuan: `QA_RETEST_W2_CORE.md` (QA-07/08/09/10), `QA_RETEST_2026-08-23_CONSOLIDATED.md` (N5/N6), `QA_RETEST_W1_AUTH.md` (NEW-1 = N5).

## Ringkasan Status

| # | Bug | Sev | Status |
|---|-----|-----|--------|
| 1 | QA-07 double-reverse jurnal VOID → 500 | MED | **FIXED** → 409 `JOURNAL_NOT_POSTED` |
| 2 | QA-08 entry_date luar periode OPEN → 404 salah arah | MED | **FIXED** → 422 `ENTRY_DATE_OUTSIDE_OPEN_PERIOD` |
| 3 | QA-09 close tanpa Idempotency-Key → raw SQL leak | MED | **FIXED** → 400 `INVALID_REQUEST` bersih |
| 4 | QA-10 account_type tidak divalidasi → tersimpan | MED | **FIXED** → 400 + daftar nilai valid |
| 5 | N5/NEW-1 jurnal manual lintas-tenant → 500 | MED-LOW | **FIXED** → 404 `ACCOUNT_NOT_FOUND` |
| 6 | N6 transfer non-CASH/BANK → 500 | LOW | **FIXED** → 422 `ACCOUNT_TYPE_MISMATCH` |

## Perubahan per Bug

### 1. QA-07 — Double-reverse jurnal VOID
Guard bisnisnya sudah ada (`cash/handler.go`: status != POSTED) tapi error polos
dipetakan `errorFor()` ke 500. Kini sentinel `errJournalNotPosted` (cash/helpers.go)
di-wrap oleh guard, dan `errorFor()` memetakannya ke **409 JOURNAL_NOT_POSTED**
("only POSTED journals can be reversed") sebelum fallback ErrNoRows.
- File: `backend/internal/cash/handler.go`, `backend/internal/cash/helpers.go`

### 2. QA-08 — Tanggal di luar periode terbuka / posting pasca-close
Akar: `resolvePeriod()` gagal dengan ErrNoRows lalu tertelan fallback
"ACCOUNT_NOT_FOUND". Sentinel baru `accounting.ErrEntryDateOutsideOpenPeriod`
didefinisikan sekali (engine.go) dan di-wrap oleh kedua implementasi
`resolvePeriod` (`cash/journal.go`, `accounting/helpers.go`). Pemetaan:
`cash/errorFor()` → **422 ENTRY_DATE_OUTSIDE_OPEN_PERIOD**;
handler jurnal manual → kode sama via `classifyPostingError()`. Urutan cek
sentinel selalu sebelum fallback ErrNoRows karena resolvePeriod membungkus
ErrNoRows.
- File: `backend/internal/accounting/engine.go`, `backend/internal/accounting/helpers.go`,
  `backend/internal/accounting/journal_manual.go`, `backend/internal/cash/helpers.go`,
  `backend/internal/cash/journal.go`

### 3. QA-09 — `/periods/close` tanpa Idempotency-Key
Handler Close membaca header mentah sehingga string kosong mencapai kolom uuid
(`SQLSTATE 22P02` bocor ke respons). Helper `idempotencyKey()` package period —
yang sebelumnya hanya dipakai Unlock — kini dipanggil Close juga; header hilang
atau bukan UUID ditolak **400 INVALID_REQUEST** tanpa menyentuh SQL. Unlock
sudah benar sejak m-021 dan diverifikasi ulang tetap bersih.
- File: `backend/internal/period/handler.go`

### 4. QA-10 — account_type bebas tersimpan
Tabel `accounts` tidak punya CHECK untuk account_type. Enum otoritatif kini
divalidasi di `coa.validateAccountInput`: 38 nilai yang benar-benar dipakai
sistem (seed `auth.SeedDefaultCOA` + semua akun yang di-provision migrasi
000001–000057) dikumpulkan di `validAccountTypes`; nilai lain ditolak
**400 INVALID_REQUEST** dengan pesan berisi daftar lengkap + nilai yang
melanggar. `report_group` sudah tervalidasi CHECK-schema dan tetap dipertahankan.
- File: `backend/internal/coa/helpers.go`
- Catatan verifikasi enum: `SELECT DISTINCT account_type FROM accounts` pada DB
  hasil migrasi penuh + seed registrasi (31 nilai) adalah subset dari daftar 38.

### 5. N5/NEW-1 — Jurnal manual referensi akun lintas-tenant
Lookup akun dalam transaksi posting kena RLS (0 baris) → ErrNoRows → 500
POST_FAILED. Handler CreateManualJournal kini lewat `classifyPostingError()`:
ErrNoRows → **404 ACCOUNT_NOT_FOUND** ("account does not exist for this
tenant"), konsisten dengan route cash. Isolasi tenant tidak berubah (tetap
rollback bersih).
- File: `backend/internal/accounting/journal_manual.go`

### 6. N6 — Transfer ke/dari akun non-CASH/BANK
Engine menolak dengan `accounting.ErrAccountTypeMismatch` tapi API membalas 500.
`cash/errorFor()` kini memetakan sentinel engine itu ke **422
ACCOUNT_TYPE_MISMATCH** ("transfers require CASH or BANK accounts on both
sides"); sekalian `ErrSameTransferAccount` → **400 SAME_TRANSFER_ACCOUNT**
(keluarga validasi transfer yang sama).
- File: `backend/internal/cash/helpers.go`

## Unit Test Baru (tabel-driven)

- `backend/internal/cash/error_classification_test.go` — `TestErrorForClassification`
  (8 kasus: sentinel QA-07/QA-08/N6, urutan sentinel-vs-ErrNoRows, fallback lama
  tidak berubah) + pin teks sentinel.
- `backend/internal/accounting/journal_manual_test.go` — `TestClassifyPostingError`
  (6 kasus: approval 409, periode 422, lintas-tenant 404, validasi engine 400,
  unknown 500) + pin teks sentinel.
- `backend/internal/period/idempotency_key_test.go` — `TestIdempotencyKeyValidation`
  (5 kasus: hilang/spasi/bukan-UUID ditolak, UUID valid & padded diterima).
- `backend/internal/coa/helpers_test.go` — `TestValidateAccountTypeEnum`
  (tipe seed+migrasi diterima, NOT_A_TYPE/lowercase ditolak dengan pesan
  berdaftar, enum sorted-distinct >= 30 nilai).

## Gate

```
cd backend && gofmt -l .        # bersih
go vet ./...                    # pass
go build ./...                  # pass
go test ./...                   # PASS semua paket (37)
```

## Verifikasi E2E (DB `finance_qa_fx4`, role terbatas `qa_fx4_app` NOSUPERUSER NOBYPASSRLS, API :18094)

Setup: 56 migrasi up.sql dijalankan per-file sebagai superuser (0 gagal —
termasuk 000030), GRANT DML penuh ke `qa_fx4_app`, server via DATABASE_URL role
terbatas tersebut. Tenant uji: **W-F4** (tenant_id=1000) dan **W-OTHER** (1100).

| Kasus | Respons |
|---|---|
| QA-07 reverse pertama jurnal POSTED #1 | 201 REVERSAL (id 2), original VOID |
| QA-07 double-reverse jurnal VOID #1 | **409** `JOURNAL_NOT_POSTED` ✅ |
| QA-08 cash-in `entry_date=2025-01-01` | **422** `ENTRY_DATE_OUTSIDE_OPEN_PERIOD` ✅ |
| QA-09 close TANPA key | **400** `INVALID_REQUEST` "Idempotency-Key header is required" — tanpa "SQLSTATE"/uuid ✅ |
| QA-09 close key bukan UUID | 400 `INVALID_REQUEST` "Idempotency-Key must be a UUID" ✅ |
| QA-09 close DENGAN key valid | 200 CLOSED + jurnal PERIOD_CLOSE ✅ |
| QA-08b cash-in pasca-close (periode CLOSED) | **422** `ENTRY_DATE_OUTSIDE_OPEN_PERIOD` ✅ |
| Unlock TANPA key | 400 bersih; DENGAN key → 200 OPEN; posting pasca-unlock → 201 ✅ |
| QA-10 `account_type:"NOT_A_TYPE"` | **400** + daftar 38 nilai valid ✅ |
| QA-10 kontrol tipe valid `OTHER_CURRENT_ASSET` | 201 ✅ |
| N5 jurnal manual tenant B pakai akun tenant A | **404** `ACCOUNT_NOT_FOUND` ✅ (kontrol normal → 201) |
| N6 transfer CASH→REVENUE | **422** `ACCOUNT_TYPE_MISMATCH` ✅ |

Log server: nol respons 500 / nol entri "internal error returned to client"
selama seluruh rangkaian.

Invarian akhir (psql, GUC app.tenant_id=1000):

```
balance_drift      = 0
journals           = 5 (posted=4, void=1)
chain_broken_links = 0
head_match         = true
trial balance W-F4   balanced=true (405.000 = 405.000)
trial balance W-OTHER balanced=true
```

## File Berubah

- `backend/internal/accounting/engine.go` — sentinel `ErrEntryDateOutsideOpenPeriod`
- `backend/internal/accounting/helpers.go` — resolvePeriod wrap sentinel
- `backend/internal/accounting/journal_manual.go` — `classifyPostingError()` (N5, QA-08)
- `backend/internal/cash/handler.go` — guard reversal wrap sentinel (QA-07)
- `backend/internal/cash/helpers.go` — `errorFor()` klasifikasi QA-07/QA-08/N6
- `backend/internal/cash/journal.go` — resolvePeriod wrap sentinel (QA-08)
- `backend/internal/period/handler.go` — Close validasi Idempotency-Key (QA-09)
- `backend/internal/coa/helpers.go` — enum `validAccountTypes` + validasi (QA-10)
- Test baru: `cash/error_classification_test.go`,
  `accounting/journal_manual_test.go`, `period/idempotency_key_test.go`,
  tambahan kasus di `coa/helpers_test.go`
- Laporan ini: `docs/FIX_F4_POSTING_ERRORS.md`

Tidak ada perubahan pada sales/purchase/costing/auth/middleware/dashboard/reports,
TASK_LEDGER, atau perilaku posting/invarian engine. Tidak ada commit.
