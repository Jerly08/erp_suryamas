-- Add purchase PPh payment tracking so purchase rows can show PPh unpaid/paid status.

ALTER TABLE purchases
  ADD COLUMN IF NOT EXISTS pph_payment_status VARCHAR(20) NOT NULL DEFAULT 'NONE',
  ADD COLUMN IF NOT EXISTS pph_paid_amount DECIMAL(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS pph_payment_date TIMESTAMP NULL,
  ADD COLUMN IF NOT EXISTS pph_payment_reference VARCHAR(100) DEFAULT '',
  ADD COLUMN IF NOT EXISTS pph_payment_notes TEXT DEFAULT '',
  ADD COLUMN IF NOT EXISTS pph_payment_journal_entry_id BIGINT NULL;

DO $$
DECLARE
  pph21_expr TEXT := '0';
  pph23_expr TEXT := '0';
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_name = 'purchases' AND column_name = 'pph21_amount'
  ) THEN
    pph21_expr := 'COALESCE(pph21_amount, 0)';
  ELSIF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_name = 'purchases' AND column_name = 'p_ph21_amount'
  ) THEN
    pph21_expr := 'COALESCE(p_ph21_amount, 0)';
  END IF;

  IF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_name = 'purchases' AND column_name = 'pph23_amount'
  ) THEN
    pph23_expr := 'COALESCE(pph23_amount, 0)';
  ELSIF EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_name = 'purchases' AND column_name = 'p_ph23_amount'
  ) THEN
    pph23_expr := 'COALESCE(p_ph23_amount, 0)';
  END IF;

  EXECUTE format($sql$
    UPDATE purchases
    SET
      pph_paid_amount = COALESCE(pph_paid_amount, 0),
      pph_payment_reference = COALESCE(pph_payment_reference, ''),
      pph_payment_notes = COALESCE(pph_payment_notes, ''),
      pph_payment_status = CASE
        WHEN %1$s + %2$s <= 0.01 THEN 'NONE'
        WHEN COALESCE(pph_paid_amount, 0) >= %1$s + %2$s - 0.01 THEN 'PAID'
        WHEN COALESCE(pph_paid_amount, 0) > 0.01 THEN 'PARTIAL_PAID'
        ELSE 'UNPAID'
      END
  $sql$, pph21_expr, pph23_expr);
END $$;

CREATE INDEX IF NOT EXISTS idx_purchases_pph_payment_status ON purchases(pph_payment_status);
CREATE INDEX IF NOT EXISTS idx_purchases_pph_payment_journal_entry_id ON purchases(pph_payment_journal_entry_id);
