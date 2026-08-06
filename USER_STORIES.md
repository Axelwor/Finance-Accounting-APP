# User Stories per Modul
## Software Pembukuan Mudah dengan Mesin Akuntansi Standar IFRS/PSAK

**Lampiran PRD** — Software Pembukuan Mudah dengan Mesin Akuntansi Standar IFRS/PSAK  
**Versi:** 1.4 — Review  
**Tanggal:** 2026-08-06  
**Status:** Review  
**Owner:** Product + Engineering  
**Normative:** Tidak; acceptance/backlog traceability

---

## 1. Tujuan

User stories per modul dengan **acceptance criteria** yang dapat diuji. Setiap story memakai bahasa pengguna (lapisan UI); dampak akuntansi di belakang layar dijelaskan di kolom "Engine".

**Konvensi prioritas:** P0 = Fase 1/MVP inti; P1 = Fase 2 (penjualan, pembelian, stok, rekonsiliasi, Mode Akuntan awal); P2 = Fase 3+ (produksi, aset tetap, pajak lanjutan, konsolidasi, integrasi). Label "Must Have" di PRD §5.1 menunjukkan scope produk, sedangkan prioritas di dokumen ini mengikuti urutan delivery PRD §10.

---

## 2. Onboarding & Setup

### US-001 Setup Usaha (Wizard 5 menit) — P0
Sebagai pemilik usaha baru, saya ingin mengisi nama usaha, jenis usaha, mata uang, dan periode buku dalam satu alur, agar saya bisa mulai mencatat tanpa konfigurasi rumit.
- **Engine:** membuat tenant, template kategori sesuai jenis usaha, COA inti.
- **AC:** wizard ≤ 5 langkah; template kategori terisi otomatis; data tersimpan & dashboard terbuka.

### US-002 Saldo Awal — P0
Sebagai pemilik, saya ingin memasukkan saldo kas, bank, piutang, hutang, dan modal saat mulai, agar laporan saya akurat sejak hari pertama.
- **Engine:** posting jurnal pembuka (opening entry); selisih → 3105 Suspense.
- **AC:** input per akun; jika tidak balance muncul peringatan & saldo ditahan di Suspense; saldo awal tidak bisa di-void.

### US-003 Tambah Pengguna & Peran — P1
Sebagai pemilik, saya ingin menambah pengguna (admin, akuntan, staf) dengan peran berbeda, agar pekerjaan bisa dibagi aman.
- **AC:** undang via email; peran owner/accountant/admin/staff/consultant; RBAC diterapkan (lihat ARCHITECTURE §8.2).

---

## 3. Master Data & Konfigurasi Akuntansi

### US-004 Kelola Customer, Supplier, dan Termin — P1
Sebagai pemilik, saya ingin menyimpan data lengkap customer dan supplier beserta termin pembayaran, agar dokumen transaksi dapat dibuat tanpa data yang hilang.
- **Engine:** `customers`, `suppliers`, `payment_terms`, alamat, akun AR/AP default, dan credit limit.
- **AC:** kode pihak lawan unik per tenant; nama, alamat, kontak, email, telepon, dan NPWP dapat dikelola; termin default dapat dioverride per dokumen; NPWP wajib tervalidasi saat transaksi kena PPN; perubahan credit limit memiliki alasan dan audit trail.

### US-005 Kelola COA dan Kategori — P1
Sebagai pemilik atau akuntan, saya ingin membuat akun dan kategori sendiri tanpa merusak struktur laporan, agar COA sesuai usaha saya.
- **Engine:** setiap akun wajib memiliki report group dan account type yang kompatibel; akun grup tidak dapat diposting; akun bertransaksi hanya dinonaktifkan, bukan dihapus.
- **AC:** kode akun unik; mismatch tipe/kelompok ditolak; kategori UI dapat dipetakan ke akun; akun kustom muncul pada baris laporan yang benar; jurnal historis tetap memakai `account_id` lama.

