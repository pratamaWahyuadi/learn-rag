# API Contract — RAG Pipeline Kursus Online

## 1. Asumsi & Konvensi

1. **Field JSON memakai `snake_case`**, sama dengan nama kolom di database schema. Contoh: `file_key`, `presigned_url`, `tenant_id`, `created_at`.
2. **Timestamp selalu RFC3339 / ISO8601 UTC**, contoh: `2026-06-01T12:00:00Z`.
3. **Resource `videos` mewakili semua materi**: video, audio, dan PDF. Jika diperlukan, `kind` membedakan jenis materi: `video`, `audio`, `pdf`.
4. **`kind` pada job tidak dikirim oleh client**. Backend menurunkannya dari `content_type` upload intent yang dipakai.
5. **Segmen dikirim sebagai array string** pada `POST /jobs`, bukan ID. Backend yang melakukan lookup atau create segmen per tenant.
6. **Manajemen API key tidak disediakan via API**. API key dibuat/direvoke langsung dari database oleh operator. Endpoint hanya memakai `X-API-Key`.
7. **Retry job gagal** disediakan lewat `POST /api/v1/jobs/{id}/retry`, karena PRD menyebut developer/operator butuh cara retry job yang gagal.
8. **Hapus materi bersifat soft delete** (`deleted_at`). Materi tidak muncul di query, tapi data tetap tersimpan.

---

## 2. Base URL, Auth, dan Header Umum

**Base URL (placeholder):**
```
https://api.example.com
```

Semua endpoint API berada di prefix:
```
/api/v1
```

### Header Umum

| Header | Wajib | Keterangan |
|---|---|---|
| `Content-Type` | Ya, untuk request ber-body | `application/json` |
| `Accept` | Ya | `application/json` |
| `X-API-Key` | Ya, kecuali `/healthz` | API key milik tenant. Scope `admin` atau `query` sesuai endpoint |

### Scope API Key

| Scope | Endpoint yang boleh diakses |
|---|---|
| `admin` | Upload intent, job, video, retry, delete |
| `query` | Query RAG |

Jika scope tidak sesuai, API mengembalikan `403`.

---

## 3. Format Error Konsisten

Semua error response memakai format:

```json
{
  "error": {
    "code": "error_code",
    "message": "Deskripsi singkat."
  }
}
```

Contoh kode error yang dipakai:

| HTTP | `code` | Situasi |
|---|---|---|
| 400 | `invalid_request` | Request body tidak valid / field wajib kosong |
| 400 | `unsupported_content_type` | MIME type tidak diizinkan |
| 400 | `invalid_file_key` | `file_key` tidak cocok dengan upload intent |
| 400 | `expired_upload_intent` | Upload intent sudah kedaluwarsa |
| 400 | `upload_intent_consumed` | Upload intent sudah dipakai |
| 400 | `job_not_failed` | Retry hanya bisa dilakukan untuk job gagal |
| 400 | `question_required` | Field `question` kosong |
| 401 | `unauthorized` | API key tidak valid / tidak ditemukan / revoked |
| 403 | `forbidden` | API key tidak punya scope yang sesuai |
| 404 | `not_found` | Resource tidak ditemukan, termasuk resource milik tenant lain |
| 429 | `rate_limited` | Rate limit per API key / IP terlampaui |
| 500 | `internal_error` | Error internal server |

Contoh response error:

```json
{
  "error": {
    "code": "invalid_request",
    "message": "Field content_type wajib diisi."
  }
}
```

---

## 4. Endpoint

---

### 4.1. `POST /api/v1/upload-intents`

Membuat upload intent dan menghasilkan presigned URL untuk upload file langsung ke R2.

**Scope:** `admin`

**Headers:**
```
X-API-Key: <admin_key>
Content-Type: application/json
Accept: application/json
```

**Request Body:**

```json
{
  "content_type": "video/mp4"
}
```

**Daftar `content_type` yang diizinkan:**

| MIME type | `kind` | Ekstensi object key |
|---|---|---|
| `video/mp4` | `video` | `.mp4` |
| `video/quicktime` | `video` | `.mov` |
| `audio/mpeg` | `audio` | `.mp3` |
| `audio/wav` | `audio` | `.wav` |
| `application/pdf` | `pdf` | `.pdf` |

