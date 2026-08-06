# Glosarium Dua Lapisan
## Software Pembukuan Mudah dengan Mesin Akuntansi Standar IFRS/PSAK

**Lampiran PRD** — Software Pembukuan Mudah dengan Mesin Akuntansi Standar IFRS/PSAK  
**Versi:** 1.2 — Review  
**Tanggal:** 2026-08-06  
**Status:** Review  
**Owner:** Product + Accounting  
**Normative:** Ya untuk terminology UI dan akuntansi

---

## 1. Tujuan

Menjembatani **bahasa pengguna awam** (lapisan UI) dengan **istilah akuntansi/PSAK** (lapisan mesin). Setiap istilah yang muncul di UI atau dokumen teknis memiliki definisi konsisten — satu konsep, satu istilah.

---

## 2. Peta Istilah Dua Lapisan

| Istilah Pengguna (UI) | Istilah Akuntansi (Engine) | Penjelasan |
|---|---|---|
| **Uang Masuk** | `CASH_IN` / Penerimaan Kas | Transaksi yang menambah kas/bank (penjualan tunai, penerimaan piutang, modal, pinjaman) |
| **Uang Keluar** | `CASH_OUT` / Pengeluaran Kas | Transaksi yang mengurangi kas/bank (beban, pembayaran hutang, pembelian tunai) |
| **Pindah Uang** | `TRANSFER` | Memindahkan kas antar akun CASH/BANK; bukan transaksi laba/rugi |
| **Untung / Rugi** | Laba Rugi (Profit & Loss) | Selisih pendapatan − beban pada periode tertentu |
| **Tagihan** | Invoice / Piutang Usaha (AR) | Dokumen penagihan ke pelanggan |
| **Hutang** | Utang Usaha (AP) / Liabilitas | Kewajiban membayar ke pemasok/ pihak lain |
| **Piutang** | Piutang Usaha (AR) / Aset | Hak menagih ke pelanggan |
| **Kategori** | Akun (COA) | Pengelompokan transaksi; tiap kategori terpetakan ke akun PSAK |
| **Barang** | Persediaan (Inventory) | Stok dagang; dinilai FIFO/rata-rata (PSAK 14) |
| **Stok Menipis** | Reorder Point / Stok Minimum | Ambang stok yang memicu pengingat pembelian |
| **Aset** | Aset Tetap (Fixed Asset) | Barang berumur > 1 tahun: mesin, kendaraan, bangunan (PSAK 16) |
| **Penyusutan** | Depresiasi | Alokasi beban aset selama masa manfaat |
| **Saldo Awal** | Opening Balance | Saldo akun saat pertama kali mulai memakai sistem |
| **Tutup Buku** | Closing Period | Mengunci periode & memindahkan laba ke ekuitas |
| **Uang Muka** | Down Payment / Uang Muka Penjualan | Pembayaran di muka; liabilitas (2201) di sisi penjualan |
| **Retur** | Credit Note / Purchase Return | Pengembalian barang; mengurangi pendapatan/hutang |
| **Diskon** | Diskon Penjualan / Pembelian | Pengurang nilai transaksi |
| **Pajak** | PPN / PPh | Pajak Pertambahan Nilai & Pajak Penghasilan |

---

## 3. Istilah Teknis Inti (Engine)

### 3.1 Jurnal & Posting
| Istilah | Definisi |
|---|---|
| **Jurnal (Journal Entry)** | Catatan double-entry: minimal 2 baris, total debet = total kredit |
| **Debet / Kredit** | Sisi kiri/kanan jurnal; bukan "tambah/kurang" — tergantung jenis akun |
| **Posting** | Proses menyimpan jurnal ke buku besar |
| **Void** | Membatalkan jurnal dengan **jurnal balik** (bukan menghapus) — jejak audit terjaga |
| **Hash Chain** | Rantai hash (SHA-256) antar jurnal — anti-tamper |
| **Source Ref** | Nomor dokumen asal (INV-2026-000123) yang memicu jurnal |
| **Intent** | Jenis transaksi terstruktur (`SALES_INVOICE`, `CASH_IN`, ...) — dasar jurnal otomatis |
| **Idempotensi** | Posting intent yang sama tidak boleh menghasilkan jurnal ganda |