### US-006 Konfigurasi Periode Akuntansi — P1
Sebagai akuntan, saya ingin mengatur periode bulanan, kuartalan, atau tahunan dan mengunci periode yang selesai, agar laporan tidak berubah tanpa otorisasi.
- **Engine:** hanya satu periode OPEN per tenant; posting ke CLOSED ditolak; closing entry dibuat saat tutup; unlock membatalkan closing entry dengan audit trail.
- **AC:** periode dan fiscal year dapat dikonfigurasi; transaksi di periode tertutup ditolak; unlock membutuhkan role/approval yang berwenang; status periode dan jurnal penutup dapat ditelusuri.

---

## 4. Pencatatan Harian (Uang Masuk / Keluar)

### US-010 Catat Uang Masuk — P0
Sebagai pemilik, saya ingin mencatat "Hari ini jualan Rp 500.000 tunai", agar saldo dan laporan langsung ter-update.
- **Engine:** `CASH_IN` → Dr Kas / Cr Pendapatan; jika barang dari stok → tambahan jurnal HPP.
- **AC:** pilih kategori → nominal → tanggal → keterangan; simpan < 1 detik; laporan Laba Rugi & Kas ter-update.

### US-011 Catat Uang Keluar — P0
Sebagai pemilik, saya ingin mencatat pengeluaran (sewa, listrik, gaji), agar beban tercatat & untung/rugi akurat.
- **Engine:** `CASH_OUT` → Dr Beban / Cr Kas.
- **AC:** kategori beban tersedia; nominal & tanggal wajib; muncul konfirmasi untuk nominal besar (> ambang).

### US-012 Pindah Kas ke Bank — P0
Sebagai pemilik, saya ingin memindahkan uang dari kas ke bank (atau sebaliknya), agar saldo keduanya benar.
- **Engine:** `TRANSFER` → Dr akun tujuan / Cr akun asal (bukan laba/rugi).
- **AC:** hanya antar akun CASH/BANK; saldo kedua akun berubah; tidak memengaruhi Laba Rugi.

### US-013 Ubah / Batalkan Transaksi — P0
Sebagai pemilik, saya ingin mengoreksi transaksi yang salah, agar catatan tetap benar.
- **Engine:** transaksi belum posted dapat diedit; transaksi posted dibatalkan melalui jurnal reversal dengan status jurnal asal `VOID`; tidak ada hapus permanen.
- **AC:** transaksi posted memerlukan alasan void; jurnal reversal memiliki `reversal_of_id`; satu jurnal asal hanya dapat memiliki satu reversal; jurnal asal tampil `VOID`, reversal tetap `POSTED`; laporan tidak menggandakan atau menghitung saldo transaksi yang telah dibalik; jejak audit tercatat.

### US-014 Template Transaksi Cepat — P1
Sebagai pemilik, saya ingin menyimpan transaksi yang sama berulang (mis. sewa bulanan) sebagai template, agar input lebih cepat.
- **Engine:** template → jurnal; recurring otomatis bila diaktifkan.
- **AC:** buat template dari transaksi; gunakan ulang dengan sekali klik; edit template tidak mengubah jurnal lama.

---

## 5. Penjualan (SQ → SO → DP → DO → INV → Pelunasan)

### US-020 Buat Penawaran (SQ) — P1
Sebagai pemilik, saya ingin membuat penawaran harga ke pelanggan, agar pelanggan bisa memutuskan sebelum pesan.
- **Engine:** SQ tanpa jurnal (komitmen).
- **AC:** pilih pelanggan (wajib) & item; total otomatis; status DRAFT → SENT → CONVERTED/EXPIRED.

### US-021 Konversi ke Pesanan (SO) & Terima DP — P1
Sebagai pemilik, saya ingin mengubah penawaran menjadi pesanan dan menerima uang muka, agar pesanan resmi & DP tercatat.
- **Engine:** SO tanpa jurnal; DP → Dr Kas/Bank / Cr 2201 Uang Muka Penjualan.
- **AC:** SO terkunci harga & qty; customer dan payment term wajib saat diproses; DP ≤ nilai SO (ditolak bila lebih); beberapa DP dapat dialokasikan ke satu SO; pembatalan/refund DP memiliki jurnal dan audit trail.

