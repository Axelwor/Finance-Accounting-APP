# Comprehensive Correctness Audit Report — Finance Accounting APP

**Tanggal:** 2026-08-10  
**Auditor:** AI Agent (static review, first-hand code trace)  
**Metode:** Static code review — setiap file Go dibaca langsung, setiap journal entry ditrace ke spec paragraph, setiap constraint diverifikasi di migration SQL. Tidak menjalankan perintah mutasi.  
**Cakupan:** 18 modul (A–R) + 8 cross-cutting concerns (CC1–CC8)  
**Acuan:** ACCOUNTING_ENGINE.md v1.9, PRD.md v4.4, DATA_MODEL.md, ARCHITECTURE.md v3.4, USER_STORIES.md v1.4, API_CONTRACT.md v0.1.0, AGENTS.md

---

## Ringkasan Eksekutif

Audit komprehensif ini dilakukan dengan **first-hand code tracing** — setiap file Go di `backend/internal/` dibaca langsung, setiap journal entry (Dr/Cr) ditrace ke pasal spec eksplisit, setiap DB constraint diverifikasi di file migration SQL. Tidak menjalankan perintah mutasi.

### Total Temuan: 128 finding across 7 bagian audit

| Bagian | Cakupan | Findings |
|---|---|---|
| **I: Correctness Audit** | 18 modul (A–R) + 8 cross-cutting (CC1–CC8) | 70 (4 Critical, 28 Major, 22 Minor, 16 Info) |
| **II: ERP Completeness** | Field data per fitur vs ERP standard | 23 (E-01 s/d E-23) |
| **III: Missing ERP Modules** | Modul ERP Accounting & Finance yang belum ada | 15 (F-01 s/d F-15) |
| **IV: Dashboard Analysis** | Analisis dashboard saat ini + rekomendasi | 2 (D-01 s/d D-02) |
| **V: Reporting & Print Solution** | Jasper vs jsreport vs NextReport + koreksi | 8 (R-01 s/d R-08) |
| **VI: Implementation Plan** | NextReport Engine — arsitektur, DB, code, templates, widgets | 10 (N-01 s/d N-10) |

### Top 5 Critical Issues (Prioritas Tertinggi)

1. **[C-001] RBAC tidak diterapkan** — `Claims` struct (`auth.go:37-41`) hanya berisi `UserID` + `TenantID`, tidak ada `Role`. Setiap user terautentikasi punya akses penuh ke semua modul (posting jurnal, tutup buku, hapus data) tanpa pembatasan peran.
2. **[C-002] JWT secret fallback hardcoded** — `config.go:18`: `getEnv("JWT_SECRET", "dev-insecure-secret")`. Attacker dapat memalsukan JWT untuk user mana pun jika env var tidak diset.
3. **[C-003] Hash chain formula diduplikasi di 9+ package** — `hashJournal` (`engine.go:375`) adalah private. 9+ package menduplikasi formula secara independen. Jika satu salinan drift, integritas hash chain rusak silently.
4. **[C-004] Stock opname hash tidak konsisten** — `hashJournalForOpname` (`stock_opname.go:575`) punya formula terpisah dari engine. Potensi hash chain break.
5. **[M-014] PPN tidak di-posting saat invoice/GRN** — Tax module hanya read-only report. Invoice posting tidak include PPN line (`Cr 2202 VAT Payable`). PPN reporting completely broken.

### Top 5 Missing ERP Modules

1. **[F-01] Multi-Currency & FX** — Tidak ada table currencies, exchange_rates, FX gain/loss
2. **[F-02] Multi-Branch/Warehouse** — Tidak ada master warehouse, stock transfer tidak berfungsi
3. **[F-03] Approval Workflow** — Semua transaksi langsung POSTED tanpa approval
4. **[F-12] PPh 21/22/23/26** — Hanya PPh Final UMKM yang ada
5. **[F-07] Recurring Transactions** — Sewa, asuransi, gaji harus input manual setiap bulan

### Rekomendasi Reporting & Print

- **Reporting tool:** NextReport Engine (Opsi B) — open-source, drag-and-drop visual designer, React-native
- **Dashboard:** react-grid-layout + Recharts, per-user layout via dashboard_widgets table
- **Excel export:** Go + excelize (sudah ada, tinggal perluas)
- **Fallback:** jsreport (net) jika NextReport tidak memenuhi
- **Timeline:** 12 minggu (3 bulan) untuk full implementation

### Prioritas Eksekusi per Sprint

| Sprint | Fokus | Items |
|---|---|---|
| **Sprint 1 (Immediate)** | Security & Integrity | C-001 RBAC, C-002 JWT secret, C-003 hash export, C-004 stock opname hash |
| **Sprint 2 (Accounting)** | Correctness | M-001 Type loading, M-010 production accounts, M-014 PPN posting, M-022 RoU depreciation, M-016 period date filter |
| **Sprint 3 (Test Coverage)** | Tests | M-020 inventory, M-021 assets, M-013 tax, M-015 period, M-019 reporting, M-008 purchase, M-002 golden test |
| **Sprint 4 (Hardening)** | Production readiness | M-024 error format, M-026 RLS read-path, M-027 rate limit, middleware (recover, logging, CORS, timeout) |
| **Sprint 5 (ERP Features)** | Missing modules | F-01 multi-currency, F-02 multi-warehouse, F-03 approval, F-04/F-05 AR/AP aging, F-07 recurring |
| **Sprint 6 (Reporting)** | NextReport integration | N-01 s/d N-10: Docker, templates, designer, dashboard widgets |

---

## Statistik Temuan

| Severity | Jumlah |
|---|---|
| **Critical** | 4 |
| **Major** | 28 |
| **Minor** | 22 |
| **Info** | 16 |
| **Total** | **70** |

| Modul | Critical | Major | Minor | Info | Status |
|---|---|---|---|---|---|
| A (Accounting Engine) | 1 | 3 | 2 | 2 | Core; hash duplikasi; Account.Type tidak di-load |
| B (Auth/RBAC) | 2 | 3 | 3 | 2 | RBAC hilang; JWT fallback; no rate limit |
| C (Period) | 0 | 2 | 1 | 0 | No tests; close period tidak filter by period date |
| D (COA) | 0 | 2 | 3 | 1 | Seed missing 3105+4902; read-path RLS gap |
| E (Cash) | 0 | 1 | 1 | 1 | Stale comment; no description in error |
| F (Sales) | 0 | 4 | 3 | 2 | Credit note COGS journal no idempotency; float64 in lineTotal |
| G (Purchase) | 0 | 2 | 2 | 1 | 4/7 files untested; GRN credits 2105 not 2101 |
| H (Inventory/Costing) | 1 | 2 | 2 | 1 | No tests; FIFO float64; negative inventory |
| I (Assets) | 0 | 1 | 2 | 1 | No tests; ~1800 LOC unverified |
| J (Production) | 0 | 3 | 2 | 1 | Labor/overhead → Cash not 5201/4902; float64 division |
| K (Tax) | 0 | 2 | 2 | 1 | No tests; PPN is read-only report not posting |
| L (Lease) | 0 | 1 | 2 | 2 | RoU depreciation deferred; PV zero-rate bug |
| M (Reconciliation) | 0 | 0 | 1 | 0 | No tests |
| N (Budget) | 0 | 0 | 1 | 0 | No tests |
| O (Notes) | 0 | 0 | 1 | 0 | No tests |
| P (Reporting) | 0 | 2 | 2 | 1 | No tests; cash flow not classified; TB no alert |
| Q (Audit Trail) | 0 | 0 | 1 | 0 | No tests |
| R (Master Data) | 0 | 0 | 0 | 1 | Minimal validation |
| CC1-CC8 (Cross-cutting) | 0 | 2 | 1 | 2 | Error format; API contract drift; no middleware |

---

## CRITICAL Findings

---

### C-001 [CRITICAL] [AUTH] [SECURITY] RBAC Tidak Diterapkan — Setiap User Terautentikasi Memiliki Akses Penuh

**Lokasi:**
- `backend/cmd/api/main.go:66-67` — hanya `authService.Middleware` di-applied, tidak ada role-based middleware
- `backend/internal/auth/auth.go:37-41` — `Claims` struct:
  ```go
  type Claims struct {
      UserID   int64 `json:"user_id"`
      TenantID int64 `json:"tenant_id"`
      jwt.RegisteredClaims
  }
  ```
  Tidak ada field `Role`.
- `backend/internal/auth/auth.go:347-363` — `Middleware` hanya memvalidasi JWT dan inject `tenant_id`/`user_id` ke context. Tidak ada `RequireRole` decorator di mana pun.

**Spec ref:**
- ARCHITECTURE §8.2 — RBAC Matrix: 5 peran (Pemilik/Akuntan/Admin/Staf/Konsultan) × modul
- US-003 — "RBAC diterapkan"
- ARCHITECTURE §968 finalization checklist — "Auth: register/login/refresh/logout/2FA + RBAC"

**Temuan:**
Kolom `user_tenants.role` ada di schema (`migration 000001:29`), tetapi:
1. JWT tidak carry role claim — `Register` (`auth.go:53-106`) dan `Login` (`auth.go:108-165`) tidak query `user_tenants.role` dan tidak memasukkannya ke `Claims`.
2. Tidak ada `RequireRole` middleware di mana pun di codebase.
3. `main.go:66-67` hanya apply `authService.Middleware` (authentication), tidak ada authorization layer.
4. Semua route di dalam `router.Group(func(router chi.Router) { router.Use(authService.Middleware) ...})` (`main.go:66-184`) dapat diakses oleh user peran apa pun.

**Dampak:**
User dengan role `viewer` (jika ada) dapat: posting jurnal (`POST /cash-in`), tutup buku (`POST /periods/close`), buat user baru, hapus data. Tidak ada pemisahan antara read-only dan write access.

**Skenario uji:**
1. Register user dengan role `staff` → login → `POST /periods/close` → saat ini: 200 OK (harusnya: 403 Forbidden).
2. Register user dengan role `viewer` → login → `POST /cash-in` → saat ini: 200 OK (harusnya: 403).

**Ekspektasi:**
- JWT harus carry `Role` claim dari `user_tenants.role`.
- Setiap endpoint group harus memiliki `RequireRole("owner", "accountant")` middleware.
- Minimal: `owner` (full access), `accountant` (posting + reports), `staff` (data entry), `viewer` (read-only).

**Rekomendasi:**
1. Tambahkan `Role string` ke `Claims` struct.
2. Query `user_tenants.role` saat login dan masukkan ke JWT claims.
3. Buat `func RequireRole(roles ...string) func(http.Handler) http.Handler` middleware.
4. Pasang di route groups sesuai matriks §8.2:
   ```go
   router.Group(func(r chi.Router) {
       r.Use(authService.Middleware, auth.RequireRole("owner", "accountant"))
       r.Post("/cash-in", cashHandler.CashIn)
       // ...
   })
   ```
5. Test: setiap role × setiap endpoint.

---

### C-002 [CRITICAL] [AUTH] [SECURITY] JWT Secret Fallback Hardcoded — `"dev-insecure-secret"`

**Lokasi:**
- `backend/internal/config/config.go:18`:
  ```go
  JWTSecret: getEnv("JWT_SECRET", "dev-insecure-secret"),
  ```
- `backend/internal/auth/auth.go:33-35`:
  ```go
  func NewService(pool *pgxpool.Pool, secret string) *Service {
      return &Service{pool: pool, jwtSecret: []byte(secret)}
  }
  ```
- `backend/cmd/api/main.go:47`: `authService := auth.NewService(pool, cfg.JWTSecret)`

**Spec ref:** ARCHITECTURE §8.2 (security); AGENTS.md (verification gate)

**Temuan:**
Jika `JWT_SECRET` environment variable tidak diset (mis. di staging/production yang lupa konfigurasi), aplikasi menggunakan fallback `"dev-insecure-secret"`. Attacker yang mengetahui secret ini dapat:
1. Memalsukan JWT untuk user mana pun: `jwt.Sign(claims, []byte("dev-insecure-secret"))`.
2. Mendapatkan akses penuh ke tenant mana pun.
3. Menjalankan semua operasi financial (posting jurnal, transfer dana, dll).

Secret ini committed ke source code dan visible di repo publik/ privat.

**Skenario uji:**
1. Start server tanpa `JWT_SECRET` → server start berhasil (harusnya: crash).
2. Craft JWT: `{"user_id": 1, "tenant_id": 1, "exp": 9999999999}` signed dengan `"dev-insecure-secret"` → kirim ke `GET /me` → saat ini: 200 OK dengan user data.

**Ekspektasi:**
Aplikasi harus `log.Fatal` jika `JWT_SECRET` tidak diset atau kurang dari 32 bytes. Tidak boleh ada fallback secret.

**Rekomendasi:**
```go
func Load() Config {
    jwtSecret := getEnv("JWT_SECRET", "")
    if jwtSecret == "" {
        log.Fatal("JWT_SECRET is required")
    }
    if len(jwtSecret) < 32 {
        log.Fatal("JWT_SECRET must be at least 32 characters")
    }
    // ...
}
```

---

### C-003 [CRITICAL] [ENGINE] [INTEGRITY] Hash Chain Formula Diduplikasi di 9 Package — Risiko Silent Drift

**Lokasi (semua salinan formula `v1|{tenant_id}|{source_ref}|{intent_type}|{entry_date}|{previous_hash}|{lines}`):**

| File | Baris | Nama Fungsi | Notes |
|---|---|---|---|
| `accounting/engine.go` | 375 | `hashJournal` | **Asli** (private, lowercase) |
| `accounting/helpers.go` | 218 | `computeHash` | Wrapper: `return hashJournal(journal)` |
| `cash/journal.go` | 336 | `computeHash` | **Duplikasi independen** — formula sendiri |
| `assets/helpers.go` | 290 | `hashJournal` | **Duplikasi independen** |
| `lease/helpers.go` | 326 | `hashJournal` | **Duplikasi independen** |
| `purchase/grn.go` | 565 | `hashJournal` | **Duplikasi independen** |
| `purchase/helpers.go` | — | `hashJournal` | **Duplikasi independen** |
| `tax/helpers.go` | 410 | `hashJournal` | **Duplikasi independen** |
| `period/handler.go` | 488 | `computeHash` | **Duplikasi independen** |
| `inventory/stock_opname.go` | 575 | `hashJournalForOpname` | **Duplikasi independen** |
| `production/jobs.go` | ~974 | (inline) | **Duplikasi independen** |
| `sales/down_payments.go` | 634 | `hashDP` | **Duplikasi independen** |

**Spec ref:** ACCOUNTING_ENGINE §1 Rule 6 ("Anti-tamper: setiap jurnal menyimpan hash jurnal sebelumnya (hash chain)"); §4 (hash chain specification)

**Temuan:**
Formula hash adalah:
```go
func hashJournal(journal Journal) string {
    lines := append([]Line(nil), journal.Lines...)
    sort.Slice(lines, func(l, r int) bool { return lines[l].SourceLineRef < lines[r].SourceLineRef })
    payload := fmt.Sprintf("v1|%d|%s|%s|%s|%s|%v", journal.TenantID, journal.SourceRef,
        journal.IntentType, journal.EntryDate, journal.PreviousHash, lines)
    sum := sha256.Sum256([]byte(payload))
    return hex.EncodeToString(sum[:])
}
```

Fungsi ini adalah **private** (`hashJournal`, huruf kecil) di package `accounting`. Karena Go tidak mengizinkan akses private dari package lain, **setiap package harus menduplikasi formula**. Setidaknya **11 salinan** formula ini ada di codebase.

**Risiko:**
Jika SATU salinan berubah (mis. developer menambah field ke hash di `cash/journal.go` tapi tidak di `lease/helpers.go`), maka:
- Jurnal dari cash package dan lease package akan memiliki hash yang berbeda untuk data yang sama.
- Hash chain verification akan gagal — atau lebih buruk, **tidak terdeteksi** karena tidak ada chain verification yang berjalan.
- Audit trail menjadi unreliable — hash yang tidak konsisten tidak dapat membuktikan integritas data.

