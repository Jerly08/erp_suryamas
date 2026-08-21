# 📘 Panduan Lengkap Penggunaan Sistem Akuntansi Modern

Dokumen ini merupakan kumpulan panduan penggunaan untuk seluruh modul dalam aplikasi Akuntansi Modern.

---

# 1. Panduan Penggunaan Awal (Initial Setup)

Selamat datang di Sistem Akuntansi Modern! Dokumen ini akan memandu Anda melakukan konfigurasi awal agar aplikasi siap digunakan untuk operasional bisnis Anda.

## 📋 Langkah 1: Konfigurasi Perusahaan (Company Settings)

Langkah pertama adalah mengatur identitas perusahaan Anda. Informasi ini akan muncul di setiap dokumen resmi seperti Invoice, Purchase Order, dan Laporan Keuangan.

1.  **Akses Menu Pengaturan:**
    *   Klik ikon **Settings** (⚙️) di pojok kanan atas atau sidebar menu.
    *   Pilih **"Company Settings"** atau **"Pengaturan Perusahaan"**.

2.  **Isi Informasi Perusahaan:**
    Pada tab **Company Info**, lengkapi data berikut:
    *   **Company Name**: Nama resmi perusahaan (Wajib).
    *   **Address**: Alamat lengkap perusahaan.
    *   **Phone**: Nomor telepon kontak.
    *   **Email**: Email resmi perusahaan.
    *   **Tax Number (NPWP)**: Nomor pokok wajib pajak perusahaan.

3.  **Konfigurasi Sistem (System Config):**
    Pindah ke tab **System Config** untuk mengatur preferensi:
    *   **Language**: Pilih **Bahasa Indonesia** atau **English**.
    *   **Timezone**: Pilih zona waktu (WIB/WITA/WIT).
    *   **Date Format**: Format tanggal yang diinginkan (misal: DD/MM/YYYY).
    *   **Default Tax Rate**: Tarif PPN standar (misal: 11%).

## 🏦 Langkah 2: Setup Akun GL (Chart of Accounts)

Sebelum membuat akun bank, Anda perlu memastikan akun GL (General Ledger) untuk Kas & Bank sudah tersedia di Chart of Accounts.

1.  **Akses Menu Chart of Accounts:**
    *   Buka menu **Accounting** > **Chart of Accounts**.

2.  **Buat Akun GL Baru (Jika belum ada):**
    *   Klik tombol **+ New Account**.
    *   Isi form sebagai berikut:
        *   **Account Code**: Gunakan kode kepala 1 (Aset). Disarankan seri `1100` untuk Kas & Bank.
            *   Contoh: `1101-001` untuk "Kas Besar".
            *   Contoh: `1102-001` untuk "Bank BCA".
        *   **Account Name**: Nama akun (misal: "Bank BCA - 1234567890").
        *   **Account Type**: Pilih **ASSET**.
        *   **Parent Account**: Pilih akun induk yang sesuai (misal: `1100 - Cash & Bank`).
    *   Klik **Create Account**.

    > **⚠️ Penting:** Pastikan Tipe Akun adalah **ASSET** dan Kategori (otomatis) adalah **CURRENT ASSET**.

## 🏦 Langkah 3: Setup Kas & Bank (Cash & Bank Setup)

Setelah Akun GL siap, sekarang Anda bisa membuat Akun Kas/Bank fungsional untuk mencatat penerimaan dan pengeluaran uang.

1.  **Akses Menu Kas & Bank:**
    *   Buka menu **Finance** > **Cash & Bank**.

2.  **Tambah Akun Baru:**
    *   Klik tombol **+ Add Account**.

3.  **Isi Detail Akun:**
    *   **Account Type**: Pilih **CASH** (untuk Kas Tunai/Petty Cash) atau **BANK** (untuk Rekening Bank).
    *   **Account Name**: Nama tampilan akun (misal: "Kas Operasional" atau "BCA Utama").
    *   **Currency**: Otomatis IDR.

    **Khusus Tipe BANK:**
    *   **Bank Name**: Nama Bank (misal: BCA, Mandiri).
    *   **Account Number**: Nomor rekening.
    *   **Account Holder**: Atas nama rekening.

4.  **Integrasi COA (Wajib):**
    *   Pindah ke tab **COA Integration**.
    *   Pada dropdown **Select GL Account**, pilih akun GL yang sudah Anda buat di Langkah 2 tadi (misal: `1102-001 - Bank BCA`).
    *   *Sistem akan menolak jika Anda belum memilih akun GL.*

5.  **Saldo Awal (Opening Balance):**
    *   **Opening Balance**: Masukkan saldo awal uang saat ini (jika ada).
    *   **Opening Date**: Tanggal saldo awal tersebut.

6.  **Simpan:**
    *   Klik **Create Account**.

---

# 2. Pengaturan (Settings)

Halaman **Settings** digunakan untuk mengonfigurasi informasi perusahaan, preferensi sistem, penomoran dokumen, akun pajak, dan melakukan tutup buku. Halaman ini hanya dapat diakses oleh pengguna dengan peran **Admin**.

## 1. Informasi Perusahaan (Company Information)

Bagian ini berisi data identitas perusahaan yang akan ditampilkan pada dokumen (Invoice, PO, dll).

