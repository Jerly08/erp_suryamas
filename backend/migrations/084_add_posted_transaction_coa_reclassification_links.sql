-- Schema only: supports the audited COA reclassification workflow for posted
-- purchases. This migration does not update balances, transactions, journals,
-- cash/bank movements, or any historical records.

ALTER TABLE posted_transaction_corrections
  ADD COLUMN IF NOT EXISTS original_journal_line_id BIGINT,
  ADD COLUMN IF NOT EXISTS original_account_id BIGINT;

CREATE INDEX IF NOT EXISTS idx_posted_transaction_corrections_original_journal_line_id
  ON posted_transaction_corrections (original_journal_line_id)
  WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_posted_transaction_corrections_original_account_id
  ON posted_transaction_corrections (original_account_id)
  WHERE deleted_at IS NULL;
