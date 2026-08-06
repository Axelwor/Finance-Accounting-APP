# Spesifikasi Accounting Engine
## Mesin Akuntansi Double-Entry Sesuai IFRS/PSAK Indonesia

**Lampiran PRD** — Software Pembukuan Mudah dengan Mesin Akuntansi Standar IFRS/PSAK  
**Versi:** 1.9 — Review  
**Tanggal:** 2026-08-06  
**Status:** Review  
**Owner:** Accounting + Engineering  
**Normative:** Ya untuk invariant dan perilaku posting

---

## 1. Tujuan & Batas

Accounting Engine adalah **modul inti di balik layar** yang menerjemahkan setiap tindakan pengguna menjadi **jurnal double-entry yang valid dan sesuai PSAK/IFRS**. Engine adalah **library murni (pure library)** — tanpa akses database langsung, tanpa HTTP — sehingga dapat diuji secara matematis dan dijamin **selalu balance**.

### Aturan Non-Negotiable
1. **Setiap transaksi menghasilkan jurnal dengan total debet = total kredit.** Jika tidak, transaksi ditolak.
2. **Tidak ada penghapusan permanen.** Kesalahan dikoreksi dengan jurnal balik (void/reversal) yang tercatat.
3. **Setiap baris jurnal wajib menyimpan referensi sumber** (dari transaksi/aksi apa), sehingga bisa di-trace (drill-down).
4. **Semua angka disimpan sebagai integer (sen) — never float.** Menghindari error pembulatan.
5. **Selalu balance invariant:** fungsi `balanceCheck(ledger)` wajib dipanggil setelah setiap posting.
6. **Anti-tamper:** setiap jurnal menyimpan hash jurnal sebelumnya (hash chain) — perubahan data setelah posting langsung terdeteksi audit trail.
7. **Posting hanya ke periode yang terbuka.** Transaksi dengan tanggal pada periode terkunci ditolak (kecuali jurnal koreksi yang diotorisasi).
8. **Idempotensi:** posting intent yang sama dua kali (retry/ganda) harus menghasilkan satu jurnal — dicegah dengan unique constraint pada `(source_ref, intent_type)`.

### Alur Data
```
Aksi Pengguna (UI sederhana / Mode Akuntan)
   ↓  "intent" terstruktur (mis. SALES_INVOICE, CASH_IN, PRODUCTION_FINISH)
Accounting Engine (pure, diuji)
   ↓  menghasilkan jurnal + efek pada sub-ledger (stok, piutang, aset)
Posting ke Database (PostgreSQL, transaksi ACID)
   ↓
Laporan (Laba Rugi, Neraca, Arus Kas, Catatan LK)
```

---

## 2. Prinsip Inti

| Prinsip | Implementasi |
|---|---|
| **Double-entry selalu** | Debet = Kredit untuk setiap journal entry; engine menolak yang tidak balance |
| **Posting berbasis intent** | UI awam mengirim intent (mis. "uang masuk dari penjualan"), engine yang menentukan akun & jurnal |
| **Sub-ledger terpisah** | Piutang, hutang, persediaan, aset tetap, job produksi dicatat di sub-ledger + direkap ke buku besar |
| **Integer cents** | Semua nominal integer; pembulatan eksplisit & terdokumentasi (§2.1) |
| **Mata uang** | Multi-mata uang dengan kurs; selisih kurs dibukukan (lihat §19) |
| **Audit trail** | Setiap perubahan: user, timestamp, sebelum/sesudah, alasan + hash chain |

### 2.1 Pembulatan & Presisi
- Seluruh nilai disimpan integer dalam **satuan sen** (Rupiah × 100).
- Pembulatan PPN ke rupiah penuh sesuai aturan DJP (dikonfigurasi).
- Selisih pembulatan dicatat pada baris **akun pajak** (bukan akun lawan), agar tidak mencemari akun kas/piutang.
- Semua perhitungan (diskon, pajak, depresiasi, selisih kurs) dijalankan dengan presisi penuh; pembulatan hanya di titik pencatatan akhir.
- **Aturan pembulatan default:** 0,5 ke atas dibulatkan ke atas (round half up) — dapat dikonfigurasi per entitas.

### 2.2 Konsistensi Tanggal & Waktu
- `tanggal_transaksi` adalah tanggal kejadian ekonomis (dokumen), bukan tanggal input.
- `created_at` adalah timestamp posting (untuk urutan hash chain & audit).
- Laporan periode dan cut-off menggunakan tanggal transaksi (`entry_date`); jurnal register dapat diurutkan berdasarkan `entry_date` lalu `created_at`. `created_at` hanya menentukan urutan posting/hash chain dan audit.

---

## 3. Chart of Accounts Inti (Kerangka PSAK/SAK Umum)

### 3.0 Dua Lapisan Klasifikasi COA

Setiap akun memiliki **dua atribut klasifikasi** yang saling melengkapi:

1. **Kelompok Laporan (Report Group)** — menentukan posisi di laporan (Neraca vs Laba Rugi): **Aset, Liabilitas, Ekuitas, Pendapatan, Beban**.
2. **Tipe Akun (Account Type)** — kategori detail yang mengendalikan **perilaku engine** (lihat tabel di bawah). Inilah kategori seperti BANK, AR, INVENTORY, FIXED ASSET, dst.

**Mengapa penting:** tipe akun menentukan bagaimana sistem memperlakukan akun tersebut — mis. akun bertipe `BANK` ikut fitur rekonsiliasi bank, `AR` ikut aging & ECL, `INVENTORY` ikut perhitungan stok & FIFO, `FIXED_ASSET` ikut penyusutan & revaluasi.

### 3.0.1 Tipe Akun Standar

| Tipe Akun | Kode | Kelompok | Perilaku Engine | Contoh Akun |
|---|---|---|---|---|
| **Kas** | CASH | Aset | Rekonsiliasi kas, arus kas | 1101 Kas, 1103 Kas Kecil |
| **Bank** | BANK | Aset | Rekonsiliasi bank, mutasi bank, arus kas | 1102 Bank (BCA, Mandiri, dst.) |
| **Piutang Usaha** | AR | Aset | Aging, ECL/penyisihan, penagihan | 1201 Piutang Usaha |
| **Piutang Lain-lain** | OTHER_RECEIVABLE | Aset | Penagihan, tanpa ECL otomatis | 1204 Piutang Lain-lain |
| **Pajak Dibayar Dimuka / PPN Masukan** | TAX_RECEIVABLE | Aset | Pelaporan PPN | 1203 PPN Masukan |
| **Uang Muka** | PREPAYMENT | Aset | Amortisasi, pelacakan DP | 1205 Uang Muka Pembelian, 1207 Beban Dibayar Dimuka |
| **Persediaan** | INVENTORY | Aset | Stok, FIFO/rata-rata, costing, NRV | 1301 Barang Dagang, 1302 Bahan Baku, 1303 WIP, 1304 Barang Jadi |
| **Aset Tetap** | FIXED_ASSET | Aset | Penyusutan, revaluasi, disposisi | 1401 Aset Tetap |
| **Kontra-Aset** | CONTRA_ASSET | Aset (kontra) | Mengurangi nilai bruto aset terkait | 1402 Akumulasi Penyusutan, 1702 Akumulasi RoU |
| **Kontra-Piutang** | CONTRA_RECEIVABLE | Aset (kontra) | Mengurangi piutang bruto, ECL/write-off | 1202 Penyisihan Piutang Tak Tertagih |
| **Aset Lain-lain** | OTHER_ASSET | Aset | Tanpa perilaku khusus | 1501 Aset Takberwujud, 1701 Right-of-Use Asset, 1206 Aset Pajak Tangguhan |
| **Hutang Usaha** | AP | Liabilitas | Aging AP, pembayaran tagihan | 2101 Hutang Usaha |
| **Hutang Akrual** | ACCRUED_LIABILITY | Liabilitas | Akrual, GRN belum ditagih | 2105 Utang Belum Ditagih, 2106 Akrual Beban |
| **Pajak Terutang** | TAX_PAYABLE | Liabilitas | Pelaporan PPN/PPh | 2202 Utang PPN, 2203 Utang PPh |
| **Uang Muka Penjualan** | CUSTOMER_DEPOSIT | Liabilitas | Pelacakan DP pelanggan | 2201 Uang Muka Penjualan |
| **Hutang Jangka Panjang** | LONG_TERM_LIABILITY | Liabilitas | Liabilitas jangka panjang non-pinjaman | 2301 Lease Liability |
| **Pinjaman** | LOAN | Liabilitas | Amortisasi pinjaman, jadwal angsuran, bunga | 2401 Utang Bank |
| **Ekuitas** | EQUITY | Ekuitas | Laba berjalan otomatis | 3101 Modal, 3201 Laba Ditahan, 3301 Laba Berjalan, 3401 Surplus Revaluasi |
| **Pendapatan** | REVENUE | Pendapatan | Pengakuan pendapatan | 4101 Pendapatan Penjualan, 4102 Pendapatan Jasa |
| **Kontra-Pendapatan** | CONTRA_REVENUE | Pendapatan (kontra) | Pengurang pendapatan | 4201 Retur Penjualan, 4202 Diskon Penjualan |
| **Harga Pokok Penjualan** | COGS | Beban | Perhitungan laba kotor | 5101 HPP, 5102 Beban Penurunan Nilai |
| **Beban** | EXPENSE | Beban | Laba Rugi | 5201 Beban Gaji, 5202 Beban Sewa, 5203, 5204, dst. |
| **Beban Lain-lain** | OTHER_EXPENSE | Beban | Laba Rugi (bawah) | 5901 Beban Lain-lain, 5903, 5904, dst. |
| **Pendapatan Lain-lain** | OTHER_INCOME | Pendapatan | Laba Rugi (bawah) | 4901 Pendapatan Lain-lain, 4903, 4904, dst. |
| **Kartu Kredit** | CREDIT_CARD | Liabilitas | Rekonsiliasi kartu kredit, pencatatan transaksi kartu | 2501 Kartu Kredit |
| **Pinjaman** | LOAN | Liabilitas | Amortisasi pinjaman, jadwal angsuran, bunga | 2401 Utang Bank (bagian pinjaman) |

### 3.0.2 Kerangka Akun Standar (Kelompok Laporan + Tipe)

