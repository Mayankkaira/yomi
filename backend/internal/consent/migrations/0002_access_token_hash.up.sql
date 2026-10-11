ALTER TABLE consent.consent_requests ADD COLUMN IF NOT EXISTS access_token_hash TEXT;
ALTER TABLE consent.consent_requests ADD COLUMN IF NOT EXISTS institution_role VARCHAR(20) NOT NULL DEFAULT 'viewer';
UPDATE consent.consent_requests SET access_token_hash = NULL;
ALTER TABLE consent.consent_requests DROP COLUMN IF EXISTS auth_token;
CREATE INDEX IF NOT EXISTS idx_consent_token_hash ON consent.consent_requests (access_token_hash);