### Kolom yang Tersedia:
- **Company Logo**: Unggah logo perusahaan.
- **Company Name**: Nama resmi perusahaan.
- **Address**: Alamat lengkap perusahaan.
- **Phone**: Nomor telepon perusahaan.
- **Email**: Alamat email resmi perusahaan.
- **Website**: Website perusahaan (opsional).
- **Tax Number**: NPWP perusahaan (opsional).

> **Cara Mengubah**: Klik tombol **Edit** atau ubah langsung pada kolom yang tersedia, lalu klik **Save Changes**.

## 2. Konfigurasi Sistem (System Configuration)

Mengatur preferensi tampilan dan format data dalam aplikasi.

### Pengaturan Tersedia:
- **Date Format**: Format tanggal (misal: DD/MM/YYYY).
- **Default Tax Rate**: Persentase pajak default (misal: 11% untuk PPN).
- **Language**: Bahasa aplikasi (Indonesia/Inggris).
- **Timezone**: Zona waktu (WIB/WITA/WIT).
- **Number Format**: Pemisah ribuan dan desimal (titik/koma).
- **Decimal Places**: Jumlah angka di belakang koma.

## 3. Pengaturan Akun Pajak (Tax Account Settings)

Memetakan akun akuntansi (Chart of Accounts) yang akan digunakan otomatis untuk transaksi tertentu.

### Mapping Akun:
- **Sales Receivable**: Akun Piutang Usaha.
- **Sales Revenue**: Akun Pendapatan Penjualan.
- **Purchase Payable**: Akun Hutang Usaha.
- **Inventory**: Akun Persediaan.
- **COGS**: Akun Harga Pokok Penjualan.
- **Tax Accounts**: Akun untuk PPN Masukan, PPN Keluaran, PPh 21, PPh 23.

> **Penting**: Pastikan akun-akun ini sudah diatur dengan benar agar jurnal otomatis terbentuk sesuai standar akuntansi.

## 4. Tutup Buku (Period Closing)

Fitur untuk menutup periode akuntansi (Bulanan/Tahunan).

### Fungsi Tutup Buku:
1. **Reset Akun Laba/Rugi**: Pendapatan dan Beban akan di-nol-kan.
2. **Transfer ke Laba Ditahan**: Selisih Pendapatan dan Beban (Laba Bersih) akan dipindahkan ke akun **Retained Earnings**.
3. **Kunci Periode**: Transaksi pada periode yang sudah ditutup tidak dapat diedit atau dihapus lagi.

### Langkah Melakukan Tutup Buku:
1. Pilih **Start Date** dan **End Date** periode yang akan ditutup.
2. Klik **Preview Closing** untuk melihat simulasi jurnal penutup.
3. Jika sudah sesuai, konfirmasi untuk melakukan tutup buku permanen.

> **PERINGATAN**: Tindakan tutup buku bersifat **PERMANEN** dan tidak dapat dibatalkan. Pastikan semua transaksi pada periode tersebut sudah lengkap dan benar.

---

# 3. Manajemen Kontak (Contact Management)

Dokumen ini menjelaskan cara menggunakan modul Manajemen Kontak (Contact Master) pada aplikasi Akuntansi. Modul ini digunakan untuk mengelola data pelanggan (Customer), pemasok (Vendor), dan karyawan (Employee).

## 1. Halaman Daftar Kontak (Contact List)

Halaman ini menampilkan seluruh kontak yang terdaftar dalam sistem, dikelompokkan berdasarkan jenisnya.

### Fitur Utama:
- **Pengelompokan Otomatis**: Kontak ditampilkan dalam grup:
  - **Customers**: Pelanggan.
  - **Vendors**: Pemasok/Supplier.
  - **Employees**: Karyawan.
- **Tabel Kontak**:
  - Menampilkan Nama, External ID, PIC Name (untuk Customer/Vendor), Email, Telepon, Alamat, dan Status.
- **Tombol Aksi**: View, Edit, dan Delete pada setiap baris kontak.

## 2. Menambah Kontak Baru (Add Contact)

Klik tombol **"Add Contact"** di pojok kanan atas untuk menambah data baru.

### Form Data Kontak:
1. **Informasi Utama**:
   - **Name**: Nama lengkap kontak/perusahaan (Wajib).
   - **Type**: Jenis kontak (Customer, Vendor, atau Employee).
   - **ID**: ID Eksternal (misal: Kode Pelanggan, NIK Karyawan).
   - **PIC Name**: Nama Penanggung Jawab (Hanya muncul untuk Customer/Vendor).

2. **Informasi Kontak**:
   - **Email**: Alamat email (Wajib).
   - **Phone**: Nomor telepon kantor/rumah (Wajib).
   - **Mobile**: Nomor handphone (Opsional).
   - **Address**: Alamat lengkap.

3. **Lainnya**:
   - **Notes**: Catatan tambahan.
   - **Active Status**: Switch untuk mengaktifkan/menonaktifkan kontak.

## 3. Mengelola Kontak

### A. Melihat Detail (View)
Klik tombol **View** (ikon mata) untuk melihat informasi lengkap kontak dalam mode baca-saja (read-only).

### B. Mengedit Kontak (Edit)
Klik tombol **Edit** (ikon pensil) untuk mengubah data kontak.
- Berguna untuk memperbarui alamat, nomor telepon, atau status aktif.

### C. Menghapus Kontak (Delete)
Klik tombol **Delete** (ikon sampah) untuk menghapus kontak.
- *Perhatian: Sistem akan meminta konfirmasi sebelum menghapus.*

---

# 4. Produk & Inventaris (Product & Inventory)

