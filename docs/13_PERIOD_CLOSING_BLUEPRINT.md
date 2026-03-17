# Period Closing Blueprint

## Tujuan

`Period closing` harus menjadi pengunci akuntansi lintas modul, bukan hanya fitur di menu Settings. Setelah suatu periode ditutup, semua transaksi yang berdampak ke ledger, kas/bank, piutang, hutang, persediaan bernilai, prepaid, dan aset tetap wajib ditolak bila tanggal akuntansinya berada di periode tertutup.

## Prinsip Dasar

1. Semua transaksi finansial harus memiliki `effective posting date`.
2. Validasi periode tertutup wajib berada di service layer, bukan hanya di route middleware atau frontend.
3. Dokumen boleh tetap berstatus `DRAFT`, tetapi aksi yang mem-post dampak akuntansi tidak boleh berjalan pada tanggal yang sudah ditutup.
4. Koreksi periode tertutup harus dilakukan dengan `reversal`, `adjusting entry`, atau `reopen period` yang tercatat jelas.
5. Semua jurnal otomatis harus menyimpan relasi sumber (`source_type`, `source_id`) dan tanggal posting yang eksplisit.

## Tanggal Yang Wajib Dipakai

| Modul | Effective posting date |
| --- | --- |
| Sales invoice | `sale.date` |
| Purchase approval | `purchase.date` |
| Payment approval | `payment.date` |
| Cash & Bank deposit/withdrawal/transfer | `request.date` |
| Manual journal | `entry_date` |
| Prepaid amortization | `schedule.scheduled_date` |
| Prepaid termination | `termination/effective date` |
| Asset capitalization/depreciation/disposal | tanggal transaksi akuntansi modul aset |

## Aksi Yang Harus Dicek

- `create` bila langsung auto-post ke ledger
- `approve`
- `post`
- `reverse`
- `void`
- `cancel`
- `delete`
- `edit` yang mengubah tanggal, jumlah, akun, atau saldo yang sudah posted

## Aturan Implementasi

### 1. Service-level period guard

Semua service yang bisa menulis efek akuntansi harus memanggil helper bersama:

```go
if err := CheckClosedPeriod(db, effectiveDate); err != nil {
    return err
}
```

Helper ini wajib dipanggil sebelum:

- membuat jurnal SSOT
- mengubah saldo `cash_banks`
- membuat `cash_bank_transactions`
- mengubah stok yang menimbulkan nilai persediaan/COGS
- mengubah outstanding AR/AP akibat posting final

### 2. Middleware hanya pelengkap

Route middleware tetap berguna untuk validasi cepat dari request body, tetapi tidak boleh menjadi satu-satunya proteksi. Callback approval, background jobs, import script, dan service internal tetap harus aman walau middleware terlewati.

### 3. Effective date, bukan server time

Jangan gunakan `time.Now()` untuk transaksi akuntansi periodik bila sistem sudah punya tanggal akuntansi yang benar. Contoh:

- amortisasi prepaid harus memakai `scheduled_date`
- terminasi prepaid harus memakai `effective_date`
- approval payment harus memakai `payment.date`

### 4. Reversal dan koreksi

- Dokumen posted di periode tertutup tidak boleh dihapus langsung.
- Koreksi harus lewat jurnal pembalik atau posting koreksi di periode terbuka.
- `reopen period` harus terbatas untuk role tertentu dan menyimpan audit trail.

## Matrix Modul

### Sales

- `CreateInvoice` wajib cek `sale.date`
- `ConfirmSale` boleh tetap terbuka bila belum ada dampak GL
- delete/cancel/return tetap tunduk ke closed period bila menyentuh transaksi posted

### Purchases

- `OnPurchaseApproved` wajib cek `purchase.date`
- immediate payment wajib menolak update kas/bank di periode tertutup
- purchase return/cancel/delete wajib mengikuti aturan yang sama

### Payments

- approval draft payment wajib cek `payment.date`
- reversal payment wajib cek `reversal date`
- cash/bank movement dari payment wajib selalu memakai tanggal payment

### Cash & Bank

- transfer, deposit, withdrawal wajib cek `request.date`
- seluruh jurnal kas/bank harus menggunakan tanggal request yang sama
- reconciliation adjustment yang mem-post jurnal juga wajib cek tanggal adjustment

### Prepaid

- amortisasi wajib cek `schedule.scheduled_date`
- terminasi wajib meminta `effective_date`
- jurnal prepaid harus dibuat dalam transaksi DB yang sama dengan update status schedule/header

### Journal

- create/post/update/delete jurnal manual wajib cek `entry_date`
- auto-generated journal juga wajib mematuhi helper yang sama

## Status Implementasi Saat Ini

Blueprint ini mulai diterapkan di service layer berikut:

- Sales invoice posting
- Purchase approval posting
- Payment approval posting
- Cash & Bank transfer/deposit/withdrawal
- Prepaid amortization dan termination
- Shared unified journal creation helper

## Gap Lanjutan Yang Masih Disarankan

- paksa approval flow lain yang masih bergantung middleware agar memanggil service guard
- standardisasi `posting_date` vs `document_date` di seluruh DTO dan controller
- tambahkan test integration untuk close -> reject posting -> reopen -> allow posting
