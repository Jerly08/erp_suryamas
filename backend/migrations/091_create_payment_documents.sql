-- Supporting document history for payment transactions.
-- This migration is additive only: it creates independent attachment/audit
-- tables and does not alter existing payments, allocations, cash/bank values,
-- or journal entries.

CREATE TABLE IF NOT EXISTS payment_documents (
    id BIGSERIAL PRIMARY KEY,
    payment_id BIGINT NOT NULL,
    document_type VARCHAR(50) NOT NULL DEFAULT 'SUPPORTING_DOCUMENT',
    reference_number VARCHAR(100) NOT NULL DEFAULT '',
    file_name VARCHAR(255) NOT NULL,
    file_path VARCHAR(500) NOT NULL,
    file_size BIGINT NOT NULL DEFAULT 0,
    mime_type VARCHAR(100) NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    status VARCHAR(30) NOT NULL DEFAULT 'ACTIVE',
    uploaded_by BIGINT NOT NULL,
    invalidated_by BIGINT NULL,
    invalidated_at TIMESTAMPTZ NULL,
    invalidation_reason TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ NULL
);

CREATE INDEX IF NOT EXISTS idx_payment_documents_payment_created
    ON payment_documents (payment_id, created_at, id)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_payment_documents_payment_status
    ON payment_documents (payment_id, status, id)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_payment_documents_uploaded_by_created
    ON payment_documents (uploaded_by, created_at, id)
    WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS payment_document_events (
    id BIGSERIAL PRIMARY KEY,
    payment_document_id BIGINT NOT NULL,
    payment_id BIGINT NOT NULL,
    action VARCHAR(50) NOT NULL,
    notes TEXT NOT NULL DEFAULT '',
    actor_id BIGINT NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_payment_document_events_document_occurred
    ON payment_document_events (payment_document_id, occurred_at, id);

CREATE INDEX IF NOT EXISTS idx_payment_document_events_payment_occurred
    ON payment_document_events (payment_id, occurred_at, id);
