# User Guide Operasional ERP Akuntansi

Dokumen ini dibuat untuk pengguna awam agar bisa menjalankan aplikasi dari tahap setup awal sampai operasional harian.

## 1. Tujuan Panduan

Panduan ini membantu tim memahami:
- urutan setup sistem yang benar
- cara memakai setiap modul utama
- hubungan antar modul agar transaksi tidak terputus
- titik approval, jurnal otomatis, dan laporan yang dihasilkan

## 2. Analisa Singkat Sistem

Secara fungsi, aplikasi ini adalah ERP akuntansi terintegrasi. Artinya, data yang diinput di satu modul akan memengaruhi modul lain.

Karakter utama sistem ini:
- `Settings` dipakai untuk konfigurasi perusahaan, pajak, format, dan akun-akun dasar.
- `Cash & Bank` menjadi pusat rekening kas dan bank yang dipakai hampir di semua transaksi pembayaran.
- `Products`, `Target Assembly`, `BOM Template`, dan `Project` membentuk alur kebutuhan material dan perencanaan pekerjaan.
- `Purchase Request` dipakai untuk permintaan pembelian internal.
- `Purchases` dipakai untuk realisasi pembelian barang, aset tetap, dan prepaid expense.
- `Assets` dipakai untuk memonitor aset tetap dan depresiasi. Input manual di sini terutama untuk aset lama saat awal go-live.
- `Prepaid Expenses` dipakai untuk memonitor biaya dibayar dimuka dan menjalankan amortisasi berkala.
- `Sales` dipakai untuk penjualan dan pembuatan invoice.
- `Payments` dipakai untuk menerima pembayaran piutang, membayar hutang, membayar beban, dan setor PPN.
- `Journal` menjadi pusat pencatatan akuntansi, baik otomatis maupun manual.
- `Reports` menarik data dari transaksi dan jurnal yang sudah berjalan.
- `Notifications` membantu user melihat approval, stok menipis, dan transaksi yang perlu tindak lanjut.

Kesimpulan penting:
- Sistem ini dirancang agar transaksi utama menghasilkan jurnal otomatis.
- Karena itu, user sebaiknya memulai dari modul sumber transaksi, bukan langsung membuat jurnal manual.
- Jurnal manual lebih cocok untuk koreksi, penyesuaian, atau transaksi khusus yang memang tidak punya modul sumber.

## 3. Urutan Kerja yang Disarankan

Untuk implementasi dan penggunaan harian, urutan paling aman adalah:

1. Isi `Settings` dan `Tax Configuration`
2. Buat akun `Cash & Bank`
3. Siapkan master `Products` dan import komponen
4. Buat `Target Assembly`
5. Buat `BOM Template`
6. Buat `Project`
7. Buat `Purchase Request`
8. Lanjutkan ke `Purchases`
9. Pantau `Assets` dan `Prepaid Expenses`
10. Jalankan `Sales` dan `Invoice`
11. Catat transaksi di `Payments`
12. Review `Journal`
13. Kelola `Users`
14. Tarik `Reports`
15. Monitor `Notifications`

## 4. Persiapan Sebelum Mulai

Sebelum user operasional mulai input transaksi, pastikan hal-hal berikut sudah siap:
- daftar akun atau COA sudah tersedia di modul akun
- data customer dan vendor sudah dibuat
- user sudah memiliki role yang sesuai
- periode akuntansi masih terbuka
- akun kas/bank sudah dibuat dan terhubung ke akun buku besar

Catatan:
- Beberapa menu dan tombol bisa berbeda tergantung role user.
- Jika tombol approval atau edit tidak terlihat, biasanya karena izin role belum sesuai.

## 5. Settings dan Tax Configuration

### 5.1. Isi data perusahaan

Masuk ke menu `Settings`, lalu isi:
- nama perusahaan
- alamat
- telepon
- email
- NPWP atau tax number
- logo perusahaan
- tipe perusahaan

Tips:
- Tipe perusahaan penting karena memengaruhi kebutuhan akun tertentu.
- Untuk perusahaan `TRADING` dan `MANUFACTURING`, akun persediaan dan HPP biasanya wajib dipetakan.
- Setelah tipe perusahaan ditetapkan, perlakukan sebagai pengaturan utama dan jangan sering diubah.

### 5.2. Atur konfigurasi sistem

Masih di halaman `Settings`, periksa dan isi:
- format tanggal
- default tax rate
- bahasa sistem
- prefix dokumen seperti invoice, quote, purchase, dan payment
- email notification jika fitur email dipakai
- SMTP host, port, username, dan from email jika diperlukan

Manfaat bagian ini:
- penomoran dokumen menjadi konsisten
- pajak default tidak perlu diketik ulang terus-menerus
- format tampilan sesuai kebiasaan perusahaan

### 5.3. Lanjut ke Tax Configuration / Tax Accounts

Dari `Settings`, buka bagian `Tax Accounts` atau konfigurasi akun pajak.

Akun yang perlu dipetakan minimal:
- piutang usaha penjualan
- akun kas penjualan
- akun bank penjualan
- pendapatan penjualan
- PPN keluaran
- hutang usaha pembelian
- akun kas pembelian
- akun bank pembelian
- PPN masukan
- akun beban pembelian

Akun tambahan yang sering dipakai:
- PPh 21
- PPh 23
- PPh 25
- hutang pajak
- persediaan
- HPP

Panduan praktis:
1. Pastikan kode akun dan nama akun sudah benar.
2. Pilih akun sesuai fungsi bisnisnya, jangan asal pilih akun yang mirip namanya.
3. Untuk perusahaan dagang/manufaktur, pastikan akun `inventory` dan `COGS/HPP` sudah terisi.
4. Simpan setelah semua mapping selesai.

Dampak konfigurasi ini:
- jurnal penjualan akan mengarah ke akun pendapatan, piutang, dan PPN yang benar
- jurnal pembelian akan mengarah ke hutang, beban, persediaan, atau pajak yang benar
- kesalahan mapping di tahap ini akan membuat seluruh laporan ikut salah

