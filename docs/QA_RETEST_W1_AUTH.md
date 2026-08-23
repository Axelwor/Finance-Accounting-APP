# QA Retest Keamanan/Auth — Worktree W1 (qa-retest-w1-auth)

**Tanggal:** 2026-08-23 · **Lingkup:** verifikasi bug auth (QA-01, QA-02, QA-18) dari `docs/QA_REPORT_2026-08-23.md` + regresi RLS Phase C pada alur auth · **Target:** worktree HEAD `78053fe` (Phase C selesai), build `cmd/api` @ `:18081`

---

## 1. Ringkasan Eksekutif

Retest dijalankan sepenuhnya meniru konfigurasi prod C3: server API terhubung ke PostgreSQL lokal sebagai role **`qa_w1_app` (`LOGIN NOSUPERUSER NOBYPASSRLS`)** — bukan superuser. Seluruh ±45 kasus (positif/negatif/edge) dieksekusi melalui satu koneksi role terbatas tersebut.

| Bug | Status | Bukti inti |
|---|---|---|
| **QA-01** Registrasi gagal di bawah RLS | **FIXED** ✅ | Register dengan `tenant_name` → **201**, `tenant_id=1000`, **48 akun seed** terverifikasi SQL (dengan GUC `app.tenant_id` dan cross-check superuser), periode OPEN + membership terbentuk. **Nol** error RLS di log Postgres untuk sesi ini. |
| **QA-02** Rate limiter agresif & shared bucket | **STILL BROKEN** ❌ | Empiris: tepat **5 req / 60 detik per IP** digabung login+register+refresh; req #6 → 429. Refresh sah ikut ter-429 oleh burst login (bucket masih shared). Satu-satunya perbaikan terlihat: header `Retry-After: 60`. |
| **QA-18** Logout tanpa/garbage token → 200 | **STILL BROKEN** ❌ | Logout garbage token → **200** `{"status":"ok"}`; tanpa header auth + body `{}` → **200**. Hanya body hilang total → 400 EOF. |

**Regresi baru: 1 (minor)** — percobaan jurnal manual lintas-tenant mengembalikan **500** alih-alih 4xx (data aman, rollback bersih; lihat §5). Isolasi lintas-tenant **UTUH**: seluruh upaya baca/tulis lintas tenant → 404/400, tidak ada kebocoran data.

---

## 2. Lingkungan & Metodologi

- DB fresh `finance_qa_rw1`, owner `qa_w1_app`; migrasi diaplikasikan berurutan `ON_ERROR_STOP=1`.
- Server: binary build worktree ini, `HTTP_ADDR=":18081"`, `JWT_SECRET` khusus QA, `DATABASE_URL` role terbatas (persistent background process).
- Rate limiter in-memory di-reset antar kelompok uji dengan restart server (limiter hidup di proses).
- Verifikasi state DB langsung via psql dua arah: (a) sebagai role aplikasi **dengan** `SET app.tenant_id`, (b) sebagai superuser (bypass RLS) — penting karena tabel `accounts` ber-status `relforcerowsecurity = t`, sehingga psql tanpa GUC otomatis melihat 0 baris.
- Log Postgres `/usr/local/var/log/postgresql@16.log` difilter untuk atribusi (instance dipakai paralel sesi QA lain).

### Catatan migrasi (environment, bukan regresi auth)

Dari 56 file `.up.sql`, **4 gagal** saat diaplikasikan sebagai role terbatas:

| File | Error |
|---|---|
| `000030_reporting_dashboard.up.sql` | FK violation `report_templates_tenant_id_fkey` (tenant_id=0 belum ada) — **known issue** QA-21/corrections.md; efek: `dashboard_layouts`/`dashboard_widgets` tidak ter-create |
| `000049_seed_missing_coa_accounts.up.sql` | RLS violation pada `accounts` (seed tenant 0) |
| `000050_seed_collision_accounts.up.sql` | idem |
| `000052_journal_posting_support.up.sql` | idem; eksekusi berhenti mid-file sehingga statement berikutnya (mis. `ALTER TABLE petty_cash_vouchers ADD replenished_at`) tidak jalan |