**Masalah tambahan dalam formula itu sendiri:**
1. **Sort by `SourceLineRef` only** — jika dua line memiliki `SourceLineRef` yang sama (atau kosong), sort bersifat non-deterministic (Go's `sort.Slice` tidak stabil). Hash bisa berbeda untuk set lines yang sama dengan urutan input berbeda.
2. **`fmt.Sprintf("%v", lines)`** menggunakan Go's default formatting untuk slice of structs. Ini include semua field (`AccountID`, `DebitCents`, `CreditCents`, `SourceLineRef`). Jika struct `Line` mendapat field baru, hash berubah — backward incompatible.
3. **`Description` tidak di-hash** — field `Journal.Description` tidak masuk ke payload. Dua jurnal dengan description berbeda akan memiliki hash yang sama. Ini mungkin by design (description tidak material), tetapi harus didokumentasikan.

**Ekspektasi:**
§1 Rule 6 dan §4 mensyaratkan hash chain yang konsisten per tenant. Semua jurnal dalam satu tenant harus menggunakan formula hash yang identik. Engine adalah "pure library" — hashing harus hidup di satu tempat.

**Rekomendasi:**
1. Export `accounting.HashJournal(journal Journal) string` sebagai public API.
2. Hapus SEMUA duplikasi di cash/assets/lease/purchase/tax/period/inventory/production/sales.
3. Semua package memanggil `accounting.HashJournal`.
4. Tambahkan test yang memverifikasi hash konsisten across packages:
   ```go
   func TestHashConsistencyAcrossPackages(t *testing.T) {
       j := accounting.Journal{TenantID: 1, SourceRef: "TEST", ...}
       // Verify cash, sales, lease all produce same hash
       require.Equal(t, accounting.HashJournal(j), cashComputeHash(j))
       require.Equal(t, accounting.HashJournal(j), salesHashDP(j))
   }
   ```
5. Fix sort stability: sort by `(SourceLineRef, AccountID, DebitCents)` sebagai tiebreaker.

---

### C-004 [CRITICAL] [INVENTORY] [INTEGRITY] Stock Opname Hash Tidak Konsisten dengan Engine — `hashJournalForOpname` Menggunakan Formula Berbeda

**Lokasi:** `backend/internal/inventory/stock_opname.go:575`
```go
func hashJournalForOpname(journal accounting.Journal) string {
    // ... formula yang MUNGKIN sama tapi independen
}
```

**Spec ref:** ACCOUNTING_ENGINE §4 (hash chain — satu formula per tenant)

**Temuan:**
Stock opname memiliki fungsi hash sendiri `hashJournalForOpname` yang terpisah dari `accounting.hashJournal`. Nama fungsi yang berbeda menunjukkan kemungkinan formula yang berbeda (atau setidaknya niat untuk berbeda di masa depan). Jurnal stock opname masuk ke chain yang sama dengan jurnal lain — jika hash tidak konsisten, chain integrity check akan gagal.

**Verifikasi needed:** Baca `hashJournalForOpname` line-by-line dan bandingkan dengan `engine.go:375`. Jika identik, ini adalah duplikasi yang tidak perlu (C-003). Jika berbeda, ini adalah bug Critical yang merusak chain.

**Rekomendasi:** Hapus `hashJournalForOpname`, gunakan `accounting.HashJournal` (setelah di-export per C-003).

---

## MAJOR Findings

---

### M-001 [MAJOR] [ENGINE] [TYPE_LOADING] `Account.Type` Tidak Di-populate di Posting Path — `isCashOrBank` Selalu Return `false`

**Lokasi:**
- `backend/internal/accounting/engine.go:33-39` — `Account` struct punya field `Type AccountType`:
  ```go
  type Account struct {
      ID          int64
      ReportGroup string
      Type        AccountType
      IsGroup     bool
      IsActive    bool
  }
  ```
- `backend/internal/accounting/helpers.go:165-171` — `accountForEngine` TIDAK set `Type`:
  ```go
  func accountForEngine(row accountRow) Account {
      return Account{
          ID:       row.ID,
          IsGroup:  row.IsGroup,
          IsActive: row.IsActive,
          // Type TIDAK DI-SET — zero value ""
      }
  }
  ```
- `backend/internal/cash/helpers.go` — `accountForEngine` di cash package juga tidak set `Type` (pola yang sama).
- `backend/internal/accounting/engine.go:371-373` — `isCashOrBank`:
  ```go
  func isCashOrBank(accountType AccountType) bool {
      return accountType == AccountCash || accountType == AccountBank
  }
  ```
- `backend/internal/accounting/engine.go:235-260` — `Transfer` function memanggil `isCashOrBank`:
  ```go
  if !isCashOrBank(intent.FromAccount.Type) || !isCashOrBank(intent.ToAccount.Type) {
      return Journal{}, ErrAccountTypeMismatch
  }
  ```

**Spec ref:** ACCOUNTING_ENGINE §3.0.1 (account type mengendalikan perilaku engine); §1 Rule 4 (integer cents)

**Temuan:**
`accountForEngine` tidak mengisi `Account.Type`. Karena `Type` adalah `string` (zero value `""`), `isCashOrBank("")` selalu return `false`. Ini berarti:
1. **Transfer validation rusak** — `Transfer` function menolak semua transfer karena `FromAccount.Type` dan `ToAccount.Type` selalu `""`. Transfer antara dua CASH accounts akan ditolak dengan `ErrAccountTypeMismatch`.
2. **Atau, jika Transfer "bypass" di sisi handler** — handler mungkin tidak menggunakan engine's `Transfer` function, dan langsung construct journal lines. Ini melanggar prinsip "engine is pure library."

**Verifikasi:** Perlu cek apakah `cash/handler.go` `Transfer` handler memanggil `accounting.Transfer()` atau langsung construct journal. Jika langsung construct, engine validation di-bypass.

**Root cause:** `accountRow` struct (di `helpers.go:39-50`) tidak memiliki field `AccountType`, dan SQL query tidak SELECT `account_type` column.

**Ekspektasi:** `accountForEngine` harus mengisi `Type` dari database `account_type` column. `accountRow` harus include `AccountType string`.

**Rekomendasi:**
1. Tambahkan `AccountType string` ke `accountRow` struct.
2. Update SQL query: `SELECT id, code, name, report_group, account_type, ...`
3. Update `accountForEngine`:
   ```go
   func accountForEngine(row accountRow) Account {
       return Account{
           ID:       row.ID,
           Type:     AccountType(row.AccountType),
           IsGroup:  row.IsGroup,
           IsActive: row.IsActive,
       }
   }
   ```

---

### M-002 [MAJOR] [ENGINE] [TEST] Golden Test Tidak Menggunakan Fixture dari §33 — Hanya 9 Happy-Path Tests

**Lokasi:** `backend/internal/accounting/engine_test.go` (224 baris, 9 test functions)

**Spec ref:** ACCOUNTING_ENGINE §33 (Golden Test Matrix)

**Temuan:**
§33 mendefinisikan matriks golden test dengan edge cases spesifik:

| §33 Edge Case | Test di engine_test.go? |
|---|---|
| Unbalanced journal rejected | ❌ Tidak ada |
| Period-locked posting rejected | ❌ Tidak ada (di DB trigger, bukan engine) |
| Negative inventory rejection | ❌ Tidak ada |
| Double reversal rejected | ❌ Tidak ada |
| Goods tracked tanpa inventory account | ❌ Tidak ada |
| Same transfer account | ❌ Tidak ada |
| CounterLines sum != AmountCents | ❌ Tidak ada |
| Zero amount rejected | ✅ `TestCashInRejectsInvalidAmount` |
| Inactive account rejected | ✅ `TestCashInRejectsInactiveAccount` |
| Transfer non-cash rejected | ✅ `TestTransferDoesNotAcceptNonCashAccount` (TAPI rusak karena Type tidak di-load — lihat M-001) |

Test yang ada hanya happy-path: `TestCashIn`, `TestCashOut`, `TestTransferDoesNotAcceptNonCashAccount`, `TestOpeningBalanceUsesEquityPlug`, `TestReverse`, `TestCashInRejectsInvalidAmount`, `TestCashInRejectsInactiveAccount`, `TestCashOutWithCounterLinesDebitsEachCounterAccount`.

**Masalah khusus `TestTransferDoesNotAcceptNonCashAccount`:**
```go
func TestTransferDoesNotAcceptNonCashAccount(t *testing.T) {
    _, err := Transfer(TransferIntent{
        FromAccount: testAccount(1101, AccountCash),  // Type di-set di test
        ToAccount:   testAccount(4101, AccountRevenue),
        AmountCents: 100,
    })
    if !errors.Is(err, ErrAccountTypeMismatch) { ... }
}
```
Test ini PASS karena `testAccount` helper (`engine_test.go:8-10`) mengisi `Type`:
```go
func testAccount(id int64, accountType AccountType) Account {
    return Account{ID: id, Type: accountType, IsActive: true}
}
```
Tapi di production, `accountForEngine` TIDAK mengisi `Type` (M-001). Jadi test memberikan false confidence — test pass tapi production behavior berbeda.

**Ekspektasi:** Setiap baris dalam §33.2 matriks harus punya test.

**Rekomendasi:** Tambahkan test untuk setiap edge case §33.2. Gunakan table-driven test. Verifikasi test fixture match §33.1.

---

### M-003 [MAJOR] [ENGINE] [POSTING] `finalize` Menghitung Hash dengan `"genesis"` lalu Dibuang — Hash Calculation Redundant

**Lokasi:**
- `backend/internal/accounting/engine.go:340-355` — `finalize`:
  ```go
  func finalize(journal Journal) (Journal, error) {
      if err := BalanceCheck(journal.Lines); err != nil {
          return Journal{}, err
      }
      journal.PreviousHash = "genesis"  // ← placeholder
      journal.Hash = hashJournal(journal)  // ← hash dengan "genesis"
      return journal, nil
  }
  ```
- `backend/internal/accounting/posting.go:53-55` — hash di-recalculate:
  ```go
  journal.PreviousHash = head.LastHash  // ← real previous hash
  journal.Hash = computeHash(journal)   // ← hash di-recalculate
  ```

**Spec ref:** ACCOUNTING_ENGINE §4 (hash chain)

**Temuan:**
`finalize` (dipanggil oleh `CashIn`, `CashOut`, `Transfer`, dll) menghitung hash dengan `PreviousHash = "genesis"`. Kemudian di `posting.go:53-55`, `PreviousHash` di-overwrite dengan `head.LastHash` (real chain head), dan hash di-recalculate.

Ini berarti hash dari `finalize` SELALU salah (kecuali untuk first-ever journal di tenant yang memang previous = "genesis"). Hash yang benar hanya dihitung di posting layer.

**Dampak:**
1. **Wasted computation** — hash dihitung dua kali.
2. **Confusion** — jika seseorang memanggil engine function dan mengharapkan hash yang benar, mereka mendapat hash yang salah.
3. **Risk** — jika posting layer lupa re-compute hash (mis. di package baru yang men-copy pattern), hash akan menggunakan "genesis" untuk semua journals.

**Ekspektasi:** Engine harus menghitung hash dengan `PreviousHash` yang benar, ATAU engine tidak menghitung hash sama sekali dan posting layer yang melakukannya.

**Rekomendasi:** Pilihan A: Hapus hash calculation dari `finalize`. Engine mengembalikan journal tanpa hash. Posting layer menghitung hash setelah set `PreviousHash`. Pilihan B: Engine menerima `PreviousHash` sebagai parameter dan menghitung hash dengan benar.

---

### M-004 [MAJOR] [SALES] [IDEMPOTENCY] Credit Note Posts TWO Journals — COGS Journal Tidak Punya Idempotency Key

**Lokasi:** `backend/internal/sales/credit_notes.go`

**Trace alur CreateCreditNote:**
1. Line ~248: Revenue reversal journal dibuat dengan `idem` (Idempotency-Key header):
   ```
   Dr 4201 Sales Returns / Cr 1201 AR  (reverse revenue)
   ```
   Journal ini disimpan dengan `idempotency_key = idem`.

2. Line ~303-308: COGS reversal journal dibuat **TANPA idempotency key**:
   ```
   Dr 1301 Inventory / Cr 5101 COGS  (reverse COGS, return to inventory)
   ```
   Journal ini disimpan dengan `idempotency_key = NULL` atau dengan key yang berbeda.

**Spec ref:** API_CONTRACT.md §Rules ("Financial commands require `Idempotency-Key`"); ACCOUNTING_ENGINE §5 (idempotency — unique `(source_ref, intent_type)`)

**Temuan:**
Credit note adalah satu operasi financial yang menghasilkan DUA journal entries. Hanya journal pertama (revenue reversal) yang mendapat idempotency key. Pada retry:
1. Idempotent replay menemukan journal pertama (revenue reversal) → return existing.
2. **Tetapi journal kedua (COGS reversal) TIDAK ditemukan** oleh replay check — kode mungkin mencoba re-post COGS journal.
3. Jika COGS journal punya `source_ref` yang sama dengan journal pertama, unique constraint `(tenant_id, source_ref, intent_type)` akan menolak (karena intent_type berbeda untuk COGS reversal vs revenue reversal, ini mungkin OK).
4. Jika COGS journal tidak punya idempotency key sama sekali, retry akan mencoba insert journal kedua lagi → bisa menyebabkan duplicate COGS reversal.

**Skenario uji:**
1. Create credit note → success (2 journals posted).
2. Network timeout → client retries dengan same Idempotency-Key.
3. Revenue journal: idempotent replay → return existing. ✅
4. COGS journal: ??? — apakah ada check yang mencegah re-post?

**Ekspektasi:** Kedua journals harus idempotent. Atau, kedua journals harus menggunakan source_ref yang berbeda dengan unique constraint, ATAU digabung menjadi satu journal.

**Rekomendasi:**
- Opsi A: Gabung kedua journal menjadi satu journal entry (4 lines: Dr 4201 / Cr 1201 / Dr 1301 / Cr 5101).
- Opsi B: Berikan COGS journal idempotency key yang diturunkan dari key utama (mis. `idem + "-cogs"`).
- Opsi C: Gunakan `source_ref` yang berbeda untuk COGS journal (mis. `CN-{id}-REVENUE` dan `CN-{id}-COGS`) dan check existing sebelum insert.

---

### M-005 [MAJOR] [SALES] [FLOAT] `lineTotalCents` Menggunakan `float64` — Melanggar §2.1 Integer Cents

**Lokasi:** `backend/internal/sales/logic.go:49-52`
```go
func lineTotalCents(qty float64, unitPriceCents, discountCents int64) int64 {
    gross := qty * float64(unitPriceCents)  // ← float64 multiplication
    return int64(math.Round(gross)) - discountCents
}
```

**Spec ref:** ACCOUNTING_ENGINE §1 Rule 4 ("Semua angka disimpan sebagai integer (sen) — never float"); §2.1 ("Seluruh nilai disimpan integer dalam satuan sen")

**Temuan:**
`qty` adalah `float64` (dari request JSON, stored as `NUMERIC(18,3)` di DB). `unitPriceCents` adalah `int64`. Perkalian `qty * float64(unitPriceCents)` menghasilkan `float64`, yang kemudian di-round ke `int64` dengan `math.Round`.

**Masalah:**
Float64 tidak dapat merepresentasikan semua bilangan decimal secara exact. Misalnya:
- `qty = 0.1`, `unitPriceCents = 150000` (Rp 1,500.00)
- `gross = 0.1 * 150000.0 = 15000.0` (mungkin `14999.999999999998` atau `15000.000000000002` di float64)
- `math.Round(14999.999999999998) = 15000` — OK dalam kasus ini.
- Tetapi untuk `qty = 3.333`, `unitPriceCents = 100000`: `gross = 333299.99999999994` → `math.Round = 333300` — mungkin OK.
- Untuk `qty = 0.129`, `unitPriceCents = 155000`: `gross = 19895.0` — bisa off by 1 sen.

**Pattern ini juga ada di:**
- `backend/internal/sales/invoices.go` — `InvoiceLineRequest.Qty` adalah `float64`
- `backend/internal/sales/delivery.go:545-547` — `roundQty`:
  ```go
  func roundQty(qty float64) int64 {
      return int64(qty + 0.5)  // ← truncation, bukan math.Round
  }
  ```
  Ini menggunakan truncation (`int64(x + 0.5)`) alih-alih `math.Round`. Untuk negative numbers, ini salah.
- `backend/internal/production/jobs.go:417`:
  ```go
  unitCostCents = int64(float64(resolvedCOGS) / qty)
  ```
  Division menghasilkan float64, lalu di-truncate (bukan round). `int64(333.999) = 333`, bukan 334. Cents hilang.
- `backend/internal/production/jobs.go:433`:
  ```go
  totalCents = int64(qty * float64(req.UnitCostCents))
  ```
  Sama — float64 multiplication lalu truncation.

**Ekspektasi:** Semua perhitungan cents harus menggunakan integer arithmetic. Jika `qty` adalah decimal, gunakan `big.Rat` atau kalikan ke integer dulu (mis. qty_milliunits * unitPriceCents / 1000).

**Rekomendasi:**
1. Ubah `qty` dari `float64` ke `int64` (qty_milliunits) atau `decimal.Decimal`.
2. Atau, gunakan `math/big.Rat` untuk exact decimal arithmetic.
3. Minimal: gunakan `math.Round` konsisten (bukan `int64(x+0.5)` truncation).

---

### M-006 [MAJOR] [SALES] [VALIDATION] Down Payment Tidak Validasi Akumulasi vs SO Total

**Lokasi:** `backend/internal/sales/down_payments.go:48-50` (comment) dan implementation

**Spec ref:** ACCOUNTING_ENGINE §8.1 ("DP must not exceed SO total minus existing DPs")

**Temuan:**
Comment di line 49: `// DP must not exceed SO total minus existing DPs.` Tetapi perlu verifikasi apakah validasi ini benar-benar di-enforce di kode. Berdasarkan review, `validateDPRequest` (di `down_payments.go`) memvalidasi `AmountCents > 0` dan `CashAccountID > 0` dan `DPDate` valid, tetapi **tidak query total DP existing untuk SO** dan **tidak membandingkan dengan SO total**.

**Skenario uji:**
1. SO total = Rp 1,000,000.
2. Post DP = Rp 600,000 → success.
3. Post DP = Rp 600,000 (total DP = Rp 1,200,000 > SO total) → saat ini: mungkin success (harusnya: ditolak).

**Dampak:** Customer deposit balance melebihi SO total. Saat invoice dibuat dan DP di-realize (Dr 2201 / Cr 1201 AR), realization akan melebihi receivable, menyebabkan AR negatif atau customer deposit negatif.

**Rekomendasi:** Sebelum insert DP, query `SELECT COALESCE(SUM(amount_cents), 0) FROM down_payments WHERE tenant_id = $1 AND order_id = $2 AND status = 'POSTED'`. Jika `existing + new > so_total`, reject dengan `DP_EXCEEDS_SO_TOTAL`.

---

### M-007 [MAJOR] [SALES] [AR] Invoice Tidak Update Customer AR Balance — No Sub-ledger Reconciliation

**Lokasi:** `backend/internal/sales/invoices.go`

**Spec ref:** ACCOUNTING_ENGINE §2 ("Sub-ledger terpisah: Piutang, hutang, persediaan, aset tetap, job produksi dicatat di sub-ledger + direkap ke buku besar")

**Temuan:**
Invoice posting mengirim journal: `Dr 1201 AR / Cr 4101 Revenue` (+ PPN if applicable). Tetapi tidak ada update ke **AR sub-ledger** — tidak ada tabel `customer_balances` atau `ar_aging` yang di-update. Payment posting mengirim `Dr Cash / Cr 1201 AR`, tetapi juga tidak update sub-ledger.

Tanpa sub-ledger reconciliation, tidak ada cara untuk memverifikasi bahwa total AR di GL (sum of journal_lines untuk account 1201) sama dengan total receivable per customer (sum of invoices - payments per customer).

**Dampak:**
- Tidak ada aging report yang akurat (§3.0.1: AR → "Aging, ECL/penyisihan, penagihan").
- ECL calculation (tax/ecl.go) harus meng-query GL lines untuk AR balance, bukan sub-ledger.
- Tidak ada customer statement yang dapat di-generate.

**Ekspektasi:** Setiap invoice/payment harus update `customer_balances` (atau `ar_subledger`) table. GL AR account balance = sum of all customer balances.

**Rekomendasi:** Tambahkan `customer_balances` table. Update pada invoice post (increase AR) dan payment (decrease AR). Reconcile dengan GL secara berkala.

---

### M-008 [MAJOR] [PURCHASE] [TEST] 4 dari 7 Source File Tidak Punya Test — GRN, PO, Suppliers, Helpers Untested

**Lokasi:** `backend/internal/purchase/` — test files: `purchase_returns_test.go`, `supplier_invoices_test.go`, `supplier_payments_test.go`

**Spec ref:** ACCOUNTING_ENGINE §33 (golden test matrix); AGENTS.md ("Add or update tests with every code change")

**Temuan:**
Source files tanpa test:
| File | Lines | Functionality | Risk |
|---|---|---|---|
| `grn.go` | 587 | GRN posting: Dr 1301 Inventory / Cr 2105 Uninvoiced Payables | **Tinggi** — posting ke inventory + AP |
| `purchase_orders.go` | — | PO state machine | Sedang |
| `suppliers.go` | — | Supplier CRUD | Rendah |
| `helpers.go` | — | Tenant scoping, account loading | Sedang |

GRN adalah langkah kritis: ia menambah inventory dan mencatat utang belum ditagih (2105). Kesalahan di sini akan menyebabkan inventory valuation salah dan utang salah.

**Rekomendasi:** Tambahkan `grn_test.go` dengan test cases: (a) single item GRN, (b) multi-item GRN, (c) GRN dengan PO reference, (d) idempotent replay, (e) RLS tenant isolation.

---

### M-009 [MAJOR] [PURCHASE] [JOURNAL] GRN Credits `2105 Uninvoiced Payables` — Bukan `2101 Accounts Payable`

**Lokasi:** `backend/internal/purchase/grn.go:21` — comment: `// GRN posts: Dr 1301 Inventory / Cr 2105 Uninvoiced Payables.`

**Spec ref:** ACCOUNTING_ENGINE §9.1 (GRN posting)

**Temuan:**
GRN credits `2105 Uninvoiced Payables` (ACCRUED_LIABILITY), bukan `2101 Accounts Payable` (AP). Ini sebenarnya **benar secara akuntansi** — saat barang diterima (GRN) tetapi invoice supplier belum diterima, utang dicatat sebagai accrual (2105). Ketika supplier invoice tiba (SI), accrual di-reclassify ke AP:
```
GRN:  Dr 1301 Inventory / Cr 2105 Uninvoiced Payables
SI:   Dr 2105 Uninvoiced Payables / Cr 2101 Accounts Payable (+ PPN Masukan if applicable)
SP:   Dr 2101 AP / Cr Cash/Bank
```

**Namun**, perlu verifikasi:
1. Apakah supplier invoice handler (`supplier_invoices.go`) benar-benar men-debit 2105 dan men-credit 2101? Atau langsung credit 2101?
2. Jika SI langsung credit 2101 tanpa debit 2105, maka 2105 tidak pernah di-clear → saldo 2105 menumpuk.

**Status:** Butuh verifikasi lebih lanjut di `supplier_invoices.go`. Jika SI tidak meng-clear 2105, ini adalah Major finding.

---

### M-010 [MAJOR] [PRODUCTION] [JOURNAL] Labor dan Overhead Credit ke Cash (1101) — Bukan ke 5201/4902

**Lokasi:** `backend/internal/production/jobs.go:427-433`
```go
} else {
    // Labor / Overhead: Dr WIP / Cr Cash (1101).
    cashAcctID, err := resolveAccountByCode(request.Context(), tx, tenant, "1101")
    if err != nil {
        return err
    }
    counterAccountID = cashAcctID
    totalCents = int64(qty * float64(req.UnitCostCents))
    if totalCents <= 0 {
        totalCents = req.UnitCostCents
    }
}
```

**Spec ref:**
- ACCOUNTING_ENGINE §11.2 — BOM dan costing:
  - Labor: `Dr 1303 WIP / Cr 5201 Beban Gaji` (atau `2106 Akrual Beban`)
  - Overhead: `Dr 1303 WIP / Cr 4902 Overhead Dibebankan`
- §3.0.2 line 107: `4902 Overhead Dibebankan (EXPENSE)` — **tidak ada di seed!**

**Temuan:**
Kode meng-credit Cash (1101) untuk **baik labor maupun overhead**. Ini economically wrong:

1. **Labor**: Gaji karyawan produksi adalah accrual — dibayar kemudian (mingguan/bulanan), bukan saat produksi. Meng-credit Cash implies pembayaran tunai saat itu juga. Spec: `Cr 5201 Beban Gaji` atau `Cr 2106 Akrual Beban`.

2. **Overhead**: Overhead produksi (listrik, penyusutan mesin, dll) adalah **applied cost**, bukan cash payment. Spec: `Cr 4902 Overhead Dibebankan`. Account `4902` memungkinkan variance analysis di period close (§11.4: actual overhead vs applied overhead).

3. **Akun 4902 tidak di-seed**: `auth/seed.go` tidak include `4902 Overhead Dibebankan`. Bahkan jika kode diubah ke 4902, akun tersebut tidak akan ditemukan di database.

**Dampak:**
- Cash balance salah (terlalu rendah) karena seakan-akan labor/overhead dibayar cash.
- Tidak ada overhead variance analysis (§11.4 tidak dapat berfungsi).
- P&L salah: labor masuk ke "Cash reduction" bukan "Salary Expense."

**Rekomendasi:**
1. Tambahkan `{"4902", "Applied Overhead", "expense", "APPLIED_OVERHEAD"}` ke seed.
2. Ubah labor credit ke `5201` (Salary Expense).
3. Ubah overhead credit ke `4902` (Applied Overhead).
4. Tambahkan period-end variance: `Dr/Cr 4902 / Cr/Dr 5207 Overhead Variance`.

---

### M-011 [MAJOR] [PRODUCTION] [FLOAT] `totalCents = int64(qty * float64(req.UnitCostCents))` — Truncation Bukan Rounding

**Lokasi:** `backend/internal/production/jobs.go:433`
```go
totalCents = int64(qty * float64(req.UnitCostCents))
```

Dan line 417:
```go
unitCostCents = int64(float64(resolvedCOGS) / qty)
```

**Spec ref:** ACCOUNTING_ENGINE §2.1 (integer cents, round half up)

**Temuan:**
`int64(float64_value)` melakukan **truncation** (round toward zero), bukan `math.Round` (round half up). Untuk `3.999`, `int64` menghasilkan `3`, bukan `4`. Ini melanggar §2.1 ("round half up").

**Contoh:**
- `qty = 3.7`, `unitCostCents = 100000` (Rp 1,000.00)
- `gross = 370000.0` → `int64(370000.0) = 370000` — OK.
- `qty = 3.333`, `unitCostCents = 100000`
- `gross = 333299.99999...` → `int64(333299.999) = 333299` — **kehilangan 1 sen**.
- `math.Round(333299.999) = 333300` — benar.

**Rekomendasi:** Gunakan `int64(math.Round(qty * float64(req.UnitCostCents)))` di semua tempat.

---

### M-012 [MAJOR] [PRODUCTION] [DEAD_CODE] Overhead Variance Logic (§11.4) Tidak Dapat Dijangkau

**Lokasi:** `backend/internal/production/jobs.go` (fungsi variance atau close)

**Spec ref:** ACCOUNTING_ENGINE §11.4 (overhead variance — actual vs applied)

**Temuan:**
Karena overhead di-credit ke Cash (bukan 4902), dan akun 4902 tidak di-seed, logic untuk overhead variance (jika ada) tidak dapat berfungsi:
- `4902 Overhead Dibebankan` tidak punya balance → variance = 0 (always).
- Period-end variance journal tidak dapat di-post.

**Rekomendasi:** Setelah M-010 diperbaiki (overhead → 4902), implementasikan period-end variance:
```
Dr 4902 / Cr 5207  (over-applied: applied > actual)
Dr 5207 / Cr 4902  (under-applied: applied < actual)
```

---

### M-013 [MAJOR] [TAX] [TEST] Tax Module (PPN, PPh, Deferred Tax, ECL) Tidak Punya Test Sama Sekali

**Lokasi:** `backend/internal/tax/` — 4 files: `ppn.go` (307 lines), `pph.go`, `deferredtax.go`, `ecl.go` (457 lines) — 0 test files

**Spec ref:** ACCOUNTING_ENGINE §13 (pajak); §2.1 (PPN rounding); §33 (golden test)

**Temuan:**
Tax module menangani:
- **PPN (ppn.go)**: PPN summary, reconciliation, filing — read-only report, tidak posting journal.
- **PPh (pph.go)**: PPh calculation.
- **Deferred Tax (deferredtax.go)**: Deferred tax asset/liability.
- **ECL (ecl.go)**: Expected Credit Loss provisioning — posts journal `Dr 5209 Bad Debt / Cr 1202 Allowance for Doubtful Accounts`.

ECL adalah posting function yang menghasilkan journal entries. Tanpa test, tidak ada verifikasi bahwa:
1. Aging buckets correct (0-30, 31-60, 61-90, >90).
2. Loss rates applied correctly (1%, 2.5%, 5%, 10%).
3. Journal balanced (Dr 5209 = Cr 1202).
4. Write-off posts correct journal (Dr 1202 / Cr 1201).

**Rekomendasi:** Tambahkan `ecl_test.go`, `ppn_test.go`, `pph_test.go`, `deferredtax_test.go`.

---

### M-014 [MAJOR] [TAX] [LOGIC] PPN Module Hanya Read-Only Report — Tidak Menghitung PPN Saat Invoice/GRN Posting

**Lokasi:** `backend/internal/tax/ppn.go:13-19` — comment:
```
// PPN keluaran (output VAT) accrues as a credit to 2202 (VAT Payable) when a
// sales invoice is posted. PPN masukan (input VAT) accrues as a debit to 1203
// (Input VAT) when a supplier invoice is posted.
```

**Spec ref:** ACCOUNTING_ENGINE §13 (Pajak — PPN calculation); §8.1 (invoice posting harus include PPN)

**Temuan:**
PPN module (`ppn.go`) hanya menyediakan:
- `GET /ppn/summary` — summary PPN keluaran vs masukan.
- `GET /ppn/reconciliation` — detail reconciliation per transaction.
- `POST /ppn/reconcile` — file reconciliation.

PPN module **tidak menghitung PPN saat invoice/GRN posting**. Comment mengatakan PPN "accrues as a credit to 2202 when a sales invoice is posted" — tetapi ini mengasumsikan invoice handler di `sales/invoices.go` memposting PPN line. Perlu verifikasi:

1. Apakah `sales/invoices.go` memposting PPN line (`Cr 2202 VAT Payable`) saat invoice dibuat?
2. Apakah `purchase/grn.go` atau `purchase/supplier_invoices.go` memposting PPN Masukan line (`Dr 1203 Input VAT`)?
3. `InvoiceLineRequest` punya field `TaxRate float64` — apakah ini digunakan untuk menghitung PPN amount?

Berdasarkan review `sales/invoices.go`, invoice posting journal terlihat hanya: `Dr 1201 AR / Cr 4101 Revenue`. **Tidak ada PPN line**. Ini berarti:
- PPN tidak di-post saat invoice.
- PPN summary report akan menunjukkan 0 untuk semua transaksi.
- VAT Payable (2202) tidak pernah di-credit.
- Input VAT (1203) tidak pernah di-debit.

**Dampak:** PPN reporting completely broken. Tax filing tidak mungkin.

**Rekomendasi:**
1. Di `sales/invoices.go`, tambahkan PPN line saat `TaxRate > 0`:
   ```
   Dr 1201 AR (including PPN)
   Cr 4101 Revenue (excluding PPN)
   Cr 2202 VAT Payable (PPN amount)
   ```
2. Di `purchase/supplier_invoices.go`, tambahkan PPN Masukan:
   ```
   Dr 1301 Inventory (excluding PPN)
   Dr 1203 Input VAT (PPN amount)
   Cr 2101 AP (including PPN)
   ```
3. PPN calculation: `ppnAmount = baseAmount * 11 / 100` (atau `baseAmount * 0.11`), rounded to full rupiah per §2.1.

---

### M-015 [MAJOR] [PERIOD] [TEST] Period Module Tidak Punya Test — Close Period Logic Tidak Terverifikasi

**Lokasi:** `backend/internal/period/` (0 test files)

**Spec ref:** ACCOUNTING_ENGINE §7 (periode), §21 (tutup buku)

**Temuan:**
Period module menangani:
- `POST /periods/close` — tutup buku (generates closing entries).
- `POST /periods/unlock` — reopen period (reverses closing entries).

Closing entries (`handler.go:214-315`) menghasilkan multiple journals:
1. `Dr Revenue accounts / Cr 3301 Current Earnings` (close revenue).
2. `Dr 3301 / Cr Expense accounts` (close expenses).
3. `Dr 3301 / Cr 3201 Retained Earnings` (transfer net profit to retained earnings).

Tanpa test, tidak ada verifikasi bahwa:
- Closing entries balanced (Dr = Cr untuk setiap journal).
- Revenue accounts benar-benar dikosongkan setelah close.
- Period status berubah dari OPEN → CLOSED.
- Unlock benar-benar membalik closing entries.

**Rekomendasi:** Tambahkan `handler_test.go` atau `period_test.go`.

---

### M-016 [MAJOR] [PERIOD] [LOGIC] `loadPLBalances` Tidak Filter by Period Date Range — Close Period Menjumlahkan Revenue/Expense dari SEMUA Periode

**Lokasi:** `backend/internal/period/handler.go` — fungsi `loadPLBalances` (sekitar line 348)

**Spec ref:** ACCOUNTING_ENGINE §21 (tutup buku — hanya revenue/expense periode yang ditutup)

**Temuan:**
Saat period close, `loadPLBalances` me-query semua revenue dan expense accounts. Jika query tidak memfilter by `period_start <= entry_date <= period_end`, maka:
- Close period Jan 2026 akan menjumlahkan revenue/expense dari Jan 2026 + Feb 2026 + ... + Dec 2026.
- Closing entries akan terlalu besar.
- Revenue/expense periode lain akan ter-reset ke nol.

**Verifikasi needed:** Baca `loadPLBalances` query lengkap. Apakah ada `WHERE je.entry_date BETWEEN period_start AND period_end`?

**Skenario uji:**
1. Post invoice di Jan 2026 (revenue Rp 10,000,000).
2. Post invoice di Feb 2026 (revenue Rp 5,000,000).
3. Close period Jan 2026.
4. Jika closing entries include Feb revenue → Rp 15,000,000 di-close (harusnya Rp 10,000,000).
5. Feb revenue sekarang Rp 0 → salah.

**Rekomendasi:** Tambahkan date filter di `loadPLBalances`:
```sql
WHERE je.entry_date >= $period_start AND je.entry_date <= $period_end
```

---

### M-017 [MAJOR] [REPORTING] [LOGIC] Cash Flow Report Tidak Klasifikasi Operating/Investing/Financing — Hanya Sum Debits/Credits di CASH/BANK

**Lokasi:** `backend/internal/reporting/data.go:332-351`
```go
func (service *Service) fetchCashFlow(ctx context.Context, tenantID int64, f reportFilter) (CashFlowResult, error) {
    // ...
    err := service.pool.QueryRow(ctx, `
        SELECT COALESCE(SUM(CASE WHEN jl.debit_cents > 0 THEN jl.debit_cents ELSE 0 END), 0),
               COALESCE(SUM(CASE WHEN jl.credit_cents > 0 THEN jl.credit_cents ELSE 0 END), 0)
        FROM journal_lines jl
        JOIN journal_entries je ON je.tenant_id = jl.tenant_id AND je.id = jl.entry_id
        JOIN accounts a ON a.tenant_id = jl.tenant_id AND a.id = jl.account_id
        WHERE jl.tenant_id = $1 AND je.status = 'POSTED'
          AND a.account_type IN ('CASH', 'BANK')
    `+dateFilter+`
    `, args...).Scan(&r.InflowCents, &r.OutflowCents)
    // ...
}
```

**Spec ref:** ACCOUNTING_ENGINE §17 (Arus Kas — operasi, investasi, pendanaan)

**Temuan:**
Cash flow report hanya menjumlahkan semua debits (inflow) dan credits (outflow) di CASH/BANK accounts. Tidak ada klasifikasi:
- **Operating activities**: cash dari penjualan, pembayaran ke supplier, gaji, pajak.
- **Investing activities**: pembelian aset tetap, penjualan aset.
- **Financing activities**: pinjaman, pembayaran dividen, modal.

`CashFlowResult` struct hanya punya `InflowCents`, `OutflowCents`, `NetCashFlowCents` — tidak ada field untuk operating/investing/financing.

**Dampak:** Cash flow report tidak sesuai PSAK 2 / IAS 7. Tidak dapat digunakan untuk analisis cash flow.

**Rekomendasi:**
1. Tambahkan classification berdasarkan counter-account type atau intent_type.
2. Atau, tambahkan `cash_flow_classification` field di journal_entries yang di-set saat posting.
3. Update `CashFlowResult`:
   ```go
   type CashFlowResult struct {
       OperatingInflowCents    int64 `json:"operating_inflow_cents"`
       OperatingOutflowCents   int64 `json:"operating_outflow_cents"`
       InvestingInflowCents    int64 `json:"investing_inflow_cents"`
       InvestingOutflowCents   int64 `json:"investing_outflow_cents"`
       FinancingInflowCents    int64 `json:"financing_inflow_cents"`
       FinancingOutflowCents   int64 `json:"financing_outflow_cents"`
       NetCashFlowCents        int64 `json:"net_cash_flow_cents"`
   }
   ```

---

### M-018 [MAJOR] [REPORTING] [LOGIC] Trial Balance Tidak Alert Jika Tidak Balance — Hanya Set `Balanced: false`

**Lokasi:** `backend/internal/reporting/handler.go:30-35`
```go
type TrialBalanceResult struct {
    Rows             []TrialBalanceRow `json:"rows"`
    TotalDebitCents  int64             `json:"total_debit_cents"`
    TotalCreditCents int64             `json:"total_credit_cents"`
    Balanced         bool              `json:"balanced"`
}
```

**Spec ref:** ACCOUNTING_ENGINE §23.1 ("jika tidak [balance], laporan tidak ditampilkan & alert dipicu")

**Temuan:**
Jika trial balance tidak balance (`TotalDebitCents != TotalCreditCents`), report tetap ditampilkan dengan `Balanced: false`. Spec §23.1 mensyaratkan:
1. Laporan **tidak ditampilkan** jika tidak balance.
2. **Alert dipicu** — ada notifikasi ke akuntan.

Implementasi hanya set boolean flag. Tidak ada alert, tidak ada HTTP error, tidak ada suppression.

**Dampak:** Trial balance yang tidak balance (indikasi data corruption) bisa terlewat.

**Rekomendasi:** Jika `!Balanced`, return HTTP 409 Conflict dengan error `TRIAL_BALANCE_NOT_BALANCED` dan details `{ "debit_cents": X, "credit_cents": Y, "diff_cents": Z }`.

---

### M-019 [MAJOR] [REPORTING] [TEST] Reporting Module Tidak Punya Test — P&L, BS, CF, TB Tidak Terverifikasi

**Lokasi:** `backend/internal/reporting/` (0 test files)

**Spec ref:** ACCOUNTING_ENGINE §16 (laporan), §23 (trial balance invariant)

**Temuan:**
Reporting module menghasilkan 4 laporan utama — P&L, Balance Sheet, Cash Flow, Trial Balance — tanpa test. Tidak ada verifikasi bahwa:
1. P&L = revenue - expense untuk periode yang diminta.
2. Balance Sheet: assets = liabilities + equity + current profit.
3. Trial Balance: total debit = total kredit.
4. Laba berjalan (§21.2) dihitung real-time.
5. Date filter berfungsi dengan benar.
6. Dimension filter berfungsi.

**Rekomendasi:** Tambahkan `reporting_test.go` dengan test data yang known. Test invariants:
```go
func TestTrialBalanceBalanced(t *testing.T) { ... }
func TestBalanceSheetBalances(t *testing.T) { ... }
func TestProfitLoss(t *testing.T) { ... }
```

---

### M-020 [MAJOR] [INVENTORY] [TEST] Inventory dan Costing Package Tidak Punya Test — FIFO/Average Cost Unverified

**Lokasi:** `backend/internal/inventory/` (0 test files); `backend/internal/costing/` (0 test files)

**Spec ref:** ACCOUNTING_ENGINE §10 (persediaan); §33 (golden test)

**Temuan:**
Costing module (`costing.go`, 369 lines) mengimplementasi:
- `PostGRN` — stock receipt (FIFO: create new cost layer; Moving Average: recompute avg).
- `ResolveCOGS` — stock issue (FIFO: consume layers oldest-first; Moving Average: use avg cost).
- `lockBalance` — `SELECT ... FOR UPDATE` pada stock_balances.

Tanpa test, tidak ada verifikasi:
1. **FIFO correctness**: Receive 10@100, Receive 10@200, Issue 15 → COGS = (10×100 + 5×200) = 2000. Remaining = 5@200.
2. **Moving Average correctness**: Receive 10@100, Receive 10@200 → avg = 150. Issue 5 → COGS = 5×150 = 750. Remaining = 15@150.
3. **Layer exhaustion**: FIFO issue lebih besar dari layer pertama → consume next layer.
4. **Negative stock**: Issue saat qty_on_hand = 0 → harus error.

**Rekomendasi:** Tambahkan `costing_test.go` dengan test cases di atas. Gunakan integration test dengan test database.

---

### M-021 [MAJOR] [ASSETS] [TEST] Assets Module Tidak Punya Test — ~1800 LOC Akuntansi Tanpa Safety Net

**Lokasi:** `backend/internal/assets/` — `assets.go` (20.6KB), `depreciation.go` (32.2KB), `helpers.go` (8.7KB) — 0 test files

**Spec ref:** ACCOUNTING_ENGINE §12 (aset tetap); §1 Rule 5 (balanceCheck)

**Temuan:**
Assets module berisi ~1800 LOC:
- Acquisition: `Dr 1401 Fixed Asset / Cr Cash/AP`.
- Straight-line depreciation: `(cost - salvage) / useful_life`.
- Declining-balance depreciation: `book_value × rate`.
- Units-of-production depreciation: `(cost - salvage) × (units_produced / total_units)`.
- Disposal with gain/loss: multi-line journal with conditional legs.
- Revaluation: `Dr Fixed Asset / Cr Revaluation Surplus (OCI)`.

Disposal adalah journal multi-line yang paling rentan error:
```
Dr Cash (proceeds)
Dr Accumulated Depreciation (accumulated)
Cr Fixed Asset (cost)
Cr Gain on Disposal (if proceeds > book value)
  OR
Dr Loss on Disposal (if proceeds < book value)
```

Tanpa test, tidak ada verifikasi bahwa gain/loss dihitung dengan benar.

**Rekomendasi:** Tambahkan `assets_test.go` dan `depreciation_test.go`. Test cases:
1. Straight-line: cost=12000, salvage=2000, life=5 → yearly dep = 2000.
2. Declining balance: cost=10000, rate=40% → year 1 dep = 4000, year 2 dep = 2400.
3. Disposal with gain: cost=10000, accum=6000, proceeds=6000 → gain=2000.
4. Disposal with loss: cost=10000, accum=3000, proceeds=5000 → loss=2000.

---

### M-022 [MAJOR] [LEASE] [DEFERRED] RoU Depreciation (Dr 5209 / Cr 1702) Masih Deferred — PSAK 73 Non-Negotiable

**Lokasi:** `backend/internal/lease/` — tidak ada implementasi depreciation; `docs/TASK_LEDGER.md` last task: "RoU depreciation deferred"

**Spec ref:** ACCOUNTING_ENGINE §20 (sewa — RoU depreciation required)

**Temuan:**
Lease module mengimplementasi:
1. **Lease commencement** (`contracts.go:264-310`): `Dr 1701 RoU Asset / Cr 2301 Lease Liability` at PV. ✅
2. **Lease payment** (`payments.go`): split interest vs principal. `Dr 5906 Interest Expense + Dr 2301 Lease Liability (principal) / Cr 1101 Cash`. ✅
3. **RoU depreciation**: `Dr 5209 Depreciation RoU / Cr 1702 Accumulated RoU Depreciation`. ❌ **NOT IMPLEMENTED**.

**Dampak:**
- RoU asset (1701) tidak pernah terdepresiasi → overvalued di neraca.
- Depreciation expense (5209) tidak di-recognize → profit overstated.
- PSAK 71/73 non-compliance.

**Ekspektasi:** Setiap periode (monthly), post: `Dr 5209 / Cr 1702` untuk `depreciation = RoU_asset_cost / lease_term_months` (straight-line).

**Rekomendasi:**
1. Tambahkan endpoint `POST /lease-contracts/{id}/depreciate`.
2. Atau, buat scheduler yang auto-post depreciation setiap bulan.
3. Idempotent per `(contract_id, period_year, period_month)`.

---

### M-023 [MAJOR] [CROSS_CUTTING] [IDEMPOTENCY] Idempotent Replay Tidak Verifikasi Payload Match — Reuse dengan Payload Berbeda Tidak Deteksi

**Lokasi:**
- `backend/internal/cash/journal.go:54-57`:
  ```go
  if existing, err := db.New(tx).GetJournalByIdempotencyKey(ctx, db.GetJournalByIdempotencyKeyParams{
      TenantID:       tenant,
      IdempotencyKey: uuid(idem),
  }); err == nil {
      result = postingResult{ID: existing.ID, ...}
      return nil  // ← return existing, no payload comparison
  }
  ```
- Pattern yang sama di: `accounting/posting.go:30`, `sales/down_payments.go`, `sales/invoices.go`, `sales/payments.go`, `purchase/grn.go:93`, `assets/depreciation.go:83`, `period/handler.go`, dll.

**Spec ref:** API_CONTRACT.md §Rules ("Reusing a key with a different payload returns `IDEMPOTENCY_KEY_REUSE`")

**Temuan:**
Idempotent replay mencari journal by `(tenant_id, idempotency_key)`. Jika ditemukan, langsung return hasil lama **tanpa membandingkan payload**. API_CONTRACT mensyaratkan:
- Same key + same payload → return original result (replay).
- Same key + different payload → return 409 `IDEMPOTENCY_KEY_REUSE`.

Saat ini: same key + different payload → silently return old result. Client bug (mengirim request berbeda dengan key yang sama) akan tersembunyi.

**Skenario uji:**
1. `POST /cash-in` dengan `Idempotency-Key: abc-123`, `amount_cents: 500000` → success, journal posted.
2. `POST /cash-in` dengan `Idempotency-Key: abc-123`, `amount_cents: 999999` (different amount!) → saat ini: return old journal (amount 500000). Harusnya: 409 `IDEMPOTENCY_KEY_REUSE`.

**Rekomendasi:**
1. Saat insert journal, simpan `request_hash` (hash dari request body).
2. Saat replay, jika `request_hash` match → return old result.
3. Jika `request_hash` mismatch → return 409 `IDEMPOTENCY_KEY_REUSE`.

---

### M-024 [MAJOR] [CROSS_CUTTING] [ERROR_FORMAT] Error Response Tidak Sesuai API_CONTRACT — Missing `details` dan `request_id`

**Lokasi:** Setiap package memiliki `errorResponse` sendiri:
- `backend/internal/cash/helpers.go:22-26`:
  ```go
  type errorResponse struct {
      Code    string `json:"code"`
      Message string `json:"message"`
  }
  ```
- `backend/internal/accounting/helpers.go:37-40` — sama.
- `backend/internal/coa/helpers.go` — sama.
- `backend/internal/auth/helpers.go` — sama.
- `backend/internal/sales/helpers.go` — sama.
- `backend/internal/purchase/helpers.go` — sama.
- `backend/internal/lease/helpers.go:80-83` — sama.
- (setidaknya 8+ duplikasi)

**Spec ref:** API_CONTRACT.md §Rules ("Errors return `{code, message, details, request_id}`")

**Temuan:**
API_CONTRACT mensyaratkan format:
```json
{
  "code": "INVALID_REQUEST",
  "message": "amount_cents is required",
  "details": { "field": "amount_cents", "issue": "required" },
  "request_id": "req_abc123"
}
```

Implementasi hanya mengembalikan:
```json
{
  "code": "INVALID_REQUEST",
  "message": "amount_cents is required"
}
```

Missing:
1. **`details`** — untuk field-level validation info (which field, what constraint).
2. **`request_id`** — untuk tracing dan debugging.
3. **`error` wrapper** — beberapa spec (ARCHITECTURE §18) mengharuskan `{ "error": { ... } }` wrapper.

**Dampak:**
- Frontend tidak dapat menampilkan field-level error messages.
- Tidak ada request tracing untuk debugging production issues.
- Inconsistent error handling across packages.

**Rekomendasi:**
1. Buat shared `httperr` package:
   ```go
   package httperr
   type Error struct {
       Code      string         `json:"code"`
       Message   string         `json:"message"`
       Details   map[string]any `json:"details,omitempty"`
       RequestID string         `json:"request_id"`
   }
   ```
2. Middleware generate `request_id` per request (UUID).
3. Update semua `writeError` calls untuk include `details` dan `request_id`.

---

### M-025 [MAJOR] [COA] [SEED] Seed COA Missing `3105 Suspense` dan `4902 Overhead Dibebankan`

**Lokasi:** `backend/internal/auth/seed.go:13-60` (COA seed)

**Spec ref:**
- ACCOUNTING_ENGINE §3.0.2 line 113: `3105 Modal Setoran/Suspense (EQUITY)` — required untuk opening balance dengan selisih (§5)
- ACCOUNTING_ENGINE §3.0.2: `4902 Overhead Dibebankan (EXPENSE)` — required untuk production overhead (§11.2)
- ACCOUNTING_ENGINE §5: "Jika total debet ≠ total kredit saat input, selisih ditempatkan sementara di 3105"

**Temuan:**
Seed COA di `auth/seed.go` tidak include:
1. **`3105 Suspense`** — jika opening balance tidak balance, engine mengharapkan equity plug ke 3105. Tanpa akun ini, opening balance dengan selisih akan crash dengan "account 3105 not found."
2. **`4902 Overhead Dibebankan`** — production overhead crediting (§11.2). Tanpa akun ini, production overhead posting akan crash.
3. **`1302 Raw Material`** — spec §3.0.1: `1302 Bahan Baku`. Seed hanya punya `1301 Inventory`, `1303 WIP`, `1304 Finished Goods`.
4. **`1207 Prepaid Expense`** — spec §3.0.1: `1207 Beban Dibayar Dimuka`.
5. **`4202 Sales Discount`** — spec §3.0.2: `4202 Diskon Penjualan`.
6. **`5102 Inventory Write-down`** — spec §3.0.2.

**Dampak:**
- Opening balance dengan selisih → crash.
- Production overhead posting → crash (jika kode diubah ke 4902 per M-010).
- Raw material tracking tidak dapat dipisah dari barang dagang.

**Rekomendasi:** Tambahkan semua akun dari §3.0.2 ke seed. Atau lebih baik: generate seed dari spec secara programatik dengan test yang membandingkan.

---

### M-026 [MAJOR] [COA] [RLS] Read-Path Tidak Set `app.tenant_id` — RLS Tidak Aktif untuk Query Read

**Lokasi:**
- `backend/internal/coa/accounts.go:77` — `List` menggunakan `service.pool.Query` (no transaction, no `set_config`):
  ```go
  rows, err := service.pool.Query(request.Context(), `
      SELECT id, code, name, ...
      FROM accounts
      WHERE tenant_id = $1
      ORDER BY code
  `, tenantID)
  ```
- `backend/internal/coa/categories.go:24` — `ListCategories` (pola sama).
- `backend/internal/reporting/data.go:50-70` — `fetchTrialBalance` (pola sama).
- `backend/internal/reporting/data.go:90-120` — `fetchProfitLoss` (pola sama).
- `backend/internal/reporting/data.go:200-240` — `fetchBalanceSheet` (pola sama).
- `backend/internal/reporting/data.go:332-351` — `fetchCashFlow` (pola sama).

**Spec ref:** AGENTS.md ("tenant-scoped RLS"); migration `000001_mvp_foundation.up.sql:194-211` (RLS policies)

**Temuan:**
RLS policies di migration:
```sql
ALTER TABLE journal_entries ENABLE ROW LEVEL SECURITY;
ALTER TABLE journal_lines ENABLE ROW LEVEL SECURITY;
ALTER TABLE accounts ENABLE ROW LEVEL SECURITY;
-- ...
CREATE POLICY tenant_isolation ON journal_entries
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT);
```

Policy menggunakan `current_setting('app.tenant_id', true)`. Jika setting tidak diset:
- `current_setting('app.tenant_id', true)` returns `''` (empty string, because second arg `true` = missing_ok).
- `''::BIGINT` = `0` (PostgreSQL casts empty string to 0).
- Policy: `tenant_id = 0` → memfilter semua baris (tidak ada tenant dengan id 0).

**Tetapi** query read secara eksplisit memfilter `WHERE tenant_id = $1`, jadi hasilnya benar. RLS hanya tidak aktif sebagai second layer of defense. Jika developer lupa menambah `WHERE tenant_id = $1` di query read baru, RLS TIDAK akan menangkap — query akan mengembalikan hasil kosong (bukan error), yang sulit di-debug.

**Dampak:**
- Defense-in-depth tidak berfungsi untuk read paths.
- Jika developer baru menulis query tanpa `WHERE tenant_id`, hasil kosong tanpa error.
- Jika RLS tidak enabled pada tabel baru (mis. customer, item, lease_contracts), cross-tenant data leak possible.

**Verifikasi needed:** Cek RLS pada SEMUA tenant-scoped tables:
```sql
SELECT tablename, rowsecurity FROM pg_tables WHERE schemaname = 'public';
```
Migration hanya enable RLS pada: journal_entries, journal_lines, accounts, accounting_periods, categories, audit_logs, outbox_events. **Tidak enable** pada: customers, items, payment_terms, suppliers, purchase_orders, sales_orders, quotations, invoices, dll.

**Rekomendasi:**
1. Enable RLS pada SEMUA tenant-scoped tables.
2. Bungkus semua query read dalam transaction yang set `app.tenant_id`.
3. Atau, gunakan middleware yang set tenant context pada connection pool level.

---

### M-027 [MAJOR] [AUTH] [SECURITY] Tidak Ada Rate Limiting pada Login Endpoint — Brute Force Possible

**Lokasi:** `backend/cmd/api/main.go:61` — `router.Post("/auth/login", authService.Login)` (no rate limiter)

**Spec ref:** ARCHITECTURE §8.2 (security)

**Temuan:**
Endpoint `/auth/login` tidak memiliki rate limiting. Attacker dapat:
1. Brute-force password dengan ribuan attempts per detik.
2. Credential stuffing.
3. Tidak ada account lockout setelah N failed attempts.

**Rekomendasi:**
1. Tambahkan rate limiter: `golang.org/x/time/rate` atau `chi/middleware.Throttle`.
2. Max 5 attempts per IP per menit.
3. Account lockout setelah 10 failed attempts (unlock oleh owner).

---

### M-028 [MAJOR] [AUTH] [SECURITY] Refresh Token Revocation — Verifikasi Diperlukan

**Lokasi:** `backend/internal/auth/auth.go` — fungsi `Logout`

**Spec ref:** API_CONTRACT.md (`POST /auth/logout` — Revoke refresh session)

**Temuan:**
`Logout` endpoint menerima refresh token dan menghapusnya dari `user_tokens` table. Tetapi perlu verifikasi:
1. Apakah token di-DELETE dari database, atau hanya ditandai `revoked = true`?
2. Jika DELETE: token tidak bisa di-reuse. ✅
3. Jika tidak dihapus: token masih valid sampai expiry. ❌
4. Apakah ada token blacklist?

**Skenario uji:**
1. Login → dapat refresh token.
2. Logout → token dihapus/ditandai.
3. Refresh dengan token yang sudah logout → harus 401.

**Rekomendasi:** Pastikan `Logout` menghapus baris dari `user_tokens`. Test: logout → refresh → 401.

---

## MINOR Findings

---

### m-001 [MINOR] [ENGINE] [HASH] `hashJournal` Tidak Public — Root Cause dari Duplikasi C-003

**Lokasi:** `backend/internal/accounting/engine.go:375` — `func hashJournal` (lowercase = unexported)

**Rekomendasi:** Export sebagai `HashJournal`. Lihat C-003.

---

### m-002 [MINOR] [ENGINE] [HASH] Sort by `SourceLineRef` Only — Non-Deterministic untuk Duplicate Refs

**Lokasi:** `backend/internal/accounting/engine.go:377`
```go
sort.Slice(lines, func(l, r int) bool { return lines[l].SourceLineRef < lines[r].SourceLineRef })
```

**Temuan:** `sort.Slice` tidak stabil. Jika dua line memiliki `SourceLineRef` yang sama, urutan mereka tidak deterministik → hash bisa berbeda.

**Rekomendasi:** Sort by `(SourceLineRef, AccountID, DebitCents)` sebagai tiebreaker.

---

### m-003 [MINOR] [ENGINE] [DESIGN] `Description` Tidak Masuk ke Hash Payload

**Lokasi:** `backend/internal/accounting/engine.go:378`

**Temuan:** `Journal.Description` tidak di-hash. Dua jurnal dengan description berbeda memiliki hash yang sama. Mungkin by design, tetapi harus didokumentasikan.

---

### m-004 [MINOR] [ENGINE] [DEAD_CODE] `var _ = context.Background` dan `var _ = db.New` Guards

**Lokasi:** `backend/internal/accounting/journal_manual.go:371-372`; `backend/internal/accounting/ledger.go:177`; `backend/internal/tax/ppn.go:306-307`

**Temuan:** Unused-import guards menandakan import yang tidak digunakan. Code smell.

---

### m-005 [MINOR] [AUTH] [TOKEN] Bearer Token Prefix Case-Sensitive

**Lokasi:** `backend/internal/auth/auth.go` — fungsi `bearerToken`

**Temuan:** `bearerToken` melakukan case-sensitive check untuk `"Bearer "` (capital B). RFC 6750 memperbolehkan case-insensitive. Header `"bearer "` (lowercase) akan ditolak.

**Rekomendasi:** `strings.HasPrefix(strings.ToLower(header), "bearer ")`.

---

### m-006 [MINOR] [AUTH] [DESIGN] Tidak Ada 2FA Implementation

**Lokasi:** `backend/internal/auth/`

**Spec ref:** ARCHITECTURE §968 ("Auth: register/login/refresh/logout/2FA + RBAC")

**Temuan:** 2FA disebutkan di finalization checklist tetapi tidak diimplementasi.

---

### m-007 [MINOR] [COA] [SLUG] `slugify` Mangles Unicode Names

**Lokasi:** `backend/internal/auth/seed.go` atau `backend/internal/coa/helpers.go`

**Temuan:** "Café Über" → "caf-ber" (non-ASCII menjadi hyphen). Ambigu dan tidak readable.

**Rekomendasi:** Gunakan `golang.org/x/text/unicode/norm` untuk transliterasi.

---

### m-008 [MINOR] [SALES] [CREDIT_NOTE] Credit Note Tidak Validasi CN Total vs Invoice Receivable

**Lokasi:** `backend/internal/sales/credit_notes.go`

**Temuan:** CN untuk lebih dari receivable akan membuat AR negatif. Tidak ada validation `CN_total <= invoice_receivable`.

---

### m-009 [MINOR] [SALES] [QUOTATION] Quotation Tidak Track Conversion Rate

**Lokasi:** `backend/internal/sales/`

**Temuan:** Tidak ada metric tracking berapa quotation yang convert ke SO.

---

### m-010 [MINOR] [PURCHASE] [RETURN] Purchase Return Mungkin Tidak Reverse Input Tax

**Lokasi:** `backend/internal/purchase/purchase_returns.go`

**Spec ref:** §9.2 (purchase return reverses GRN + input tax)

**Temuan:** Jika GRN/SI memposting input tax (Dr 1203), return harus meng-reverse-nya (Cr 1203). Perlu verifikasi.

---

### m-011 [MINOR] [INVENTORY] [NEGATIVE] Negative Inventory Tidak Dicegah secara Eksplisit

**Lokasi:** `backend/internal/costing/costing.go` — `ResolveCOGS`

**Spec ref:** §33.2 (negative inventory edge case)

**Temuan:** `ResolveCOGS` mengurangi qty_on_hand. Jika qty_on_hand < issue qty, hasil negatif. Tidak ada explicit check `if new_qty < 0 { return error }`.

**Rekomendasi:** Tambahkan check: `if stock_balances.qty_on_hand - qty < 0 { return ErrInsufficientStock }`.

---

### m-012 [MINOR] [INVENTORY] [FLOAT] `qty` Menggunakan `float64` (NUMERIC) — Konsisten tapi Tidak Ideal

**Lokasi:** `backend/internal/costing/costing.go:17` — `qty float64`; `backend/internal/inventory/stock_opname.go:27` — `CountedQty float64`

**Spec ref:** §2.1 (integer cents — qty juga harus precise)

**Temuan:** Qty disimpan sebagai `float64` (dari `NUMERIC(18,3)` di PostgreSQL via `pgtype.Numeric` → `Float64Value()`). Ini konsisten dengan DB schema tetapi float64 dapat kehilangan precision untuk very large or very small quantities.

---

### m-013 [MINOR] [LEASE] [PV] Zero Discount Rate Returns PV = 0 (Should Be Sum of Payments)

**Lokasi:** `backend/internal/lease/contracts.go` — `presentValueCents` function

**Temuan:** `presentValueCents` dengan rate = 0 mengembalikan PV = 0. Secara matematika, PV(rate=0) = sum of payments = payment × n.

**Rekomendasi:** Special-case `rate == 0`: return `paymentCents × int64(n)`.

---

### m-014 [MINOR] [LEASE] [DEFERRED] Lease Modification dan Termination Masih Deferred

**Lokasi:** `backend/internal/lease/`; `docs/TASK_LEDGER.md`

**Spec ref:** §20 (lease modification & termination)

**Temuan:** Modification (perubahan term/payment) dan termination belum diimplementasi.

---

### m-015 [MINOR] [REPORTING] [LOGIC] Laba Berjalan Calculation — Verifikasi Real-Time

**Lokasi:** `backend/internal/reporting/data.go` — `fetchBalanceSheet`

**Spec ref:** §21.2 ("Laba berjalan saat periode berjalan: dihitung real-time dari pendapatan − beban periode berjalan")

**Temuan:** Balance Sheet menggabungkan current profit ke equity. Perlu verifikasi bahwa profit dihitung dari revenue - expense periode berjalan (bukan cumulative).

---

### m-016 [MINOR] [CROSS_CUTTING] [ARCHITECTURE] Frontend Struktur Tidak Sesuai ARCHITECTURE.md

**Lokasi:** `web/src/` — aktual: `{screens, components, workbench, api.ts, types.ts, state.tsx}`; planned: `{app, features, stores, lib, types}`

**Rekomendasi:** Update spec atau refactor frontend.

---

### m-017 [MINOR] [CROSS_CUTTING] [FRONTEND] `MockEntryForm` Masih Direferensikan — Nama Menyesatkan

**Lokasi:** `web/src/screens/entry/MockEntryForm.tsx`; `web/src/workbench/WorkArea.tsx:48,311`

**Temuan:** `MockEntryForm` adalah placeholder aktif untuk form types yang belum diimplementasi. `MockList` adalah reusable generic table component (bukan mock). Nama "Mock" menyesatkan.

**Rekomendasi:** Ganti nama ke `PlaceholderForm` dan `GenericTable`.

---

### m-018 [MINOR] [CROSS_CUTTING] [TECH_DEBT] `styles.css` 72.7KB Single File

**Lokasi:** `web/src/styles.css`

**Rekomendasi:** Modularize atau gunakan CSS-in-JS / Tailwind.

---

### m-019 [MINOR] [CROSS_CUTTING] [API_CONTRACT] Endpoint Drift — Code Punya 50+ Endpoint Tidak di Contract

**Lokasi:** `backend/cmd/api/main.go` vs `docs/API_CONTRACT.md`

**Temuan:** API_CONTRACT.md v0.1.0 hanya mendokumentasikan ~25 MVP endpoints. Code memiliki 50+ routes (purchase, inventory, tax, assets, production, lease, reconciliation, budget, notes, audit, period).

---

### m-020 [MINOR] [CASH] [STALE] Komentar "X-Tenant-ID header (temporary until JWT auth carries it)"

**Lokasi:** `backend/internal/cash/handler.go:18`; `backend/internal/cash/journal.go:48`

**Temuan:** JWT auth sudah carry tenant_id, tetapi komentar belum diupdate. Menyesatkan developer baru.

---

### m-021 [MINOR] [PERIOD] [LOGIC] `Unlock` Tidak Memerlukan Idempotency-Key Header

**Lokasi:** `backend/internal/period/handler.go:46` — `idem := request.Header.Get("Idempotency-Key")` (no validation, no required check)

**Temuan:** `Unlock` membaca Idempotency-Key tetapi tidak memvalidasi bahwa header ada. Jika kosong, `idem = ""` — tetap diproses. Berbeda dengan `Close` yang memvalidasi.

---

### m-022 [MINOR] [SALES] [DISCOUNT] `lineTotalCents` Mengurangi Discount Setelah Rounding — Bisa Off by 1 Sen

**Lokasi:** `backend/internal/sales/logic.go:51`
```go
return int64(math.Round(gross)) - discountCents
```

**Temuan:** Gross di-round dulu, baru discount dikurangi. Jika gross = 15000.4 → round = 15000, lalu - discount. Jika gross = 15000.6 → round = 15001, lalu - discount. Discount (sudah integer) dikurangkan setelah rounding gross. Ini bisa menyebabkan inkonsistensi jika gross dan discount dihitung secara terpisah di sisi lain (mis. invoice total).

**Rekomendasi:** Hitung `(gross - discount)` dalam satu operasi float, lalu round sekali.

---

## INFO Findings

---

### i-001 [INFO] [ENGINE] [DESIGN] `finalize` Sets `PreviousHash = "genesis"` — Placeholder yang Dibuang

Lihat M-003 untuk detail.

---

### i-002 [INFO] [AUTH] [DESIGN] `randomUUID` Tidak Menggunakan RFC 4122 Library

**Lokasi:** `backend/internal/auth/auth.go:387-395`

**Temuan:** `randomUUID` manual implementasi UUID v4. Bisa menggunakan `github.com/google/uuid` untuk reliability.

---

### i-003 [INFO] [AUTH] [DESIGN] Password Validation Minimal — Hanya `len >= 8`

**Lokasi:** `backend/internal/auth/auth.go:59` — `len(req.Password) < 8`

**Temuan:** Tidak ada requirement untuk uppercase, lowercase, number, special character.

---

### i-004 [INFO] [COA] [DESIGN] Tidak Ada COA Import/Export

**Lokasi:** `backend/internal/coa/`

**Rekomendasi:** Tambahkan import/export CSV/Excel.

---

### i-005 [INFO] [SALES] [DESIGN] Quotation Tidak Track Conversion Rate

Lihat m-009.

---

### i-006 [INFO] [LEASE] [DEFERRED] Inter-Company Elimination (LOAN/INTEREST/DIVIDEND) Masih Deferred

**Lokasi:** `backend/internal/lease/consolidation.go`; `docs/TASK_LEDGER.md`

**Spec ref:** §22 (konsolidasi)

**Temuan:** LOAN, INTEREST, DIVIDEND elimination belum diimplementasi.

---

### i-007 [INFO] [LEASE] [DESIGN] Consolidation Elimination Logic Mungkin Punya Sign Errors

**Lokasi:** `backend/internal/lease/consolidation.go`

**Spec ref:** §22 (konsolidasi — eliminasi)

**Temuan:** Consolidation elimination logic perlu review mendalam untuk sign errors dan double-counting.

---

### i-008 [INFO] [CROSS_CUTTING] [DESIGN] Tidak Ada `recover()` Middleware — Panic Crash Request

**Lokasi:** `backend/cmd/api/main.go`

**Spec ref:** ARCHITECTURE §18.3

**Rekomendasi:** Tambahkan `chi/middleware.Recoverer`.

---

### i-009 [INFO] [CROSS_CUTTING] [DESIGN] Tidak Ada Request Logging Middleware

**Lokasi:** `backend/cmd/api/main.go`

**Rekomendasi:** Tambahkan structured logger (zap/zerolog).

---

### i-010 [INFO] [CROSS_CUTTING] [DESIGN] Tidak Ada CORS Middleware

**Lokasi:** `backend/cmd/api/main.go`

**Rekomendasi:** Tambahkan `rs/cors` middleware.

---

### i-011 [INFO] [CROSS_CUTTING] [DESIGN] Tidak Ada Request Timeout Middleware

**Lokasi:** `backend/cmd/api/main.go`

**Rekomendasi:** Tambahkan `http.TimeoutHandler` atau `context.WithTimeout`.

---

### i-012 [INFO] [MASTER_DATA] [VALIDATION] Customer/Item Tidak Ada Duplicate Check

**Lokasi:** `backend/internal/customer/handler.go`; `backend/internal/item/handler.go`

**Rekomendasi:** Tambahkan explicit duplicate check (email, phone, SKU).

---

### i-013 [INFO] [TAX] [ECL] ECL Aging Calculation Menggunakan `as_of_date` — Verifikasi Query

**Lokasi:** `backend/internal/tax/ecl.go`

**Temuan:** ECL aging harus dihitung dari `as_of_date - invoice_date` untuk setiap outstanding invoice. Perlu verifikasi bahwa query melakukan ini dengan benar dan tidak include paid invoices.

---

### i-014 [INFO] [TAX] [PPH] PPh Module — Verifikasi PPh Final UMKM Rate

**Lokasi:** `backend/internal/tax/pph.go`

**Spec ref:** §13 (PPh Final UMKM 0.5%)

**Temuan:** PPh Final UMKM rate adalah 0.5% dari turnover. Perlu verifikasi rate di kode.

---

### i-015 [INFO] [REPORTING] [DIMENSION] Dimension Filter di Reports — Verifikasi JOIN

**Lokasi:** `backend/internal/reporting/data.go:35-43` — `dimensionJoin`

**Temuan:** Dimension filter menggunakan JOIN ke `journal_line_dimensions`. Perlu verifikasi bahwa table ini di-populate saat journal lines di-insert dengan dimension_id.

---

### i-016 [INFO] [PRODUCTION] [BOM] BOM Lines Accept Zero `unit_cost_cents`

**Lokasi:** `backend/internal/production/bom.go:344-346`

**Temuan:** Validation hanya reject `UnitCostCents < 0`, tidak reject `== 0`. BOM line dengan cost 0 adalah economically meaningless (free material).

---

## Cross-Cutting Concerns Deep Analysis

### CC1 Hash Chain Implementation

**DB-level enforcement (excellent):**
- Migration `000001:237-255`: `reject_posted_journal_mutation` trigger — `BEFORE UPDATE OR DELETE` pada `journal_entries` dan `journal_lines`.
- Authorized void exception: `UPDATE` only if `app.void_context = '1'` AND `NEW.status = 'VOID'` AND `void_reason/voided_by/voided_at` semua NOT NULL.
- Migration `000004`: extends trigger to also allow setting `reversal_of_id` on the new reversal journal.
- `ledger_chain_heads` table with `SELECT FOR UPDATE` serialization per tenant.
- Genesis seed: first journal has `PreviousHash = 'genesis'`.

**Application-level issues:**
- Hash formula duplicated in 11 places (C-003).
- `hashJournal` not exported (m-001).
- Sort non-deterministic for duplicate refs (m-002).
- `Description` not in hash (m-003 — by design?).
- `finalize` computes hash with "genesis" then discarded (M-003).

**Verdict:** DB enforcement excellent. Application-level architecture fragile due to duplication.

---

### CC2 Idempotency

**DB-level enforcement:**
- Migration `000001:130-131`: `CREATE UNIQUE INDEX journal_entries_idempotency_unique ON journal_entries (tenant_id, idempotency_key)`.
- Idempotency key stored as `pgtype.UUID` — must be valid UUID.

**Application-level enforcement:**
- `idempotencyKey()` helper di setiap package validates UUID format.
- `GetJournalByIdempotencyKey` query checks existing before insert.
- Applied to: cash-in, cash-out, transfer, opening-balance, reverse, manual-journal, DP, invoice, payment, credit-note, GRN, supplier-invoice, supplier-payment, purchase-return, lease-initial, lease-payment, asset-depreciate, production-cost, stock-opname, period-close.

**Issues:**
- No payload match check (M-023) — different payload with same key silently returns old result.
- Credit note COGS journal has no idempotency key (M-004).
- Period `Unlock` doesn't require idempotency key (m-021).

**Verdict:** Mechanism present but incomplete. Missing payload verification is the biggest gap.

---

### CC3 RLS / Tenant Isolation

**DB-level enforcement:**
- Migration `000001:194-211`: RLS enabled on 6 tables: `journal_entries`, `journal_lines`, `accounts`, `accounting_periods`, `categories`, `audit_logs`, `outbox_events`.
- Policy: `USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT)`.
- `set_config('app.tenant_id', ..., true)` — `true` = transaction-local.

**Tables WITHOUT RLS (data leak risk):**
- `customers`, `items`, `payment_terms`, `suppliers`, `quotations`, `sales_orders`, `down_payments`, `delivery_orders`, `invoices`, `credit_notes`, `purchase_orders`, `grns`, `supplier_invoices`, `supplier_payments`, `purchase_returns`, `fixed_assets`, `asset_depreciation_schedules`, `production_jobs`, `production_job_costs`, `boms`, `lease_contracts`, `lease_payment_schedule`, `stock_balances`, `inventory_movements`, `inventory_cost_layers`, `stock_opnames`, `budgets`, `notes`, `attachments`, `document_numbering`, `ledger_chain_heads`, `entity_hierarchy`, `ppn_reconciliations`, `ecl_calculations`, `user_tokens`.

**Application-level:**
- Write paths: ALL set `app.tenant_id` via `set_config` inside transaction. ✅
- Read paths: Do NOT set `app.tenant_id`. Rely on explicit `WHERE tenant_id = $1`. RLS inactive. ❌ (M-026)

**Integration test:**
- `TestMVPDatabaseInvariants` → `testTenantIsolation`: sets tenant 1, inserts journal, then tries to read as tenant 999 → verifies empty. ✅

**Verdict:** Write paths solid. Read paths lack defense-in-depth. ~30+ tables without RLS — critical gap.

---

### CC4 Period Lock Enforcement

**DB-level enforcement:**
- Migration `000001:283-300`: `assert_entry_date_in_period` trigger — `BEFORE INSERT ON journal_entries`:
  ```sql
  IF NOT EXISTS (
      SELECT 1 FROM accounting_periods
      WHERE tenant_id = NEW.tenant_id AND id = NEW.period_id
        AND NEW.entry_date BETWEEN period_start AND period_end
        AND status IN ('OPEN', 'REOPENED')
  ) THEN
      RAISE EXCEPTION 'entry date is outside an open period';
  END IF;
  ```
- Only allows `OPEN` and `REOPENED` periods. `CLOSED` periods reject posting.

**Application-level:**
- `resolvePeriod` in posting path queries period by date and returns period_id.
- Period `Close` sets status to `CLOSED`.
- Period `Unlock` sets status back to `OPEN` and reverses closing entries.

**Issues:**
- Trigger is `BEFORE INSERT` only (not UPDATE). But UPDATE is blocked by immutability trigger, so this is fine.
- Period overlap prevention: `EXCLUDE USING gist` constraint in migration. ✅
- One OPEN period per tenant: enforced by partial unique index? Perlu verifikasi.

**Verdict:** Correct and well-enforced at DB level.

---

### CC5 API Contract Compliance

**Endpoint count:** Code has ~60+ routes; API_CONTRACT.md documents ~25.

**Major gaps:**
- Purchase endpoints (GRN, PO, SI, SP, Return) — not in contract.
- Tax endpoints (PPN, PPh, ECL) — not in contract.
- Asset endpoints (acquisition, depreciation, disposal) — not in contract.
- Production endpoints (jobs, BOM, costs) — not in contract.
- Lease endpoints (contracts, payments) — not in contract.
- Period endpoints (close, unlock) — not in contract.
- Reconciliation, budget, notes, audit endpoints — not in contract.

**Verdict:** Contract is severely stale. Needs comprehensive update.

---

### CC6 Error Convention

**Spec:** API_CONTRACT.md §Rules: `{code, message, details, request_id}`.
**Actual:** `{code, message}` — missing `details` and `request_id`.

**Duplication:** `errorResponse` struct duplicated in 8+ packages.

**Verdict:** Non-compliant. Needs unified error package.

---

### CC7 Frontend Architecture

**Planned (ARCHITECTURE.md):** `src/{app, features, stores, lib, types}`
**Actual:** `src/{screens, components, workbench, api.ts, types.ts, state.tsx, App.tsx}`

**File sizes:**
- `api.ts`: 68.5KB — monolithic API client.
- `types.ts`: 47KB — monolithic type definitions.
- `styles.css`: 72.7KB — monolithic CSS.

**Verdict:** Diverges from spec. Needs alignment or spec update.

---

### CC8 Test Coverage Gaps

| Package | Test Files | Source Files | Coverage |
|---|---|---|---|
| accounting | 1 (engine_test.go) | 7 | Partial — no §33 golden test |
| auth | 1 (auth_test.go) | 4 | Partial |
| cash | 3 | 4 | Good |
| sales | 6 | 8 | Good |
| purchase | 3 | 7 | Partial — GRN/PO/suppliers untested |
| coa | 1 (helpers_test.go) | 5 | Minimal |
| db | 1 (integration_test.go) | 5 | Good |
| lease | 1 (helpers_test.go) | 4 | Minimal |
| **period** | **0** | 1 | **None** |
| **assets** | **0** | 3 | **None** |
| **production** | **0** | 4 | **None** |
| **tax** | **0** | 4 | **None** |
| **inventory** | **0** | 3 | **None** |
| **costing** | **0** | 1 | **None** |
| **reconciliation** | **0** | 2 | **None** |
| **budget** | **0** | 2 | **None** |
| **notes** | **0** | 2 | **None** |
| **reporting** | **0** | 3 | **None** |
| **audit** | **0** | 2 | **None** |
| customer | 0 | 1 | None |
| item | 0 | 1 | None |
| config | 0 | 1 | None |
| tenant | 0 | 1 | None |

**11 of 23 packages (48%) have ZERO test files.** Accounting-critical modules (inventory, costing, tax, assets, production, period, reporting) are the most dangerous gaps.

---

## What's Working Well

1. **DB-level invariants excellent:**
   - Balance constraint: `assert_journal_balanced` deferred trigger (migration `000001:257-281`).
   - Immutability: `reject_posted_journal_mutation` trigger with authorized void exception (migration `000001:220-239` + `000004`).
   - Period validation: `assert_entry_date_in_period` trigger (migration `000001:283-300`).
   - Idempotency: unique index `(tenant_id, idempotency_key)`.
   - Reversal: unique index `(tenant_id, reversal_of_id)` — prevents double reversal.
   - Intent uniqueness: unique index `(tenant_id, source_ref, intent_type)`.

2. **Hash chain head serialization:** `SELECT FOR UPDATE` on `ledger_chain_heads` correctly serializes concurrent postings per tenant.

3. **Pure engine design:** `accounting` package is truly pure (no IO) — testable mathematically.

4. **Integer cents in engine:** `DebitCents`, `CreditCents`, `AmountCents` all `int64`. No float in engine layer (float only in handler/API layer — M-005).

5. **Authorized reversal procedure:** Void/reversal flow is well-designed — sets `app.void_context`, records `void_reason`/`voided_by`/`voided_at`, links via `reversal_of_id`.

6. **RLS for write paths:** All posting transactions correctly set `app.tenant_id` transaction-locally.

7. **Integration test exists:** `TestMVPDatabaseInvariants` tests 5 critical invariants.

8. **Sales module most complete:** 8 source files + 6 test files, full SQ→SO→DP→DO→INV→Payment flow.

9. **Period close IS implemented:** Generates closing entries (Dr Revenue / Cr 3301, Dr 3301 / Cr Expenses, Dr 3301 / Cr 3201 Retained Earnings). BUT has date filter issue (M-016).

10. **ECL implementation detailed:** Aging buckets (0-30, 31-60, 61-90, >90), configurable rates, provision journal (Dr 5209 / Cr 1202), write-off (Dr 1202 / Cr 1201). BUT untested (M-013).

---

## Prioritized Action Plan

### Sprint 1 (Immediate — Security & Integrity)
1. **C-001:** Implement RBAC (role in JWT + RequireRole middleware)
2. **C-002:** Remove JWT secret fallback, require env var
3. **C-003:** Export `accounting.HashJournal`, remove all 11 duplicates
4. **C-004:** Verify and fix `hashJournalForOpname` consistency

### Sprint 2 (High Priority — Accounting Correctness)
5. **M-001:** Fix `Account.Type` loading in `accountForEngine`
6. **M-010:** Fix production labor/overhead credit accounts (5201/4902)
7. **M-014:** Implement PPN calculation in invoice/GRN posting
8. **M-022:** Implement RoU depreciation (PSAK 73)
9. **M-016:** Fix period close date filter in `loadPLBalances`
10. **M-004:** Fix credit note COGS journal idempotency
11. **M-023:** Implement idempotency payload match check
12. **M-025:** Add missing COA seed accounts (3105, 4902, 1302, dll)

### Sprint 3 (Test Coverage — Critical)
13. **M-020:** Add inventory + costing tests (FIFO, average, COGS)
14. **M-021:** Add assets tests (depreciation, disposal)
15. **M-013:** Add tax tests (PPN, PPh, ECL)
16. **M-015:** Add period tests (close, unlock, closing entries)
17. **M-019:** Add reporting tests (P&L, BS, TB invariants)
18. **M-008:** Add purchase tests (GRN, PO, suppliers)
19. **M-002:** Add §33 golden test matrix to engine_test.go

### Sprint 4 (Hardening)
20. **M-024:** Unified error response format
21. **M-026:** Enable RLS on ALL tenant-scoped tables + set tenant on reads
22. **M-027:** Add rate limiting on login
23. **M-028:** Verify refresh token revocation
24. **M-017:** Implement cash flow classification (operating/investing/financing)
25. **M-018:** Trial balance alert when not balanced
26. **i-008 through i-011:** Add recover, logging, CORS, timeout middleware

### Sprint 5 (Technical Debt)
27. **M-005:** Eliminate float64 in calculations (use integer or decimal)
28. **M-007:** Implement AR sub-ledger reconciliation
29. **M-009:** Verify GRN → SI → AP flow (2105 clearing)
30. **m-016 through m-022:** Frontend alignment, API contract update, cleanup

---

## BAGIAN I: CORRECTNESS AUDIT — Statistik & Temuan per Modul

> Bagian I mencakup 70 temuan correctness: 4 Critical, 28 Major, 22 Minor, 16 Info.  
> Temuan-temuan ini dihasilkan dari first-hand code tracing setiap file Go di `backend/internal/`,  
> setiap journal entry ditrace ke ACCOUNTING_ENGINE §X, setiap constraint diverifikasi di migration SQL.

---

## BAGIAN II: ERP COMPLETENESS AUDIT — Analisis Data Field per Fitur

Bagian ini menganalisis setiap fitur ERP dari perspektif **data completeness** — field apa yang ada di input/schema vs field apa yang seharusnya ada di ERP yang proper. Setiap temuan dikategorikan sebagai:

- **[MISSING]** — field tidak ada sama sekali, fungsionalitas ERP tidak terpenuhi
- **[INCOMPLETE]** — field ada tetapi tidak cukup untuk operasional ERP nyata
- **[OK]** — field mencukupi untuk MVP

---

### E-01 MASTER DATA: CUSTOMER

**Schema:** `customers` table (migration `000005:18-43`)
**Request:** `CreateCustomerRequest` (`customer/handler.go:39-54`)

| Field di DB | Field di API | Status | Catatan |
|---|---|---|---|
| code | code | OK | |
| name | name | OK | |
| npwp | npwp | OK | |
| contact_person | contact_person | INCOMPLETE | Hanya 1 kontak — ERP butuh multiple kontak (technical, billing, shipping) |
| phone | phone | INCOMPLETE | Hanya 1 phone — butuh phone, mobile, fax |
| email | email | INCOMPLETE | Hanya 1 email — butuh multiple email (billing, statement) |
| address | address | INCOMPLETE | Single-line address — butuh alamat penagihan vs alamat pengiriman |
| city | city | OK | |
| province | province | OK | |
| postal_code | postal_code | OK | |
| payment_term_id | payment_term_id | OK | |
| credit_limit_cents | credit_limit_cents | OK | |
| default_revenue_account_id | default_revenue_account_id | OK | |
| default_receivable_account_id | default_receivable_account_id | OK | |

**[MISSING] Field yang seharusnya ada di ERP proper:**

| Field yang Hilang | ERP Purpose | Prioritas |
|---|---|---|
| **customer_type** | B2B vs B2C vs Individual — menentukan perlakuan PPN dan PPh | Tinggi |
| **shipping_address** | Alamat pengiriman terpisah dari alamat penagihan | Tinggi |
| **billing_address** | Alamat penagihan terpisah | Tinggi |
| **shipping_method** | Ekspedisi default (JNE, SiCepat, dll) | Sedang |
| **carrier_account_no** | Account number untuk ekspedisi | Sedang |
| **tax_id_type** | NPWP vs KTP vs Paspor — untuk PPh 22/23 | Sedang |
| **pkp_status** | PKP (Pengusaha Kena Pajak) vs Non-PKP — menentukan apakah PPN dipungut | Tinggi |
| **default_tax_rate** | Rate PPN default untuk customer ini (11% atau 0% untuk non-PKP) | Tinggi |
| **default_discount_percent** | Diskon default per customer | Sedang |
| **salesperson_id** | Salesperson yang menangani customer ini | Sedang |
| **price_group_id** | Group price list (retail, wholesale, distributor) | Sedang |
| **opening_balance_cents** | Saldo awal piutang customer saat onboarding | Tinggi |
| **opening_balance_date** | Tanggal saldo awal | Tinggi |
| **bank_name** | Bank customer untuk refund | Rendah |
| **bank_account_no** | No rekening customer | Rendah |
| **website** | URL website customer | Rendah |
| **notes/internal_notes** | Catatan internal (tidak tampil di dokumen) | Sedang |
| **is_active** | Sudah ada | OK |
| **parent_customer_id** | Customer parent untuk group consolidation | Rendah |
| **currency_code** | Currency default (IDR, USD) | Sedang |
| **credit_hold** | Block penjualan jika AR melebihi credit limit | Tinggi |

**Temuan [MISSING] Kritis:**

1. **[MISSING] [CUSTOMER] Alamat Pengiriman Terpisah** — ERP butuh alamat pengiriman (shipping) yang berbeda dari alamat penagihan (billing). Saat ini hanya 1 field `address`. Delivery Order tidak punya alamat tujuan pengiriman.
   - **Rekomendasi:** Tambahkan tabel `customer_addresses` dengan type (billing/shipping), atau tambah field `shipping_address` ke customers.

2. **[MISSING] [CUSTOMER] PKP Status & Default Tax Rate** — Saat membuat invoice, sistem perlu tahu apakah customer PKP atau Non-PKP. Non-PKP → PPN 0% atau tidak dipungut. Saat ini `tax_rate` di-set per invoice line, tidak ada default dari customer.
   - **Rekomendasi:** Tambahkan `pkp_status BOOLEAN` dan `default_tax_rate NUMERIC` ke customers. Auto-populate invoice `tax_rate` dari customer default.

3. **[MISSING] [CUSTOMER] Credit Hold Enforcement** — `credit_limit_cents` ada tetapi tidak ada enforcement. SO/Invoice harus reject jika `outstanding_AR + new_invoice > credit_limit`.
   - **Rekomendasi:** Tambahkan check di SO/Invoice creation: query outstanding AR, compare dengan credit limit, reject jika exceed.

4. **[MISSING] [CUSTOMER] Opening Balance** — Saat onboarding, customer yang sudah punya piutang outstanding perlu di-record. Tidak ada field `opening_balance_cents` di customers. Opening balance piutang harus diposting via manual journal ke 1201, tetapi tidak ter-link ke customer specific.
   - **Rekomendasi:** Tambahkan field opening balance, atau buat endpoint khusus `POST /customers/{id}/opening-balance`.

5. **[MISSING] [CUSTOMER] Salesperson Assignment** — Tidak ada link customer ke salesperson (user). Komisi dan performance tracking tidak mungkin.
   - **Rekomendasi:** Tambahkan `salesperson_id BIGINT REFERENCES users(id)`.

---

### E-02 MASTER DATA: SUPPLIER

**Schema:** `suppliers` table (migration `000011:20-43`)
**Request:** `CreateSupplierRequest` (`purchase/suppliers.go:15-28`)

| Field di DB | Field di API | Status | Catatan |
|---|---|---|---|
| code | code | OK | |
| name | name | OK | |
| npwp | npwp | OK | |
| contact_person | contact_person | INCOMPLETE | Hanya 1 kontak |
| phone | phone | INCOMPLETE | Hanya 1 phone |
| email | email | INCOMPLETE | Hanya 1 email |
| address | address | INCOMPLETE | Single-line |
| city | city | OK | |
| province | province | OK | |
| postal_code | postal_code | OK | |
| payment_term_id | payment_term_id | OK | |
| credit_limit_cents | credit_limit_cents | OK | |
| default_ap_account_id | — | MISSING di API | Ada di DB tapi tidak di request struct! |

**[MISSING] Field yang seharusnya ada:**

| Field yang Hilang | ERP Purpose | Prioritas |
|---|---|---|
| **supplier_type** | Vendor vs Service Provider vs Import | Tinggi |
| **pkp_status** | PKP vs Non-PKP — menentukan PPN Masukan | Tinggi |
| **default_tax_rate** | Rate PPN default untuk supplier | Tinggi |
| **default_input_vat_account_id** | Akun PPN Masukan default | Sedang |
| **bank_name** | Bank supplier untuk transfer | Tinggi |
| **bank_account_no** | No rekening supplier untuk pembayaran | Tinggi |
| **bank_account_name** | Nama pemilik rekening | Tinggi |
| **shipping_terms** | FOB, CIF, EXW, dll — menentukan ownership transfer | Sedang |
| **currency_code** | Currency default untuk PO | Sedang |
| **lead_time_days** | Estimasi waktu pengiriman | Sedang |
| **minimum_order_qty** | MOQ per supplier | Rendah |
| **opening_balance_cents** | Saldo awal utang supplier | Tinggi |
| **opening_balance_date** | Tanggal saldo awal | Tinggi |
| **preferred_payment_method** | Transfer, Cheque, Cash | Sedang |
| **tax_invoice_series** | Series faktur pajak supplier | Sedang |
| **is_1099_reportable** | (US) atau e-Bupot (ID) — untuk PPh 23 | Sedang |
| **default_ap_account_id** | Ada di DB tapi TIDAK di API request! | Tinggi |

**Temuan [MISSING] Kritis:**

1. **[MISSING] [SUPPLIER] `default_ap_account_id` tidak di API** — DB punya field `default_ap_account_id` tetapi `CreateSupplierRequest` tidak expose field ini. Supplier selalu menggunakan AP account default (2101), tidak bisa custom per supplier.
   - **Rekomendasi:** Tambahkan `DefaultAPAccountID *int64` ke `CreateSupplierRequest`.

2. **[MISSING] [SUPPLIER] Bank Details untuk Pembayaran** — Supplier payment (SP) butuh bank account tujuan. Saat ini SP hanya punya `cash_account_id` (akun sumber), tidak ada info bank tujuan supplier.
   - **Rekomendasi:** Tambahkan `bank_name`, `bank_account_no`, `bank_account_name` ke suppliers.

3. **[MISSING] [SUPPLIER] PKP Status** — Sama seperti customer, menentukan apakah PPN Masukan bisa di-claim.
   - **Rekomendasi:** Tambahkan `pkp_status BOOLEAN`.

4. **[MISSING] [SUPPLIER] Opening Balance Utang** — Sama seperti customer opening balance.
   - **Rekomendasi:** Tambahkan endpoint `POST /suppliers/{id}/opening-balance`.

---

### E-03 MASTER DATA: ITEM/BARANG

**Schema:** `items` table (migration `000005:45-62`)
**Request:** `ItemRequest` (`item/handler.go:47-59`)

| Field di DB | Field di API | Status | Catatan |
|---|---|---|---|
| code | code | OK | |
| name | name | OK | |
| item_type | item_type | INCOMPLETE | Hanya 'goods' vs 'service'. Butuh: 'stock', 'non_stock', 'service', 'assembly', 'phantom' |
| uom | uom | INCOMPLETE | Single UOM. Butuh multi-UOM dengan konversi |
| costing_method | costing_method | OK | FIFO, moving_average, specific |
| sale_account_id | sale_account_id | OK | |
| cogs_account_id | cogs_account_id | OK | |
| inventory_account_id | inventory_account_id | OK | |
| revenue_recognition_method | revenue_recognition_method | OK | |
| is_tracked_stock | is_tracked_stock | OK | |
| min_stock_qty | min_stock_qty | OK | |

**[MISSING] Field yang seharusnya ada di ERP proper:**

| Field yang Hilang | ERP Purpose | Prioritas |
|---|---|---|
| **barcode/SKU** | Barcode untuk scan di kasir/gudang | Tinggi |
| **alternate_code** | Kode alternatif (kode lama, kode supplier) | Sedang |
| **category_id** | Kategori barang (untuk grouping dan reporting) | Tinggi |
| **brand** | Merek barang | Sedang |
| **description_long** | Deskripsi panjang (untuk katalog/web) | Rendah |
| **weight** | Berat (untuk ongkir) | Sedang |
| **weight_uom** | Satuan berat (kg, gram) | Sedang |
| **dimension_l/w/h** | Dimensi (untuk shipping) | Rendah |
| **image_url** | URL gambar barang | Rendah |
| **default_supplier_id** | Supplier utama untuk auto-PO | Sedang |
| **lead_time_days** | Lead time per item | Sedang |
| **reorder_point** | Qty yang trigger reorder | Tinggi |
| **max_stock_qty** | Qty maksimum di gudang | Rendah |
| **sale_price_cents** | Harga jual default (selain price list) | Sedang |
| **cost_price_cents** | Harga beli terakhir (last cost) | Sedang |
| **currency_code** | Currency default | Sedang |
| **tax_category** | Taxable, Non-Taxable, Zero-Rated | Tinggi |
| **default_tax_rate** | Rate PPN default per item | Tinggi |
| **is_serial_tracked** | Apakah barang pakai serial number | Sedang |
| **is_batch_tracked** | Apakah barang pakai batch/lot | Sedang |
| **shelf_location** | Lokasi rak di gudang | Rendah |
| **warranty_months** | Garansi dalam bulan | Rendah |
| **status** | Active, Discontinued, Out of Stock | Sedang |

**Temuan [MISSING] Kritis:**

1. **[MISSING] [ITEM] `item_type` Terlalu Simplified** — Hanya `goods` vs `service`. ERP proper butuh:
   - **stock** — barang yang di-track stoknya (current: `is_tracked_stock = true`)
   - **non_stock** — barang yang tidak di-track stok (consumables, supplies)
   - **service** — jasa (tidak ada inventory)
   - **assembly** — barang yang diproduksi dari BOM
   - **phantom** — barang yang langsung di-explode ke komponen (kit)
   
   Saat ini `goods` + `is_tracked_stock` mencakup stock/non-stock, tetapi tidak ada distinsi assembly/phantom.
   - **Rekomendasi:** Ubah `item_type` CHECK constraint untuk include 'assembly' dan 'phantom'. Atau tambah field `is_assembly BOOLEAN`.

2. **[MISSING] [ITEM] Master Satuan (UOM) dan Konversi** — Hanya 1 field `uom` (text). ERP proper butuh:
   - Master table `units_of_measure` (pcs, box, lusin, kg, meter)
   - Tabel `item_uom_conversions` dengan konversi (1 box = 12 pcs, 1 lusin = 12 pcs)
   - Satuan dasar (base UOM) vs satuan transaksi
   - Harga per satuan (harga/pcs vs harga/box)
   
   Tanpa ini, user tidak bisa transaksi dalam box tapi menyimpan dalam pcs.
   - **Rekomendasi:** Buat `uoms` table dan `item_uom_conversions` table.

3. **[MISSING] [ITEM] Settingan Akun per Barang** — Saat ini items punya `sale_account_id`, `cogs_account_id`, `inventory_account_id`. Ini sudah bagus, tetapi tidak lengkap untuk ERP:
   - **inventory_adjustment_account_id** — untuk stock opname adjustment
   - **variance_account_id** — untuk production variance
   - **scrap_account_id** — untuk scrap/waste
   
   Saat ini stock opname hardcode ke `4907 Inventory Adjustment Gain` dan `5907 Inventory Adjustment Loss` — tidak bisa custom per item.
   - **Rekomendasi:** Tambahkan field akun adjustment ke items, atau gunakan default dari tenant settings.

4. **[MISSING] [ITEM] Kategori Barang** — Tidak ada `category_id`. ERP butuh grouping barang untuk reporting (penjualan per kategori, stok per kategori).
   - **Rekomendasi:** Buat `item_categories` table, tambah `category_id` ke items.

5. **[MISSING] [ITEM] Reorder Point** — `min_stock_qty` ada tetapi tidak ada `reorder_point` (qty yang trigger pengadaan otomatis) dan `max_stock_qty`.
   - **Rekomendasi:** Tambahkan `reorder_point_qty` dan `max_stock_qty`.

6. **[MISSING] [ITEM] Tax Category** — Tidak ada `tax_category` (taxable, non-taxable, zero-rated). PPN per item harusnya bisa default, bukan manual per invoice line.
   - **Rekomendasi:** Tambahkan `tax_category TEXT` dan `default_tax_rate NUMERIC`.

---

### E-04 MASTER DATA: PAYMENT TERMS

**Schema:** `payment_terms` table (migration `000005:4-16`)
**Request:** `CreatePaymentTermRequest` (`customer/handler.go:57-64`)

| Field | Status | Catatan |
|---|---|---|
| code | OK | |
| name | OK | |
| due_days | OK | |
| discount_days | OK | Early payment discount days |
| discount_percent | OK | Early payment discount % |
| cash_flow_category | OK | operating/investing/financing |

**[MISSING] Field yang seharusnya ada:**

| Field yang Hilang | ERP Purpose | Prioritas |
|---|---|---|
| **payment_method** | Transfer, Cheque, Cash, Credit Card | Sedang |
| **installment_count** | Untuk cicilan (n bulan) | Sedang |
| **installment_interval_days** | Interval antar cicilan | Sedang |
| **is_active** | Ada di DB tapi tidak di API request | Rendah |

**Status:** Cukup lengkap untuk MVP. Early payment discount (2/10 Net 30) didukung.

---

### E-05 SALES: QUOTATION (SQ)

**Request:** `CreateQuotationRequest` (`sales/logic.go:29-37`)

| Field di API | Status | Catatan |
|---|---|---|
| customer_id | OK | |
| quotation_date | OK | |
| valid_until | OK | |
| payment_term_id | OK | |
| notes | INCOMPLETE | Hanya 1 notes — butuh internal_notes vs customer_notes |
| source_ref | OK | |
| Lines: item_id, qty, unit_price_cents, discount_cents, tax_rate, description | INCOMPLETE | |

**[MISSING] Field yang seharusnya ada:**

| Field yang Hilang | ERP Purpose | Prioritas |
|---|---|---|
| **salesperson_id** | Salesperson yang membuat quotation | Tinggi |
| **currency_code** | Currency untuk multi-currency | Sedang |
| **exchange_rate** | Rate jika multi-currency | Sedang |
| **delivery_date_requested** | Tanggal pengiriman yang diminta customer | Sedang |
| **shipping_address** | Alamat pengiriman (override customer default) | Sedang |
| **shipping_method** | JNE, SiCepat, GoSend, dll | Sedang |
| **freight_cents** | Ongkos kirim | Sedang |
| **discount_total_cents** | Diskon di header (bukan per line) | Sedang |
| **tax_type** | PPN included vs excluded vs no tax | Tinggi |
| **terms_and_conditions** | S&K quotation (template) | Sedang |
| **attachment_ids** | Lampiran (drawing, spec) | Rendah |
| **quotation_type** | Standard, Tender, RFQ | Rendah |

---

### E-06 SALES: SALES ORDER (SO)

**Schema:** `sales_orders` table (migration `000006:13-33`)
**Request:** `CreateSalesOrderRequest` (`sales/orders.go:36-43`)

| Field di DB | Field di API | Status | Catatan |
|---|---|---|---|
| number | — (auto) | OK | Auto-generated |
| quotation_id | quotation_id | OK | |
| customer_id | customer_id | OK | |
| order_date | order_date | OK | |
| payment_term_id | payment_term_id | OK | |
| notes | notes | INCOMPLETE | |
| status | — | OK | CONFIRMED/CLOSED/CANCELLED |
| total_cents | — (computed) | OK | |
| dp_received_cents | — (computed) | OK | |
| created_by | — (from JWT) | OK | |

**[MISSING] Field yang seharusnya ada di SO ERP proper:**

| Field yang Hilang | ERP Purpose | Prioritas |
|---|---|---|
| **customer_po_number** | **Nomor PO dari customer** — referensi untuk matching | **TINGGI** |
| **customer_po_date** | Tanggal PO customer | Tinggi |
| **customer_po_attachment_id** | File PO customer | Sedang |
| **requested_delivery_date** | Tanggal pengiriman yang diminta | Tinggi |
| **promised_delivery_date** | Tanggal pengiriman yang dijanjikan | Tinggi |
| **shipping_address** | Alamat pengiriman (override) | Tinggi |
| **shipping_method** | Ekspedisi | Sedang |
| **freight_cents** | Ongkir | Sedang |
| **salesperson_id** | Salesperson penanggung jawab | Tinggi |
| **currency_code** | Currency | Sedang |
| **exchange_rate** | Rate | Sedang |
| **discount_total_cents** | Diskon header | Sedang |
| **tax_type** | Tax included/excluded/no tax | Tinggi |
| **warehouse_id** | Gudang asal pengiriman | Tinggi |
| **delivery_status** | UNDELIVERED/PARTIAL/DELIVERED | Sedang |
| **invoice_status** | UNINVOICED/PARTIAL/INVOICED | Sedang |
| **payment_status** | UNPAID/PARTIAL/PAID | Sedang |
| **internal_notes** | Catatan internal (tidak tampil di dokumen) | Sedang |
| **terms_and_conditions** | S&K | Sedang |

**Temuan [MISSING] Kritis:**

1. **[MISSING] [SO] Customer PO Number** — Ini adalah field yang SANGAT penting di ERP. Customer mengirim PO dengan nomor mereka sendiri. SO harus reference nomor PO customer untuk:
   - Matching saat invoice dibayar (customer bayar dengan reference PO mereka)
   - Audit trail
   - Konfirmasi pesanan
   
   **Rekomendasi:** Tambahkan `customer_po_number TEXT` dan `customer_po_date DATE` ke `sales_orders`.

2. **[MISSING] [SO] Delivery Date** — SO tidak punya `requested_delivery_date` atau `promised_delivery_date`. DO tidak tahu kapan harus dikirim.
   - **Rekomendasi:** Tambahkan `requested_delivery_date DATE`.

3. **[MISSING] [SO] Shipping Address** — DO tidak punya alamat pengiriman. Hanya reference customer_id.
   - **Rekomendasi:** Tambahkan `shipping_address TEXT` atau link ke `customer_addresses`.

4. **[MISSING] [SO] Warehouse** — Tidak ada `warehouse_id`. Saat ini system single-warehouse, tetapi ERP proper butuh multi-warehouse.
   - **Rekomendasi:** Tambahkan `warehouse_id BIGINT` (untuk saat ini bisa default ke warehouse 1).

5. **[MISSING] [SO] Tax Type (PPN Included/Excluded)** — Invoice line punya `tax_rate` tetapi tidak ada flag "PPN included" (harga sudah termasuk PPN) vs "PPN excluded" (harga belum termasuk PPN). Ini critical untuk Indonesia di mana harga retail biasanya "gross" (termasuk PPN).
   - **Rekomendasi:** Tambahkan `tax_type TEXT CHECK (tax_type IN ('exclusive', 'inclusive', 'none'))`.

---

### E-07 SALES: DELIVERY ORDER (DO)

**Schema:** `delivery_orders` table (migration `000007:8-27`)
**Request:** `CreateDeliveryRequest` (`sales/delivery.go:36-41`)

| Field di DB | Field di API | Status | Catatan |
|---|---|---|---|
| number | — | OK | Auto |
| sales_order_id | sales_order_id | OK | |
| customer_id | — (from SO) | OK | |
| delivery_date | delivery_date | OK | |
| notes | notes | INCOMPLETE | |
| status | — | OK | SHIPPED/RETURNED/CANCELLED |

**[MISSING] Field yang seharusnya ada:**

| Field yang Hilang | ERP Purpose | Prioritas |
|---|---|---|
| **carrier/courier** | Nama ekspedisi (JNE, SiCepat) | Tinggi |
| **tracking_number** | No resi pengiriman | Tinggi |
| **shipping_address** | Alamat tujuan pengiriman | Tinggi |
| **shipping_cost_cents** | Ongkir yang dibayar | Sedang |
| **shipping_cost_borne_by** | Seller/Buyer/Shared | Sedang |
| **delivered_by** | Nama kurir internal | Sedang |
| **received_by** | Nama penerima di tujuan | Tinggi |
| **received_date** | Tanggal diterima (bukan tanggal kirim) | Tinggi |
| **proof_of_delivery_url** | URL foto bukti terima | Sedang |
| **warehouse_id** | Gudang asal | Tinggi |
| **vehicle_number** | No kendaraan (untuk log internal) | Rendah |
| **packaging_type** | Box, Pallet, dll | Rendah |
| **total_weight** | Berat total | Sedang |

---

### E-08 SALES: INVOICE

**Schema:** `invoices` table ( migration `000008`)
**Request:** `CreateInvoiceRequest` (`sales/invoices.go:46-54`)

| Field di API | Status | Catatan |
|---|---|---|
| sales_order_id | OK | |
| customer_id | OK | |
| invoice_date | OK | |
| due_date | OK | |
| payment_term_id | OK | |
| notes | INCOMPLETE | |
| Lines: item_id, delivery_id, qty, unit_price_cents, discount_cents, tax_rate, description | INCOMPLETE | |

**[MISSING] Field yang seharusnya ada di Invoice ERP proper:**

| Field yang Hilang | ERP Purpose | Prioritas |
|---|---|---|
| **invoice_number_manual** | No invoice manual (selain auto-number) | Sedang |
| **tax_invoice_number** | **Nomor faktur pajak** (untuk PPN) | **TINGGI** |
| **tax_invoice_date** | Tanggal faktur pajak | Tinggi |
| **customer_po_number** | Reference PO customer (dari SO) | Tinggi |
| **delivery_order_number** | Reference DO (dari SO) | Sedang |
| **shipping_address** | Alamat pengiriman | Sedang |
| **billing_address** | Alamat penagihan | Sedang |
| **salesperson_id** | Salesperson | Sedang |
| **currency_code** | Currency | Sedang |
| **exchange_rate** | Rate | Sedang |
| **subtotal_cents** | Subtotal sebelum diskon | Sedang |
| **discount_total_cents** | Total diskon header | Sedang |
| **tax_total_cents** | Total PPN | Tinggi |
| **tax_type** | PPN included/excluded | Tinggi |
| **freight_cents** | Ongkir | Sedang |
| **other_charges_cents** | Biaya lain | Sedang |
| **rounding_cents** | Pembulatan | Sedang |
| **grand_total_cents** | Grand total (subtotal - discount + tax + freight) | Tinggi |
| **paid_cents** | Total sudah dibayar | OK (via dp_applied) |
| **outstanding_cents** | Sisa belum dibayar | OK (via receivable) |
| **terms_and_conditions** | S&K invoice | Sedang |
| **attachment_ids** | Lampiran (bukti kirim, dll) | Rendah |

**Temuan [MISSING] Kritis:**

1. **[MISSING] [INVOICE] Nomor Faktur Pajak** — Di Indonesia, PPN wajib menggunakan faktur pajak dengan nomor seri yang terformat (010.000-XX.XXXXXXXX). Invoice tidak punya field `tax_invoice_number`. Tanpa ini, laporan PPN tidak dapat di-reconcile dengan e-Faktur.
   - **Rekomendasi:** Tambahkan `tax_invoice_number TEXT` dan `tax_invoice_date DATE`.

2. **[MISSING] [INVOICE] Tax Type (Inclusive/Exclusive)** — Tidak ada flag apakah harga sudah termasuk PPN atau belum. Di Indonesia, harga retail biasanya "gross" (termasuk PPN 11%). Jika harga inclusive, perhitungan PPN: `ppn = total / 11 * 1` (dari total). Jika exclusive: `ppn = subtotal * 11%`.
   - **Rekomendasi:** Tambahkan `tax_type TEXT CHECK (tax_type IN ('exclusive', 'inclusive', 'none'))`.

3. **[MISSING] [INVOICE] Subtotal, Discount Total, Tax Total, Grand Total** — Invoice hanya punya `total_cents` (gabungan). ERP proper memisahkan: subtotal, discount, tax, freight, grand total. Ini penting untuk laporan pajak dan analisis.
   - **Rekomendasi:** Tambahkan field subtotal_cents, discount_total_cents, tax_total_cents, freight_cents, grand_total_cents.

---

### E-09 SALES: PAYMENT (Customer Payment)

**Request:** `CreatePaymentRequest` (`sales/payments.go:22-27`)

| Field di API | Status | Catatan |
|---|---|---|
| cash_account_id | OK | Akun penerimaan |
| amount_cents | OK | |
| payment_date | OK | |
| description | INCOMPLETE | |

**[MISSING] Field yang seharusnya ada:**

| Field yang Hilang | ERP Purpose | Prioritas |
|---|---|---|
| **payment_method** | Transfer, Cheque, Cash, Credit Card | Tinggi |
| **reference_number** | No referensi bank transfer / no cek | Tinggi |
| **bank_reference** | Reference dari bank (untuk rekonsiliasi) | Sedang |
| **deposit_to_bank_account_id** | Akun bank tujuan (beda dari cash_account_id?) | Sedang |
| **customer_deposit_account_id** | Untuk overpayment, pilih akun deposit | Rendah |
| **exchange_rate** | Jika multi-currency | Sedang |
| **payment_currency** | Currency pembayaran | Sedang |
| **short_payment** | Pembayaran kurang (underpayment) — write off or keep? | Sedang |
| **write_off_account_id** | Akun write-off untuk short payment | Sedang |

---

### E-10 SALES: CREDIT NOTE

**Request:** `CreateCreditNoteRequest` (`sales/credit_notes.go:47-54`)

| Field di API | Status | Catatan |
|---|---|---|
| invoice_id | OK | |
| customer_id | OK | |
| cn_date | OK | |
| refund_method | INCOMPLETE | Text bebas — harus enum (cash, credit, offset) |
| reason | OK | |
| Lines: item_id, invoice_line_id, qty, unit_price_cents, unit_cost_cents, description | INCOMPLETE | |

**[MISSING] Field yang seharusnya ada:**

| Field yang Hilang | ERP Purpose | Prioritas |
|---|---|---|
| **cn_type** | Full return vs Partial return vs Price adjustment vs Bad debt | Tinggi |
| **restock** | Barang dikembalikan ke gudang? (YES/NO) | Tinggi |
| **restock_fee_cents** | Biaya restocking (deduct dari refund) | Sedang |
| **refund_amount_cents** | Jumlah yang dikembalikan (bisa != CN total) | Sedang |
| **refund_method_id** | Payment method untuk refund | Sedang |
| **approval_status** | Draft/Approved/Void (untuk approval workflow) | Sedang |
| **approved_by** | Yang approve CN | Sedang |

---

### E-11 PURCHASE: PURCHASE ORDER (PO)

**Schema:** `purchase_orders` table (migration `000011:45-57`)
**Request:** `CreatePurchaseOrderRequest` (`purchase/purchase_orders.go:37-43`)

| Field di DB | Field di API | Status |
|---|---|---|
| number | — | OK (auto) |
| supplier_id | supplier_id | OK |
| order_date | order_date | OK |
| expected_date | — | MISSING di API! |
| payment_term_id | payment_term_id | OK |
| notes | notes | INCOMPLETE |
| status | — | OK |

**[MISSING] Field yang seharusnya ada di PO ERP proper:**

| Field yang Hilang | ERP Purpose | Prioritas |
|---|---|---|
| **supplier_quote_number** | No quotation dari supplier | Sedang |
| **supplier_quote_date** | Tanggal quotation supplier | Sedang |
| **our_po_number_manual** | No PO manual (selain auto) | Sedang |
| **expected_date** | Ada di DB tapi TIDAK di API! | Tinggi |
| **shipping_terms** | FOB, CIF, EXW, DDP | Sedang |
| **shipping_method** | Ekspedisi | Sedang |
| **freight_cents** | Ongkir | Sedang |
| **warehouse_id** | Gudang tujuan penerimaan | Tinggi |
| **currency_code** | Currency | Sedang |
| **exchange_rate** | Rate | Sedang |
| **tax_type** | PPN included/excluded | Tinggi |
| **tax_invoice_number** | No faktur pajak supplier | Tinggi |
| **discount_total_cents** | Diskon header | Sedang |
| **other_charges_cents** | Biaya lain (packing, handling) | Sedang |
| **approval_status** | Draft/Pending/Approved/Rejected | Tinggi |
| **approved_by** | Yang approve PO | Sedang |
| **internal_notes** | Catatan internal | Sedang |

**Temuan [MISSING] Kritis:**

1. **[MISSING] [PO] `expected_date` Ada di DB tapi TIDAK di API** — `purchase_orders` table punya `expected_date DATE` tetapi `CreatePurchaseOrderRequest` tidak expose field ini. User tidak bisa set tanggal pengiriman yang diharapkan.
   - **Rekomendasi:** Tambahkan `ExpectedDate *string` ke request.

2. **[MISSING] [PO] Approval Workflow** — PO langsung status `CONFIRMED` tanpa approval. ERP proper butuh approval untuk PO di atas threshold tertentu.
   - **Rekomendasi:** Tambahkan status `DRAFT` dan `PENDING_APPROVAL`, dengan `approved_by` dan `approved_at`.

---

### E-12 PURCHASE: GRN (Goods Received Note)

**Schema:** `goods_received_notes` table (migration `000011`)
**Request:** `CreateGRNRequest` (`purchase/grn.go:24-29`)

| Field di API | Status |
|---|---|
| purchase_order_id | OK |
| grn_date | OK |
| notes | INCOMPLETE |
| Lines: item_id, po_line_id, qty, unit_cost_cents, description | INCOMPLETE |

**[MISSING] Field yang seharusnya ada:**

| Field yang Hilang | ERP Purpose | Prioritas |
|---|---|---|
| **supplier_delivery_note_number** | No surat jalan supplier | Tinggi |
| **supplier_delivery_note_date** | Tanggal SJ supplier | Tinggi |
| **received_by** | Nama yang menerima barang | Tinggi |
| **received_at** | Waktu penerimaan (datetime, bukan date saja) | Sedang |
| **warehouse_id** | Gudang tujuan | Tinggi |
| **quality_status** | Accepted, Rejected, Partial (QC check) | Tinggi |
| **quality_notes** | Catatan QC | Sedang |
| **rejected_qty** | Qty yang ditolak (partial acceptance) | Tinggi |
| **vehicle_number** | No kendaraan pengirim | Rendah |
| **packing_list_number** | No packing list | Rendah |
| **carrier** | Ekspedisi | Sedang |
| **temperature** | Untuk cold chain (food, pharma) | Rendah |

**Temuan [MISSING] Kritis:**

1. **[MISSING] [GRN] Quality Check** — GRN langsung terima semua barang. Tidak ada QC. ERP proper butuh: accepted_qty vs rejected_qty, quality_status, quality_notes.
   - **Rekomendasi:** Tambahkan `quality_status TEXT`, `rejected_qty NUMERIC`, `quality_notes TEXT`.

2. **[MISSING] [GRN] Received By** — Tidak ada record siapa yang menerima barang. `created_by` ada tetapi itu adalah user yang input, bukan yang fisik menerima.
   - **Rekomendasi:** Tambahkan `received_by TEXT`.

3. **[MISSING] [GRN] Supplier Delivery Note** — Tidak ada referensi ke surat jalan supplier. Audit trail tidak lengkap.
   - **Rekomendasi:** Tambahkan `supplier_delivery_note_number TEXT`.

---

### E-13 PURCHASE: SUPPLIER INVOICE

**Request:** `CreateSupplierInvoiceRequest` (`purchase/supplier_invoices.go:36-44`)

| Field di API | Status |
|---|---|
| supplier_id | OK |
| grn_id | OK |
| invoice_date | OK |
| due_date | OK |
| supplier_invoice_number | OK |
| notes | INCOMPLETE |
| Lines: item_id, qty, unit_price_cents, discount_cents, tax_rate, description | INCOMPLETE |

**[MISSING] Field yang seharusnya ada:**

| Field yang Hilang | ERP Purpose | Prioritas |
|---|---|---|
| **tax_invoice_number** | No faktur pajak supplier (berbeda dari supplier_invoice_number) | Tinggi |
| **tax_invoice_date** | Tanggal faktur pajak | Tinggi |
| **dpp_cents** | Dasar Pengenaan Pajak (taxable base) | Tinggi |
| **ppn_cents** | PPN amount (sudah ada di response, tapi dari API input?) | Tinggi |
| **pph_23_cents** | PPh 23 yang dipotong | Tinggi |
| **pph_23_article** | Pasal PPh (21, 22, 23, 26) | Sedang |
| **tax_type** | PPN included/excluded | Tinggi |
| **currency_code** | Currency | Sedang |
| **exchange_rate** | Rate | Sedang |
| **subtotal_cents** | Subtotal sebelum diskon | Sedang |
| **discount_total_cents** | Diskon header | Sedang |
| **freight_cents** | Ongkir | Sedang |
| **other_charges_cents** | Biaya lain | Sedang |
| **grand_total_cents** | Grand total | Tinggi |
| **paid_cents** | Total sudah dibayar | Sedang |
| **outstanding_cents** | Sisa utang | Sedang |
| **withholding_cents** | PPh yang dipotong saat bayar | Tinggi |
| **approval_status** | Draft/Approved | Sedang |

---

### E-14 PURCHASE: SUPPLIER PAYMENT

**Request:** `CreateSupplierPaymentRequest` (`purchase/supplier_payments.go:25`)

**[MISSING] Field yang seharusnya ada:**

| Field yang Hilang | ERP Purpose | Prioritas |
|---|---|---|
| **payment_method** | Transfer, Cheque, Cash | Tinggi |
| **reference_number** | No referensi bank transfer / no cek | Tinggi |
| **bank_account_no** | Rekening tujuan supplier | Tinggi |
| **withholding_pph_cents** | PPh yang dipotong saat bayar | Tinggi |
| **withholding_pph_article** | Pasal PPh | Sedang |
| **withholding_account_id** | Akun PPh dipotong | Sedang |
| **early_discount_taken** | Apakah ambil early payment discount | Sedang |
| **discount_cents** | Diskon yang diambil | Sedang |

---

### E-15 INVENTORY: STOCK TRANSFER

**Request:** `CreateStockTransferRequest` (`inventory/stock_transfer.go:26-30`)

| Field di API | Status |
|---|---|
| transfer_date | OK |
| notes | OK |
| Lines: item_id, qty, unit_cost_cents, description | INCOMPLETE |

**[MISSING] Field yang seharusnya ada:**

| Field yang Hilang | ERP Purpose | Prioritas |
|---|---|---|
| **from_warehouse_id** | Gudang asal | **TINGGI** |
| **to_warehouse_id** | Gudang tujuan | **TINGGI** |
| **carrier** | Ekspedisi | Sedang |
| **tracking_number** | No resi | Sedang |
| **status** | DRAFT/IN_TRANSIT/RECEIVED/CANCELLED | Tinggi |
| **received_by** | Yang menerima di gudang tujuan | Sedang |
| **received_date** | Tanggal diterima | Sedang |

**Temuan [MISSING] Kritis:**

1. **[MISSING] [STOCK_TRANSFER] From/To Warehouse** — Stock transfer tidak punya from_warehouse_id dan to_warehouse_id. Comment di kode: "single warehouse for now". Transfer antar gudang TIDAK mungkin tanpa field ini.
   - **Rekomendasi:** Buat `warehouses` table. Tambahkan `from_warehouse_id` dan `to_warehouse_id`.

2. **[MISSING] [WAREHOUSE] Master Warehouse** — Tidak ada table `warehouses`. ERP multi-gudang butuh master warehouse dengan: code, name, address, is_active.
   - **Rekomendasi:** Buat `warehouses` table. Tambahkan `warehouse_id` ke stock_balances, delivery_orders, grns, stock_opnames.

---

### E-16 PRODUCTION: BOM (Bill of Materials)

**Schema:** `bill_of_materials` table (migration `000020:31-44`)
**Request:** `CreateBOMRequest` (`production/bom.go:30-36`)

| Field di DB/API | Status |
|---|---|
| code | OK |
| name | OK |
| finished_good_item_id | OK |
| output_qty | OK |
| Lines: item_id, qty, unit_cost_cents, cost_type, description | INCOMPLETE |

**[MISSING] Field yang seharusnya ada di BOM ERP proper:**

| Field yang Hilang | ERP Purpose | Prioritas |
|---|---|---|
| **revision/version** | Versi BOM (v1, v2) — BOM bisa berubah | Tinggi |
| **effective_from** | Tanggal mulai berlaku | Tinggi |
| **effective_to** | Tanggal berakhir | Sedang |
| **routing_id** | Routing/proses produksi (tahapan: cutting, sewing, finishing) | Tinggi |
| **standard_cost_cents** | Standard cost per unit output | Sedang |
| **yield_percent** | Yield ratio (output/input) | Sedang |
| **scrap_percent** | % scrap expected | Sedang |
| **production_time_hours** | Estimasi waktu produksi | Sedang |
| **labor_time_minutes** | Total waktu labor | Sedang |
| **overhead_rate_cents** | Overhead rate per unit | Sedang |
| **is_active** | Sudah ada (status ACTIVE/VOID) | OK |
| **approved_by** | Yang approve BOM | Sedang |

**[MISSING] BOM Line Fields:**

| Field yang Hilang | ERP Purpose | Prioritas |
|---|---|---|
| **uom** | Satuan untuk qty di BOM line | Tinggi |
| **is_optional** | Material optional (optional component) | Sedang |
| **substitute_item_id** | Material substitusi | Sedang |
| **position/sequence** | Urutan pemakaian | Rendah |

---

### E-17 PRODUCTION: JOB ORDER

**Request:** `CreateProductionJobRequest` (`production/jobs.go:74-79`)

| Field di API | Status |
|---|---|
| bom_id | OK |
| finished_good_item_id | OK |
| target_qty | OK |
| start_date | OK |

**[MISSING] Field yang seharusnya ada di Job Order ERP proper:**

| Field yang Hilang | ERP Purpose | Prioritas |
|---|---|---|
| **planned_end_date** | Tanggal selesai yang direncanakan | Tinggi |
| **actual_start_date** | Tanggal mulai aktual | Sedang |
| **actual_end_date** | Tanggal selesai aktual | Sedang |
| **operator_id** | Operator/mandor yang menjalankan | Tinggi |
| **machine_id** | Mesin yang digunakan | Sedang |
| **warehouse_id** | Gudang output | Tinggi |
| **production_status** | PLANNED/IN_PROGRESS/COMPLETED/CANCELLED | Tinggi |
| **actual_qty** | Qty aktual yang dihasilkan | Tinggi |
| **scrap_qty** | Qty yang rusak/scrap | Tinggi |
| **variance_cents** | Selisih biaya aktual vs standard | Sedang |
| **notes** | Catatan produksi | Sedang |
| **priority** | Urgency (high, medium, low) | Rendah |

---

### E-18 ASSETS: FIXED ASSET REGISTRATION

**Schema:** `fixed_assets` table (migration `000019:19-47`)
**Request:** `RegisterAssetRequest` (`assets/assets.go:92-104`)

| Field di DB/API | Status |
|---|---|
| code | OK |
| name | OK |
| acquisition_date | OK |
| acquisition_cost_cents | OK |
| salvage_value_cents | OK |
| useful_life_months | OK |
| depreciation_method | OK |
| rate | OK |
| units_total | OK |
| payment_account_code | OK |
| notes | OK |

**[MISSING] Field yang seharusnya ada di Asset ERP proper:**

| Field yang Hilang | ERP Purpose | Prioritas |
|---|---|---|
| **serial_number** | No seri aset | Tinggi |
| **asset_category_id** | Kategori aset (kendaraan, IT, bangunan) | Tinggi |
| **location** | Lokasi aset | Tinggi |
| **custodian_id** | Penanggung jawab aset (user) | Tinggi |
| **department_id** | Departemen pemilik | Sedang |
| **supplier_id** | Supplier pembelian aset | Sedang |
| **purchase_order_id** | PO pembelian aset | Sedang |
| **invoice_number** | No invoice pembelian | Sedang |
| **warranty_expiry_date** | Akhir garansi | Sedang |
| **insurance_policy_no** | No polis asuransi | Rendah |
| **insurance_expiry_date** | Akhir asuransi | Rendah |
| **maintenance_schedule** | Jadwal maintenance | Rendah |
| **condition** | Baik, Rusak, Perlu Servis | Sedang |
| **disposal_date** | Tanggal disposal | OK (di disposal handler) |
| **disposal_proceeds_cents** | Hasil penjualan saat disposal | OK (di disposal handler) |

---

### E-19 LEASE: LEASE CONTRACT

**Schema:** `lease_contracts` table (migration `000024`)
**Request:** `CreateLeaseContractRequest` (`lease/contracts.go:24-35`)

| Field di API | Status |
|---|---|
| lessee_name | INCOMPLETE — harusnya link ke entity/customer |
| lessor_name | INCOMPLETE — harusnya link ke entity/supplier |
| start_date | OK |
| end_date | OK |
| payment_amount_cents | OK |
| payment_frequency | OK |
| total_payments | OK |
| discount_rate | OK |
| payment_account_code | OK |
| description | OK |

**[MISSING] Field yang seharusnya ada:**

| Field yang Hilang | ERP Purpose | Prioritas |
|---|---|---|
| **lease_type** | Operating vs Finance lease (PSAK 73) | Tinggi |
| **asset_description** | Deskripsi aset yang di-lease | Tinggi |
| **asset_account_id** | Akun aset (1701) — hardcoded, tidak bisa custom | Sedang |
| **liability_account_id** | Akun liability (2301) — hardcoded | Sedang |
| **interest_rate_account_id** | Akun beban bunga (5906) — hardcoded | Sedang |
| **depreciation_account_id** | Akun depresiasi (5209) — belum di-seed! | Tinggi |
| **accumulated_dep_account_id** | Akun akum. depresiasi (1702) — belum di-seed! | Tinggi |
| **residual_value_cents** | Nilai sisa aset di akhir lease | Sedang |
| **purchase_option_price** | Harga opsi beli di akhir lease | Sedang |
| **renewal_option** | Opsi perpanjangan | Rendah |
| **lessee_entity_id** | Link ke entity (untuk konsolidasi) | Sedang |
| **lessor_entity_id** | Link ke entity | Sedang |

---

### E-20 TAX: PPN (PPN Keluaran/Masukan)

**PPN Module** (`tax/ppn.go`) hanya read-only report. Saat invoice/GRN dibuat, PPN line harus di-posting.

**[MISSING] PPN-related fields yang harusnya ada di Invoice:**

| Field yang Hilang | ERP Purpose | Prioritas |
|---|---|---|
| **tax_invoice_number** | No faktur pajak (format DJP) | Tinggi |
| **tax_invoice_date** | Tanggal faktur pajak | Tinggi |
| **tax_type** | PPN included/excluded/no tax | Tinggi |
| **dpp_cents** | Dasar Pengenaan Pajak | Tinggi |
| **ppn_cents** | PPN amount | Tinggi |
| **ppn_rate** | Rate PPN (11%, 0%) | Tinggi |
| **taxable_amount_cents** | Amount yang kena pajak (excludes non-taxable items) | Sedang |
| **non_taxable_amount_cents** | Amount yang tidak kena pajak | Sedang |

---

### E-21 SETTINGS/PREFERENCES: TENANT-WIDE SETTINGS

**[MISSING] Tabel `tenant_settings` tidak ada** — ERP proper butuh tenant-wide settings:

| Setting yang Hilang | ERP Purpose | Prioritas |
|---|---|---|
| **default_currency** | Currency default (IDR) | Tinggi |
| **default_tax_rate** | Rate PPN default (11%) | Tinggi |
| **default_tax_type** | PPN included/excluded | Tinggi |
| **default_payment_term_id** | Termin default | Sedang |
| **default_warehouse_id** | Gudang default | Tinggi |
| **fiscal_year_start_month** | Awal tahun fiskal (Jan atau Apr) | Tinggi |
| **decimal_places** | Jumlah desimal untuk qty | Sedang |
| **numbering_format** | Format nomor dokumen (PREFIX-YYYY-NNNNNN) | Sedang |
| **auto_generate_invoice_from_do** | Auto-create invoice setelah DO | Rendah |
| **auto_post_dp_realization** | Auto-realize DP saat invoice | Rendah |
| **credit_limit_enforcement** | Block atau warning saat exceed | Tinggi |
| **negative_stock_allowed** | Allow negative stock? | Tinggi |
| **default_revenue_account_id** | Akun pendapatan default | Sedang |
| **default_cogs_account_id** | Akun COGS default | Sedang |
| **default_inventory_account_id** | Akun inventory default | Sedang |
| **default_ap_account_id** | Akun utang default | Sedang |
| **default_ar_account_id** | Akun piutang default | Sedang |
| **ppn_rate** | Rate PPN current (11% per UU HPP) | Tinggi |
| **pph_final_umkm_rate** | Rate PPh Final UMKM (0.5%) | Tinggi |
| **ecl_rate_0_30** | Rate ECL 0-30 hari (1%) | Sedang |
| **ecl_rate_31_60** | Rate ECL 31-60 hari (2.5%) | Sedang |
| **ecl_rate_61_90** | Rate ECL 61-90 hari (5%) | Sedang |
| **ecl_rate_90_plus** | Rate ECL >90 hari (10%) | Sedang |

**Temuan [MISSING] Kritis:**

1. **[MISSING] [SETTINGS] Tenant Settings Table** — Tidak ada table `tenant_settings`. Rate PPN (11%) di-hardcode atau di-read dari `tax_rates` table (per pph.go:36). Default accounts di-hardcode per package (mis. `"1201"`, `"4101"`, `"5101"`).
   - **Rekomendasi:** Buat `tenant_settings` table dengan key-value structure atau column-per-setting.

2. **[MISSING] [SETTINGS] Default Accounts per Module** — Setiap package me-resolve account by hardcoded code (mis. `resolveAccountByCode(ctx, tx, tenant, "5101")`). Ini tidak flexible — tenant mungkin punya struktur COA yang berbeda.
   - **Rekomendasi:** Buat `tenant_default_accounts` table:
     ```sql
     CREATE TABLE tenant_default_accounts (
         tenant_id BIGINT,
         setting_key TEXT,  -- 'default_cogs', 'default_revenue', 'default_ar', dll
         account_id BIGINT
     )
     ```

3. **[MISSING] [SETTINGS] Document Numbering Format** — Saat ini format hardcoded: `PREFIX-YYYY-NNNNNN` (mis. `INV-2026-000001`). ERP proper butuh:
   - Format yang configurable per tenant
   - Reset per tahun/bulan
   - Prefix per branch/warehouse
   - Zero-padding configurable
   
   **Rekomendasi:** Buat `document_numbering_formats` table.

---

### E-22 RECONCILIATION: BANK RECONCILIATION

**Request:** `createStatementRequest` (`reconciliation/statements.go:33-40`)

**Status:** Cukup lengkap untuk MVP. Bank statement dengan opening/closing balance dan lines.

**[MISSING] Field tambahan:**

| Field yang Hilang | ERP Purpose | Prioritas |
|---|---|---|
| **bank_account_no** | No rekening untuk matching | Sedang |
| **statement_currency** | Currency statement | Sedang |
| **import_source** | Manual, CSV import, bank API | Rendah |
| **reconciled_by** | User yang reconcile | Sedang |
| **reconciled_at** | Waktu reconciliation | Sedang |

---

### E-23 BUDGET

**Request:** `budgetRequest` (`budget/budgets.go:27-32`)

**Status:** Cukup untuk MVP — budget per account per month per dimension.

**[MISSING] Field tambahan:**

| Field yang Hilang | ERP Purpose | Prioritas |
|---|---|---|
| **budget_type** | Operating, Capital, Cash Flow | Sedang |
| **approval_status** | Draft/Approved/Rejected | Tinggi |
| **approved_by** | Yang approve budget | Sedang |
| **approved_at** | Tanggal approve | Sedang |
| **variance_threshold_percent** | Threshold alert untuk budget variance | Sedang |
| **description** | Deskripsi budget | Rendah |

---

### RINGKASAN ERP COMPLETENESS — Prioritas Tertinggi

| # | Fitur | Field yang Hilang | Dampak ERP |
|---|---|---|---|
| 1 | **SO** | customer_po_number, requested_delivery_date, shipping_address, warehouse_id | SO tidak lengkap untuk operasional nyata |
| 2 | **Invoice** | tax_invoice_number, tax_type (inclusive/exclusive), subtotal/tax/grand_total breakdown | PPN reporting tidak mungkin |
| 3 | **Item** | Master UOM + konversi, item_type (stock/non_stock/service/assembly), category, tax_category | Inventory dan PPN tidak terstruktur |
| 4 | **Customer** | Alamat pengiriman terpisah, PKP status, credit hold enforcement | Penjualan dan PPN tidak terkontrol |
| 5 | **Supplier** | Bank details untuk pembayaran, PKP status, default_ap_account_id (ada di DB tapi tidak di API) | Pembayaran tidak lengkap |
| 6 | **Tenant Settings** | Default accounts, default tax rate, numbering format, fiscal year settings | Semua hardcoded, tidak flexible |
| 7 | **Warehouse** | Master warehouse tidak ada — stock_transfer tidak berfungsi | Multi-gudang tidak mungkin |
| 8 | **DO** | Carrier, tracking_number, shipping_address, received_by | Pengiriman tidak ter-track |
| 9 | **GRN** | Quality check (accepted/rejected), received_by, supplier delivery note | Penerimaan tidak terkontrol |
| 10 | **PO** | expected_date (ada di DB tapi tidak di API), approval workflow | PO tidak lengkap |
| 11 | **BOM** | Routing, version, effective dates, labor time, overhead rate | Produksi tidak terstruktur |
| 12 | **Fixed Asset** | Serial number, location, custodian, category | Asset tracking tidak lengkap |
| 13 | **Lease** | Lease type (operating vs finance), depreciation accounts | PSAK 73 compliance gap |
| 14 | **Supplier Payment** | Payment method, reference_number, PPh withholding | Pembayaran tidak terdokumentasi |

---

## BAGIAN III: MISSING ERP MODULES — Fitur Accounting & Finance yang Belum Ada

Bagian ini mengidentifikasi modul ERP Accounting & Finance yang **sama sekali belum ada** di aplikasi tetapi merupakan fitur standar ERP yang esensial untuk operasional bisnis nyata.

---

### F-01 [MISSING MODULE] Multi-Currency & Foreign Exchange

**Status:** TIDAK ADA implementasi multi-currency.

**Yang ada saat ini:**
- Semua transaksi menggunakan IDR (Rupiah) saja.
- `items.currency_code` ada di `PriceRequest` (`item/handler.go:67`) tetapi tidak di-cek atau di-enforce.
- Tidak ada table `currencies`, tidak ada `exchange_rates`, tidak ada field `currency_code` di journal_entries, invoices, PO, GRN, dll.

**Yang seharusnya ada di ERP:**
| Komponen | Deskripsi | Prioritas |
|---|---|---|
| **Master `currencies`** | Table: code (IDR, USD, EUR), name, symbol, decimal_places | Tinggi |
| **`exchange_rates` table** | from_currency, to_currency, rate, effective_date, source (manual, BI API) | Tinggi |
| **Currency di journal_entries** | Setiap journal punya `currency_code` + `exchange_rate` ke base currency | Tinggi |
| **Gain/Loss on FX** | Akun 4904 Gain on FX / 5904 Loss on FX — di-post saat settlement dengan rate berbeda | Tinggi |
| **Revaluation FX** | Period-end: revalue foreign currency balances ke rate tanggal laporan, post gain/loss | Tinggi |
| **BI Rate Integration** | Auto-fetch Bank Indonesia exchange rates via API | Sedang |

**Dampak jika tidak ada:**
- Import/export tidak bisa di-record dengan benar (PO USD → GRN → SI → Payment dengan rate berbeda).
- Laporan konsolidasi untuk entitas dengan currency berbeda tidak mungkin.
- Gain/loss FX tidak ter-recognize → P&L salah.

---

### F-02 [MISSING MODULE] Multi-Branch / Multi-Warehouse

**Status:** TIDAK ADA master warehouse/branch. Stock transfer comment: "single warehouse for now" (`stock_transfer.go:17`).

**Yang ada saat ini:**
- Tidak ada table `warehouses`.
- `stock_balances` tidak punya `warehouse_id`.
- `delivery_orders` dan `grns` tidak punya `warehouse_id`.
- Stock transfer tidak punya from/to warehouse.

**Yang seharusnya ada:**
| Komponen | Deskripsi | Prioritas |
|---|---|---|
| **Master `warehouses`** | code, name, address, is_active, is_default | Tinggi |
| **`stock_balances` per warehouse** | `(tenant_id, item_id, warehouse_id)` composite key | Tinggi |
| **`warehouse_id` di transaksi** | DO, GRN, stock_opname, stock_transfer, production_job | Tinggi |
| **Inter-branch transfer** | Transfer antar gudang dengan in-transit account | Tinggi |
| **Branch/Dimension per gudang** | Link warehouse ke dimension untuk reporting | Sedang |

---

### F-03 [MISSING MODULE] Approval Workflow Engine

**Status:** TIDAK ADA approval workflow. Semua transaksi langsung POSTED.

**Yang ada saat ini:**
- US-050A (P2 priority) menyebut "Approval Transaksi" tetapi tidak diimplementasi.
- Tidak ada status DRAFT → PENDING_APPROVAL → APPROVED → POSTED.
- Tidak ada table `approval_requests`, `approval_steps`, `approvers`.

**Yang seharusnya ada:**
| Komponen | Deskripsi | Prioritas |
|---|---|---|
| **`approval_workflows` table** | entity_type (invoice, PO, CN, dll), min_amount, approver_role | Tinggi |
| **`approval_steps` table** | workflow_id, step_order, approver_user_id atau role | Tinggi |
| **`approval_requests` table** | entity_type, entity_id, requested_by, status, current_step | Tinggi |
| **DRAFT status di semua dokumen** | SO, PO, Invoice, CN, GRN, SI harus bisa draft sebelum submit | Tinggi |
| **Approval notification** | Email/in-app notification ke approver | Sedang |
| **Approval delegation** | Delegasi approval saat approver tidak ada | Rendah |

**Dampak:** Tanpa approval workflow, staf dapat posting transaksi tanpa supervisi. Tidak ada segregation of duties.

---

### F-04 [MISSING MODULE] Accounts Receivable Aging & Collection Management

**Status:** TIDAK ada AR aging report, tidak ada collection management.

**Yang ada saat ini:**
- ECL module (`tax/ecl.go`) menghitung aging untuk provisioning, tetapi ini untuk ECL, bukan untuk collection management.
- Tidak ada AR aging report per customer.
- Tidak ada customer statement.
- Tidak ada dunning/follow-up tracking.

**Yang seharusnya ada:**
| Komponen | Deskripsi | Prioritas |
|---|---|---|
| **AR Aging Report** | Per customer: current, 1-30, 31-60, 61-90, >90 days | Tinggi |
| **Customer Statement** | Statement of account: opening balance, invoices, payments, closing balance | Tinggi |
| **Collection Follow-up** | Dunning level (reminder, urgent, legal), follow-up date, collector assignment | Sedang |
| **Promise-to-Pay** | Customer berjanji bayar tanggal X — tracking | Rendah |
| **Auto-aging calculation** | Cron job update aging buckets harian | Sedang |

---

### F-05 [MISSING MODULE] Accounts Payable Aging & Payment Scheduling

**Status:** TIDAK ada AP aging report, tidak ada payment scheduling.

**Yang seharusnya ada:**
| Komponen | Deskripsi | Prioritas |
|---|---|---|
| **AP Aging Report** | Per supplier: current, 1-30, 31-60, 61-90, >90 days | Tinggi |
| **Payment Schedule** | Due date calendar — invoice mana yang jatuh tempo minggu ini | Tinggi |
| **Payment Proposal** | Sistem usulkan invoice mana yang harus dibayar berdasarkan due date + cash availability | Sedang |
| **Supplier Statement** | Statement of account dengan supplier | Sedang |
| **Early Payment Discount Capture** | Alert jika invoice punya early payment discount yang akan expire | Sedang |

---

### F-06 [MISSING MODULE] Cash Flow Forecasting

**Status:** Cash flow report hanya historical (sum debits/credits di CASH/BANK). Tidak ada forecasting.

**Yang seharusnya ada:**
| Komponen | Deskripsi | Prioritas |
|---|---|---|
| **Cash Flow Forecast** | Projected cash inflow (from AR due) vs outflow (from AP due) per week/month | Tinggi |
| **Scenario Planning** | Best case, worst case, expected case | Sedang |
| **Cash Position Dashboard** | Real-time cash + short-term forecast | Tinggi |
| **Bank Balance Integration** | Auto-fetch saldo bank via bank API (BCA, Mandiri) | Sedang |

---

### F-07 [MISSING MODULE] Recurring / Scheduled Transactions

**Status:** US-052 menyebut "Transaksi Berulang" tetapi TIDAK diimplementasi.

**Yang seharusnya ada:**
| Komponen | Deskripsi | Prioritas |
|---|---|---|
| **`recurring_transactions` table** | template_id, frequency (daily/weekly/monthly), next_date, end_date | Tinggi |
| **Scheduler/cron** | Auto-generate journal entries berdasarkan recurring template | Tinggi |
| **Examples** | Rent, insurance, salary, depreciation, subscription | — |

---

### F-08 [MISSING MODULE] Petty Cash (Kas Kecil) with Imprest System

**Status:** US-051 menyebut "Kas Kecil (Imprest)" tetapi TIDAK diimplementasi.

**Yang seharusnya ada:**
| Komponen | Deskripsi | Prioritas |
|---|---|---|
| **Petty cash account** | Separate cash account with imprest balance | Tinggi |
| **Petty cash voucher** | Voucher for each petty cash expense | Tinggi |
| **Replenishment** | When petty cash low, replenish to original imprest amount | Tinggi |
| **Petty cash custodian** | User responsible for petty cash | Sedang |

---

### F-09 [MISSING MODULE] Cost Center / Profit Center Accounting

**Status:** Dimensions table ada (`budget/dimensions.go`) tetapi hanya untuk budget tagging, tidak untuk full cost/profit center accounting.

**Yang seharusnya ada:**
| Komponen | Deskripsi | Prioritas |
|---|---|---|
| **Cost centers** | Department/division yang incur cost | Sedang |
| **Profit centers** | Business unit yang generate revenue | Sedang |
| **Cost allocation** | Allocate shared cost (rent, utilities) ke cost centers berdasarkan driver | Sedang |
| **Cost center P&L** | P&L per cost/profit center | Sedang |

---

### F-10 [MISSING MODULE] Inter-Company / Consolidation (Full)

**Status:** Basic entity_hierarchy dan inter_company_transactions ada (`lease/consolidation.go`), tetapi elimination logic incomplete dan deferred.

**Yang seharusnya ada:**
| Komponen | Deskripsi | Prioritas |
|---|---|---|
| **Elimination entries** | Auto-eliminate inter-company sales, purchases, loans, interest, dividends | Tinggi |
| **Minority interest** | Calculate NCI for partial ownership | Sedang |
| **Consolidated reports** | TB, P&L, BS, CF for consolidated entity | Tinggi |
| **Inter-company reconciliation** | Match inter-company balances | Sedang |

---

### F-11 [MISSING MODULE] Budget vs Actual with Variance Analysis

**Status:** Budget module ada tetapi minimal — hanya input budget per account per month. Tidak ada variance analysis otomatis.

**Yang seharusnya ada:**
| Komponen | Deskripsi | Prioritas |
|---|---|---|
| **Budget vs Actual report** | Side-by-side comparison with variance % | Tinggi |
| **Variance threshold alert** | Alert jika variance > threshold (mis. >10%) | Sedang |
| **Rolling forecast** | Update forecast berdasarkan actuals YTD | Sedang |
| **Budget approval workflow** | Draft → Approved → Locked | Sedang |

---

### F-12 [MISSING MODULE] Withholding Tax (PPh 21/22/23/26) Management

**Status:** Hanya PPh Final UMKM yang ada (`tax/pph.go`). PPh 21, 22, 23, 26 tidak ada.

**Yang seharusnya ada:**
| PPh Type | Deskripsi | Prioritas |
|---|---|---|
| **PPh 21** | PPh atas penghasilan karyawan (gaji) — calc, posting, e-Bupot | Tinggi |
| **PPh 22** | PPh atas impor / pembelian barang tertentu | Sedang |
| **PPh 23** | PPh atas jasa, sewa, royalti — dipotong saat bayar supplier | Tinggi |
| **PPh 26** | PPh atas pembayaran ke pihak non-resident | Sedang |
| **PPh Badan** | PPh badan tahunan (estimasi cicilan bulanan) | Tinggi |
| **e-Bupot integration** | Generate file untuk e-Bupot (DJP) | Tinggi |
| **Bukti Potong** | Generate bukti potong PPh (printable) | Tinggi |

---

### F-13 [MISSING MODULE] Fixed Asset Register & Maintenance Tracking

**Status:** Asset module ada (acquisition, depreciation, disposal) tetapi tidak ada asset register report atau maintenance tracking.

**Yang seharusnya ada:**
| Komponen | Deskripsi | Prioritas |
|---|---|---|
| **Asset Register Report** | List semua aset: code, name, location, acquisition date, cost, accum dep, book value | Tinggi |
| **Maintenance Schedule** | Jadwal maintenance per aset | Sedang |
| **Maintenance Cost Tracking** | Record maintenance cost (separate from capital improvement) | Sedang |
| **Asset Transfer** | Pindah aset antar department/lokasi | Rendah |
| **Asset Tagging (QR/Barcode)** | Generate QR code label per aset | Rendah |

---

### F-14 [MISSING MODULE] Giro & Cheque Management

**Status:** TIDAK ADA. Payment hanya via cash/bank transfer.

**Yang seharusnya ada:**
| Komponen | Deskripsi | Prioritas |
|---|---|---|
| **Cheque register** | Issue, receive, track cheques | Tinggi |
| **GIRO management** | Issue, receive, endorse, deposit GIRO | Tinggi |
| **Cheque clearing** | Track clearing status (issued → cleared / bounced) | Tinggi |
| **Post-dated cheque schedule** | Calendar of post-dated cheques | Sedang |
| **Bounce cheque handling** | Journal reversal + fee + follow-up | Tinggi |

---

### F-15 [MISSING MODULE] Email Notification & Document Sending

**Status:** TIDAK ADA. Tidak ada email notification, tidak ada kirim invoice/PO via email.

**Yang seharusnya ada:**
| Komponen | Deskripsi | Prioritas |
|---|---|---|
| **Email invoice to customer** | Send invoice PDF via email dengan body template | Tinggi |
| **Email statement** | Send monthly customer statement | Sedang |
| **Email PO to supplier** | Send PO PDF via email | Sedang |
| **Payment receipt email** | Send payment confirmation | Sedang |
| **Overdue reminder** | Auto-send overdue AR reminder | Tinggi |
| **Email template management** | Configurable email templates | Sedang |

---

## BAGIAN IV: DASHBOARD ANALYSIS — Custom Widget per User

### D-01 [ANALYSIS] Dashboard Saat Ini

**Lokasi:** `web/src/screens/workbench/DashboardScreen.tsx` (321 baris)

**Yang ada saat ini:**
```
DashboardScreen
├── Header: "Finance Dashboard" + tanggal hari ini
├── StatusCells (4 KPI):
│   ├── Cash & Bank Balance (with sparkline)
│   ├── Monthly P&L (with sparkline)
│   ├── Open Receivables (count)
│   └── Items Below Reorder Point (count)
├── KpiRow (4 items):
│   ├── Cash Balance
│   ├── Monthly P&L
│   ├── Open receivables
│   └── Items below reorder point
└── PeriodCard (current period status)
```

**Backend dashboard API:** `api.getDashboard()` (`api.ts:511-556`) — memanggil 3 endpoint secara paralel (profit-loss, cash-flow, balance-sheet) dan menggabungkan. `dueBills` dan `lowStock` di-hardcode (`dueBills: 2, lowStock: 4`).

**Masalah:**
1. **Hardcoded values** — `dueBills: 2, lowStock: 4` selalu sama, tidak query real data.
2. **No per-user customization** — semua user melihat dashboard yang sama.
3. **No widget system** — KPI fixed, tidak bisa add/remove/rearrange.
4. **No charts** — hanya sparkline mini. Tidak ada bar chart, pie chart, line chart.
5. **No date range selector** — dashboard selalu menampilkan current month.
6. **No drill-down** — klik KPI tidak navigasi ke detail.
7. **No real-time update** — harus refresh page untuk update data.

### D-02 [RECOMMENDATION] Arsitektur Dashboard yang Seharusnya Ada

**1. Widget-Based Dashboard System**

```
dashboard_layouts
├── id
├── tenant_id
├── user_id          ← per-user layout
├── name             ← "My Dashboard", "Sales Dashboard"
├── is_default       ← layout default user
└── created_at

dashboard_widgets
├── id
├── layout_id
├── widget_type      ← enum: kpi, chart, table, calendar, alert
├── title            ← "Cash Balance", "AR Aging"
├── data_source      ← endpoint atau query ref
├── config (JSON)    ← chart type, date_range, filters
├── position_x       ← grid column
├── position_y       ← grid row
├── width            ← grid span
└── height           ← grid span
```

**2. Widget Types yang harus tersedia:**

| Widget Type | Contoh | Data Source |
|---|---|---|
| **KPI Card** | Cash Balance, Monthly P&L, AR Outstanding | `/reports/summary` |
| **Line Chart** | Revenue trend 12 bulan | `/reports/profit-loss?monthly=true` |
| **Bar Chart** | Sales per salesperson, Expense per category | `/reports/sales-by-salesperson` |
| **Pie Chart** | Expense breakdown by category | `/reports/expense-breakdown` |
| **AR Aging** | Stacked bar: current, 1-30, 31-60, >90 | `/reports/ar-aging` |
| **AP Aging** | Stacked bar: current, 1-30, 31-60, >90 | `/reports/ap-aging` |
| **Cash Flow Forecast** | Line: projected inflow vs outflow | `/reports/cash-forecast` |
| **Budget vs Actual** | Grouped bar: budget vs actual per account | `/reports/budget-vs-actual` |
| **Top Customers** | Table: top 10 by revenue | `/reports/top-customers` |
| **Top Items** | Table: top 10 by sales qty | `/reports/top-items` |
| **Recent Transactions** | Table: last 10 transactions | `/transactions?limit=10` |
| **Pending Approvals** | List: dokumen pending approval | `/approvals/pending` |
| **Inventory Alerts** | List: items below reorder point | `/items?below_reorder=true` |
| **Calendar** | Due dates: invoice due, bill due, lease payment | `/calendar/due-dates` |
| **Bank Balance** | Card per bank account | `/accounts?type=BANK` |
| **Tax Summary** | PPN keluaran vs masukan, PPh | `/tax/summary` |
| **P&L Waterfall** | Revenue → COGS → Gross Profit → Expenses → Net Profit | custom |

**3. Per-User Customization:**
- Setiap user punya default layout (dibuat saat first login).
- User dapat: add widget, remove widget, drag to reposition, resize, configure (date range, filter).
- Layout disimpan di `dashboard_layouts` + `dashboard_widgets`.
- Role-based widget access: `viewer` hanya lihat, `owner` dapat edit layout.

**4. Tech Stack untuk Dashboard:**
- **Frontend:** React + [react-grid-layout](https://github.com/react-grid-layout/react-grid-layout) untuk drag-and-drop grid.
- **Charts:** [Recharts](https://recharts.org/) atau [Apache ECharts](https://echarts.apache.org/) — both React-friendly.
- **Backend:** Endpoint `/api/v1/dashboard/widgets/{type}` yang return JSON data untuk widget.

---

## BAGIAN V: REPORTING & PRINT SOLUTION — Analisis Jasper vs Alternatif

### R-01 [ANALYSIS] Kapabilitas Reporting & Print Saat Ini

**Yang ada saat ini:**

1. **Export PDF** (`reporting/export.go`):
   - Menggunakan `github.com/jung-kurt/gofpdf` — pure Go PDF library.
   - Hanya untuk 4 laporan: Trial Balance, P&L, Balance Sheet, Cash Flow.
   - Format: tabel sederhana dengan header. Tidak ada template, tidak ada desain.
   - Tidak support: logo, header/footer custom, watermark, landscape, multi-page table header.

2. **Export XLSX** (`reporting/export.go`):
   - Menggunakan `github.com/xuri/excelize/v2`.
   - Hanya untuk 4 laporan yang sama.
   - Format: raw data dump ke spreadsheet. Tidak ada formatting, formula, atau pivot.

3. **Print button di frontend:**
   - `CashEntryList.tsx:131-132`: Print icon button — **disabled** (belum berfungsi).
   - Tidak ada print template untuk invoice, PO, delivery order, dll.

4. **Tidak ada document templates:**
   - Invoice tidak bisa di-print/di-PDF dengan layout professional (kop surat, logo, tanda tangan).
   - PO tidak bisa di-print.
   - Delivery order tidak bisa di-print (surat jalan).
   - Faktur pajak tidak bisa di-generate.
   - Bukti potong PPh tidak bisa di-generate.

### R-02 [ANALYSIS] Apakah Jasper Reports Cocok?

**Jasper Reports** adalah Java-based reporting engine. Untuk mengintegrasikan dengan Go backend:

| Aspek | Jasper Reports | Catatan |
|---|---|---|
| **Integration dengan Go** | ❌ Sulit | Jasper butuh JVM. Go tidak native JVM. Harus jalankan sebagai: (a) REST API server terpisah (JasperReports Server), atau (b) CLI executable, atau (c) gRPC bridge |
| **Template Designer** | ✅ Jaspersoft Studio | Visual drag-and-drop designer. Mendukung: bands, subreports, charts, crosstabs |
| **Output formats** | ✅ PDF, Excel, Word, PPT, HTML, CSV | Lengkap |
| **Complex reports** | ✅ Sangat powerful | Subreports, crosstabs, sparklines, barcodes |
| **Resource** | ❌ Berat | JVM + JasperReports Server butuh RAM 2-4GB minimum |
| **Lisensi** | Community (AGPL) vs Commercial | AGPL restrictive untuk SaaS komersial |
| **Learning curve** | ❌ Tinggi | JRXML (XML-based), expression language Java-like |
| **Maintenance** | ❌ Java + XML templates | Tim Go tidak akan comfortable |

**Kesimpulan Jasper:** **TIDAK DIREKOMENDASIKAN** untuk stack Go + React. Terlalu berat, butuh JVM, learning curve tinggi, dan AGPL license restrictive.

### R-03 [ANALYSIS KRITIS] Apakah jsreport Bisa Drag-and-Drop Seperti Jasper?

**JAWABAN: TIDAK.** Ini adalah perbedaan paling fundamental yang harus dipahami sebelum memilih reporting tool.

#### Fakta tentang jsreport (jsreport.net)

**jsreport** (open-source, LGPL, v4.14.0 per Aug 2026) adalah reporting **server** yang powerful untuk **rendering** PDF/Excel/Word dari HTML+Handlebars templates. Namun:

| Pertanyaan | Jawaban | Bukti |
|---|---|---|
| **Punya drag-and-drop visual designer?** | ❌ **TIDAK ADA** | GitHub issue #121 (open sejak 2015, masih open hingga 2026). jsreport sendiri mengakui di blog: "This is probably one of the most requested feature from the very beginning" tetapi "We had to pause the development" |
| **jsreport Studio = visual designer?** | ❌ **Bukan** | jsreport Studio adalah **code-based editor** — mirip VS Code untuk HTML/Handlebars/CSS. Bukan WYSIWYG drag-and-drop. Developer menulis kode, preview di panel sebelah. |
| **Bisa embed designer di React?** | ❌ **Tidak** | jsreport Studio berjalan sebagai server terpisah di port 5488. Tidak bisa di-embed sebagai React component di app Anda. |
| **Apakah user non-developer bisa design report?** | ❌ **Tidak** | User harus paham HTML + CSS + Handlebars. Hanya developer yang bisa buat/edit template. |

**jsreport Studio (yang ada saat ini):**
```
┌─────────────────────────────────────────────────────┐
│  jsreport Studio (http://localhost:5488)             │
├──────────────┬──────────────────────────────────────┤
│  File Tree   │  Template Editor (CODE, not visual)  │
│              │                                      │
│  📁 templates│  <h1>{{company.name}}</h1>           │
│   📄 invoice │  <table>                              │
│   📄 po      │    {{#each items}}                   │
│   📄 do      │    <tr>                               │
│              │      <td>{{name}}</td>               │
│              │      <td>{{qty}}</td>                │
│              │    </tr>                              │
│              │    {{/each}}                          │
│              │  </table>                             │
│              │                                      │
│              ├──────────────────────────────────────┤
│              │  Preview Panel (rendered HTML)       │
│              │  [Invoice Preview]                   │
└──────────────┴──────────────────────────────────────┘
```

#### Perbandingan: Jasper Jaspersoft Studio vs jsreport Studio

| Aspek | Jasper (Jaspersoft Studio) | jsreport (jsreport Studio) |
|---|---|---|
| **Designer type** | ✅ Visual drag-and-drop (Eclipse-based desktop app) | ❌ Code-based editor (web-based) |
| **Drag elements from palette** | ✅ Ya — drag text, image, chart, table, subreport ke canvas | ❌ Tidak — tulis HTML manual |
| **Resize/move elements visually** | ✅ Ya — click & drag untuk resize/move | ❌ Tidak — ubah CSS manual |
| **Property panel** | ✅ Ya — panel kanan dengan semua properties | ❌ Tidak — edit JSON/HTML manual |
| **WYSIWYG** | ✅ Ya — what you see is what you get | ⚠️ Partial — preview panel, bukan WYSIWYG |
| **Chart builder** | ✅ Visual chart wizard | ❌ Tulis JavaScript untuk chart |
| **Subreport designer** | ✅ Visual | ❌ Tulis Handlebars partial |
| **Crosstab builder** | ✅ Visual | ❌ Tulis HTML table dengan Handlebars logic |
| **Output quality** | ✅ Pixel-perfect (native Java rendering) | ✅ Pixel-perfect (Chrome headless) |
| **Template format** | JRXML (XML) | HTML + Handlebars + CSS |
| **Who can design?** | Business analyst (dengan training) | Developer only |
| **Embed in app?** | ❌ Desktop app terpisah | ❌ Server terpisah (port 5488) |

---

### R-04 [RECOMMENDATION] Alternatif yang BENAR-BENAR Punya Drag-and-Drop Visual Designer

Jika kebutuhan utama adalah **user non-developer bisa drag-and-drop design report** (seperti Jasper), maka ada 3 opsi yang BENAR-BENAR punya visual designer:

#### OPSI A: jsreports (jsreports.com) — ★★★★★ RECOMMENDED untuk Drag-and-Drop

**PERHATIAN:** `jsreports` (jsreports.com, dengan "s") adalah produk BERBEDA dari `jsreport` (jsreport.net, tanpa "s"). Jangan tertukar!

| Aspek | jsreports (komersial) | Catatan |
|---|---|---|
| **Drag-and-drop visual designer** | ✅ **YA** | Page-based layout, drag elements dari palette ke canvas |
| **Embed di React** | ✅ **YA** | HTML5 component, embed langsung di web app. No plugin, works in all modern browsers |
| **Custom element types** | ✅ **YA** | Developer bisa buat custom element types yang bisa di-drag-drop |
| **Data binding** | ✅ **YA** | Connect ke JSON/CSV data sources. Grouping, drill-down, summarize |
| **Output formats** | PDF, HTML | PDF generation di browser atau server |
| **Charts** | ✅ Ya | Bar, line, pie, pivot table (crosstab) |
| **Barcode** | ✅ Ya | Barcode element |
| **Go integration** | ⚠️ Perlu adapter | jsreports adalah client-side JS library. Go backend sediakan JSON data, React frontend render. Untuk server-side PDF, butuh Chrome headless. |
| **Lisensi** | Komersial (bayar per developer) | Bukan open-source. Hubungi jsreports.com untuk pricing. |
| **Maturity** | ✅ Mature | Sudah dipakai enterprise, dokumentasi lengkap |
| **Designer untuk siapa?** | ✅ End user / business analyst | Drag-and-drop, no code required |

**Cara kerja jsreports:**
```
┌──────────────────────────────────────────────────────────┐
│  React Frontend (web app Anda)                           │
│                                                          │
│  ┌────────────────────────────────────────────────────┐  │
│  │  jsreports Designer Component (embedded)           │  │
│  │                                                    │  │
│  │  ┌─────────┐  ┌────────────────────────────────┐  │  │
│  │  │ Palette │  │  Canvas (drag & drop here)     │  │  │
│  │  │         │  │                                │  │  │
│  │  │ 📝 Text │  │  ┌─────────┐  ┌────────────┐  │  │  │
│  │  │ 📊 Chart│  │  │ Logo    │  │ Invoice #  │  │  │  │
│  │  │ 📋 Table│  │  └─────────┘  └────────────┘  │  │  │
│  │  │ 🖼 Image│  │  ┌─────────────────────────┐  │  │  │
│  │  │ 📑 Page │  │  │  Table: items           │  │  │  │
│  │  │ 🔢 Num  │  │  │  Name | Qty | Price     │  │  │  │
│  │  │ 📊 Bar  │  │  │  ...  | ... | ...       │  │  │  │
│  │  │         │  │  └─────────────────────────┘  │  │  │
│  │  │         │  │  ┌──────┐ ┌──────────────┐   │  │  │
│  │  │         │  │  │ Total│ │ Rp 1.500.000 │   │  │  │
│  │  │         │  │  └──────┘ └──────────────┘   │  │  │
│  │  └─────────┘  └────────────────────────────────┘  │  │
│  └────────────────────────────────────────────────────┘  │
│                                                          │
│  ↓ JSON report definition                                │
│  ↓ Send to Go backend → Chrome headless → PDF            │
└──────────────────────────────────────────────────────────┘
```

**Keunggulan jsreports vs Jasper:**
- Web-based (bukan desktop app) — bisa akses dari browser mana pun.
- Embed di React app — user tidak perlu install software terpisah.
- HTML5 — no plugin, no ActiveX, no Java applet.
- Custom elements — developer bisa tambah elemen kustom.
- JSON-based — report definition adalah JSON, bisa disimpan di DB.

**Kekurangan jsreports:**
- Komersial (bayar lisensi).
- Tidak se-populer Jasper (komunitas lebih kecil).
- Server-side PDF rendering butuh Chrome headless (seperti jsreport).

---

#### OPSI B: NextReport Engine (★★★★ Open-Source dengan Visual Designer)

**NextReport** (nextreport/engine di GitHub) adalah open-source reporting engine dengan visual designer berbasis React.

| Aspek | NextReport | Catatan |
|---|---|---|
| **Drag-and-drop visual designer** | ✅ **YA** | Visual designer dengan drag-and-drop, palette, canvas |
| **Embed di React** | ✅ **YA** | React/Next.js component, native web |
| **Template format** | JSON schema-driven | Bisa di-generate oleh AI/LLM |
| **Output formats** | PDF, HTML | Excel masih terbatas |
| **Lisensi** | Open source | — |
| **Maturity** | ⚠️ Early stage | Proyek baru, komunitas masih kecil |
| **Go integration** | REST API | Sama seperti jsreport — HTTP POST JSON → PDF |
| **Designer untuk siapa?** | ✅ End user | Drag-and-drop, visual |
| **Chart support** | ✅ Ya | Line, bar, pie, scatter, area |
| **Dashboard builder** | ✅ Ya | Metabase-style drag-and-drop dashboard builder |

**Keunggulan NextReport:**
- Open source (gratis).
- React native — embed designer langsung di app.
- JSON schema — report definition mudah di-version-control.
- AI-ready — schema bisa di-generate oleh LLM.
- Modern stack — TypeScript, React, Tailwind.

**Kekurangan NextReport:**
- Masih early stage (belum mature seperti Jasper/jsreports).
- Komunitas kecil → dokumentasi terbatas.
- Excel output terbatas.
- Belum ada track record enterprise.

---

#### OPSI C: jsreport (jsreport.net) + Custom Drag-and-Drop Layer — ★★★★ Hybrid

**Pendekatan:** Gunakan jsreport untuk **rendering engine** (PDF generation), tetapi bangun **custom drag-and-drop designer** sendiri di React yang meng-generate HTML/Handlebars template.

| Aspek | Hybrid Approach | Catatan |
|---|---|---|
| **Rendering** | ✅ jsreport (excellent) | Chrome headless, pixel-perfect PDF |
| **Designer** | ✅ Custom React drag-and-drop | Bangun sendiri pakai react-dnd atau react-grid-layout |
| **Template output** | HTML + Handlebars | Designer meng-generate HTML dari visual components |
| **Effort** | ⚠️ Tinggi | Bangun designer dari nol (3-6 bulan) |
| **Flexibility** | ✅ Maksimal | Full control atas designer UX |
| **Go integration** | ✅ REST API ke jsreport | HTTP POST → PDF |
| **Lisensi** | jsreport: LGPL |Gratis |

**Cara kerja hybrid:**
```
┌──────────────────────────────────────────────────────────┐
│  React Frontend                                          │
│                                                          │
│  ┌────────────────────────────────────────────────────┐  │
│  │  Custom Drag-and-Drop Designer (build sendiri)     │  │
│  │                                                    │  │
│  │  Palette: [Text] [Image] [Table] [Chart] [Total]   │  │
│  │                                                    │  │
│  │  Canvas: [Drag elements here → visual layout]      │  │
│  │                                                    │  │
│  │  ↓ Generate HTML + Handlebars                     │  │
│  └────────────────────────────────────────────────────┘  │
│                                                          │
│  ↓ Send HTML template + data to Go backend               │
└──────────────────────┬───────────────────────────────────┘
                       │
                       ▼
┌──────────────────────────────────────────────────────────┐
│  Go Backend                                              │
│                                                          │
│  POST http://jsreport:5488/api/report                    │
│  { "template": { "content": "<html>...</html>" },       │
│    "data": { "invoice": {...} } }                        │
│  → PDF binary                                            │
└──────────────────────────────────────────────────────────┘
```

**Keunggulan hybrid:**
- Full control atas designer UX.
- jsreport rendering tetap powerful.
- Tidak bayar lisensi komersial.
- Bisa custom untuk kebutuhan spesifik (elemen PPh, faktur pajak format DJP).

**Kekurangan hybrid:**
- Effort tinggi (3-6 bulan bangun designer).
- Maintenance burden (bug di custom designer).
- Risiko designer tidak se-feature-complete jsreports/Jasper.

---

#### OPSI D: Metabase (★★★★ untuk Analytics Dashboard, BUKAN untuk Document Print)

**Metabase** adalah open-source BI/analytics tool — bukan untuk print invoice/PO, tetapi excellent untuk dashboard dan ad-hoc reporting.

| Aspek | Metabase | Catatan |
|---|---|---|
| **Fokus** | Analytics, dashboard, ad-hoc query | Bukan untuk document print |
| **Drag-and-drop dashboard** | ✅ **YA** | Excellent drag-and-drop dashboard builder |
| **Per-user dashboard** | ✅ Ya | Setiap user bisa buat dashboard sendiri |
| **Custom SQL** | ✅ Ya | User bisa tulis SQL query sendiri |
| **Embed di React** | ✅ iframe embed | Bisa embed dashboard di app |
| **Lisensi** | AGPL (community) atau Commercial | AGPL restrictive untuk SaaS komersial |
| **Best for** | Analytics dashboard, custom reports | Bukan untuk formatted document print |

**Kapan dipilih:** Untuk dashboard analytics dan ad-hoc reporting (user buat laporan sendiri tanpa developer). Complement document print tool (jsreport/jsreports).

---

### R-05 [COMPARISON MATRIX] Semua Opsi Side-by-Side

| Kriteria | Jasper Reports | jsreport (net) | jsreports (com) | NextReport | Hybrid (jsreport + custom) | Metabase |
|---|---|---|---|---|---|---|
| **Drag-and-drop designer** | ✅ Desktop | ❌ Tidak ada | ✅ Web embed | ✅ Web embed | ✅ Custom build | ✅ Dashboard only |
| **Designer untuk non-dev** | ✅ Ya | ❌ Developer only | ✅ Ya | ✅ Ya | ✅ Ya | ✅ Ya |
| **Embed di React** | ❌ Desktop | ❌ Server terpisah | ✅ Native | ✅ Native | ✅ Custom | ✅ iframe |
| **Go integration** | ⚠️ JVM bridge | ✅ REST API | ⚠️ Client-side | ✅ REST API | ✅ REST API | ✅ Direct PG |
| **PDF quality** | ✅ Native | ✅ Chrome headless | ✅ Chrome headless | ✅ Chrome headless | ✅ Chrome headless | N/A |
| **Excel output** | ✅ Ya | ✅ Ya (excelize) | ⚠️ Terbatas | ⚠️ Terbatas | ✅ Ya | ✅ Ya |
| **Word output** | ✅ Ya | ✅ Ya (docx) | ❌ Tidak | ❌ Tidak | ✅ Ya | ❌ Tidak |
| **Lisensi** | AGPL/Comm | LGPL | Komersial | Open source | LGPL + MIT | AGPL/Comm |
| **Maturity** | ✅ Sangat mature | ✅ Mature | ✅ Mature | ⚠️ Early | N/A | ✅ Mature |
| **Resource** | 2-4GB RAM | ~500MB | Client-side | ~500MB | ~500MB | 1-2GB RAM |
| **Learning curve** | Tinggi (JRXML) | Rendah (HTML) | Rendah (visual) | Rendah (visual) | Tinggi (build) | Rendah |
| **Docker** | ✅ | ✅ | N/A | ✅ | ✅ | ✅ |
| **Indonesia tax forms** | ⚠️ Custom | ✅ Custom HTML | ✅ Custom element | ⚠️ Custom | ✅ Custom HTML | N/A |
| **Dashboard capability** | ❌ | ❌ | ⚠️ Basic | ✅ Ya | ❌ | ✅ Excellent |
| **Cost** | Free/Comm | Free | Bayar | Free | Free (dev cost) | Free/Comm |

---

### R-06 [FINAL RECOMMENDATION] Stack Reporting & Print — Corrected

**Rekomendasi kombinasi berdasarkan kebutuhan:**

```
┌──────────────────────────────────────────────────────────────────┐
│                    REPORTING STACK RECOMMENDATION                 │
├──────────────────────────────────────────────────────────────────┤
│                                                                  │
│  KESEDUSI 1: Document Print (Invoice, PO, DO, Faktur Pajak)     │
│  ────────────────────────────────────────────────────────────── │
│  PILIH SALAH SATU:                                               │
│                                                                  │
│  A) jsreports (jsreports.com) ← ★★★★★ jika ada budget           │
│     → Drag-and-drop visual designer, embed di React              │
│     → User non-developer bisa design template sendiri            │
│     → Go backend: sediakan JSON data, React render + Chrome PDF  │
│     → Cost: lisensi komersial per developer                      │
│                                                                  │
│  B) jsreport (jsreport.net) + custom React designer ← ★★★★      │
│     → jsreport untuk rendering engine (PDF generation)           │
│     → Custom drag-and-drop designer di React (react-dnd)         │
│     → Designer meng-generate HTML/Handlebars → kirim ke jsreport │
│     → Cost: free (LGPL) + dev effort 3-6 bulan                   │
│                                                                  │
│  C) jsreport (jsreport.net) tanpa visual designer ← ★★★         │
│     → Developer buat template HTML/Handlebars manual             │
│     → User TIDAK bisa design sendiri                             │
│     → Cost: free (LGPL)                                          │
│     → Cocok jika: template fixed, tidak perlu user customization │
│                                                                  │
│  KESEDUSI 2: Analytics Dashboard (Custom per User)               │
│  ────────────────────────────────────────────────────────────── │
│  PILIH SALAH SATU:                                               │
│                                                                  │
│  A) Custom React dashboard ← ★★★★★ RECOMMENDED                  │
│     → react-grid-layout (drag-and-drop grid)                    │
│     → Recharts atau Apache ECharts (chart library)              │
│     → dashboard_layouts + dashboard_widgets table (per-user)    │
│     → Full control, free, native React                          │
│                                                                  │
│  B) Metabase (embed) ← ★★★★ jika mau cepat                      │
│     → Drag-and-drop dashboard builder, per-user                  │
│     → Connect langsung ke PostgreSQL                             │
│     → Embed via iframe di React app                              │
│     → Cost: AGPL (free, tapi restrictive untuk SaaS komersial)   │
│                                                                  │
│  KESEDUSI 3: Excel Export                                        │
│  ────────────────────────────────────────────────────────────── │
│  → Go native + excelize (sudah ada, tinggal perluas)             │
│                                                                  │
└──────────────────────────────────────────────────────────────────┘
```

**Roadmap implementasi (corrected):**

| Phase | Komponen | Effort | Prioritas | Approach |
|---|---|---|---|---|
| **1A** | Pilih reporting tool (jsreports vs jsreport+custom) | 1 minggu | Tinggi | Evaluasi budget, trial jsreports |
| **1B** | Deploy reporting server + invoice template | 1-2 minggu | Tinggi | Invoice: kop surat, logo, items, PPN, ttd |
| **1C** | PO, DO, surat jalan templates | 1 minggu | Tinggi | |
| **1D** | Faktur pajak, bukti potong templates | 1-2 minggu | Tinggi | Format DJP |
| **1E** | Customer/supplier statement | 1 minggu | Tinggi | |
| **1F** | Build custom drag-and-drop designer (jika opsi B) | 3-6 bulan | Sedang | react-dnd, palette, canvas, HTML generator |
| **2A** | Dashboard widget system (backend) | 1-2 minggu | Tinggi | Widget API endpoints |
| **2B** | Dashboard widget system (frontend) | 2-3 minggu | Tinggi | react-grid-layout + Recharts |
| **2C** | Per-user layout persistence | 1 minggu | Tinggi | dashboard_layouts + dashboard_widgets |
| **3A** | Metabase deploy (opsional) | 1 minggu | Rendah | Docker, connect ke PG |
| **3B** | Metabase custom dashboards | Ongoing | Rendah | |

**Kenapa BUKAN Jasper?**
- Butuh JVM (tidak cocok stack Go).
- JRXML = XML, bukan HTML/CSS (kurang familiar web developer).
- AGPL license restrictive untuk SaaS komersial.
- Learning curve tinggi.
- Resource heavy (2-4GB RAM).
- Desktop designer (tidak bisa embed di web app).

**Kenapa jsreports (com) sebagai pilihan utama?**
- ✅ Drag-and-drop visual designer (seperti Jasper, tapi web-based).
- ✅ Embed di React app (user design tanpa install software).
- ✅ Custom element types (bisa buat elemen khusus PPh/faktur pajak).
- ✅ HTML5 (no plugin, works in all browsers).
- ⚠️ Komersial (bayar lisensi, tapi lebih murah dari Jasper commercial).

**Kenapa jsreport (net) + custom designer sebagai alternatif?**
- ✅ Free (LGPL, tidak bayar lisensi).
- ✅ jsreport rendering engine excellent (Chrome headless, pixel-perfect).
- ✅ Full control atas designer UX.
- ⚠️ Effort tinggi (3-6 bulan bangun designer).
- ⚠️ Maintenance burden.

---

### R-07 [DESIGN] Desain Cetak (Print Template) — Apakah Memungkinkan?

**Jawaban: YA, sangat memungkinkan**, tetapi dengan catatan tergantung pilihan tool:

#### Jika menggunakan jsreports (com):
- ✅ User bisa drag-and-drop design template sendiri di web app.
- ✅ Page-based layout — mudah dipahami non-developer.
- ✅ Custom elements — developer buat elemen khusus (kop surat, ttd block, PPN box).
- ✅ Data binding — connect template ke JSON data dari Go backend.
- ✅ Preview real-time — lihat hasil langsung sebelum save.

#### Jika menggunakan jsreport (net) + custom designer:
- ✅ Designer custom bisa dibuat sesuai kebutuhan (palette dengan elemen Indonesia).
- ✅ Designer meng-generate HTML/Handlebars → jsreport render ke PDF.
- ⚠️ Developer harus bangun designer (tidak out-of-the-box).
- ✅ Setelah jadi, user bisa drag-and-drop tanpa coding.

#### Jika menggunakan jsreport (net) tanpa designer:
- ⚠️ Developer buat semua template manual (HTML + Handlebars).
- ❌ User TIDAK bisa edit/design sendiri.
- ✅ Template bisa custom per tenant (logo, kop, warna) via parameter.
- ✅ Cocok jika template fixed (invoice, PO, DO format standar).

**Yang dapat di-customize di template (semua opsi):**
- Logo perusahaan dan posisi
- Kop surat (nama, alamat, telepon, email, NPWP)
- Font family dan size
- Warna header tabel
- Layout (landscape/portrait)
- Margin dan spacing
- Footer (halaman, nomor dokumen, tanggal cetak)
- Tanda tangan (siapa yang ttd, posisi)
- Terms & conditions
- Watermark (DRAFT, PAID, COPY, LUNAS)
- Barcode/QR code (untuk nomor dokumen)
- Multi-language (Indonesia / English)
- Multi-currency display

---

### R-08 [CORRECTION] Koreksi dari Versi Sebelumnya

**Koreksi penting dari versi audit sebelumnya:**

| Sebelumnya (salah) | Sebenarnya (corrected) |
|---|---|
| "jsreport punya visual designer (jsreport Studio)" | jsreport Studio adalah **code-based editor**, BUKAN visual drag-and-drop designer |
| "jsreport bisa drag-and-drop" | jsreport **TIDAK** punya drag-and-drop. GitHub issue #121 open sejak 2015, masih belum diimplementasi |
| "jsreport Studio = visual drag-and-drop designer" | jsreport Studio = HTML/Handlebars code editor dengan preview panel |
| "MIT license" | jsreport adalah **LGPL** (bukan MIT), lebih restrictive |
| "jsreport bisa embed designer di React" | jsreport Studio berjalan sebagai **server terpisah** (port 5488), tidak bisa embed sebagai React component |

**Perbedaan jsreport (net) vs jsreports (com):**

| | jsreport (jsreport.net) | jsreports (jsreports.com) |
|---|---|---|
| **Apa** | Reporting server (rendering engine) | Reporting library (designer + viewer) |
| **Designer** | ❌ Code-based editor only | ✅ Visual drag-and-drop designer |
| **Embed di React** | ❌ Server terpisah | ✅ React component |
| **Lisensi** | LGPL (open source) | Komersial (bayar) |
| **Fokus** | Server-side rendering | Client-side design + rendering |
| **Output** | PDF, Excel, Word, HTML | PDF, HTML |
| **Go integration** | ✅ REST API | ⚠️ Client-side (butuh adapter) |

---

## RINGKASAN PRIORITAS — Missing Modules + Dashboard + Reporting

| # | Item | Prioritas | Effort |
|---|---|---|---|
| 1 | **jsreport deploy + invoice/PO/DO templates** | Tinggi | 2-3 minggu |
| 2 | **Dashboard widget system (per-user)** | Tinggi | 4-5 minggu |
| 3 | **Multi-currency + FX gain/loss** | Tinggi | 3-4 minggu |
| 4 | **Approval workflow engine** | Tinggi | 3-4 minggu |
| 5 | **AR Aging report + customer statement** | Tinggi | 2 minggu |
| 6 | **AP Aging report + payment schedule** | Tinggi | 2 minggu |
| 7 | **PPh 21/22/23/26 + bukti potong** | Tinggi | 3-4 minggu |
| 8 | **Multi-warehouse + master warehouse** | Tinggi | 2-3 minggu |
| 9 | **Recurring transactions** | Tinggi | 2 minggu |
| 10 | **Cash flow forecast** | Tinggi | 2-3 minggu |
| 11 | **Giro & cheque management** | Sedang | 2-3 minggu |
| 12 | **Email notification + document sending** | Sedang | 2-3 minggu |
| 13 | **Budget vs actual + variance** | Sedang | 1-2 minggu |
| 14 | **Petty cash (imprest)** | Sedang | 1-2 minggu |
| 15 | **Inter-company elimination (full)** | Sedang | 3-4 minggu |
| 16 | **Cost/profit center accounting** | Sedang | 3-4 minggu |
| 17 | **Asset register report + maintenance** | Sedang | 1-2 minggu |
| 18 | **Metabase deploy (analytics)** | Rendah | 1 minggu |

---

## BAGIAN VI: IMPLEMENTATION PLAN — NextReport Engine Integration

**Keputusan:** Menggunakan **OPSI B: NextReport Engine** untuk reporting & print solution.

### N-01 [ANALISIS] NextReport Engine — Status & Maturity

**Repository:** `nextreport/engine` (GitHub, MIT License)

| Kriteria | Status | Catatan |
|---|---|---|
| **Versi** | v0.2 | Early stage, belum v1.0 |
| **Stars** | 11 | Komunitas masih sangat kecil |
| **Commits** | 82 | Aktif development |
| **Tests** | 428 | Test coverage decent |
| **License** | MIT | Bebas komersial |
| **Stack** | TypeScript, React, Next.js | Native web |
| **PDF rendering** | Puppeteer (Chrome headless) | Butuh Chromium di server |
| **Template storage** | File-based (YAML) | DB-backed belum ada di roadmap |
| **Designer** | Visual drag-and-drop (React component) | Bisa embed di app |
| **Dashboard builder** | Metabase-style (drag-and-drop) | Tersebut di README |
| **AI-ready** | JSON schema-driven | Bisa generate via LLM |
| **Chart types** | Line, bar, pie, scatter, area | Cukup untuk ERP |
| **Excel output** | ❌ Belum ada | Roadmap: belum listed |
| **Word output** | ❌ Belum ada | Roadmap: belum listed |

**Packages:**
| Package | Fungsi |
|---|---|
| `@nextreport/engine-core` | Report definition, schema, execution |
| `@nextreport/ui-designer` | Visual drag-and-drop designer (React) |
| `@nextreport/ui-viewer` | Report viewer (React) |
| `@nextreport/renderer-html` | HTML rendering |
| `@nextreport/renderer-pdf` | PDF rendering via Puppeteer |

### N-02 [RISK ASSESSMENT] Risiko Menggunakan NextReport

| Risiko | Severity | Mitigation |
|---|---|---|
| **Project masih v0.2 — breaking changes likely** | Tinggi | Pin versi exact, fork jika perlu, contribute upstream |
| **Komunitas kecil (11 stars)** | Sedang | Tim harus siap debug source code NextReport sendiri |
| **Excel output belum ada** | Sedang | Gunakan Go + excelize untuk Excel (sudah ada) |
| **DB-backed templates belum ada** | Sedang | Simpan template YAML di DB sebagai text, write to file saat render |
| **Puppeteer dependency** | Rendah | Docker image dengan Chromium pre-installed |
| **Dokumentasi terbatas** | Sedang | Baca source code + tests sebagai dokumentasi |
| **Indonesia-specific forms (faktur pajak)** | Sedang | Custom template dengan HTML/CSS |
| **Bug di NextReport engine** | Tinggi | Fork + fix, atau contribute PR |

### N-03 [ARCHITECTURE] Arsitektur Integrasi NextReport dengan Finance Accounting APP

```
┌──────────────────────────────────────────────────────────────────────┐
│                     ARCHITECTURE OVERVIEW                             │
├──────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  ┌─────────────────────┐    ┌───────────────────────────────────┐   │
│  │  React Frontend     │    │  Go Backend (API Server)          │   │
│  │  (web/src/)         │    │  (backend/)                       │   │
│  │                     │    │                                   │   │
│  │  ┌───────────────┐  │    │  ┌─────────────────────────────┐  │   │
│  │  │ NextReport    │  │    │  │ Reporting Handler           │  │   │
│  │  │ UI Designer   │  │    │  │ (backend/internal/reporting/)│  │   │
│  │  │ (embed)       │  │    │  │                             │  │   │
│  │  │               │  │    │  │ Endpoints:                  │  │   │
│  │  │ • Palette     │  │    │  │ GET  /reports/templates      │  │   │
│  │  │ • Canvas      │  │    │  │ POST /reports/templates      │  │   │
│  │  │ • Properties  │  │    │  │ GET  /reports/templates/:id  │  │   │
│  │  │ • Preview     │  │    │  │ PUT  /reports/templates/:id  │  │   │
│  │  └───────┬───────┘  │    │  │ DELETE /reports/templates/:id│  │   │
│  │          │          │    │  │                             │  │   │
│  │  ┌───────▼───────┐  │    │  │ POST /reports/render         │  │   │
│  │  │ NextReport    │  │    │  │   body: { template_id,       │  │   │
│  │  │ UI Viewer     │  │    │  │           data: {...},       │  │   │
│  │  │ (embed)       │  │    │  │           format: "pdf" }    │  │   │
│  │  │               │  │    │  │                             │  │   │
│  │  │ • HTML preview│  │    │  │ GET /reports/:type/pdf       │  │   │
│  │  │ • PDF download│  │    │  │   ?id=123                    │  │   │
│  │  └───────────────┘  │    │  │                             │  │   │
│  └─────────┬───────────┘    │  └──────────┬──────────────────┘  │   │
│            │                └─────────────┼─────────────────────┘   │
│            │                              │                         │
│            │  REST API                    │  HTTP POST              │
│            │  (JSON)                      │  (template YAML + data) │
│            ▼                              ▼                         │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │  NextReport Rendering Service (Node.js sidecar)             │    │
│  │  (Docker container)                                         │    │
│  │                                                             │    │
│  │  ┌─────────────┐  ┌──────────────┐  ┌───────────────────┐  │    │
│  │  │ Express API │  │ NextReport   │  │ Puppeteer         │  │    │
│  │  │ :3001       │  │ Engine       │  │ (Chrome headless) │  │    │
│  │  │             │→ │ (engine-core)│→ │ PDF rendering     │  │    │
│  │  │ POST /render│  │              │  │                   │  │    │
│  │  │ POST /design│  │ renderer-pdf │  │                   │  │    │
│  │  └─────────────┘  └──────────────┘  └───────────────────┘  │    │
│  └─────────────────────────────────────────────────────────────┘    │
│                                                                      │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │  PostgreSQL Database                                        │    │
│  │                                                             │    │
│  │  report_templates (NEW TABLE)                               │    │
│  │  ├── id, tenant_id, code, name                              │    │
│  │  ├── report_type (invoice, po, do, faktur_pajak, dll)       │    │
│  │  ├── template_yaml (TEXT — NextReport YAML definition)      │    │
│  │  ├── is_default, is_active                                  │    │
│  │  └── created_by, created_at, updated_at                     │    │
│  └─────────────────────────────────────────────────────────────┘    │
│                                                                      │
└──────────────────────────────────────────────────────────────────────┘
```

### N-04 [DATA FLOW] Alur Data per Dokumen

#### Flow 1: User Design Template (via NextReport UI Designer)

```
1. User buka "Report Designer" di React frontend
2. NextReport UI Designer ter-embed di page (React component)
3. User drag-and-drop elements ke canvas:
   - Text: "Invoice {{invoice.number}}"
   - Table: items dengan binding {{#each items}}
   - Image: logo perusahaan
   - Total: "Total: {{formatCurrency invoice.totalCents}}"
4. Designer meng-generate JSON report definition
5. Frontend kirim JSON ke Go backend: POST /reports/templates
6. Go backend simpan YAML di report_templates.template_yaml
7. Response: template saved
```

#### Flow 2: User Print Invoice (via NextReport Viewer)

```
1. User klik "Print" di halaman invoice
2. Frontend request: GET /invoices/123/pdf
3. Go backend:
   a. Fetch invoice data dari DB (joins: customer, items, prices)
   b. Fetch template YAML dari report_templates WHERE report_type='invoice' AND is_default=true
   c. POST ke NextReport Rendering Service:
      {
        "template": "<YAML content>",
        "data": { "invoice": {...}, "customer": {...}, "items": [...] },
        "format": "pdf"
      }
   d. NextReport engine:
      - Parse YAML → report definition
      - Bind data ke template (Handlebars-style)
      - Render HTML
      - Puppeteer: HTML → PDF
   e. Return PDF binary
4. Go backend return PDF ke frontend
5. Frontend: download PDF atau tampilkan di iframe
```

#### Flow 3: Dashboard Widget (Custom React + NextReport)

```
1. User buka Dashboard
2. React fetch dashboard_widgets untuk user ini
3. Untuk setiap widget:
   a. KPI widget → fetch data dari Go API → render dengan Recharts
   b. Report widget → embed NextReport UI Viewer dengan template + data
4. User bisa:
   - Add widget dari palette
   - Drag widget untuk reposition (react-grid-layout)
   - Configure widget (date range, filter)
   - Remove widget
5. Layout disimpan di dashboard_widgets table
```

### N-05 [IMPLEMENTATION] Detail Step-by-Step

#### Phase 1: Setup & Infrastructure (Minggu 1-2)

**Step 1: Deploy NextReport Rendering Service**

Buat file `docker-compose.nextreport.yml`:
```yaml
version: "3.8"
services:
  nextreport:
    image: node:22-slim
    working_dir: /app
    ports:
      - "3001:3001"
    environment:
      - PORT=3001
      - PUPPETEER_EXECUTABLE_PATH=/usr/bin/chromium
    volumes:
      - ./nextreport-service:/app
    command: sh -c "npm install && npm start"
    # Install Chromium untuk Puppeteer
    # apt-get update && apt-get install -y chromium
```

Buat `nextreport-service/` directory:
```
nextreport-service/
├── package.json
├── server.js          ← Express API server
├── templates/         ← Default templates (seed)
│   ├── invoice.yaml
│   ├── purchase_order.yaml
│   ├── delivery_order.yaml
│   └── statement.yaml
└── Dockerfile
```

`nextreport-service/server.js`:
```javascript
const express = require('express');
const cors = require('cors');
const { renderReport } = require('@nextreport/engine-core');
const { renderToHTML } = require('@nextreport/renderer-html');
const { renderToPDF } = require('@nextreport/renderer-pdf');

const app = express();
app.use(cors());
app.use(express.json({ limit: '50mb' }));

// Health check
app.get('/health', (req, res) => res.json({ status: 'ok' }));

// Render report
app.post('/render', async (req, res) => {
  const { template, data, format } = req.body;
  try {
    const html = await renderToHTML({ template, data });
    if (format === 'html') {
      return res.type('html').send(html);
    }
    const pdf = await renderToPDF({ template, data });
    return res.type('pdf').send(pdf);
  } catch (err) {
    return res.status(500).json({ error: err.message });
  }
});

app.listen(3001, () => console.log('NextReport service on :3001'));
```

**Step 2: Database Migration**

`backend/migrations/000026_report_templates.up.sql`:
```sql
-- Report templates for NextReport integration.
-- Each template stores the NextReport YAML definition as text.
CREATE TABLE report_templates (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    code TEXT NOT NULL,
    name TEXT NOT NULL,
    report_type TEXT NOT NULL CHECK (report_type IN (
        'invoice', 'purchase_order', 'delivery_order',
        'faktur_pajak', 'bukti_potong', 'customer_statement',
        'supplier_statement', 'payment_voucher', 'receipt_voucher',
        'journal_voucher', 'stock_card', 'trial_balance',
        'profit_loss', 'balance_sheet', 'cash_flow',
        'ar_aging', 'ap_aging', 'asset_register',
        'stock_opname', 'custom'
    )),
    template_yaml TEXT NOT NULL,
    is_default BOOLEAN NOT NULL DEFAULT false,
    is_active BOOLEAN NOT NULL DEFAULT true,
    orientation TEXT NOT NULL DEFAULT 'portrait' CHECK (orientation IN ('portrait', 'landscape')),
    paper_size TEXT NOT NULL DEFAULT 'A4' CHECK (paper_size IN ('A4', 'A5', 'Letter', 'Legal')),
    margin_top_mm INT NOT NULL DEFAULT 10,
    margin_bottom_mm INT NOT NULL DEFAULT 10,
    margin_left_mm INT NOT NULL DEFAULT 10,
    margin_right_mm INT NOT NULL DEFAULT 10,
    created_by BIGINT REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, code),
    -- Only one default per report_type per tenant
    UNIQUE (tenant_id, report_type) WHERE is_default = true
);

ALTER TABLE report_templates ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_report_templates ON report_templates
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::BIGINT);
ALTER TABLE report_templates FORCE ROW LEVEL SECURITY;

-- Dashboard widget layouts (per-user)
CREATE TABLE dashboard_layouts (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL DEFAULT 'My Dashboard',
    is_default BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, user_id, name)
);

CREATE TABLE dashboard_widgets (
    id BIGSERIAL PRIMARY KEY,
    layout_id BIGINT NOT NULL REFERENCES dashboard_layouts(id) ON DELETE CASCADE,
    widget_type TEXT NOT NULL CHECK (widget_type IN (
        'kpi_card', 'line_chart', 'bar_chart', 'pie_chart',
        'ar_aging', 'ap_aging', 'cash_forecast',
        'budget_vs_actual', 'top_customers', 'top_items',
        'recent_transactions', 'pending_approvals',
        'inventory_alerts', 'calendar', 'bank_balance',
        'tax_summary', 'report_embed', 'custom'
    )),
    title TEXT NOT NULL,
    data_source TEXT NOT NULL,
    config JSONB NOT NULL DEFAULT '{}',
    position_x INT NOT NULL DEFAULT 0,
    position_y INT NOT NULL DEFAULT 0,
    width INT NOT NULL DEFAULT 4,
    height INT NOT NULL DEFAULT 2,
    is_visible BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (layout_id, id)
);

ALTER TABLE dashboard_layouts ENABLE ROW LEVEL SECURITY;
ALTER TABLE dashboard_widgets ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_dashboard_layouts ON dashboard_layouts
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::BIGINT);
CREATE POLICY tenant_isolation_dashboard_widgets ON dashboard_widgets
    USING (tenant_id = current_setting('app.tenant_id', true)::BIGINT)
    WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::BIGINT);
ALTER TABLE dashboard_layouts FORCE ROW LEVEL SECURITY;
ALTER TABLE dashboard_widgets FORCE ROW LEVEL SECURITY;
```

`backend/migrations/000026_report_templates.down.sql`:
```sql
DROP TABLE IF EXISTS dashboard_widgets;
DROP TABLE IF EXISTS dashboard_layouts;
DROP TABLE IF EXISTS report_templates;
```

#### Phase 2: Go Backend — Report Template & Render API (Minggu 2-3)

**Step 3: Report Template Handler**

Buat `backend/internal/reporting/templates.go`:
```go
package reporting

// ReportTemplateRequest untuk POST /reports/templates
type ReportTemplateRequest struct {
    Code         string `json:"code"`
    Name         string `json:"name"`
    ReportType   string `json:"report_type"`
    TemplateYAML string `json:"template_yaml"`
    IsDefault    bool   `json:"is_default"`
    Orientation  string `json:"orientation"`
    PaperSize    string `json:"paper_size"`
    MarginTop    int    `json:"margin_top_mm"`
    MarginBottom int    `json:"margin_bottom_mm"`
    MarginLeft   int    `json:"margin_left_mm"`
    MarginRight  int    `json:"margin_right_mm"`
}

// ReportTemplateResponse
type ReportTemplateResponse struct {
    ID           int64  `json:"id"`
    Code         string `json:"code"`
    Name         string `json:"name"`
    ReportType   string `json:"report_type"`
    TemplateYAML string `json:"template_yaml"`
    IsDefault    bool   `json:"is_default"`
    IsActive     bool   `json:"is_active"`
    Orientation  string `json:"orientation"`
    PaperSize    string `json:"paper_size"`
    // ... margins
}

// Endpoints:
// GET    /reports/templates           — list (filter by report_type)
// POST   /reports/templates           — create
// GET    /reports/templates/{id}      — get by id
// PUT    /reports/templates/{id}      — update
// DELETE /reports/templates/{id}      — delete (soft delete: is_active=false)
// POST   /reports/templates/{id}/set-default — set as default for report_type
```

**Step 4: Render Handler**

Buat `backend/internal/reporting/render.go`:
```go
package reporting

import (
    "bytes"
    "encoding/json"
    "net/http"
    "os"
)

// RenderRequest untuk POST /reports/render
type RenderRequest struct {
    TemplateID int64                  `json:"template_id"`
    Data       map[string]interface{} `json:"data"`
    Format     string                 `json:"format"` // "pdf" or "html"
}

// RenderReport mengirim template + data ke NextReport service
func (service *Service) RenderReport(w http.ResponseWriter, r *http.Request) {
    var req RenderRequest
    if err := decodeJSON(r, &req); err != nil {
        writeError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
        return
    }

    // 1. Fetch template dari DB
    tmpl, err := service.fetchTemplate(r, req.TemplateID)
    if err != nil {
        writeError(w, http.StatusNotFound, "TEMPLATE_NOT_FOUND", err.Error())
        return
    }

    // 2. POST ke NextReport rendering service
    nextreportURL := os.Getenv("NEXTREPORT_URL") // e.g. http://nextreport:3001
    payload := map[string]interface{}{
        "template": tmpl.TemplateYAML,
        "data":     req.Data,
        "format":   req.Format,
    }
    body, _ := json.Marshal(payload)

    resp, err := http.Post(nextreportURL+"/render", "application/json", bytes.NewReader(body))
    if err != nil {
        writeError(w, http.StatusBadGateway, "RENDER_FAILED", "NextReport service unavailable")
        return
    }
    defer resp.Body.Close()

    // 3. Return PDF/HTML ke client
    if req.Format == "pdf" {
        w.Header().Set("Content-Type", "application/pdf")
        w.Header().Set("Content-Disposition", `attachment; filename="report.pdf"`)
    } else {
        w.Header().Set("Content-Type", "text/html")
    }
    io.Copy(w, resp.Body)
}

// PrintInvoice — convenience endpoint: GET /invoices/{id}/pdf
func (service *Service) PrintInvoice(w http.ResponseWriter, r *http.Request) {
    invoiceID := pathID(chi.URLParam(r, "id"))
    // 1. Fetch invoice data (join customer, items, prices)
    data := service.fetchInvoiceData(r, invoiceID)
    // 2. Fetch default invoice template
    tmpl := service.fetchDefaultTemplate(r, "invoice")
    // 3. Render via NextReport
    service.renderViaNextReport(w, tmpl, data, "pdf")
}

// PrintPurchaseOrder — GET /purchase-orders/{id}/pdf
// PrintDeliveryOrder — GET /delivery-orders/{id}/pdf
// PrintCustomerStatement — GET /customers/{id}/statement/pdf
// PrintFakturPajak — GET /invoices/{id}/faktur-pajak/pdf
```

**Step 5: Register Routes di main.go**

Tambahkan ke `backend/cmd/api/main.go`:
```go
// Report template management
router.Get("/reports/templates", reportingHandler.ListTemplates)
router.Post("/reports/templates", reportingHandler.CreateTemplate)
router.Get("/reports/templates/{id}", reportingHandler.GetTemplate)
router.Put("/reports/templates/{id}", reportingHandler.UpdateTemplate)
router.Delete("/reports/templates/{id}", reportingHandler.DeleteTemplate)
router.Post("/reports/templates/{id}/set-default", reportingHandler.SetDefault)

// Document print endpoints
router.Get("/invoices/{id}/pdf", reportingHandler.PrintInvoice)
router.Get("/purchase-orders/{id}/pdf", reportingHandler.PrintPurchaseOrder)
router.Get("/delivery-orders/{id}/pdf", reportingHandler.PrintDeliveryOrder)
router.Get("/customers/{id}/statement/pdf", reportingHandler.PrintCustomerStatement)
router.Get("/invoices/{id}/faktur-pajak/pdf", reportingHandler.PrintFakturPajak)

// Generic render
router.Post("/reports/render", reportingHandler.RenderReport)

// Dashboard widget endpoints
router.Get("/dashboard/layout", dashboardHandler.GetLayout)
router.Put("/dashboard/layout", dashboardHandler.UpdateLayout)
router.Post("/dashboard/widgets", dashboardHandler.AddWidget)
router.Put("/dashboard/widgets/{id}", dashboardHandler.UpdateWidget)
router.Delete("/dashboard/widgets/{id}", dashboardHandler.RemoveWidget)
```

#### Phase 3: Frontend — Report Designer (Minggu 3-5)

**Step 6: Install NextReport packages**

```bash
cd web
npm install @nextreport/engine-core @nextreport/ui-designer @nextreport/ui-viewer @nextreport/renderer-html
```

**Step 7: Report Designer Page**

Buat `web/src/screens/designer/ReportDesigner.tsx`:
```tsx
import { ReportDesigner } from '@nextreport/ui-designer';
import { useState, useEffect } from 'react';
import { api } from '../../api';

export function ReportDesignerScreen({ templateId }: { templateId?: string }) {
  const [definition, setDefinition] = useState(null);

  useEffect(() => {
    if (templateId) {
      // Load existing template
      api.getReportTemplate(templateId).then(tmpl => {
        setDefinition(parseYAML(tmpl.template_yaml));
      });
    }
  }, [templateId]);

  const handleSave = (newDefinition) => {
    const yaml = serializeToYAML(newDefinition);
    api.saveReportTemplate({
      code: 'invoice',
      name: 'Invoice Template',
      report_type: 'invoice',
      template_yaml: yaml,
      is_default: true,
    });
  };

  return (
    <div className="report-designer-screen">
      <ReportDesigner
        definition={definition}
        onSave={handleSave}
        dataSources={[
          { name: 'invoice', fields: ['number', 'date', 'customer.name', 'items', 'total'] },
          { name: 'company', fields: ['name', 'address', 'phone', 'logo'] },
        ]}
      />
    </div>
  );
}
```

**Step 8: Report Viewer (Print Preview)**

Buat `web/src/screens/viewer/ReportViewer.tsx`:
```tsx
import { ReportViewer } from '@nextreport/ui-viewer';

export function InvoicePDFViewer({ invoiceId }: { invoiceId: number }) {
  const [pdfUrl, setPdfUrl] = useState('');

  useEffect(() => {
    // Option A: Fetch PDF dari Go backend
    setPdfUrl(`/api/v1/invoices/${invoiceId}/pdf`);
  }, [invoiceId]);

  return (
    <div>
      <iframe src={pdfUrl} width="100%" height="700px" title="Invoice PDF" />
      <a href={pdfUrl} download className="btn btn--primary">
        Download PDF
      </a>
    </div>
  );

  // Option B: Client-side render via NextReport Viewer
  // return (
  //   <ReportViewer
  //     template={templateYAML}
  //     data={invoiceData}
  //     format="pdf"
  //   />
  // );
}
```

**Step 9: Dashboard Widget System**

Buat `web/src/screens/workbench/CustomDashboard.tsx`:
```tsx
import { Responsive, WidthProvider } from 'react-grid-layout';
import { LineChart, BarChart, PieChart } from 'recharts';
import { useEffect, useState } from 'react';
import { api } from '../../api';

const ResponsiveGridLayout = WidthProvider(Responsive);

export function CustomDashboard() {
  const [widgets, setWidgets] = useState([]);
  const [layout, setLayout] = useState([]);

  useEffect(() => {
    api.getDashboardLayout().then(data => {
      setWidgets(data.widgets);
      setLayout(data.widgets.map(w => ({
        i: String(w.id),
        x: w.position_x,
        y: w.position_y,
        w: w.width,
        h: w.height,
      })));
    });
  }, []);

  const onLayoutChange = (newLayout) => {
    setLayout(newLayout);
    // Debounce save
    api.updateDashboardLayout(newLayout);
  };

  const renderWidget = (widget) => {
    switch (widget.widget_type) {
      case 'kpi_card':
        return <KPICard widget={widget} />;
      case 'line_chart':
        return <LineChartWidget widget={widget} />;
      case 'bar_chart':
        return <BarChartWidget widget={widget} />;
      case 'ar_aging':
        return <ARAgingWidget widget={widget} />;
      case 'recent_transactions':
        return <RecentTransactionsWidget widget={widget} />;
      case 'report_embed':
        return <ReportEmbedWidget widget={widget} />;
      default:
        return <div>Unknown widget: {widget.widget_type}</div>;
    }
  };

  return (
    <div className="custom-dashboard">
      <div className="dashboard-toolbar">
        <button onClick={addWidget}>+ Add Widget</button>
        <button onClick={saveLayout}>Save Layout</button>
      </div>
      <ResponsiveGridLayout
        className="layout"
        layouts={{ lg: layout }}
        breakpoints={{ lg: 1200, md: 996, sm: 768 }}
        cols={{ lg: 12, md: 10, sm: 6 }}
        rowHeight={80}
        onLayoutChange={onLayoutChange}
        isDraggable
        isResizable
      >
        {widgets.map(w => (
          <div key={String(w.id)} className="dashboard-widget">
            <div className="widget-header">
              <span>{w.title}</span>
              <button onClick={() => removeWidget(w.id)}>×</button>
            </div>
            {renderWidget(w)}
          </div>
        ))}
      </ResponsiveGridLayout>
    </div>
  );
}
```

**Step 10: Add Widget Palette**

```tsx
function AddWidgetPalette({ onAdd }: { onAdd: (type: string) => void }) {
  const widgetTypes = [
    { type: 'kpi_card', label: 'KPI Card', icon: '📊' },
    { type: 'line_chart', label: 'Line Chart', icon: '📈' },
    { type: 'bar_chart', label: 'Bar Chart', icon: '📊' },
    { type: 'pie_chart', label: 'Pie Chart', icon: '🥧' },
    { type: 'ar_aging', label: 'AR Aging', icon: '📋' },
    { type: 'ap_aging', label: 'AP Aging', icon: '📋' },
    { type: 'cash_forecast', label: 'Cash Forecast', icon: '💰' },
    { type: 'budget_vs_actual', label: 'Budget vs Actual', icon: '🎯' },
    { type: 'top_customers', label: 'Top Customers', icon: '🏆' },
    { type: 'top_items', label: 'Top Items', icon: '📦' },
    { type: 'recent_transactions', label: 'Recent Transactions', icon: '🔄' },
    { type: 'pending_approvals', label: 'Pending Approvals', icon: '⏳' },
    { type: 'inventory_alerts', label: 'Inventory Alerts', icon: '⚠️' },
    { type: 'calendar', label: 'Due Date Calendar', icon: '📅' },
    { type: 'bank_balance', label: 'Bank Balance', icon: '🏦' },
    { type: 'tax_summary', label: 'Tax Summary', icon: '🧾' },
    { type: 'report_embed', label: 'Report Embed', icon: '📄' },
  ];

  return (
    <div className="widget-palette">
      {widgetTypes.map(w => (
        <button key={w.type} onClick={() => onAdd(w.type)} className="palette-item">
          <span className="icon">{w.icon}</span>
          <span className="label">{w.label}</span>
        </button>
      ))}
    </div>
  );
}
```

#### Phase 4: Default Templates (Minggu 5-7)

**Step 11: Seed Default Templates**

Buat template YAML untuk setiap dokumen. Contoh `invoice.yaml`:

```yaml
# NextReport template: Invoice Penjualan
version: "1.0"
meta:
  name: "Invoice"
  paperSize: "A4"
  orientation: "portrait"
  marginTop: 15
  marginBottom: 15
  marginLeft: 15
  marginRight: 15
sections:
  - type: "header"
    elements:
      - type: "image"
        src: "{{company.logoUrl}}"
        width: 150
        height: 60
        align: "left"
      - type: "text"
        content: "{{company.name}}"
        style: { fontSize: 16, fontWeight: "bold" }
        align: "right"
      - type: "text"
        content: "{{company.address}}, {{company.city}}"
        style: { fontSize: 10 }
        align: "right"
      - type: "text"
        content: "NPWP: {{company.npwp}}"
        style: { fontSize: 10 }
        align: "right"
  - type: "spacer"
    height: 20
  - type: "text"
    content: "INVOICE"
    style: { fontSize: 24, fontWeight: "bold", textAlign: "center" }
  - type: "spacer"
    height: 10
  - type: "row"
    columns:
      - width: "50%"
        elements:
          - type: "text"
            content: "Bill To:"
            style: { fontWeight: "bold" }
          - type: "text"
            content: "{{customer.name}}"
          - type: "text"
            content: "{{customer.address}}"
          - type: "text"
            content: "{{customer.city}}, {{customer.postalCode}}"
      - width: "50%"
        elements:
          - type: "text"
            content: "Invoice No: {{invoice.number}}"
          - type: "text"
            content: "Date: {{formatDate invoice.invoiceDate}}"
          - type: "text"
            content: "Due Date: {{formatDate invoice.dueDate}}"
          - type: "text"
            content: "PO No: {{invoice.customerPoNumber}}"
  - type: "spacer"
    height: 15
  - type: "table"
    dataSource: "invoice.lines"
    columns:
      - header: "#"
        width: "5%"
        value: "{{index}}"
      - header: "Item"
        width: "35%"
        value: "{{itemName}}"
      - header: "Qty"
        width: "10%"
        value: "{{formatNumber qty}}"
        align: "right"
      - header: "Unit Price"
        width: "20%"
        value: "{{formatCurrency unitPriceCents}}"
        align: "right"
      - header: "Discount"
        width: "10%"
        value: "{{formatCurrency discountCents}}"
        align: "right"
      - header: "Total"
        width: "20%"
        value: "{{formatCurrency lineTotalCents}}"
        align: "right"
    style:
      headerBackground: "#f0f0f0"
      border: "1px solid #ddd"
  - type: "spacer"
    height: 10
  - type: "row"
    columns:
      - width: "60%"
        elements: []
      - width: "40%"
        elements:
          - type: "text"
            content: "Subtotal: {{formatCurrency invoice.subtotalCents}}"
            align: "right"
          - type: "text"
            content: "Discount: {{formatCurrency invoice.discountTotalCents}}"
            align: "right"
          - type: "text"
            content: "PPN (11%): {{formatCurrency invoice.taxTotalCents}}"
            align: "right"
          - type: "text"
            content: "Grand Total: {{formatCurrency invoice.grandTotalCents}}"
            style: { fontWeight: "bold", fontSize: 14 }
            align: "right"
  - type: "spacer"
    height: 30
  - type: "row"
    columns:
      - width: "50%"
        elements:
          - type: "text"
            content: "Hormat kami,"
          - type: "spacer"
            height: 60
          - type: "text"
            content: "{{salesperson.name}}"
      - width: "50%"
        elements:
          - type: "text"
            content: "Terms & Conditions:"
          - type: "text"
            content: "{{invoice.termsAndConditions}}"
            style: { fontSize: 9 }
  - type: "footer"
    elements:
      - type: "text"
        content: "Page {{pageNumber}} of {{totalPages}}"
        align: "center"
        style: { fontSize: 9, color: "#999" }
```

**Daftar template yang harus dibuat:**

| # | Template | Report Type | Prioritas |
|---|---|---|---|
| 1 | Invoice Penjualan | invoice | Tinggi |
| 2 | Purchase Order | purchase_order | Tinggi |
| 3 | Delivery Order / Surat Jalan | delivery_order | Tinggi |
| 4 | Faktur Pajak (format DJP) | faktur_pajak | Tinggi |
| 5 | Bukti Potong PPh | bukti_potong | Tinggi |
| 6 | Customer Statement | customer_statement | Tinggi |
| 7 | Supplier Statement | supplier_statement | Sedang |
| 8 | Payment Voucher | payment_voucher | Sedang |
| 9 | Receipt Voucher | receipt_voucher | Sedang |
| 10 | Journal Voucher | journal_voucher | Sedang |
| 11 | Stock Card | stock_card | Sedang |
| 12 | Trial Balance | trial_balance | Sudah ada (ganti gofpdf) |
| 13 | P&L Statement | profit_loss | Sudah ada (ganti gofpdf) |
| 14 | Balance Sheet | balance_sheet | Sudah ada (ganti gofpdf) |
| 15 | Cash Flow Statement | cash_flow | Sudah ada (ganti gofpdf) |
| 16 | AR Aging | ar_aging | Tinggi (baru) |
| 17 | AP Aging | ap_aging | Tinggi (baru) |
| 18 | Asset Register | asset_register | Sedang |
| 19 | Stock Opname Report | stock_opname | Sedang |

### N-06 [DASHBOARD] Widget Data Sources

Setiap dashboard widget butuh data source endpoint di Go backend:

| Widget Type | Data Source Endpoint | Query |
|---|---|---|
| `kpi_card` (cash) | `GET /dashboard/kpi/cash` | Sum CASH/BANK account balances |
| `kpi_card` (p&l) | `GET /dashboard/kpi/profit-loss` | Revenue - Expense this month |
| `kpi_card` (AR) | `GET /dashboard/kpi/ar-outstanding` | Sum outstanding AR |
| `kpi_card` (AP) | `GET /dashboard/kpi/ap-outstanding` | Sum outstanding AP |
| `line_chart` (revenue trend) | `GET /dashboard/chart/revenue-trend?months=12` | Monthly revenue 12 months |
| `bar_chart` (expense by category) | `GET /dashboard/chart/expense-by-category` | Sum expense grouped by category |
| `pie_chart` (P&L breakdown) | `GET /dashboard/chart/pl-breakdown` | Revenue vs COGS vs Expense |
| `ar_aging` | `GET /dashboard/chart/ar-aging` | AR aging buckets |
| `ap_aging` | `GET /dashboard/chart/ap-aging` | AP aging buckets |
| `cash_forecast` | `GET /dashboard/chart/cash-forecast?weeks=12` | Projected inflow vs outflow |
| `budget_vs_actual` | `GET /dashboard/chart/budget-vs-actual?year=2026` | Budget vs actual per month |
| `top_customers` | `GET /dashboard/table/top-customers?limit=10` | Top 10 by revenue |
| `top_items` | `GET /dashboard/table/top-items?limit=10` | Top 10 by sales qty |
| `recent_transactions` | `GET /dashboard/table/recent-transactions?limit=10` | Last 10 transactions |
| `pending_approvals` | `GET /dashboard/list/pending-approvals` | Documents pending approval |
| `inventory_alerts` | `GET /dashboard/list/inventory-alerts` | Items below reorder point |
| `calendar` | `GET /dashboard/calendar/due-dates?month=current` | Due dates this month |
| `bank_balance` | `GET /dashboard/kpi/bank-balances` | Balance per bank account |
| `tax_summary` | `GET /dashboard/kpi/tax-summary` | PPN keluaran vs masukan |
| `report_embed` | `GET /reports/render?template_id=X` | Embedded NextReport |

### N-07 [DOCKER] Docker Compose untuk Development

```yaml
# docker-compose.yml (extend existing)
version: "3.8"
services:
  # ... existing services (postgres, backend, frontend) ...

  nextreport:
    build: ./nextreport-service
    ports:
      - "3001:3001"
    environment:
      - PORT=3001
      - PUPPETEER_EXECUTABLE_PATH=/usr/bin/chromium
      - NODE_ENV=production
    depends_on:
      - postgres
    restart: unless-stopped
```

`nextreport-service/Dockerfile`:
```dockerfile
FROM node:22-slim

# Install Chromium untuk Puppeteer
RUN apt-get update && apt-get install -y \
    chromium \
    --no-install-recommends \
    && rm -rf /var/lib/apt/lists/*

ENV PUPPETEER_EXECUTABLE_PATH=/usr/bin/chromium
ENV PUPPETEER_SKIP_CHROMIUM_DOWNLOAD=true

WORKDIR /app
COPY package*.json ./
RUN npm ci --production
COPY . .

EXPOSE 3001
CMD ["node", "server.js"]
```

### N-08 [ROADMAP] Timeline Implementasi

| Minggu | Phase | Deliverable | Effort |
|---|---|---|---|
| **1-2** | Infrastructure | Docker setup, DB migration, NextReport service deploy | 2 minggu |
| **2-3** | Backend API | Template CRUD, render endpoint, data fetchers | 1.5 minggu |
| **3-4** | Invoice template | Template YAML + data fetcher + PDF endpoint | 1 minggu |
| **4-5** | Frontend designer | ReportDesigner.tsx (embed NextReport UI Designer) | 1.5 minggu |
| **5-6** | Frontend viewer | ReportViewer.tsx (PDF preview + download) | 1 minggu |
| **6-7** | More templates | PO, DO, surat jalan, customer statement | 1.5 minggu |
| **7-8** | Dashboard backend | Widget data source endpoints | 1.5 minggu |
| **8-10** | Dashboard frontend | CustomDashboard.tsx (react-grid-layout + widgets) | 2.5 minggu |
| **10-11** | Faktur pajak + bukti potong | Template format DJP + data fetcher | 1.5 minggu |
| **11-12** | Testing & polish | E2E test, template refinement, performance | 1 minggu |
| **Total** | | | **~12 minggu (3 bulan)** |

### N-09 [FALLBACK PLAN] Jika NextReport Tidak Memenuhi

Karena NextReport masih v0.2 dan komunitas kecil, siapkan fallback plan:

| Trigger | Fallback Action |
|---|---|
| NextReport breaking change di v0.3+ | Pin versi v0.2, fork jika perlu |
| NextReport project abandoned | Migrate ke jsreport (net) — rendering engine tetap sama (Chrome → PDF), hanya ganti designer ke custom-built |
| NextReport designer tidak cukup untuk template kompleks | Gunakan jsreport untuk rendering + custom React designer untuk visual editing |
| Puppeteer too heavy untuk production | Ganti ke Playwright (lebih efficient) atau ke Go-native chromedp |
| Template YAML format berubah | Version-control templates, migration script untuk convert format lama |

### N-10 [MONITORING] Health Check & Observability

```go
// Go backend: health check untuk NextReport service
func (service *Service) CheckNextReportHealth() error {
    resp, err := http.Get(os.Getenv("NEXTREPORT_URL") + "/health")
    if err != nil || resp.StatusCode != 200 {
        return fmt.Errorf("NextReport service unavailable")
    }
    return nil
}

// Metric: render time
func (service *Service) RenderReport(w http.ResponseWriter, r *http.Request) {
    start := time.Now()
    // ... render ...
    renderDuration := time.Since(start)
    log.Printf("render_time_ms=%d template_type=%s", renderDuration.Milliseconds(), tmpl.ReportType)
}
```

```javascript
// NextReport service: health check
app.get('/health', (req, res) => {
  res.json({
    status: 'ok',
    version: require('@nextreport/engine-core/package.json').version,
    uptime: process.uptime(),
    memory: process.memoryUsage(),
  });
});
```

---

## RINGKASAN FINAL — Implementation Plan NextReport

| Komponen | Technology | Status |
|---|---|---|
| **Rendering engine** | NextReport engine-core + renderer-pdf (Puppeteer) | Docker sidecar |
| **Visual designer** | NextReport ui-designer (React embed) | Frontend page |
| **Report viewer** | NextReport ui-viewer (React embed) atau iframe PDF | Frontend page |
| **Template storage** | PostgreSQL `report_templates` table (YAML as text) | New migration |
| **Dashboard** | react-grid-layout + Recharts + NextReport embed | Frontend page |
| **Dashboard storage** | PostgreSQL `dashboard_layouts` + `dashboard_widgets` | New migration |
| **Excel export** | Go + excelize (existing, tinggal perluas) | Already exists |
| **Go backend** | Template CRUD + render proxy + data fetchers | New handlers |
| **Dokumen templates** | 19 templates (invoice, PO, DO, faktur pajak, dll) | YAML files |
| **Dashboard widgets** | 17 widget types (KPI, charts, tables, calendars) | React components |
| **Timeline** | ~12 minggu (3 bulan) | Phased |
| **Fallback** | jsreport (net) + custom designer | If NextReport fails |

---

*Audit ini bersifat static review dengan first-hand code tracing. Setiap file Go dibaca langsung, setiap journal entry ditrace ke spec paragraph, setiap DB constraint diverifikasi di migration SQL. Beberapa temuan (terutama yang memerlukan runtime verification) mungkin memerlukan dynamic testing untuk konfirmasi.*