Dokumen ini menjelaskan cara menggunakan modul Produk (Product) pada aplikasi Akuntansi. Modul ini digunakan untuk mengelola data master barang, jasa, kategori, unit, dan lokasi gudang.

## 1. Halaman Katalog Produk (Product Catalog)

Halaman ini menampilkan daftar seluruh produk dan jasa yang terdaftar dalam sistem.

### Fitur Utama:
- **Pencarian & Filter**:
  - **Search**: Cari produk berdasarkan nama atau kode.
  - **Filter Kategori**: Tampilkan produk dalam kategori tertentu.
  - **Filter Lokasi**: Tampilkan produk di lokasi gudang tertentu.
  - **Filter Status**: Tampilkan produk Aktif atau Tidak Aktif.
  - **Sort**: Urutkan berdasarkan Nama, Kode, Kategori, Stok, atau Harga.
- **Manajemen Data Master**: Tombol cepat untuk mengelola Kategori, Unit, dan Lokasi Gudang.

## 2. Menambah Produk Baru (Add Product)

Untuk menambahkan produk baru, klik tombol **"Add Product"**.

### Form Data Produk:
1. **Informasi Dasar**:
   - **Kode Produk**: Kode unik (SKU) untuk identifikasi.
   - **Nama Produk**: Nama lengkap produk.
   - **Deskripsi**: Penjelasan detail produk.
   - **Kategori**: Kelompok produk (misal: Elektronik, Jasa, Bahan Baku).
   - **Lokasi Gudang**: Lokasi fisik penyimpanan barang.
   - **Unit**: Satuan hitung (Pcs, Kg, Box, dll).

2. **Detail Produk**:
   - **Merek & Model**: Identitas brand produk.
   - **Barcode & SKU**: Kode scan dan stock keeping unit.
   - **Berat & Dimensi**: Data fisik untuk pengiriman.

3. **Harga (Pricing)**:
   - **Harga Beli**: Harga pembelian dari supplier.
   - **Harga Pokok (COGS)**: Harga modal dasar.
   - **Harga Jual**: Harga jual ke pelanggan.
   - **Tingkat Harga**: Kategori harga (Standard, Premium, Wholesale, dll).

4. **Inventaris (Inventory)**:
   - **Stok Saat Ini**: Jumlah stok awal.
   - **Stok Minimum**: Batas bawah untuk peringatan restock.
   - **Stok Maksimum**: Batas atas kapasitas penyimpanan.
   - **Reorder Level**: Titik jumlah stok dimana pemesanan ulang harus dilakukan.

5. **Gambar**:
   - Upload gambar produk untuk memudahkan identifikasi visual.

6. **Pengaturan**:
   - **Aktif**: Status produk (dijual/tidak).
   - **Produk Jasa**: Centang jika ini adalah jasa (tidak punya stok fisik).
   - **Kena Pajak**: Apakah produk ini objek pajak (PPN).

## 3. Mengelola Produk

Pada setiap baris produk di tabel, terdapat tombol aksi:

- **View Details** (Ikon Mata):
  - Melihat informasi lengkap produk secara detail.
  - Menampilkan gambar, info dasar, harga, dan status stok dalam satu tampilan ringkas.
  
- **Edit** (Ikon Pensil):
  - Mengubah data produk yang sudah ada.
  
- **Delete** (Ikon Sampah):
  - Menghapus produk dari sistem.
  - *Catatan: Produk yang sudah memiliki transaksi mungkin tidak dapat dihapus untuk menjaga integritas data.*

- **Upload Image** (Ikon Upload):
  - Mengganti atau mengupload gambar produk secara cepat tanpa masuk ke menu edit.

## 4. Manajemen Data Master

Klik tombol **"Manage Categories, Units & Locations"** untuk membuka menu pengelolaan data pendukung.

### A. Categories (Kategori)
Mengelompokkan produk untuk kemudahan pelaporan dan pencarian.
- **Fitur**: Tambah, Edit, Hapus Kategori.
- **Data**: Kode, Nama, Deskripsi, Parent Category (untuk sub-kategori).

### B. Units (Satuan)
Satuan pengukuran untuk produk.
- **Fitur**: Tambah, Edit, Hapus Unit.
- **Data**: Kode (pcs), Nama (Pieces), Simbol, Tipe.

### C. Warehouse Locations (Lokasi Gudang)
Daftar lokasi fisik penyimpanan barang.
- **Fitur**: Tambah, Edit, Hapus Lokasi.
- **Data**: Kode, Nama Gudang, Alamat, Deskripsi.

---

# 5. Manajemen Penjualan (Sales Management)

Dokumen ini menjelaskan cara menggunakan modul Manajemen Penjualan (Sales Management) pada aplikasi Akuntansi. Modul ini digunakan untuk mencatat transaksi penjualan, membuat invoice, dan mengelola pembayaran pelanggan.

## 1. Halaman Daftar Penjualan (Sales List)

Halaman ini menampilkan seluruh riwayat transaksi penjualan.

### Fitur Utama:
- **Ringkasan Statistik**: Menampilkan total penjualan, jumlah invoice belum lunas, dan metrik penting lainnya.
- **Pencarian & Filter**:
  - **Search**: Cari berdasarkan nomor invoice, nama customer, atau referensi.
  - **Filter Status**: Tampilkan berdasarkan status (Draft, Invoiced, Paid, Overdue, Cancelled).
  - **Filter Tanggal**: Tampilkan transaksi dalam rentang tanggal tertentu.