### US-026 DP Pembelian — P1
Sebagai pemilik, saya ingin membayar uang muka kepada supplier sebelum barang diterima, agar hak saya terhadap supplier tetap terlacak.
- **Engine:** Dr 1205 Uang Muka Pembelian / Cr Kas/Bank; saat tagihan direalisasi, DP mengurangi hutang usaha.
- **AC:** supplier dan purchase order wajib; DP tidak boleh melebihi nilai PO tanpa approval; DP dapat diterapkan sebagian ke supplier invoice; pesanan batal mendukung refund, kompensasi, atau DP hangus dengan alasan dan approval.

### US-022 Kirim Barang (DO) — P1
Sebagai pemilik, saya ingin mencatat pengiriman barang, agar stok berkurang & HPP tercatat.
- **Engine:** DO mengurangi stok; HPP diposting saat DO atau saat invoice sesuai kebijakan pengakuan pengendalian yang dikonfigurasi, tetapi tidak boleh diposting dua kali.
- **AC:** stok berkurang per qty yang dikirim; DO bertahap didukung; invoice tidak boleh melebihi qty yang belum ditagih; stok negatif ditolak secara default; pembatalan/retur DO membalik mutasi dan jurnal yang relevan.

### US-023 Terbitkan Invoice — P1
Sebagai pemilik, saya ingin menerbitkan invoice atas barang yang dikirim, agar piutang & pendapatan diakui.
- **Engine:** INV → Dr 1201 / Cr 4101; realisasi DP; PPN bila kena. HPP hanya diposting di INV jika kebijakan pengakuan HPP tidak diposting saat DO.
- **AC:** invoice per DO atau gabungan; invoice tidak boleh melebihi sisa qty DO yang belum ditagih; NPWP wajib bila kena PPN; tarif diambil dari `tax_rates` efektif tanggal invoice; DP direalisasi otomatis; sisa piutang benar; alamat tagih/kirim untuk dokumen final tervalidasi.

### US-024 Terima Pelunasan — P1
Sebagai pemilik, saya ingin mencatat pembayaran pelanggan, agar piutang menutup.
- **Engine:** Dr Kas / Cr 1201; alokasi ke invoice tertua dulu; overpayment customer → Cr 2402 CUSTOMER_DEPOSIT.
- **AC:** pembayaran parsial didukung; alokasi otomatis ke invoice; overpayment customer ditampung di 2402 dan dapat dikompensasi atau dikembalikan.

### US-025 Retur & Credit Note — P1
Sebagai pemilik, saya ingin mencatat pengembalian barang dari pelanggan, agar pendapatan & stok benar.
- **Engine:** CN → Dr 4201 Retur dan Dr 2202 untuk pembalikan PPN / Cr Piutang, Kas, atau saldo kredit; HPP dibalik dan stok ditambah hanya bila barang layak jual.
- **AC:** CN tidak boleh melebihi sisa invoice; retur parsial didukung; barang rusak masuk beban dan tidak menambah stok; `refund_method` menentukan potong piutang/refund/saldo kredit; CN setelah periode tutup masuk periode berjalan; tax breakdown dan jurnal reversal tercatat.

---

## 6. Pembelian (PR → PO → GRN → Tagihan → Bayar)

### US-030 Permintaan Pembelian (PR) — P1
Sebagai pemilik/staf, saya ingin membuat permintaan pembelian, agar pembelian terstruktur.
- **Engine:** PR tanpa jurnal; boleh tanpa supplier.
- **AC:** PR → APPROVED → ORDERED; ditolak → REJECTED.

### US-031 Purchase Order (PO) — P1
Sebagai pemilik, saya ingin membuat pesanan pembelian ke supplier, agar barang dipesan resmi.
- **Engine:** PO tanpa jurnal.
- **AC:** supplier wajib; harga & qty terkunci; status CONFIRMED → PARTIALLY_RECEIVED → RECEIVED.