**Response — `201 Created`:**

```json
{
  "id": "0f8fad5b-d9cb-469f-a165-70867728950e",
  "tenant_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
  "file_key": "3fa85f64-5717-4562-b3fc-2c963f66afa6/0f8fad5b-d9cb-469f-a165-70867728950e/5f0a4d3e-8c22-4b38-9c2e-1c2b0f2b3d7a.mp4",
  "content_type": "video/mp4",
  "status": "issued",
  "presigned_url": "https://r2.example.com/...",
  "expires_at": "2026-06-01T12:10:00Z"
}
```

**Response — `400 Bad Request`:**

```json
{
  "error": {
    "code": "unsupported_content_type",
    "message": "Tipe konten tidak diizinkan."
  }
}
```

**Response — `401 Unauthorized`:**

```json
{
  "error": {
    "code": "unauthorized",
    "message": "API key tidak valid."
  }
}
```

**Response — `403 Forbidden`:**

```json
{
  "error": {
    "code": "forbidden",
    "message": "API key tidak memiliki akses ke resource ini."
  }
}
```

**Response — `429 Too Many Requests`:**

```json
{
  "error": {
    "code": "rate_limited",
    "message": "Terlalu banyak permintaan. Coba lagi nanti."
  }
}
```

**Response — `500 Internal Server Error`:**

```json
{
  "error": {
    "code": "internal_error",
    "message": "Terjadi kesalahan internal."
  }
}
```

---

### 4.2. Upload Langsung ke R2 via Presigned URL

Ini bukan endpoint API, tapi alur yang wajib dipahami:

1. Client menerima `presigned_url` dari `POST /api/v1/upload-intents`.
2. Client melakukan `PUT` file langsung ke `presigned_url`.
3. Header `Content-Type` saat upload harus sama dengan `content_type` dari upload intent.
4. Setelah upload selesai, client memanggil `POST /api/v1/jobs`.

---

### 4.3. `POST /api/v1/jobs`

Membuat job pemrosesan untuk file yang sudah diupload.

**Scope:** `admin`

**Headers:**
```
X-API-Key: <admin_key>
Content-Type: application/json
Accept: application/json
```

**Request Body:**

```json
{
  "file_key": "3fa85f64-5717-4562-b3fc-2c963f66afa6/0f8fad5b-d9cb-469f-a165-70867728950e/5f0a4d3e-8c22-4b38-9c2e-1c2b0f2b3d7a.mp4",
  "title": "Pengenalan HTML",
  "segments": ["web desain", "html dasar"]
}
```

**Aturan validasi:**

- `file_key` wajib cocok dengan upload intent milik tenant yang sama.
- Upload intent harus berstatus `issued` dan belum kedaluwarsa.
- `title` wajib diisi, maksimal 255 karakter.
- `segments` opsional, maksimal 50 item, tiap item maksimal 100 karakter.
- Backend otomatis menentukan `kind` dari `content_type` upload intent.

**Response — `201 Created`:**

```json
{
  "id": "b4c4a1e2-8f2d-4c7a-9b3e-1f2a6d7c8b9a",
  "tenant_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
  "upload_intent_id": "0f8fad5b-d9cb-469f-a165-70867728950e",
  "file_key": "3fa85f64-5717-4562-b3fc-2c963f66afa6/0f8fad5b-d9cb-469f-a165-70867728950e/5f0a4d3e-8c22-4b38-9c2e-1c2b0f2b3d7a.mp4",
  "title": "Pengenalan HTML",
  "kind": "video",
  "status": "pending",
  "stage": "queued",
  "segments": ["web desain", "html dasar"],
  "created_at": "2026-06-01T12:15:00Z",
  "updated_at": "2026-06-01T12:15:00Z"
}
```

**Response — `400 Bad Request`:**

```json
{
  "error": {
    "code": "expired_upload_intent",
    "message": "Upload intent sudah kedaluwarsa."
  }
}
```

**Response — `404 Not Found`** (jika `file_key` tidak dikenali atau bukan milik tenant ini):

```json
{
  "error": {
    "code": "not_found",
    "message": "Resource tidak ditemukan."
  }
}
```

Kode error lain yang mungkin: `401`, `403`, `429`, `500`.

