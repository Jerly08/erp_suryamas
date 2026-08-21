# API Quick Start (Public Swagger Edition)

Dokumen ini disusun mengikuti dokumentasi publik di Swagger UI: http://localhost:8080/swagger/index.html#/

Cara pakai URL dari Swagger
- BASE_URL: http://localhost:8080
- Di Swagger UI, setiap endpoint menampilkan Path (mis: /api/cashbank/accounts). Panggil endpoint dengan cara: FULL_URL = BASE_URL + PATH_DARI_SWAGGER.
- Jangan menambahkan /api/v1 lagi secara manual. Cukup pakai persis path yang muncul di UI.
- Selalu sertakan header Authorization: Bearer {{ACCESS_TOKEN}} untuk endpoint yang butuh autentikasi.

1) Health & Auth (dasar)
A. Health check
```bash path=null start=null
curl -X GET "http://localhost:8080/api/v1/health" -H "Accept: application/json"
```
B. Login (ambil access_token & refresh_token)
```bash path=null start=null
curl -X POST "http://localhost:8080/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@example.com","password":"your_password"}'
```
C. Refresh token (saat 401)
```bash path=null start=null
curl -X POST "http://localhost:8080/api/v1/auth/refresh" \
  -H "Content-Type: application/json" \
  -d '{"refresh_token":"{{REFRESH_TOKEN}}"}'
```
D. Profile
```bash path=null start=null
curl -H "Authorization: Bearer {{ACCESS_TOKEN}}" \
  "http://localhost:8080/api/v1/profile"
```

2) Admin (CashBank GL links)
- GET /api/admin/check-cashbank-gl-links – cek status link GL account
- POST /api/admin/fix-cashbank-gl-links – perbaiki link GL account

Contoh:
```bash path=null start=null
curl -H "Authorization: Bearer {{ACCESS_TOKEN}}" \
  "http://localhost:8080/api/admin/check-cashbank-gl-links"
```

3) CashBank (yang tampil di Swagger)
- GET  /api/cashbank/accounts — daftar akun kas/bank
- POST /api/cashbank/accounts — buat akun kas/bank
- GET  /api/cashbank/accounts/{id}
- PUT  /api/cashbank/accounts/{id}
- GET  /api/cashbank/accounts/{id}/transactions
- GET  /api/cashbank/balance-summary — ringkas saldo
- POST /api/cashbank/deposit — setoran
- POST /api/v1/cash-bank/transactions/withdrawal — pengambilan pemilik/Prive terkontrol
- POST /api/cashbank/transfer — transfer antar akun
- GET  /api/cashbank/payment-accounts — akun untuk pembayaran

Contoh setoran:
```bash path=null start=null
curl -X POST "http://localhost:8080/api/cashbank/deposit" \
  -H "Authorization: Bearer {{ACCESS_TOKEN}}" -H "Content-Type: application/json" \
  -d '{"account_id":1,"amount":100000,"date":"2025-01-02"}'
```

4) Purchases → Integrated Payment
- GET  /api/purchases/{id}/for-payment — data siap bayar
- POST /api/purchases/{id}/integrated-payment — buat pembayaran terintegrasi
- GET  /api/purchases/{id}/payments — daftar pembayaran purchase

Contoh integrated payment:
```bash path=null start=null
curl -X POST "http://localhost:8080/api/purchases/123/integrated-payment" \
  -H "Authorization: Bearer {{ACCESS_TOKEN}}" -H "Content-Type: application/json" \
  -d '{"amount":250000,"method":"transfer","notes":"Pembayaran vendor"}'
```

5) Payments (Integration + Umum)
Payment Integration (SSOT):
- POST /api/payments/preview-journal — preview jurnal
- POST /api/payments/enhanced-with-journal — buat pembayaran + jurnal
- GET  /api/payments/{id}/account-updates — dampak saldo
- POST /api/payments/{id}/reverse — reverse payment & jurnal
- GET  /api/payments/{id}/with-journal — detail + jurnal
- GET  /api/payments/account-balances/real-time
- POST /api/payments/account-balances/refresh

Payments (reporting & utilitas):
- GET /api/payments/analytics
- GET /api/payments/debug/recent
- GET /api/payments/summary
- GET /api/payments/export/excel
- GET /api/payments/report/pdf
- GET /api/payments/unpaid-bills/{vendor_id}
- GET /api/payments/unpaid-invoices/{customer_id}
- DELETE /api/payments/{id}
- POST /api/payments/{id}/cancel
- GET /api/payments/{id}/pdf