- **Tabel Penjualan**:
  - Menampilkan Kode, Customer, Tanggal, Total, Sisa Tagihan (Outstanding), dan Status.
  - **Tombol Aksi**: View, Edit, Create Invoice, Record Payment, Cancel, Download Invoice, Create Receipt.
- **Export Data**: Unduh laporan penjualan ke format PDF atau CSV.

## 2. Membuat Penjualan Baru (Create Sale)

Klik tombol **"Create Sale"** untuk membuat transaksi baru.

### Form Data Penjualan:
1. **Informasi Pelanggan**:
   - **Customer**: Pilih pelanggan dari database.
   - **Sales Person**: Karyawan yang menangani penjualan (opsional).
   - **Invoice Type**: Jenis dokumen (Invoice, Quotation, Sales Order).

2. **Detail Transaksi**:
   - **Date**: Tanggal transaksi.
   - **Payment Terms**: Termin pembayaran (COD, Net 15, Net 30, dll).
   - **Due Date**: Tanggal jatuh tempo (otomatis dihitung dari termin).
   - **Currency**: Mata uang transaksi (default IDR).

3. **Item Penjualan**:
   - **Product**: Pilih produk/jasa. Deskripsi dan harga akan terisi otomatis.
   - **Quantity**: Jumlah barang.
   - **Unit Price**: Harga satuan.
   - **Discount**: Diskon per item (%).
   - **Taxable**: Centang jika kena pajak (PPN).

4. **Informasi Tambahan**:
   - **Shipping**: Metode dan biaya pengiriman.
   - **Notes**: Catatan untuk pelanggan (muncul di invoice).
   - **Internal Notes**: Catatan internal perusahaan.

5. **Ringkasan Keuangan**:
   - Subtotal, Diskon Global, PPN (11%), PPh (jika ada), Biaya Kirim, dan Grand Total.

## 3. Proses Penjualan (Workflow)

Status transaksi penjualan mengikuti alur berikut:

1. **DRAFT**: Status awal saat transaksi dibuat.
   - Bisa diedit sepenuhnya.
   - Belum menjurnal ke akuntansi.
   - **Aksi**: Edit, Delete, **Create Invoice**.

2. **INVOICED**: Transaksi telah disetujui dan menjadi piutang.
   - Jurnal akuntansi terbentuk (Piutang Usaha pada Pendapatan).
   - Tidak bisa diedit sembarangan (harus dibatalkan jika salah).
   - **Aksi**: **Record Payment**, Download Invoice, Cancel Sale.

3. **PAID**: Pembayaran telah lunas.
   - Saldo piutang menjadi nol.
   - **Aksi**: **Create Receipt** (Bukti Pembayaran), View Details.

4. **CANCELLED**: Transaksi dibatalkan.
   - Jurnal pembalik otomatis dibuat.

## 4. Mencatat Pembayaran (Record Payment)

Untuk mencatat pembayaran dari pelanggan:
1. Cari transaksi dengan status **INVOICED** (atau yang memiliki *Outstanding Amount*).
2. Klik tombol aksi (titik tiga) -> pilih **Record Payment**.
3. Isi form pembayaran:
   - **Payment Date**: Tanggal terima pembayaran.
   - **Amount**: Jumlah yang dibayar (bisa sebagian/partial).
     - *Gunakan tombol cepat (25%, 50%, Full) untuk kemudahan.*
   - **Payment Method**: Transfer Bank, Tunai, Cek, dll.
   - **Account**: Pilih akun Kas/Bank penerima dana.
   - **Reference**: Nomor referensi bukti transfer.
4. Klik **Record Payment**. Status akan berubah menjadi **PAID** jika lunas, atau tetap **INVOICED** (dengan sisa tagihan berkurang) jika bayar sebagian.

## 5. Dokumen & Laporan

- **Download Invoice**: Unduh faktur penjualan (PDF) untuk dikirim ke pelanggan.
- **Create Receipt**: Unduh bukti pembayaran (Kuitansi) setelah lunas.
- **Export Report**: Unduh rekap penjualan (PDF/CSV) dari tombol di pojok kanan atas halaman utama.

---

# 6. Pembelian & Pengadaan (Purchase & Procurement)

Dokumen ini menjelaskan cara menggunakan modul Pembelian (Purchase) pada aplikasi Akuntansi. Modul ini mencakup manajemen pembelian, penerimaan barang (Goods Receipt), dan pembayaran pembelian.

## 1. Halaman Utama Purchase

Halaman ini menampilkan daftar seluruh transaksi pembelian yang telah dibuat.

### Fitur Utama:
- **Filter & Pencarian**:
  - **Search**: Cari berdasarkan nomor purchase, nama vendor, atau referensi.
  - **Filter Status**: Tampilkan purchase berdasarkan status (Draft, Pending Approval, Approved, Completed, Cancelled).
  - **Filter Tanggal**: Tampilkan purchase dalam rentang tanggal tertentu.
- **Statistik Ringkas**: Menampilkan total pembelian, jumlah yang belum dibayar (outstanding), dan status persetujuan.
- **Tabel Daftar Purchase**:
  - **Date**: Tanggal transaksi.
  - **Number**: Nomor referensi purchase (misal: PO-2023-001).
  - **Vendor**: Nama pemasok.
  - **Amount**: Total nilai pembelian.
  - **Status**: Status transaksi saat ini.
  - **Approval**: Status persetujuan dari direktur/manajer.
  - **Payment**: Status pembayaran (Draft, Pending, Complete).