Ini konsisten dengan desain C3 (**migrasi = superuser-only**; prod `deploy.sh` menjalankan migrasi sebagai superuser), jadi tidak dihitung sebagai bug auth — tetapi membuktikan rantai migrasi **tidak replay-clean** sebagai role aplikasi. Tidak memengaruhi skenario auth (tabel users/user_tokens/user_tenants/tenants/accounts utuh).

---

## 3. Tabel Verdict per Kasus

Skala: FIXED ✅ / STILL-BROKEN ❌ / PARTIAL 🟡 / NEW 🆕 / PASS ✅ (kasus non-bug)

### 3.1 QA-01 — Onboarding tenant di bawah role terbatas

| Kasus | HTTP | Hasil | Verdict |
|---|---|---|---|
| Register `{"tenant_name":"W1 Corp","email":"owner@w1.test","password":"Sup3rSecret!2026","full_name":"W1 Owner"}` | **201** | Body: `access_token`,`refresh_token`,`tenant_id:1000`,`role:"owner"`,`family_id` | **FIXED** ✅ |
| Seed COA: `SELECT count(*) FROM accounts WHERE tenant_id=1000` (GUC + superuser) | — | **48** akun ✅ | FIXED ✅ |
| Periode akuntansi | — | 1 baris `OPEN` untuk tenant 1000 ✅ | FIXED ✅ |
| Membership `user_tenants` | — | 1 baris owner ✅ | FIXED ✅ |
| Log Postgres sesi register | — | **Tidak ada** `row-level security policy` error dari API ✅ | FIXED ✅ |

Klaim fix commit acd4766 (SeedDefaultCOA menyetel GUC `app.tenant_id` dalam transaksi) **terverifikasi** di bawah role NOSUPERUSER NOBYPASSRLS.

### 3.2 Validasi register

| Kasus | HTTP | Respons | Verdict |
|---|---|---|---|
| Duplikat email (`owner@w1.test`) | **409** | `EMAIL_EXISTS` | PASS ✅ |
| Email kosong | **400** | `INVALID_REQUEST` "email and full_name are required" | PASS ✅ |
| Password kosong | **400** | `WEAK_PASSWORD` | PASS ✅ |
| Password lemah `"123"` | **400** | `WEAK_PASSWORD` "at least 8 characters" | PASS ✅ |
| `full_name` kosong | **400** | `INVALID_REQUEST` | PASS ✅ |
| Tanpa `tenant_name` | **201** | `tenant_id:0`; user id=3 **tanpa baris membership** (diverifikasi SQL) | PASS ✅ |
| JSON malformed | **400** | `INVALID_REQUEST` (pesan parser, tanpa stack leak) | PASS ✅ |
| Content-Type salah (`text/plain`) | **400** | `INVALID_REQUEST` | PASS ✅ |
| Register tenant kedua (`B Corp`) | **201** | `tenant_id:1001`, 48 akun seed sendiri | PASS ✅ |

### 3.3 Login

| Kasus | HTTP | Respons | Verdict |
|---|---|---|---|
| Login benar | **200** | `access_token` + `refresh_token` + `family_id` | PASS ✅ |
| Password salah | **401** | `INVALID_CREDENTIALS` (pesan generik) | PASS ✅ |
| Email tak dikenal | **401** | `INVALID_CREDENTIALS` — pesan **identik** dgn password salah (tidak membocorkan keberadaan email) | PASS ✅ |

### 3.4 Refresh rotation & deteksi reuse