### 5.4. Period Closing

Di halaman `Settings` juga tersedia fitur `Period Closing`.

Gunakan fitur ini saat tutup buku periode.

Langkah umumnya:
1. Pilih tanggal awal dan tanggal akhir periode yang ingin ditutup.
2. Jalankan preview lebih dulu.
3. Review hasil preview.
4. Jika sudah benar, lakukan execute closing.
5. Jika dibutuhkan dan user memiliki izin, periode terakhir dapat dibuka kembali lewat fitur reopen.

Saran:
- lakukan closing hanya setelah semua transaksi periode tersebut lengkap
- pastikan depresiasi, amortisasi, pembayaran, dan jurnal penyesuaian sudah diposting

## 6. Cash & Bank

Menu `Cash & Bank` sangat penting karena menjadi sumber dan tujuan dana di banyak modul.

### 6.1. Fungsi utama Cash & Bank

Modul ini dipakai untuk:
- membuat rekening kas dan bank operasional
- melihat saldo, dana dibekukan, dan saldo tersedia
- melakukan deposit
- melakukan withdrawal khusus pengambilan pemilik/Prive
- transfer antar rekening internal
- melihat histori transaksi
- freeze dan unfreeze dana
- menjalankan pengecekan rekonsiliasi

### 6.2. Cara membuat akun Cash atau Bank

Langkah:
1. Masuk ke menu `Cash & Bank`.
2. Klik tambah akun.
3. Pilih jenis akun: `CASH` atau `BANK`.
4. Isi kode dan nama akun.
5. Jika tipe bank, isi detail bank bila diminta.
6. Hubungkan akun tersebut ke akun COA yang sesuai.
7. Simpan.

Praktik terbaik:
- satu akun cash/bank sebaiknya terhubung ke satu akun buku besar khusus
- jangan memakai satu akun COA yang sama untuk beberapa rekening operasional berbeda
- beri nama yang mudah dikenali, misalnya `BCA Operasional`, `Kas Kecil Gudang`, `Mandiri Payroll`

### 6.3. Memahami saldo di layar

Di daftar akun, user biasanya akan melihat:
- `balance`: saldo total
- `frozen amount`: dana yang dibekukan sementara
- `available balance`: saldo yang benar-benar bisa dipakai

Arti sederhananya:
- jika ada dana yang di-freeze, maka available balance akan lebih kecil dari total balance

### 6.4. Cara melakukan Deposit

Gunakan `Deposit` jika ada uang masuk ke akun kas/bank.

Contoh kasus:
- setoran modal owner
- pencairan pinjaman
- pendapatan bunga bank
- koreksi saldo masuk

Langkah:
1. Pilih akun kas/bank tujuan.
2. Klik `Deposit`.
3. Isi tanggal, nominal, dan keterangan.
4. Pilih akun lawan transaksi bila diminta.
5. Simpan.

Tips:
- tulis keterangan yang jelas, misalnya `Setoran modal awal` atau `Bunga giro Maret`
- pastikan rekening tujuan tidak salah pilih

### 6.5. Cara melakukan Withdrawal

Gunakan `Withdrawal` hanya untuk **pengambilan pemilik/Prive** dari akun kas/bank.

Contoh kasus:
- pengambilan dana perusahaan oleh pemilik yang dicatat ke akun Prive

Langkah:
1. Pilih akun.
2. Klik `Withdrawal`.
3. Pilih akun Prive/Pengambilan Pemilik yang telah disetujui.
4. Isi tanggal, nominal, nomor referensi, dan alasan pengambilan.
5. Simpan.

Catatan:
- biaya operasional atau non-operasional → gunakan `Pembayaran` / Pembayaran COA
- utang vendor/PO → gunakan pembayaran PO atau termin di `Purchases`
- pajak dan payroll → gunakan settlement pada modul terkait
- pembelian aset → gunakan `Purchases` dengan tipe item `ASSET`, agar Asset Register dan depresiasi dibuat dengan benar
- perpindahan bank ke kas kecil atau antar rekening internal → gunakan `Transfer`, bukan Withdrawal

### 6.6. Cara transfer antar Cash & Bank

Gunakan `Transfer` untuk memindahkan dana antar rekening internal.

Contoh:
- dari bank operasional ke kas kecil
- dari bank A ke bank B

Langkah:
1. Pilih rekening asal.
2. Klik `Transfer`.
3. Pilih rekening tujuan.
4. Isi nominal dan keterangan.
5. Simpan.

Hasil yang diharapkan:
- saldo rekening asal berkurang
- saldo rekening tujuan bertambah
- histori perpindahan dana tetap terlacak

### 6.7. Freeze dan Unfreeze dana

Gunakan fitur ini jika sejumlah dana belum boleh dipakai.

Contoh:
- dana disisihkan untuk pembayaran vendor besar
- dana disiapkan untuk kewajiban tertentu

Langkah:
1. Pilih akun.
2. Klik `Freeze Funds` untuk membekukan dana.
3. Isi nominal dan alasan.
4. Jika sudah boleh dipakai lagi, gunakan `Unfreeze Funds`.

### 6.8. Transaction History

Gunakan histori transaksi untuk:
- melacak uang masuk dan keluar
- mengecek asal pergerakan saldo
- memudahkan audit internal

### 6.9. Reconciliation Check

Gunakan `Reconciliation Check` atau modul rekonsiliasi jika ingin mencocokkan saldo sistem dengan rekening koran bank.

Cocok dipakai saat:
- akhir minggu
- akhir bulan
- sebelum tutup buku

### 6.10. Catatan penting Cash & Bank

- Akun non-operasional atau holding account biasanya punya pembatasan transaksi langsung.
- Jika tombol deposit/withdrawal tidak aktif, cek jenis akun dan aturan penggunaannya. Withdrawal hanya tersedia untuk pengambilan pemilik/Prive yang disetujui.
- Cash & Bank sangat terintegrasi dengan pembelian, penjualan, dan pembayaran. Pastikan setup-nya rapi sejak awal.

