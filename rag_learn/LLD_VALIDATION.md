## Kontradiksi Tech Stack

1. **Presigned URL conditions — PRD mewajibkan, LLD menyatakan tidak mungkin dengan PUT presigned.**
   - PRD FR-001: *"Presigned URL conditions: `content-length-range` maksimal 2 GB, `content-type` whitelist (video/mp4, audio/mpeg, application/pdf, dll)."*
   - Coding Plan Asumsi #1: *"S3/R2 presigned PUT tidak bisa memaksa `content-length-range` dan `content-type` whitelist secara langsung."*
   - Ini kontradiksi langsung. PRD secara eksplisit mensyaratkan condition `content-length-range` dan `content-type` pada presigned URL, tetapi LLD justru memilih mekanisme PUT biasa yang tidak mendukung condition tersebut, lalu menggantinya dengan validasi manual di worker. Keputusan LLD mengubah perilaku yang sudah ditetapkan PRD di FR-001.

## Asumsi Tambahan di LLD yang Tidak Ada di PRD

1. **Endpoint `POST /api/v1/jobs/{id}/retry`** — API Contract menambahkan endpoint retry khusus. PRD tidak mencantumkan endpoint ini di Functional Requirements (hanya berupa user story "retry job yang gagal" pada persona developer). LLD membuat endpoint baru tanpa dasar eksplisit dari FR PRD.

2. **Implementasi `audit_logs` di MVP** — PRD Threat Report #8 menyatakan: *"Audit trail (Threat #8) masuk backlog post-MVP. Tidak memblokir demo, tapi kita catat sebagai open item."* Namun Database Schema dan Coding Plan justru mengimplementasikan tabel `audit_logs` dan memasukkannya ke Fase 1, Fase 4, dan Fase 7. Ini asumsi tambahan yang melanggar keputusan PRD untuk menunda fitur tersebut ke post-MVP.

3. **Rate limiter in-memory tanpa Redis** — PRD hanya menyebut "rate limit per key dan IP" tanpa menentukan mekanisme penyimpanan. Coding Plan memilih token bucket in-memory dan menyatakan *"Tidak perlu Redis"*. Ini keputusan teknis baru yang tidak disebut PRD, meskipun masih masuk akal untuk skala demo 1 VPS.

4. **Circuit breaker self-implemented** — PRD menyebut "circuit breaker ke Groq, DeepSeek, Cloudflare AI" tanpa detail implementasi. Coding Plan memutuskan membuat sendiri di `internal/circuitbreaker` tanpa dependency eksternal. Ini asumsi teknis tambahan yang tidak ditentukan PRD, tapi tidak bertentangan.

5. **Batch embedding default 16** — PRD hanya mensyaratkan "wajib batch" untuk Cloudflare BGE-M3, tanpa angka. Coding Plan menetapkan default `EMBEDDING_BATCH_SIZE=16`. Ini asumsi implementasi baru yang tidak disebut PRD.

## Kesimpulan

Stack utama (bahasa Go, framework Gin, database Supabase Postgres + pgvector, sqlc, goose, R2, Groq Whisper, LlamaParse, Cloudflare BGE-M3, DeepSeek, API key SHA-256, pg_notify + SKIP LOCKED, systemd, slog) **konsisten dengan pilihan PRD**. Namun ada satu kontradiksi teknis signifikan pada mekanisme presigned URL yang melanggar FR-001, serta beberapa asumsi tambahan di LLD (retry endpoint, audit_logs, in-memory rate limiter, circuit breaker custom, batch size) yang tidak berdasar atau bahkan bertentangan dengan keputusan PRD. Karena ada kontradiksi, **tidak dapat dikatakan "LLD konsisten penuh dengan tech stack yang diputuskan PRD."** Perlu keputusan eksplisit dari pemilik produk untuk menyelesaikan bentrok FR-001 vs implementasi PUT presigned.