| Kasus | HTTP | Respons / Bukti | Verdict |
|---|---|---|---|
| Refresh token valid (#1) | **200** | Token baru, `family_id` sama | PASS ✅ |
| REUSE token lama | **401** | `INVALID_REFRESH`; DB: **seluruh family revoked** (kedua baris `revoked_at` terisi, token baru `replaced_by` kosong) | PASS ✅ |
| Token baru pasca-revoke family | **401** | `INVALID_REFRESH` (via API, pasca-restart limiter) | PASS ✅ |

Catatan: percobaan pertama kasus ketiga mendapat **429** karena jatah limiter habis dipakai rangkaian tes — dampak nyata QA-02 pada alur sah (lihat §3.6).

### 3.5 Validasi token akses (protected endpoint `GET /api/v1/tenants/me`)

| Kasus | HTTP | Respons | Verdict |
|---|---|---|---|
| Tanpa header Authorization | **401** | `TOKEN_REQUIRED` | PASS ✅ |
| Garbage token | **401** | `INVALID_TOKEN` | PASS ✅ |
| JWT forged `alg:none` (payload valid, exp masa depan) | **401** | `INVALID_TOKEN` | PASS ✅ |
| Signature palsu (payload asli, sig diganti) | **401** | `INVALID_TOKEN` | PASS ✅ |

### 3.6 QA-02 — Rate limiter (pengukuran empiris)

| Kasus | Hasil | Verdict |
|---|---|---|
| Burst 12× login password salah | Req **1–5 → 401**, req **6–12 → 429** `RATE_LIMITED` | Konfigurasi masih `NewRateLimiter(5, time.Minute)` (`backend/cmd/api/main.go:125`) |
| Register saat bucket habis | **429** — bucket **shared** login+register+refresh+2fa | **STILL BROKEN** ❌ |
| Refresh sah saat bucket habis | **429** — refresh frontend otomatis ikut terkunci oleh aktivitas login lain | **STILL BROKEN** ❌ |
| 429 vs 401 saat brute-force | 5 percobaan pertama tetap 401 (sinyal terlihat); setelah itu semua 429 hingga window habis | PARTIAL 🟡 |
| Header `Retry-After: 60` pada 429 | Ada ✅ | Perbaikan kecil sejak laporan awal |

**Perilaku persis saat ini:** sliding/bucket per-IP 5 request per 60 detik, digabung untuk semua endpoint `/auth/login`, `/auth/register`, `/auth/refresh`, `/auth/2fa/*` (per kode maupun empiris). Lockout pengguna sah di NAT/kantor bersama dan saling-makan-jatah antar-endpoint **tetap terjadi**.

### 3.7 QA-18 — Logout

| Kasus | HTTP | Respons | Verdict |
|---|---|---|---|
| Logout `{"refresh_token":"garbage-nonsense-token"}` | **200** | `{"status":"ok"}` — UPDATE 0 baris tetap sukses | **STILL BROKEN** ❌ |
| Logout tanpa header auth, body `{}` | **200** | `{"status":"ok"}` | **STILL BROKEN** ❌ |
| Logout tanpa body sama sekali | **400** | `INVALID_REQUEST` "EOF" | (satu-satunya yang ditolak) |

Route `/auth/logout` masih di luar middleware auth (`main.go:133`) dan handler tidak memvalidasi keberadaan token (`auth.go:357-370`). Jejak audit logout tetap menyesatkan.

### 3.8 Isolasi lintas-tenant (token B terhadap resource tenant A)

| Kasus | HTTP | Respons | Verdict |
|---|---|---|---|
| Kontrol: B baca jurnal miliknya (id=1) | **200** | normal | PASS ✅ |
| B `GET /journal-entries/2` (jurnal A) | **404** | `NOT_FOUND` "journal entry not found" | PASS ✅ |
| B `GET /journal-entries` (list) | **200** | 0 item — jurnal A tidak bocor | PASS ✅ |
| B reverse jurnal A (payload lengkap) | **404** | `ACCOUNT_NOT_FOUND` "account does not exist for this tenant"; jurnal A tetap `POSTED`, count tetap 1 | PASS ✅ |
| B deactivate akun A (`accounts/4/deactivate`) | **404** | `ACCOUNT_NOT_FOUND`; `is_active` tetap `t` | PASS ✅ |
| B cash-in pakai `cash_account_id` milik A (=4) | **404** | `ACCOUNT_NOT_FOUND` | PASS ✅ |
| B jurnal manual referensi akun A | **500** ⚠️ | `POST_FAILED`; log: `raw_message:"no rows in result set"` — **RLS berhasil blok tulisan** (rollback bersih, 0 jurnal tercipta), tapi error class salah | NEW 🆕 (minor) |
| Kontrol: jurnal manual akun milik B sendiri | **201** | `JRN-2026-000001` POSTED (numbering per-tenant benar — sama dgn nomor jurnal A, id berbeda) | PASS ✅ |

### 3.9 Regresi C3 — operan berjalan sebagai role terbatas

| Check | Hasil |
|---|---|
| Semua operan auth+posting di atas via satu koneksi `NOSUPERUSER NOBYPASSRLS` | ✅ Berjalan semua (register, seed 48 COA, login/refresh/logout, posting jurnal, reverse path, healthz) |
| `GET /healthz/detail` awal & akhir | ✅ `{"database":{"status":"up"},...,"status":"ok"}` |
| Error RLS di log Postgres dari traffic API | ✅ Nol (register seed bersih) |
| Migrasi sebagai role terbatas | ⚠️ 4 file gagal (§2) — sesuai desain migrasi=superuser-only, dicatat sebagai environment note |

---

## 4. Daftar Bug Baru

**NEW-1 (MEDIUM-LOW) — Jurnal manual referensi akun lintas-tenant → 500 `POST_FAILED`, bukan 4xx**
- **Repro:** login tenant B → `POST /api/v1/journal-entries` dengan `lines[].account_id` milik tenant A → **500**. Log server: `"internal error returned to client","code":"POST_FAILED","raw_message":"no rows in result set"`.
- **Akar:** lookup akun dalam transaksi posting kena RLS (0 baris) → `pgx.ErrNoRows` dipetakan ke 500 generik, bukan `ACCOUNT_NOT_FOUND` 4xx.
- **Dampak:** isolasi data **tetap utuh** (rollback bersih, diverifikasi SQL: 0 jurnal tercipta); murni kelas error salah + raw message di log internal. Konsisten dengan pola lama "500 untuk kondisi bisnis" pada endpoint yang belum memakai klasifikasi error.
- **Saran:** map ErrNoRows pada lookup akun/jurnal ke 404 `ACCOUNT_NOT_FOUND` seperti sudah dilakukan route lain (cash-in, deactivate).

---

## 5. Kesimpulan

Fase C cutover **tidak merusak alur auth**: QA-01 benar-benar fixed dan seluruh siklus auth (register→seed 48 COA→login→refresh rotation→reuse detection→family revocation→logout) serta posting dasar berjalan mulus sebagai role DB terbatas. Dua bug lama bertahan tanpa perubahan perilaku berarti: rate limiter 5/menit shared (QA-02, kini setidaknya menyertakan `Retry-After`) dan logout yang selalu "sukses" (QA-18). Isolasi lintas-tenant utuh di semua jalur uji; satu temuan baru minor soal kelas error 500 pada probe lintas-tenant jurnal manual.

**Jawaban ringkas:**
- **QA-01: FIXED** — 201, tenant_id>0, 48 akun seed, tanpa error RLS, di bawah role NOSUPERUSER NOBYPASSRLS.
- **QA-02: STILL BROKEN** — 5 req/60 dtk shared login+register+refresh; req #6+ → 429 (dengan `Retry-After: 60`); refresh sah ikut terkunci; 401 brute-force tampak hanya untuk 5 percobaan pertama.
- **QA-18: STILL BROKEN** — logout garbage token & tanpa token → tetap **200 ok** (hanya body kosong total → 400).
- **Regresi baru: 1 (minor, error-class saja, data aman)** — lihat NEW-1.

*Tidak ada perubahan kode produksi, tidak ada commit, TASK_LEDGER tidak disentuh. Artefak uji di `/var/folders/kv/.../kilo/qa/w1-*` (sementara).*
