-- Immutable, append-only business audit events for changes made while a
-- purchase is open for a pre-posting Finance Staff revision.
CREATE TABLE IF NOT EXISTS purchase_pre_posting_revision_changes (
    id BIGSERIAL PRIMARY KEY,
    purchase_pre_posting_revision_id BIGINT NOT NULL,
    purchase_id BIGINT NOT NULL,
    changed_by BIGINT NOT NULL,
    changed_fields TEXT NOT NULL DEFAULT '',
    before_snapshot TEXT NOT NULL DEFAULT '',
    after_snapshot TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_purchase_pre_posting_revision_changes_revision_created
    ON purchase_pre_posting_revision_changes (purchase_pre_posting_revision_id, created_at, id);

CREATE INDEX IF NOT EXISTS idx_purchase_pre_posting_revision_changes_purchase_created
    ON purchase_pre_posting_revision_changes (purchase_id, created_at, id);