### US-032 Terima Barang (GRN) — P1
Sebagai pemilik, saya ingin mencatat penerimaan barang dari supplier, agar stok bertambah & utang akrual tercatat.
- **Engine:** GRN → Dr 1301 / Cr 2105 Utang Belum Ditagih.
- **AC:** beberapa GRN per PO; over-delivery ditolak (perlu approval); barang rusak dicatat qty_rejected.

### US-033 Terima Tagihan Supplier — P1
Sebagai pemilik, saya ingin mencatat tagihan dari supplier, agar hutang usaha benar.
- **Engine:** tagihan → PPN masukan + reklasifikasi 2105 → 2101; realisasi DP pembelian.
- **AC:** selisih harga vs PO dicatat (tambah/kurang persediaan); NPWP supplier wajib bila PPN.

### US-034 Bayar Supplier — P1
Sebagai pemilik, saya ingin mencatat pembayaran ke supplier, agar hutang menutup.
- **Engine:** Dr 2101 / Cr Kas; alokasi ke tagihan; diskon pembelian → 4905; overpayment supplier → 1204 Piutang Lain-lain.
- **AC:** pembayaran parsial; alokasi otomatis; overpayment supplier tidak pernah diposting ke 2402 customer deposit; saldo 1204 dapat direfund atau dikompensasikan ke tagihan berikutnya.

### US-035 Retur Pembelian — P1
Sebagai pemilik, saya ingin mencatat pengembalian barang ke supplier, agar hutang & stok benar.
- **Engine:** Dr 2101 / Cr 1301 + balik PPN masukan (1203).
- **AC:** retur ≤ tagihan; metode: potong hutang/refund/saldo kredit.

---

## 7. Persediaan

### US-040 Kelola Item — P1
Sebagai pemilik, saya ingin menambah/edit barang/jasa dengan harga & akun default, agar transaksi cepat.
- **Engine:** items + item_price_lists.
- **AC:** kode unik; metode penilaian per item (FIFO/rata-rata); akun default (pendapatan/HPP/persediaan).

### US-041 Harga Multi-Level — P1
Sebagai pemilik, saya ingin menetapkan harga berbeda per pelanggan/grup (retail/grosir), agar penawaran akurat.
- **AC:** price list per item; harga aktif sesuai tanggal; perubahan tidak mengubah dokumen lama.

### US-041A Validasi Barang dan Jasa — P1
Sebagai pemilik, saya ingin sistem membedakan barang dan jasa dengan benar, agar jasa tidak masuk stok dan barang memiliki costing yang lengkap.
- **Engine:** `goods` tracked memakai inventory account + costing method; `service` memakai revenue recognition method dan tidak boleh menghasilkan inventory movement.
- **AC:** service pada DO/GRN ditolak `SERVICE_INVENTORY_INVALID`; goods tracked tanpa inventory account/costing ditolak `GOODS_POLICY_INVALID`; perubahan item policy tidak mengubah snapshot dokumen lama.

### US-041B Kontrak dan Pengakuan Pendapatan Jasa — P2
Sebagai akuntan, saya ingin mengelola kewajiban pelaksanaan dan jadwal pengakuan pendapatan jasa, agar PSAK 72 dapat ditelusuri.
- **Engine:** `revenue_contracts`, `performance_obligations`, dan `revenue_recognition_schedules`; total pengakuan tidak boleh melebihi allocated price.
- **AC:** kontrak multi-element mengalokasikan transaction price berdasarkan SSP; pengakuan point-in-time/over-time/milestone didukung; setiap pengakuan menghasilkan jurnal idempoten; schedule dapat direkonsiliasi ke invoice dan contract liability.

### US-042 Transfer Antar Gudang — P1
Sebagai pemilik, saya ingin memindahkan stok antar lokasi, agar stok per gudang benar.
- **Engine:** TRANSFER_IN/OUT; total stok tetap.
- **AC:** stok keluar gudang A & masuk gudang B; riwayat mutasi.

