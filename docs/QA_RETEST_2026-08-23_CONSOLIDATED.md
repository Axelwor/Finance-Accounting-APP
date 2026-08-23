# Konsolidasi Retest Multi-Agent — 2026-08-23 (pasca Phase C1/C3)

**Task Ledger:** QA-RETEST-002 · **Target:** HEAD `78053fe` · **Metode:** 4 sesi Agent Manager worktree paralel, masing-masing DB + port sendiri, seluruhnya menguji sebagai role terbatas `NOSUPERUSER NOBYPASSRLS` (meniru cutover C3).

Laporan per sesi: [W1 Auth](QA_RETEST_W1_AUTH.md) · [W2 Core Accounting](QA_RETEST_W2_CORE.md) · [W3 Sales Chain](QA_RETEST_W3_SALES.md) · [W4 Purchase & Misc](QA_RETEST_W4_PURCHASE_MISC.md)

## Skor Akhir

| Kategori | Jumlah | ID |
|---|---|---|
| **FIXED sejak QA-FULL-001** | 2 | QA-01 (CRITICAL — register di bawah RLS) ✅, QA-24 (template global via 000057) ✅ |
| **STILL BROKEN / PARTIAL** | 15 | QA-02, 03, 04, 05, 06, 07, 08, 09, 10, 11, 12, 13, 14🟡, 15, 16🟡, 18, 19, 20 |
| **BUG BARU ditemukan retest** | 8 | lihat bawah |

## Yang Terbukti FIXED

1. **QA-01 (CRITICAL)** — Register + seed 48 COA kini **lulus penuh di bawah role NOSUPERUSER NOBYPASSRLS**: 201, tenant_id>0, periode OPEN, nol error RLS di log Postgres. Fix commit acd4766 (GUC `app.tenant_id` dalam transaksi seed) terverifikasi.
2. **QA-24** — Migration 000057: 19 template global `tenant_id=0` **terlihat oleh tenant biasa**, tulisan tetap own-only (per-command policies). Data global utuh.

## Yang Masih Rusak (poin terpenting)

- **QA-03 DO "conn busy"** — deterministik, akar sama (`consumeFIFO` Exec dalam loop rows). Seluruh pengiriman barang berstok mati.
- **QA-06 multi-payment** — akar dipastikan dua sisi: sales `PMT-{invoiceID}` statis & purchase `fmt.Sprintf("PAY-%d", invoiceID)` (`supplier_payments.go:159`) menabrak `journal_entries_intent_unique`. Pelunasan bertahap mustahil.
- **QA-04 supplier NULL-scan** — 9 kolom opsional wajib-faktual; supplier minimal mustahil dibuat.
- **QA-05 SQ tanpa payment_term_id → 500** — NULL-scan identik.
- Kelompok error-class/validasi lama bertahan semua: QA-02 (limiter 5/menit shared, kini ada `Retry-After`), QA-07/08/09/10/11/15/19/20, QA-18 (logout tanpa token → 200), QA-12/13 (500 schema drift), QA-14🟡 (workaround `"config":{}`), QA-16🟡 (rate eksplisit jalan).

## Bug Baru (temuan retest)

| # | Sev | Bug |
|---|---|---|
| N1 | HIGH | **Credit note item goods SELALU 500**: `idem + "-cogs"` menghasilkan string bukan UUID dimasukkan ke kolom uuid (`sales/credit_notes.go:396`) → CN rusak total |
| N2 | HIGH | **Purchase return SELALU 500**: `movement_type='PURCHASE_RETURN'` tidak ada di CHECK `inventory_movements_movement_type_check` (`purchase/purchase_returns.go:326`) → return mustahil, stok/jurnal tak berubah |
| N3 | MED | Lease depreciate → 500 kolom `rou_cost_cents` tidak ada (schema drift) |
| N4 | MED | Dimensions create → 500 `created_at` timestamptz discan ke `*string` (`budget/dimensions.go`) |
| N5 | MED-LOW | Jurnal manual lintas-tenant → 500 `POST_FAILED` (ErrNoRows→500; isolasi tetap aman) |
| N6 | LOW | Transfer ke akun non-CASH (penolakan bisnis) → 500 alih-alih 4xx |
| N7 | LOW | PUT/DELETE template global tenant lain → 200 false-success (RowsAffected tak dicek; data dilindungi RLS) |
| N8 | OBS | Overpay sebagai pembayaran PERTAMA kini 201 + `overpayment_cents` (dulu 409) — konfirmasi produk: desain baru atau regresi? |

## Kesehatan Umum

- **Nol regresi RLS** pada seluruh alur inti yang diuji oleh 4 sesi di bawah role terbatas.
- Invarian akuntansi utuh di semua sesi: balance drift 0, hash-chain tersambung + head cocok, rollback bersih pada setiap skenario gagal, isolasi tenant 404 di semua jalur, idempotency race tetap 1 jurnal.
- Catatan deploy (persisten): rantai migrasi tetap tidak replay-clean oleh role terbatas (000030 FK tenant_id=0 sebelum 000038; seed 000033/000049/000050/000052 tertahan FORCE RLS tanpa GUC) — migrasi memang desain superuser-only pasca-C3, tapi pastikan `deploy.sh` menjalankan fase migrasi sebagai superuser.

## Prioritas Perbaikan Berikutnya

1. N1 credit-note UUID & N2 purchase-return enum (fitur mati total)
2. QA-03 conn-busy FIFO + QA-06 source_ref payment (inti siklus usaha)
3. QA-04/05 NULL-scan family
4. Kelompok error-classification (QA-07/08/09/11 + N5/N6/N7) via httperr.Classify menyeluruh