| Kelompok | Kode | Contoh Akun (tipe) |
|---|---|---|
| **Aset** | 1xxx | 1101 Kas (CASH), 1102 Bank (BANK), 1103 Kas Kecil (CASH), 1201 Piutang Usaha (AR), 1202 Penyisihan Piutang (CONTRA_RECEIVABLE), 1203 PPN Masukan (TAX_RECEIVABLE), 1204 Piutang Lain-lain (OTHER_RECEIVABLE), 1205 Uang Muka Pembelian (PREPAYMENT), 1206 Aset Pajak Tangguhan (OTHER_ASSET), 1207 Beban Dibayar Dimuka (PREPAYMENT), 1301–1305 Persediaan (INVENTORY), 1401 Aset Tetap (FIXED_ASSET), 1402 Akumulasi Penyusutan (CONTRA_ASSET), 1501 Aset Takberwujud (OTHER_ASSET), 1701 RoU (OTHER_ASSET), 1702 Akumulasi RoU (CONTRA_ASSET) |
| **Liabilitas** | 2xxx | 2101 Hutang Usaha (AP), 2105 Utang Belum Ditagih (ACCRUED_LIABILITY), 2106 Akrual Beban (ACCRUED_LIABILITY), 2201 Uang Muka Penjualan (CUSTOMER_DEPOSIT), 2202 Utang PPN (TAX_PAYABLE), 2203 Utang PPh (TAX_PAYABLE), 2204 Utang BPJS (ACCRUED_LIABILITY), 2205 Utang Gaji (ACCRUED_LIABILITY), 2301 Lease Liability (LONG_TERM_LIABILITY), 2401 Utang Bank (LOAN), 2402 Kelebihan Pembayaran Pelanggan (CUSTOMER_DEPOSIT), 2501 Kartu Kredit (CREDIT_CARD) |
| **Ekuitas** | 3xxx | 3101 Modal (EQUITY), 3105 Suspense (EQUITY), 3201 Laba Ditahan (EQUITY), 3301 Laba Berjalan (EQUITY), 3401 Surplus Revaluasi (EQUITY) |
| **Pendapatan** | 4xxx | 4101 Pendapatan Penjualan (REVENUE), 4102 Pendapatan Jasa (REVENUE), 4201 Retur Penjualan (CONTRA_REVENUE), 4202 Diskon Penjualan (CONTRA_REVENUE), 4901 Pendapatan Lain-lain (OTHER_INCOME), 4902 Overhead Dibebankan (OTHER_INCOME), 4903 Keuntungan Pelepasan Aset (OTHER_INCOME), 4904 Keuntungan Selisih Kurs (OTHER_INCOME), 4905 Diskon Pembelian (OTHER_INCOME), 4906 Pemulihan Piutang (OTHER_INCOME), 4907 Kelebihan Stok (OTHER_INCOME) |
| **Beban** | 5xxx | 5101 HPP (COGS), 5102 Beban Penurunan Nilai (COGS), 5201 Beban Gaji (EXPENSE), 5202 Beban Sewa (EXPENSE), 5203 Beban Transport (EXPENSE), 5204 Beban Listrik/Air (EXPENSE), 5205 Beban Penyisihan Piutang (EXPENSE), 5206 Beban Penyusutan (EXPENSE), 5207 Beban Penurunan Nilai Aset (EXPENSE), 5208 Beban Pajak Final (EXPENSE), 5209 Beban Penyusutan RoU (EXPENSE), 5210 Beban Admin Bank (EXPENSE), 5901 Beban Lain-lain (OTHER_EXPENSE), 5903 Kerugian Pelepasan Aset (OTHER_EXPENSE), 5904 Beban Pajak Tangguhan (OTHER_EXPENSE), 5905 Kerugian Selisih Kurs (OTHER_EXPENSE), 5906 Beban Bunga (OTHER_EXPENSE), 5907 Beban Susut & Kehilangan Stok (OTHER_EXPENSE) |

**Aturan penempatan (penting):**
- **Uang Muka Penjualan** (DP diterima dari pelanggan) = **liabilitas** (2201, tipe CUSTOMER_DEPOSIT) — kewajiban menyerahkan barang/jasa.
- **Uang Muka Pembelian** (DP dibayar ke pemasok) = **aset** (1205, tipe PREPAYMENT) — hak menerima barang/jasa.
- **PPN Masukan** = aset (1203, TAX_RECEIVABLE); **PPN Keluaran** = liabilitas (2202, TAX_PAYABLE). Klasifikasi akun tidak berubah; koreksi PPN dilakukan melalui jurnal reversal/adjustment yang terdokumentasi (mis. retur, credit note, atau penyesuaian DP), bukan dengan mengedit jurnal asal.
- **Penyisihan Piutang & Penurunan Nilai Persediaan** = akun kontra-aset (pengurang nilai bruto di neraca).
- **2105 Utang Belum Ditagih** dipakai saat barang sudah diterima (GRN) tapi tagihan supplier belum masuk; direklasifikasi ke 2101 Hutang Usaha saat tagihan diterima.
- **2402 Kelebihan Pembayaran Pelanggan** menampung overpayment (pelunasan melebihi piutang) sampai dikembalikan/dikompensasi.
- **4902 Overhead Dibebankan** adalah akun pengendali (clearing) dalam job costing — diselisihkan dengan beban overhead riil di akhir periode (§11.4).

### 3.0.3 Konsistensi Kode & Tipe
| Aturan | Keterangan |
|---|---|
| Kode kelompok | 1xxx Aset, 2xxx Liabilitas, 3xxx Ekuitas, 4xxx Pendapatan, 5xxx Beban |
| Tipe wajib | Setiap akun wajib punya tipe; tipe menentukan perilaku & laporan |
| Tipe vs kelompok | Tipe harus konsisten dengan kelompok (mis. tipe BANK → kelompok Aset). Mismatch ditolak (`TYPE_GROUP_MISMATCH`) |
| Satu akun, satu tipe | Akun tidak dapat punya dua tipe sekaligus (mis. tidak bisa BANK sekaligus INVENTORY); `2401` bertipe `LOAN`, bukan dua tipe sekaligus |

### 3.1 COA Buatan Pengguna (Custom Account)

Pengguna (pemilik maupun akuntan) **dapat membuat akun sendiri** — sistem tetap menjaga struktur COA tersusun dan laporan tetap benar:

| Aturan | Keterangan |
|---|---|
| **Wajib pilih kelompok** | Setiap akun kustom wajib ditetapkan ke salah satu kelompok laporan: **Aset, Liabilitas, Ekuitas, Pendapatan, Beban** — ini menentukan posisinya di laporan (Neraca vs Laba Rugi) |
| **Wajib pilih tipe akun** | Setiap akun kustom wajib ditetapkan ke salah satu **tipe akun** (§3.0.1): BANK, AR, AP, INVENTORY, FIXED_ASSET, OTHER_ASSET, EXPENSE, REVENUE, dll. Tipe menentukan perilaku engine (rekonsiliasi bank, aging, stok, penyusutan). Tipe harus konsisten dengan kelompok (`TYPE_GROUP_MISMATCH`) |
| **Kode unik & valid** | Kode akun unik per entitas; format bebas (mis. `1101`, `1.1.01`, `A-101`) namun **harus konsisten dalam rentang kelompoknya** agar urutan laporan tersusun rapi. Kode duplikat ditolak (`ACCOUNT_EXISTS`); kode di luar rentang kelompok ditolak (`INVALID_ACCOUNT_CODE`) |
| **Hirarki grup vs detail** | Akun dapat punya parent (grup) dan child (detail). **Hanya akun detail yang boleh diposting**; akun grup = penjumlahan otomatis anaknya |
| **Status aktif/nonaktif** | Akun baru aktif. Akun yang sudah memiliki transaksi **tidak dapat dihapus** — hanya dinonaktifkan (tetap tampil di laporan historis) |
| **Pemetaan laporan otomatis** | Posisi di laporan mengikuti kelompoknya; perilaku mengikuti tipe akunnya. Pemetaan ke baris laporan dapat disesuaikan akuntan |
| **Pemetaan kategori UI** | Kategori input awam (mis. "Penjualan Online") dapat diarahkan ke akun kustom mana pun — mesin tetap membuat jurnal yang benar |
| **Periode berlaku** | Opsional: akun dapat diberi tanggal aktif/nonaktif (mis. akun lama diganti) |

**Contoh akun kustom yang umum dibuat pengguna:**
| Akun | Kelompok | Tipe | Perilaku yang didapat |
|---|---|---|---|
| Bank BRI 1234 | Aset | BANK | Ikut rekonsiliasi bank |
| Piutang PT Maju | Aset | AR | Ikut aging & ECL |
| Kendaraan Operasional | Aset | FIXED_ASSET | Ikut penyusutan |
| Toko Online (Shopee) | Aset | BANK | Ikut rekonsiliasi e-wallet |
| Sewa Gudang | Beban | EXPENSE | Masuk Laba Rugi |
| Pendapatan Bunga | Pendapatan | OTHER_INCOME | Masuk Laba Rugi (bawah) |

**Dampak ke engine:** identifikasi akun selalu via `account_id` — perubahan nama/kode akun tidak pernah mengubah jurnal lama (immutable), cukup mengubah label & pemetaan.

---

## 4. Struktur Jurnal & Nomor Dokumen

### 4.1 Struktur Data Jurnal
```
JournalEntry
├── id, nomor_jurnal (unik)
├── jenis: MANUAL | SISTEM (asal intent)
├── tanggal_transaksi
├── periode (YYYY-MM)          → acuan kunci periode
├── deskripsi, user_creator
├── status: POSTED | VOID
├── hash (anti-tamper), hash_sebelumnya
├── source_ref (dokumen asal, mis. INV-2026-000123)
├── intent_type (mis. SALES_INVOICE, CASH_RECEIPT)
└── lines: JournalLine[]
      ├── account_id, debit_cents, kredit_cents (salah satu = 0)
      ├── deskripsi_baris
      ├── source_ref (nomor dokumen baris)
      └── sub_ledger_ref (id piutang/stok/aset/job, opsional)
```

### 4.2 Format Nomor Dokumen
| Jenis | Format | Contoh |
|---|---|---|
| Jurnal | `JRN-{YYYY}-{seq}` | JRN-2026-000123 |
| Invoice | `INV-{YYYY}-{seq}` | INV-2026-000123 |
| Credit Note | `CN-{YYYY}-{seq}` | CN-2026-000007 |
| Purchase Order | `PO-{YYYY}-{seq}` | PO-2026-000045 |
| GRN / Penerimaan | `GRN-{YYYY}-{seq}` | GRN-2026-000030 |
| DO / Pengiriman | `DO-{YYYY}-{seq}` | DO-2026-000089 |
| Bukti Penerimaan Kas | `BK-{YYYY}-{seq}` | BK-2026-000055 |
| Bukti Pengeluaran Kas | `KK-{YYYY}-{seq}` | KK-2026-000041 |

- Urutan (seq) per jenis dokumen, per entitas — **tidak pernah diulang** meskipun dokumen di-void.
- Setiap jurnal sistem menyimpan `source_ref` (nomor dokumen asal) pada tiap barisnya.

### 4.3 Aturan Posting & Periode
- Periode posting ditentukan dari **tanggal transaksi** (bukan tanggal input).
- Posting ditolak bila periode terkunci (§21) — error `PERIOD_CLOSED`.
- Backdate pada periode terbuka diperbolehkan; urutan jurnal mengikuti timestamp posting (bukan tanggal transaksi).
- **Void berjenjang:** void dokumen induk (mis. DO) menolak jika sudah ada dokumen turunan (INV). Void dilakukan bottom-up: void INV dulu, baru DO.

---

## 5. Saldo Awal (Opening Balances)

Saat setup usaha (onboarding), saldo awal dimasukkan per akun. Engine membuat **satu jurnal pembuka (opening entry)**:

| Debet | Kredit | Keterangan |
|---|---|---|
| 1101 Kas, 1102 Bank, 1201 Piutang, 1301 Persediaan, 1401 Aset Tetap, dst. | — | Seluruh saldo aset awal |
| — | 2101 Hutang, 2401 Utang Bank, dst. | Seluruh saldo liabilitas awal |
| — | 3101 Modal | **Selisih (plug)** — saldo modal pemilik |

- Jika total debet ≠ total kredit saat input, selisih ditempatkan sementara di **3105 Modal Setoran/Suspense** dan harus di-reklasifikasi akuntan sebelum periode ditutup.
- Saldo awal **tidak dapat di-void** — koreksi melalui jurnal penyesuaian.
- Contoh: aset Rp 20.000.000, hutang Rp 5.000.000 → Debet 20.000.000, Kredit 5.000.000 + 15.000.000 (Modal).

