# FIX F5 — Auth & Infra (QA-02, QA-18, QA-14, N7, QA-20)

Worktree: `fix-wave3-f5-auth-infra` · Tanggal: 2026-08-23 · Status: **SEMUA 5 BUG FIXED + terverifikasi E2E**

Lingkup sesuai penugasan: `backend/internal/middleware/*`, `backend/cmd/api/main.go` (wiring limiter/logout), `backend/internal/auth/auth.go` (handler logout), `backend/internal/dashboard/handler.go`, `backend/internal/reports/templates.go`, `backend/internal/reporting` (validasi query tanggal). Tidak ada perubahan sales/purchase/costing/cash/period/coa. TASK_LEDGER tidak disentuh, tidak ada commit.

---

## Ringkasan Bug → Status

| # | Bug | Severity | Status | Bukti |
|---|-----|----------|--------|-------|
| 1 | QA-02: satu bucket rate limiter dibagi login+register+refresh+2FA | HIGH | **FIXED** | Burst login 10×401 → 429+Retry-After; register tetap 201; refresh 6× tetap 200 |
| 2 | QA-18: `/auth/logout` garbage/tanpa token → 200 false-success | MED | **FIXED** | no token → 401; garbage bearer → 401; valid logout → 200 + refresh revoked (401 saat dipakai lagi) |
| 3 | QA-14: `POST /dashboard/widgets` tanpa `config` → 500 not-null | MED | **FIXED** | payload minimalis → 201, `config_json` tersimpan `{}` |
| 4 | N7: PUT/DELETE template lintas-tenant → 200 false-success | LOW | **FIXED** | PUT/DELETE id tenant lain → 404 NOT_FOUND; data utuh; PUT milik sendiri tetap 200 |
| 5 | QA-20: query tanggal invalid diam-diam diabaikan → 200 tanpa filter | LOW | **FIXED** | `?from_date=invalid` / `?date_from=invalid` → 400 INVALID_REQUEST di 4 laporan; param kosong tetap sah (200) |

---

## Detail Perbaikan

### 1. QA-02 (HIGH) — Rate limiter per endpoint group

**Masalah:** `loginLimiter` tunggal (5 req/menit) dipakai bersama login, register, refresh, dan 2FA. Empiris di retest: burst login gagal mengunci pengguna sah di endpoint lain.

**Perbaikan:**
- `middleware.NewAuthLimiters()` (`middleware.go`) — instansiasi terpisah per endpoint group, per-IP sliding window 1 menit dipertahankan, `Retry-After: 60` dipertahankan pada 429.
- `main.go` — tiap route group memakai bucket sendiri; `defer authLimiters.Stop()`.
- Bonus kecil terkait: goroutine cleanup limiter kini dijamin tunggal per instance via `sync.Once` (sebelumnya `Middleware()` men-spawn goroutine baru setiap kali dipasang ke route — 6 goroutine per start).

**Angka final (tune di `middleware.go`, konstanta ter-export):**

| Endpoint | Limit | Konstanta |
|----------|-------|-----------|
| `POST /auth/login` | **10/menit/IP** | `middleware.AuthLoginRatePerMinute` |
| `POST /auth/register` | **10/menit/IP** | `middleware.AuthRegisterRatePerMinute` |
| `POST /auth/refresh` | **30/menit/IP** | `middleware.AuthRefreshRatePerMinute` |
| `POST /auth/2fa/{setup,verify,disable}` | **10/menit/IP** (gabungan) | `middleware.AuthTwoFARatePerMinute` |

`/auth/switch-tenant` sengaja tidak dilimitasi (butuh refresh token valid, sudah dirotasi + family-revocation F-06).

**Test:** `TestAuthLimiters_IndependentBuckets`, `TestAuthLimiters_RefreshHigherThanLogin`, `TestRateLimiter_MiddlewareSpawnsSingleCleanup` (`middleware_test.go`).

