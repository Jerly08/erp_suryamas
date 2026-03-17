# Panduan Deployment ERP Akuntansi ke VPS Production

Dokumen ini menggantikan alur deploy lama. Rekomendasi terbaru memakai `docker-compose.prod.yml` + Caddy reverse proxy agar:

- HTTPS aktif otomatis dengan sertifikat Let's Encrypt
- hanya port `80/443` yang terbuka ke internet
- backend dan database tidak terekspos publik
- ada healthcheck, restart policy, log rotation, dan volume persistence

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
```

Catatan penting:

- `DOMAIN` harus sudah resolve ke IP VPS sebelum deploy pertama, supaya HTTPS otomatis berhasil.
- Jangan buka port PostgreSQL ke publik.
- Jangan pakai password dan secret pendek.

## 4. Deploy production

Cara paling cepat:

```bash
chmod +x scripts/vps/deploy.sh
./scripts/vps/deploy.sh
```

Atau manual:

```bash
docker compose --env-file .env.production -f docker-compose.prod.yml up -d --build
```

## 5. Verifikasi

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

## 6. Cara update aplikasi

```bash
cd /opt/erp_akuntansi
git pull
./scripts/vps/deploy.sh
```

## 7. Backup database

Script backup sudah disiapkan:

```bash
chmod +x scripts/vps/backup-db.sh
./scripts/vps/backup-db.sh
```

Hasil backup akan masuk ke folder `backups/`.

Disarankan jadwalkan via `cron`, misalnya backup harian jam 02:00.

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
