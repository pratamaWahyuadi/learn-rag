--- Pertanyaan Discovery ---
Oke, gw bakal gali semuanya biar PRD/LLD nanti nggak ngarang asumsi sendiri. Gw pisah per kategori biar gampang di-scan. Jawab sejujurnya meskipun masih “belum kepikiran” — itu juga informasi penting.

**Kategori 1: Skala & Beban Sistem**
- Buat demo pertama, perkiraan ada berapa klien B2B yang ngeliat/pake secara bersamaan? Dan kalau udah production, target total klien aktif berapa?
- Berapa banyak video/audio yang bakal di-upload per hari atau per minggu? Berapa durasi rata-rata tiap media, dan durasi maksimal yang mungkin (misal 30 menit, 2 jam)?
- Berapa banyak query dari klien per hari? Apakah query itu real-time pas demo (misal 10 klien tanya barengan) atau lebih ke internal testing?
- Apakah ada skenario lonjakan traffic, misal event launch atau klien besar lagi presentasi? Kalau ada, kira-kira peak request per detik berapa?

**Kategori 2: Timeline & Prioritas**
- Kapan target demo pertama harus siap? Kapan target production kalau deal deal?
- Kalau waktunya mepet, mana yang paling gampang dikorbankan: fitur PDF, filter segmen, kualitas transkrip, atau error handling yang lengkap?
- Apakah PDF wajib ada di demo pertama, atau bisa nyusul setelah demo?

**Kategori 3: Budget & Constraint Infrastruktur**
- Ada perkiraan budget bulanan untuk semua layanan (Groq, DeepSeek, LlamaParse, Cloudflare AI, R2, Supabase/Neon, hosting Go)? Atau masih belum dihitung?
- Data materi kursus kan proprietary klien. Apakah boleh data itu dikirim ke Groq, DeepSeek, dan Cloudflare yang servernya kemungkinan di AS? Ada batasan compliance atau data residency?
- Apakah perlu ada perjanjian pemrosesan data dengan provider pihak ketiga, atau cukup pakai TOS mereka aja?

**Kategori 4: Tech Stack — Bahasa & Framework**
- Kamu pilih Go `net/http`. Itu keputusan final atau masih open? Kenapa milih stdlib dibanding framework seperti Gin/Echo? Tim udah familiar dengan Go?
- Apakah kamu bersedia nambah dependency untuk routing/middleware/validation, atau targetnya pure standard library sebisa mungkin?
- Untuk proses upload yang bisa banyak sekaligus, kamu mau handle concurrency-nya gimana: cukup pakai goroutine per upload, atau perlu antrian/worker pool yang jelas?

**Kategori 5: Tech Stack — Database**
- Supabase/Neon, mana yang bakal dipakai buat demo? Dan apakah arsitekturnya harus gampang pindah dari satu ke yang lain, atau nggak ada masalah kalau beda?
- Data klien B2B harus diisolasi per tenant nggak? Misal Klien A harus nggak bisa akses data Klien B. Kalau iya, apakah semua tabel bakal punya `tenant_id`?
- Segmen yang kamu sebut kayak “Web Desain” itu per video, per kursus, atau per topik yang bisa lintas video? Apakah segmennya satu level atau bisa hierarki (misal Web Desain → HTML Dasar → Tag)?
- Soal parent-child chunk: kamu bilang child = 1 kalimat pertama, parent = 3 kalimat setelahnya. Maksudnya parent itu 3 kalimat *setelah* kalimat pertama, bukan gabungan dari 4 kalimat? Atau parent = 3 kalimat *termasuk* kalimat pertama? Tolong perjelas karena ini ngaruh ke skema tabel retrieval.
- Untuk tabel retrieval, apakah parent dan child disimpan di tabel yang sama dengan relasi, atau dua tabel terpisah? Apakah satu child bisa punya lebih dari satu parent?

**Kategori 6: Tech Stack — Akses Data**
- Kamu pilih sqlc. Semua query bakal ditulis manual terus di-generate? Tim udah familiar dengan sqlc atau baru rencana?
- Query untuk filter segmen itu sifatnya dinamis (bisa kombinasi segmen lain) atau hanya satu filter tunggal? Kalau dinamis, gimana rencananya di sqlc yang cenderung statis — apakah kamu mau tetep pake sqlc dengan kondisi optional, atau mau tambah query builder untuk kasus itu?
- Alat migration database apa yang mau dipakai bareng sqlc (goose, golang-migrate, tern, dsb)? Atau belum ada preferensi?
- Pas injeksi chunk, apakah ada kebutuhan transaksi antar tabel (retrieval + summary) biar kalau embedding gagal nggak ada data setengah jadi? Atau cukup proses terpisah dan di-retry manual?

