# Delcom Blog — Backend (Golang Gin REST API)

REST API platform publikasi artikel multi-tier (Publik, Penulis, Admin) dengan
Clean Architecture & Feature-First, PostgreSQL 17, JWT, Swagger UI, serta
**AI Blog Generator** yang terintegrasi langsung dengan **Search Engine**.

## Teknologi
Go 1.27.4 · Gin v1.12 · GORM v1.31 (PostgreSQL / SQLite in-memory untuk test) ·
JWT v5 · bcrypt · godotenv · swaggo · testify

## Menjalankan

```bash
psql -U postgres -c "CREATE DATABASE pabwe_p9_blog;"
cp .env.example .env    # sesuaikan DB_* dan OPENAI_* / SEARCH_ENGINE_ENDPOINT
go mod download
go run .                    # atau: go run cmd/server/main.go
```

- API: `http://localhost:8081`
- Swagger UI: `http://localhost:8081/swagger/index.html`
- Spesifikasi OpenAPI: `http://localhost:8081/api.json`
- Akun admin awal dibuat otomatis dari `ADMIN_USERNAME` / `ADMIN_PASSWORD` (default `admin` / `admin`).

Build binary:

```bash
go build -o server .        # Windows: go build -o server.exe .
./server
```

## Pengujian (100% statement coverage)

```bash
go test -v -cover ./internal/...

# sama persis dengan stage Test di Jenkinsfile (total 100%)
go test -v -coverprofile=coverage.out -covermode=atomic ./...
go tool cover -func=coverage.out
```

Regenerasi dokumentasi Swagger setelah mengubah anotasi handler:

```bash
go run github.com/swaggo/swag/cmd/swag@v1.16.6 init -g main.go -o docs --parseInternal
cp docs/swagger.json docs/api.json
```

## Struktur

```
main.go                   entry point root (dipakai Jenkins: go build -o server .)
cmd/server/main.go        entry point alternatif (go run cmd/server/main.go)
internal/app              bootstrap: config -> DB -> migrate -> seed -> router :8081
Jenkinsfile               pipeline CI/CD (test, build, Trivy, package, publish, deploy)
docs/                     Swagger/OpenAPI (docs.go, swagger.json/yaml, api.json)
internal/config           loader .env
internal/database         koneksi PostgreSQL/SQLite, model User & Post, migrasi, seed admin
internal/middleware       CORS, JWT Bearer (+ revoke saat logout), RequireRole, RateLimiter
internal/response         format JSON standar {status, message, data?}
internal/features/auth    register, login, logout, profil
internal/features/post    CRUD artikel, slug unik, reading time (kata/200 wpm), statistik, admin
internal/features/ai      search.go (Search Engine), client.go (OpenAI Chat Completions), service, handler
internal/server           router Gin, Swagger UI, NoRoute 404
```

## Endpoint

| Method | Endpoint | Akses |
|---|---|---|
| GET | `/swagger/*any`, `/api.json` | Publik |
| GET | `/api/public/categories` | Publik |
| GET | `/api/public/posts?q=&category=` | Publik |
| GET | `/api/public/posts/:slug` | Publik |
| POST | `/api/auth/register`, `/api/auth/login` | Publik |
| POST | `/api/auth/logout` | Login |
| GET/PUT | `/api/auth/profile` | Login |
| GET/POST | `/api/posts` | Penulis |
| GET | `/api/posts/stats` | Penulis |
| GET/PUT/DELETE | `/api/posts/:id` | Penulis |
| PATCH | `/api/posts/:id/toggle-publish` | Penulis |
| POST | `/api/posts/ai-generate` | Penulis |
| GET | `/api/admin/users`, `/api/admin/posts`, `/api/admin/users/:id/posts` | Admin |

## Alur AI Blog Generator

1. Frontend → `POST /api/posts/ai-generate` `{topic, category, tone, keywords, include_web_search, max_sources}`
2. Jika `include_web_search=true`, backend mengirim `POST SEARCH_ENGINE_ENDPOINT` dengan body
   `{"query", "q", "limit", "max_results"}`. Respons dinormalkan secara fleksibel
   (`results` / `items` / `data` / `web.results` / array langsung; field `title|name`, `url|link|href`,
   `snippet|content|description`).
3. Prompt topik + konteks hasil penelusuran dikirim ke `OPENAI_ENDPOINT/chat/completions`
   (`Authorization: Bearer OPENAI_KEY`, `model: OPENAI_MODEL`).
4. Jawaban JSON model (boleh dibungkus ```json) diurai menjadi judul, kategori, excerpt, konten Markdown;
   daftar **Sumber Rujukan** ditambahkan otomatis bila model belum menuliskannya.
5. Jika mesin pencari gagal, artikel tetap dibuat dan respons menyertakan `search_warning`.

> Catatan: format persis API `delcom.org/api/v1/search` tidak terdokumentasi di spesifikasi, sehingga
> parser dibuat toleran terhadap beberapa bentuk respons umum. Sesuaikan `internal/features/ai/search.go`
> bila format resminya berbeda.

## CI/CD Jenkins

`Jenkinsfile` menjalankan: Checkout → Install Dependencies (`go mod download/verify`) →
Test (`go test ./...` + coverage, total **100%**) → Build (`CGO_ENABLED=0 go build -o server .`) →
Trivy (`HIGH,CRITICAL`, exit-code 1; saat ini 0 temuan) → Package (`git archive`, tanpa `.env` dan biner) →
Publish → Deploy. Variabel `URL_REDEPLOY`, `URL_PROGRESS`, `DEPLOY_TOKEN`, `WEBSITE_ID` diatur di Jenkins.

## Database Neon (PostgreSQL 17, cloud)

Project Neon: **delcom-blog** (`sweet-wave-85984970`), region `aws-ap-southeast-1` (Singapura),
branch `main`, database `pabwe_p9_blog`. Tabel `users` & `posts` (skema identik dengan GORM AutoMigrate)
dan akun admin awal (`admin` / `admin`) sudah dibuat.

Cara menghubungkan backend:

1. Buka https://console.neon.tech → project **delcom-blog** → **Connect**, pilih database `pabwe_p9_blog`,
   salin connection string.
2. Isi environment variable (di `.env` lokal **atau** di pengaturan platform deploy — `.env` tidak ikut
   ke `latest-app.zip` hasil Jenkins):

   ```
   DATABASE_URL=postgresql://neondb_owner:PASSWORD@ep-xxxx.ap-southeast-1.aws.neon.tech/pabwe_p9_blog?sslmode=require
   APP_KEY=<string acak panjang>
   ADMIN_PASSWORD=<ganti dari default>
   ```

   Jika `DATABASE_URL` diisi, variabel `DB_*` diabaikan. Port server memakai `APP_PORT`, lalu `PORT`
   (umum diset otomatis oleh platform deploy), lalu default `8081`.
