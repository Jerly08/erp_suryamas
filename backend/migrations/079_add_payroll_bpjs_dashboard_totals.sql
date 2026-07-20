-- Add dashboard-friendly BPJS totals split between BPJS Ketenagakerjaan and BPJS Kesehatan.

ALTER TABLE payroll_runs
ADD COLUMN IF NOT EXISTS total_bpjs_ketenagakerjaan DECIMAL(15,2) DEFAULT 0,
ADD COLUMN IF NOT EXISTS total_bpjs_kesehatan DECIMAL(15,2) DEFAULT 0;

WITH item_totals AS (
  SELECT
    payroll_run_id,
    SUM(
      COALESCE(bpjs_tk_jht_employer, 0) +
      CASE WHEN COALESCE(bpjs_tk_jht_employee, 0) > 0 THEN COALESCE(bpjs_tk_jht_employee, 0) ELSE COALESCE(bpjstkjht, 0) END +
      COALESCE(bpjs_tk_jp_employer, 0) +
      CASE WHEN COALESCE(bpjs_tk_jp_employee, 0) > 0 THEN COALESCE(bpjs_tk_jp_employee, 0) ELSE COALESCE(bpjstkjp, 0) END +
      COALESCE(bpjs_tk_jkk_employer, 0) +
      COALESCE(bpjs_tk_jkk_employee, 0) +
      COALESCE(bpjs_tk_jkm_employer, 0) +
      COALESCE(bpjs_tk_jkm_employee, 0)
    ) AS total_bpjs_ketenagakerjaan,
    SUM(
      COALESCE(bpjs_kesehatan_employer, 0) +
      CASE WHEN COALESCE(bpjs_kesehatan_employee, 0) > 0 THEN COALESCE(bpjs_kesehatan_employee, 0) ELSE COALESCE(bpjs_kesehatan, 0) END
    ) AS total_bpjs_kesehatan
  FROM payroll_items
  WHERE deleted_at IS NULL
  GROUP BY payroll_run_id
)
UPDATE payroll_runs pr
SET
  total_bpjs_ketenagakerjaan = ROUND(COALESCE(item_totals.total_bpjs_ketenagakerjaan, 0)::numeric, 2),
  total_bpjs_kesehatan = ROUND(COALESCE(item_totals.total_bpjs_kesehatan, 0)::numeric, 2)
FROM item_totals
WHERE pr.id = item_totals.payroll_run_id;

UPDATE payroll_runs
SET total_bpjs_ketenagakerjaan = COALESCE(total_bpjs, 0)
WHERE COALESCE(total_bpjs, 0) > 0
  AND COALESCE(total_bpjs_ketenagakerjaan, 0) = 0
  AND COALESCE(total_bpjs_kesehatan, 0) = 0;