---

### 4.4. `GET /api/v1/jobs`

Mendapatkan daftar job milik tenant.

**Scope:** `admin`

**Query Parameters:**

| Parameter | Tipe | Wajib | Keterangan |
|---|---|---|---|
| `status` | string | Tidak | Filter status: `pending`, `processing`, `completed`, `failed` |
| `page` | integer | Tidak | Default `1` |
| `limit` | integer | Tidak | Default `10`, maksimal `100` |

**Headers:**
```
X-API-Key: <admin_key>
Accept: application/json
```

**Response — `200 OK`:**

```json
{
  "data": [
    {
      "id": "b4c4a1e2-8f2d-4c7a-9b3e-1f2a6d7c8b9a",
      "tenant_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
      "upload_intent_id": "0f8fad5b-d9cb-469f-a165-70867728950e",
      "file_key": "3fa85f64-5717-4562-b3fc-2c963f66afa6/0f8fad5b-d9cb-469f-a165-70867728950e/5f0a4d3e-8c22-4b38-9c2e-1c2b0f2b3d7a.mp4",
      "title": "Pengenalan HTML",
      "kind": "video",
      "status": "processing",
      "stage": "transcribing",
      "error_message": null,
      "retry_count": 0,
      "segments": ["web desain", "html dasar"],
      "created_at": "2026-06-01T12:15:00Z",
      "updated_at": "2026-06-01T12:16:00Z"
    }
  ],
  "meta": {
    "page": 1,
    "limit": 10,
    "total": 1
  }
}
```

Kode error yang mungkin: `401`, `403`, `429`, `500`.

---

### 4.5. `GET /api/v1/jobs/{id}`

Mendapatkan detail satu job.

**Scope:** `admin`

**Path Parameter:**

| Parameter | Tipe | Keterangan |
|---|---|---|
| `id` | uuid | ID job |

**Headers:**
```
X-API-Key: <admin_key>
Accept: application/json
```

**Response — `200 OK`:**

```json
{
  "id": "b4c4a1e2-8f2d-4c7a-9b3e-1f2a6d7c8b9a",
  "tenant_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
  "upload_intent_id": "0f8fad5b-d9cb-469f-a165-70867728950e",
  "file_key": "3fa85f64-5717-4562-b3fc-2c963f66afa6/0f8fad5b-d9cb-469f-a165-70867728950e/5f0a4d3e-8c22-4b38-9c2e-1c2b0f2b3d7a.mp4",
  "title": "Pengenalan HTML",
  "kind": "video",
  "status": "failed",
  "stage": "transcribing",
  "error_message": "Transkripsi gagal: audio terlalu panjang.",
  "retry_count": 1,
  "segments": ["web desain", "html dasar"],
  "started_at": "2026-06-01T12:16:00Z",
  "finished_at": "2026-06-01T12:18:00Z",
  "created_at": "2026-06-01T12:15:00Z",
  "updated_at": "2026-06-01T12:18:00Z"
}
```

**Response — `404 Not Found`** (termasuk jika job milik tenant lain):

```json
{
  "error": {
    "code": "not_found",
    "message": "Resource tidak ditemukan."
  }
}
```

Kode error lain yang mungkin: `401`, `403`, `429`, `500`.

---

### 4.6. `POST /api/v1/jobs/{id}/retry`

Me-retry job yang gagal. Job direset ke status `pending` dan dimasukkan kembali ke antrian worker.

**Scope:** `admin`

**Path Parameter:**

| Parameter | Tipe | Keterangan |
|---|---|---|
| `id` | uuid | ID job |

**Headers:**
```
X-API-Key: <admin_key>
Accept: application/json
```

**Response — `200 OK`:**

```json
{
  "id": "b4c4a1e2-8f2d-4c7a-9b3e-1f2a6d7c8b9a",
  "tenant_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
  "upload_intent_id": "0f8fad5b-d9cb-469f-a165-70867728950e",
  "file_key": "3fa85f64-5717-4562-b3fc-2c963f66afa6/0f8fad5b-d9cb-469f-a165-70867728950e/5f0a4d3e-8c22-4b38-9c2e-1c2b0f2b3d7a.mp4",
  "title": "Pengenalan HTML",
  "kind": "video",
  "status": "pending",
  "stage": "queued",
  "error_message": null,
  "retry_count": 2,
  "segments": ["web desain", "html dasar"],
  "created_at": "2026-06-01T12:15:00Z",
  "updated_at": "2026-06-01T12:20:00Z"
}
```

