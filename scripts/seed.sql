-- scripts/seed.sql
--
-- Development/demo seed for the RAG pipeline. API keys are managed here via SQL,
-- not via the API, per the technical assumptions in the LLD.
--
-- Generating a SHA-256 hex hash for a plaintext key:
--
--     printf 'rag-admin-secret-1' | sha256sum
--     -> 1b6f98d5f3d4f92bc9bd61d5d3afa8d94d5bda3d4f92bc9bd61d5d3afa8d94d5  -
--
-- Copy the 64-character hex digest (everything before the space/hyphen) into the
-- key_hash column below. The plaintext key is printed to the developer and then
-- discarded — only the hash is ever stored.
--
-- Example: session
--
--     \i scripts/seed.sql
--
-- The script is idempotent: it only inserts a tenant when its slug is absent, and
-- only inserts an API key when its key_hash is absent. Re-running is safe.

-- ---------------------------------------------------------------------------
-- Tenant: Acme Education (demo)
-- ---------------------------------------------------------------------------
INSERT INTO tenants (id, name, slug)
SELECT gen_random_uuid(), 'Acme Education', 'acme'
WHERE NOT EXISTS (SELECT 1 FROM tenants WHERE slug = 'acme');

-- ---------------------------------------------------------------------------
-- API key: admin scope (upload-intent / job / video management)
-- plaintext:  rag-admin-secret-1
-- hash:       <REPLACE WITH `printf 'rag-admin-secret-1' | sha256sum`>
-- ---------------------------------------------------------------------------
INSERT INTO api_keys (id, tenant_id, name, key_hash, scope)
SELECT
    gen_random_uuid(),
    (SELECT id FROM tenants WHERE slug = 'acme'),
    'acme-admin',
    '<admin-key-sha256-hex-64-char>',
    'admin'
WHERE NOT EXISTS (
    SELECT 1 FROM api_keys
    WHERE key_hash = '<admin-key-sha256-hex-64-char>'
);

-- ---------------------------------------------------------------------------
-- API key: query scope (POST /api/v1/query)
-- plaintext:  rag-query-secret-1
-- hash:       <REPLACE WITH `printf 'rag-query-secret-1' | sha256sum`>
-- ---------------------------------------------------------------------------
INSERT INTO api_keys (id, tenant_id, name, key_hash, scope)
SELECT
    gen_random_uuid(),
    (SELECT id FROM tenants WHERE slug = 'acme'),
    'acme-query',
    '<query-key-sha256-hex-64-char>',
    'query'
WHERE NOT EXISTS (
    SELECT 1 FROM api_keys
    WHERE key_hash = '<query-key-sha256-hex-64-char>'
);