## 7. Products: Import Komponen dan Membuat Target Assembly

Menu `Products` dipakai untuk mengelola barang, bahan baku, jasa, dan produk hasil rakitan.

### 7.1. Siapkan master data pendukung dulu

Di halaman produk biasanya tersedia pengelolaan:
- kategori
- satuan
- lokasi gudang

Sebelum import atau input produk, pastikan ketiga master ini sudah siap.

### 7.2. Cara import komponen atau bahan baku

Gunakan fitur `Import` jika jumlah komponen banyak.

Alur import:
1. Masuk ke menu `Products`.
2. Buka tab `Komponen & Bahan Baku`.
3. Klik `Import`.
4. Upload file CSV atau Excel yang sudah disimpan sebagai CSV.
5. Cocokkan kolom file dengan field sistem.
6. Review hasil mapping.
7. Jalankan import.

Field yang umumnya dipakai saat import:
- nama produk
- kode produk
- kategori
- purchase price
- sale price
- unit
- description

Tips penting:
- gunakan kode unik agar mudah dicari
- pastikan satuan konsisten, misalnya `pcs`, `kg`, `meter`
- cek harga beli standar karena harga ini akan ikut membantu estimasi biaya di BOM dan PR
- jika nama produk sama dengan data lama, sistem dapat memperbarui data yang sudah ada

### 7.3. Cara membuat produk biasa

Gunakan produk biasa untuk:
- bahan baku
- barang dagang
- barang habis pakai
- jasa

Isi data penting:
- kode
- nama
- kategori
- satuan
- lokasi gudang
- purchase price
- cost price
- sale price
- stok minimum

### 7.4. Cara membuat Target Assembly

Target assembly adalah produk hasil rakitan atau produk jadi yang tersusun dari beberapa komponen.

Langkah:
1. Masuk ke menu `Products`.
2. Buat produk baru.
3. Aktifkan opsi `Produk Hasil Rakitan` atau `Target Assembly`.
4. Isi kode dan nama produk jadi.
5. Isi deskripsi jika perlu.
6. Pilih kategori dan satuan yang sesuai.
7. Simpan.

Contoh target assembly:
- panel listrik rakitan
- meja custom
- paket mesin tertentu
- set produk bundling yang diproduksi dari beberapa komponen

Prinsip sederhananya:
- komponen ada di tab bahan baku
- hasil akhirnya dibuat sebagai target assembly
- hubungan antara keduanya diatur di `BOM Template`

## 8. BOM Template

Menu `BOM Template` dipakai untuk menyusun resep produksi atau komposisi komponen dari target assembly.

### 8.1. Fungsi BOM Template

BOM dipakai untuk:
- menentukan komponen apa saja yang dibutuhkan
- menentukan kuantitas setiap komponen
- menghitung estimasi biaya per unit dan total batch
- menjadi dasar kebutuhan material untuk project atau pembelian

### 8.2. Cara membuat BOM Template

Langkah:
1. Masuk ke menu `BOM Template`.
2. Klik `Add New BOM`.
3. Isi `BOM code`.
4. Isi `BOM name`.
5. Pilih `target product` yang sudah dibuat sebagai target assembly.
6. Isi `target quantity`.
7. Tambahkan daftar komponen.
8. Isi kebutuhan qty tiap komponen.
9. Review biaya per unit dan total biaya batch.
10. Simpan.

Informasi yang biasanya muncul:
- target product
- target qty
- component count
- cost per unit
- total batch cost

### 8.3. Menggunakan template komponen

Jika ada komposisi yang sering dipakai berulang, gunakan `Component Template`.

Caranya:
- klik `Use Template` saat membuat BOM
- pilih template komponen yang sudah tersedia
- sistem akan mengisi komponen secara otomatis

### 8.4. Menyimpan komponen sebagai template

Jika komposisi baru kemungkinan sering dipakai lagi:
1. susun dulu komponen pada BOM
2. klik `Save components as template`
3. beri nama template
4. simpan

### 8.5. Mengelola template komponen

Melalui `Manage Templates`, user bisa:
- membuat template komponen baru
- mengedit template lama
- menghapus template yang tidak terpakai

Kapan fitur ini sangat berguna:
- perusahaan sering membuat produk dengan struktur komponen mirip
- ingin mempercepat pembuatan BOM baru
- ingin menjaga standar material

## 9. Project

Menu `Project` dipakai untuk mengelola pekerjaan atau proyek yang punya anggaran, jadwal, dan kebutuhan material sendiri.

### 9.1. Cara membuat project

Langkah:
1. Masuk ke menu `Project`.
2. Klik tambah project.
3. Isi kode project.
4. Isi nama project.
5. Isi deskripsi.
6. Isi tanggal mulai dan tanggal selesai.
7. Isi `total budget`.
8. Simpan.

### 9.2. Memahami budget di project

Perlu dipahami:
- `total budget` adalah target atau pagu manual
- budget ini tidak otomatis dihitung dari BOM
- biaya aktual project akan bertambah dari transaksi pembelian yang di-link ke project tersebut

### 9.3. Menambahkan BOM ke project

Setelah project dibuat, user bisa masuk ke tab `Bill of Materials (BOM)`.

Langkah:
1. buka detail atau edit project
2. masuk ke tab `BOM`
3. pilih BOM template
4. isi jumlah unit yang dibutuhkan
5. simpan

Manfaatnya:
- sistem dapat menghitung estimasi kebutuhan material project
- tim pembelian lebih mudah menentukan apa yang perlu dibeli

### 9.4. Generate PR dari project

Jika stok komponen belum cukup, gunakan fitur `Generate PR (Stock Check)`.

Fungsi fitur ini:
- mengecek stok yang tersedia
- menghitung kekurangan bahan
- membuat draft purchase request berdasarkan shortage