**Kategori 7: Tech Stack — Frontend**
- Apakah backend ini pure API, atau perlu ada halaman admin untuk upload video/PDF dan lihat progress transkrip/summary?
- Kalau perlu frontend admin, framework frontend-nya mau apa, atau nggak peduli asal jalan? Kalau nggak perlu frontend, siapa yang bakal panggil API upload — klien punya dashboard sendiri?
- Untuk klien yang query, apakah mereka bakal akses lewat API langsung (karena mereka punya frontend) atau kamu juga perlu sediakan halaman chat/query buat demo?

**Kategori 8: Tech Stack — Auth**
- Belum disebut auth sama sekali. Gimana admin dan klien diautentikasi? API key per tenant, atau akun dengan password?
- Apakah perlu role yang jelas: admin bisa upload/delete, klien cuma bisa query? Apakah klien cuma boleh akses data tenant-nya sendiri?
- Token yang diinginkan: JWT, session cookie, atau OAuth? Ada preferensi ke provider kayak Supabase Auth, atau mau handle manual?

**Kategori 9: Tech Stack — Deployment**
- Backend Go mau di-deploy di mana? VPS (Hetzner/DigitalOcean), PaaS (Railway/Fly.io), atau container di Cloudflare? Atau belum ada preferensi?
- Loop demo vs production: apakah pakai environment terpisah? Perlu nggak CI/CD yang otomatis (misal GitHub Actions) atau cukup build manual?
- R2 disebut buat penyimpanan sementara. Setelah video/PDF diproses, file aslinya dihapus dari R2 atau tetap disimpan sebagai arsip? Kalau tetap, apakah butuh signed URL buat akses?

**Kategori 10: Integrasi Eksternal**
- Daftar integrasi yang udah disebut: Groq Whisper, DeepSeek, LlamaParse, Cloudflare AI Worker, R2, Supabase/Neon. Ada integrasi lain yang kamu rasa wajib, misal notifikasi via email/Slack, webhook ke klien soal status proses, atau koneksi ke LMS?
- Sebaliknya, apa yang eksplisit *tidak perlu* diintegrasikan sekarang — misal payment gateway, email service, atau speech-to-text lain? Biar nggak diasumsikan.
- Untuk upload, apakah admin upload langsung dari browser ke R2 lewat presigned URL, atau upload ke backend Go dulu baru backend yang teruskan ke R2?

**Kategori 11: Data & Privasi**
- Data sensitif yang ditangani: materi video course, transkrip, PDF, mungkin nama admin/klien. Ada PII lain nggak, kayak data peserta/nama murid?
- Kebijakan retensi: setelah video berhasil di-transkrip dan di-embed, video asli di R2 apakah langsung dihapus? Transkrip dan summary disimpan permanen?
- Kalau klien minta data mereka dihapus (right to be forgotten), apakah perlu hard delete atau cukup soft delete?

**Kategori 12: Offline/Sinkronisasi**
- Apakah sistem harus tetep jalan kalau koneksi internet lagi jelek, misal pas demo di kantor klien yang wifinya buruk? Atau sistem ini online-only dan nggak masalah?
- Kalau salah satu provider pihak ketiga (Groq/DeepSeek/Cloudflare) lagi down, apakah perlu fallback lokal untuk transkrip/embedding, atau cukup error dan tunggu pulih?

**Kategori 13: Observability & Maintenance**
- Siapa yang maintain setelah rilis? Tim developer ini juga support, atau bakal diserahke ke tim klien?
- Level pengalaman tim teknis buat handle pipeline ini gimana — udah pernah pegang Go + sqlc + pgvector + Cloudflare Workers, atau masih belajar?
- Kebutuhan logging/monitoring: apakah perlu log terstruktur di tiap tahap (upload → transkrip → chunk → embed → summary → query), metrik durasi, error rate, alarm kalau gagal? Kalau iya, tool yang diinginkan (Sentry, Grafana, OpenTelemetry, dsb) atau nggak peduli asalkan ada?

**Kategori 14: Non-functional Requirements**
- Ekspektasi latency query: dari klien kirim pertanyaan sampai dapat jawaban + referensi segmen, maksimal berapa detik?
- Ekspektasi processing upload: buat video 1 jam, berapa lama maksimal sampai siap di-query? Apakah admin perlu lihat progress real-time?
- Uptime: apakah ada target SLA ke klien B2B, misal 99.9%? Atau masih wajar kalau ada downtime di masa awal?
- Skalabilitas jangka panjang: perkiraan total jam konten dan jumlah klien 1–2 tahun ke depan. Apakah arsitektur ini harus siap multi-region atau cukup single-region dulu?

