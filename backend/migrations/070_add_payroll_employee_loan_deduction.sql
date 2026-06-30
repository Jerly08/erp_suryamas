-- Add employee loan deduction support to payroll imports and accrual journals.
-- The deduction reduces net salary and credits the employee-loan receivable COA.

ALTER TABLE payroll_runs
  ADD COLUMN IF NOT EXISTS total_employee_loan_deduction DECIMAL(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS employee_loan_account_id BIGINT NULL;

ALTER TABLE payroll_items
  ADD COLUMN IF NOT EXISTS employee_loan_deduction DECIMAL(15,2) DEFAULT 0;

UPDATE payroll_runs
SET total_employee_loan_deduction = COALESCE(total_employee_loan_deduction, 0);

UPDATE payroll_items
SET employee_loan_deduction = COALESCE(employee_loan_deduction, 0);

DO $$
DECLARE
  parent_current_asset_id BIGINT;
  employee_receivable_parent_id BIGINT;
BEGIN
  SELECT id INTO parent_current_asset_id
  FROM accounts
  WHERE code = '1100' AND deleted_at IS NULL
  ORDER BY id ASC
  LIMIT 1;

  SELECT id INTO employee_receivable_parent_id
  FROM accounts
  WHERE code = '1107' AND deleted_at IS NULL
  ORDER BY id ASC
  LIMIT 1;

  IF employee_receivable_parent_id IS NULL THEN
    INSERT INTO accounts (
      code, name, type, category, parent_id, level, is_header, is_active, balance, created_at, updated_at
    )
    VALUES (
      '1107', 'PIUTANG LAIN-LAIN', 'ASSET', 'CURRENT_ASSET', parent_current_asset_id, 3, true, true, 0, NOW(), NOW()
    )
    RETURNING id INTO employee_receivable_parent_id;
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM accounts WHERE code = '1107-001' AND deleted_at IS NULL
  ) THEN
    INSERT INTO accounts (
      code, name, type, category, parent_id, level, is_header, is_active, balance, created_at, updated_at
    )
    VALUES (
      '1107-001', 'PINJAMAN KARYAWAN', 'ASSET', 'CURRENT_ASSET', employee_receivable_parent_id, 4, false, true, 0, NOW(), NOW()
    );
  END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_payroll_runs_employee_loan_account_id ON payroll_runs(employee_loan_account_id);