Ini sangat membantu agar pembelian benar-benar mengikuti kebutuhan project.

## 10. Purchase Request

Menu `Purchase Request` dipakai untuk permintaan pembelian internal sebelum dibuat transaksi pembelian resmi.

### 10.1. Kapan Purchase Request dipakai

Gunakan PR jika:
- user operasional meminta pembelian ke bagian purchasing atau finance
- perusahaan membutuhkan approval sebelum pembelian
- kebutuhan pembelian terkait department atau project tertentu

### 10.2. Cara membuat Purchase Request

Langkah:
1. Masuk ke menu `Purchase Request`.
2. Klik tambah PR.
3. Isi department jika diperlukan.
4. Hubungkan ke project jika kebutuhan berasal dari project.
5. Tambahkan item satu per satu.
6. Isi qty.
7. Isi atau cek estimated unit price.
8. Tambahkan catatan per item bila perlu.
9. Isi catatan umum.
10. Simpan.

Data yang biasanya terlihat di daftar PR:
- nomor PR
- requester
- department atau project
- tanggal dibuat
- total estimasi
- status

### 10.3. Status Purchase Request

Status yang umum muncul:
- `PENDING_APPROVAL`: menunggu persetujuan
- `APPROVED`: disetujui dan siap dijadikan dasar pembelian
- `REJECTED`: ditolak
- `COMPLETED`: sudah diproses ke pembelian

### 10.4. Cara approval atau rejection Purchase Request

Jika user punya hak approval:
1. buka detail PR
2. review item, qty, estimasi harga, dan tujuan pembelian
3. klik `Approve` jika setuju
4. klik `Reject` jika tidak setuju
5. jika reject, isi alasan penolakan

Saran approval:
- cek apakah item memang dibutuhkan
- cek apakah terkait project yang valid
- cek apakah estimasi harga masih masuk akal

## 11. Purchases

Menu `Purchases` adalah pusat transaksi pembelian resmi. Di modul ini user bisa membuat pembelian barang, pembelian dari PR, pembelian untuk project, pembelian aset tetap, dan prepaid expense.

### 11.1. Fitur utama di halaman Purchases

Fitur yang tersedia di halaman ini umumnya meliputi:
- membuat purchase baru
- memilih vendor
- mengambil referensi dari PR yang sudah approved
- menghubungkan purchase ke project
- mengatur pajak dan metode pembayaran
- submit untuk approval
- approval bertahap melalui tombol `Action Required`
- membuat receipt barang
- mencatat pembayaran hutang
- melihat jurnal terkait
- duplicate purchase
- export PDF dan CSV
- filter pencarian berdasarkan vendor, project, tanggal, dan status approval

### 11.2. Alur dasar membuat Purchase

Langkah umum:
1. Masuk ke menu `Purchases`.
2. Klik buat purchase.
3. Pilih vendor.
4. Isi tanggal transaksi dan jatuh tempo jika diperlukan.
5. Tambahkan item.
6. Isi qty, harga, diskon, dan catatan.
7. Pilih tipe item dengan benar.
8. Atur pajak dan metode pembayaran.
9. Simpan draft.
10. Klik `Submit for Approval` jika data sudah siap.

Penting:
- purchase baru biasanya dibuat sebagai `draft`
- setelah draft selesai, user operasional perlu menekan `Submit for Approval`
- approver akan memproses lewat tombol `Action Required`

### 11.3. Membeli produk biasa tanpa PR

Pakai cara ini jika perusahaan mengizinkan pembelian langsung.

Langkah:
1. buat purchase baru
2. pilih vendor
3. isi item barang yang dibeli
4. pilih item type yang sesuai, biasanya `INVENTORY` atau akun beban biasa
5. simpan
6. submit approval

Cocok untuk:
- pembelian rutin kecil
- pembelian yang tidak wajib lewat PR
- koreksi pembelian yang sifatnya cepat

### 11.4. Membeli produk berdasarkan Purchase Request

Gunakan cara ini jika sebelumnya sudah ada PR yang disetujui.

Langkah:
1. buat purchase baru
2. pilih `purchase request` yang statusnya `APPROVED`
3. sistem akan mengambil item terkait PR
4. cek kembali vendor, qty, dan harga realisasi
5. isi pajak dan metode pembayaran
6. simpan draft
7. submit approval

Manfaatnya:
- alur permintaan internal sampai pembelian lebih rapi
- audit trail lebih jelas
- PR akan lebih mudah ditandai selesai setelah diproses

### 11.5. Membeli untuk project tertentu

Gunakan `project_id` atau pilihan project saat membuat purchase jika pembelian berkaitan dengan satu project.

Dampaknya:
- biaya pembelian masuk ke actual cost project
- monitoring realisasi project menjadi lebih akurat
- pembelian bisa ditelusuri kembali ke pekerjaan yang memakainya

Contoh:
- pembelian material proyek instalasi
- pembelian jasa subkon untuk project tertentu
- pembelian alat spesifik yang dibebankan ke satu project

### 11.6. Memilih item type yang benar

Di purchase, pemilihan `item_type` sangat penting.

#### A. `INVENTORY`

Gunakan untuk:
- barang dagang
- bahan baku
- stok gudang
- barang yang memang masuk persediaan

Dampak umum:
- barang dapat dilanjutkan ke proses receipt
- nilai pembelian mengikuti akun persediaan atau beban sesuai setup

#### B. `ASSET`

Gunakan untuk pembelian aset tetap.

Contoh:
- mesin
- kendaraan
- komputer kantor
- peralatan produksi

Saat memilih `ASSET`, biasanya user juga perlu memperhatikan:
- kategori aset
- akun aset tetap yang digunakan

Catatan penting:
- pembelian aset baru sebaiknya dicatat lewat `Purchases`, bukan input manual dari halaman `Assets`
- sistem ini dirancang agar aset baru terbentuk dari alur pembelian
- halaman `Assets` lebih fokus untuk monitoring, depresiasi, dan input opening balance aset lama