### 3.2 Akun & Laporan
| Istilah | Definisi |
|---|---|
| **COA (Chart of Accounts)** | Daftar akun; kelompok laporan (Aset/Liabilitas/Ekuitas/Pendapatan/Beban) + tipe akun (BANK, AR, AP, INVENTORY, ...) |
| **Akun Grup vs Detail** | Grup = penjumlahan otomatis anak; hanya detail yang boleh diposting |
| **Trial Balance** | Neraca saldo — semua akun debet vs kredit harus seimbang |
| **Neraca (Posisi Keuangan)** | Aset = Liabilitas + Ekuitas pada satu waktu |
| **Laba Rugi** | Pendapatan − Beban pada satu periode |
| **Arus Kas** | Pergerakan kas masuk/keluar (operasi, investasi, pendanaan) |
| **ECL** | Expected Credit Loss — penyisihan piutang tak tertagih (PSAK 71) |
| **NRV** | Nilai realisasi neto persediaan (PSAK 14) |
| **OCI** | Penghasilan komprehensif lain (mis. surplus revaluasi) |

### 3.3 Transaksi & Dokumen
| Istilah | Definisi |
|---|---|
| **SQ → SO → DP → DO → INV** | Alur penjualan: Penawaran → Pesanan → Uang Muka → Pengiriman → Invoice |
| **PR → PO → GRN** | Alur pembelian: Permintaan → Pesanan → Penerimaan Barang |
| **HPP (COGS)** | Harga Pokok Penjualan — beban pokok barang yang terjual |
| **WIP** | Work In Progress — barang dalam proses produksi |
| **BOM** | Bill of Materials — daftar komponen produk |
| **Job Costing** | Pembebanan biaya (material, tenaga, overhead) per job/pesanan |

### 3.4 Pajak & Standar
| Istilah | Definisi |
|---|---|
| **PPN Masukan** | PPN atas pembelian — **aset** (dapat dikreditkan), bukan utang |
| **PPN Keluaran** | PPN atas penjualan — **liabilitas** (disetor ke negara) |
| **PPh Final UMKM** | Pajak final berdasarkan skema wajib pajak dan regulasi efektif; 0,5% dan ambang Rp 4,8 M hanya berlaku bila memenuhi ketentuan periode terkait |
| **Pajak Tangguhan** | Dampak pajak dari perbedaan temporer (PSAK 46) |
| **SAK EMKM / ETAP / Umum** | Kerangka penyajian laporan — dari satu sumber pencatatan |

---

## 4. Peta Kode Akun (Ringkas)

| Kode | Akun | Kategori UI Terkait |
|---|---|---|
| 1101 | Kas | Uang Masuk/Keluar tunai |
| 1102 | Bank | Transfer bank, rekonsiliasi |
| 1201 | Piutang Usaha | Tagihan ke pelanggan |
| 1301 | Persediaan | Stok barang dagang |
| 1401 | Aset Tetap | Mesin, kendaraan, bangunan |
| 2101 | Hutang Usaha | Tagihan dari supplier |
| 2201 | Uang Muka Penjualan | DP dari pelanggan |
| 2202 | Utang PPN | PPN keluaran |
| 2203 | Utang PPh | PPh yang dipotong/disetor |
| 3101 | Modal | Setoran pemilik |
| 3301 | Laba Berjalan | Untung/rugi periode berjalan |
| 4101 | Pendapatan Penjualan | Penjualan barang/jasa |
| 5101 | HPP | Pokok barang terjual |
| 5201 | Beban Gaji | Gaji karyawan |

*(COA lengkap: lihat [ACCOUNTING_ENGINE.md](ACCOUNTING_ENGINE.md) §3.)*

---

## 5. Aturan Pemakaian Istilah

1. **UI awam** selalu pakai istilah lapisan pengguna — istilah akuntansi tidak muncul (kecuali Mode Akuntan).
2. **Dokumen teknis** (engine, data model, arsitektur) memakai istilah akuntansi konsisten — nama kolom & enum bahasa Inggris (`payable_cents`, `receivable_cents`).
3. **Satu konsep = satu istilah** — tidak boleh berganti-ganti (mis. jangan pakai "hutang" dan "utang" bergantian untuk konsep sama; standar: **Hutang** = liabilitas, **Piutang** = aset).
4. Kode akun mengikuti ACCOUNTING_ENGINE.md §3 — referensi tunggal, tidak diduplikasi di UI.

---

*Dokumen ini referensi untuk tim produk, UI/UX, dan engineering. Perbarui saat istilah baru diperkenalkan.*
