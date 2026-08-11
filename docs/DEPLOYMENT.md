# Deployment Guide — Finance Accounting APP

**Server:** `119.28.116.123` (Tencent Cloud VM)
**SSH Alias:** `finance-accounting-vps` (sudah dikonfigurasi di `~/.ssh/config`)
**Domain:** `https://accounting.tikuma.net/` (via Cloudflare proxy)

## SSH Setup (Sekali Saja)

SSH key sudah di-copy ke `~/.ssh/tikuma.pem` dan alias `finance-accounting-vps` sudah dikonfigurasi.
Untuk sesi baru, cukup gunakan alias tanpa perlu upload ulang:

```bash
# Connect langsung (tidak perlu -i /Users/user/Downloads/tikuma.pem lagi)
ssh finance-accounting-vps

# Jalankan command di server
ssh finance-accounting-vps 'docker ps'
ssh finance-accounting-vps 'cd ~/Finance-Accounting-APP && git pull origin main'
```

Jika SSH config hilang, recreate dengan:
```bash
mkdir -p ~/.ssh && chmod 700 ~/.ssh
cat >> ~/.ssh/config << 'EOF'
Host finance-accounting-vps
  HostName 119.28.116.123
  User ubuntu
  Port 22
  IdentityFile ~/.ssh/tikuma.pem
  IdentitiesOnly yes
  ServerAliveInterval 60
EOF
chmod 600 ~/.ssh/config
# Copy key jika belum ada di ~/.ssh/tikuma.pem:
# cp /Users/user/Downloads/tikuma.pem ~/.ssh/tikuma.pem && chmod 600 ~/.ssh/tikuma.pem
```

## Arsitektur Deployment

```
Internet → Cloudflare (accounting.tikuma.net) → Caddy (:80/:443)
                                                    ├─ /api/* + /healthz* → finance-api (:8080)
                                                    ├─ /* → finance-web (:80, React)
                                                    └─ finance-api → finance-db (:5432) + finance-nextreport (:3100)
```

### Containers (docker compose)
| Container | Image/Build | Port | Fungsi |
|---|---|---|---|
| finance-db | postgres:16-alpine | 5432 (internal) | PostgreSQL 16 |
| finance-api | build `./Dockerfile` | 8080 (internal) | Go API |
| finance-nextreport | build `./nextreport/Dockerfile` | 3100 (internal) | Report rendering (zero-dep Node) |
| finance-web | build `./web/Dockerfile` | 80 (internal) | React SPA (static) |
| finance-caddy | caddy:2-alpine | 80, 443 | Reverse proxy + gzip |

### Volumes
| Volume | Isi |
|---|---|
| db_data | PostgreSQL data |
| audit_data | Audit attachments (/data/audit) |
| caddy_data | Caddy TLS certs |
| caddy_config | Caddy config |

## Deploy Procedure

### 1. Deploy Code Baru (Standard Flow)

```bash
# Dari lokal: commit + push
git add -A && git commit -m "feat: ..." && git push origin main

# Di server: pull + rebuild
ssh finance-accounting-vps 'cd ~/Finance-Accounting-APP && git pull origin main && docker compose up -d --build api web'
```

Rebuild semua container:
```bash
ssh finance-accounting-vps 'cd ~/Finance-Accounting-APP && docker compose up -d --build'
```

### 2. Apply Migration Baru

Migrations TIDAK auto-apply. Apply manual via psql:
```bash
# Apply satu migration
ssh finance-accounting-vps 'cd ~/Finance-Accounting-APP && cat backend/migrations/0000XX_name.up.sql | docker exec -i finance-db psql -U finance -d finance'

# Rollback
ssh finance-accounting-vps 'cd ~/Finance-Accounting-APP && cat backend/migrations/0000XX_name.down.sql | docker exec -i finance-db psql -U finance -d finance'

# Cek migration terakhir yang applied
ssh finance-accounting-vps 'docker exec finance-db psql -U finance -d finance -c "\dt"'
```

**Migration yang sudah applied:** 000001 s/d 000044 (lihat `backend/migrations/`)

### 3. Environment Variables

Didefinisikan via default di `docker-compose.yml` (atau override via `.env` di server):
| Variable | Default | Fungsi |
|---|---|---|
| `DB_PASSWORD` | `changeme_secure_2026` | PostgreSQL password |
| `JWT_SECRET` | `super_secure_jwt_secret_32chars_min_2026` | JWT signing (WAJIB 32+ char, fail-fast jika kosong) |

