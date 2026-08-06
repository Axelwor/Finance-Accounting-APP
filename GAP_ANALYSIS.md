# Gap Analysis — Perbandingan dengan Software Akuntansi Lain

**Lampiran PRD** — Software Pembukuan Mudah dengan Mesin Akuntansi Standar IFRS/PSAK  
**Versi:** 1.2  
**Tanggal:** 2026-08-06  
**Status:** Review  
**Owner:** Product  
**Normative:** Tidak; competitive prioritization

---

## 1. Tujuan

Membandingkan cakupan produk (PRD + ACCOUNTING_ENGINE.md) dengan fitur umum pada software akuntansi kompetitor (QuickBooks, Xero, Jurnal.id, Accurate, Kledo, MYOB) untuk mengidentifikasi **fitur yang belum ada** dan memprioritaskan penambahan.

---

## 2. Matriks Perbandingan

### 2.1 Modul Inti
| Fitur | Kita | QBO | Xero | Jurnal.id | Accurate | Kledo | Keterangan |
|---|---|---|---|---|---|---|---|
| Double-entry engine | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | Fondasi |
| Trial Balance | ❌ | ✅ | ✅ | ✅ | ✅ | ✅ | **WAJIB segera** |
| Jurnal Umum & Buku Besar | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | |
| COA custom + tipe akun | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | |
| Laporan Laba Rugi / Neraca / Arus Kas | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | |
| Perbandingan antar periode | ❌ | ✅ | ✅ | ✅ | ✅ | ✅ | Sedang |

### 2.2 Penjualan & Pembelian
| Fitur | Kita | QBO | Xero | Jurnal.id | Accurate | Kledo |
|---|---|---|---|---|---|---|
| Alur SQ→SO→DP→DO→INV→Lunas | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Credit note / retur | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Diskon & pembayaran parsial | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Overpayment | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Dunning/reminder tagihan otomatis | ❌ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Credit limit pelanggan | ❌ | ✅ | ✅ | ✅ | ✅ | ✅ |

### 2.3 Kas & Bank
| Fitur | Kita | QBO | Xero | Jurnal.id | Accurate | Kledo |
|---|---|---|---|---|---|---|
| Rekonsiliasi manual | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| **Bank feeds** (import mutasi bank, match otomatis) | ❌ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Transfer antar akun | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Kas kecil imprest | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Tipe akun Credit Card & Loan | ❌ | ✅ | ✅ | ✅ | ✅ | ✅ |

### 2.4 Persediaan & Produksi
| Fitur | Kita | QBO | Xero | Jurnal.id | Accurate | Kledo |
|---|---|---|---|---|---|---|
| FIFO / rata-rata bergerak | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Multi-gudang | ❌ | ❌ | ✅ | ✅ | ✅ | ✅ |
| Batch/serial number | ❌ | ❌ | ✅ | ✅ | ✅ | ❌ |
| Job order costing | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| BOM multi-level / MRP | ❌ (fase lanjut) | ❌ | ❌ | ❌ | ✅ | ❌ |

### 2.5 Penggajian & Pajak
| Fitur | Kita | QBO | Xero | Jurnal.id | Accurate | Kledo |
|---|---|---|---|---|---|---|
| PPN / PPh (21, 23/26, 22, 4(2), final) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Payroll penuh (slip gaji, BPJS, THR) | ❌ (v2+) | ✅ | ✅ | ✅ | ✅ | ✅ |
| e-Faktur / e-Bupot integrasi DJP | 🔜 (fase 3) | ❌ | ❌ | ✅ | ✅ | ✅ |
| Pajak tangguhan | ✅ | ✅ | ✅ | ❌ | ✅ | ❌ |

### 2.6 Analisis & Pengaturan
| Fitur | Kita | QBO | Xero | Jurnal.id | Accurate | Kledo |
|---|---|---|---|---|---|---|
| Budget & variance | ❌ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Cost center / dimensi | ❌ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Approval workflow | ❌ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Transaksi berulang (recurring) | ❌ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Attachment/foto bukti | ❌ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Multi-mata uang | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Rasio keuangan & forecast | ❌ | ✅ | ✅ | ❌ | ✅ | ❌ |
| API / integrasi eksternal | 🔜 (fase 3) | ✅ | ✅ | ✅ | ✅ | ✅ |

---

## 3. Prioritasi Penambahan

### P1 / Fase 2 — Wajib (fondasi akuntansi, cepat)
| Fitur | Alasan |
|---|---|
| **Trial Balance** | Laporan dasar yang pasti dicari akuntan; menutup "mode akuntan" |
| **GL reports**: Jurnal Register, Buku Besar per akun, Mutasi Kas | Pendukung audit & drill-down |
| **Perbandingan antar periode** | Standar di semua kompetitor |

### P1 / Fase 2 — Sangat Disarankan (nilai tinggi untuk kemudahan & analisis)
| Fitur | Alasan |
|---|---|
| **Transaksi berulang (recurring)** | Murah dibangun, otomatiskan sewa/langganan/cicilan — terasa "pintar" bagi pemula |
| **Bank feeds** | Input transaksi bisa ~90% otomatis dari mutasi bank; sejalan dengan nilai utama "kemudahan" |
| **Cost center / dimensi** | Analisis laba per produk/departemen/cabang; kebutuhan multi-cabang |
| **Master data lengkap** | Customer (credit limit, term), supplier, item (price list, UoM, gudang) |

### P2 / Fase 3 — Disarankan
| Fitur | Alasan |
|---|---|
| Budget & variance | Penting untuk yang mulai planning |
| Approval workflow | Dibutuhkan saat perusahaan tumbuh (SoD) |
| Attachment bukti | Kepatuhan & kemudahan input |
| Dunning/reminder otomatis | Meningkatkan kecepatan tagihan |

### P3 / Fase 3+ — Bisa Ditunda (jelas alasannya)
| Fitur | Alasan |
|---|---|
| Payroll penuh | Kompleks; regulasi berubah; target awal UMKM belum butuh |
| Batch/serial number | Jarang dipakai UMKM |
| Tipe akun Credit Card & Loan | Jarang dipakai UMKM; bisa ditambahkan belakangan |
| BOM multi-level / MRP | Di luar cakupan produk sederhana |
| Integrasi payment gateway | Butuh legal/partner (Midtrans, Xendit) |

---

## 4. Kesimpulan & Rencana

- **Fitur P1 / Fase 2** (Trial Balance, GL reports, perbandingan periode, recurring, master data, rekonsiliasi, dimensi dasar) sudah didetailkan di ACCOUNTING_ENGINE.md dan USER_STORIES.md.
- **Fitur P2 / Fase 3** (budget, approval, attachment, ECL, aset, produksi, pajak lanjutan) sudah memiliki detail awal dan acceptance criteria; implementasi mengikuti roadmap.
- **Fitur P3 / Fase 3+** (payroll, batch/serial, BOM/MRP, integrasi payment gateway, integrasi DJP, OCR) tetap menjadi roadmap, bukan bagian MVP.

---

*Dokumen ini referensi untuk tim produk & engineering; prioritas dapat disesuaikan dengan umpan balik pengguna.*
