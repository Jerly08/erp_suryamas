-- Add project_id column to unified_journal_ledger table
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'unified_journal_ledger')
       AND NOT EXISTS (SELECT 1 FROM information_schema.columns 
                       WHERE table_name = 'unified_journal_ledger' 
                       AND column_name = 'project_id') THEN
        
        RAISE NOTICE '🔧 Adding project_id column to unified_journal_ledger';
        
        ALTER TABLE unified_journal_ledger 
        ADD COLUMN project_id BIGINT;
        
        RAISE NOTICE '✅ Added project_id column';
    ELSE
        RAISE NOTICE '✅ Column project_id already exists or table does not exist';
    END IF;
END $$;
