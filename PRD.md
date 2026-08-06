# Product Requirements Document (PRD)
## Software Pembukuan Mudah dengan Mesin Akuntansi Standar IFRS/PSAK

**Versi:** 4.4 — Review  
**Tanggal:** 2026-08-06  
**Status:** Review  
**Owner:** Product + Accounting  
**Normative:** Ya untuk scope dan roadmap

---

## 1. Ringkasan Eksekutif

Software pembukuan berbasis web dengan **konsep dua lapisan**:

- **Lapisan Pengguna (UI):** Sesederhana mungkin — pebisnis pemula, bahkan yang tidak paham akuntansi sama sekali, bisa mencatat transaksi dengan bahasa sehari-hari ("Hari ini jualan Rp 500.000 tunai"). Tidak ada istilah debet/kredit, jurnal, atau buku besar yang membingungkan.
- **Lapisan Mesin (Accounting Engine):** Di balik layar, setiap input sederhana otomatis diterjemahkan menjadi **jurnal double-entry yang benar dan lengkap sesuai standar IFRS/PSAK Indonesia (SAK Umum)** — termasuk pajak tangguhan, penilaian aset, dan pengungkapan yang dibutuhkan.

Hasilnya: pemula mendapat kemudahan setara aplikasi catat kas sederhana, sementara laporan yang dihasilkan **berkualitas audit dan sesuai PSAK** — aman untuk pajak, bank, investor, dan auditor.

### Prinsip Utama
1. **Input sederhana, output standar.** Pengguna mencatat seperti di buku tulis; sistem menyusun laporan IFRS/PSAK secara otomatis.
2. **Jurnal otomatis yang benar.** Setiap transaksi menghasilkan double-entry yang valid — tidak pernah ada selisih.
3. **Kepatuhan penuh di belakang layar.** Standar IFRS/PSAK dikelola engine: akun, pengukuran, reklasifikasi, dan pengungkapan.
4. **Mode Lanjutan (opsional).** Akuntan profesional tetap bisa mengakses jurnal, buku besar, dan penyesuaian manual bila diperlukan — tanpa mengganggu pengguna awam.
5. **Tetap "paling sederhana yang berfungsi".** Kompleksitas standar tidak pernah bocor ke UI pengguna awam.

### Nilai Utama Produk
| Untuk Siapa | Nilai yang Diberikan |
|---|---|
| Pebisnis pemula | Pembukuan tanpa belajar akuntansi — cukup catat seperti di buku tulis |
| UMKM tumbuh | Laporan standar untuk pajak, kredit bank, dan investor |
| Akuntan profesional | Mode lanjutan penuh: jurnal, buku besar, penyesuaian, export audit |

---

## 2. Target Pengguna (Persona)

| Persona | Profil | Kebutuhan Utama |
|---|---|---|
| **Pemilik Usaha Pemula** | Baru mulai (warung, kafe, online shop, jasa) | Mencatat uang masuk/keluar tanpa ribet, tahu untung/rugi |
| **Pemilik UMKM Tumbuh** | Omset hingga Rp 4,8 M/tahun | Laporan sesuai standar untuk pajak, kredit bank, investor |
| **Pemilik Multi-Usaha** | Punya beberapa usaha/cabang | Pisah catatan per usaha, laporan terpisah |
| **Akuntan / Pembukuan Profesional** | Menyusun laporan untuk klien | Jurnal penuh, buku besar, penyesuaian, dan export audit |
| **Konsultan / Pendamping UMKM** | Mendampingi klien | Akses multi-klien, template, export laporan |

### Alur Pengguna Utama (User Journey)

**A. Pengguna baru pertama kali (Onboarding):**
1. Mendaftar (email/Google) → 2. Isi data usaha (nama, jenis, mata uang, periode buku) → 3. Pilih template kategori sesuai jenis usaha → 4. Masukkan saldo awal (kas, bank, hutang, piutang) → 5. Selesai — langsung ke dashboard

