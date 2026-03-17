ALTER TABLE payments
ADD COLUMN IF NOT EXISTS expense_account_id BIGINT,
ADD COLUMN IF NOT EXISTS cash_bank_id BIGINT;

CREATE INDEX IF NOT EXISTS idx_payments_expense_account_id ON payments(expense_account_id);
CREATE INDEX IF NOT EXISTS idx_payments_cash_bank_id ON payments(cash_bank_id);

UPDATE payments
SET expense_account_id = COALESCE(
        expense_account_id,
        CAST((regexp_match(reference, '^__EXP_ACC:(\d+),CB:(\d+)__'))[1] AS BIGINT)
    ),
    cash_bank_id = COALESCE(
        cash_bank_id,
        CAST((regexp_match(reference, '^__EXP_ACC:(\d+),CB:(\d+)__'))[2] AS BIGINT)
    )
WHERE reference ~ '^__EXP_ACC:\d+,CB:\d+__';

UPDATE payments
SET cash_bank_id = COALESCE(
        cash_bank_id,
        CAST((regexp_match(reference, '^__CB:(\d+)__'))[1] AS BIGINT)
    )
WHERE reference ~ '^__CB:\d+__';

UPDATE payments
SET reference = regexp_replace(reference, '^__EXP_ACC:\d+,CB:\d+__', '')
WHERE reference ~ '^__EXP_ACC:\d+,CB:\d+__';

UPDATE payments
SET reference = regexp_replace(reference, '^__CB:\d+__', '')
WHERE reference ~ '^__CB:\d+__';
