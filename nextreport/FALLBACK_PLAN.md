# NextReport — Fallback Plan (N-09)

Dokumen ini mendefinisikan rencana fallback jika komponen NextReport bermasalah
di production. Arsitektur rendering dipisahkan melalui satu kontrak HTTP
sederhana, sehingga setiap komponen dapat diganti tanpa mengubah aplikasi utama.

## Kontrak Rendering (Interface Stabil)

Go backend (`reports/templates.go` → `RenderReport`) berkomunikasi dengan
rendering sidecar hanya via:

```
POST {NEXTREPORT_URL}/render
Body: { "template_yaml": "<yaml>", "data": {...}, "format": "html"|"pdf" }
Response: rendered bytes (text/html atau application/pdf) + status code

GET {NEXTREPORT_URL}/health
Response: { "status": "ok", ... }
```

Selama kontrak ini dipenuhi, implementasi sidecar dapat diganti bebas.

## Skenario Fallback

### 1. Sidecar Down / Tidak Merespons

- **Deteksi**: `GET /healthz/detail` melaporkan `nextreport.status = down`.
- **Dampak**: hanya fitur render template (`POST /reports/templates/{id}/render`)
  yang gagal; seluruh aplikasi akuntansi tidak terpengaruh.
- **Aksi**:
  1. `docker compose restart nextreport`
  2. Jika berulang, cek log: `docker compose logs nextreport --tail=50`
  3. Jika image rusak, rebuild: `docker compose build --no-cache nextreport`

### 2. NextReport Package Upstream Breaking Change (v0.3+)

- Sidecar saat ini **zero-dependency** (plain Node http, tanpa npm packages),
  jadi breaking change npm `@nextreport/*` TIDAK berdampak — kita tidak
  menginstal package upstream tersebut.
- Jika tetap ingin mengikuti upstream: pin versi eksplisit di `package.json`
  sebelum adopt.

### 3. Kebutuhan Rendering Lebih Kompleks (CSS table, styling kaya di PDF)

PDF writer sidecar saat ini adalah writer PDF 1.4 minimal (text-only).
Jika dibutuhkan PDF dengan styling kaya:

- **Opsi A (recommended)**: ganti engine PDF sidecar dengan headless Chromium
  (Puppeteer/Playwright) di dalam sidecar — kontrak HTTP tidak berubah.
- **Opsi B**: ganti seluruh sidecar dengan **jsreport** (jsreport.net, LGPL):
  1. Jalankan `jsreport/jsreport` Docker image sebagai pengganti `nextreport` service.
  2. Buat adapter kecil agar endpoint jsreport memenuhi kontrak `/render`,
     ATAU ubah `NEXTREPORT_URL` + sesuaikan `RenderReport` (perubahan 1 file Go).
  3. Template YAML harus dikonversi ke format jsreport (handlebars + recipe).

### 4. Migrasi Template

Template tersimpan di dua tempat (harus sinkron):
- File: `nextreport/templates/*.yaml` (19 template)
- DB: `report_templates` (seed via migration `000038`)

Aturan: DB adalah source of truth saat runtime; file YAML adalah referensi
developer. Jika mengubah template, update keduanya atau edit via API
`PUT /reports/templates/{id}` (perubahan DB only, file tidak berubah).

### 5. Rollback Lengkap

Jika seluruh fitur template harus dimatikan sementara:
1. Hapus/comment blok `nextreport` di `docker-compose.yml`, hapus env
   `NEXTREPORT_URL` dari service `api`.
2. `RenderReport` akan gagal koneksi → mengembalikan error jelas; endpoint lain
   tetap berfungsi.
3. UI Report Templates akan menampilkan error render — acceptable untuk
   rollback sementara.

## Keputusan: Pertahankan Arsitektur Saat Ini

Arsitektur sidecar zero-dependency dipertahankan karena:
- Tidak ada ketergantungan npm (attack surface & build risk minimal)
- Kontrak HTTP stabil memungkinkan swap engine tanpa perubahan aplikasi
- Ukuran image kecil, start cepat, mudah di-restart
