# Panduan Deployment Sistem Akuntansi ke VPS (Menggunakan Docker)

Panduan ini berisi langkah-langkah untuk melakukan deployment aplikasi ERP Akuntansi (Frontend + Backend + PostgreSQL) ke VPS Anda menggunakan server Linux (Ubuntu/Debian) via GitHub.

## Prasyarat Server (VPS)
Sebelum mulai, pastikan VPS Anda:
1. Menjalankan OS Ubuntu 20.04/22.04 LTS (atau varian Linux lainnya).
2. Memiliki **Git** yang terinstall.
3. Memiliki **Docker** dan **Docker Compose** terinstall.

## Langkah 1: Install Git dan Docker di VPS
Jika VPS belum terinstall Docker, jalankan script ini di terminal VPS Anda:

```bash
# Update repository
sudo apt update && sudo apt upgrade -y

# Install Git
sudo apt install git -y

# Install Docker
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh

# Install Docker Compose Plugin
sudo apt-get install docker-compose-plugin -y
```

## Langkah 2: Clone Repository Anda dari GitHub
Di terminal VPS Anda, clone repository menggunakan Git:

```bash
# Pindah ke direktori home (optional)
cd ~

# Clone repo (Ganti link-nya jika berubah)
git clone https://github.com/Jerly08/erp_akuntansi.git

# Masuk ke folder project
cd erp_akuntansi
```

## Langkah 3: Konfigurasi Environment Variables (.env)
Dalam metode deployment ini, seluruh _environment variables_ (termasuk kredensial Database, Secret JWT, dan URL Front End) diatur pada satu file terpusat.

Buat file `.env` di root project Anda:
```bash
nano .env
```

Isi dengan template berikut:
```env
# Database Credentials
DB_USER=postgres
DB_PASSWORD=GantiDenganPasswordSuperKuatUntukDatabase!
DB_NAME=sistem_akuntansi

# JWT Secrets (HARUS KUAT MINIMAL 32 KARAKTER)
JWT_SECRET=GenerateStringAcakSuperKuatPanajgnRahasia!
JWT_ACCESS_SECRET=GenerateStringAcakSuperKuatPanjangRahasia!
JWT_REFRESH_SECRET=GenerateStringAcakSuperKuatPanjangRahasiaRefresh!
SESSION_SECRET=GenerateStringAcakSuperKuatPanjangRahasiaSession!

# Pengaturan Lingkungan
ENVIRONMENT=production
SERVER_PORT=8080

# Pengaturan Domain & CORS
DOMAIN=vps-domain-anda.com
APP_URL=http://vps-domain-anda.com:3003

# URLs (Frontend Proxy)
# FRONTEND_URL adalah URL/IP public VPS (beserta / tanpa port 3003) yang bisa diakses user, cth: http://ip-api-vps-anda:3003 atau IP Domain public anda
FRONTEND_URL=http://localhost:3003
```
Tekan `CTRL + X`, lalu tekan `Y`, lalu `Enter` untuk menyimpan.

> **Note:** Pada `docker-compose.yml` yang dikonfigurasi, url API dari frontend `NEXT_PUBLIC_API_URL` tidak perlu diubah, API proxy NextJS (`/api/...`) sudah otomatis mengarah ke container backend!

## Langkah 4: Build dan Run Docker Compose
Sekarang Anda bisa menjalankan seluruh aplikasi mulai dari frontend, backend, hingga databasenya menggunakan perintah:

```bash
# Menjalankan dalam mode Detached (Berjalan otomatis di background)
sudo docker compose up --build -d
```

Docker akan mengunduh dependencies image, meng-compile build aplikasi backend Anda, dan membangun image production (Standalone) untuk frontend. Proses ini mungkin memakan waktu 2-10 menit.

## Langkah 5: Pengecekan Running Container
Untuk memastikan aplikasi berjalan dengan baik:
```bash
sudo docker compose ps
```
Anda akan melihat tiga container: `erp_db`, `erp_backend`, dan `erp_frontend` dalam status `Up`.

Anda juga dapat melihat logs-nya:
```bash
sudo docker compose logs -f
```

## Akses Aplikasi Anda
Secara default, aplikasi akan _exposed_ pada VPS melalui port:
* **Frontend**: `http://<IP_VPS_ANDA>:3003`
* **Backend**: `http://<IP_VPS_ANDA>:8084` (Akan diproteksi, namun frontend terhubung ke backend ini)
* **PostgreSQL (Optional Access)**: Port `5434` 

*Selesai!* Aplikasi Sistem Akuntansi Anda telah live menggunakan Docker Compose.
