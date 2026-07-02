# Panduan Deployment ERP Akuntansi ke VPS Production

Dokumen ini menggantikan alur deploy lama. Rekomendasi terbaru memakai `docker-compose.prod.yml` + Caddy reverse proxy agar:

- HTTPS aktif otomatis dengan sertifikat Let's Encrypt
- hanya port `80/443` yang terbuka ke internet
- backend dan database tidak terekspos publik
- ada healthcheck, restart policy, log rotation, dan volume persistence
- build image aplikasi dilakukan di GitHub Actions, sehingga VPS production hanya `pull` image dan restart container

## Arsitektur Production

Request user masuk ke:

1. `Caddy` pada port `80/443`
2. `frontend` Next.js pada jaringan Docker internal
3. `backend` Go API pada jaringan Docker internal
4. `db` PostgreSQL pada jaringan Docker internal

## Rekomendasi Spesifikasi VPS

Minimum yang realistis untuk aplikasi ini:

- `2 vCPU`
- `4 GB RAM`
- `60 GB SSD`

Untuk user aktif lebih banyak atau report berat:

- `4 vCPU`
- `8 GB RAM`
- `100+ GB SSD`

## Prasyarat

- Ubuntu 22.04 LTS atau Debian 12
- Domain sudah mengarah ke IP VPS
- Port `80` dan `443` dibuka di firewall/security group
- Docker Engine dan Docker Compose plugin terpasang
- Image production sudah berhasil dibuat oleh GitHub Actions pada branch `production`

## 1. Setup server

```bash
sudo apt update && sudo apt upgrade -y
sudo apt install -y git curl ufw

curl -fsSL https://get.docker.com | sudo sh
sudo usermod -aG docker $USER
newgrp docker

sudo ufw allow OpenSSH
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw enable
```

## 2. Clone repository

```bash
cd /opt
sudo git clone https://github.com/Jerly08/erp_akuntansi.git
sudo chown -R $USER:$USER /opt/erp_akuntansi
cd /opt/erp_akuntansi
```

## 3. Siapkan environment production

Salin template env:

```bash
cp .env.production.example .env.production
nano .env.production
```

Isi minimal:

```env
DOMAIN=erp.example.com
ACME_EMAIL=admin@example.com

DB_USER=postgres
DB_PASSWORD=ganti-dengan-password-kuat
DB_NAME=erp_akuntansi

JWT_SECRET=ganti-dengan-secret-acak-panjang
JWT_ACCESS_SECRET=ganti-dengan-secret-acak-panjang
JWT_REFRESH_SECRET=ganti-dengan-secret-acak-panjang
SESSION_SECRET=ganti-dengan-secret-acak-panjang

BACKEND_IMAGE=ghcr.io/jerly08/erp_akuntansi-backend
FRONTEND_IMAGE=ghcr.io/jerly08/erp_akuntansi-frontend
IMAGE_TAG=production
```

Catatan penting:

- `DOMAIN` harus sudah resolve ke IP VPS sebelum deploy pertama, supaya HTTPS otomatis berhasil.
- Jangan buka port PostgreSQL ke publik.
- Jangan pakai password dan secret pendek.
- Jika image GHCR masih private, login Docker di VPS dengan token GitHub yang punya akses package:

```bash
echo "GITHUB_TOKEN_ATAU_PAT" | docker login ghcr.io -u Jerly08 --password-stdin
```

## 4. Build image production

Setiap push ke branch `production` akan menjalankan GitHub Actions:

1. validasi frontend dengan `npm run lint` dan `npm run typecheck`
2. build image backend
3. build image frontend
4. push image ke GHCR dengan tag `production` dan commit SHA

Image default yang dipakai VPS:

```text
ghcr.io/jerly08/erp_akuntansi-backend:production
ghcr.io/jerly08/erp_akuntansi-frontend:production
```

## 5. Deploy production

Cara paling cepat:

```bash
chmod +x scripts/vps/deploy.sh
./scripts/vps/deploy.sh
```

Atau manual:

```bash
docker compose --env-file .env.production -f docker-compose.prod.yml pull
docker compose --env-file .env.production -f docker-compose.prod.yml up -d --no-build
```

Deploy production tidak melakukan build di VPS. Server hanya mengambil image terbaru lalu menjalankan ulang container yang berubah.

## 6. Verifikasi

Cek status container:

```bash
docker compose --env-file .env.production -f docker-compose.prod.yml ps
```

Cek log saat pertama kali boot:

```bash
docker compose --env-file .env.production -f docker-compose.prod.yml logs -f
```

Cek health backend dari dalam VPS:

```bash
docker compose --env-file .env.production -f docker-compose.prod.yml exec backend wget -qO- http://127.0.0.1:8080/api/v1/health
```

Jika semua normal, aplikasi dapat diakses di:

```text
https://DOMAIN_ANDA
```

## 7. Cara update aplikasi

Pastikan GitHub Actions untuk branch `production` sudah selesai dan image `production` sudah ter-push ke GHCR, lalu jalankan di VPS:

```bash
cd /opt/erp_akuntansi
git pull
./scripts/vps/deploy.sh
```

Untuk rollback ke image commit tertentu, ubah sementara `IMAGE_TAG` di `.env.production` menjadi SHA commit yang sudah pernah dibuild, lalu jalankan `./scripts/vps/deploy.sh`.

## 8. Backup harian lokal

Script backup database saja:

```bash
chmod +x scripts/vps/backup-db.sh
./scripts/vps/backup-db.sh
```