**B. Mencatat transaksi harian:**
1. Klik "Catat Transaksi" atau "+" → 2. Pilih "Uang Masuk" / "Uang Keluar" → 3. Pilih kategori (Penjualan, Pembelian, dll) → 4. Isi nominal, tanggal, keterangan → 5. Simpan → sistem posting jurnal otomatis, saldo & laporan langsung ter-update

**C. Menutup periode / menyiapkan pajak:**
1. Dashboard menampilkan pengingat (periode belum ditutup, pajak jatuh tempo) → 2. Review laporan Laba Rugi → 3. Tutup periode (sistem kunci data, pindahkan laba ke ekuitas) → 4. Export laporan & SPT

---

## 3. Standar Akuntansi (Mesin di Balik Layar)

### 3.1 Kerangka Standar
| Standar | Peran |
|---|---|
| **SAK Umum (PSAK berbasis IFRS)** | Standar utama mesin akuntansi — untuk entitas yang perlu laporan berkualitas audit |
| **SAK ETAP** | Alternatif penyajian untuk entitas tanpa akuntabilitas publik |
| **SAK EMKM** | Mode penyajian ringkas untuk usaha mikro/kecil (omset ≤ Rp 4,8 M) |

Sistem **menyajikan laporan dalam beberapa kerangka** (EMKM / ETAP / SAK Umum) dari satu sumber pencatatan — pengguna memilih kerangka saat membuat laporan. Perbedaan antar kerangka (mis. pengakuan beban, penyajian) ditangani engine, bukan pengguna.

### 3.2 Standar Inti yang Didukung Engine (PSAK/IFRS)
| Kode PSAK | Standar IFRS | Topik |
|---|---|---|
| PSAK 1 | IAS 1 | Penyajian laporan keuangan |
| PSAK 2 | IAS 7 | Laporan arus kas |
| PSAK 14 | IAS 2 | Persediaan |
| PSAK 16 | IAS 16 | Aset tetap & penyusutan |
| PSAK 19 | IAS 38 | Aset takberwujud |
| PSAK 22 | IFRS 3 | Kombinasi bisnis |
| PSAK 24 | IAS 19 | Imbalan kerja |
| PSAK 26 | IAS 23 | Biaya pinjaman |
| PSAK 46 | IAS 12 | Pajak penghasilan & pajak tangguhan |
| PSAK 48 | IAS 36 | Penurunan nilai aset (impairment) |
| PSAK 50 | IAS 32 | Instrumen keuangan: penyajian |
| PSAK 55 → digantikan PSAK 71 | IAS 39 → IFRS 9 | Pengakuan & pengukuran instrumen keuangan |
| PSAK 60 | IFRS 7 | Instrumen keuangan: pengungkapan |
| PSAK 65 | IFRS 10 | Laporan keuangan konsolidasian |
| PSAK 67 | IFRS 12 | Pengungkapan kepentingan dalam entitas lain |
| PSAK 68 | IFRS 13 | Pengukuran nilai wajar |
| PSAK 71 | IFRS 9 | Instrumen keuangan (pengganti PSAK 55) |
| PSAK 72 | IFRS 15 | Pendapatan dari kontrak dengan pelanggan |
| PSAK 73 | IFRS 16 | Sewa |
| PSAK 74 | IFRS 17 | Kontrak asuransi (jika relevan) |

