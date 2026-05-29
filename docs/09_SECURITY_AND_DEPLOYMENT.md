# Keamanan & Deployment

## Keamanan
- Autentikasi: JWT + Refresh Token, blacklist token saat logout/kompromi
- Rate Limiting & Session Tracking: mencegah brute force & monitor aktivitas
- Audit Log: rekam aktivitas penting (create/update/delete, login attempts)
- Security Incidents: dashboard insiden & resolusi
- CORS, security headers, dan pembatasan origin di production

## Checklist Produksi
- Ganti JWT_SECRET kuat & aman
- Isi `BOOTSTRAP_ADMIN_EMAIL` dan `BOOTSTRAP_ADMIN_PASSWORD` hanya untuk deploy pertama pada database kosong; hapus password bootstrap setelah login pertama
- Pastikan `SEED_DEMO_DATA=false` agar akun/data demo tidak masuk production
- Aktifkan HTTPS untuk API & frontend
- Konfigurasi SSL database (jika perlu)
- Atur CORS origin spesifik
- Rate limiting sesuai trafik
- Setup alerting untuk security & balance monitoring
- Kebijakan retensi audit log

## Deployment Backend
- Build: `go build -o sistem-akuntansi cmd/main.go`
- Env produksi: DB_HOST, DB_NAME, JWT_SECRET, GIN_MODE=release
- Jalankan biner & service manager (systemd/PM2 args exec)

## Deployment Frontend
- Build: `npm run build`
- Vercel (disarankan) atau Node server (PM2)
- Set `NEXT_PUBLIC_API_URL` ke domain API produksi
