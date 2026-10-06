CREATE SCHEMA IF NOT EXISTS auth;

CREATE TABLE IF NOT EXISTS auth.citizens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    aadhaar_id VARCHAR(12) UNIQUE NOT NULL,
    full_name TEXT NOT NULL,
    email TEXT UNIQUE NOT NULL,
    phone TEXT,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_citizens_aadhaar ON auth.citizens (aadhaar_id);