## 2. Membuat Purchase Baru (Create Purchase)

Untuk membuat transaksi pembelian baru, klik tombol **"Create Purchase"** di pojok kanan atas.

### Langkah-langkah:
1. **Informasi Vendor**:
   - Pilih **Vendor** dari daftar yang tersedia.
   - Jika vendor belum ada, klik ikon **(+)** di sebelah dropdown untuk menambahkan vendor baru secara langsung (lihat bagian *Menambah Vendor Baru*).
   
2. **Detail Transaksi**:
   - **Date**: Tanggal transaksi pembelian.
   - **Due Date**: Tanggal jatuh tempo pembayaran.
   - **Reference**: Nomor referensi internal (opsional).

3. **Item Pembelian**:
   - Klik **"Add Item"** untuk menambahkan barang/jasa yang dibeli.
   - Pilih **Product** dari daftar.
   - Jika produk belum ada, klik ikon **(+)** untuk menambahkan produk baru (lihat bagian *Menambah Produk Baru*).
   - Masukkan **Quantity** (jumlah) dan **Unit Price** (harga satuan).
   - Sistem akan otomatis menghitung subtotal.
   - Anda dapat menambahkan diskon atau pajak per item jika diperlukan.

4. **Informasi Pembayaran & Akun**:
   - **Payment Method**: Pilih metode pembayaran (Cash, Bank Transfer, Credit/Utang).
   - **Bank/Cash Account**: Pilih akun kas/bank sumber dana (jika tunai) atau akun kewajiban (jika kredit).
   - **Tax & Discount**: Tambahkan pajak (PPN) atau diskon global jika berlaku untuk seluruh transaksi.

5. **Simpan**:
   - Klik **"Create Purchase"** untuk menyimpan transaksi sebagai **Draft** atau langsung mengajukan persetujuan tergantung konfigurasi.

## 3. Fitur Tambahan (Add New)

### Menambah Vendor Baru
Dapat diakses saat membuat purchase baru:
- Klik ikon **(+)** di sebelah pilihan Vendor.
- Isi form **Add New Vendor**:
  - **Name**: Nama vendor (Wajib).
  - **Email/Phone**: Kontak vendor.
  - **Address**: Alamat vendor.
- Klik **"Create Vendor"** untuk menyimpan.

### Menambah Produk Baru
Dapat diakses saat menambah item purchase:
- Klik ikon **(+)** di sebelah pilihan Product.
- Isi form **Add New Product**:
  - **Name**: Nama produk (Wajib).
  - **Unit**: Satuan (pcs, kg, box, dll).
  - **Purchase Price**: Harga beli standar.
  - **Sale Price**: Harga jual standar.
  - **Expense Account**: Akun beban yang terkait (opsional).
- Klik **"Create Product"** untuk menyimpan.

## 4. Manajemen Purchase

Setelah purchase dibuat, Anda dapat melakukan beberapa aksi melalui tombol aksi di sebelah kanan setiap baris tabel:

- **View Details** (Ikon Mata): Melihat detail lengkap purchase.
- **Submit for Approval**: Mengajukan purchase untuk disetujui oleh atasan (jika status masih Draft).
- **Delete** (Ikon Sampah): Menghapus purchase (hanya jika status masih Draft).
- **Print/Download**: Mengunduh invoice atau purchase order dalam format PDF.

## 5. Penerimaan Barang (Goods Receipt)

Setelah purchase disetujui (Approved), Anda dapat mencatat penerimaan barang.

1. Klik tombol aksi pada purchase yang berstatus **Approved**.
2. Pilih **"Create Receipt"**.
3. Isi form penerimaan:
   - **Received Date**: Tanggal barang diterima.
   - **Items**: Masukkan jumlah yang diterima (Received Qty). Bisa sebagian (Partial) atau seluruhnya.
   - **Condition**: Kondisi barang (Good, Damaged).
   - **Create Asset**: Centang jika barang ini adalah aset tetap (Fixed Asset) untuk otomatis membuat data aset.
4. Klik **"Create Receipt"**.
   - Status purchase akan berubah menjadi **Received** atau **Partial Received**.
   - Stok barang akan bertambah otomatis.

## 6. Pencatatan Pembayaran (Record Payment)

Untuk mencatat pembayaran atas purchase (terutama yang kredit):

1. Klik tombol aksi pada purchase yang belum lunas.
2. Pilih **"Record Payment"**.
3. Isi form pembayaran:
   - **Payment Date**: Tanggal pembayaran.
   - **Payment Method**: Transfer, Cash, atau Cek.
   - **Account**: Pilih akun kas/bank yang digunakan.
   - **Amount**: Jumlah yang dibayarkan (bisa sebagian/cicil).
     - Gunakan tombol cepat (25%, 50%, Full) untuk pengisian otomatis.
4. Klik **"Record Payment"**.
   - Status pembayaran akan diperbarui.
   - Saldo kas/bank akan berkurang dan utang usaha akan berkurang.

## 7. Jurnal Akuntansi (Journal Entries)

Setiap transaksi purchase otomatis menghasilkan jurnal akuntansi.
- Klik tombol **"View Journal"** pada detail purchase untuk melihat debit/kredit yang terbentuk.
- **Contoh Jurnal Pembelian Kredit**:
  - (Dr) Inventory / Expense
  - (Cr) Accounts Payable (Utang Usaha)
