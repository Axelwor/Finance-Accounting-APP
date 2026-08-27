# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

Primary (wins design trade-offs): **Akuntan profesional / pembukuan profesional** — menyusun laporan untuk usaha sendiri maupun klien; butuh jurnal penuh, buku besar, penyesuaian/reklasifikasi, dan export berkualitas audit. Terkonfirmasi pemilik sesi (2026-08-26).

Audience lain (dari PRD v4.4): pemilik usaha pemula (warung, kafe, online shop, jasa), pemilik UMKM tumbuh (omzet hingga Rp 4,8 M/tahun), pemilik multi-usaha/cabang, konsultan/pendamping UMKM dengan akses multi-klien.

## Product Purpose

Aplikasi pembukuan web berkonsep dua lapisan: lapisan pengguna menyederhanakan pencatatan transaksi dalam bahasa sehari-hari ("Hari ini jualan Rp 500.000 tunai"), sementara lapisan mesin (accounting engine) menerjemahkan setiap input menjadi jurnal double-entry yang benar sesuai SAK Umum (PSAK berbasis IFRS). Keberhasilan berarti: pencatatan yang mudah menghasilkan laporan berkualitas audit yang aman untuk pajak, bank, investor, dan auditor.

## Positioning

Mekanisme dua lapisan yang tidak bisa diklaim jujur oleh pembukuan sederhana biasa: satu sumber pencatatan sederhana menghasilkan laporan dalam pilihan kerangka EMKM / SAK ETAP / SAK Umum, dengan kepatuhan engine penuh — pajak tangguhan (PSAK 46), aset tetap & revaluasi (PSAK 16), persediaan FIFO/rata-rata (PSAK 14), sewa ROU (PSAK 73), ECL (PSAK 71/48), konsolidasi antar-entitas (PSAK 65), pendapatan kontrak (PSAK 72).

## Operating Context

- Bahasa UI: Indonesia (`lang="id"`); terminologi UI mengikuti GLOSSARY.md di root repo.
- Alur jualan: SQ → SO → DP → DO → INV → pelunasan; alur belanja: PR → approval → PO → GRN → tagihan → bayar.
- Regulasi pajak Indonesia: PPh Final UMKM, PPN (dasar e-Faktur), PPh 21 TER (PMK 168/2023), PPh 23/26, PPh 22 & PPh 4(2), UU KUP (NPWP per entitas).
- Deployment: Docker Compose di VPS (prod) dan lokal dev; API Go modular monolith + PostgreSQL dengan RLS tenant-scoped; SPA React/Vite.
- Multi-tenant SaaS: satu tenant = satu periode OPEN; periode tidak boleh overlap.
- Pengguna memilih kerangka penyajian (EMKM/ETAP/SAK Umum) saat membuat laporan — perbedaan antar kerangka ditangani engine, bukan pengguna.

## Capabilities and Constraints

- Modul Must Have (v1): Dashboard, Uang Masuk/Keluar, Kategori Cerdas, alur Penjualan lengkap, alur Pembelian, Persediaan, Kas & Bank, Saldo Awal, Laporan Otomatis, Setup Usaha (onboarding wizard).
- Mode Lanjutan (v2, prioritas pengguna primer): Jurnal Umum, Buku Besar, COA, Penyesuaian & Reklasifikasi, Aset Tetap, dan lanjutan lainnya.
- Constraint engine (tidak boleh dilanggar UI apa pun): jurnal yang sudah posting immutable — koreksi hanya via reversal/correction; hash-chain head diserialisasi per tenant; posting command wajib idempotency key; outbox event ditulis dalam transaksi yang sama dengan jurnal.
- Belum diputuskan (jangan dianggap ada): integrasi penuh e-Faktur/e-Bupot/e-SPT API DJP baru direncanakan fase 3+.

## Brand Commitments

- Nama produk: **Trexo** (terkonfirmasi pemilik sesi, 2026-08-26, bersifat binding). Seluruh teks UI, title, dan metadata web sudah memakai Trexo (2026-08-26); storage key internal (`ledgerly.*`) sengaja dipertahankan agar session/draft pengguna tidak hilang saat upgrade.
- Standar visual korporat bersifat mengikat: konten halaman tampil penuh tanpa terpotong di sisi kanan, tata letak rapi, font profesional, konsisten di semua fitur.
- Tipografi terpasang dan dipertahankan: Inter (UI) + JetBrains Mono (angka finansial).

## Evidence on Hand

- Spesifikasi root: PRD.md (v4.4), ACCOUNTING_ENGINE.md, DATA_MODEL.md, ARCHITECTURE.md, USER_STORIES.md, GLOSSARY.md.
- docs/UI_CONTRACT.md, docs/API_CONTRACT.md, QA reports 2026-08 (docs/QA_*).
- Tidak ada testimoni, studi kasus, data pelanggan, atau press release nyata — pekerjaan desain tidak boleh membuatnya.

## Product Principles

1. **Keberbenaran akuntansi tak bisa dinegosiasi.** Setiap input menghasilkan double-entry yang valid; laporan harus layak audit — keputusan UI tidak boleh mengorbankan invarian engine.
2. **Kedalaman profesional kelas satu.** Permukaan jurnal, buku besar, dan penyesuaian adalah permukaan inti produk, bukan mode tempelan.
3. **Kompleksitas tinggal di engine.** Standar dan istilah teknis tidak pernah bocor ke alur pencatatan pengguna awam.
4. **Satu sumber kebenaran, banyak kerangka.** Pencatatan tunggal; pilihan kerangka laporan terjadi di output, tidak pernah di input.
5. **Terlacok karena tercatat.** Jejak audit penuh, jurnal immutable, dan reversibility adalah nilai produk yang harus terlihat, bukan disembunyikan.

## Accessibility & Inclusion

Target wajib: **WCAG 2.2 AA+** (terkonfirmasi pemilik sesi, 2026-08-26) — mencakup kontras, navigasi keyboard penuh, label screen reader, target sentuh, dan fokus visible; penuhi AAA pada kontras teks utama bila layak.
