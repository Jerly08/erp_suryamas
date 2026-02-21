-- Remove old simple_ssot_journals tables
-- Since these are legacy tables and data was migrated to unified_journal_ledger, this action is permanent.

DROP TABLE IF EXISTS simple_ssot_journal_items;
DROP TABLE IF EXISTS simple_ssot_journals;
