# QA RETEST W2 — Core Accounting + Regresi RLS

Tanggal: 2026-08-23 · Lingkup: Cash/Journal/Period/Reports (bug lama `docs/QA_REPORT_2026-08-23.md`) + regresi Phase C RLS.
Lingkungan uji: DB fresh `finance_qa_rw2`, API dijalankan sebagai role **terbatas** `qa_w2_app` (LOGIN, NOSUPERUSER, NOBYPASSRLS) — identik pola prod pasca-cutover. Tenant uji: W2 Corp (tenant_id 1000). Tidak ada perubahan kode produksi dalam task ini.

## Ringkasan Verdict

| ID | Kasus | Verdict | Bukti singkat |
|---|---|---|---|
| QA-07 | Double-reverse jurnal VOID | **STILL-BROKEN** | Reverse ke-2 jurnal VOID → **500** `JOURNAL_POST_FAILED`; log server: raw_message `journal 5 is not posted`. Guard bisnisnya ada (`internal/cash/handler.go`) tapi dipetakan ke 500, bukan 409/4xx bisnis. Data aman (rollback bersih). |
| QA-08 | Tanggal di luar periode terbuka | **STILL-BROKEN** | cash-in `entry_date=2025-01-01` → **404** `ACCOUNT_NOT_FOUND` "account does not exist for this tenant" — pesan salah arah, belum ada kode semacam 422 ENTRY_DATE_OUTSIDE_OPEN_PERIOD. Efek samping: penolakan posting pasca-close juga tampil sebagai 404 yang sama. |
| QA-09 | `/periods/close` tanpa Idempotency-Key | **STILL-BROKEN** | → **400** `CLOSE_FAILED` dengan body: `ERROR: invalid input syntax for type uuid: "" (SQLSTATE 22P02)` — raw SQL leak persis temuan lama; helper `idempotencyKey()` di package period tetap tidak dipanggil handler Close. |
| QA-10 | `account_type` tidak divalidasi | **STILL-BROKEN** | POST /accounts `{account_type:"NOT_A_TYPE"}` → **201** tersimpan (id 61). DB: satu-satunya CHECK constraint hanya menjaga `report_group`, bukan `account_type`. |
| QA-20 | Query tanggal invalid diabaikan diam | **STILL-BROKEN** | `?date_from=invalid` pada trial-balance/balance-sheet/profit-loss/cash-flow → semua **200** tanpa filter & tanpa error. |

## Regresi RLS

**Tidak ditemukan regresi baru pada endpoint inti.** Seluruh operan di bawah ini dieksekusi murni sebagai role terbatas `qa_w2_app` dan berperilaku setara (atau lebih baik dari) laporan lama:

- Register tenant → 201 (48 akun seed + periode OPEN terbuat) — di laporan lama ini gagal di bawah role terbatas (QA-01); kini lolos berkat GUC `app.tenant_id` pada seed.
- GET /accounts (48), create 201, duplikat 409 `ACCOUNT_CODE_EXISTS`, export CSV 200 text/csv.
- Semua posting jurnal (cash-in/out/transfer/opening-balances) 201 status POSTED dengan hash-chain tersambung.
- Idempotensi, reverse, close/unlock periode, reports & export — semua 200/201 sesuai harapan tanpa 500/403 terkait permission/RLS.

## Hasil Uji Detail

### COA
| Kasus | HTTP | Catatan |
|---|---|---|
| List seed | 200 | tepat 48 akun |
| Create valid | 201 | id 59, kode 9999 |
| Duplikat kode | 409 | `ACCOUNT_CODE_EXISTS` |
| QA-10 invalid type | 201 ❌ | tersimpan; CHECK constraint hanya untuk report_group |
| Export CSV | 200 | content-type text/csv |

### Cash & Posting
cash-in (id 1), cash-out (id 2), opening-balances (id 3), transfer CASH→CASH (id 4): semua **201 POSTED**, prev_hash menyambung ke hash sebelumnya, intent_type benar (CASH_IN/CASH_OUT/OPENING_BALANCE/TRANSFER).
Catatan: transfer ke akun non-CASH ditolak engine sebagai `ACCOUNT_TYPE_MISMATCH` tapi API mengembalikan **500 JOURNAL_POST_FAILED** (temuan baru, LOW — kelas error salah, bukan regresi).