- **Contoh Jurnal Pembayaran**:
  - (Dr) Accounts Payable
  - (Cr) Cash/Bank

---

# 7. Pembayaran (Payments)

Modul **Payments** digunakan untuk mengelola seluruh transaksi pembayaran, baik pembayaran masuk (Receivable/Piutang) dari pelanggan maupun pembayaran keluar (Payable/Hutang) kepada vendor.

## 1. Dashboard Pembayaran

Halaman utama Payments menampilkan ringkasan dan daftar transaksi pembayaran.

### Ringkasan (Summary Cards)
Di bagian atas halaman, Anda dapat melihat statistik cepat:
- **Total Payments**: Jumlah total transaksi pembayaran dalam periode ini.
- **Total Amount**: Total nilai nominal pembayaran.
- **Completed**: Nilai pembayaran yang statusnya sudah selesai.
- **Avg Payment Value**: Rata-rata nilai per transaksi.

### Daftar Pembayaran
Tabel utama menampilkan informasi berikut:
- **Code**: Kode unik pembayaran (misal: `PAY-2023-001`, `RCV-2023-001`).
- **Contact**: Nama Customer (untuk pembayaran masuk) atau Vendor (untuk pembayaran keluar).
- **Date**: Tanggal pembayaran.
- **Amount**: Jumlah pembayaran.
- **Method**: Metode pembayaran (Cash, Bank Transfer, Check, dll).
- **Status**: Status pembayaran (Pending, Completed, Failed).
- **Actions**: Menu untuk melihat detail, edit, atau hapus.

## 2. Membuat Pembayaran Baru

Klik tombol **+ Create Payment** di pojok kanan atas. Akan muncul menu dropdown dengan 4 opsi:

### a. Receivable Payment (Pembayaran Masuk)
Digunakan untuk mencatat penerimaan pembayaran dari Customer (Pelunasan Invoice).
1. Pilih **Receivable Payment**.
2. Isi form:
    - **Contact**: Pilih Customer.
    - **Cash/Bank Account**: Akun penerima dana.
    - **Amount**: Jumlah yang diterima.
    - **Allocation**: Alokasikan dana ke Invoice yang belum lunas (Auto/Manual).

### b. Payable Payment (Pembayaran Keluar)
Digunakan untuk mencatat pembayaran hutang ke Vendor (Pelunasan Bill).
1. Pilih **Payable Payment**.
2. Isi form:
    - **Contact**: Pilih Vendor.
    - **Cash/Bank Account**: Akun sumber dana.
    - **Amount**: Jumlah yang dibayar.
    - **Allocation**: Alokasikan dana ke Bill yang belum lunas.

### c. Setor PPN (Tax Remittance)
Digunakan untuk menyetor PPN Terutang (Selisih PPN Keluaran - PPN Masukan) ke Negara.
1. Pilih **Setor PPN**.
2. Modal akan menampilkan **Perhitungan PPN**:
    - **PPN Keluaran**: Total PPN dari penjualan.
    - **PPN Masukan**: Total PPN dari pembelian.
    - **PPN Terutang**: Selisih yang harus dibayar (Kurang Bayar) atau dikompensasi (Lebih Bayar).
3. Isi form:
    - **Tanggal Pembayaran**: Tanggal setor.
    - **Jumlah Pembayaran**: Otomatis terisi sesuai PPN Terutang (bisa diubah).
    - **Akun Kas/Bank**: Sumber dana pembayaran.
4. Klik **Bayar PPN**. Sistem akan membuat jurnal otomatis (Debit PPN Keluaran, Kredit PPN Masukan, Kredit Kas/Bank).

### d. Expense Payment (Pembayaran Biaya)
Digunakan untuk mencatat pengeluaran biaya operasional langsung (tanpa melalui Bill) atau pembayaran kewajiban lainnya.
1. Pilih **Expense Payment**.
2. Isi form:
    - **Contact**: Vendor/Pihak penerima (opsional).
    - **Expense/Liability Account**: Pilih akun Biaya (Expense) atau Kewajiban (Liability) dari COA.
    - **Payment From**: Akun Kas/Bank sumber dana.
    - **Date**: Tanggal transaksi.
    - **Amount**: Jumlah biaya.
    - **Description**: Keterangan biaya.
3. Klik **Create Payment**.

### Alokasi Pembayaran (Khusus Receivable & Payable)
Fitur penting dalam pembayaran hutang/piutang adalah **Alokasi**. Sistem memungkinkan Anda untuk mengalokasikan pembayaran ke satu atau lebih Invoice/Bill yang belum lunas (Outstanding).

- **Auto Allocate**: Jika diaktifkan, sistem otomatis mengalokasikan dana ke tagihan terlama.
- **Manual Allocation**: Pada tabel "Allocation", Anda dapat memasukkan nominal secara manual pada kolom "Allocate" untuk setiap Invoice/Bill yang ingin dibayar.
- **Sisa Alokasi**: Pastikan "Remaining Amount" adalah 0 agar seluruh dana teralokasi dengan benar.

## 3. Detail & Aksi Pembayaran

Klik tombol titik tiga (Actions) pada baris pembayaran untuk opsi lanjutan:

- **View Details**: Melihat detail lengkap pembayaran, termasuk jurnal akuntansi yang terbentuk dan riwayat alokasi.
- **Edit**: Mengubah data pembayaran (Hanya bisa dilakukan jika periode akuntansi belum ditutup).
- **Export PDF**: Mengunduh bukti pembayaran dalam format PDF.
- **Delete**: Menghapus data pembayaran (Hati-hati: Ini akan membatalkan jurnal dan mengembalikan status Invoice/Bill menjadi belum lunas).

