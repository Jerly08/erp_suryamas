-- Preserve the source-line audit trail for batch COA reclassifications.
-- This is schema-only: existing corrections and journals are not modified.
CREATE TABLE IF NOT EXISTS posted_transaction_correction_reclassification_lines (
    id BIGSERIAL PRIMARY KEY,
    correction_id BIGINT NOT NULL REFERENCES posted_transaction_corrections(id) ON DELETE CASCADE,
    original_journal_line_id BIGINT NOT NULL,
    original_account_id BIGINT NOT NULL,
    proposed_account_id BIGINT NOT NULL,
    line_number INTEGER NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    debit_amount NUMERIC(20,2) NOT NULL DEFAULT 0,
    credit_amount NUMERIC(20,2) NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_posted_correction_reclassification_line_amount
        CHECK (
            (debit_amount > 0 AND credit_amount = 0)
            OR (credit_amount > 0 AND debit_amount = 0)
        )
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_posted_correction_reclass_source_line
    ON posted_transaction_correction_reclassification_lines (correction_id, original_journal_line_id);

CREATE INDEX IF NOT EXISTS idx_posted_correction_reclass_correction
    ON posted_transaction_correction_reclassification_lines (correction_id);

CREATE INDEX IF NOT EXISTS idx_posted_correction_reclass_journal_line
    ON posted_transaction_correction_reclassification_lines (original_journal_line_id);