### US-043 Stock Opname — P1
Sebagai pemilik, saya ingin mencocokkan stok fisik dengan sistem, agar selisih terdeteksi & dijurnal.
- **Engine:** selisih → jurnal penyesuaian (lebih → 4907; kurang → 5907).
- **AC:** status DRAFT → COUNTED → APPROVED; selisih wajib ada alasan; jurnal dibuat otomatis saat APPROVED.

### US-044 Peringatan Stok Menipis — P2
Sebagai pemilik, saya ingin mendapat notifikasi saat stok di bawah minimum, agar tidak kehabisan barang.
- **AC:** notifikasi `stock_low` di dashboard; ambang per item.

---

## 8. Kas & Bank

### US-050 Rekonsiliasi Bank — P1
Sebagai pemilik, saya ingin mencocokkan mutasi bank dengan catatan, agar saldo kas akurat.
- **Engine:** bank_statements + bank_statement_lines + bank_reconciliations.
- **AC:** hanya akun bertipe BANK yang dapat direkonsiliasi; import CSV menolak/menandai baris duplikat tanpa menggandakan transaksi; tanggal, nominal, dan referensi tersimpan; cocokkan otomatis/manual; selisih wajib 0 saat RECONCILED; jurnal penyesuaian harus memiliki alasan dan audit trail.

### US-050A Approval Transaksi — P2
Sebagai pemilik, saya ingin transaksi berisiko melewati approval oleh orang berbeda, agar pemisahan tugas terjaga.
- **Engine:** `approvals`; transaksi yang termasuk aturan approval tenant terkunci sampai APPROVED; pembuat tidak boleh menjadi approver; reject wajib menyimpan catatan.
- **AC:** aturan approval dapat dikonfigurasi per tenant/jenis transaksi; transaksi di luar aturan dapat diposting sesuai RBAC; status PENDING/APPROVED/REJECTED terlihat; transaksi yang ditolak tidak dapat diposting; semua keputusan tercatat di audit trail.

### US-051 Kas Kecil (Imprest) — P1
Sebagai pemilik, saya ingin mengelola kas kecil dengan dana tetap, agar pengeluaran kecil tercatat rapi.
- **Engine:** imprest — jurnal hanya saat pembentukan & pengisian.
- **AC:** pembentukan dana; pengisian kembali (reimburse) per beban; selisih opname diakui.

### US-052 Transaksi Berulang — P1
Sebagai pemilik, saya ingin sewa/langganan tercatat otomatis tiap bulan, agar tidak lupa.
- **Engine:** recurring_templates + instances (RCR-...); idempoten.
- **AC:** jadwal harian/mingguan/bulanan/kuartalan/tahunan; auto-post atau reminder; jatuh di periode terkunci → ditunda.

---

## 9. Aset Tetap

### US-060 Registrasi Aset — P2
Sebagai akuntan, saya ingin mencatat pembelian aset tetap, agar nilai aset & penyusutan tercatat.
- **Engine:** Dr 1401 / Cr Kas-Hutang; metode penyusutan.
- **AC:** pilih metode (garis lurus/saldo menurun/unit produksi); masa manfaat & residu; komponen aset didukung.

### US-061 Penyusutan Otomatis — P2
Sebagai akuntan, saya ingin penyusutan dihitung otomatis per periode, agar beban akurat.
- **Engine:** Dr 5206 / Cr 1402 per periode.
- **AC:** jadwal penyusutan; jurnal dibuat saat tutup periode; tidak dobel di periode sama.

### US-062 Revaluasi & Disposisi — P2
Sebagai akuntan, saya ingin mencatat revaluasi dan penjualan/penghapusan aset, agar nilai buku benar.
- **Engine:** revaluasi naik → OCI (3401); disposisi → untung/rugi 4903/5903.
- **AC:** revaluasi up/down; disposisi menghitung untung/rugi; aset nonaktif tidak disusutkan lagi.

### US-063 Penurunan Nilai (Impairment) — P2
Sebagai akuntan, saya ingin mencatat penurunan nilai aset, agar laporan tidak melebih-lebihkan nilai.
- **AC:** impairment → Dr 5207 / Cr 1401; dicatat di asset_transactions.

---

## 10. Produksi (Job Order Costing)

