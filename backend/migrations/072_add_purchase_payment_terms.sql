CREATE TABLE IF NOT EXISTS purchase_payment_terms (
  id BIGSERIAL PRIMARY KEY,
  purchase_id BIGINT NOT NULL REFERENCES purchases(id) ON DELETE CASCADE,
  sequence INTEGER NOT NULL DEFAULT 1,
  percentage DECIMAL(8,4) NOT NULL DEFAULT 0,
  amount DECIMAL(15,2) NOT NULL DEFAULT 0,
  due_date TIMESTAMPTZ,
  description VARCHAR(255),
  status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_purchase_payment_terms_purchase_id
  ON purchase_payment_terms(purchase_id);

CREATE INDEX IF NOT EXISTS idx_purchase_payment_terms_deleted_at
  ON purchase_payment_terms(deleted_at);

CREATE INDEX IF NOT EXISTS idx_purchase_payment_terms_status
  ON purchase_payment_terms(status);

ALTER TABLE payment_allocations
  ADD COLUMN IF NOT EXISTS purchase_payment_term_id BIGINT;

CREATE INDEX IF NOT EXISTS idx_payment_allocations_purchase_payment_term_id
  ON payment_allocations(purchase_payment_term_id);

INSERT INTO purchase_payment_terms (
  purchase_id,
  sequence,
  percentage,
  amount,
  due_date,
  description,
  status,
  created_at,
  updated_at
)
SELECT
  p.id,
  1,
  100,
  COALESCE(NULLIF(p.total_amount, 0), COALESCE(p.outstanding_amount, 0), 0),
  CASE
    WHEN p.due_date IS NULL OR p.due_date < TIMESTAMPTZ '1900-01-01'
      THEN COALESCE(NULLIF(p.date, TIMESTAMPTZ '0001-01-01'), NOW())
    ELSE p.due_date
  END,
  'Default credit term',
  CASE
    WHEN COALESCE(p.paid_amount, 0) >= COALESCE(p.total_amount, 0) - 0.01 THEN 'PAID'
    WHEN COALESCE(p.paid_amount, 0) > 0.01 THEN 'PARTIAL'
    ELSE 'PENDING'
  END,
  NOW(),
  NOW()
FROM purchases p
WHERE UPPER(COALESCE(p.payment_method, '')) = 'CREDIT'
  AND p.deleted_at IS NULL
  AND NOT EXISTS (
    SELECT 1
    FROM purchase_payment_terms ppt
    WHERE ppt.purchase_id = p.id
      AND ppt.deleted_at IS NULL
  );