### 2. QA-18 (MED) — Logout di balik middleware auth

**Masalah:** `POST /auth/logout` terpasang di luar grup auth; garbage/tanpa token → handler tetap jalan dan jawab 200 sambil merevoke tidak ada apa-apa (false-success, jejak audit palsu).

**Perbaikan (opsi dampak-frontend terkecil, sesuai instruksi):** route dipindah ke dalam `router.Group` bermiddleware `authService.Middleware` di `main.go`. Handler (`auth.go`) kini juga menolak body `refresh_token` kosong dengan 400; token non-kosong yang tak dikenal tetap idempoten 200.

**Semantik final:** tanpa/garbage bearer → **401**; bearer valid + body kosong → **400 INVALID_REQUEST**; bearer valid + refresh token apa pun → **200** (+ revoke bila token masih hidup).

**Catatan follow-up frontend (di luar scope sesi ini):** `web/src/api.ts:701` memanggil logout tanpa header `Authorization` — tambahkan header access token agar logout tetap merevoke server-side; saat ini kegagalannya tertelan `.catch()` sehingga UX lokal tetap bersih.

**Test:** `logout_route_test.go` — 401 tanpa token, 401 garbage bearer, 401 scheme non-Bearer, 400 body rusak/kosong (tanpa DB).

### 3. QA-14 (MED) — Widget tanpa config → 201

**Masalah:** `AddWidget` mengirim `[]byte(req.Config)`; saat field `config` tidak ada, `json.RawMessage` nil → binding `[]byte(nil)` yang **menimpa** default kolom `config_json JSONB NOT NULL DEFAULT '{}'::jsonb` dengan NULL → 500 not-null violation.

**Perbaikan (`dashboard/handler.go`):** helper `normalizeWidgetConfig` — kosong/whitespace → `[]byte("{}")`; JSON invalid → 400 INVALID_REQUEST (bukan 500 cast di DB); valid → passthrough. Diterapkan di `AddWidget` **dan** `UpdateWidget` (pola bug identik: update tanpa config sebelumnya juga menimpa config lama dengan NULL).

**Test:** `TestNormalizeWidgetConfig` (unit) + `TestAddWidget_EmptyConfigCreates201` (integrasi, guard `TEST_DATABASE_URL`, memverifikasi `config_json` tersimpan `{}`).

### 4. N7 (LOW) — Templates PUT/DELETE cek RowsAffected

**Masalah:** `UPDATE/DELETE report_templates ... WHERE tenant_id=$1 AND id=$2` yang match 0 baris (id milik tenant lain / global `tenant_id=0`) tetap dijawab 200 false-success karena `RowsAffected` tak dicek.

**Perbaikan (`reports/templates.go`):** `UpdateTemplate` dan `DeleteTemplate` memeriksa `ct.RowsAffected() == 0` → **404 NOT_FOUND**. Global `tenant_id=0` tetap terlindungi: predikat `tenant_id=$1` (dengan GUC RLS dari `db.WithTenantData`) mengecualikannya dari tulis — tidak ada perubahan pada query, hanya pelaporan hasil.

**Test:** `templates_n7_test.go` (integrasi, guard `TEST_DATABASE_URL`) — PUT/DELETE lintas-tenant 404 + data utuh + sanity PUT/DELETE milik sendiri 200.

### 5. QA-20 (LOW) — Query tanggal invalid → 400

**Masalah:** `parseDateRange` membuang tanggal invalid secara diam-diam → `?from_date=invalid` menghasilkan 200 **tanpa filter** (laporan menampilkan semua data seolah filter tak pernah diminta).

