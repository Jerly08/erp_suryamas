CREATE TABLE IF NOT EXISTS purchase_coa_revisions (
    id BIGSERIAL PRIMARY KEY,
    purchase_id BIGINT NOT NULL,
    status VARCHAR(40) NOT NULL,
    reason TEXT NOT NULL,
    coa_snapshot TEXT NOT NULL DEFAULT '',
    requested_by BIGINT NOT NULL,
    requested_at TIMESTAMPTZ NOT NULL,
    finance_manager_decision_notes TEXT NOT NULL DEFAULT '',
    finance_manager_decision_by BIGINT NULL,
    finance_manager_decision_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ NULL
);

CREATE INDEX IF NOT EXISTS idx_purchase_coa_revisions_purchase_active
    ON purchase_coa_revisions (purchase_id, status, created_at DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_purchase_coa_revisions_requested_by
    ON purchase_coa_revisions (requested_by)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_purchase_coa_revisions_finance_manager_decision_by
    ON purchase_coa_revisions (finance_manager_decision_by)
    WHERE deleted_at IS NULL;