### 3.3 Kemampuan Kepatuhan Engine
- **Pajak tangguhan (deferred tax)** sesuai PSAK 46 — otomatis dihitung dari perbedaan temporer (aset tetap, penyisihan, dll).
- **Penyusutan, revaluasi & disposisi aset** sesuai PSAK 16 — metode garis lurus, saldo menurun, unit produksi; komponen aset; surplus/deficit revaluasi ke ekuitas (OCI); keuntungan/kerugian pelepasan ke laba rugi.
- **Persediaan** sesuai PSAK 14 — metode FIFO, rata-rata bergerak, identifikasi khusus; penilaian nilai realisasi neto (NRV); harga pokok penjualan otomatis saat barang terjual.
- **Alur penjualan SQ → SO → DP → DO → INV → pelunasan** — SQ/SO belum membukukan jurnal (hanya komitmen); DP membukukan kas/bank debet & uang muka penjualan kredit; DO mengurangi stok; INV mengakui pendapatan (PSAK 72) & HPP, mereklasifikasi uang muka ke piutang; pelunasan menutup piutang.
- **Alur pembelian PR → PO → GRN → tagihan → bayar** — PO belum membukukan jurnal; penerimaan barang menambah persediaan & hutang akrual; tagihan/supplier invoice menyesuaikan; pembayaran menutup hutang; uang muka pembelian (DP) dibukukan sebagai aset.
- **Harga pokok produksi sederhana** — job order costing: bill of materials (BOM), pengeluaran material, barang dalam proses (WIP), finished goods; biaya per job dihitung otomatis.
- **Impairment** aset & instrumen keuangan (PSAK 48, PSAK 71) — model ekspektasi kerugian kredit (ECL).
- **Sewa** PSAK 73 — right-of-use asset + lease liability otomatis dari kontrak sewa.
- **Pendapatan bertahap** PSAK 72 — identifikasi kewajiban kinerja, pengakuan bertahap.
- **Konsolidasi & eliminasi** antar-entitas/cabang (PSAK 65) dengan jurnal eliminasi otomatis.
- **Penutupan periode & reklasifikasi** yang benar dengan jejak audit penuh.

---

## 4. Regulasi Pajak Indonesia (Dukungan Awal)

Produk mendukung kepatuhan pajak dasar sejak awal, dengan mekanisme pembaruan regulasi terjadwal.

| Regulasi | Cakupan Dukungan |
|---|---|
| **PPh Final UMKM** (sesuai skema dan regulasi efektif) | Perhitungan otomatis sesuai eligibility, tarif, dan ambang omzet yang berlaku; 0,5%/Rp4,8 M hanya contoh konfigurasi historis; laporan pajak dapat ditelusuri |
| **PPN** (sesuai regulasi efektif) | Perhitungan PPN masukan/keluaran, faktur pajak sederhana, dasar e-Faktur; tarif dan eligibility berasal dari konfigurasi regulasi |
| **PPh 21** | Tarif efektif rata-rata (TER) sesuai PMK 168/2023; perhitungan gaji & tunjangan |
| **PPh 23/26** | Pemotongan atas jasa, royalti, sewa; bukti potong (dasar e-Bupot) |
| **PPh 22 & PPh 4(2)** | Pemotongan impor/bendaharawan; PPh final sewa tanah/bangunan |
| **UU KUP** | NPWP per entitas, periode pembukuan, kewajiban pencatatan |

**Catatan:** Integrasi penuh e-Faktur/e-Bupot/e-SPT (API DJP) direncanakan di fase 3+, dengan pembaruan berkala mengikuti perubahan regulasi.

---

## 5. Cakupan Fungsional (MoSCoW)