### US-070 Buat Job Produksi — P2
Sebagai pemilik, saya ingin membuat job produksi dengan target qty & BOM, agar biaya per job terhitung.
- **Engine:** jobs + bom; WIP 1303.
- **AC:** job per produk jadi; qty target; BOM komponen.

### US-071 Catat Biaya Job — P2
Sebagai pemilik, saya ingin mencatat material/tenaga/overhead per job, agar biaya produksi akurat.
- **Engine:** Dr 1303 WIP / Cr Persediaan-Kas.
- **AC:** per jenis biaya (material/labor/overhead); total per job real-time.

### US-072 Selesaikan Job (Barang Jadi) — P2
Sebagai pemilik, saya ingin memindahkan WIP ke barang jadi saat produksi selesai, agar stok & HPP benar.
- **Engine:** Dr 1304 / Cr 1303; selisih overhead → variance.
- **AC:** qty_completed; unit cost dihitung; variance dibukukan (4902/5901).

---

## 11. Pajak

### US-080 PPN (Masukan/Keluaran) — P2
Sebagai pemilik PKP, saya ingin PPN dihitung otomatis di invoice & tagihan, agar kewajiban PPN tercatat benar.
- **Engine:** PPN keluaran → 2202; PPN masukan → 1203; tarif diambil dari `tax_rates` berdasarkan tanggal efektif dan jenis transaksi.
- **AC:** status PKP wajib; tarif tidak di-hard-code; NPWP tervalidasi bila diperlukan; pembulatan konsisten; PPN masukan tidak pernah diposting sebagai liabilitas; retur/credit note membalik PPN; laporan PPN dapat direkonsiliasi dengan detail `tax_breakdowns`.

### US-081 PPh Final UMKM — P2
Sebagai pemilik UMKM yang memilih skema pajak final, saya ingin pajak dihitung dari omset bulanan, agar kewajiban pajak terpenuhi.
- **Engine:** Dr 5208 / Cr 2203 berdasarkan tarif dan periode yang dikonfigurasi; saat setor → Dr 2203 / Cr Kas.
- **AC:** sistem meminta konfirmasi skema pajak dan status wajib pajak; ambang omzet dan tarif berasal dari konfigurasi regulasi efektif, bukan asumsi permanen 0,5%/Rp4,8 M; perubahan status di tengah tahun menghentikan perhitungan mulai periode yang benar; jurnal akrual tidak dobel; laporan setor dapat ditelusuri ke sumber omzet.

### US-082 Penyisihan Piutang (ECL) — P2
Sebagai akuntan, saya ingin penyisihan piutang dihitung dari umur piutang, agar nilai piutang realistis.
- **Engine:** ECL per bucket aging → Dr 5205 / Cr 1202; write-off → Dr 1202 / Cr 1201.
- **AC:** bucket 0-30/31-60/61-90/>90; persentase konfigurabel; write-off wajib approval; pemulihan → 4906.

### US-083 Pajak Tangguhan — P2
Sebagai akuntan, saya ingin perbedaan temporer (mis. penyusutan fiskal vs akuntansi) menghasilkan pajak tangguhan.
- **Engine:** Dr 1206 / Cr 5904 (atau sebaliknya) per akhir periode (PSAK 46).
- **AC:** dihitung per periode; perubahan tarif → penyesuaian.

---

## 12. Laporan & Penutupan

### US-090 Laporan Laba Rugi / Neraca / Arus Kas — P0
Sebagai pemilik, saya ingin laporan dasar satu klik, agar mengetahui kondisi usaha tanpa memahami jurnal.
- **Engine:** agregasi jurnal POSTED; akun dipetakan melalui report group/report_mappings; laporan tidak memasukkan VOID.
- **AC:** pilih tanggal/periode; laporan menghasilkan total yang konsisten dengan Trial Balance; Neraca memenuhi Aset = Liabilitas + Ekuitas; angka periode berjalan ter-update setelah posting; export menghasilkan file yang dapat dibuka.

