-- Ensure payroll PPh settlement columns exist on staging/production databases.
-- Legacy databases may have GORM-generated PPh columns such as total_p_ph and
-- p_ph_paid_amount. The application API uses total_pph and pph_* names, so this
-- migration creates the canonical columns and copies legacy values forward.

ALTER TABLE payroll_runs
  ADD COLUMN IF NOT EXISTS total_pph DECIMAL(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS pph_payment_status VARCHAR(20) NOT NULL DEFAULT 'UNPAID',
  ADD COLUMN IF NOT EXISTS pph_paid_amount DECIMAL(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS pph_recognition_date TIMESTAMP NULL,
  ADD COLUMN IF NOT EXISTS pph_payment_date TIMESTAMP NULL,
  ADD COLUMN IF NOT EXISTS pph_payment_reference VARCHAR(100) DEFAULT '',
  ADD COLUMN IF NOT EXISTS pph_payment_notes TEXT DEFAULT '',
  ADD COLUMN IF NOT EXISTS pph_payable_journal_entry_id BIGINT NULL,
  ADD COLUMN IF NOT EXISTS pph_payment_journal_entry_id BIGINT NULL;

DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_name = 'payroll_runs' AND column_name = 'total_p_ph'
  ) THEN
    EXECUTE 'UPDATE payroll_runs SET total_pph = COALESCE(NULLIF(total_pph, 0), total_p_ph, 0)';
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_name = 'payroll_runs' AND column_name = 'p_ph_paid_amount'
  ) THEN
    EXECUTE 'UPDATE payroll_runs SET pph_paid_amount = CASE WHEN COALESCE(pph_paid_amount, 0) <= 0.01 THEN COALESCE(p_ph_paid_amount, 0) ELSE pph_paid_amount END';
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_name = 'payroll_runs' AND column_name = 'p_ph_recognition_date'
  ) THEN
    EXECUTE 'UPDATE payroll_runs SET pph_recognition_date = COALESCE(pph_recognition_date, p_ph_recognition_date)';
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_name = 'payroll_runs' AND column_name = 'p_ph_payment_date'
  ) THEN
    EXECUTE 'UPDATE payroll_runs SET pph_payment_date = COALESCE(pph_payment_date, p_ph_payment_date)';
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_name = 'payroll_runs' AND column_name = 'p_ph_payment_reference'
  ) THEN
    EXECUTE 'UPDATE payroll_runs SET pph_payment_reference = COALESCE(NULLIF(pph_payment_reference, ''''), p_ph_payment_reference, '''')';
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_name = 'payroll_runs' AND column_name = 'p_ph_payment_notes'
  ) THEN
    EXECUTE 'UPDATE payroll_runs SET pph_payment_notes = COALESCE(NULLIF(pph_payment_notes, ''''), p_ph_payment_notes, '''')';
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_name = 'payroll_runs' AND column_name = 'p_ph_payable_journal_entry_id'
  ) THEN
    EXECUTE 'UPDATE payroll_runs SET pph_payable_journal_entry_id = COALESCE(pph_payable_journal_entry_id, p_ph_payable_journal_entry_id)';
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_name = 'payroll_runs' AND column_name = 'p_ph_payment_journal_entry_id'
  ) THEN
    EXECUTE 'UPDATE payroll_runs SET pph_payment_journal_entry_id = COALESCE(pph_payment_journal_entry_id, p_ph_payment_journal_entry_id)';
  END IF;
END $$;

UPDATE payroll_runs
SET
  total_pph = COALESCE(total_pph, 0),
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