### Idempotensi
| Kasus | Hasil |
|---|---|
| Replay key sama payload sama | 201, **ID jurnal sama** (id 5 dua kali) ✅ |
| Key sama payload beda | 409 `IDEMPOTENCY_KEY_REUSE` ✅ |
| Race 5 curl paralel 1 key | HTTP 4×201 (semua body identik id 6) + 1×409 `DUPLICATE_INTENT`; SQL: `count(jurnal QA-RACE)=1` — **tepat 1 jurnal** ✅ |

Catatan: satu respons 409 DUPLICATE_INTENT (guard source_ref+intent_type) muncul bersama replay idempoten — dedup tetap benar, namun respons antar-request tidak seragam untuk payload identik.

### Validasi Input
amount 0 → 400 · amount −1 → 400 · entry_date `2026-13-45` → 400 ("must be a valid date in YYYY-MM-DD") · source_ref kosong → 400 · Idempotency-Key `not-a-uuid` → 400. Pesan bersih tanpa SQLSTATE. ✅

### Reverse & QA-07
- Reverse pertama jurnal POSTED (id 5) → **201** REVERSAL (id 10); SQL: original `status=VOID reason=reversed`, reversal `reversal_of_id=5`. ✅ *(pada DB yang migrasinya lengkap — lihat catatan setup)*
- Double-reverse jurnal VOID → **500** `JOURNAL_POST_FAILED`, log: `journal 5 is not posted`. ❌ QA-07 STILL-BROKEN (harusnya 409 bisnis). Tidak ada SQLSTATE leak pada kasus ini.

### Period Lifecycle & QA-09
| Kasus | Hasil |
|---|---|
| Close DENGAN key | 200 CLOSED + jurnal PERIOD_CLOSE (JRN-2026-000008) ✅ |
| Double-close | 400 `CLOSE_FAILED` "no open period" ✅ |
| Posting pasca-close | ditolak (404 — salah kode, lihat QA-08) ✅/❌ |
| Unlock | 200 OPEN + jurnal PERIOD_REOPEN ✅ |
| Posting pasca-unlock | 201 POSTED ✅ |
| Close TANPA key (QA-09) | 400 raw SQL leak `(SQLSTATE 22P02)` ❌ STILL-BROKEN |

### Reports & QA-20
trial-balance `balanced=true` (total debit = total kredit) · balance-sheet `balanced=true` · profit-loss 200 · cash-flow 200.
Export: `/reports/trial-balance/export?format=pdf` → 200 application/pdf (PDF v1.3 valid); `/reports/balance-sheet/export?format=xlsx` → 200 xlsx valid. ✅
QA-20: `?date_from=invalid` → 200 diam di 4 endpoint. ❌ STILL-BROKEN.

## Invarian Akhir (psql, GUC app.tenant_id=1000)

```
balance_drift      = 0            -- sum(debit)-sum(credit) seluruh journal_lines
journals           = 10           -- posted=9 void=1
chain_broken_links = 0            -- tiap prev_hash == hash entri sebelumnya
chain_head_match   = true         -- ledger_chain_heads.last_hash == hash jurnal head; head_is_max=true
audit_logs         = 10 baris     -- action: POST, VOID, CLOSE, UNLOCK (close/unlock tercatat)
```

## Catatan Setup (bukan bug produk)

Migrasi di DB QA dijalankan per-file: 000030 gagal seed `report_templates tenant_id=0` (FK violation — issue lama QA-21/koreksi memory) sehingga DDL tabel dashboard diterapkan tanpa baris seed tsb; 000049+ dijalankan sebagai superuser sesuai desain (FORCE RLS membuat owner pun tunduk policy). Penting: file **000004_allow_reversal_link** wajib terpasang — tanpanya seluruh operasi reverse gagal 500 oleh trigger immutability versi 000001 (reproduksi awal saya sempat 500 karena file ini terlewat dari resume loop, lulus setelah diterapkan). Duplikat nomor 000052 masih ada di tree migrasi (QA-21 masih relevan).

## Kesimpulan

Mesin posting, idempotensi (termasuk race), hash-chain, siklus periode, dan reports sehat dan **lolos penuh di bawah role DB terbatas** — tidak ada regresi RLS baru. Kelima bug lama lingkup core (QA-07/08/09/10/20) **masih belum diperbaiki**: polanya seragam yaitu kelas error/pesan yang salah (500 atau 404/raw-SQL untuk kondisi bisnis) plus validasi input yang bolong (account_type, query tanggal). Satu temuan baru kecil: penolakan bisnis transfer non-CASH ikut terpetakan ke 500.