#### C. `PREPAID`

Gunakan untuk biaya dibayar dimuka.

Contoh:
- sewa dibayar di muka
- asuransi dibayar di muka
- maintenance kontrak tahunan
- langganan dibayar di muka

Field penting yang perlu diisi:
- akun prepaid
- lama amortisasi dalam bulan

Catatan penting:
- item `PREPAID` tidak memerlukan goods receipt
- setelah pembelian diproses, data prepaid akan dipantau di halaman `Prepaid Expenses`

### 11.7. Tax dan Payment Configuration pada Purchase

Sebelum approval, user finance bisa mengatur:
- PPN rate
- PPh 21 rate
- PPh 23 rate
- pajak tambahan lain
- pengurang lain
- metode pembayaran
- akun cash/bank jika bayar langsung
- akun hutang atau liability jika pembayaran kredit
- referensi pembayaran

Aturan sederhana:
- jika `payment method` adalah kredit, pilih akun hutang atau liability yang benar
- jika bukan kredit, pilih akun cash/bank yang benar

### 11.8. Approval Purchase

Alur sederhananya:
1. employee atau user operasional membuat draft purchase
2. klik `Submit for Approval`
3. approver yang berwenang akan melihat tombol `Action Required`
4. finance dan director dapat memproses approval sesuai workflow
5. jika ditolak, status menjadi `REJECTED`
6. jika disetujui, purchase bisa lanjut ke payment, receipt, dan monitoring jurnal

Saran operasional:
- jangan submit purchase setengah jadi
- pastikan vendor, item, pajak, dan metode bayar sudah benar dulu
- jika purchase ditolak, perbaiki alasan yang menyebabkan reject

### 11.9. Receipt barang

Setelah purchase approved, user dapat membuat `Receipt` untuk item yang memang perlu diterima fisiknya.

Cocok untuk:
- inventory
- barang fisik tertentu
- beberapa pembelian aset yang memang diterima barangnya

Tidak diperlukan untuk:
- item `PREPAID`

### 11.10. Record Payment hutang pembelian

Jika purchase memakai metode kredit dan masih ada outstanding:
1. buka purchase yang approved atau paid sebagian
2. klik `Record Payment`
3. isi tanggal bayar, nominal, metode, dan akun cash/bank
4. simpan

Manfaat:
- saldo hutang berkurang
- histori pembayaran vendor tercatat
- jurnal pembayaran ikut terbentuk

### 11.11. Fitur tambahan yang berguna di Purchases

Selain membuat pembelian, user juga bisa memanfaatkan:
- `Duplicate` untuk menyalin transaksi serupa
- `Export PDF/CSV` untuk arsip atau review
- `View Receipts` untuk melihat histori penerimaan barang
- `View Journal Entries` untuk melihat dampak akuntansi transaksi

## 12. Assets

Menu `Assets` dipakai untuk memonitor aset tetap, depresiasi, lokasi, dan histori aset.

### 12.1. Fungsi utama halaman Assets

Modul ini dipakai untuk:
- melihat daftar aset tetap
- melihat nilai perolehan dan nilai buku
- mengelola kategori aset
- mencatat aset lama sebagai opening balance
- menghitung dan menjalankan depresiasi bulanan
- melihat histori depresiasi
- menyimpan foto dan lokasi aset

### 12.2. Kapan membuat aset dari halaman Assets

Gunakan tombol tambah aset di halaman ini terutama untuk:
- aset yang sudah dimiliki sebelum sistem mulai digunakan
- input opening balance aset lama

Jangan gunakan halaman ini untuk pembelian aset baru jika pembelian itu terjadi saat sistem sudah berjalan.

Untuk aset baru:
- catat pembeliannya dari modul `Purchases` dengan `item_type = ASSET`

### 12.3. Cara input opening balance aset lama

Langkah:
1. Masuk ke menu `Assets`.
2. Klik `Record Existing Asset` atau tambah aset.
3. Isi nama aset.
4. Pilih kategori aset.
5. Isi nomor seri jika ada.
6. Isi kondisi aset.
7. Isi tanggal beli.
8. Isi harga perolehan.
9. Isi nilai residu jika ada.
10. Isi masa manfaat.
11. Pilih metode depresiasi.
12. Isi akun aset tetap.
13. Isi akun beban depresiasi.
14. Isi akun akumulasi depresiasi.
15. Jika aset lama sudah pernah disusutkan, isi opening accumulated depreciation.
16. Isi lokasi fisik.
17. Tambahkan catatan atau foto bila perlu.
18. Simpan.

### 12.4. Menjalankan depresiasi bulanan

Langkah umum:
1. Masuk ke menu `Assets`.
2. Buka fitur `Monthly Depreciation`.
3. Pilih bulan dan tahun.
4. Review daftar aset yang akan disusutkan.
5. Jalankan proses.

Hasilnya:
- nilai buku aset berkurang sesuai perhitungan
- jurnal depresiasi terbentuk
- histori depresiasi tersimpan

### 12.5. Recalculate depreciation

Jika ada kebutuhan pengecekan per aset, buka detail aset lalu gunakan fitur perhitungan ulang depresiasi berdasarkan tanggal tertentu.

### 12.6. Foto dan lokasi aset

Halaman ini juga berguna untuk kontrol fisik aset karena user dapat:
- upload foto aset
- mencatat lokasi aset
- menyimpan koordinat atau titik peta jika dipakai

## 13. Prepaid Expenses

Menu `Prepaid Expenses` dipakai untuk memonitor biaya dibayar dimuka yang dibuat dari pembelian tipe `PREPAID`.

### 13.1. Fungsi utama halaman Prepaid Expenses

Modul ini dipakai untuk:
- melihat daftar prepaid yang aktif
- melihat nominal dan jumlah bulan amortisasi
- melihat jadwal amortisasi
- menentukan akun beban amortisasi
- memposting amortisasi per jadwal
- menghentikan prepaid lebih cepat jika diperlukan
- melihat hubungan ke jurnal