**Edge cases saldo awal:**
| Kasus | Perlakuan |
|---|---|
| Saldo awal tidak balance (selisih) | Selisih → 3105 Suspense; sistem menampilkan warning sampai direklasifikasi |
| Piutang awal dengan aging sudah lewat | Tetap dicatat di saldo awal; ECL dihitung dari tanggal saldo awal |
| Persediaan awal dengan batch berbeda | Input per batch (qty + cost) agar FIFO berjalan benar |
| Aset tetap sudah terpakai | Input cost, akumulasi penyusutan, dan tanggal mulai pakai — penyusutan lanjut dari nilai buku |

---

## 6. Pencatatan Transaksi Harian (Inti)

### 6.1 Uang Masuk (CASH_IN)
Intent dari UI awam: *"Uang masuk — Penjualan Tunai — Rp 500.000"*

| Debet | Kredit | Jumlah | Keterangan |
|---|---|---|---|
| 1101 Kas | — | 500.000 | Kas bertambah |
| — | 4101 Pendapatan Penjualan | 500.000 | Pengakuan pendapatan tunai |

*(Jika produk terjual dari stok → tambahan jurnal HPP, lihat §9)*

### 6.2 Uang Keluar (CASH_OUT)
Intent: *"Uang keluar — Sewa Toko — Rp 2.000.000"*

| Debet | Kredit | Jumlah |
|---|---|---|
| 5202 Beban Sewa | — | 2.000.000 |
| — | 1101 Kas | 2.000.000 |

### 6.3 Pindah Kas ke Bank (TRANSFER)
| Debet | Kredit | Jumlah |
|---|---|---|
| 1102 Bank | — | X |
| — | 1101 Kas | X |

### 6.4 Kategori → Pemetaan Akun
Setiap kategori UI sudah terpetakan ke akun default:

| Kategori UI | Akun Debet | Akun Kredit (saat Uang Keluar/Masuk) |
|---|---|---|
| Penjualan | Kas/Bank | 4101 Pendapatan Penjualan |
| Pembelian barang dagang | 1301 Persediaan | Kas/Bank / 2101 Hutang |
| Gaji karyawan | 5201 Beban Gaji | Kas/Bank |
| Sewa | 5202 Beban Sewa | Kas/Bank |
| Transport | 5203 Beban Transport | Kas/Bank |
| Listrik/Air | 5204 Beban Listrik/Air | Kas/Bank |
| Modal pemilik | 1101 Kas/Bank | 3101 Modal |

Akun dapat dikustomisasi akuntan di Mode Akuntan.

### 6.5 Edge Cases Transaksi Harian
| Kasus | Perlakuan |
|---|---|
| Salah pilih kategori (mis. sewa diinput sebagai transport) | Koreksi: jurnal balik + jurnal benar (bukan edit in-place) |
| Uang masuk yang sebenarnya pelunasan piutang | Sistem menawarkan alur "Terima Pembayaran" → pilih invoice → Dr Kas / Cr Piutang |
| Transaksi duplikat (input dua kali) | Deteksi kemiripan (nominal + tanggal + kategori); warning sebelum simpan |
| Biaya administrasi bank | 5210 Beban Administrasi Bank (Dr Beban / Cr Bank) |

---

## 7. Alur Penjualan: SQ → SO → DP → DO → INV → Pelunasan

| Tahap | Dokumen | Efek Jurnal | Status |
|---|---|---|---|
| **1. SQ** | Sales Quotation | **Tidak ada jurnal** — hanya penawaran/komitmen | Draft → Dikirim |
| **2. SO** | Sales Order | **Tidak ada jurnal** — komitmen pesanan; kunci harga, kuantitas, cek stok tersedia | Dikonfirmasi |
| **3. DP** | Down Payment (uang muka) | Debet Kas/Bank, Kredit **2201 Uang Muka Penjualan** | DP diterima |
| **4. DO** | Delivery Order (pengiriman) | **Jurnal HPP**: Debet 5101 HPP, Kredit 1301 Persediaan (nilai pokok barang yang dikirim) | Dikirim |
| **5. INV** | Invoice | ① Debet **1201 Piutang Usaha** (nilai invoice), Kredit 4101 Pendapatan; ② **Realisasi DP**: Debet 2201 Uang Muka Penjualan, Kredit 1201 Piutang; ③ Debet 5101 HPP, Kredit 1301 Persediaan (jika DO belum membukukan HPP) | Diterbitkan |
| **6. Pelunasan** | Pembayaran | Debet Kas/Bank, Kredit **1201 Piutang Usaha** (sisa piutang) | Lunas |

**Contoh angka** (1 unit, harga Rp 5.000.000, HPP Rp 3.000.000, DP 50%):
1. DP Rp 2.500.000 → Debet Kas 2.500.000 / Kredit 2201 Uang Muka Penjualan 2.500.000
2. DO → Debet 5101 HPP 3.000.000 / Kredit 1301 Persediaan 3.000.000
3. INV Rp 5.000.000 → Debet 1201 Piutang 5.000.000 / Kredit 4101 Pendapatan 5.000.000; lalu Debet 2201 Uang Muka 2.500.000 / Kredit 1201 Piutang 2.500.000 → **sisa piutang Rp 2.500.000**
4. Pelunasan → Debet Kas 2.500.000 / Kredit 1201 Piutang 2.500.000

**Kepatuhan PSAK 72:** pendapatan diakui saat pengendalian beralih ke pelanggan. Default: **saat INV**. Jika syarat penjualan menyatakan pengendalian beralih saat pengiriman, konfigurasi dapat diubah **saat DO**. Uang muka dicatat sebagai liabilitas kontrak (2201) sampai direalisasi.

**Validasi engine:** DP tidak boleh melebihi nilai SO (kecuali dikonfigurasi); INV tidak boleh melebihi sisa DO yang belum di-invoice.

### 7.1 Edge Cases Alur Penjualan
| Kasus | Perlakuan / Jurnal |
|---|---|
| **DO bertahap** (kirim 2× untuk 1 SO) | Setiap DO membukukan HPP sesuai qty yang dikirim; INV dapat diterbitkan per DO atau digabung (konfigurabel) |
| **Beberapa DP** pada satu SO | Setiap DP → Dr Kas / Cr 2201; saat INV, seluruh saldo 2201 terkait direalisasi |
| **Pembatalan order setelah DP** | Refund DP: **Dr 2201 Uang Muka Penjualan / Cr Kas**; bila tidak di-refund, DP dipindah ke 2402 Kelebihan Pembayaran atas persetujuan pelanggan |
| **Pelunasan sebagian** | Alokasi pembayaran ke invoice tertua dulu (FIFO aging, konfigurabel); piutang tersisa tetap tampil |
| **Pelunasan lebih (overpayment)** | Dr Kas (nilai penuh) / Cr Piutang (sisa) / **Cr 2402 Kelebihan Pembayaran** (selisih) |
| **Overpayment dikembalikan** | Dr 2402 / Cr Kas |
| **Overpayment dikompensasi ke invoice berikutnya** | Dr 2402 / Cr 1201 Piutang (invoice baru) |
| **Retur setelah pelunasan penuh** | Retur diakui + refund tunai: Dr 4201 Retur, Dr 2202 Utang PPN (bagian PPN), Cr Kas; balik HPP & tambah stok |
| **Invoice kena PPN + DP** | DP tanpa PPN (belum ada faktur); saat INV, PPN dihitung atas DPP dikurangi DP — ikuti ketentuan faktur pajak |
| **Pelanggan tidak membayar (macet)** | Lihat §15 ECL (penyisihan), lalu write-off |

---

## 8. Retur & Diskon Penjualan

### 8.1 Retur Penjualan (Sales Return)
Saat pelanggan mengembalikan barang:

| Debet | Kredit | Keterangan |
|---|---|---|
| 4201 Retur Penjualan | — | Mengurangi pendapatan (kontra-pendapatan) |
| — | 1201 Piutang / 1101 Kas | Refund atau pengurang piutang |
| 1301 Persediaan | — | Barang kembali ke stok (jika layak jual) |
| — | 5101 HPP | Membalik HPP yang sudah dibukukan |

*(Jika PPN — faktur pajak pengganti: Debet 2202 Utang PPN atas nilai retur)*

### 8.2 Diskon Penjualan (Early Payment / Trade Discount)
Diskon tunai saat pelunasan cepat (mis. 2/10 n/30):

| Debet | Kredit | Keterangan |
|---|---|---|
| 1101 Kas | — | Nilai yang diterima (neto) |
| 4202 Diskon Penjualan | — | Nilai diskon yang diberikan |
| — | 1201 Piutang | Nilai piutang penuh |

### 8.3 Edge Cases Retur & Diskon
| Kasus | Perlakuan |
|---|---|
| Retur sebagian (1 dari 3 unit) | Retur dihitung proporsional; credit note (CN) otomatis sebesar nilai barang diretur |
| Barang retur rusak (tidak layak jual) | Tidak kembali ke stok → **Dr 5901 Beban Lain-lain / Cr 1301** (atau dibuang; kebijakan konfigurabel) |
| Diskon lebih besar dari sisa piutang | Ditolak engine (`DISCOUNT_EXCEEDS_RECEIVABLE`); diskon maksimal = sisa piutang |
| Retur setelah periode ditutup | Jurnal retur masuk periode berjalan (bukan mengubah periode lama); lihat §21 |
| Credit note lebih besar dari invoice | Ditolak (`CN_EXCEEDS_INVOICE`) kecuali diotorisasi akuntan |

---

## 9. Persediaan (PSAK 14)

### 9.1 Metode Penilaian
| Metode | Implementasi |
|---|---|
| **FIFO** | Barang keluar dihargai dari batch/lapisan pertama masuk |
| **Rata-rata bergerak** | Nilai stok = total nilai / total qty, diperbarui tiap transaksi masuk |
| **Identifikasi khusus** | Tiap unit punya cost tersendiri (untuk item bernilai tinggi) |

### 9.2 Jurnal Persediaan
| Peristiwa | Debet | Kredit |
|---|---|---|
| Pembelian (GRN) tanpa PPN | 1301 Persediaan | 2105 Utang Belum Ditagih / Kas |
| Pembelian dengan PPN (PKP) | 1301 Persediaan + 1203 PPN Masukan | 2101 Hutang Usaha / Kas (nilai total) |
| Penjualan (DO/INV) | 5101 HPP | 1301 Persediaan |
| Stock opname (lebih) | 1301 Persediaan | 4907 Kelebihan Stok |
| Stock opname (kurang) | 5907 Beban Susut & Kehilangan Stok | 1301 Persediaan |
| Transfer antar lokasi | 1301 Persediaan (Lokasi B) | 1301 Persediaan (Lokasi A) |

### 9.3 Penurunan Nilai (NRV)
Jika nilai realisasi neto (NRV) < cost, selisih dibukukan sebagai **penurunan nilai persediaan**:

| Debet | Kredit |
|---|---|
| 5102 Beban Penurunan Nilai Persediaan | 1305 Penyisihan Penurunan Nilai Persediaan |

*(Pemulihan saat NRV membaik: jurnal balik; penyisihan disajikan sebagai pengurang persediaan di neraca)*

