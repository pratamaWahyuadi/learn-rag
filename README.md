# RAG Pipeline — Kursus Online

Backend RAG untuk kursus online. Stack: **Go + Gin + sqlc + Postgres (pgvector) + pg_notify worker**. Menerima upload video/audio/PDF via presigned URL ke **Cloudflare R2**, mentranskripsi/mem-parsing, membuat chunk + embedding, menyimpan summary, lalu menjawab query RAG dengan referensi segmen.

> **Keamanan**: API key hanya disimpan sebagai hash SHA-256. Row Level Security (RLS) mengisolasi data antar tenant; semua query user-facing memakai `tenant_id` sebagai defense-in-depth. Jangan pernah menaruh API key, presigned URL, `file_key`, transkrip, atau prompt ke dalam log.

## Arsitektur

- **`cmd/server`** — `rag-server`, HTTP API.
- **`cmd/worker`** — `rag-worker`, memproses antrian job (download → transcribe/parse → chunk → embed → store → summarize) dengan `pg_notify` + `LISTEN` dan fallback polling `FOR UPDATE SKIP LOCKED`.
- **`migrations/`** — skema Postgres berbasis **goose** (`001_init.sql`).
- **`internal/`** — layanan, ports/adapters, provider eksternal (R2, Groq, LlamaParse, Cloudflare AI, DeepSeek).
- **`scripts/seed.sql`** — seed tenant + API key untuk development/demo.

## Prasyarat

- Go 1.25+
- Docker + Docker Compose (Postgres + pgvector lokal)
- Cloudflare R2 bucket, dan key provider eksternal (Groq, LlamaParse, Cloudflare AI, DeepSeek)

## Menjalankan lokal

```sh
make deps        # go mod tidy
make db-up       # docker compose up -d db
make migrate-up  # goose migrate (set DATABASE_URL)
make build       # bin/rag-server + bin/rag-worker
make test        # unit tests
make test-integration  # integration tests (butuh DATABASE_URL + DB)
```

Contoh koneksi lokal: `export DATABASE_URL=postgres://rag:rag@localhost:5432/rag`

Jalankan server dan worker di terminal terpisah:

```sh
DATABASE_URL=$DATABASE_URL bin/rag-server
DATABASE_URL=$DATABASE_URL bin/rag-worker
```

> **Catatan migrasi**: goose v3 mengharapkan satu file `{version}_name.sql` dengan seksi `-- +goose Up` / `-- +goose Down`. Jika masih ada file `*.up.sql`/`*.down.sql` dari format lama, pindahkan ke direktori lain sebelum `goose up`.

## Seed tenant & API key

API key dibuat via seeding SQL (bukan via API), sesuai asumsi teknis:

1. Generate hash SHA-256 dari plaintext key (plaintext hanya ditampilkan sekali, lalu dibuang):
   ```sh
   printf 'rag-admin-secret-1' | sha256sum
   ```
2. Isi `key_hash` di `scripts/seed.sql` dengan digest 64-char hasil perintah di atas untuk key `admin` dan `query`.
3. Jalankan seed (idempotent):
   ```sh
   psql "$DATABASE_URL" -f scripts/seed.sql
   ```

`scripts/seed.sql` menyediakan satu tenant demo (`acme`) dengan satu API key scope `admin` dan satu scope `query`.

## Deployment ke VPS (1 VPS, systemd)

Two systemd services: `rag-api` dan `rag-worker`.

1. **Buat user `rag`**:
   ```sh
   sudo useradd --system --create-home --home-dir /var/lib/rag rag
   ```
2. **Instal binary** (dibuat dengan `make build`, statik `CGO_ENABLED=0`):
   ```sh
   sudo install -m 0755 bin/rag-server /usr/local/bin/rag-server
   sudo install -m 0755 bin/rag-worker /usr/local/bin/rag-worker
   ```
3. **Salin env** (awali dari `deploy/env.example`):
   ```sh
   sudo mkdir -p /etc/rag-pipeline
   sudo cp deploy/env.example /etc/rag-pipeline/api.env
   sudo cp deploy/env.example /etc/rag-pipeline/worker.env
   sudo chown rag:rag /etc/rag-pipeline/api.env /etc/rag-pipeline/worker.env
   sudo chmod 600 /etc/rag-pipeline/api.env /etc/rag-pipeline/worker.env
   # lalu edit isi variabel sesuai lingkungan, termasuk DATABASE_URL
   ```
4. **Pasang unit systemd**:
   ```sh
   sudo cp deploy/systemd/rag-api.service    /etc/systemd/system/rag-api.service
   sudo cp deploy/systemd/rag-worker.service /etc/systemd/system/rag-worker.service
   sudo systemctl daemon-reload
   sudo systemctl enable --now rag-api
   sudo systemctl enable --now rag-worker
   ```
5. **Seed database** setelah migrasi:
   ```sh
   sudo -u rag bash -c 'psql "$DATABASE_URL" -f /path/to/scripts/seed.sql'
   ```
6. **Verifikasi health**:
   ```sh
   curl -s http://localhost:8080/healthz
   # -> {"status":"ok","service":"api","time":"...","db":"up"}
   ```

### CORS R2 untuk upload langsung dari browser

Presigned URL R2 ditulis langsung dari browser. Aktifkan **CORS** pada bucket R2 di panel Cloudflare untuk origin frontend (mis. `https://app.example.com`), misalnya:

```
AllowedOrigins: ["https://app.example.com"]
AllowedMethods: ["PUT", "POST", "GET", "DELETE", "HEAD"]
AllowedHeaders: ["*"]
ExposeHeaders:  ["ETag"]
MaxAgeSeconds:  3600
```

Tanpa ini, browser akan memblokir upload langsung ke presigned URL.

## Contoh penggunaan API

Ambil API key hasil seed (scope `admin` dan `query`).

```sh
# 1. Buat upload intent -> dapatkan presigned URL
curl -X POST http://localhost:8080/api/v1/upload-intents \
  -H "Content-Type: application/json" \
  -H "X-API-Key: <admin key>" \
  -d '{"content_type":"application/pdf"}'

# 2. Upload file langsung ke R2 via presigned_url (biasanya dari browser)

# 3. Buat job dari file_key hasil langkah 1
curl -X POST http://localhost:8080/api/v1/jobs \
  -H "Content-Type: application/json" \
  -H "X-API-Key: <admin key>" \
  -d '{"file_key":"<file_key>","title":"Materi Web","segments":["web design"]}'

# 4. Query RAG (scope query)
curl -X POST http://localhost:8080/api/v1/query \
  -H "Content-Type: application/json" \
  -H "X-API-Key: <query key>" \
  -d '{"question":"Apa itu tag HTML?","segment":"web design"}'
```

Proses job dilakukan oleh `rag-worker`; status bisa dicek via `GET /api/v1/jobs`.

## CI / GitHub Actions

`.github/workflows/ci.yml` menjalankan terhadap service Postgres `pgvector:pg15`: install goose, migrasi, `go build ./...`, `go vet ./...`, unit test (`go test ./...`), dan integration test (`go test -tags integration ./...`).

## Konfigurasi environment

Lihat `deploy/env.example` untuk daftar lengkap variabel (tanpa secret) dan default-nya.