### 5.1 Lapisan Pengguna — Modul Sederhana (Must Have — v1)
| Modul | Deskripsi | Fitur Utama |
|---|---|---|
| **Dashboard Rumah** | Ringkasan kondisi usaha | Saldo kas/bank, untung/rugi bulan ini, tagihan jatuh tempo, tren pemasukan, pengingat pajak, nilai stok & stok menipis |
| **Uang Masuk / Uang Keluar** | Inti input — bahasa sehari-hari | Pilih jenis transaksi → pilih kategori → nominal → keterangan; **jurnal dibuat otomatis** |
| **Kategori Cerdas** | Pemetaan otomatis ke akun | Kategori siap pakai per jenis usaha + kustom; tiap kategori terpetakan ke akun PSAK yang benar |
| **Penjualan: SQ → SO → DP → DO → INV → Pelunasan** | Alur jualan lengkap dari penawaran sampai lunas | Sales Quotation (SQ) → Sales Order (SO) → Down Payment/uang muka (DP) → Delivery Order/pengiriman (DO) → Invoice (INV) → pelunasan; status & riwayat tiap tahap, otomatis mengakui pendapatan (PSAK 72) & mengurangi stok dengan HPP |
| **Pembelian: PR → PO → Penerimaan** | Alur belanja rapi dari permintaan sampai bayar | Permintaan pembelian (PR) → approval → Purchase Order (PO) → penerimaan barang (GRN) → tagihan & pembayaran |
| **Persediaan (Stok Barang)** | Kelola stok & nilai barang | Item master, stok masuk/keluar, transfer antar tempat, stock opname, penilaian otomatis (FIFO/rata-rata), nilai stok real-time |
| **Kas & Bank** | Rekonsiliasi sederhana | Cocokkan saldo, catat tunai, lihat posisi kas |
| **Saldo Awal** | Setup saldo pembukaan | Input saldo kas, bank, piutang, hutang, aset, modal saat mulai |
| **Laporan Otomatis** | Satu klik, format standar | Laba Rugi, Posisi Keuangan, Arus Kas, Catatan atas Laporan Keuangan — pilihan kerangka EMKM/ETAP/SAK Umum |
| **Setup Usaha (Onboarding)** | Wizard 5 menit | Nama usaha, jenis usaha, mata uang, periode buku, profil pajak, template kategori |

### 5.2 Mode Lanjutan — Akses Akuntansi Penuh (Should Have — v2)
| Modul | Deskripsi |
|---|---|
| **Jurnal Umum** | Lihat & edit jurnal hasil otomatis; input jurnal manual |
| **Buku Besar** | Mutasi per akun, saldo berjalan, drill-down ke transaksi sumber |
| **Chart of Accounts (COA)** | Struktur akun lengkap PSAK, akun kustom, pemetaan laporan |
| **Penyesuaian & Reklasifikasi** | Jurnal penyesuaian akhir periode, reklasifikasi antar akun |
| **Aset Tetap** | Registrasi, penyusutan multi-metode, revaluasi, disposisi/pelepasan (penjualan, transfer, penghapusan) |
| **Produksi (Job Order Costing)** | Job/produksi per pesanan, BOM, pengeluaran material, WIP, perhitungan harga pokok per job, barang jadi |
| **Pajak & Pelaporan** | PPh Final UMKM, PPN, PPh 21/23/26, laporan SPT |
| **Penutupan Periode** | Kunci periode, pindahkan laba ke ekuitas, buka periode baru |
| **Audit Trail** | Jejak lengkap: siapa, apa, kapan, sebelum/sesudah |
| **Export Profesional** | PDF/Excel/CSV, format untuk auditor dan bank |

### 5.3 Fitur Pelengkap (Could Have — v3+)
- Foto struk/kuitansi → transaksi otomatis (OCR)
- Notifikasi & pengingat (jatuh tempo, saldo rendah, pajak)
- Multi-usaha / multi-cabang sederhana dengan laporan gabungan
- Import data dari Excel/CSV atau aplikasi lain
- Hak akses peran: Pemilik / Admin / Konsultan / Akuntan
- Template usaha per jenis bisnis (kafe, online shop, jasa, dagang)

### 5.4 Di Luar Cakupan (Won't Have — v1)
- Payroll penuh (BPJS, PPh 21 detail per karyawan) — v1 cukup kategori "Gaji Karyawan"
- Manufaktur kompleks (BOM multi-level, routing, MRP, kapasitas mesin)
- WMS lanjutan (multi-gudang kompleks, barcode/RFID, logistik)
- Konsolidasi lintas entitas hukum yang kompleks (fase lanjut)
- Integrasi ERP eksternal (fase lanjut)