## 4. Filter & Laporan

### Filter Data
Gunakan fitur filter di bagian atas untuk mencari pembayaran spesifik:
- **Status**: Filter berdasarkan status (All, Completed, Pending).
- **Method**: Filter berdasarkan metode pembayaran.
- **Date Range**: Filter berdasarkan rentang tanggal transaksi.
- **Search**: Cari berdasarkan Kode Pembayaran, Nama Kontak, atau Referensi.

### Export Laporan
- **Export Report**: Klik tombol ini untuk mengunduh laporan daftar pembayaran dalam format Excel atau PDF sesuai filter yang diterapkan.

---

# 8. Kas & Bank (Cash & Bank)

Modul **Cash & Bank** digunakan untuk mengelola seluruh akun kas tunai dan rekening bank perusahaan, serta mencatat transaksi penerimaan, pengeluaran, dan transfer dana.

## 1. Daftar Akun (Account List)

Halaman utama menampilkan daftar seluruh akun kas dan bank yang terdaftar.

### Informasi yang Ditampilkan:
- **Account Code**: Kode akun unik (sesuai Chart of Accounts).
- **Account Name**: Nama akun (misal: "Kas Besar", "Bank BCA").
- **Type**: Jenis akun (**CASH** untuk uang tunai, **BANK** untuk rekening bank).
- **GL Account**: Status integrasi dengan Chart of Accounts (COA).
    - ✅ **Integrated**: Terhubung dengan akun GL.
    - ⚠️ **Unlinked**: Belum terhubung (perlu diatur).
- **Bank Details**: Informasi detail bank (Nama Bank, No. Rekening) untuk tipe BANK.
- **Current Balance**: Saldo saat ini.
    - Warna **Hijau**: Saldo Positif.
    - Warna **Merah**: Saldo Negatif (Overdraft).
- **Status**: **ACTIVE** atau **INACTIVE**.

## 2. Membuat Akun Baru (Create Account)

1. Klik tombol **+ Add Accounts** di pojok kanan atas.
2. Pilih **Account Type**:
    - **Cash Account**: Untuk uang tunai fisik (Petty Cash, Kas Besar).
    - **Bank Account**: Untuk rekening bank.
3. Isi **Basic Information**:
    - **Account Name**: Nama identifikasi akun.
    - **Currency**: Mata uang (Otomatis IDR).
    - **Description**: Catatan tambahan (opsional).
4. Isi **Bank Details** (Khusus tipe BANK):
    - **Bank Name**: Nama Bank (wajib).
    - **Account Number**: Nomor rekening.
    - **Atas Nama**: Nama pemegang rekening.
    - **Branch**: Cabang bank.
5. **Initial Setup** (Hanya saat pembuatan):
    - **Opening Balance**: Saldo awal akun.
    - **Opening Date**: Tanggal saldo awal.
6. **COA Integration**:
    - Pilih akun GL dari **Chart of Accounts** yang akan dipetakan ke akun ini.
    - *Catatan*: Anda harus membuat akun tipe **Asset** (Current Asset) di modul Chart of Accounts terlebih dahulu jika belum ada.

## 3. Transaksi (Transactions)

Setiap akun memiliki menu aksi (tombol titik tiga atau ikon cepat) untuk melakukan transaksi.

### a. Setor Dana (Make Deposit)
Digunakan untuk mencatat penerimaan uang ke dalam akun (misal: Setoran Modal).
1. Klik ikon **Make Deposit** (panah naik hijau).
2. Isi form:
    - **Transaction Date**: Tanggal transaksi.
    - **Amount**: Jumlah uang yang diterima.
    - **Reference**: Nomor referensi/bukti transaksi.
    - **Notes**: Catatan.
    - **Credit Account (Equity Source)**: Pilih akun Modal/Ekuitas yang menjadi sumber dana.
3. Sistem akan menampilkan **Double-Entry Preview** (Debit: Kas/Bank, Kredit: Modal).
4. Klik **Process Deposit**.

### b. Tarik Dana (Make Withdrawal)
Digunakan khusus untuk mencatat **pengambilan pemilik/Prive** dari akun kas atau bank.
1. Klik ikon **Make Withdrawal** (panah turun oranye).
2. Isi form:
    - **Transaction Date**: Tanggal transaksi.
    - **Amount**: Jumlah uang yang dikeluarkan.
    - **Reference**: Nomor referensi.
    - **Notes**: Alasan pengambilan.
    - **Owner Drawing / Prive Account**: Pilih akun Prive yang telah disetujui.
3. Hanya Finance Manager atau Direktur yang dapat memposting transaksi ini.
4. Klik **Process Withdrawal**.

Untuk biaya, utang vendor, pajak/payroll, atau pembelian aset, gunakan modul Pembayaran, Purchases, Pajak, atau Payroll yang sesuai. Untuk perpindahan antar rekening internal gunakan Transfer Dana.

### c. Transfer Dana (Transfer Funds)
Digunakan untuk memindahkan dana antar akun internal (misal: dari Bank ke Kas Kecil).
1. Klik ikon **Transfer Funds** (panah kanan biru).
2. Isi form:
    - **To (Destination)**: Pilih akun tujuan transfer.
    - **Date**: Tanggal transfer.
    - **Amount**: Jumlah yang ditransfer.
    - **Reference**: Referensi.
    - **Notes**: Catatan.