**Perbaikan (`reporting/handler.go` + `export.go`):**
- `parseDateRange`/`parseReportFilter` kini mengembalikan error; keempat handler laporan (`trial-balance`, `profit-loss`, `balance-sheet`, `cash-flow`) + `Export` memetakannya ke **400 INVALID_REQUEST** dengan pesan bersih (mis. `from_date must be YYYY-MM-DD, got "invalid"`).
- **Param kosong tetap sah** (`?from_date=` = tanpa filter) — perilaku toolbar yang mengirim blank dipertahankan.
- Alias `date_from`/`date_to` diterima selain `from_date`/`to_date` agar reproduksi QA (`?date_from=invalid`) juga terdeteksi 400.

**Test:** `TestParseDateRange` & `TestParseReportFilter` dirombak ke semantik baru (kasus invalid kini ekspektasi error), plus `TestReportHandlers_InvalidDateParamReturns400` (unit tanpa DB — 400 ditulis sebelum akses DB).

---

## Verifikasi E2E

Lingkungan: DB **finance_qa_fx5** (56 migrasi per-file; seed global 000030 dilewati sesuai insiden 000030 yang terdokumentasi), role **qa_fx5_app** (`NOSUPERUSER NOBYPASSRLS`, GRANT penuh tabel+sequence), server port **:18095**, register tenant **W-F5** sukses via role terbatas.

| Bukti | Hasil |
|-------|-------|
| Burst login salah password | `401×10` lalu `429` + `Retry-After: 60` (limit login 10/menit) |
| REGISTER setelah login bucket habis | `201` tenant_id=1001 — **bucket terpisah terbukti** |
| REFRESH 6× saat login terlimit | `200×6` — tidak terkena limit login |
| Logout tanpa token / garbage bearer | `401` / `401` |
| Logout valid (bearer + refresh_token) | `200`; refresh token yang sama dipakai lagi → `401` (revoked) |
| Logout bearer valid + refresh_token kosong | `400 INVALID_REQUEST` |
| `POST /dashboard/widgets` tanpa config | `201` (id=1), `config_json` = `{}` di DB |
| PUT/DELETE template tenant lain | `404 NOT_FOUND`; data tenant pemilik utuh |
| DELETE id template nonexistent | `404 NOT_FOUND` |
| `trial-balance/profit-loss/balance-sheet/cash-flow?from_date=invalid` | `400 INVALID_REQUEST` (juga `?date_from=invalid` → 400) |
| `?from_date=&to_date=` (kosong) | `200` tanpa filter — param kosong sah |

## Gate

```
cd backend && gofmt -l .        → bersih
go vet ./...                    → ok
go build ./...                  → ok
go test ./...                   → semua paket ok (unit; test integrasi skip tanpa TEST_DATABASE_URL)
```

Test integrasi baru (dashboard QA-14, reports N7) dijalankan hijau terhadap `TEST_DATABASE_URL=finance_qa_fx5` saat penulisan laporan ini.

## File Berubah

| File | Perubahan |
|------|-----------|
| `backend/internal/middleware/middleware.go` | `AuthLimiters` + konstanta limit; `sync.Once` cleanup goroutine |
| `backend/internal/middleware/middleware_test.go` | 3 test baru QA-02 |
| `backend/cmd/api/main.go` | wiring 4 limiter terpisah; logout pindah ke grup auth |
| `backend/internal/auth/auth.go` | handler logout validasi `refresh_token` kosong; helper `ContextKeyUserID` (untuk test) |
| `backend/internal/auth/logout_route_test.go` | **baru** — test QA-18 |
| `backend/internal/dashboard/handler.go` | `normalizeWidgetConfig`; dipakai Add+UpdateWidget |
| `backend/internal/dashboard/handler_qa14_test.go` | **baru** — unit + integrasi QA-14 |
| `backend/internal/reports/templates.go` | RowsAffected → 404 di Update/Delete |
| `backend/internal/reports/templates_n7_test.go` | **baru** — integrasi N7 |
| `backend/internal/reporting/handler.go` | parse tanggal strict + 400 di 4 handler |
| `backend/internal/reporting/export.go` | 400 pada Export |
| `backend/internal/reporting/reporting_test.go` | test dirombak ke semantik QA-20 + test handler 400 |