---

## 6. Sitemap (Struktur Halaman)

```
Login / Registrasi
└── Onboarding Wizard (data usaha, saldo awal, template kategori)
    └── Dashboard Rumah
        ├── Uang Masuk / Uang Keluar  (input transaksi)
        ├── Kategori
        ├── Penjualan (SQ → SO → DP → DO → INV → Pelunasan)
        ├── Pembelian (PR → PO → Penerimaan)
        ├── Persediaan / Stok Barang
        ├── Kas & Bank  (rekonsiliasi)
        ├── Laporan  (Laba Rugi, Neraca, Arus Kas, Catatan LK)
        ├── Pengaturan  (profil usaha, periode, pajak, pengguna)
        └── [Mode Akuntan]
            ├── Jurnal Umum
            ├── Buku Besar
            ├── Chart of Accounts
            ├── Penyesuaian & Reklasifikasi
            ├── Aset Tetap  (registrasi, penyusutan, revaluasi, disposisi)
            ├── Produksi  (job order costing)
            ├── Pajak
            └── Penutupan Periode
```

---

## 7. Prinsip Desain "Pembukuan Mudah, Mesin Akuntansi Penuh"

| Prinsip | Implementasi |
|---|---|
| **UI tanpa istilah akuntansi** | Pengguna awam hanya melihat "Uang Masuk", "Uang Keluar", "Untung/Rugi", "Tagihan" — bukan Debet/Kredit |
| **Jurnal otomatis di balik layar** | Tiap transaksi user → jurnal double-entry valid; selisih = 0 selalu |
| **Kategori = jalan pintas ke akun** | Pemetaan kategori → akun PSAK dikelola sistem (mode lanjutan untuk akuntan) |
| **Panduan kontekstual** | Tooltip, tips, FAQ bahasa Indonesia sederhana |
| **Mode ganda** | UI sederhana default; saklar "Mode Akuntan" membuka jurnal, buku besar, penyesuaian |
| **Koreksi mudah & aman** | Edit transaksi yang belum diposting; transaksi posted dikoreksi melalui jurnal reversal/koreksi dengan audit trail |
| **Laporan satu klik** | Format standar PSAK/SAK, pilihan kerangka (EMKM/ETAP/SAK Umum), siap cetak |

---

## 8. Kebutuhan Non-Fungsional

| Aspek | Spesifikasi |
|---|---|
| **Performa** | Input < 1 detik; dashboard < 2 detik; laporan 100.000 transaksi < 5 detik |
| **Integritas Data** | Double-entry selalu balance; transaksi tidak dihapus permanen (hanya void dengan jejak) |
| **Keamanan** | Enkripsi TLS & at-rest, login aman + 2FA opsional, kepatuhan UU PDP, isolasi data per entitas |
| **Auditability** | Audit trail permanen untuk semua perubahan; hash chain anti-tamper pada jurnal |
| **Ketersediaan** | Uptime 99.5%; backup harian otomatis |
| **Kemudahan Pakai** | Pengguna awam mencatat transaksi pertama < 5 menit tanpa pelatihan |
| **Skalabilitas** | Hingga jutaan transaksi per entitas tanpa degradasi; multi-tenant |

---

## 9. Rekomendasi Tech Stack