**PENTING:** Production server sudah punya `.env` dengan `JWT_SECRET` random (dibuat saat setup awal). JANGAN overwrite kecuali memang ingin reset semua session.

### 4. Verifikasi Deploy

```bash
# Health check
curl -s https://accounting.tikuma.net/healthz
curl -s https://accounting.tikuma.net/healthz/detail | python3 -m json.tool

# Login test
curl -s -X POST https://accounting.tikuma.net/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@test.com","password":"Test123456!"}'

# Container status
ssh finance-accounting-vps 'docker ps --format "{{.Names}}: {{.Status}}"'

# Logs jika ada masalah
ssh finance-accounting-vps 'docker logs finance-api --tail=50'
```

## Setup VPS dari Nol (Jika Ganti Server)

### Prerequisites di VPS (Ubuntu)
```bash
ssh ubuntu@NEW_IP
sudo apt update && sudo apt install -y docker.io docker-compose-v2 git
sudo systemctl enable docker && sudo systemctl start docker
sudo usermod -aG docker $USER  # logout/login ulang agar aktif
```

### Clone & Setup
```bash
cd ~ && git clone git@github.com:Axelwor/Finance-Accounting-APP.git
cd Finance-Accounting-APP

# Buat .env dengan secret kuat
cat > .env << EOF
DB_PASSWORD=$(openssl rand -hex 24)
JWT_SECRET=$(openssl rand -hex 32)
EOF

# Build & start semua
docker compose up -d --build

# Tunggu DB healthy, lalu apply semua migrations
for f in backend/migrations/*.up.sql; do
  echo "Applying $f"
  cat "$f" | docker exec -i finance-db psql -U finance -d finance || echo "FAILED: $f"
done
```

### DNS & SSL (Cloudflare)
1. Di Cloudflare → DNS: buat A record `accounting` → IP VPS baru (Proxied).
2. SSL/TLS mode: **Full** (bukan Flexible).
3. Caddy otomatis pakai Let's Encrypt (via HTTP-01). Pastikan port 80/443 terbuka.

### Update Caddyfile jika ganti domain
Edit `Caddyfile` di repo, ganti `accounting.tikuma.net` dengan domain baru, commit + deploy.

## Backup & Restore Database

```bash
# Backup
ssh finance-accounting-vps 'docker exec finance-db pg_dump -U finance finance | gzip > ~/backup-finance-$(date +%F).sql.gz'

# Download backup ke lokal
scp finance-accounting-vps:~/backup-finance-*.sql.gz .

# Restore
cat backup.sql | ssh finance-accounting-vps 'gunzip | docker exec -i finance-db psql -U finance finance'
```

## Troubleshooting

| Masalah | Solusi |
|---|---|
| API 502/503 | `docker logs finance-api --tail=50`; cek `docker compose ps` |
| DB connection refused | Tunggu DB healthy: `docker compose ps`; cek `db_data` volume |
| Migration gagal | Cek error spesifik, pastikan urutan migration benar (000001 dulu) |
| JWT invalid setelah deploy | `.env` server ter-overwrite; restore JWT_SECRET lama |
| Caddy cert gagal | Pastikan port 80 terbuka + Cloudflare SSL mode = Full |
| Frontend tidak update | Rebuild web: `docker compose up -d --build web` |

## Checklist Deploy Cepat (Copy-Paste)

```bash
# Full deploy dalam satu command
ssh finance-accounting-vps 'cd ~/Finance-Accounting-APP && git pull origin main && docker compose up -d --build api web && docker compose ps'
```

## File Penting

| File | Fungsi |
|---|---|
| `docker-compose.yml` | Orchestration (5 services + 4 volumes) |
| `Dockerfile` | Go API image (multi-stage build) |
| `nextreport/Dockerfile` | Report rendering service (zero-dep Node) |
| `web/Dockerfile` | React SPA static build |
| `Caddyfile` | Reverse proxy config |
| `.env` (server) | Secrets (DB_PASSWORD, JWT_SECRET) |
| `backend/migrations/` | 44 migrations (000001 s/d 000044) |
| `docs/API_CONTRACT.md` | API contract (150+ endpoints) |
