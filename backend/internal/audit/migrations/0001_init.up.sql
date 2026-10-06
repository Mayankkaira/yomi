CREATE SCHEMA IF NOT EXISTS audit;

CREATE TABLE IF NOT EXISTS audit.audit_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sequence_no BIGSERIAL UNIQUE,
    event_type TEXT NOT NULL,
    citizen_id UUID,
    institution_id UUID,
    request_id UUID,
    metadata JSONB,
    prev_hash TEXT,
    entry_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_audit_citizen ON audit.audit_log (citizen_id);
CREATE INDEX IF NOT EXISTS idx_audit_institution ON audit.audit_log (institution_id);
CREATE INDEX IF NOT EXISTS idx_audit_sequence ON audit.audit_log (sequence_no);