| Lapisan | Rekomendasi | Alasan |
|---|---|---|
| **Frontend** | React + TypeScript (Vite) — SPA statis, tanpa Next.js | UI modern & cepat; deploy ke CDN, tidak butuh SSR untuk aplikasi dashboard |
| **Backend / Accounting Engine** | **Go (Golang)** — engine double-entry sebagai pure package | Cepat olah data (perhitungan & agregasi laporan), satu binary statis → mudah deploy, mudah dikembangkan |
| **Database** | PostgreSQL | ACID — wajib untuk integritas double-entry |
| **DB Access** | sqlc (SQL type-safe) + pgx | SQL eksplisit & type-safe, tanpa overhead ORM |
| **Auth** | JWT + refresh token; OAuth (Google) | Login mudah untuk pemula |
| **Job Queue** | Redis + asynq | Posting massal, perhitungan pajak, export besar |
| **OCR (fase 3)** | Google Vision / Tesseract | Foto struk → transaksi |
| **Deployment** | Binary Go (image Docker ~15MB) + cloud (Render/Fly.io/AWS), frontend di CDN; CI/CD GitHub Actions | Sangat mudah & cepat deploy |

**Detail lengkap:** lihat [ARCHITECTURE.md](ARCHITECTURE.md).

---

## 10. Roadmap

### Fase 1 — MVP Inti (4–6 minggu)
- Setup project, login, onboarding wizard
- **Accounting Engine dasar**: double-entry engine (debet/kredit otomatis), COA inti PSAK, posting, balance check
- Saldo awal & setup usaha
- Dashboard rumah & pencatatan Uang Masuk/Keluar dengan kategori otomatis
- Laporan Laba Rugi & Arus Kas sederhana

### Fase 2 — Penjualan, Pembelian, Stok & Laporan (6–8 minggu)
- **Alur penjualan lengkap**: SQ (penawaran) → SO → DP (uang muka) → DO (pengiriman) → Invoice → pelunasan + pengingat jatuh tempo
- Alur PR → PO → penerimaan barang (GRN) → tagihan & pembayaran (+ DP pembelian)
- **Persediaan**: item master, stok masuk/keluar, penilaian FIFO/rata-rata (PSAK 14), stock opname
- Laporan Posisi Keuangan + Catatan atas Laporan Keuangan (dasar)
- Rekonsiliasi kas & bank, export PDF/Excel
- Mode Akuntan v1: jurnal, buku besar, COA
- Penutupan periode (lock + pindah laba ke ekuitas)

### Fase 3 — Produksi, Aset Tetap & Kepatuhan (8–10 minggu)
- **Aset Tetap lengkap**: registrasi, penyusutan multi-metode, revaluasi (ke ekuitas/OCI), disposisi & penjualan aset (PSAK 16)
- **Produksi sederhana (Job Order Costing)**: BOM, job per pesanan, pengeluaran material, WIP, finished goods, biaya per job
- Pajak: PPh Final UMKM 0,5%, PPN & e-Faktur, PPh 21/23/26
- Pajak tangguhan (PSAK 46), impairment (PSAK 48)
- Multi-usaha/multi-cabang + laporan konsolidasi sederhana (PSAK 65)
- Pilihan kerangka laporan: EMKM / ETAP / SAK Umum
- Audit trail penuh, import data, foto struk (OCR)
- Integrasi API DJP (e-Faktur, e-Bupot, e-SPT)

---

## 11. Model Bisnis & Monetisasi

| Model | Deskripsi |
|---|---|
| **Freemium** | Gratis untuk 1 usaha, hingga 100 transaksi/bulan — menarik pengguna pemula |
| **Tier Standar** | Rp 99.000–149.000/bulan — unlimited transaksi, invoice, laporan standar |
| **Tier Pro** | Rp 249.000–399.000/bulan — multi-usaha, mode akuntan, pajak lengkap, prioritas support |
| **Tier Konsultan/Akuntan** | Per-seat untuk akses multi-klien, white-label export |

**GTC (Go-To-Channel):** komunitas UMKM, webinar pajak & pembukuan, kemitraan dengan pendamping UMKM & koperasi, konten edukasi (Instagram/TikTok/YouTube), kolaborasi dengan bank (paket kredit UMKM).

---

## 12. Analisis Kompetitor