### 9.4 Edge Cases Persediaan
| Kasus | Perlakuan |
|---|---|
| **Stok negatif** (jual lebih dari stok) | Default: ditolak (`STOCK_NEGATIVE`). Opsi konfigurabel: izinkan dengan catatan — HPP memakai cost rata-rata terakhir; selisih menjadi selisih opname |
| **Retur penjualan → batch mana?** | FIFO: masuk sebagai **batch baru** (cost = cost asal barang yang diretur) agar tidak merusak lapisan FIFO |
| Barang rusak saat penerimaan (GRN) | Dicatat di GRN sebagai qty baik + qty rusak; qty rusak → Dr 5907 / Cr 2105 (atau ditolak terima) |
| Transfer antar gudang dengan biaya kirim | Biaya transfer dibebankan ke beban transport (bukan ke nilai stok, kebijakan konfigurabel) |
| Barang konsinyasi (titip jual) | Dicatat off-balance (memorial) — bukan milik perusahaan; hanya komisi yang jadi beban saat terjual |
| Persediaan usang/expired | Penurunan nilai NRV (Dr 5102 / Cr 1305); saat dibuang: Dr 1305 / Cr 1301 |
| Pembulatan harga rata-rata | Sisa pembulatan diserap di baris terakhir konsumsi (cost tidak bisa berubah total nilai stok) |

---

## 10. Alur Pembelian: PR → PO → GRN → Tagihan → Bayar

| Tahap | Dokumen | Efek Jurnal |
|---|---|---|
| **1. PR** | Permintaan Pembelian | Tidak ada jurnal — hanya permintaan internal |
| **2. PO** | Purchase Order | Tidak ada jurnal — komitmen pembelian |
| **3. DP Pembelian** (opsional) | Uang muka | Debet **1205 Uang Muka Pembelian** (aset), Kredit Kas/Bank |
| **4. GRN** | Penerimaan Barang | Debet 1301 Persediaan (nilai barang), Kredit **2105 Utang Belum Ditagih** |
| **5. Tagihan** | Supplier Invoice | ① Debet 1203 PPN Masukan, Kredit 2105 Utang Belum Ditagih (jika PPN); ② Reklasifikasi: Debet 2105 Utang Belum Ditagih (total + PPN), Kredit 2101 Hutang Usaha; ③ Realisasi DP: Debet 2101 Hutang Usaha, Kredit 1205 Uang Muka Pembelian |
| **6. Bayar** | Pembayaran | Debet 2101 Hutang Usaha, Kredit Kas/Bank |

**Contoh angka** (barang Rp 5.000.000 + PPN 11% Rp 550.000, DP Rp 1.000.000):
1. DP → Debet 1205 Uang Muka Pembelian 1.000.000 / Kredit Kas 1.000.000
2. GRN → Debet 1301 Persediaan 5.000.000 / Kredit 2105 Utang Belum Ditagih 5.000.000
3. Tagihan → Debet 1203 PPN Masukan 550.000 / Kredit 2105 Utang Belum Ditagih 550.000; Reklasifikasi: Debet 2105 Utang Belum Ditagih 5.550.000 / Kredit 2101 Hutang Usaha 5.550.000; Realisasi DP: Debet 2101 Hutang Usaha 1.000.000 / Kredit 1205 Uang Muka Pembelian 1.000.000 → **sisa hutang Rp 4.550.000**
4. Bayar → Debet 2101 Hutang Usaha 4.550.000 / Kredit Kas 4.550.000

### 10.1 Retur Pembelian (Purchase Return)
| Debet | Kredit | Keterangan |
|---|---|---|
| 2101 Hutang Usaha / 1101 Kas | — | Nilai retur |
| — | 1301 Persediaan | Barang dikembalikan |
| — | 1203 PPN Masukan | Membalik PPN masukan atas barang diretur |

### 10.2 Diskon Pembelian
| Debet | Kredit | Keterangan |
|---|---|---|
| 2101 Hutang Usaha | — | Nilai hutang penuh |
| — | 1101 Kas | Nilai yang dibayar (neto) |
| — | 4905 Diskon Pembelian | Selisih diskon |

### 10.3 Edge Cases Pembelian
| Kasus | Perlakuan |
|---|---|
| **Selisih harga tagihan vs PO/GRN** | Selisih dibukukan saat tagihan: tambah/kurang 1301 Persediaan (atau 5901 Beban Selisih Harga — kebijakan konfigurabel) |
| **Penerimaan parsial (GRN sebagian)** | Beberapa GRN per PO; setiap GRN menambah stok & 2105 |
| **Barang lebih dari PO (over-delivery)** | Ditolak default (`OVER_DELIVERY`); perlu approval akuntan untuk menerima & membayar kelebihan |
| **Tagihan datang sebelum barang** | Hutang dibukukan saat tagihan (Dr Persediaan/Dr PPN Masukan / Cr 2101); saat GRN tiba tidak ada jurnal baru |
| **DP pembelian tidak terealisasi** (pesanan batal) | Dr 2101 / Cr 1205 (kompensasi) atau Dr 5901 / Cr 1205 (hangus) |
| **Pembayaran kurang dari tagihan (potongan lain-lain)** | Sisa hutang ditutup: Dr 2101 / Cr 5901 Beban Lain-lain (atau 4901 bila pengurangan dari supplier) |
| **Biaya angkut pembelian** | Ditambahkan ke cost persediaan (PSAK 14 — biaya perolehan termasuk angkut): Dr 1301 / Cr Kas |

---

## 11. Produksi Sederhana — Job Order Costing

### 11.1 Struktur
- **Job** = unit produksi per pesanan (nomor job, produk, tanggal mulai-selesai).
- **BOM (Bill of Materials)** = daftar bahan + kuantitas + komponen biaya (material, tenaga kerja, overhead) untuk 1 produk.
- Aliran akun: 1302 Bahan Baku → 1303 WIP → 1304 Barang Jadi.

### 11.2 Jurnal Produksi
| Peristiwa | Debet | Kredit |
|---|---|---|
| **Mulai job** (alokasi material) | 1303 WIP | 1302 Bahan Baku |
| **Tenaga kerja** (jam kerja job) | 1303 WIP | 5201 Beban Gaji (akun upah langsung) |
| **Overhead dialokasikan** | 1303 WIP | 4902 Overhead Dibebankan |
| **Barang jadi (job selesai)** | 1304 Barang Jadi | 1303 WIP |
| **Penjualan barang jadi** | 5101 HPP | 1304 Barang Jadi |

### 11.3 Contoh Angka (Job #001 — 100 unit)
| Komponen | Biaya |
|---|---|
| Material langsung (BOM) | Rp 4.000.000 |
| Tenaga kerja langsung (200 jam × Rp 10.000) | Rp 2.000.000 |
| Overhead (rate Rp 5.000/jam × 200 jam) | Rp 1.000.000 |
| **Total biaya job** | **Rp 7.000.000** |
| **Biaya per unit** | **Rp 70.000** |

### 11.4 Selisih Overhead (Variance)
Di akhir periode, akun 4902 Overhead Dibebankan diselisihkan dengan beban overhead riil (listrik, sewa pabrik, dll):
- Dibebankan > riil → selisih dipindahkan ke pendapatan lain-lain (kebijakan konfigurabel).
- Dibebankan < riil (under-applied) → selisih dibebankan ke HPP/beban overhead.

**Kepatuhan:** harga pokok produksi sesuai konsep biaya perolehan (cost) PSAK 14 — seluruh biaya langsung + alokasi overhead yang wajar.

### 11.5 Edge Cases Produksi
| Kasus | Perlakuan |
|---|---|
| **Job dibatalkan** setelah material dikeluarkan | Material tidak terpakai dikembalikan: **Dr 1302 Bahan Baku / Cr 1303 WIP**; WIP tersisa di-reklasifikasi ke 5901 Beban (kebijakan konfigurabel) |
| **Sisa material (scrap/remnant)** | Dr 1302 Bahan Baku (nilai sisa) / Cr 1303 WIP — mengurangi biaya job |
| **Job selesai sebagian** | Produksi parsial: sebagian WIP → Barang Jadi sesuai qty selesai; WIP tetap untuk yang belum |
| **Produk cacat** | Unit cacat: Dr 5901 Beban / Cr 1304 Barang Jadi (cost unit cacat) |
| **Tenaga kerja dibebankan setelah job selesai** | Jurnal koreksi: Dr 1303 WIP (buka lagi) / Cr 5201; lalu transfer ke Barang Jadi |
| **Dua job berbagi material** | Alokasi material per job sesuai BOM; selisih pemakaian dihitung saat selesai |

---

## 12. Aset Tetap (PSAK 16)

### 12.1 Registrasi & Penyusutan
| Peristiwa | Debet | Kredit |
|---|---|---|
| **Perolehan** (tunai) | 1401 Aset Tetap | Kas/Bank |
| **Perolehan** (kredit) | 1401 Aset Tetap | 2101 Hutang Usaha |
| **Penyusutan bulanan** | 5206 Beban Penyusutan | 1402 Akumulasi Penyusutan |

Metode: **garis lurus**, **saldo menurun**, **unit produksi** — konfigurabel per aset (masa manfaat, nilai residu). Setelah revaluasi, penyusutan dihitung dari nilai tercatat baru.

### 12.2 Revaluasi (model revaluasi PSAK 16)
| Skenario | Debet | Kredit |
|---|---|---|
| **Naik** (surplus) | 1401 Aset Tetap | 3401 Surplus Revaluasi (OCI/Ekuitas) |
| **Turun** (tidak ada surplus revaluasi sebelumnya) | 5207 Beban Penurunan Nilai Aset | 1401 Aset Tetap |
| **Turun** (masih ada surplus dari revaluasi sebelumnya) | 3401 Surplus Revaluasi (sebesar surplus tersisa; sisanya ke beban) | 1401 Aset Tetap |

**Contoh:** aset nilai buku Rp 80.000.000 direvaluasi naik ke Rp 100.000.000 → Debet 1401 20.000.000 / Kredit 3401 20.000.000.

### 12.3 Disposisi / Pelepasan (Penjualan, Transfer, Penghapusan)
Satu jurnal lengkap saat penjualan aset (nilai buku = nilai perolehan − akumulasi):

| Debet | Kredit | Keterangan |
|---|---|---|
| 1101 Kas/Bank | — | Harga jual |
| 1402 Akumulasi Penyusutan | — | Akumulasi yang dihapus |
| — | 1401 Aset Tetap | Nilai perolehan bruto |
| — | 4903 Keuntungan Pelepasan Aset | Selisih bila nilai buku < harga jual |
| 5903 Kerugian Pelepasan Aset | — | Selisih bila nilai buku > harga jual |

**Contoh:** cost Rp 50.000.000, akumulasi Rp 30.000.000 (nilai buku 20.000.000), dijual Rp 25.000.000 → Debet Kas 25.000.000, Debet Akumulasi 30.000.000, Kredit Aset 50.000.000, **Kredit Keuntungan 5.000.000**.

**Kepatuhan:** surplus revaluasi tetap di ekuitas (OCI) dan **direalisasi ke laba ditahan** saat aset dilepas (PSAK 16); keuntungan/kerugian disposisi masuk laba rugi.

### 12.4 Edge Cases Aset Tetap
| Kasus | Perlakuan |
|---|---|
| **Aset dibeli di tengah bulan** | Penyusutan pro-rata bulan berjalan (hari/30, konfigurabel) |
| **Revaluasi proporsional** (cost + akumulasi dinaikkan proporsional) | Kedua akun disesuaikan sehingga nilai buku = nilai wajar; selisih ke OCI |
| **Aset dijual kena PPN** | Dr Piutang/Kas (total) / Cr 4903 Aset (harga jual) / Cr 2202 Utang PPN; akumulasi & cost dibalik |
| **Tukar tambah (trade-in)** | Aset baru: Dr 1401 (nilai wajar aset baru) / Dr 1402 Akumulasi lama / Cr 1401 lama / Cr 5903–4903 selisih / Cr Kas (selisih bayar) |
| **Aset hilang/rusak total** | Hapus: Dr 1402 / Dr 5903 (nilai buku) / Cr 1401; klaim asuransi → Dr Piutang Klaim / Cr 4901 |
| **Aset dengan komponen** (PSAK 16 komponen) | Setiap komponen dicatat & disusutkan terpisah; penggantian komponen → hapus komponen lama, catat baru |
| **Kapitalisasi biaya perbaikan besar** | Perbaikan menambah masa manfaat → Dr 1401 / Cr Kas (bukan beban) |
| **Penyusutan saat revaluasi naik** | Depresiasi dihitung dari nilai tercatat baru (bisa lebih tinggi); biaya ekstra dari OCI langsung ke laba ditahan (praktik umum) |

