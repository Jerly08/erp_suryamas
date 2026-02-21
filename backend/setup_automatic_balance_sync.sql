-- =================================================================
-- AUTOMATIC BALANCE SYNCHRONIZATION SYSTEM
-- =================================================================
-- This script creates triggers to automatically update account.balance
-- whenever SSOT journal entries are posted, ensuring frontend always
-- shows current balances without manual intervention.

-- 1. Create function to update account balance from SSOT journal entries
CREATE OR REPLACE FUNCTION sync_account_balance_from_ssot(account_id_param INTEGER)
RETURNS VOID AS $$
DECLARE
    account_type_var VARCHAR(50);
    new_balance DECIMAL(20,2);
BEGIN
    -- Get account type
    SELECT type INTO account_type_var 
    FROM accounts 
    WHERE id = account_id_param;
    
    -- Calculate balance based on account type and SSOT journal entries
    SELECT 
        CASE 
            WHEN account_type_var IN ('ASSET', 'EXPENSE') THEN 
                COALESCE(SUM(ujl.debit_amount), 0) - COALESCE(SUM(ujl.credit_amount), 0)
            ELSE 
                COALESCE(SUM(ujl.credit_amount), 0) - COALESCE(SUM(ujl.debit_amount), 0)
        END
    INTO new_balance
    FROM unified_journal_lines ujl 
    LEFT JOIN unified_journal_ledger uje ON uje.id = ujl.journal_id
    WHERE ujl.account_id = account_id_param 
    AND uje.status = 'POSTED'
    AND uje.deleted_at IS NULL;
    
    -- Update account balance
    UPDATE accounts 
    SET 
        balance = COALESCE(new_balance, 0),
        updated_at = NOW()
    WHERE id = account_id_param;
    
    RAISE NOTICE 'Updated account ID % balance to %', account_id_param, COALESCE(new_balance, 0);
END;
$$ LANGUAGE plpgsql;

-- 2. Create trigger function for automatic sync
CREATE OR REPLACE FUNCTION trigger_sync_account_balance()
RETURNS TRIGGER AS $$
DECLARE
    affected_account_id INTEGER;
BEGIN
    -- Handle INSERT or UPDATE of journal lines
    IF TG_OP = 'INSERT' OR TG_OP = 'UPDATE' THEN
        affected_account_id := NEW.account_id;
        
        -- Only sync if the parent journal entry is POSTED
        IF EXISTS (
            SELECT 1 FROM unified_journal_ledger 
            WHERE id = NEW.journal_id AND status = 'POSTED'
        ) THEN
            PERFORM sync_account_balance_from_ssot(affected_account_id);
        END IF;
        
        RETURN NEW;
    END IF;
    
    -- Handle DELETE of journal lines
    IF TG_OP = 'DELETE' THEN
        affected_account_id := OLD.account_id;
        PERFORM sync_account_balance_from_ssot(affected_account_id);
        RETURN OLD;
    END IF;
    
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

-- 3. Create trigger function for journal status changes
CREATE OR REPLACE FUNCTION trigger_sync_on_journal_status_change()
RETURNS TRIGGER AS $$
DECLARE
    line_record RECORD;
BEGIN
    -- When journal entry status changes to POSTED, sync all related accounts
    IF NEW.status = 'POSTED' AND (OLD.status IS NULL OR OLD.status != 'POSTED') THEN
        FOR line_record IN 
            SELECT DISTINCT account_id 
            FROM unified_journal_lines 
            WHERE journal_id = NEW.id
        LOOP
            PERFORM sync_account_balance_from_ssot(line_record.account_id);
        END LOOP;
        
        RAISE NOTICE 'Synced account balances for journal entry %', NEW.entry_number;
    END IF;
    
    -- When journal entry is reversed or cancelled, sync all related accounts
    IF NEW.status IN ('REVERSED', 'CANCELLED') AND OLD.status = 'POSTED' THEN
        FOR line_record IN 
            SELECT DISTINCT account_id 
            FROM unified_journal_lines 
            WHERE journal_id = NEW.id
        LOOP
            PERFORM sync_account_balance_from_ssot(line_record.account_id);
        END LOOP;
        
        RAISE NOTICE 'Synced account balances after reversing journal entry %', NEW.entry_number;
    END IF;
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- 4. Drop existing triggers if they exist
DROP TRIGGER IF EXISTS trg_sync_account_balance_on_line_change ON unified_journal_lines;
DROP TRIGGER IF EXISTS trg_sync_account_balance_on_status_change ON unified_journal_ledger;

