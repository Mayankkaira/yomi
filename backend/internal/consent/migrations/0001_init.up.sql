CREATE SCHEMA IF NOT EXISTS consent;

CREATE TABLE IF NOT EXISTS consent.consent_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    citizen_id UUID NOT NULL,
    institution_id UUID NOT NULL,
    data_scope TEXT[] NOT NULL,
    purpose TEXT NOT NULL,
    status VARCHAR(20) DEFAULT 'pending',
    auth_token TEXT,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_consent_citizen ON consent.consent_requests (citizen_id);
CREATE INDEX IF NOT EXISTS idx_consent_institution ON consent.consent_requests (institution_id);
CREATE INDEX IF NOT EXISTS idx_consent_status ON consent.consent_requests (status);