**Kategori 15: Edge Case & Kegagalan**
- Skenario kegagalan mana yang paling kamu takutkan: Groq down, DeepSeek down, Cloudflare AI Worker kena rate limit, R2 gagal upload, database penuh, atau data retrieval/chunk yang salah?
- Buat video yang durasinya panjang banget (misal 2 jam) atau audio jelek sampai transkripnya ngaco, gimana penanganannya? Apakah admin harus review transkrip dulu sebelum di-embed, atau langsung diproses otomatis?
- Kalau PDF yang di-upload ternyata hasil scan dan LlamaParse gagal ekstrak teks, apakah perlu fallback OCR atau cukup dianggap gagal? 
- Bahasa konten: apakah video/PDF isinya beragam bahasa (Indonesia, Inggris, campur)? Kalau iya, apakah summary dari DeepSeek harus dalam bahasa tertentu sesuai dokumen, atau selalu bahasa Indonesia/Inggris?

Ini dulu yang gw perlu tahu. Jawab sebisanya, kalau ada yang masih belum kepikiran bilang aja “belum mikir” — nanti biar PRD nggak mutusin sendiri.


=== Jawaban User ===
Kategori 1: Skala & Beban Sistem
Klien B2B demo bersamaan: 5–10 tenant.

Target total klien production: 50 tenant aktif (bukan unlimited).

Total user aktif: 100–500 user (asumsi 1–5 user per tenant yang query).

Upload awal (populasi data): 500–1.000 file (video/PDF) diproses lewat antrian background.

Upload setelah stabil: 1–5 video/minggu per tenant (penambahan materi biasa).

Durasi video rata-rata: 30–60 menit.

Durasi maksimal: 2 jam (hard limit, file >2 jam ditolak atau di-split manual).

Query per hari (production): 1.000–5.000 request/hari.

Query saat demo: 5–10 tenant tanya bareng, estimasi 10–20 req/menit.

Skenario lonjakan (launch/event): peak pendek 20–50 req/detik.

Kategori 2: Timeline & Prioritas
Target demo pertama: 1 bulan dari sekarang.

Target production: Langsung setelah demo (free trial ke calon klien).

Fitur yang bisa dikorbankan: TIDAK ADA — PDF, filter segmen, kualitas transkrip, error handling semuanya WAJIB.

PDF di demo pertama: WAJIB ada.

Kategori 3: Budget & Constraint Infrastruktur
Budget bulanan: Belum dihitung.

Data residency / compliance: Tidak masalah data dikirim ke AS (Groq, DeepSeek, Cloudflare). Kursus online tidak terlalu sensitif. Self-hosted bisa ditambahkan nanti karena pakai interface.

Perjanjian pemrosesan data: Cukup pakai ToS provider masing-masing.

Kategori 4: Tech Stack — Bahasa & Framework
Framework: Gin (bukan net/http stdlib) karena lebih maintainable dan tidak verbose. Tim sudah familiar dengan Go.

Dependency tambahan: Bersedia asalkan battle-tested dan jarang breaking change.

Concurrency upload: Pakai antrian/worker pool yang jelas, bukan goroutine per upload. Worker concurrency 3–5 proses paralel untuk transkrip (jaga rate limit Groq).

Kategori 5: Tech Stack — Database
Database untuk demo: Supabase (Postgres + pgvector).

Kemudahan migrasi: Arsitektur harus gampang dipindah ke Neon atau DB lain.

Isolasi tenant: WAJIB — semua tabel punya kolom tenant_id. Klien A tidak boleh akses data Klien B.

Struktur Segmen: Flat tag (bukan hierarki), many-to-many: 1 video bisa punya banyak segmen, 1 segmen bisa dipakai banyak video.

Parent-Child Chunking: Parent = 4 kalimat utuh. Child = 1 kalimat pertama dari Parent. Overlap/stride = 2 kalimat.

Tabel Retrieval: 2 tabel terpisah dengan Foreign Key. 1 Child hanya punya 1 Parent (relasi 1-to-Many).

Kategori 6: Tech Stack — Akses Data
sqlc: Sudah familiar. Semua query ditulis manual lalu di-generate.

Filter segmen: Filter tunggal (bukan kombinasi dinamis). Frontend kirim segmen di JSON, backend filter WHERE tenant_id = $1 AND segmen = $2.

Alat migration: Belum ada preferensi (goose, golang-migrate, tern, dll).

Transaksi injeksi chunk: TIDAK pakai transaksi antara tabel retrieval dan summary. Proses terpisah dan retry manual kalau gagal.

Kategori 7: Tech Stack — Frontend
Perlu frontend? Ya, halaman web minimalis untuk demo.

Cakupan frontend:

Halaman admin: upload video/PDF via presigned URL ke R2, polling status GET /api/v1/jobs/{id}.