### 13.2. Dari mana data prepaid berasal

Praktik normalnya:
- user membuat purchase dengan `item_type = PREPAID`
- purchase diproses sesuai approval
- data prepaid kemudian muncul di halaman `Prepaid Expenses`

Artinya, halaman ini lebih cocok untuk pengelolaan lanjutan, bukan untuk transaksi pembelian awal.

### 13.3. Cara menggunakan halaman Prepaid Expenses

Langkah:
1. Masuk ke menu `Prepaid Expenses`.
2. Buka salah satu prepaid.
3. Review nama biaya, nominal, lama amortisasi, dan status.
4. Cek jadwal amortisasi.
5. Tentukan atau update akun beban amortisasi bila diperlukan.
6. Jalankan amortisasi untuk jadwal yang sudah jatuh tempo.

### 13.4. Integrasi prepaid dengan jurnal

Setiap kali amortisasi diposting:
- nilai prepaid akan dikurangi sesuai jadwal
- beban periode berjalan akan diakui
- jurnal akan dibuat dan ditautkan ke schedule tersebut

Karena itu, user tidak perlu membuat jurnal amortisasi manual jika sudah diproses dari modul prepaid.

### 13.5. Menghentikan prepaid sebelum waktunya

Jika kontrak selesai lebih cepat atau ada perubahan kondisi:
1. buka prepaid yang dimaksud
2. pilih terminate
3. isi alasan penghentian
4. pilih akun beban bila diminta
5. simpan

### 13.6. Kapan prepaid tidak boleh dihapus

Prepaid biasanya tidak bisa dihapus sembarangan jika:
- sudah ada jurnal amortisasi
- sudah ada jurnal terminasi

Karena itu, lakukan review dulu sebelum menghapus.

## 14. Sales dan Invoiced

Menu `Sales` dipakai untuk membuat transaksi penjualan sampai menjadi invoice dan menerima pembayaran.

### 14.1. Fungsi utama modul Sales

Modul ini dipakai untuk:
- membuat draft penjualan
- memilih customer dan item penjualan
- mengatur invoice type
- mengatur payment terms
- membuat invoice resmi
- mengunduh invoice PDF
- menerima pembayaran
- mengunduh receipt jika sudah lunas

### 14.2. Cara membuat draft penjualan

Langkah umum:
1. Masuk ke menu `Sales`.
2. Klik buat sales.
3. Pilih customer.
4. Isi tanggal invoice.
5. Pilih invoice type jika perusahaan memakainya.
6. Tentukan payment terms.
7. Tambahkan item penjualan.
8. Isi qty, harga, diskon, dan pajak sesuai kebutuhan.
9. Simpan draft.

### 14.3. Cara membuat invoice dari sales

Setelah draft siap:
1. buka transaksi sales
2. klik aksi untuk `Invoice`
3. status transaksi akan berubah menjadi `INVOICED`

Hal penting:
- saat invoice dibuat, sistem membentuk jurnal otomatis
- karena itu, pastikan data penjualan sudah benar sebelum klik invoice

### 14.4. Download invoice

Setelah status invoiced, user dapat mengunduh invoice PDF untuk dikirim ke customer atau dijadikan arsip.

### 14.5. Cara menerima pembayaran dari invoice

Pembayaran bisa direkam dari halaman sales melalui form pembayaran.

Langkah umum:
1. buka sales yang sudah invoiced
2. klik `Record Payment`
3. isi tanggal bayar
4. isi nominal yang diterima
5. pilih metode pembayaran
6. pilih akun cash/bank penerimaan
7. isi referensi pembayaran jika ada
8. simpan

Kemampuan penting modul pembayaran sales:
- mendukung pembayaran penuh atau sebagian
- dapat menyesuaikan ke termin invoice jika termin dipakai
- status invoice akan berubah mengikuti progress pembayaran

### 14.6. Receipt penjualan

Jika invoice sudah lunas, user dapat membuat atau mengunduh receipt PDF sebagai bukti penerimaan pembayaran.

## 15. Payments

Menu `Payments` adalah pusat transaksi kas masuk dan kas keluar yang lebih terstruktur.

### 15.1. Jenis payment yang tersedia

Di halaman `Payments`, user biasanya bisa membuat:
- pembayaran piutang atau `receivable payment`
- pembayaran hutang atau `payable payment`
- `Setor PPN`
- pembayaran beban atau `expense payment`

### 15.2. Kapan memakai menu Payments

Gunakan menu ini jika ingin mengelola transaksi pembayaran dari satu tempat terpusat.

Contoh penggunaan:
- finance menerima pembayaran customer
- finance membayar hutang vendor
- finance membayar biaya operasional
- finance setor pajak PPN

### 15.3. Cara penggunaan umum halaman Payments

Langkah dasar:
1. Masuk ke menu `Payments`.
2. Klik tambah payment.
3. Pilih jenis payment yang sesuai.
4. Isi pihak terkait, tanggal, nominal, metode, dan akun cash/bank.
5. Lengkapi referensi atau catatan jika perlu.
6. Simpan.

### 15.4. Status payment

Status yang umumnya dipakai:
- `DRAFT`
- `PENDING`
- `COMPLETED`
- `FAILED`

### 15.5. Fitur tambahan halaman Payments

User dapat memanfaatkan fitur:
- approve payment draft jika memiliki hak
- edit payment yang belum final
- export detail payment ke PDF
- hapus payment yang masih diperbolehkan sistem
- filter pembayaran berdasarkan status dan metode
- melihat ringkasan total pembayaran dan rata-rata nilai payment

### 15.6. Hubungan Payments dengan modul lain

Hubungan umumnya seperti ini:
- pembayaran customer bisa berasal dari invoice di modul `Sales`
- pembayaran vendor bisa berasal dari hutang di modul `Purchases`
- payment yang diposting akan berdampak ke `Cash & Bank`
- jurnal transaksi payment akan masuk ke modul `Journal`

