-- Split payroll BPJS components into employer-paid and employee-paid portions.
-- Employee-paid portions reduce net salary; employer-paid portions are company
-- expenses and still settle together through Utang BPJS & Potongan Payroll.

ALTER TABLE payroll_runs
  ADD COLUMN IF NOT EXISTS total_bpjs_employer DECIMAL(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS total_bpjs_employee DECIMAL(15,2) DEFAULT 0;

ALTER TABLE payroll_items
  ADD COLUMN IF NOT EXISTS bpjs_tk_jht_employer DECIMAL(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS bpjs_tk_jht_employee DECIMAL(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS bpjs_tk_jp_employer DECIMAL(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS bpjs_tk_jp_employee DECIMAL(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS bpjs_tk_jkk_employer DECIMAL(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS bpjs_tk_jkk_employee DECIMAL(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS bpjs_tk_jkm_employer DECIMAL(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS bpjs_tk_jkm_employee DECIMAL(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS bpjs_kesehatan_employer DECIMAL(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS bpjs_kesehatan_employee DECIMAL(15,2) DEFAULT 0;

UPDATE payroll_items
SET
  bpjs_tk_jht_employee = COALESCE(NULLIF(bpjs_tk_jht_employee, 0), COALESCE(bpjstkjht, 0)),
  bpjs_tk_jp_employee = COALESCE(NULLIF(bpjs_tk_jp_employee, 0), COALESCE(bpjstkjp, 0)),
  bpjs_kesehatan_employee = COALESCE(NULLIF(bpjs_kesehatan_employee, 0), COALESCE(bpjs_kesehatan, 0)),
  bpjs_tk_jht_employer = COALESCE(bpjs_tk_jht_employer, 0),
  bpjs_tk_jp_employer = COALESCE(bpjs_tk_jp_employer, 0),
  bpjs_tk_jkk_employer = COALESCE(bpjs_tk_jkk_employer, 0),
  bpjs_tk_jkk_employee = COALESCE(bpjs_tk_jkk_employee, 0),
  bpjs_tk_jkm_employer = COALESCE(bpjs_tk_jkm_employer, 0),
  bpjs_tk_jkm_employee = COALESCE(bpjs_tk_jkm_employee, 0),
  bpjs_kesehatan_employer = COALESCE(bpjs_kesehatan_employer, 0);

UPDATE payroll_items
SET bpjs = COALESCE(bpjs_tk_jht_employer, 0)
  + COALESCE(bpjs_tk_jht_employee, 0)
  + COALESCE(bpjs_tk_jp_employer, 0)
  + COALESCE(bpjs_tk_jp_employee, 0)
  + COALESCE(bpjs_tk_jkk_employer, 0)
  + COALESCE(bpjs_tk_jkk_employee, 0)
  + COALESCE(bpjs_tk_jkm_employer, 0)
  + COALESCE(bpjs_tk_jkm_employee, 0)
  + COALESCE(bpjs_kesehatan_employer, 0)
  + COALESCE(bpjs_kesehatan_employee, 0);

UPDATE payroll_runs pr
SET
  total_bpjs_employer = COALESCE(t.total_bpjs_employer, 0),
  total_bpjs_employee = COALESCE(t.total_bpjs_employee, 0),
  total_bpjs = COALESCE(t.total_bpjs, COALESCE(pr.total_bpjs, 0))
FROM (
  SELECT
    payroll_run_id,
    SUM(COALESCE(bpjs_tk_jht_employer, 0)
      + COALESCE(bpjs_tk_jp_employer, 0)
      + COALESCE(bpjs_tk_jkk_employer, 0)
      + COALESCE(bpjs_tk_jkm_employer, 0)
      + COALESCE(bpjs_kesehatan_employer, 0)) AS total_bpjs_employer,
    SUM(COALESCE(bpjs_tk_jht_employee, 0)
      + COALESCE(bpjs_tk_jp_employee, 0)
      + COALESCE(bpjs_tk_jkk_employee, 0)
      + COALESCE(bpjs_tk_jkm_employee, 0)
      + COALESCE(bpjs_kesehatan_employee, 0)) AS total_bpjs_employee,
    SUM(COALESCE(bpjs, 0)) AS total_bpjs
  FROM payroll_items
  WHERE deleted_at IS NULL
  GROUP BY payroll_run_id
) t
WHERE pr.id = t.payroll_run_id;

UPDATE payroll_runs
SET
  total_bpjs_employer = COALESCE(total_bpjs_employer, 0),
  total_bpjs_employee = COALESCE(NULLIF(total_bpjs_employee, 0), COALESCE(total_bpjs, 0));

DO $$
DECLARE
  operating_parent_id BIGINT;
BEGIN
  SELECT id INTO operating_parent_id
  FROM accounts
  WHERE code = '6000' AND deleted_at IS NULL
  ORDER BY id ASC
  LIMIT 1;

  IF NOT EXISTS (
    SELECT 1 FROM accounts WHERE code = '6107' AND deleted_at IS NULL
  ) THEN
    INSERT INTO accounts (
      code, name, type, category, parent_id, level, is_header, is_active, balance, created_at, updated_at
    )
    VALUES (
      '6107', 'BEBAN BPJS PEMBERI KERJA', 'EXPENSE', 'OPERATING_EXPENSE', operating_parent_id, 2, false, true, 0, NOW(), NOW()
    );
  END IF;
END $$;
