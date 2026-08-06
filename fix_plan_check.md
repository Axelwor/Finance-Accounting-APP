# Rekap Saran Perbaikan — Accounting Engine v1.9, Data Model v1.9, Architecture v3.4

**Tanggal:** 2026-08-06  
**Sumber:** Hasil analisis lintas-dokumen (ACCOUNTING_ENGINE.md, DATA_MODEL.md, ARCHITECTURE.md)  
**Status:** Review — belum diimplementasikan  

---

## Daftar Isi

1. [Ringkasan](#ringkasan)
2. [🔴 Critical — 10 Isu Harus Di-resolve Sebelum Koding](#-critical--10-isu-harus-di-resolve-sebelum-koding)
3. [🟠 High — 14 Isu Akan Menyulitkan Jika Ditunda](#-high--14-isu-akan-menyulitkan-jika-ditunda)
4. [🟡 Medium — 12 Isu Bisa Di-improve Setelah P1](#-medium--12-isu-bisa-di-improve-setelah-p1)
5. [📋 Per Target Dokumen](#-per-target-dokumen)
6. [📊 Test Case Tambahan](#-test-case-tambahan)
7. [🗺️ Urutan Pengerjaan yang Direkomendasikan](#️-urutan-pengerjaan-yang-direkomendasikan)

---

## Ringkasan

| Kategori | Jumlah |
|---|---|
| 🔴 Critical | 10 |
| 🟠 High | 14 |
| 🟡 Medium | 12 |
| **Total** | **36** |

---

## 🔴 Critical — 10 Isu Harus Di-resolve Sebelum Koding

### C1 — HPP di DO vs INV: Ambiguitas Double HPP

| Dokumen Terkena | Engine, Data Model, Architecture |
|---|---|
| **Engine §7** | "DO → Jurnal HPP: Debet 5101 / Kredit 1301" DAN "INV → Debet 5101 / Kredit 1301 (jika DO belum membukukan HPP)" — tidak ada flag koordinasi |
| **Data Model** | `delivery_lines.unit_cost_cents` dan `invoice_lines.cogs_cents` — dua-duanya exist, bisa double |
| **Architecture §6.7** | "Delivery → HPP sesuai kebijakan pengakuan" — terlalu ambigu |

**Dampak:** Double HPP, laba kotor salah, stok salah.

**Rekomendasi:**
```sql
-- 1. Tambah flag di deliveries
ALTER TABLE deliveries ADD COLUMN hpp_booked BOOLEAN NOT NULL DEFAULT false;

-- 2. Tambah kebijakan di tenant_settings
-- setting_key: 'hpp_recognition'
-- setting_value: 'delivery' | 'invoice'
-- Default: 'delivery'

-- 3. Validasi di engine saat posting INV:
--    IF tenant_config.hpp_recognition = 'delivery' AND delivery.hpp_booked = true
--      → INV tidak buat jurnal HPP
--    IF tenant_config.hpp_recognition = 'invoice'
--      → DO tidak buat jurnal HPP
```

---

### C2 — Invoice Tidak Punya `sales_order_id` (Traceability Putus)

| Dokumen Terkena | Engine, Data Model |
|---|---|
| **Engine §7** | Alur penuh: SQ → SO → DP → DO → INV → Pelunasan |
| **Data Model** | `invoices.delivery_id` (bisa NULL), tapi **tidak ada** `sales_order_id` |
| **Data Model** | `sales_orders_lines` tidak punya `invoiced_qty` |

**Dampak:**
- Invoice langsung (tanpa DO, mis. jasa) tidak terhubung ke SO
- Tidak bisa validasi `INV_EXCEEDS_DO` di level SO
- Tidak bisa tracking berapa qty SO yang sudah di-invoice

**Rekomendasi:**
```sql
-- 1. Tambah FK ke SO di invoices
ALTER TABLE invoices ADD COLUMN sales_order_id BIGINT
    REFERENCES sales_orders(id);

-- 2. Tambah tracking qty di SO lines
ALTER TABLE sales_orders_lines ADD COLUMN invoiced_qty NUMERIC(18,3)
    NOT NULL DEFAULT 0;

-- 3. Validasi: invoiced_qty ≤ (qty - delivered_qty) untuk goods per SO line
```

---

### C3 — DP Alokasi Hanya Satu Invoice (Tidak Support Split)

| Dokumen Terkena | Engine, Data Model |
|---|---|
| **Engine §7.1** | "Beberapa DP pada satu SO... saat INV, seluruh saldo 2201 terkait direalisasi" — implikasinya bisa ke beberapa invoice |
| **Data Model** | `down_payments.applied_to BIGINT` — hanya menunjuk satu invoice |

**Dampak:** Satu SO dengan multi-invoice tidak bisa alokasi DP secara proporsional.

**Rekomendasi:**
```sql
-- Buat tabel alokasi DP (seperti payment_allocations)
CREATE TABLE down_payment_allocations (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id),
    down_payment_id BIGINT NOT NULL REFERENCES down_payments(id),
    invoice_id BIGINT NOT NULL REFERENCES invoices(id),
    allocated_cents BIGINT NOT NULL CHECK (allocated_cents > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE (down_payment_id, invoice_id),
    -- validasi: total allocated ≤ dp.amount_cents (di trigger/engine)
);

-- Deprecate: down_payments.applied_to (keep for single-invoice backward compat, NULL if multi)
```

---

### C4 — ECL Policies Bucket Terlalu Kaku

| Dokumen Terkena | Engine, Data Model |
|---|---|
| **Engine §15** | "Persentase berdasarkan aging piutang — **dapat dikonfigurasi**" |
| **Data Model §3.11** | `bucket TEXT CHECK (0_30/31_60/61_90/over_90)` — hanya 4 bucket fixed |

**Dampak:** Bisnis dengan 5+ bucket aging (0-30, 31-60, 61-90, 91-120, 121-180, >180) tidak terakomodasi. Perbankan/multi-finance butuh lebih granular.

**Rekomendasi:**
```sql
-- Drop tabel lama, ganti dengan range-based
DROP TABLE IF EXISTS ecl_policies;

CREATE TABLE ecl_policies (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id),
    bucket_min_days INT NOT NULL CHECK (bucket_min_days >= 0),    -- 0, 31, 61, 91, 121, 181
    bucket_max_days INT CHECK (bucket_max_days > bucket_min_days), -- 30, 60, 90, 120, 180, NULL
    loss_rate NUMERIC(9,6) NOT NULL CHECK (loss_rate >= 0 AND loss_rate <= 1),
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE (tenant_id, bucket_min_days, bucket_max_days),
    -- Exclusion: no overlapping buckets per tenant
    EXCLUDE USING gist (
        tenant_id WITH =,
        int4range(bucket_min_days, COALESCE(bucket_max_days, 99999), '[)') WITH &&
    )
);
-- bucket_max_days NULL = ∞ (unlimited)
```

---

### C5 — `consolidation_entities` FK Self-Reference Salah

| Dokumen Terkena | Engine, Data Model |
|---|---|
| **Engine §22** | Transaksi antar-entitas: dari entitas A ke entitas B |
| **Data Model §3.18** | `intercompany_transactions.from_entity_id` dan `to_entity_id` → `consolidation_entities(id)` |

**Dampak:** Tidak bisa merepresentasikan transaksi antar dua subsidiary tanpa melibatkan parent. `consolidation_entities` mencampur parent & subsidiary.

**Rekomendasi:**
```sql
-- Ganti FK ke tenants langsung (lebih tepat untuk multi-tenant)
ALTER TABLE intercompany_transactions
    DROP COLUMN IF EXISTS from_entity_id,
    DROP COLUMN IF EXISTS to_entity_id,
    ADD COLUMN from_tenant_id BIGINT NOT NULL REFERENCES tenants(id),
    ADD COLUMN to_tenant_id BIGINT NOT NULL REFERENCES tenants(id),
    ADD COLUMN debit_account_id BIGINT NOT NULL REFERENCES accounts(id),   -- akun di from_tenant
    ADD COLUMN credit_account_id BIGINT NOT NULL REFERENCES accounts(id);  -- akun di to_tenant

-- Dua akun: piutang (dari) dan hutang (ke) — bukan satu intercompany_account_id
```

---

### C6 — `accounts` Tidak Punya Flag `requires_dimension`

| Dokumen Terkena | Engine, Data Model |
|---|---|
| **Engine §26.2** | "Posting wajib melengkapi dimensi untuk akun yang ditandai wajib dimensi" → error `DIMENSION_REQUIRED` |
| **Data Model §3.2** | `accounts` tidak punya kolom `requires_dimension` |

**Dampak:** Tidak bisa enforce dimensi wajib di level database atau engine.

**Rekomendasi:**
```sql
ALTER TABLE accounts
    ADD COLUMN requires_dimension BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN required_dimension_type TEXT CHECK (required_dimension_type IN ('branch','project','department','custom'));
-- NULL required_dimension_type = any dimension accepted
-- Contoh: akun "Beban Gaji Dept A" wajib isi dimensi 'department'
```

---

### C7 — `intercompany_account_id` Satu Arah (Harusnya Dua)

| Dokumen Terkena | Engine, Data Model |
|---|---|
| **Engine §22** | Piutang dan hutang antar-entitas: dua arah, dua akun berbeda |
| **Data Model §3.18** | `intercompany_transactions.intercompany_account_id` — hanya satu akun |

**Rekomendasi:** Sudah tercakup di **C5** di atas — ganti dengan `debit_account_id` + `credit_account_id`.

---

### C8 — Hash Chain: Algoritma & Collision Risk Tidak Spesifik

| Dokumen Terkena | Engine, Architecture |
|---|---|
| **Engine §4.1** | "hash (anti-tamper), hash_sebelumnya" — tidak spesifik algoritma |
| **Architecture §5.4** | "SHA256(canonical_version + tenant_id + journal_number + entry_date + source_ref + intent_type + sorted_lines + prev_hash)" — baru muncul di Architecture, tidak di Engine |

**Dampak:** Engine spec tidak menyebutkan detail hash; Architecture menyebutkan tapi dua dokumen tidak sinkron.

**Rekomendasi:**
```
1. Pindahkan spesifikasi hash dari Architecture §5.4 ke Engine §4.1 (Engine = source of truth)
2. Tambahkan di Engine §4.1:
   - Algoritma: SHA-256
   - Input canonical: (tenant_id || journal_number || entry_date || source_ref || intent_type || sorted_lines_json || prev_hash)
   - sorted_lines diurutkan berdasarkan (account_id, debit_cents DESC, credit_cents DESC) — deterministik
   - Encoding: UTF-8
   - Jurnal pertama (genesis): prev_hash = "GENESIS" + tenant_id
3. Tambahkan di Architecture §5.4:
   - Verifikasi rutin via cron job
   - Alert jika hash mismatch terdeteksi
```

---

### C9 — `created_at` Race Condition untuk Urutan Posting

| Dokumen Terkena | Engine |
|---|---|
| **Engine §2.2** | "`created_at` hanya menentukan urutan posting/hash chain" |

**Dampak:** Dua transaksi simultan bisa punya `created_at` identik (same microsecond), hash chain ordering tidak deterministik.

**Rekomendasi:**
```
Urutan kanonikal untuk hash chain:
  1. entry_date (tanggal transaksi)
  2. journal_number (monotonik, dari sequence DB) — ini deterministik
  3. created_at (sebagai tiebreaker)
  
Karena journal_number sudah unique + monotonik dari DB sequence,
pakai journal_number untuk ordering hash chain.
Hapus ketergantungan pada created_at untuk ordering.

Atau jika tetap pakai created_at: gunakan hybrid (entry_date, journal_id, created_at)
```

---

### C10 — Refresh Token Disimpan di localStorage (Tidak Aman)

| Dokumen Terkena | Architecture |
|---|---|
| **Architecture §11.2** | "Zustand (persist localStorage untuk refresh token aman? → lebih aman httpOnly cookie untuk refresh)" — masih ambigu |

**Dampak:** XSS bisa mencuri refresh token → full account takeover.

**Rekomendasi:**
```
1. Access token (15 menit): boleh di memory (Zustand/closure), jangan localStorage
2. Refresh token (30 hari): WAJIB httpOnly, Secure, SameSite=Strict cookie
3. Hapus mention localStorage untuk refresh token di Architecture §11.2
4. Backend: POST /auth/refresh membaca refresh token dari cookie, bukan body
5. Frontend: tidak perlu tahu refresh token — cookie dikirim otomatis
```

---

## 🟠 High — 14 Isu Akan Menyulitkan Jika Ditunda

### H1 — Akun 42xx Tidak Ada di COA Standar Engine

| Dokumen | Engine §8.1 pakai `4201 Retur Penjualan` dan `4202 Diskon Penjualan` |
|---|---|
| **Masalah** | Kedua akun tidak muncul di tabel kerangka akun standar §3.0.2 |

**Rekomendasi:** Tambahkan ke tabel §3.0.2 Engine:
```
| Kontra-Pendapatan | 4xxx | 4201 Retur Penjualan (CONTRA_REVENUE), 4202 Diskon Penjualan (CONTRA_REVENUE) |
```

---

### H2 — Akun 2402 Kelebihan Pembayaran Pelanggan — Tipe Tidak Eksplisit

| Dokumen | Engine §3.0.2 menempatkan 2402 di CUSTOMER_DEPOSIT |
|---|---|
| **Masalah** | Tabel §3.0.1 hanya menyebut CUSTOMER_DEPOSIT untuk 2201 Uang Muka Penjualan |

**Rekomendasi:** Di Engine §3.0.1, tambahkan baris eksplisit: `2402 Kelebihan Pembayaran Pelanggan → CUSTOMER_DEPOSIT (Liabilitas)`.

---

### H3 — Batch FIFO: Retur Penjualan Masuk Batch Baru — Perilaku Tidak Spesifik

| Dokumen | Engine §9.4: "FIFO: masuk sebagai batch baru (cost = cost asal barang yang diretur)" |
|---|---|
| **Masalah** | Tidak jelas: apakah batch retur dijual sebelum batch normal lain? Atau FIFO murni? |

**Rekomendasi:**
```sql
-- Tambah flag di inventory_batches
ALTER TABLE inventory_batches ADD COLUMN is_return_batch BOOLEAN NOT NULL DEFAULT false;

-- Kebijakan konsumsi FIFO dengan retur:
-- tenant_settings: 'fifo_return_priority' → 'fifo_strict' (default) | 'return_first'
-- fifo_strict: retur masuk ke antrian sesuai tanggal terima
-- return_first: batch retur dijual duluan (mencegah fluktuasi cost)
```

---

### H4 — Selisih Overhead (Job Costing) — Treatment Default Tidak Jelas

| Dokumen | Engine §11.4: "selisih dipindahkan ke pendapatan lain-lain (kebijakan konfigurabel)" |
|---|---|
| **Masalah** | PSAK 14 umumnya: under-applied → COGS, over-applied → pengurang COGS. Engine spec tidak punya default. |

**Rekomendasi:**
```
Tambahkan di Engine §11.4:
- Default behavior: under-applied overhead → Dr 5101 HPP; over-applied → Cr 5101 HPP
- tenant_settings: 'overhead_variance_treatment' → 'cogs' (default) | 'other_income'
- Jurnal otomatis saat tutup buku: Dr/Cr 5101 / Cr/Dr 4902
```

---

### H5 — PPh Final UMKM: Tarif Disebut Tapi Dikaburkan (Lawyer-Speak)

| Dokumen | Engine §13.2: "`0,5%` dan `Rp 4,8 M` hanya digunakan bila aturan yang berlaku memang memenuhi syarat" |
|---|---|
| **Masalah** | Engine spec tidak menetapkan mekanisme penyimpanan tarif historis |

**Rekomendasi:**
```sql
-- tax_rates sudah ada, pastikan efektif:
-- tax_type = 'PPh_FINAL'
-- rate = 0.005 (0,5%)
-- eligibility_rules = {"max_revenue": 480000000000, "effective_from": "2022-01-01"}
-- effective_to = NULL (sampai diubah)
-- Engine query: SELECT rate FROM tax_rates WHERE tax_type='PPh_FINAL' 
--   AND effective_from <= transaction_date AND (effective_to IS NULL OR effective_to >= transaction_date)
```

---

### H6 — Akrual Auto-Reversal: Tidak Ada Unique Constraint

| Dokumen | Engine §16.2: "dibalik otomatis di awal periode berikutnya" |
|---|---|
| **Masalah** | Tidak ada mekanisme mencegah double reversal. Data Model tidak punya constraint. |

**Rekomendasi:**
```sql
-- Di accruals table:
ALTER TABLE accruals ADD COLUMN reversed_journal_id BIGINT REFERENCES journal_entries(id);
CREATE UNIQUE INDEX accruals_one_reversal ON accruals (reversed_journal_id)
    WHERE reversed_journal_id IS NOT NULL;
-- Saat auto-reversal: set reversed_journal_id = journal_reversal.id
-- Validasi: tidak bisa membuat reversal kedua
```

---

### H7 — Konsolidasi Multi-Currency: Current Method Tidak Dijelaskan

| Dokumen | Engine §22: "Konversi kurs (current method)" |
|---|---|
| **Masalah** | Tidak dijelaskan akun mana pakai kurs mana (closing rate vs average rate vs historical) |

**Rekomendasi:** Tambahkan tabel di Engine §22:
```
| Akun | Kurs yang Digunakan |
|---|---|
| Aset & Liabilitas (neraca) | Closing rate (tanggal laporan) |
| Ekuitas (modal, laba ditahan) | Historical rate (tanggal transaksi) |
| Pendapatan & Beban (laba rugi) | Average rate (rata-rata periode) |
| Selisih konversi | → OCI (PSAK 10/65) |
```

---

### H8 — Suspense 3105: Tidak Ada Tipe Akun & Tracking

| Dokumen | Engine §5, §21.5 |
|---|---|
| **Masalah** | Tidak ada tipe `SUSPENSE` di `accounts.account_type`, tidak ada tabel tracking suspense items |

**Rekomendasi:**
```sql
-- 1. Tambah enum di account_type
--    Tambahkan 'SUSPENSE' ke CHECK constraint accounts.account_type

-- 2. Tabel tracking suspense
CREATE TABLE suspense_items (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id),
    account_id BIGINT NOT NULL REFERENCES accounts(id),  -- akun suspense
    journal_id BIGINT NOT NULL REFERENCES journal_entries(id),
    amount_cents BIGINT NOT NULL,
    description TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'OPEN' CHECK (status IN ('OPEN','RESOLVED')),
    resolution_account_id BIGINT REFERENCES accounts(id),
    resolution_journal_id BIGINT REFERENCES journal_entries(id),
    resolved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- Validasi tutup buku: COUNT(*) WHERE status='OPEN' = 0
```

---

### H9 — OCI (Other Comprehensive Income): Tidak Ada Tipe Akun

| Dokumen | Engine §12.2: `3401 Surplus Revaluasi (OCI/Ekuitas)` |
|---|---|
| **Masalah** | Tipe `EQUITY` tidak membedakan OCI dari ekuitas biasa |

**Rekomendasi:**
```sql
-- Tambah flag, bukan tipe baru (OCI tetap bagian ekuitas di neraca)
ALTER TABLE accounts ADD COLUMN is_oci BOOLEAN NOT NULL DEFAULT false;
-- 3401 Surplus Revaluasi → report_group='equity', account_type='EQUITY', is_oci=true
-- Saat aset dilepas: surplus → 3201 Laba Ditahan (realisasi OCI)
```

---

### H10 — Tabel `credit_notes.refund_method = credit_balance` Tanpa Tabel Credit Balance

| Dokumen | Engine §7.1, Data Model |
|---|---|
| **Masalah** | Credit note bisa buat saldo kredit, tapi tidak ada tracking terpisah |

**Rekomendasi:**
```sql
CREATE TABLE customer_credit_balances (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id),
    customer_id BIGINT NOT NULL REFERENCES customers(id),
    source_type TEXT NOT NULL CHECK (source_type IN ('OVERPAYMENT','CREDIT_NOTE','DEPOSIT')),
    source_id BIGINT NOT NULL,  -- payment_id / credit_note_id / down_payment_id
    amount_cents BIGINT NOT NULL CHECK (amount_cents > 0),
    remaining_cents BIGINT NOT NULL CHECK (remaining_cents >= 0),
    status TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','EXHAUSTED','REFUNDED')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- Auto-create saat: overpayment, credit_note dengan refund_method='credit_balance'
-- Auto-consume saat: invoice berikutnya (kompensasi)
```

---

### H11 — `delivery_lines.unit_cost_cents` — Bisa Manual vs Harus Kalkulasi Engine

| Dokumen | Data Model §3.5: `delivery_lines.unit_cost_cents` editable |
|---|---|
| **Masalah** | Engine §9: HPP dihitung dari FIFO/rata-rata, bukan input manual |

**Rekomendasi:**
```sql
-- 1. Di delivery_lines: unit_cost_cents tetap ada tapi di-set oleh engine (read-only untuk user)
-- 2. Validasi di service layer:
--    IF unit_cost_cents != engine.CalculateCOGS(item_id, qty, method)
--       → Error: "HPP harus sesuai kalkulasi engine"
-- 3. Atau: buat delivery_lines.unit_cost_cents sebagai GENERATED ALWAYS? (Tidak feasible karena perlu query)
--    Lebih baik: validasi di Go service sebelum persist
```

---

### H12 — `sales_orders_lines` Tidak Ada `invoiced_qty`

| Dokumen | Data Model §3.5 |
|---|---|
| **Masalah** | `delivered_qty` ada, tapi `invoiced_qty` tidak — tidak bisa validasi invoice limit |

**Rekomendasi:** Sudah tercakup di **C2** — `ALTER TABLE sales_orders_lines ADD COLUMN invoiced_qty`.

---

### H13 — `inventory_movements.movement_type = ADJUSTMENT` — Use Case Tidak Jelas

| Dokumen | Data Model §3.7, Engine §9 |
|---|---|
| **Masalah** | Ada `OPNAME_IN/OUT` (dari stock opname) tapi ada juga `ADJUSTMENT` — bedanya? |

**Rekomendasi:**
```
1. Dokumentasikan di Engine §9.4:
   - OPNAME_IN/OUT: dari stock_opnames (wajib approval + alasan)
   - ADJUSTMENT: penyesuaian langsung tanpa opname formal (mis. koreksi batch, 
     retur rusak, scrap internal) — tetap butuh alasan di adjustment_reason
2. Atau: gabung OPNAME dan ADJUSTMENT → semua lewat stock_opnames
   (lebih aman — semua selisih stok harus approval)
```

---

### H14 — `fixed_assets` — Ada Kolom `impairment_cents` Tapi Tidak Ada Proses Impairment

| Dokumen | Data Model §3.8, Engine §12 |
|---|---|
| **Masalah** | Ada kolom `impairment_cents` + status `IMPAIRED`, tapi `asset_transactions.transaction_type` tidak punya `IMPAIRMENT` |

**Rekomendasi:**
```sql
-- 1. Tambah transaction type
--    CHECK constraint asset_transactions.transaction_type: tambahkan 'IMPAIRMENT'

-- 2. Tambah jurnal impairment di Engine §12:
--    Penurunan nilai (impairment): Dr 5207 Beban Penurunan Nilai Aset / Cr 1401 Aset Tetap
--    (ATAU Dr 5207 / Cr akun akumulasi impairment terpisah — kebijakan PSAK 48)

-- 3. Dokumentasikan: impairment test dilakukan manual oleh akuntan;
--    engine hanya mencatat jurnal impairment yang disetujui
```

---

## 🟡 Medium — 12 Isu Bisa Di-improve Setelah P1

### M1 — `report_mappings` Tidak Disebut di Architecture API Surface

| Masalah | Architecture §4.5 tidak punya endpoint REST untuk `report_mappings` |
|---|---|
| **Rekomendasi** | Tambahkan: `GET /accounts/:id/report-mappings`, `PUT /accounts/:id/report-mappings` |

---

### M2 — `credit_limit_changes` Tidak Dipakai Engine

| Masalah | Tabel `credit_limit_changes` ada di Data Model, tapi Engine §27.1 tidak referensi riwayat perubahan |
|---|---|
| **Rekomendasi** | Engine query `credit_limit_cents` dari `customers` (current). Riwayat di `credit_limit_changes` sebagai audit. Validasi: saat invoice, pakai credit limit saat ini (bukan historis). |

---

### M3 — Revenue Contract Link ke Sales Documents

| Masalah | `revenue_contracts` linked ke `customers` saja, tidak ke SO/INV |
|---|---|
| **Rekomendasi** | `sales_orders` dan `invoices` tambah `revenue_contract_id BIGINT NULL` — opsional, untuk kontrak multi-element |

---

### M4 — `payment_terms.cash_flow_category` Tidak Mengalir ke Journal Lines

| Masalah | Klasifikasi arus kas di termin, tapi `journal_lines` tidak punya field ini |
|---|---|
| **Rekomendasi** | Tambah `cash_flow_category TEXT` di `journal_lines` (NULL, diisi dari payment_term saat posting kas/bank) ATAU hitung dari akun+konteks saat generate laporan arus kas |

---

### M5 — Tabel `2401 Utang Bank` Muncul di Dua Baris (Duplikasi)

| Masalah | Engine §3.0.1: baris ke-21 (LOAN) dan ke-25 (Pinjaman) — keduanya `2401 Utang Bank` |
|---|---|
| **Rekomendasi** | Hapus baris duplikat ke-25; cukup satu `LOAN` di baris ke-21 |

---

### M6 — `recurring_templates.template_type NULL` vs Specific

| Masalah | Data Model: `template_type CHECK (journal/sales_invoice/purchase_bill/payment) NULL` |
|---|---|
| **Rekomendasi** | Fase 1: cukup `journal`. Tambah `sales_invoice` dll di fase lanjut (butuh generate invoice + jurnal, bukan cuma jurnal) |

---

### M7 — Tidak Ada `accounts_history` (Audit Perubahan COA)

| Masalah | Saat akun di-rename/dinonaktifkan, jurnal lama tetap pakai nama baru di UI |
|---|---|
| **Rekomendasi** | `accounts` table: simpan nama & kode di journal_lines sebagai snapshot? Atau tabel `accounts_history`: `account_id, field_name, old_value, new_value, changed_at` |

---

### M8 — `btree_gist` Extension Tidak Disebut di Architecture

| Masalah | Exclusion constraint periode butuh `btree_gist` extension; Architecture migration example tidak mention |
|---|---|
| **Rekomendasi** | Tambah di migration pertama: `CREATE EXTENSION IF NOT EXISTS btree_gist;` |

---

### M9 — Redis Failure Mode & Dead Letter Queue

| Masalah | Architecture tidak menyebutkan apa yang terjadi jika Redis down untuk asynq jobs |
|---|---|
| **Rekomendasi** | Tambah di Architecture §7.2: retry max 3x dengan exponential backoff; dead-letter queue untuk failed jobs; alert jika dead-letter queue > threshold |

---

### M10 — API Rate Limiting — Tidak Ada Detail Implementasi

| Masalah | Architecture §15 menyebutkan rate limiting tapi tidak implementasi |
|---|---|
| **Rekomendasi** | Middleware Go: token bucket per user/tenant. Endpoint sensitif (posting, auth) lebih ketat. Konfigurasi di env. |

---

### M11 — Idempotency Key TTL — Berapa Lama Disimpan?

| Masalah | Unique constraint `(tenant_id, idempotency_key)` permanen — tabel membesar |
|---|---|
| **Rekomendasi** | Tambah `idempotency_key_expires_at TIMESTAMPTZ` di `journal_entries`. Background job hapus expired keys (30 hari?). Atau: partial index hanya untuk keys < 30 hari. |

---

### M12 — WebSocket Auth & Reconnect Strategy Tidak Detail

| Masalah | Architecture §10 & §11.2 menyebut WebSocket untuk notifikasi real-time, tapi tidak ada detail |
|---|---|
| **Rekomendasi** | Tambahkan di Architecture: WS auth via JWT (query param atau first message), auto-reconnect dengan exponential backoff, fallback ke polling 30 detik |

---

## 📋 Per Target Dokumen

### Yang Perlu Diupdate di ACCOUNTING_ENGINE.md

| # | Isu | Prioritas | Bagian |
|---|---|---|---|
| 1 | HPP DO vs INV — tambah flag & kebijakan | 🔴 C1 | §7 |
| 2 | Akun 4201/4202 di COA standar | 🟠 H1 | §3.0.2 |
| 3 | Akun 2402 tipe eksplisit | 🟠 H2 | §3.0.1 |
| 4 | Batch FIFO retur — flag is_return_batch | 🟠 H3 | §9.4 |
| 5 | Selisih overhead default behavior | 🟠 H4 | §11.4 |
| 6 | PPh Final tarif — referensi tax_rates table | 🟠 H5 | §13.2 |
| 7 | Akrual auto-reversal — one reversal constraint | 🟠 H6 | §16.2 |
| 8 | Konsolidasi kurs per akun | 🟠 H7 | §22 |
| 9 | Suspense tracking & tipe akun | 🟠 H8 | §5, §21.5 |
| 10 | OCI flag di akun | 🟠 H9 | §12.2 |
| 11 | Hash algorithm detail (dari Architecture → Engine) | 🔴 C8 | §4 |
| 12 | created_at ordering → pakai journal_number | 🔴 C9 | §2.2 |
| 13 | Inventory ADJUSTMENT vs OPNAME | 🟠 H13 | §9.4 |
| 14 | Fixed asset impairment jurnal | 🟠 H14 | §12 |
| 15 | Duplikasi baris 2401 | 🟡 M5 | §3.0.1 |

### Yang Perlu Diupdate di DATA_MODEL.md

| # | Isu | Prioritas | Bagian |
|---|---|---|---|
| 1 | `deliveries.hpp_booked` flag | 🔴 C1 | §3.5 |
| 2 | `invoices.sales_order_id` | 🔴 C2 | §3.5 |
| 3 | `sales_orders_lines.invoiced_qty` | 🔴 C2 | §3.5 |
| 4 | `down_payment_allocations` table | 🔴 C3 | §3.5 |
| 5 | `ecl_policies` rework ke range-based | 🔴 C4 | §3.11 |
| 6 | `intercompany_transactions` FK ganti | 🔴 C5 | §3.18 |
| 7 | `accounts.requires_dimension` | 🔴 C6 | §3.2 |
| 8 | `customer_credit_balances` table | 🟠 H10 | §3.5 |
| 9 | `delivery_lines.unit_cost_cents` — validasi engine | 🟠 H11 | §3.5 |
| 10 | `suspense_items` table | 🟠 H8 | (baru) |
| 11 | `accounts.is_oci` flag | 🟠 H9 | §3.2 |
| 12 | `asset_transactions` tambah `IMPAIRMENT` | 🟠 H14 | §3.8 |
| 13 | `accruals.reversed_journal_id` + unique index | 🟠 H6 | §3.13 |
| 14 | `accounts_history` table | 🟡 M7 | (baru) |
| 15 | `journal_lines.cash_flow_category` | 🟡 M4 | §3.3 |
| 16 | `journal_entries.idempotency_key_expires_at` | 🟡 M11 | §3.3 |

### Yang Perlu Diupdate di ARCHITECTURE.md

| # | Isu | Prioritas | Bagian |
|---|---|---|---|
| 1 | HPP booking logic di service layer | 🔴 C1 | §4, §6.7 |
| 2 | Refresh token → httpOnly cookie (bukan localStorage) | 🔴 C10 | §11.2, §8.1 |
| 3 | Hash chain — sinkronkan dengan Engine spec | 🔴 C8 | §5.4 |
| 4 | `btree_gist` di migration | 🟡 M8 | §6 |
| 5 | Redis dead letter queue strategy | 🟡 M9 | §7.2 |
| 6 | API rate limiting implementation | 🟡 M10 | §15 |
| 7 | WebSocket auth & reconnect | 🟡 M12 | §10, §11.2 |
| 8 | `report_mappings` endpoint | 🟡 M1 | §4.5 |
| 9 | Idempotency key TTL | 🟡 M11 | §6.7 |
| 10 | Endpoint untuk `tax_breakdowns` | — | §4.5 |

---

## 📊 Test Case Tambahan

Dari analisis Engine §33, test matrix perlu ditambah:

| # | Skenario | Intent | Ekspektasi | Prioritas |
|---|---|---|---|---|
| T1 | Pembulatan PPN 12% × Rp 1.234 = Rp 148,08 | TAX_ROUNDING | PPN Rp 148; selisih 0,08 di baris pajak | 🟠 |
| T2 | Dua retry: idempotency key sama, payload berbeda | IDEMPOTENCY | `IDEMPOTENCY_KEY_REUSE` — hasil pertama | 🔴 |
| T3 | Multi-currency: revaluasi akhir periode | FX_REVALUATION | Selisih kurs → laba rugi (moneter) | 🟠 |
| T4 | Write-off piutang setelah ECL 100% | WRITE_OFF | Dr 1202 / Cr 1201; P&L tidak terpengaruh | 🟠 |
| T5 | Konsolidasi: laba antar entitas dalam persediaan | CONSOL_ELIM | Dr Pendapatan A / Cr HPP A / Cr Persediaan B | 🟠 |
| T6 | HPP di DO → INV tidak buat HPP (config delivery) | SALES_FLOW | Hanya satu jurnal HPP, di DO | 🔴 |
| T7 | HPP di INV → DO tidak buat HPP (config invoice) | SALES_FLOW | Hanya satu jurnal HPP, di INV | 🔴 |
| T8 | DP split ke 2 invoice (50/50) | SALES_FLOW | Dua alokasi; total = DP amount | 🔴 |
| T9 | Dimensi wajib tidak diisi | POSTING | `DIMENSION_REQUIRED` | 🔴 |
| T10 | Overhead under-applied → COGS | PRODUCTION_CLOSE | Dr 5101 / Cr 4902 | 🟠 |

---

## 🗺️ Urutan Pengerjaan yang Direkomendasikan

### Batch 1: Dokumen Dulu (1-2 hari, sebelum koding)

Perbarui ketiga dokumen secara paralel:

| # | Task | Dokumen |
|---|---|---|
| 1 | Tambah flag `hpp_booked` + kebijakan tenant | Engine §7 + Data Model §3.5 |
| 2 | Tambah `sales_order_id` di invoices + `invoiced_qty` | Data Model §3.5 |
| 3 | Buat tabel `down_payment_allocations` | Data Model §3.5 |
| 4 | Rework `ecl_policies` ke range-based | Data Model §3.11 |
| 5 | Ganti `intercompany_transactions` FK | Data Model §3.18 |
| 6 | Tambah `requires_dimension` di accounts | Data Model §3.2 |
| 7 | Ganti `intercompany_account_id` → dua akun | Combined C5 |
| 8 | Pindahkan hash spec dari Architecture ke Engine | Engine §4 + Architecture §5.4 |
| 9 | Perbaiki `created_at` ordering → `journal_number` | Engine §2.2 |
| 10 | Refresh token → httpOnly cookie | Architecture §8.1 + §11.2 |

### Batch 2: High-Priority (3-5 hari)

| # | Task |
|---|---|
| 11-24 | 14 high-priority items (H1-H14) |

### Batch 3: Medium (setelah P1)

| # | Task |
|---|---|
| 25-36 | 12 medium-priority items (M1-M12) |

### Batch 4: Test Matrix

| # | Task |
|---|---|
| 37 | Tulis 10 test case tambahan (T1-T10) |
| 38 | Golden test fixtures dari Engine §33 + test baru |

---

*Dokumen ini companion untuk ketiga spec document. Update spec document terlebih dahulu, baru implementasi kode mengikuti spec yang sudah direvisi.*