---

## 13. Pajak

### 13.1 PPN (sesuai regulasi efektif)
**Penjualan kena PPN:**
| Debet | Kredit | Jumlah |
|---|---|---|
| 1201 Piutang / Kas | — | Nilai total (DPP + PPN) |
| — | 4101 Pendapatan | DPP (nilai jual) |
| — | 2202 Utang PPN Keluaran | PPN (tarif × DPP) |

Contoh: jual Rp 10.000.000 + PPN 11% → Debet Piutang 11.100.000 / Kredit Pendapatan 10.000.000 / Kredit Utang PPN 1.100.000.

**Pembelian kena PPN (PKP):**
| Debet | Kredit | Jumlah |
|---|---|---|
| 1301 Persediaan / Beban | — | DPP |
| 1203 PPN Masukan | — | PPN (tarif × DPP) |
| — | 2101 Hutang Usaha / Kas | Nilai total |

*(PPN masukan adalah aset yang dapat dikreditkan — **bukan** utang. Kesalahan umum yang dihindari engine.)*

### 13.2 PPh Final UMKM (sesuai regulasi efektif)
- Basis, tarif, ambang omzet, dan eligibility mengikuti skema wajib pajak serta regulasi yang efektif pada periode transaksi; `0,5%` dan `Rp 4,8 M` hanya digunakan bila aturan yang berlaku memang memenuhi syarat tersebut.
- **Akrual bulanan:** Debet 5208 Beban Pajak Final, Kredit 2203 Utang PPh Final sebesar tarif efektif × basis omzet.
- Saat setor: Debet 2203 Utang PPh Final, Kredit Kas/Bank.
- Rekapitulasi omzet bulanan → dasar pelaporan pajak sesuai skema yang dipilih.

### 13.3 PPh 21 (TER — PMK 168/2023)
- Penghasilan bruto → dikurangi biaya jabatan → tarif efektif rata-rata (TER) bulanan.
- Jurnal penggajian: Debet 5201 Beban Gaji (+ tunjangan), Kredit 2203 Utang PPh 21, Kredit 2204 Utang BPJS, Kredit Kas/Bank (net yang diterima).
- Saat setor: Debet 2203 Utang PPh 21 / 2204 Utang BPJS, Kredit Kas/Bank.

### 13.4 PPh 23/26
- Pemotongan atas jasa/royalti/sewa: Debet beban terkait, Kredit 2203 Utang PPh 23, Kredit Kas/Bank (nilai neto).
- Saat setor: Debet 2203 Utang PPh 23, Kredit Kas/Bank.

### 13.5 PPh 22 & PPh 4(2)
| Jenis | Debet | Kredit |
|---|---|---|
| **PPh 22** (impor/bendaharawan) | Beban terkait / Persediaan | 2203 Utang PPh 22 + Kas (neto) |
| **PPh 4(2)** (sewa tanah/bangunan final) | 5202 Beban Sewa | 2203 Utang PPh 4(2) + Kas (neto) |

### 13.6 Edge Cases Pajak
| Kasus | Perlakuan |
|---|---|
| **Pembulatan PPN** (mis. PPN 12% × Rp 1.234 = Rp 148,08) | Dibulatkan ke Rp 148 (atau 149 sesuai aturan); selisih 0,08 dicatat di baris akun pajak |
| **PPN atas DP/uang muka** (faktur pajak atas uang muka) | Saat DP: Dr Kas / Cr 2201 Uang Muka + **Cr 2202 Utang PPN** (bagian PPN); saat INV, PPN disesuaikan |
| **Retur kena PPN** | Faktur pajak pengganti: Dr 2202 Utang PPN (nilai PPN retur) |
| **Omset melewati batas 4,8 M di tengah tahun** | PPh final berhenti per bulan melebihi; sistem memberi peringatan & menghitung PPh normal dari bulan berikutnya |
| **Tarif PPN berubah (11% → 12%)** | Tarif per tanggal faktur; engine menyimpan histori tarif |
| **PPh 21 untuk pegawai yang baru mulai di tengah bulan** | TER dihitung proporsional sesuai PMK 168 |

---

## 14. Pajak Tangguhan (PSAK 46)

- Dihitung dari **perbedaan temporer** antara basis akuntansi dan basis fiskal.
- Contoh: penyusutan aset — basis fiskal berbeda dari basis akuntansi.
- Jurnal: Debet 1206 Aset Pajak Tangguhan / Kredit 5904 Beban Pajak Tangguhan (atau sebaliknya untuk liabilitas pajak tangguhan).
- Dievaluasi setiap akhir periode; saldo disesuaikan terhadap tarif pajak yang berlaku.

**Edge cases:**
| Kasus | Perlakuan |
|---|---|
| Perubahan tarif pajak (mis. PPh badan naik) | Saldo pajak tangguhan dihitung ulang dengan tarif baru; selisih ke beban/pendapatan pajak tangguhan |
| Aset pajak tangguhan tidak dapat dimanfaatkan | Hanya diakui sebesar kemungkinan realisasi; sisanya tidak diakui (dicatat off-balance) |
| Perbedaan permanen (bukan temporer) | Tidak menimbulkan pajak tangguhan (mis. beban denda yang tidak boleh dikurangkan) |

---

## 15. Penyisihan Piutang Tak Tertagih (ECL — PSAK 71)

- **Perhitungan:** model ekspektasi kerugian kredit (ECL) sederhana — persentase berdasarkan aging piutang (0-30, 31-60, 61-90, >90 hari), dapat dikonfigurasi.
- **Jurnal penyisihan:**
| Debet | Kredit |
|---|---|
| 5205 Beban Penyisihan Piutang | 1202 Penyisihan Piutang Tak Tertagih |

- **Penghapusan piutang (write-off):**
| Debet | Kredit |
|---|---|
| 1202 Penyisihan Piutang Tak Tertagih | 1201 Piutang Usaha |

- **Pemulihan** (pelunasan piutang yang sudah dihapus): Debet Kas/Bank, Kredit 4906 Pemulihan Piutang.

**Edge cases:**
| Kasus | Perlakuan |
|---|---|
| Write-off parsial | Hapus sebagian piutang; sisa tetap ditagih |
| Piutang dibayar sebagian lalu macet | ECL dihitung atas sisa piutang |
| Pemulihan piutang yang sudah dihapus | Dr Kas / Cr 4906 (bukan kembali ke 1201) |
| Koreksi ECL naik/turun antar periode | Selisih ke 5205 (tambah/kurang) — penyesuaian saldo penyisihan |

**Tabel terkait (DATA_MODEL.md):** `ecl_policies` (persentase per bucket), `write_offs` (penghapusan + pemulihan → 4906, wajib approval).

---

## 16. Akrual & Beban Dibayar Dimuka

### 16.1 Beban Dibayar Dimuka (Prepaid)
Saat membayar di muka (mis. asuransi 12 bulan):

| Peristiwa | Debet | Kredit |
|---|---|---|
| Pembayaran di muka | 1207 Beban Dibayar Dimuka | Kas/Bank |
| Amortisasi bulanan (1/12) | Beban terkait (5201–5204) | 1207 Beban Dibayar Dimuka |

### 16.2 Akrual Beban (Accrued Expenses)
Beban yang sudah terjadi tapi belum dibayar/ditagih (listrik, gaji, bunga):

| Peristiwa | Debet | Kredit |
|---|---|---|
| Akrual akhir periode | Beban terkait | 2106 Akrual Beban |
| Saat dibayar/ditagih | 2106 Akrual Beban | Kas/Bank / 2101 Hutang |

*(Jurnal akrual dibuat otomatis di akhir periode dan dibalik otomatis di awal periode berikutnya — kebijakan konfigurabel.)*

**Edge cases:**
| Kasus | Perlakuan |
|---|---|
| Prepaid dibatalkan (refund asuransi) | Dr Kas / Cr 1207 (sisa belum diamortisasi) |
| Amortisasi tidak genap (mulai tanggal 15) | Pro-rata harian |
| Tagihan riil lebih besar dari akrual | Selisih dibebankan ke periode tagihan (akrual telah dibalik) |
| Akrual gaji pegawai di akhir tahun | Dr 5201 / Cr 2205 Utang Gaji (bukan 2106) |

---

## 17. Pendapatan Jasa & Kontrak Multi-Element (PSAK 72)

### 17.1 Jasa dengan Pembayaran di Muka
| Peristiwa | Debet | Kredit |
|---|---|---|
| Terima pembayaran di muka | Kas/Bank | 2201 Uang Muka Penjualan (liabilitas kontrak) |
| Pengakuan bertahap (bulanan/saat jasa diserahkan) | 2201 Uang Muka Penjualan | 4102 Pendapatan Jasa |

### 17.2 Kontrak Multi-Element (barang + jasa, mis. mesin + instalasi)
- **Alokasi harga transaksi** ke masing-masing kewajiban kinerja berdasarkan harga jual berdiri sendiri (standalone selling price).
- Pendapatan diakui saat tiap kewajiban kinerja diselesaikan:
  - Barang → saat pengendalian beralih (DO/INV).
  - Jasa instalasi → saat selesai dikerjakan (persentase penyelesaian untuk kontrak jangka panjang).

| Peristiwa | Debet | Kredit |
|---|---|---|
| Penyerahan barang | 1201 Piutang (alokasi barang) | 4101 Pendapatan Penjualan |
| Penyelesaian jasa | 1201 Piutang (alokasi jasa) | 4102 Pendapatan Jasa |

**Edge cases:**
| Kasus | Perlakuan |
|---|---|
| Kontrak langganan bulanan (SaaS, retainer) | Pembayaran diterima di muka → 2201; diakui proporsional per bulan (Dr 2201 / Cr 4102) |
| Kontrak dibatalkan sebelum jasa selesai | Pendapatan diakui hanya yang sudah dikerjakan; sisa 2201 di-refund |
| Bonus/penalti tergantung kinerja | Diestimasi (probable & reliably measurable) dan disesuaikan tiap periode (variable consideration) |
| Garansi produk | Bagian harga dialokasikan ke kewajiban garansi; diakui saat garansi diselesaikan/berakhir |

---

## 18. Kas Kecil (Sistem Imprest)

| Peristiwa | Debet | Kredit |
|---|---|---|
| Pembentukan dana kas kecil | 1103 Kas Kecil | 1101 Kas / 1102 Bank |
| Pengisian kembali (reimburse) | Beban-beban terkait (sesuai bukti) | 1101 Kas / 1102 Bank |

- Saldo 1103 Kas Kecil tetap (imprest) selama tidak ada perubahan dana.
- Pengeluaran kecil tidak dijurnal per item — cukup buku kas kecil; jurnal hanya saat pembentukan & pengisian kembali.

**Edge cases:**
| Kasus | Perlakuan |
|---|---|
| Selisih kas kecil (kurang/lebih saat opname) | Dr 5901 Beban (kurang) / Cr 1103; atau Dr 1103 / Cr 4901 (lebih) — saat pengisian kembali |
| Dana kas kecil dinaikkan/diturunkan | Dr 1103 / Cr Kas (naik); sebaliknya saat turun |