### US-090A Pilihan Kerangka Laporan — P2
Sebagai akuntan, saya ingin menyajikan laporan dalam kerangka EMKM, ETAP, atau SAK Umum, agar format sesuai kebutuhan entitas.
- **Engine:** data pencatatan tetap satu; presentation layer memilih kerangka dan mapping laporan.
- **AC:** kerangka tersimpan per laporan; setiap kerangka menampilkan komponen yang diwajibkan; perubahan kerangka tidak mengubah jurnal; perbedaan penyajian dapat ditelusuri ke mapping.

### US-091 Trial Balance & GL Reports — P1
Sebagai akuntan, saya ingin melihat neraca saldo, jurnal register, dan buku besar per akun, agar bisa audit.
- **Engine:** Trial Balance, Jurnal Register, Buku Besar, Mutasi Kas (§23 engine).
- **AC:** drill-down dari laporan ke jurnal sumber; perbandingan antar periode.

### US-092 Penutupan Periode — P1
Sebagai pemilik/akuntan, saya ingin menutup periode dengan aman, agar data periode terkunci & laba dipindah ke ekuitas.
- **Engine:** closing entry (P&L → 3301 → 3201); lock periode.
- **AC:** ceklist dokumen belum lengkap tampil; Suspense terbuka → ditolak; unlock butuh otorisasi & membatalkan closing entry.

### US-093 Laporan per Dimensi / Budget — P2
Sebagai pemilik multi-cabang, saya ingin melihat laba per cabang/proyek & membandingkan budget, agar kontrol keuangan lebih tajam.
- **Engine:** `journal_lines.dimension_ids` untuk dimensi; `budgets`/`budget_lines` untuk anggaran; realisasi berasal dari jurnal POSTED saja.
- **AC:** dimensi dapat dipilih pada transaksi yang relevan; filter laporan per dimensi; periode budget valid; realisasi vs budget menunjukkan nominal dan persentase variance; alert > X% dapat dikonfigurasi; VOID tidak masuk realisasi.

---

## 13. Lampiran & Audit

### US-100 Lampirkan Bukti — P2
Sebagai pemilik, saya ingin melampirkan foto struk/PDF ke transaksi, agar bukti tersimpan & mudah di-audit.
- **Engine:** `attachments` menyimpan owner polymorphic, file_key, MIME, ukuran, status OCR; file disimpan di object storage terenkripsi, bukan di JSONB jurnal.
- **AC:** tipe file dan ukuran divalidasi; upload sebelum/bersamaan/sesudah posting sesuai kebijakan; attachment dapat ditemukan dari dokumen dan drill-down jurnal; dokumen tanpa bukti dapat ditandai; hapus attachment memiliki audit trail; OCR hanya menghasilkan draft yang harus dikonfirmasi (fase 3).

### US-101 Lihat Audit Trail — P1
Sebagai akuntan, saya ingin melihat riwayat lengkap perubahan (siapa, apa, kapan), agar bisa diaudit.
- **AC:** `audit_logs` append-only; sebelum/sesudah tampil; filter per entitas/pengguna; aksi post/void/close/unlock tercatat; audit log tidak dapat diubah atau dihapus melalui role aplikasi.

---

## 14. Konsolidasi & Sewa (Fase Lanjut)

### US-110 Konsolidasi Multi-Entitas — P2
Sebagai pemilik grup usaha, saya ingin laporan gabungan antar cabang/entitas, agar melihat kinerja total.
- **Engine:** eliminasi transaksi antar-entitas (PSAK 65).
- **AC:** entitas induk-anak; eliminasi piutang-hutang & penjualan antar entitas; laporan konsolidasi.

### US-111 Sewa (PSAK 73) — P2
Sebagai akuntan, saya ingin kontrak sewa dicatat sebagai aset hak-guna & liabilitas sewa, agar sesuai PSAK 73.
- **AC:** kontrak sewa → 1701/2301; pembayaran → pokok + bunga; sewa jangka pendek/bernilai rendah → beban langsung.

---

*Dokumen ini referensi untuk tim produk & engineering. Prioritas dapat disesuaikan; setiap story siap dipecah menjadi task implementasi.*