## 16. Journal

Menu `Journal` adalah tempat untuk melihat dampak akuntansi semua transaksi.

### 16.1. Tab utama di Journal

Umumnya tersedia:
- `Jurnal Umum`
- `Buku Besar`
- `Neraca`

### 16.2. Jurnal otomatis

Transaksi berikut biasanya membentuk jurnal otomatis:
- sales yang sudah di-invoice
- purchase yang sudah diproses
- payment
- transfer cash & bank
- asset depreciation
- prepaid amortization
- transaksi opening atau adjustment tertentu

### 16.3. Kapan pakai jurnal manual

Gunakan jurnal manual hanya jika:
- ada penyesuaian akhir periode
- ada koreksi yang tidak berasal dari modul transaksi
- ada transaksi khusus yang memang belum punya modul khusus

Hindari membuat jurnal manual untuk transaksi yang sebenarnya sudah bisa dibuat dari `Sales`, `Purchases`, `Payments`, `Assets`, atau `Prepaid Expenses`.

### 16.4. Cara membuat jurnal manual

Langkah umum:
1. Masuk ke `Jurnal Umum`.
2. Klik tambah manual entry.
3. Isi tanggal dan deskripsi.
4. Masukkan akun debit dan kredit.
5. Pastikan total debit = total kredit.
6. Simpan.
7. Jika sistem memakai draft, lakukan posting setelah review.

### 16.5. Fitur penting di Jurnal Umum

User biasanya bisa:
- filter jurnal berdasarkan tanggal
- mencari jurnal tertentu
- lihat detail jurnal
- post draft journal
- edit draft journal
- delete draft journal
- reverse jurnal yang sudah posted dengan alasan yang jelas
- export PDF atau CSV

### 16.6. Buku Besar

Gunakan tab `Buku Besar` untuk:
- melihat mutasi satu akun secara rinci
- menelusuri saldo awal, mutasi, dan saldo akhir
- audit pergerakan suatu akun

Cocok dipakai saat mengecek:
- akun bank
- piutang customer
- hutang vendor
- akun aset tetap
- akun beban tertentu

## 17. Users

Menu `Users` dipakai untuk mengatur siapa yang boleh masuk ke sistem dan modul apa saja yang bisa mereka gunakan.

### 17.1. Fungsi utama halaman Users

Modul ini dipakai untuk:
- membuat user baru
- mengedit user lama
- mengatur role
- mengatur permission per user
- menghapus user tertentu jika diperbolehkan

### 17.2. Role yang umum tersedia

Role yang tampak di sistem antara lain:
- `admin`
- `finance`
- `director`
- `employee`
- `inventory_manager`

Arti sederhananya:
- `admin`: akses paling luas
- `finance`: fokus transaksi keuangan, payment, jurnal, dan review pembelian
- `director`: approval dan laporan tingkat manajemen
- `employee`: input transaksi dasar sesuai izin
- `inventory_manager`: fokus produk, stok, dan monitoring barang

### 17.3. Cara membuat user baru

Langkah:
1. Masuk ke menu `Users`.
2. Klik tambah user.
3. Isi username.
4. Isi nama depan dan nama belakang.
5. Isi email.
6. Isi password dan konfirmasi password.
7. Pilih role.
8. Simpan.

### 17.4. Mengatur permission

Selain role, beberapa user bisa memiliki pengaturan permission tambahan.

Gunakan fitur `Manage Permissions` untuk:
- menampilkan atau menyembunyikan menu tertentu
- memberi akses view, create, edit, approve, atau delete sesuai kebutuhan
- membatasi user hanya ke modul yang memang relevan

Praktik terbaik:
- jangan semua user diberi akses penuh
- pisahkan user input, reviewer, approver, dan finance
- review izin akses secara berkala

## 18. Reports

Menu `Reports` dipakai untuk mengambil laporan keuangan dan laporan operasional dari transaksi yang sudah berjalan.

### 18.1. Laporan yang tersedia

Laporan utama yang biasanya tersedia:
- `Profit Loss`
- `Balance Sheet`
- `Cash Flow`
- `Sales Summary`
- `Purchase Report`
- `Trial Balance`
- `General Ledger`
- `Customer History`
- `Vendor History`

### 18.2. Cara memakai Reports

Langkah dasar:
1. Masuk ke menu `Reports`.
2. Pilih jenis laporan.
3. Isi filter tanggal.
4. Jika perlu, pilih customer atau vendor tertentu.
5. Tampilkan laporan.
6. Export ke PDF, CSV, atau Excel sesuai ketersediaan.

### 18.3. Kapan tiap laporan dipakai

Gunakan secara sederhana seperti ini:
- `Profit Loss`: melihat laba rugi periode tertentu
- `Balance Sheet`: melihat posisi aset, kewajiban, dan modal
- `Cash Flow`: melihat arus kas masuk dan keluar
- `Sales Summary`: melihat performa penjualan
- `Purchase Report`: melihat total pembelian dan detail vendor
- `Trial Balance`: mengecek keseimbangan saldo akun
- `General Ledger`: audit transaksi per akun
- `Customer History`: melihat histori transaksi satu customer
- `Vendor History`: melihat histori transaksi satu vendor

### 18.4. Syarat agar laporan akurat

Agar laporan benar, pastikan:
- tax configuration sudah tepat
- purchase dan sales tidak salah akun
- payment sudah dicatat
- depresiasi dan amortisasi sudah dijalankan
- jurnal draft yang perlu diposting sudah diposting
- periode tidak dicampur-campur

## 19. Notifications

Notifikasi membantu user mengetahui transaksi yang perlu ditindaklanjuti tanpa harus membuka semua menu satu per satu.

### 19.1. Jenis notifikasi yang umum

Yang umum muncul antara lain:
- approval pending
- approval approved
- approval rejected
- low stock
- reorder alert
- stock out