---

## 19. Multi-Mata Uang

| Peristiwa | Debet | Kredit |
|---|---|---|
| **Transaksi valas** (nilai fungsional IDR) | Aset/Beban (kurs saat transaksi) | Kas/Liabilitas |
| **Selisih kurs saat pembayaran/pelaporan** (untung) | Kas/Piutang | 4904 Keuntungan Selisih Kurs |
| **(rugi)** | 5905 Kerugian Selisih Kurs | Kas/Piutang |

Kurs harian: sumber kurs tengah BI (atau bank acuan) — konfigurabel. Revaluasi saldo moneter valas dilakukan pada akhir periode.

**Edge cases:**
| Kasus | Perlakuan |
|---|---|
| **Piutang valas: invoiced USD 1.000 @15.000, dibayar @15.200** | Dr Kas 15.200.000 / Cr Piutang 15.000.000 / **Cr 4904 Selisih Kurs 200.000** |
| **Hutang valas: PO USD 500 @15.000, bayar @14.800** | Dr Hutang 7.500.000 / Cr Kas 7.400.000 / **Cr 4904 Selisih Kurs 100.000** |
| Revaluasi akhir periode (AR/AP/kas valas) | Selisih kurs dibukukan ke laba rugi (moneter) |
| Kurs tidak tersedia di tanggal transaksi | `CURRENCY_MISMATCH` — pakai kurs terdekat (konfigurabel) |

---

## 20. Sewa (PSAK 73 / IFRS 16)

| Peristiwa | Debet | Kredit |
|---|---|---|
| **Awal kontrak sewa** | 1701 Right-of-Use Asset | 2301 Lease Liability |
| **Pembayaran sewa bulanan** | 2301 Lease Liability + 5906 Beban Bunga | Kas/Bank |
| **Penyusutan RoU asset** | 5209 Beban Penyusutan RoU | 1702 Akumulasi Penyusutan RoU |

Pengecualian: sewa jangka pendek (≤ 12 bulan) & aset bernilai rendah → dicatat langsung sebagai beban sewa (diizinkan IFRS 16).

**Edge cases:**
| Kasus | Perlakuan |
|---|---|
| **Modifikasi sewa** (perpanjangan/penyempitan masa) | Hitung ulang lease liability (PV pembayaran baru); selisih ke 1701 |
| **Terminasi dini** | Hapus 1701 & 2301 (sisa); selisih → 4903/5903 |
| Sewa dibayar di muka penuh | Dr 1701 / Cr Kas (tidak ada lease liability) |
| Insentif sewa dari lessor (mis. gratis 3 bulan) | Kurangi nilai 1701/2301 (tidak diakui sebagai pendapatan) |

---

## 21. Penutupan Periode

### 21.1 Konfigurasi Periode Akuntansi
- **Periode default:** bulanan (calendar month), dapat diatur entitas: bulanan, kuartalan, atau tahunan.
- **Tahun buku (fiscal year):** 1 Jan – 31 Des (default Indonesia) atau disesuaikan (mis. April – Maret).
- **Periode dapat dibuat terlebih dahulu** (bulan depan, kuartal berikutnya) — periode yang belum dibuka otomatis dibuat saat transaksi pertama masuk.
- Hanya **satu periode aktif (terbuka)** per entitas pada satu waktu; periode lain terkunci.

### 21.2 Pengakuan Laba Berjalan Otomatis (Auto-Recognition of Retained Earnings)
Sistem **mengakui laba berjalan secara otomatis setiap periode** sesuai settingan periode akuntansi:

| Periode | Perilaku Otomatis |
|---|---|
| **Bulanan** | Setiap akhir bulan: pendapatan & beban ditutup → 3301 Laba Berjalan → 3201 Laba Ditahan |
| **Kuartalan** | Setiap akhir kuartal: akumulasi 3 bulan ditutup → 3201 Laba Ditahan |
| **Tahunan** | Setiap akhir tahun buku: akumulasi 12 bulan ditutup → 3201 Laba Ditahan |

**Aturan otomatis:**
1. Saat periode ditutup, engine **otomatis membuat jurnal penutup** (closing entry): seluruh saldo Pendapatan & Beban → 3301 Laba Berjalan.
2. Saldo 3301 Laba Berjalan **langsung dipindah** ke 3201 Laba Ditahan pada jurnal yang sama (laba berjalan tidak menumpuk antar periode).
3. Jika periode **belum ditutup**, saldo pendapatan/beban tetap tampil sebagai Laba Berjalan di laporan (laba berjalan = pendapatan − beban periode berjalan) — perhitungan **real-time** dari buku besar.
4. Bila periode dikunci, **tidak ada posting manual** ke periode tersebut — koreksi laba periode lalu hanya via prior period adjustment di periode berjalan.
5. Saat periode dibuka kembali (unlock, dengan otorisasi), jurnal penutup **dibatalkan otomatis** dan saldo kembali ke pendapatan/beban periode.

**Contoh:**
- Entitas periode bulanan. Des 2026: Pendapatan Rp 50 jt, Beban Rp 30 jt.
- Sebelum tutup buku: Laba Berjalan tampil Rp 20 jt di Laporan Laba Rugi (real-time).
- Setelah tutup Des: Jurnal otomatis Dr Pendapatan 50 jt / Cr Laba Berjalan 50 jt; Dr Laba Berjalan 30 jt / Cr Beban 30 jt; lalu Dr Laba Berjalan 20 jt / Cr Laba Ditahan 20 jt.
- Jan 2027 mulai dengan saldo Laba Ditahan Rp 20 jt.

**Efek ke laporan:**
- **Laporan Laba Rugi** selalu menampilkan laba periode berjalan (tidak termasuk periode yang sudah ditutup).
- **Neraca** menampilkan 3201 Laba Ditahan (kumulatif) + 3301 Laba Berjalan (periode berjalan, jika belum ditutup).

### 21.3 Alur Tutup Buku

1. **Review** — pastikan semua transaksi periode tercatat; jalankan balance check.
2. **Jurnal penyesuaian** (Mode Akuntan): akrual (§16), penyusutan, revaluasi, stok opname, penyisihan piutang, pajak.
3. **Tutup buku (otomatis):**
   - Pindahkan saldo Pendapatan & Beban → 3301 Laba Berjalan.
   - Pindahkan 3301 Laba Berjalan → 3201 Laba Ditahan.
   - Reklasifikasi surplus revaluasi yang direalisasi (jika ada) → 3201 Laba Ditahan.
4. **Kunci periode** — periode terkunci: tidak ada edit/void; koreksi hanya melalui jurnal koreksi periode berjalan (opening balance adjustment) bila diperlukan.
5. **Buka periode baru** — saldo aset, liabilitas, ekuitas dibawa otomatis; laporan periode baru mulai dari saldo awal.

### 21.4 Matriks Operasi Periode Terkunci

| Operasi | Periode OPEN | Periode CLOSED |
|---|---|---|
| Posting transaksi biasa | Diizinkan setelah validasi | Ditolak `PERIOD_CLOSED` |
| Void transaksi posted | Diizinkan dengan alasan dan approval bila dikonfigurasi | Ditolak; gunakan koreksi di periode berjalan atau unlock resmi |
| Jurnal koreksi | Diizinkan di periode aktif | Diposting di periode berjalan dengan `reversal_of_id` dan penanda prior-period adjustment |
| Unlock | Tidak diperlukan | Hanya role berwenang + approval + audit trail; membatalkan jurnal penutup secara atomik |
| Recurring jatuh tempo | Diposting sesuai jadwal | Ditunda ke periode OPEN berikutnya |

**Aturan:** tidak ada posting biasa, edit, atau void langsung ke periode CLOSED. Exception hanya melalui prosedur unlock yang diaudit.

**Lifecycle periode:** `FUTURE → OPEN → CLOSED → REOPENED → CLOSED`. Periode FUTURE tidak menerima posting; hanya periode OPEN yang menerima transaksi. `entry_date` wajib berada dalam range periode terkait. Closing entry memiliki intent/idempotency key khusus per period dan tidak boleh bertabrakan dengan intent transaksi biasa.

### 21.5 Edge Cases Penutupan Periode
| Kasus | Perlakuan |
|---|---|
| Masih ada saldo 3105 Suspense saat tutup buku | Tutup buku ditolak (`SUSPENSE_OPEN`) — wajib direklasifikasi dulu |
| Ada dokumen belum lengkap (SO belum jadi INV, GRN belum ditagih) | Daftar ceklist muncul; bisa ditutup dengan catatan (configurabel) |
| Koreksi ditemukan setelah periode dikunci | Jurnal koreksi periode berjalan (bukan buka periode lama); sistem menandai sebagai prior period adjustment |
| Buka kembali periode (unlock) | Hanya dengan otorisasi akuntan senior + audit trail; jurnal penutup dibatalkan otomatis |
| Akrual otomatis yang dibalik | Jurnal balik tanggal 1 periode berikutnya (auto-reversal) |
| **Ganti settingan periode** (bulanan → kuartalan) | Berlaku untuk periode berikutnya; periode yang sudah ditutup tetap (tidak dihitung ulang) |
| **Laba berjalan saat periode berjalan** | Dihitung real-time dari pendapatan − beban periode berjalan (tanpa menunggu tutup buku) |

---

## 22. Konsolidasi (PSAK 65 / IFRS 10)

- **Entitas & induk:** setiap cabang/entitas punya buku sendiri (COA terpisah).
- **Jurnal eliminasi otomatis** saat konsolidasi:
  - Eliminasi transaksi antar-entitas (penjualan/pembelian antar cabang).
  - Eliminasi piutang-hutang antar-entitas.
  - Eliminasi keuntungan belum direalisasi dari persediaan antar-entitas.
- Laporan konsolidasi = gabungan + jurnal eliminasi → laporan final.

**Edge cases:**
| Kasus | Perlakuan |
|---|---|
| **Laba antar-entitas dalam persediaan** | Contoh: Cabang A jual ke B cost 80, harga 100; B belum jual → eliminasi laba 20: Dr 4101 Pendapatan 100 / Cr 5101 HPP 80 / Cr 1301 Persediaan 20 |
| Piutang-hutang antar entitas tidak sama (selisih) | Selisih ditelusuri & dieliminasi dengan memeriksa dokumen dua arah |
| Cabang pakai mata uang fungsional berbeda | Konversi kurs (current method) sebelum eliminasi; selisih kurs konsolidasi → OCI |
| Transaksi antar-entitas di periode berbeda (labu belum direalisasi lintas tahun) | Pelacakan saldo laba ditahan saat periode berikutnya |

---

## 23. Laporan Pendukung GL & Trial Balance

### 23.1 Trial Balance (Neraca Saldo)
- **Definisi:** daftar seluruh akun dengan saldo debet dan kredit pada tanggal tertentu — memastikan total debet = total kredit.
- **Penyajian:** per akun (kode, nama, debet, kredit), total di bagian bawah.
- **Fitur:** filter per periode, per tanggal, per dimensi (jika aktif); kolom saldo awal, mutasi, saldo akhir.
- **Invariant:** total debet = total kredit **wajib** nol selisih — jika tidak, laporan tidak ditampilkan & alert dipicu (reuse `balanceCheck`).
- **Kegunaan:** dasar akuntan mereview sebelum tutup buku & menyusun laporan.

### 23.2 Jurnal Register
- Daftar seluruh jurnal dalam rentang periode/tanggal: nomor jurnal, tanggal, deskripsi, akun, debet, kredit, source_ref.
- Filter: per akun, per dokumen asal (INV/PO/BK/KK), per user, per jenis (MANUAL/SISTEM).

