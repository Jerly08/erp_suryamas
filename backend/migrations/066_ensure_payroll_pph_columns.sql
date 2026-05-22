-- Ensure payroll PPh settlement columns exist on staging/production databases.
-- Older staging data can have payroll_runs without these fields, which blocks
-- Finance Manager approval when the approval flow records PPh payable journals.

ALTER TABLE payroll_runs
  ADD COLUMN IF NOT EXISTS pph_payment_status VARCHAR(20) NOT NULL DEFAULT 'UNPAID',
  ADD COLUMN IF NOT EXISTS pph_paid_amount DECIMAL(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS pph_recognition_date TIMESTAMP NULL,
  ADD COLUMN IF NOT EXISTS pph_payment_date TIMESTAMP NULL,
  ADD COLUMN IF NOT EXISTS pph_payment_reference VARCHAR(100) DEFAULT '',
  ADD COLUMN IF NOT EXISTS pph_payment_notes TEXT DEFAULT '',
  ADD COLUMN IF NOT EXISTS pph_payable_journal_entry_id BIGINT NULL,
  ADD COLUMN IF NOT EXISTS pph_payment_journal_entry_id BIGINT NULL;

UPDATE payroll_runs
SET
  pph_paid_amount = COALESCE(pph_paid_amount, 0),
  pph_recognition_date = COALESCE(pph_recognition_date, period_end),
  pph_payment_reference = COALESCE(pph_payment_reference, ''),
  pph_payment_notes = COALESCE(pph_payment_notes, ''),
  pph_payment_status = CASE
    WHEN COALESCE(total_pph, 0) <= 0.01 THEN 'PAID'
    WHEN COALESCE(pph_paid_amount, 0) >= COALESCE(total_pph, 0) - 0.01 THEN 'PAID'
    WHEN COALESCE(pph_paid_amount, 0) > 0.01 THEN 'PARTIAL_PAID'
    ELSE 'UNPAID'
  END;

CREATE INDEX IF NOT EXISTS idx_payroll_runs_pph_payment_status ON payroll_runs(pph_payment_status);
CREATE INDEX IF NOT EXISTS idx_payroll_runs_pph_payable_journal_entry_id ON payroll_runs(pph_payable_journal_entry_id);
CREATE INDEX IF NOT EXISTS idx_payroll_runs_pph_payment_journal_entry_id ON payroll_runs(pph_payment_journal_entry_id);
