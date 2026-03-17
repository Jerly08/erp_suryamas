-- Add USD/IDR audit fields for sales receivable payments.
ALTER TABLE sale_payments
ADD COLUMN IF NOT EXISTS native_amount NUMERIC(20,6) DEFAULT 0,
ADD COLUMN IF NOT EXISTS currency VARCHAR(5) DEFAULT 'IDR',
ADD COLUMN IF NOT EXISTS exchange_rate NUMERIC(15,6) DEFAULT 1,
ADD COLUMN IF NOT EXISTS book_amount_id_r NUMERIC(20,2) DEFAULT 0,
ADD COLUMN IF NOT EXISTS forex_gain_loss_amount NUMERIC(20,2) DEFAULT 0,
ADD COLUMN IF NOT EXISTS forex_gain_loss_type VARCHAR(10);

UPDATE sale_payments
SET native_amount = COALESCE(native_amount, amount),
    currency = COALESCE(NULLIF(currency, ''), 'IDR'),
    exchange_rate = COALESCE(NULLIF(exchange_rate, 0), 1),
    book_amount_id_r = CASE
        WHEN COALESCE(book_amount_id_r, 0) = 0 THEN COALESCE(amount, 0)
        ELSE book_amount_id_r
    END
WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_sale_payments_currency ON sale_payments(currency);