### 23.3 Buku Besar (General Ledger) per Akun
- Per akun: saldo awal, setiap baris mutasi (jurnal, source_ref, lawan akun), saldo berjalan, saldo akhir.
- Drill-down: klik baris → jurnal asal → dokumen sumber.

### 23.4 Mutasi Kas & Bank
- Per akun CASH/BANK: saldo awal, setiap transaksi (uang masuk/keluar, transfer), saldo berjalan.
- Menjadi dasar rekonsiliasi bank (§24).

### 23.5 Perbandingan Antar Periode
- Laporan Laba Rugi & Neraca menampilkan **kolom periode berjalan vs periode sebelumnya** (+ selisih & %).
- Konfigurabel: bandingkan dengan periode lalu, atau tahun lalu (same period last year).

---

## 24. Bank Feeds & Rekonsiliasi Otomatis

### 24.1 Import Mutasi Bank
- Import file mutasi bank (CSV/XLS/OFX) atau integrasi API bank (fase lanjut).
- Setiap baris mutasi: tanggal, deskripsi, debet/kredit, saldo, referensi.

### 24.2 Pencocokan Otomatis (Auto-Match)
| Sumber | Aturan Pencocokan |
|---|---|
| Mutasi bank ↔ Transaksi kas di sistem | Cocokkan berdasarkan: tanggal (± N hari, konfigurabel), nominal, referensi/deskripsi |
| Mutasi bank ↔ Invoice/Piutang | Cocokkan pembayaran pelanggan dengan piutang terbuka |
| Mutasi bank ↔ Tagihan/Hutang | Cocokkan pembayaran ke supplier dengan hutang terbuka |

- **Skor kecocokan:** tanggal + nominal + referensi; sistem memberi saran "match" jika skor tinggi, "review" jika meragukan.
- **Unmatched:** transaksi bank tanpa pasangan → buat transaksi baru (uang masuk/keluar) atau tandai "pending review".
- **Salah match** → un-match dengan audit trail.

### 24.3 Rekonsiliasi
- Setelah match, sistem hitung **selisih rekonsiliasi** (mutasi bank yang belum cocok).
- Saldo bank sistem = saldo bank riil → status **Reconciled**.
- Rekonsiliasi disimpan per periode; laporan rekonsiliasi (dokumen pendukung).

**Tabel terkait (DATA_MODEL.md):** `bank_statements` (mutasi + saldo), `bank_statement_lines` (baris, match_status), `bank_reconciliations` (batch: saldo bank vs buku, selisih wajib 0 saat RECONCILED).

### 24.4 Kartu Kredit (Tipe CREDIT_CARD)
| Peristiwa | Debet | Kredit |
|---|---|---|
| Transaksi pakai kartu kredit | Beban terkait | 2501 Kartu Kredit |
| Bayar tagihan kartu kredit | 2501 Kartu Kredit | 1102 Bank |

- Mirip alur rekonsiliasi bank; saldo kartu = tagihan yang belum dibayar.

### 24.5 Pinjaman (Tipe LOAN)
| Peristiwa | Debet | Kredit |
|---|---|---|
| Terima pinjaman | 1102 Bank | 2401 Utang Bank |
| Bayar angsuran (pokok) | 2401 Utang Bank | 1102 Bank |
| Bayar bunga | 5906 Beban Bunga | 1102 Bank |

- **Jadwal amortisasi pinjaman** (skedul angsuran: pokok + bunga per bulan) dibuat otomatis saat pinjaman dicatat; pembayaran dicocokkan ke jadwal.

---

## 25. Transaksi Berulang (Recurring Transactions)

### 25.1 Definisi
Template transaksi yang di-post secara otomatis pada jadwal: **sewa bulanan, langganan, cicilan, gaji, akrual tetap**.

### 25.2 Pengaturan
| Field | Keterangan |
|---|---|
| Frekuensi | Harian, mingguan, bulanan, kuartalan, tahunan |
| Tanggal mulai & (opsional) berakhir | Rentang aktif recurring |
| Jumlah & akun | Sama setiap kali (template) atau bervariasi (ingatkan, bukan auto-post) |
| Opsi | Auto-post / hanya pengingat (reminder) |

### 25.3 Perilaku Engine
- Auto-post dibuat sebagai **jurnal SISTEM** dengan `source_ref` = `RCR-{YYYY}-{seq}` (satu per instance).
- Jika tanggal jatuh pada periode terkunci → ditunda ke periode berikutnya (tidak gagal).
- **Idempoten:** jadwal yang sama tidak membuat duplikat (unique constraint per instance).
- Pengingat (reminder) muncul di dashboard sampai di-post manual.
- Recurring dapat di-pause, diresume, atau dihapus (instance yang sudah di-post tetap ada).

### 25.4 Contoh
- Sewa Rp 2.000.000/bulan, tiap tanggal 1 → Dr 5202 Beban Sewa / Cr 1102 Bank, otomatis.

**Tabel terkait (DATA_MODEL.md):** `recurring_templates` (frekuensi, baris jurnal `lines`, auto_post), `recurring_instances` (per jatuh tempo, UNIQUE `(template_id, due_date)` — idempotensi).

---

## 26. Dimensi / Cost Center & Pelaporan

### 26.1 Konsep
Dimensi = label tambahan pada baris jurnal untuk analisis: **per proyek, departemen, cabang, produk, outlet**.

### 26.2 Desain
| Aspek | Detail |
|---|---|
| Dimensi standar | Cabang (wajib untuk multi-cabang), plus dimensi kustom (proyek, departemen, produk) — konfigurabel per entitas |
| Penerapan | Setiap baris jurnal (JournalLine) dapat membawa `dimensi_id`; posting wajib melengkapi dimensi untuk akun yang ditandai "wajib dimensi" |
| Default dimensi | Jika tidak diisi, memakai dimensi default entitas (mis. cabang utama) |
| Pemisahan | Dimensi **tidak mengubah jurnal** (double-entry tetap), hanya tagging untuk analisis |

### 26.3 Laporan per Dimensi
- Laba Rugi per proyek/departemen/cabang.
- Neraca per cabang.
- Trial balance per dimensi (filter).
- Perbandingan antar dimensi.

### 26.4 Kaitannya dengan Job Costing
- Job produksi = salah satu dimensi opsional; biaya per job (§11) tetap dihitung dari sub-ledger, dimensi menyediakan analisis silang.

---

## 27. Master Data (Customer, Supplier, Item)

### 27.1 Customer & Supplier
| Field | Keterangan |
|---|---|
| Identitas | Nama, NPWP, alamat, kontak, kategori |
| Syarat pembayaran | Term (mis. 2/10 n/30), jatuh tempo default |
| Credit limit | Batas piutang maksimum — posting melebihi limit ditolak (`CREDIT_LIMIT_EXCEEDED`) atau butuh approval |
| Saldo awal | Piutang/hutang pembukaan per pelanggan/pemasok |
| Akun default | Akun AR/AP & pendapatan/beban default |

### 27.2 Item (Barang/Jasa)
| Field | Keterangan |
|---|---|
| Identitas | Kode, nama, jenis (barang/jasa), UoM (unit), gudang default |
| Harga | Price list (harga jual & beli), multi-level harga |
| Akun | Akun pendapatan, HPP, persediaan default |
| Stok | Metode penilaian per item (FIFO/rata-rata), stok minimum (untuk reminder) |

---

## 28. Budget & Varians

### 28.1 Setup Budget
- Budget per akun (atau per dimensi), per periode (bulanan/kuartalan/tahunan).
- Dibuat manual atau **di-generate dari tahun lalu** (dengan % kenaikan).

### 28.2 Laporan Realisasi vs Budget
| Kolom | Isi |
|---|---|
| Akun | Nama akun |
| Budget | Nilai anggaran periode |
| Realisasi | Nilai aktual dari buku besar |
| Selisih & % | Variance & persentase |

- Bisa di-breakdown per dimensi (proyek/cabang).
- Alert otomatis jika realisasi melebihi budget > X% (konfigurabel).

---

## 29. Attachment & Bukti Transaksi

- Setiap transaksi (jurnal, invoice, PO, GRN, DO, pembayaran) dapat dilampiri **file bukti** (foto struk, PDF, scan faktur).
- Attachment tersimpan aman (enkripsi at-rest) & tampil di drill-down jurnal.
- Upload dilakukan sebelum/bersamaan dengan posting; dokumen yang belum punya attachment dapat ditandai "belum ada bukti" (reminder di dashboard).
- Dukungan OCR: foto struk → **pratinjau otomatis** ke data transaksi (fase lanjut, lihat PRD).

---

## 30. Invariant, Balance Check, Void & Error Codes

### 30.1 Invariant Wajib
| Invariant | Pemeriksaan |
|---|---|
| `totalDebit == totalKredit` | Setiap journal entry |
| `sum(ledger) == 0` untuk seluruh akun pada setiap titik | Buku besar |
| Saldo aset = saldo liabilitas + ekuitas | Neraca selalu balance |
| Stok tidak negatif (kecuali diizinkan config) | Sub-ledger persediaan |
| Piutang/hutang tidak negatif setelah pelunasan | Sub-ledger AR/AP |
| Hash chain valid (setiap jurnal = hash dari jurnal sebelumnya) | Anti-tamper |

### 30.2 Balance Check
Fungsi `balanceCheck(ledger)` dieksekusi:
- Setelah setiap posting (synchronous).
- Sebelum penutupan periode.
- Saat membangun laporan (defensif — jika gagal, laporan tidak dihasilkan & alert dipicu).

### 30.3 Void & Koreksi
- **Void**: tidak menghapus baris; mencatat jurnal balik + status `VOID` pada jurnal asal dengan alasan.
- **Koreksi**: jurnal koreksi baru, bukan edit in-place (kecuali transaksi belum diposting).
- Semua void/koreksi tercatat di audit trail (user, waktu, alasan).