### 19.2. Cara memakai notifikasi

Langkah:
1. Klik ikon lonceng di bagian atas aplikasi.
2. Baca daftar notifikasi yang belum dibaca.
3. Klik notifikasi yang relevan.
4. Sistem akan membawa user langsung ke halaman transaksi terkait.

Contoh arah notifikasi:
- ke halaman `Purchases`
- ke halaman `Sales`
- ke halaman `Purchase Requests`
- ke halaman `Products`

### 19.3. Manfaat notifikasi

Notifikasi sangat membantu untuk:
- approval yang belum diproses
- stok barang yang hampir habis
- transaksi yang baru disetujui atau ditolak
- tindak lanjut yang harus segera dikerjakan oleh role tertentu

## 20. Contoh Alur End-to-End yang Disarankan

Berikut contoh alur operasional yang paling masuk akal untuk perusahaan yang memakai komponen, project, pembelian, dan penjualan.

### 20.1. Alur setup awal

1. Isi `Settings`
2. Lengkapi `Tax Configuration`
3. Buat akun `Cash & Bank`
4. Siapkan `Products`, kategori, satuan, dan lokasi gudang
5. Import komponen atau bahan baku
6. Buat `Target Assembly`
7. Buat `BOM Template`
8. Buat `Users` dan atur role

### 20.2. Alur project dan pengadaan

1. Buat `Project`
2. Pasang `BOM` ke project
3. Jalankan `Generate PR (Stock Check)` jika stok kurang
4. Review dan submit `Purchase Request`
5. Approver menyetujui PR
6. Buat `Purchase` dari PR yang approved
7. Hubungkan purchase ke project
8. Submit purchase untuk approval
9. Setelah approved, lakukan receipt jika barang fisik
10. Lakukan pembayaran sesuai metode yang dipakai

### 20.3. Alur aset tetap

1. Buat purchase dengan `item_type = ASSET`
2. Selesaikan approval purchase
3. Pantau aset di halaman `Assets`
4. Jalankan depresiasi tiap bulan
5. Review jurnal depresiasi di `Journal`

### 20.4. Alur prepaid expense

1. Buat purchase dengan `item_type = PREPAID`
2. Isi masa amortisasi
3. Selesaikan approval purchase
4. Pantau data di `Prepaid Expenses`
5. Jalankan amortisasi sesuai jadwal
6. Review jurnal amortisasi di `Journal`

### 20.5. Alur penjualan dan penagihan

1. Buat `Sales`
2. Review draft
3. Klik `Invoice`
4. Kirim invoice PDF ke customer
5. Catat pembayaran dari customer
6. Jika lunas, unduh receipt
7. Review dampak transaksi di `Reports`

## 21. Kesalahan Umum yang Harus Dihindari

Berikut kesalahan yang paling sering terjadi saat sistem seperti ini dipakai:
- langsung membuat jurnal manual padahal modul sumber transaksi sudah tersedia
- salah mapping akun pajak di awal setup
- membuat aset baru langsung dari halaman `Assets` padahal seharusnya lewat `Purchases`
- memakai `PREPAID` untuk beban biasa yang seharusnya langsung diakui di periode berjalan
- lupa menghubungkan pembelian ke project sehingga biaya actual project tidak terbaca
- lupa submit purchase untuk approval setelah draft selesai
- memilih akun cash/bank yang salah saat pembayaran
- tidak menjalankan depresiasi atau amortisasi di akhir bulan
- menarik laporan saat masih banyak jurnal draft belum final

## 22. Rekomendasi SOP Operasional

Agar pemakaian sistem stabil, berikut SOP sederhana yang disarankan.

### Harian
- cek notifikasi
- proses approval yang masuk
- input purchase, sales, dan payment hari itu
- cek cash & bank utama

### Mingguan
- review PR dan purchase outstanding
- cek stok minimum dan reorder alert
- cek piutang dan hutang yang mendekati jatuh tempo
- cek project yang biaya aktualnya mulai mendekati budget

### Bulanan
- rekonsiliasi cash & bank
- jalankan depresiasi aset
- jalankan amortisasi prepaid
- review jurnal penyesuaian
- cetak laporan utama
- lakukan period closing jika semua transaksi sudah lengkap

## 23. Ringkasan Integrasi Antar Modul

Supaya mudah dipahami, hubungan antar modul bisa diringkas seperti ini:
- `Settings` menentukan akun dan aturan dasar
- `Cash & Bank` dipakai oleh `Purchases`, `Sales`, dan `Payments`
- `Products` memberi data barang untuk `PR`, `Purchases`, `Sales`, dan `BOM`
- `Target Assembly` dan `BOM` mendukung kebutuhan `Project`
- `Project` bisa menghasilkan kebutuhan pembelian melalui `Purchase Request`
- `Purchase Request` menjadi dasar `Purchases`
- `Purchases` dapat membentuk data `Assets` atau `Prepaid Expenses` tergantung tipe item
- `Sales`, `Purchases`, `Payments`, `Assets`, dan `Prepaid Expenses` berdampak ke `Journal`
- `Journal` menjadi dasar `Reports`
- `Notifications` membantu user masuk ke transaksi yang perlu diproses

## 24. Penutup

Jika tim mengikuti urutan penggunaan yang benar, aplikasi ini bisa dipakai sebagai sistem operasional dan akuntansi yang saling terhubung.

Urutan paling penting yang perlu diingat adalah:
- setup akun dan pajak dulu
- buat cash & bank
- siapkan master produk dan BOM
- jalankan project, PR, dan purchase dengan disiplin approval
- gunakan sales untuk invoicing
- gunakan payments untuk arus kas masuk dan keluar
- biarkan jurnal dan laporan mengikuti transaksi yang sudah benar

Asumsi panduan ini:
- nama menu mengikuti tampilan aplikasi saat ini
- beberapa tombol bisa berbeda tergantung role user
- jika satu fitur tidak terlihat, periksa permission user lebih dulu