**Response — `400 Bad Request`:**

```json
{
  "error": {
    "code": "job_not_failed",
    "message": "Hanya job dengan status failed yang bisa di-retry."
  }
}
```

**Response — `404 Not Found`:**

```json
{
  "error": {
    "code": "not_found",
    "message": "Resource tidak ditemukan."
  }
}
```

Kode error lain yang mungkin: `401`, `403`, `429`, `500`.

---

### 4.7. `GET /api/v1/videos`

Mendapatkan daftar materi milik tenant. Materi yang sudah di-soft-delete tidak pernah muncul.

**Scope:** `admin`

**Query Parameters:**

| Parameter | Tipe | Wajib | Keterangan |
|---|---|---|---|
| `segment` | string | Tidak | Filter berdasarkan nama segmen, case-insensitive |
| `status` | string | Tidak | Filter status: `processing`, `completed`, `failed` |
| `page` | integer | Tidak | Default `1` |
| `limit` | integer | Tidak | Default `10`, maksimal `100` |

**Headers:**
```
X-API-Key: <admin_key>
Accept: application/json
```

**Response — `200 OK`:**

```json
{
  "data": [
    {
      "id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
      "tenant_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
      "job_id": "b4c4a1e2-8f2d-4c7a-9b3e-1f2a6d7c8b9a",
      "title": "Pengenalan HTML",
      "kind": "video",
      "file_key": "3fa85f64-5717-4562-b3fc-2c963f66afa6/0f8fad5b-d9cb-469f-a165-70867728950e/5f0a4d3e-8c22-4b38-9c2e-1c2b0f2b3d7a.mp4",
      "status": "completed",
      "duration_seconds": 3600,
      "segments": ["web desain", "html dasar"],
      "created_at": "2026-06-01T12:15:00Z",
      "updated_at": "2026-06-01T12:25:00Z"
    }
  ],
  "meta": {
    "page": 1,
    "limit": 10,
    "total": 1
  }
}
```

Kode error yang mungkin: `401`, `403`, `429`, `500`.

---

### 4.8. `GET /api/v1/videos/{id}`

Mendapatkan detail materi, termasuk segmen dan summary.

**Scope:** `admin`

**Path Parameter:**

| Parameter | Tipe | Keterangan |
|---|---|---|
| `id` | uuid | ID video |

**Headers:**
```
X-API-Key: <admin_key>
Accept: application/json
```

**Response — `200 OK`:**

```json
{
  "id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "tenant_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
  "job_id": "b4c4a1e2-8f2d-4c7a-9b3e-1f2a6d7c8b9a",
  "title": "Pengenalan HTML",
  "kind": "video",
  "file_key": "3fa85f64-5717-4562-b3fc-2c963f66afa6/0f8fad5b-d9cb-469f-a165-70867728950e/5f0a4d3e-8c22-4b38-9c2e-1c2b0f2b3d7a.mp4",
  "status": "completed",
  "duration_seconds": 3600,
  "segments": ["web desain", "html dasar"],
  "summary": {
    "id": "f47ac10b-58cc-4372-a567-0e02b2c3d479",
    "status": "completed",
    "content": "Video ini menjelaskan dasar HTML, termasuk struktur dokumen, tag, dan elemen umum.",
    "language": "id",
    "model": "deepseek-chat",
    "created_at": "2026-06-01T12:24:00Z",
    "updated_at": "2026-06-01T12:24:00Z"
  },
  "created_at": "2026-06-01T12:15:00Z",
  "updated_at": "2026-06-01T12:25:00Z"
}
```

Jika summary belum tersedia, `summary` bernilai `null`.

**Response — `404 Not Found`** (termasuk jika video milik tenant lain atau sudah di-soft-delete):

```json
{
  "error": {
    "code": "not_found",
    "message": "Resource tidak ditemukan."
  }
}
```

Kode error lain yang mungkin: `401`, `403`, `429`, `500`.

---

### 4.9. `DELETE /api/v1/videos/{id}`