Halaman chat/query untuk klien bertanya.

Upload flow: Frontend minta presigned URL ke backend → upload langsung ke R2 → kirim file_key + metadata ke backend → backend buat job.

Kategori 8: Tech Stack — Auth
Metode Auth: API Key sederhana (header X-API-Key) untuk semua endpoint di demo.

Role: Implisit dari API key per tenant. Admin punya akses upload, klien hanya query (tenant_id sudah isolasi data).

Token: Tidak pakai JWT/OAuth dulu. Multi-tenant auth layer mature ditunda post-demo.

Kategori 9: Tech Stack — Deployment
Target deployment: 1 VPS (Hetzner/DigitalOcean).

Arsitektur: 2 binary terpisah:

cmd/server (API)

cmd/worker (proses background)

Masing-masing pakai systemd dengan Restart=on-failure dan health check /healthz.

Environment: Terpisah untuk demo/prod via env var.

CI/CD: Manual dulu (build sendiri).

Penyimpanan file: Wajib R2 via presigned URL. File TIDAK lewat backend. Worker download dari R2 untuk diproses. File asli dihapus otomatis 7 hari setelah selesai diproses.

Kategori 10: Integrasi Eksternal
Integrasi wajib:

Groq Whisper (transkrip)

DeepSeek (summary & RAG answer)

LlamaParse (parsing PDF)

Cloudflare Workers AI (BGE-M3 embedding batch)

Supabase (DB + pgvector)

Cloudflare R2 (object storage)

Yang TIDAK perlu: Payment gateway, email service, SMS, LMS integration.

Mekanisme upload: Admin upload langsung ke R2 via presigned URL, bukan lewat backend Go. Backend hanya generate presigned URL, terima metadata, dan buat job.

Kategori 11: Data & Privasi
Data sensitif: Video course, transkrip, PDF, nama admin/tenant. Tidak ada PII murid/participant.

Retensi file asli: File video/PDF di R2 dihapus otomatis 7 hari setelah selesai diproses.

Retensi transkrip/summary: Disimpan permanen di database.

Right to be forgotten: Belum ada kebijakan spesifik. Untuk sekarang, hard delete manual dari DB jika diminta (post-demo).

Kategori 12: Offline / Sinkronisasi
Koneksi internet jelek: Sistem online-only, tidak ada mode offline.

Provider down (Groq/DeepSeek/Cloudflare): Cukup error dan tunggu pulih. Retry dengan backoff. Wajib pakai interface agar fallback lokal (Ollama, dsb) bisa ditambahkan tanpa refactor besar.

Kategori 13: Observability & Maintenance
Tim maintenance: Tim developer sendiri.

Level pengalaman tim: Familiar dengan Go, sqlc, pgvector. Cloudflare Workers AI API sudah ditangani.

Logging & Monitoring: Wajib structured logging di tiap tahap (upload → transkrip → chunk → embed → summary → query). Health check endpoint ada. Prometheus/Grafana ditunda post-demo.

Kategori 14: Non-functional Requirements
Latency query: Maksimal 3–5 detik dari pertanyaan sampai jawaban + referensi segmen.

Processing video 1 jam: Tidak ada batasan waktu (background). Admin tidak wajib lihat real-time progress, tapi tersedia endpoint status.

Uptime SLA: Tidak ada SLA ketat untuk demo. Downtime wajar di masa awal.

Skalabilitas jangka panjang: Cukup single-region dulu. Multi-region/horizontal scaling ditunda post-demo.

Kategori 15: Edge Case & Kegagalan
Kegagalan paling ditakuti: Groq/DeepSeek/Cloudflare down atau kena rate limit. Penanganan: retry dengan backoff + log error.

Video panjang (2 jam) / audio jelek: Transkrip langsung diproses otomatis tanpa review admin. Summary pakai Map-Reduce untuk transkrip >12.000 token, direct untuk ≤12.000 token.

PDF scan (gagal ekstrak teks): Belum ada OCR fallback. Status job jadi failed, perlu di-upload ulang/manual.

Bahasa konten: Video/PDF bisa campur Indonesia-Inggris. DeepSeek handle dua-duanya. Summary default ikut bahasa dominan dokumen, bisa diatur lewat prompt template.

📌 Catatan Teknis Tambahan (Final)
Batch Embedding: Wajib dipakai (kirim banyak teks sekaligus ke Cloudflare, bukan satu-satu).

Job Orchestration: pg_notify + LISTEN sebagai jalur utama, polling SKIP LOCKED sebagai fallback (interval 60 detik).

Summary: Map-Reduce untuk transkrip >12.000 token, direct untuk ≤12.000 token.

Upload: Direct-to-R2 via presigned URL; backend tidak menerima file besar.



