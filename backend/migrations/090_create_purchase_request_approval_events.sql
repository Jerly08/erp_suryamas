-- Immutable approval audit trail for Purchase Requests.
-- This migration is additive: it does not alter existing PR approval data or
-- backfill inferred history. Existing values are snapshotted only when a PR is
-- subsequently routed for another approval round.

CREATE TABLE IF NOT EXISTS purchase_request_approval_events (
    id BIGSERIAL PRIMARY KEY,
    purchase_request_id BIGINT NOT NULL,
    approval_revision INTEGER NOT NULL DEFAULT 1,
    event_type VARCHAR(50) NOT NULL,
    stage VARCHAR(40) NOT NULL DEFAULT '',
    action VARCHAR(50) NOT NULL,
    notes TEXT NOT NULL DEFAULT '',
    reapproval_reason VARCHAR(50) NOT NULL DEFAULT '',
    actor_id BIGINT NULL,
    status_before VARCHAR(30) NOT NULL DEFAULT '',
    status_after VARCHAR(30) NOT NULL DEFAULT '',
    approval_stage_before VARCHAR(40) NOT NULL DEFAULT '',
    approval_stage_after VARCHAR(40) NOT NULL DEFAULT '',
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    source VARCHAR(40) NOT NULL DEFAULT 'LIVE_ACTION',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_pr_approval_events_request_revision_occurred
    ON purchase_request_approval_events (purchase_request_id, approval_revision, occurred_at, id);

CREATE INDEX IF NOT EXISTS idx_pr_approval_events_actor_occurred
    ON purchase_request_approval_events (actor_id, occurred_at, id);

-- Revision snapshots already exist and are now used for all reroute reasons.
-- These columns retain the review context that used to live only on the
-- mutable purchase_requests row.
ALTER TABLE purchase_request_revision_snapshots
    ADD COLUMN IF NOT EXISTS finance_staff_review_notes TEXT NOT NULL DEFAULT '';

ALTER TABLE purchase_request_revision_snapshots
    ADD COLUMN IF NOT EXISTS finance_manager_review_notes TEXT NOT NULL DEFAULT '';

ALTER TABLE purchase_request_revision_snapshots
    ADD COLUMN IF NOT EXISTS rejection_reason TEXT NOT NULL DEFAULT '';
