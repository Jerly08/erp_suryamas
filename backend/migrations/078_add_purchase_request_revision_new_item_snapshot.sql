-- Preserve the exact item list after a requester revision so additions and
-- removals remain auditable even after the PR is revised again later.
ALTER TABLE purchase_request_revision_snapshots
ADD COLUMN IF NOT EXISTS new_item_snapshot TEXT;
