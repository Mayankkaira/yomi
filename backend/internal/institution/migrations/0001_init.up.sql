CREATE SCHEMA IF NOT EXISTS institution;

CREATE TABLE IF NOT EXISTS institution.institutions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    sector TEXT NOT NULL,
    api_key_hash TEXT NOT NULL,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_institutions_api_key ON institution.institutions (api_key_hash);