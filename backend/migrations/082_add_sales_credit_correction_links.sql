-- Adds audit links for the fail-closed legacy sales credit correction flow.
-- This migration changes schema only: it never modifies balances, sales,
-- payments, allocations, or existing journals.

ALTER TABLE posted_transaction_corrections
  ADD COLUMN IF NOT EXISTS sale_id BIGINT,
  ADD COLUMN IF NOT EXISTS linked_payment_id BIGINT;

CREATE INDEX IF NOT EXISTS idx_posted_transaction_corrections_sale_id
  ON posted_transaction_corrections (sale_id)
  WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_posted_transaction_corrections_linked_payment_id
  ON posted_transaction_corrections (linked_payment_id)
  WHERE deleted_at IS NULL;