### 30.4 Error Codes (Ringkasan)
| Kode | Pesan | Saat Terjadi |
|---|---|---|
| `NOT_BALANCED` | Jurnal tidak balance | Posting jurnal manual |
| `ACCOUNT_NOT_FOUND` | Akun tidak ditemukan / nonaktif | Pemetaan kategori/akun |
| `PERIOD_CLOSED` | Periode sudah dikunci | Posting ke periode tertutup |
| `STOCK_NEGATIVE` | Stok tidak mencukupi | DO/penjualan melebihi stok |
| `DP_EXCEEDS_SO` | Uang muka melebihi nilai order | Input DP |
| `INV_EXCEEDS_DO` | Invoice melebihi sisa pengiriman | Input invoice |
| `INVALID_TAX_RATE` | Tarif pajak tidak valid | Konfigurasi pajak |
| `CURRENCY_MISMATCH` | Kurs tidak tersedia untuk tanggal | Transaksi valas |
| `VOID_ALREADY_POSTED` | Void ditolak karena periode terkunci | Void transaksi lama |
| `OPENING_LOCKED` | Saldo awal tidak dapat diubah | Edit saldo awal |
| `DISCOUNT_EXCEEDS_RECEIVABLE` | Diskon melebihi sisa piutang | Pelunasan dengan diskon |
| `CN_EXCEEDS_INVOICE` | Credit note melebihi invoice | Retur/credit note |
| `OVER_DELIVERY` | Penerimaan melebihi PO | GRN tanpa approval |
| `SUSPENSE_OPEN` | Ada saldo suspense | Tutup buku |
| `HAS_DESCENDANT` | Dokumen induk punya turunan | Void berjenjang |
| `DUPLICATE_INTENT` | Intent ganda (idempotensi) | Retry posting |
| `ACCOUNT_EXISTS` | Kode akun sudah dipakai | Buat akun kustom |
| `INVALID_ACCOUNT_CODE` | Kode akun di luar rentang kelompok | Buat akun kustom |
| `ACCOUNT_IN_USE` | Akun sudah memiliki transaksi | Hapus akun |
| `POST_TO_GROUP_ACCOUNT` | Posting ke akun grup (harus ke detail) | Posting jurnal |
| `PERIOD_ALREADY_CLOSED` | Periode sudah ditutup dua kali | Tutup buku ulang |
| `PERIOD_OVERLAP` | Dua periode tumpang tindih | Setup periode akuntansi |
| `TYPE_GROUP_MISMATCH` | Tipe akun tidak konsisten dengan kelompok | Buat akun kustom |
| `CREDIT_LIMIT_EXCEEDED` | Melebihi credit limit pelanggan | Invoice/pembayaran |
| `RECONCILIATION_MISMATCH` | Selisih rekonsiliasi tidak nol | Tutup rekonsiliasi bank |
| `DIMENSION_REQUIRED` | Akun wajib dimensi tapi tidak diisi | Posting jurnal |
| `IDEMPOTENCY_KEY_REUSE` | Key dipakai ulang dengan payload berbeda | Retry posting |
| `ENTRY_DATE_OUTSIDE_PERIOD` | Tanggal transaksi di luar range periode | Posting jurnal |
| `SERVICE_INVENTORY_INVALID` | Jasa dipakai pada inventory/DO/GRN | Posting sub-ledger |
| `GOODS_POLICY_INVALID` | Barang tracked tidak memiliki akun persediaan/costing | Setup item/posting |

---

## 31. Kerangka Laporan (EMKM / ETAP / SAK Umum)

Dari satu sumber buku besar, laporan dapat disajikan dalam 3 kerangka:

| Kerangka | Perbedaan Penyajian |
|---|---|
| **SAK EMKM** | 3 laporan: Posisi Keuangan (sederhana), Laba Rugi (tunggal), Catatan atas LK; tanpa pengukuran kompleks |
| **SAK ETAP** | Penyajian lebih rinci; tanpa perhitungan pajak tangguhan wajib |
| **SAK Umum (PSAK)** | Lengkap: Laba Rugi & Penghasilan Komprehensif, Posisi Keuangan, Perubahan Ekuitas, Arus Kas, Catatan LK; pengungkapan penuh |

Engine memisahkan **data (pencatatan)** dari **penyajian (reporting)** — pengguna memilih kerangka saat generate laporan.

---

## 32. Struktur Modul Teknis (Referensi)

```
packages/accounting-engine/          # pure library, tanpa IO
  ├── chart-of-accounts/             # COA inti + pemetaan kategori + tipe akun
  ├── double-entry/                  # journal entry, posting, balance check, hash chain
  ├── document-numbering/            # nomor dokumen per jenis & entitas
  ├── opening-balances/              # setup saldo awal
  ├── transactions/                  # intent → jurnal (cash, sales, purchase)
  ├── inventory/                     # FIFO, moving average, costing, NRV, batch
  ├── production/                    # job order costing + overhead variance
  ├── fixed-assets/                  # depresiasi, revaluasi, disposisi, komponen
  ├── tax/                           # PPN, PPh, pajak tangguhan
  ├── receivables/                   # ECL, aging, penyisihan piutang
  ├── accruals/                      # akrual & beban dibayar dimuka
  ├── revenue/                       # PSAK 72 multi-element & jasa
  ├── petty-cash/                    # kas kecil (imprest)
  ├── foreign-currency/              # kurs, selisih kurs, revaluasi
  ├── leases/                        # PSAK 73
  ├── reporting/                     # trial balance, GL, buku besar, mutasi kas
  ├── bank-reconciliation/           # bank feeds, auto-match, rekonsiliasi
  ├── recurring/                     # transaksi berulang
  ├── dimensions/                    # cost center, proyek, departemen, cabang
  ├── master-data/                   # customer, supplier, item
  ├── budgeting/                     # budget & variance
  ├── attachments/                   # bukti transaksi & OCR
  ├── period-close/                  # tutup buku & lock
  ├── consolidation/                 # eliminasi antar-entitas
  └── reporting-frameworks/          # kerangka EMKM/ETAP/SAK Umum
```

**Praktik:** setiap modul = kumpulan fungsi murni `(state, intent) → { journal, subLedgerEffects }` — mudah di-unit-test dengan tabel kasus.

---

## 33. Test Matrix & Prioritas Implementasi

### 33.1 Test Matrix — Kasus Normal
| Skenario | Intent | Ekspektasi Jurnal | Verifikasi |
|---|---|---|---|
| Penjualan tunai Rp 500.000 | CASH_IN | Dr Kas 500.000 / Cr Pendapatan 500.000 | Balance; pendapatan bertambah |
| Alur penjualan kredit + DP + pelunasan (§7) | SALES_FLOW | 4 jurnal sesuai alur | Piutang akhir = 0; DP terealisasi |
| Pembelian + PPN + DP (§10) | PURCHASE_FLOW | Persediaan + PPN Masukan; hutang tersisa 4.550.000 | Hutang akhir = 4.550.000 |
| Job selesai 100 unit (§11) | PRODUCTION_FINISH | Dr Barang Jadi 7.000.000 / Cr WIP 7.000.000 | WIP = 0; unit cost 70.000 |
| Revaluasi naik Rp 20.000.000 | ASSET_REVALUATION | Dr Aset 20.000.000 / Cr OCI 20.000.000 | Neraca balance |
| Penjualan aset (§12.3) | ASSET_DISPOSAL | Kas + Akumulasi; Keuntungan 5.000.000 | Akun aset nol |
| Retur penjualan (§8.1) | SALES_RETURN | Retur & balik HPP + stok | Pendapatan & stok terkoreksi |
| Akrual listrik akhir periode (§16.2) | ACCRUE_EXPENSE | Dr Beban / Cr 2106 | Dibalik di periode berikutnya |
| PPh final akrual (§13.2) | TAX_ACCRUAL | Dr 5208 / Cr 2203 | Utang PPh Final terbentuk |
| Void invoice | SALES_INVOICE_VOID | Jurnal balik + status VOID | Selisih = 0, audit trail tercatat |
| **Trial balance konsisten** (§23) | TRIAL_BALANCE | Total debet = total kredit | Nol selisih |
| **Recurring sewa bulanan** (§25) | RECURRING_POST | Dr 5202 / Cr 1102 tiap bulan | Satu jurnal per instance |

### 33.2 Test Matrix — Edge Cases
| Skenario | Perlakuan yang Diharapkan | Verifikasi |
|---|---|---|
| Pembatalan order setelah DP (§7.1) | Dr 2201 / Cr Kas (refund) | 2201 menjadi 0 |
| Pelunasan customer lebih (overpayment) (§7.1) | Cr 2402 Kelebihan Pembayaran Pelanggan | Saldo 2402 = selisih |
| Pembayaran supplier lebih (§10.3) | Dr 2101 sebesar hutang + Dr 1204 sebesar selisih / Cr Kas sebesar pembayaran total | Saldo 1204 = tagihan kepada supplier |
| Retur barang rusak (§8.3) | Dr 5901 Beban / Cr 1301 (tidak balik stok) | Stok tidak bertambah |
| Stok negatif saat DO (§9.4) | Ditolak `STOCK_NEGATIVE` (default) | Error terlempar |
| Selisih harga tagihan vs PO (§10.3) | Selisih ke 1301/5901 | Hutang sesuai tagihan |
| Pembayaran valas dengan kurs berubah (§19) | Dr Piutang / Cr Kas / Cr 4904 selisih | Selisih kurs tercatat |
| Revaluasi turun setelah surplus (§12.2) | Dr 3401 (surplus) dulu, sisanya 5207 | OCI tidak negatif |
| Tutup buku dengan suspense terbuka (§21) | Ditolak `SUSPENSE_OPEN` | Tutup buku gagal |
| Buat akun kustom tanpa kelompok (§3.1) | Ditolak — wajib pilih kelompok | Error validasi |
| Buat akun tipe tidak konsisten (§3.0.3) | Ditolak `TYPE_GROUP_MISMATCH` | Error terlempar |
| Buat akun kode duplikat (§3.1) | Ditolak `ACCOUNT_EXISTS` | Error terlempar |
| Posting ke akun grup (§3.1) | Ditolak `POST_TO_GROUP_ACCOUNT` | Posting gagal |
| Hapus akun yang sudah bertransaksi (§3.1) | Ditolak `ACCOUNT_IN_USE` | Akun nonaktif |
| Tutup periode → laba otomatis (§21.2) | Jurnal penutup otomatis → Laba Ditahan | Laba Berjalan 0 |
| Unlock periode → jurnal penutup dibatalkan (§21.2) | Auto-reversal jurnal penutup | Saldo kembali |
| Void DO yang sudah punya INV (§4.3) | Ditolak `HAS_DESCENDANT` | Void gagal |
| Import data tidak balance | Ditolak `NOT_BALANCED` | Tidak ada partial import |
| **Invoice melebihi credit limit** (§27.1) | Ditolak `CREDIT_LIMIT_EXCEEDED` | Error terlempar |
| **Posting tanpa dimensi wajib** (§26.2) | Ditolak `DIMENSION_REQUIRED` | Error terlempar |
| **Bank feed match ganda** (§24.2) | Pilih match dengan skor tertinggi; sisanya review | Tidak ada double-post |
| **Jurnal tidak balance** | Deferred trigger/procedure menolak commit | Tidak ada partial journal |
| **Retry posting dengan idempotency key sama** | Kembalikan hasil jurnal pertama | Tepat satu journal effect |
| **Retry dengan payload berbeda** | Ditolak `IDEMPOTENCY_KEY_REUSE` | Tidak ada perubahan kedua |
| **Cross-tenant period/account reference** | Ditolak composite FK/tenant validation | Tidak ada data lintas tenant |
| **Entry date di luar period range** | Ditolak `ENTRY_DATE_OUTSIDE_PERIOD` | Tidak ada posting |
| **Mutasi journal POSTED** | Ditolak immutable trigger/privilege | Hash dan lines tetap |
| **Service dipakai pada DO/GRN** | Ditolak `SERVICE_INVENTORY_INVALID` | Tidak ada inventory movement |
| **Goods tracked tanpa inventory account/costing** | Ditolak validasi item policy | Tidak ada posting |
| **Double reversal** | Ditolak unique `reversal_of_id` | Tepat satu reversal |

### 33.3 Prioritas Implementasi
| Prioritas | Modul | Fase PRD |
|---|---|---|
| P1 | Double-entry inti, **COA (custom account & laba berjalan otomatis)**, saldo awal, kas/bank, kategori → jurnal | Fase 1 |
| P2 | Persediaan (FIFO/rata-rata), alur penjualan & pembelian, retur & diskon, nomor dokumen, **Trial Balance & GL reports** | Fase 2 |
| P3 | Produksi (job costing), aset tetap, penyisihan piutang (ECL), pajak lanjutan, budget, attachment | Fase 3 |
| P4 | Jasa & multi-element, kas kecil, multi-mata uang, sewa, konsolidasi, integrasi DJP, bank feeds lanjutan, OCR | Fase 3+ |

---

*Dokumen ini referensi teknis untuk tim engineering & review konsultan akuntansi. Seluruh jurnal mengikuti PSAK/IFRS terkini; pembaruan standar dilakukan terjadwal.*