| Kompetitor | Kekuatan | Kelemahan | Posisi Kami |
|---|---|---|---|
| **BukuWarung / Kledo / Moka** | Fokus UMKM, UI sederhana, fitur pajak dasar | Jurnal tidak transparan, kurang untuk kebutuhan audit | Mode akuntan + laporan PSAK penuh |
| **Jurnal.id / Accurate / Zahir** | Fitur lengkap, multi-entitas | Harga mahal, learning curve tinggi | Kemudahan input ala aplikasi sederhana |
| **Excel / buku tulis** | Gratis, fleksibel | Rawan salah, tidak ada laporan otomatis | Otomatisasi + akurasi standar |
| **Xero / QuickBooks (internasional)** | Engine akuntansi matang | Tidak spesifik pajak Indonesia, harga USD | Lokalisasi pajak & regulasi Indonesia |

**Diferensiasi utama:** *kemudahan setara aplikasi catat sederhana + kedalaman akuntansi setara software profesional*, dalam satu produk berbahasa Indonesia.

---

## 13. Metrik Kesuksesan (KPI)

| KPI | Target |
|---|---|
| Waktu mencatat transaksi pertama (TTFT) | < 5 menit untuk pengguna baru |
| Akurasi laporan (selisih nol) | 100% — dijamin engine double-entry |
| Pengguna aktif mingguan (WAU) | > 60% dari pengguna terdaftar |
| Transaksi per pengguna aktif | ≥ 15 transaksi/bulan |
| Retensi 3 bulan | > 50% |
| Laporan siap kirim (audit/pajak/bank) | 100% sesuai format standar |
| Keluhan "sulit digunakan" | < 5% dari umpan balik |
| Konversi free → berbayar | > 10% |

---

## 14. Risiko & Mitigasi

| Risiko | Dampak | Mitigasi |
|---|---|---|
| Interpretasi PSAK/IFRS berubah | Salah penyajian | Konsultan akuntansi/auditor; update standar terjadwal |
| Engine double-entry salah | Laporan tidak balance | Unit test ketat, invariant "selalu balance", review auditor |
| Pengguna awam salah input | Laporan salah | Validasi, konfirmasi nominal besar, kategori jelas |
| Over-engineering UI | Produk jadi rumit | Prinsip "paling sederhana yang berfungsi"; uji pengguna rutin |
| Kompleksitas pajak Indonesia dinamis | Ketidakpatuhan | Tim pajak; update regulasi terjadwal |
| Migrasi data dari sistem lain | Data hilang/duplikat | Import bertahap, validasi saldo awal, dry-run |
| Persaingan dengan pemain besar | Sulit diferensiasi | Fokus pada kemudahan + kedalaman PSAK; konten edukasi |

---

## 15. Lampiran

- **Glosarium Dua Lapisan**: Lihat [GLOSSARY.md](GLOSSARY.md) — peta istilah UI awam ↔ istilah akuntansi/PSAK
- **User Stories per Modul**: Lihat [USER_STORIES.md](USER_STORIES.md) — user stories + acceptance criteria per modul
- **Data Model & ERD (double-entry)**: Lihat [DATA_MODEL.md](DATA_MODEL.md) — skema lengkap jurnal, COA, alur penjualan/pembelian, persediaan, aset, produksi, pajak, RLS, index
- **Spesifikasi Accounting Engine**: Lihat [ACCOUNTING_ENGINE.md](ACCOUNTING_ENGINE.md) — jurnal detail, COA inti, alur penjualan/pembelian, produksi, aset tetap, pajak, penutupan periode
- **Gap Analysis vs Kompetitor**: Lihat [GAP_ANALYSIS.md](GAP_ANALYSIS.md) — perbandingan fitur & prioritas penambahan
- **Arsitektur Teknis**: Lihat [ARCHITECTURE.md](ARCHITECTURE.md) — tech stack final (Go + React SPA), modular monolith, database, deployment, ADR

---

*Dokumen ini draft awal. Masukan dari konsultan akuntansi dan pendamping UMKM diperlukan sebelum finalisasi.*
