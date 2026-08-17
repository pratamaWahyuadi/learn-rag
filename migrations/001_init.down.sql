-- 001_init.down.sql

-- Drop RLS policies
DROP POLICY IF EXISTS summaries_isolation ON summaries;
DROP POLICY IF EXISTS chunks_isolation ON chunks;
DROP POLICY IF EXISTS transcripts_isolation ON transcripts;
DROP POLICY IF EXISTS video_segments_isolation ON video_segments;
DROP POLICY IF EXISTS videos_isolation ON videos;
DROP POLICY IF EXISTS job_segments_isolation ON job_segments;
DROP POLICY IF EXISTS segments_isolation ON segments;
DROP POLICY IF EXISTS jobs_isolation ON jobs;
DROP POLICY IF EXISTS upload_intents_isolation ON upload_intents;

-- Disable RLS on tenant data tables
ALTER TABLE summaries DISABLE ROW LEVEL SECURITY;
ALTER TABLE chunks DISABLE ROW LEVEL SECURITY;
ALTER TABLE transcripts DISABLE ROW LEVEL SECURITY;
ALTER TABLE video_segments DISABLE ROW LEVEL SECURITY;
ALTER TABLE videos DISABLE ROW LEVEL SECURITY;
ALTER TABLE job_segments DISABLE ROW LEVEL SECURITY;
ALTER TABLE segments DISABLE ROW LEVEL SECURITY;
ALTER TABLE jobs DISABLE ROW LEVEL SECURITY;
ALTER TABLE upload_intents DISABLE ROW LEVEL SECURITY;

-- Drop job notify trigger and function
DROP TRIGGER IF EXISTS trg_jobs_notify ON jobs;
DROP FUNCTION IF EXISTS notify_job_created();

-- Drop updated_at triggers before their tables are dropped
DROP TRIGGER IF EXISTS trg_summaries_updated_at ON summaries;
DROP TRIGGER IF EXISTS trg_videos_updated_at ON videos;
DROP TRIGGER IF EXISTS trg_jobs_updated_at ON jobs;
DROP TRIGGER IF EXISTS trg_api_keys_updated_at ON api_keys;
DROP TRIGGER IF EXISTS trg_tenants_updated_at ON tenants;
DROP FUNCTION IF EXISTS set_updated_at();

-- Drop tables in dependency-safe order
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS summaries;
DROP TABLE IF EXISTS chunks;
DROP TABLE IF EXISTS transcripts;
DROP TABLE IF EXISTS video_segments;
DROP TABLE IF EXISTS videos;
DROP TABLE IF EXISTS job_segments;
DROP TABLE IF EXISTS segments;
DROP TABLE IF EXISTS jobs;
DROP TABLE IF EXISTS upload_intents;
DROP TABLE IF EXISTS api_keys;
DROP TABLE IF EXISTS tenants;

-- Drop extensions
DROP EXTENSION IF EXISTS vector;
DROP EXTENSION IF EXISTS pgcrypto;