Contoh enhanced-with-journal:
```bash path=null start=null
curl -X POST "http://localhost:8080/api/payments/enhanced-with-journal" \
  -H "Authorization: Bearer {{ACCESS_TOKEN}}" -H "Content-Type: application/json" \
  -d '{"source":"sale","source_id":456,"amount":300000,"method":"cash"}'
```

6) Journal (Unified)
- GET  /api/v1/journals — list (filter/paging)
- POST /api/v1/journals — create
- GET  /api/v1/journals/{id} — detail
- GET  /api/v1/journals/account-balances — saldo MV
- POST /api/v1/journals/account-balances/refresh — refresh MV
- GET  /api/v1/journals/summary — ringkasan jurnal

Contoh list jurnal:
```bash path=null start=null
curl -H "Authorization: Bearer {{ACCESS_TOKEN}}" \
  "http://localhost:8080/api/v1/journals?status=posted&limit=20&page=1"
```

7) Optimized Reports
- GET  /api/v1/reports/optimized/balance-sheet
- GET  /api/v1/reports/optimized/profit-loss
- POST /api/v1/reports/optimized/refresh-balances
- GET  /api/v1/reports/optimized/trial-balance

8) SSOT Reports (+ Purchase Reports)
- GET  /api/v1/ssot-reports/general-ledger
- GET  /api/v1/ssot-reports/integrated
- GET  /api/v1/ssot-reports/journal-analysis
- GET  /api/v1/ssot-reports/purchase-report
- POST /api/v1/ssot-reports/refresh
- GET  /api/v1/ssot-reports/sales-summary
- GET  /api/v1/ssot-reports/status
- GET  /api/v1/ssot-reports/trial-balance
- GET  /api/v1/ssot-reports/vendor-analysis
- GET  /reports/ssot-profit-loss
- GET  /reports/ssot/balance-sheet
- GET  /reports/ssot/balance-sheet/account-details
- GET  /reports/ssot/cash-flow
- GET  /api/v1/ssot-reports/purchase-report/validate
- GET  /api/v1/ssot-reports/purchase-summary

Contoh Trial Balance (SSOT):
```bash path=null start=null
curl -H "Authorization: Bearer {{ACCESS_TOKEN}}" \
  "http://localhost:8080/api/v1/ssot-reports/trial-balance?as_of_date=2025-06-30"
```

9) Monitoring
- GET  /api/monitoring/balance-health
- GET  /api/monitoring/balance-sync
- GET  /api/monitoring/discrepancies
- POST /api/monitoring/fix-discrepancies
- GET  /api/monitoring/sync-status

Contoh cek kesehatan saldo:
```bash path=null start=null
curl -H "Authorization: Bearer {{ACCESS_TOKEN}}" \
  "http://localhost:8080/api/monitoring/balance-health"
```

10) Security (Admin)
- GET  /api/v1/admin/security/alerts
- PUT  /api/v1/admin/security/alerts/{id}/acknowledge
- POST /api/v1/admin/security/cleanup
- GET  /api/v1/admin/security/config
- GET  /api/v1/admin/security/incidents
- GET  /api/v1/admin/security/incidents/{id}
- PUT  /api/v1/admin/security/incidents/{id}/resolve
- GET  /api/v1/admin/security/ip-whitelist
- POST /api/v1/admin/security/ip-whitelist
- GET  /api/v1/admin/security/metrics

Step-by-step alur singkat (praktis)
1. Login → simpan {{ACCESS_TOKEN}}
2. Cek kesehatan: GET /api/v1/health & /api/monitoring/balance-health
3. CashBank: daftar akun → lakukan deposit, transfer, atau withdrawal Prive → cek transaksi per akun. Biaya/utang/aset dicatat lewat modul Pembayaran atau Purchases.
4. Purchases: GET /api/purchases/{id}/for-payment → POST /api/purchases/{id}/integrated-payment
5. Payments (SSOT): preview-journal → enhanced-with-journal → (opsional) reverse
6. Reports: jalankan optimized/SSOT sesuai kebutuhan
7. Journal: baca ringkasan/daftar jurnal atau refresh MV bila perlu

Tips
- Gunakan persis path yang tampil di Swagger UI (FULL_URL = BASE_URL + PATH_SWAGGER)
- Simpan token ke variabel lingkungan saat uji di terminal.
- Hindari endpoint Deprecated-Payments untuk produksi; utamakan SSOT.