3. Klik **Transfer Funds**.

## 4. Riwayat Transaksi (Transaction History)

Untuk melihat mutasi rekening:
1. Klik ikon **View Details** (mata) pada akun yang diinginkan.
2. Modal **Transaction History** akan muncul menampilkan:
    - Ringkasan Saldo & Total Transaksi.
    - Tabel mutasi (Tanggal, Tipe, Referensi, Jumlah, Saldo Akhir).
3. **Filter**: Anda dapat menyaring transaksi berdasarkan Tanggal Mulai dan Tanggal Akhir.

## 5. Rekonsiliasi (Reconciliation)

Fitur untuk mencocokkan saldo di sistem dengan rekening koran bank.
- Klik menu opsi pada akun -> **Reconcile**.
- Anda akan diarahkan ke halaman Rekonsiliasi Bank (modul terpisah) atau melihat status rekonsiliasi terakhir.

---

# 9. Aset Tetap (Fixed Assets)

Dokumen ini menjelaskan cara menggunakan modul Aset Tetap (Fixed Assets) pada aplikasi Akuntansi. Modul ini digunakan untuk mencatat, melacak, dan mengelola penyusutan aset perusahaan.

## 1. Halaman Daftar Aset (Assets List)

Halaman ini menampilkan seluruh aset tetap yang dimiliki perusahaan beserta status dan nilai bukunya.

### Fitur Utama:
- **Ringkasan Aset**: Menampilkan total nilai aset, total penyusutan, dan nilai buku bersih saat ini.
- **Pencarian & Filter**:
  - **Search**: Cari aset berdasarkan nama, kode, serial number, atau lokasi.
  - **Filter Kategori**: Tampilkan aset berdasarkan kategori (Office Equipment, Vehicle, Building, dll).
  - **Filter Status**: Tampilkan aset Aktif, Terjual, atau Tidak Aktif.
- **Tabel Aset**:
  - Menampilkan Kode, Nama, Kategori, Harga Perolehan, Nilai Buku, Status, dan Lokasi.
  - **Tombol Aksi**: View, Edit, Delete, dan Upload Image.
- **Export Data**: Unduh data aset ke format CSV.

## 2. Menambah Aset Baru (Add Asset)

Untuk mencatat aset baru, klik tombol **"Add Asset"**.

### Form Data Aset:
1. **Informasi Dasar (Basic Information)**:
   - **Asset Name**: Nama aset (wajib).
   - **Category**: Kategori aset (menentukan akun akuntansi default).
   - **Serial Number**: Nomor seri unik aset.
   - **Status**: Kondisi status aset (Active, Inactive, Sold).
   - **Condition**: Kondisi fisik (Excellent, Good, Fair, Poor).

2. **Informasi Keuangan (Financial Information)**:
   - **Purchase Date**: Tanggal pembelian/perolehan.
   - **Purchase Price**: Harga beli aset.
   - **Salvage Value**: Estimasi nilai sisa di akhir masa manfaat.
   - **Useful Life**: Masa manfaat ekonomis dalam tahun.
   - **Depreciation Method**: Metode penyusutan (Straight Line / Garis Lurus atau Declining Balance / Saldo Menurun).
   - *Sistem akan otomatis menghitung estimasi nilai buku saat ini berdasarkan data di atas.*

3. **Informasi Lokasi (Location Information)**:
   - **Location Description**: Deskripsi lokasi (misal: Lantai 2, Ruang Meeting).
   - **GPS Coordinates**: Koordinat peta (opsional).
   - **Map Picker**: Fitur untuk memilih lokasi titik koordinat secara visual melalui peta.

4. **Gambar Aset**:
   - Upload foto aset untuk dokumentasi visual (dapat dilakukan setelah aset disimpan).

## 3. Mengelola Aset

### A. Melihat Detail Aset (View Details)
Klik tombol **View** (ikon mata) untuk melihat informasi lengkap aset, termasuk:
- Detail lengkap aset.
- Foto aset.
- Peta lokasi (jika koordinat diisi).
- **Kalkulator Depresiasi**: Fitur untuk menghitung nilai buku dan akumulasi penyusutan per tanggal tertentu secara *real-time*.

### B. Mengedit Aset (Edit)
Klik tombol **Edit** (ikon pensil) untuk mengubah data aset.
- Berguna untuk update lokasi, kondisi, atau koreksi data keuangan.

### C. Upload Gambar
Klik tombol **Upload** atau ikon gambar pada tabel untuk mengunggah foto aset.

### D. Menghapus Aset
Klik tombol **Delete** (ikon sampah) untuk menghapus data aset.
- *Perhatian: Penghapusan bersifat permanen.*

## 4. Perhitungan Penyusutan (Depreciation)

Sistem mendukung dua metode penyusutan utama:

1. **Straight Line (Garis Lurus)**:
   - Penyusutan merata setiap tahun selama masa manfaat.
   - Rumus: `(Harga Perolehan - Nilai Sisa) / Masa Manfaat`.

2. **Declining Balance (Saldo Menurun)**:
   - Penyusutan lebih besar di tahun-tahun awal.
   - Menggunakan tarif penyusutan ganda dari metode garis lurus.

Nilai buku aset akan berkurang secara otomatis seiring berjalannya waktu berdasarkan metode yang dipilih.

