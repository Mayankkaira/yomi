ALTER TABLE consent.consent_requests ADD COLUMN IF NOT EXISTS auth_token TEXT;
ALTER TABLE consent.consent_requests DROP COLUMN IF EXISTS access_token_hash;
ALTER TABLE consent.consent_requests DROP COLUMN IF EXISTS institution_role;
DROP INDEX IF EXISTS consent.idx_consent_token_hash;