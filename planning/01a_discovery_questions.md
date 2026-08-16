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