Menghapus materi secara soft delete. Materi tidak lagi muncul di query RAG, tetapi data tetap tersimpan di database.

**Scope:** `admin`

**Path Parameter:**

| Parameter | Tipe | Keterangan |
|---|---|---|
| `id` | uuid | ID video |

**Headers:**
```
X-API-Key: <admin_key>
Accept: application/json
```

**Response — `200 OK`:**

```json
{
  "id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "deleted_at": "2026-06-01T13:00:00Z"
}
```

**Response — `404 Not Found`** (termasuk jika video milik tenant lain atau sudah dihapus):

```json
{
  "error": {
    "code": "not_found",
    "message": "Resource tidak ditemukan."
  }
}
```

Kode error lain yang mungkin: `401`, `403`, `429`, `500`.

---

### 4.10. `POST /api/v1/query`

Menanyakan knowledge base dengan bahasa natural. Jawaban dihasilkan dari chunk materi yang relevan, plus referensi segmen.

**Scope:** `query`

**Headers:**
```
X-API-Key: <query_key>
Content-Type: application/json
Accept: application/json
```

**Request Body:**

```json
{
  "question": "Apa itu tag HTML?",
  "segment": "web desain"
}
```

**Aturan validasi:**

- `question` wajib diisi, maksimal 1000 karakter.
- `segment` opsional, string. Jika diisi, pencarian dibatasi ke materi yang memiliki segmen tersebut.

**Response — `200 OK`:**

```json
{
  "answer": "Tag HTML adalah elemen dasar yang digunakan untuk menyusun struktur halaman web, misalnya <html>, <head>, dan <body>.",
  "references": [
    {
      "video_id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
      "video_title": "Pengenalan HTML",
      "chunk_index": 3,
      "snippet": "Tag HTML adalah elemen dasar yang digunakan untuk menyusun struktur halaman web...",
      "segments": ["web desain", "html dasar"]
    }
  ]
}
```

Jika tidak ada materi relevan:

```json
{
  "answer": null,
  "references": []
}
```

**Response — `400 Bad Request`:**

```json
{
  "error": {
    "code": "question_required",
    "message": "Field question wajib diisi."
  }
}
```

**Response — `401 Unauthorized`:**

```json
{
  "error": {
    "code": "unauthorized",
    "message": "API key tidak valid."
  }
}
```

**Response — `403 Forbidden`:**

```json
{
  "error": {
    "code": "forbidden",
    "message": "API key tidak memiliki akses ke resource ini."
  }
}
```

**Response — `429 Too Many Requests`:**

```json
{
  "error": {
    "code": "rate_limited",
    "message": "Terlalu banyak permintaan. Coba lagi nanti."
  }
}
```

**Response — `500 Internal Server Error`:**

```json
{
  "error": {
    "code": "internal_error",
    "message": "Terjadi kesalahan internal."
  }
}
```

---

### 4.11. `GET /healthz`

Health check untuk API server.

**Auth:** Tidak diperlukan.

**Headers:**
```
Accept: application/json
```

**Response — `200 OK`:**

```json
{
  "status": "ok",
  "service": "api",
  "time": "2026-06-01T12:00:00Z",
  "db": "up"
}
```

Jika database bermasalah, API mengembalikan `503 Service Unavailable` dengan format error yang sama.

---

## 5. Ringkasan Endpoint

| Method | Path | Scope | Deskripsi |
|---|---|---|---|
| `POST` | `/api/v1/upload-intents` | `admin` | Generate presigned URL upload |
| `POST` | `/api/v1/jobs` | `admin` | Buat job pemrosesan materi |
| `GET` | `/api/v1/jobs` | `admin` | List job |
| `GET` | `/api/v1/jobs/{id}` | `admin` | Detail job |
| `POST` | `/api/v1/jobs/{id}/retry` | `admin` | Retry job gagal |
| `GET` | `/api/v1/videos` | `admin` | List materi |
| `GET` | `/api/v1/videos/{id}` | `admin` | Detail materi + summary |
| `DELETE` | `/api/v1/videos/{id}` | `admin` | Soft delete materi |
| `POST` | `/api/v1/query` | `query` | Query RAG dengan referensi segmen |
| `GET` | `/healthz` | Publik | Health check API server |