-- 5. Create triggers
CREATE TRIGGER trg_sync_account_balance_on_line_change
    AFTER INSERT OR UPDATE OR DELETE ON unified_journal_lines
    FOR EACH ROW 
    EXECUTE FUNCTION trigger_sync_account_balance();

CREATE TRIGGER trg_sync_account_balance_on_status_change
    AFTER UPDATE ON unified_journal_ledger
    FOR EACH ROW 
    WHEN (OLD.status IS DISTINCT FROM NEW.status)
    EXECUTE FUNCTION trigger_sync_on_journal_status_change();

-- 6. Create materialized view refresh function (for performance)
CREATE OR REPLACE FUNCTION refresh_account_balances_view()
RETURNS VOID AS $$
BEGIN
    -- Refresh materialized view if it exists
    IF EXISTS (
        SELECT 1 FROM information_schema.tables 
        WHERE table_name = 'account_balances' 
        AND table_type = 'MATERIALIZED VIEW'
    ) THEN
        REFRESH MATERIALIZED VIEW account_balances;
        RAISE NOTICE 'Refreshed account_balances materialized view';
    END IF;
END;
$$ LANGUAGE plpgsql;

-- 7. Create scheduled job to refresh materialized views (optional, for performance)
-- This can be called periodically by your application or cron job
CREATE OR REPLACE FUNCTION schedule_balance_view_refresh()
RETURNS VOID AS $$
BEGIN
    PERFORM refresh_account_balances_view();
    
    -- Log the refresh
    INSERT INTO migration_logs (migration_name, status, executed_at, message, description) 
    VALUES (
        'account_balances_refresh', 
        'SUCCESS', 
        NOW(), 
        'Automatic refresh of account balances materialized view',
        'Balance view refresh completed'
    ) ON CONFLICT (migration_name) DO UPDATE SET
        status = 'SUCCESS',
        executed_at = NOW(),
        message = 'Automatic refresh of account balances materialized view';
END;
$$ LANGUAGE plpgsql;

-- 8. Initial sync of all accounts (run once)
DO $$
DECLARE
    account_record RECORD;
    sync_count INTEGER := 0;
BEGIN
    RAISE NOTICE 'Starting initial account balance sync from SSOT...';
    
    FOR account_record IN 
        SELECT DISTINCT a.id 
        FROM accounts a
        WHERE EXISTS (
            SELECT 1 FROM unified_journal_lines ujl
            LEFT JOIN unified_journal_ledger uje ON uje.id = ujl.journal_id
            WHERE ujl.account_id = a.id AND uje.status = 'POSTED'
              AND uje.deleted_at IS NULL
        )
    LOOP
        PERFORM sync_account_balance_from_ssot(account_record.id);
        sync_count := sync_count + 1;
    END LOOP;
    
    RAISE NOTICE 'Initial sync completed for % accounts', sync_count;
END;
$$;

-- 9. Create index for better performance
-- Note: Partial index with subquery not supported, using simple index instead
CREATE INDEX IF NOT EXISTS idx_unified_journal_lines_account_id_posted 
ON unified_journal_lines (account_id, journal_id);

-- 10. Log the setup
INSERT INTO migration_logs (migration_name, status, executed_at, message, description) 
VALUES (
    'setup_automatic_balance_sync', 
    'SUCCESS', 
    NOW(), 
    'Created automatic balance synchronization system with triggers and functions',
    'Balance sync system installed'
) ON CONFLICT (migration_name) DO UPDATE SET
    status = 'SUCCESS',
    executed_at = NOW(),
    message = 'Updated automatic balance synchronization system with triggers and functions';

COMMENT ON FUNCTION sync_account_balance_from_ssot(INTEGER) IS 
'Synchronizes account.balance with SSOT journal entries for a specific account';

COMMENT ON FUNCTION trigger_sync_account_balance() IS 
'Trigger function to automatically sync account balances when journal lines change';

COMMENT ON FUNCTION trigger_sync_on_journal_status_change() IS 
'Trigger function to sync account balances when journal entry status changes';

-- Success message
SELECT 
    '🎉 AUTOMATIC BALANCE SYNC SYSTEM INSTALLED SUCCESSFULLY!' as status,
    'Account balances will now automatically sync with SSOT journal entries' as description,
    'No more manual intervention required for balance synchronization' as benefit;