Untuk backup harian production, gunakan script gabungan database + file upload:

```bash
chmod +x scripts/vps/backup-daily.sh
./scripts/vps/backup-daily.sh
```

Hasil backup harian lokal masuk ke:

```text
backups/daily/database/
backups/daily/uploads/
backups/daily/manifests/
```

Default retensi backup lokal adalah `30` hari. Retensi bisa diubah saat menjalankan script:

```bash
BACKUP_RETENTION_DAYS=60 ./scripts/vps/backup-daily.sh
```

Jadwalkan via `cron` agar berjalan setiap hari jam 00:00:

```bash
sudo crontab -e
```

Tambahkan:

```cron
0 0 * * * cd /opt/erp_akuntansi && BACKUP_RETENTION_DAYS=30 ./scripts/vps/backup-daily.sh >> /var/log/erp-backup.log 2>&1
```

Catatan: backup lokal tetap berada di VPS yang sama. Untuk production yang lebih aman, tetap siapkan backup offsite/cloud agar data aman jika VPS rusak.

### Backup ke Google Drive

Backup harian bisa otomatis di-copy ke Google Drive memakai `rclone`.

Install rclone di VPS:

```bash
sudo apt update
sudo apt install -y rclone
```

Hubungkan Google Drive:

```bash
rclone config
```

Pilih:

```text
n) New remote
name: erp-gdrive
Storage: drive
```

Ikuti proses login Google sampai selesai, lalu test:

```bash
rclone lsd erp-gdrive:
```

Tambahkan target Google Drive di `.env.production`:

```env
BACKUP_OFFSITE_TARGET=erp-gdrive:erp-akuntansi/backups
```

Jika `BACKUP_OFFSITE_TARGET` diisi, `backup-daily.sh` akan:

- tetap membuat backup lokal terlebih dahulu
- upload database dump ke Google Drive
- upload archive uploads ke Google Drive
- upload manifest/checksum ke Google Drive

Untuk keamanan yang lebih kuat, gunakan `rclone crypt` di atas Google Drive agar isi backup terenkripsi sebelum naik ke cloud.

### Notifikasi email backup

Notifikasi email dikirim oleh `backup-daily.sh` jika `BACKUP_NOTIFY_EMAIL_TO` diisi. Cara paling ringan adalah memakai `msmtp` sebagai sendmail-compatible mailer.

Install:

```bash
sudo apt install -y msmtp msmtp-mta bsd-mailx
```

Buat konfigurasi:

```bash
sudo nano /etc/msmtprc
```

Contoh konfigurasi SMTP:

```text
defaults
auth           on
tls            on
tls_trust_file /etc/ssl/certs/ca-certificates.crt
logfile        /var/log/msmtp.log

account        backup
host           smtp.gmail.com
port           587
from           backup@example.com
user           backup@example.com
password       APP_PASSWORD_SMTP

account default : backup
```

Amankan file:

```bash
sudo chmod 600 /etc/msmtprc
```

Test kirim email:

```bash
echo "SMTP backup notification test" | mail -s "ERP Backup Test" tujuan@example.com
```

Tambahkan email tujuan di `.env.production`:

```env
BACKUP_NOTIFY_EMAIL_TO=tujuan@example.com
BACKUP_NOTIFY_EMAIL_FROM=backup@example.com
BACKUP_NOTIFY_SUBJECT_PREFIX="[ERP Backup]"
BACKUP_NOTIFY_ON_SUCCESS=true
BACKUP_NOTIFY_ON_FAILURE=true
```

Setelah itu cron backup harian tetap sama:

```cron
0 0 * * * cd /opt/erp_akuntansi && BACKUP_RETENTION_DAYS=30 ./scripts/vps/backup-daily.sh >> /var/log/erp-backup.log 2>&1
```

### Restore-test bulanan

Backup yang baik harus pernah dites restore. Script berikut akan mengambil backup database dan upload terbaru, lalu:

- restore database ke container PostgreSQL sementara
- validasi dump database bisa dibaca dan memiliki tabel
- validasi archive upload bisa dibuka
- membuat laporan restore-test lokal

Jalankan manual:

```bash
chmod +x scripts/vps/restore-test-latest.sh
./scripts/vps/restore-test-latest.sh
```

Hasil laporan masuk ke:

```text
backups/daily/restore-tests/
```

Jadwalkan restore-test otomatis 1x per bulan, misalnya tanggal 1 jam 01:00:

```cron
0 1 1 * * cd /opt/erp_akuntansi && ./scripts/vps/restore-test-latest.sh >> /var/log/erp-restore-test.log 2>&1
```

## 8. Catatan production penting

- Compose production baru tidak mengekspos `db` dan `backend` ke internet.
- TLS/HTTPS ditangani Caddy otomatis.
- Upload file tetap persisten di volume `erp_uploads`.
- Database tetap persisten di volume `postgres_data`.
- Logging container memakai rotasi ukuran supaya disk VPS tidak cepat penuh.
- Backend sekarang aman dipasang di belakang reverse proxy dan melakukan graceful shutdown saat restart/deploy.

## 9. Jika deploy pertama gagal

Periksa 4 hal ini dulu:

1. Domain belum mengarah ke IP VPS.
2. Port `80/443` masih tertutup firewall provider.
3. Secret/env belum lengkap di `.env.production`.
4. VPS kehabisan RAM saat build frontend Next.js.

Jika build frontend berat di VPS kecil, tambahkan swap minimal `2 GB`.
