-- Migration: Rename the system opening-balance equity account from 3104 to 3105.
-- The account ID is deliberately preserved so historic journal links and balances remain unchanged.

DO $$
DECLARE
    legacy_account_id BIGINT;
    target_account_id BIGINT;
    critical_trigger_exists BOOLEAN;
BEGIN
    SELECT id
    INTO legacy_account_id
    FROM accounts
    WHERE code = '3104'
      AND name = 'EKUITAS SALDO AWAL'
      AND deleted_at IS NULL
    FOR UPDATE;

    SELECT id
    INTO target_account_id
    FROM accounts
    WHERE code = '3105'
      AND deleted_at IS NULL
    FOR UPDATE;

    -- Fresh installations are seeded directly with 3105. Existing installations
    -- already migrated to 3105 also have nothing left to change.
    IF legacy_account_id IS NULL THEN
        RETURN;
    END IF;

    -- Never merge two account records automatically. A duplicate active target
    -- must be reviewed manually so journal ownership cannot be ambiguous.
    IF target_account_id IS NOT NULL AND target_account_id <> legacy_account_id THEN
        RAISE EXCEPTION
            'Cannot rename opening-balance equity account 3104 to 3105: an active account already uses code 3105 (id %)',
            target_account_id;
    END IF;

    SELECT EXISTS (
        SELECT 1
        FROM pg_trigger
        WHERE tgrelid = 'accounts'::regclass
          AND tgname = 'check_critical_account_update'
          AND NOT tgisinternal
    )
    INTO critical_trigger_exists;

    -- The critical-account trigger correctly blocks ad-hoc code changes. Disable
    -- it only within this atomic migration, then restore it before commit.
    IF critical_trigger_exists THEN
        EXECUTE 'ALTER TABLE accounts DISABLE TRIGGER check_critical_account_update';
    END IF;

    UPDATE accounts
    SET code = '3105', updated_at = NOW()
    WHERE id = legacy_account_id;

    IF critical_trigger_exists THEN
        EXECUTE 'ALTER TABLE accounts ENABLE TRIGGER check_critical_account_update';
    END IF;
END;
$$;
