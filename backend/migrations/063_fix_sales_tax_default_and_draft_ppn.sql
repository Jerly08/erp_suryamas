ALTER TABLE sales
    ALTER COLUMN ppn_percent SET DEFAULT 0;

UPDATE sales s
SET
    ppn_percent = 0,
    ppn_rate = 0,
    ppn = 0,
    ppn_amount = 0,
    total_tax = COALESCE(s.other_tax_additions, 0) - COALESCE(s.total_tax_deductions, 0),
    total_amount = COALESCE(s.taxable_amount, COALESCE(s.subtotal, 0) - COALESCE(s.discount_amount, 0))
        + COALESCE(s.other_tax_additions, 0)
        + COALESCE(s.shipping_cost, 0)
        - COALESCE(s.total_tax_deductions, 0),
    outstanding_amount = (
        COALESCE(s.taxable_amount, COALESCE(s.subtotal, 0) - COALESCE(s.discount_amount, 0))
        + COALESCE(s.other_tax_additions, 0)
        + COALESCE(s.shipping_cost, 0)
        - COALESCE(s.total_tax_deductions, 0)
    ) - COALESCE(s.paid_amount, 0),
    updated_at = NOW()
WHERE s.status IN ('DRAFT', 'CONFIRMED', 'PENDING')
  AND COALESCE(s.ppn, 0) = 0
  AND COALESCE(s.ppn_amount, 0) = 0
  AND COALESCE(s.total_tax, 0) = 0
  AND COALESCE(s.ppn_percent, 0) <> 0
  AND NOT EXISTS (
      SELECT 1
      FROM sale_items si
      WHERE si.sale_id = s.id
        AND COALESCE(si.ppn_amount, 0) <> 0
  );
