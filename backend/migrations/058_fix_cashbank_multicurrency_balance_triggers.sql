-- 058_fix_cashbank_multicurrency_balance_triggers.sql
-- Purpose:
--   Align legacy CashBank/COA triggers with multi-currency rules:
--   - cash_banks.balance stores native currency amount
--   - cash_banks.book_balance_id_r stores IDR book value
--   - accounts.balance always stores IDR book value

-- Validate CashBank against the effective COA/book balance
CREATE OR REPLACE FUNCTION validate_cashbank_coa_balance()
RETURNS TRIGGER AS $$
DECLARE
    coa_balance DECIMAL(20,2);
    expected_coa_balance DECIMAL(20,2);
BEGIN
    IF NEW.account_id IS NOT NULL THEN
        SELECT balance
          INTO coa_balance
          FROM accounts
         WHERE id = NEW.account_id
           AND deleted_at IS NULL;

        expected_coa_balance := CASE
            WHEN UPPER(COALESCE(NEW.currency, 'IDR')) = 'IDR' THEN COALESCE(NEW.balance, 0)
            ELSE COALESCE(NEW.book_balance_id_r, 0)
        END;

        IF coa_balance IS NOT NULL AND ABS(coa_balance - expected_coa_balance) > 0.01 THEN
            RAISE NOTICE 'WARNING: Cash Bank % effective book balance (%) differs from COA account % balance (%)',
                NEW.code, expected_coa_balance, NEW.account_id, coa_balance;
        END IF;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_validate_cashbank_coa_balance ON cash_banks;
CREATE TRIGGER trg_validate_cashbank_coa_balance
BEFORE UPDATE OF balance, book_balance_id_r ON cash_banks
FOR EACH ROW
WHEN (
    NEW.account_id IS NOT NULL
    AND (
        NEW.balance IS DISTINCT FROM OLD.balance
        OR NEW.book_balance_id_r IS DISTINCT FROM OLD.book_balance_id_r
    )
)
EXECUTE FUNCTION validate_cashbank_coa_balance();

-- Sync COA from CashBank using effective book balance
CREATE OR REPLACE FUNCTION sync_coa_balance_from_cashbank()
RETURNS TRIGGER AS $$
DECLARE
    expected_coa_balance DECIMAL(20,2);
BEGIN
    IF NEW.account_id IS NOT NULL
       AND (
            NEW.balance IS DISTINCT FROM OLD.balance
            OR NEW.book_balance_id_r IS DISTINCT FROM OLD.book_balance_id_r
       ) THEN

        expected_coa_balance := CASE
            WHEN UPPER(COALESCE(NEW.currency, 'IDR')) = 'IDR' THEN COALESCE(NEW.balance, 0)
            ELSE COALESCE(NEW.book_balance_id_r, 0)
        END;

        UPDATE accounts
           SET balance = expected_coa_balance,
               updated_at = CURRENT_TIMESTAMP
         WHERE id = NEW.account_id
           AND deleted_at IS NULL;

        RAISE NOTICE 'Synced COA account % effective balance to %.2f (from cash bank %)',
            NEW.account_id, expected_coa_balance, NEW.code;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_sync_coa_balance_from_cashbank ON cash_banks;
CREATE TRIGGER trg_sync_coa_balance_from_cashbank
AFTER UPDATE OF balance, book_balance_id_r ON cash_banks
FOR EACH ROW
WHEN (
    NEW.account_id IS NOT NULL
    AND (
        NEW.balance IS DISTINCT FROM OLD.balance
        OR NEW.book_balance_id_r IS DISTINCT FROM OLD.book_balance_id_r
    )
)
EXECUTE FUNCTION sync_coa_balance_from_cashbank();

-- Sync CashBank from COA without overwriting native foreign-currency balance
CREATE OR REPLACE FUNCTION sync_cashbank_balance_from_coa()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.balance IS DISTINCT FROM OLD.balance THEN
        UPDATE cash_banks
           SET balance = CASE
                    WHEN UPPER(COALESCE(currency, 'IDR')) = 'IDR' THEN NEW.balance
                    ELSE balance
               END,
               book_balance_id_r = CASE
                    WHEN UPPER(COALESCE(currency, 'IDR')) = 'IDR' THEN NEW.balance
                    ELSE NEW.balance
               END,
               last_exchange_rate = CASE
                    WHEN UPPER(COALESCE(currency, 'IDR')) = 'IDR' THEN 1
                    WHEN COALESCE(balance, 0) <> 0 THEN NEW.balance / balance
                    ELSE last_exchange_rate
               END,
               updated_at = CURRENT_TIMESTAMP
         WHERE account_id = NEW.id
           AND deleted_at IS NULL
           AND (
                UPPER(COALESCE(currency, 'IDR')) = 'IDR'
                OR book_balance_id_r IS DISTINCT FROM NEW.balance
           );

        IF FOUND THEN
            RAISE NOTICE 'Synced cash bank effective balance from COA account %', NEW.id;
        END IF;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_sync_cashbank_balance_from_coa ON accounts;
CREATE TRIGGER trg_sync_cashbank_balance_from_coa
AFTER UPDATE OF balance ON accounts
FOR EACH ROW
WHEN (NEW.balance IS DISTINCT FROM OLD.balance)
EXECUTE FUNCTION sync_cashbank_balance_from_coa();

-- Sync CashBank + COA from transaction history using native sum and book sum
CREATE OR REPLACE FUNCTION sync_cashbank_balance_to_coa()
RETURNS TRIGGER AS $$
DECLARE
    affected_cash_bank_id INTEGER;
    coa_account_id INTEGER;
    currency_code TEXT;
    native_sum DECIMAL(20,2);
    book_sum DECIMAL(20,2);
    effective_book_sum DECIMAL(20,2);
BEGIN
    affected_cash_bank_id := COALESCE(NEW.cash_bank_id, OLD.cash_bank_id);

    SELECT account_id, UPPER(COALESCE(currency, 'IDR'))
      INTO coa_account_id, currency_code
      FROM cash_banks
     WHERE id = affected_cash_bank_id;

    IF coa_account_id IS NULL THEN
        RETURN COALESCE(NEW, OLD);
    END IF;

    SELECT COALESCE(SUM(amount), 0),
           COALESCE(SUM(book_amount_id_r), 0)
      INTO native_sum, book_sum
      FROM cash_bank_transactions
     WHERE cash_bank_id = affected_cash_bank_id
       AND deleted_at IS NULL;

    effective_book_sum := CASE
        WHEN currency_code = 'IDR' THEN native_sum
        ELSE book_sum
    END;

    UPDATE cash_banks
       SET balance = native_sum,
           book_balance_id_r = effective_book_sum,
           last_exchange_rate = CASE
                WHEN currency_code = 'IDR' THEN 1
                WHEN native_sum <> 0 THEN effective_book_sum / native_sum
                ELSE last_exchange_rate
           END,
           updated_at = NOW()
     WHERE id = affected_cash_bank_id;

    UPDATE accounts
       SET balance = effective_book_sum,
           updated_at = NOW()
     WHERE id = coa_account_id;

    RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_sync_cashbank_coa ON cash_bank_transactions;
CREATE TRIGGER trg_sync_cashbank_coa
AFTER INSERT OR UPDATE OR DELETE ON cash_bank_transactions
FOR EACH ROW
EXECUTE FUNCTION sync_cashbank_balance_to_coa();
