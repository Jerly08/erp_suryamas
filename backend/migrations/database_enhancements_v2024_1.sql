-- =====================================================
-- Database Enhancements Migration v2024.1
-- Sistema Akuntansi - Comprehensive Database Optimization
-- =====================================================

-- STEP 1: Journal Entry Performance Indexes
-- Creates indexes for better journal entry and line performance
CREATE INDEX IF NOT EXISTS idx_journal_entries_entry_date ON journal_entries(entry_date);
CREATE INDEX IF NOT EXISTS idx_journal_entries_reference_type_id ON journal_entries(reference_type, reference_id);
CREATE INDEX IF NOT EXISTS idx_journal_entries_status_date ON journal_entries(status, entry_date);
CREATE INDEX IF NOT EXISTS idx_journal_entries_user_id_date ON journal_entries(user_id, entry_date);
CREATE INDEX IF NOT EXISTS idx_journal_entries_journal_id ON journal_entries(journal_id);
CREATE INDEX IF NOT EXISTS idx_journal_entries_period ON journal_entries(entry_date) WHERE status = 'POSTED';

-- Journal lines indexes
CREATE INDEX IF NOT EXISTS idx_journal_lines_entry_account ON journal_lines(journal_entry_id, account_id);
CREATE INDEX IF NOT EXISTS idx_journal_lines_account_debit ON journal_lines(account_id, debit_amount) WHERE debit_amount > 0;
CREATE INDEX IF NOT EXISTS idx_journal_lines_account_credit ON journal_lines(account_id, credit_amount) WHERE credit_amount > 0;
CREATE INDEX IF NOT EXISTS idx_journal_lines_amounts ON journal_lines(debit_amount, credit_amount);

-- Composite indexes for complex queries
CREATE INDEX IF NOT EXISTS idx_journal_entries_complete ON journal_entries(entry_date, status, reference_type) WHERE status = 'POSTED';
CREATE INDEX IF NOT EXISTS idx_journal_lines_balance ON journal_lines(account_id, debit_amount, credit_amount);

-- STEP 2: Accounting Performance Indexes
-- Account balance calculation indexes
CREATE INDEX IF NOT EXISTS idx_accounts_type_balance ON accounts(type, balance) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_accounts_category_balance ON accounts(category, balance) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_accounts_parent_children ON accounts(parent_id) WHERE parent_id IS NOT NULL;

-- Transaction reporting indexes
CREATE INDEX IF NOT EXISTS idx_transactions_date_account_amount ON transactions(transaction_date, account_id, amount);
CREATE INDEX IF NOT EXISTS idx_transactions_period_reporting ON transactions(transaction_date, account_id) WHERE deleted_at IS NULL;

-- Sales and purchases reporting indexes
CREATE INDEX IF NOT EXISTS idx_sales_reporting ON sales(date, status, total_amount) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_purchases_reporting ON purchases(date, status, total_amount) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_sales_customer_period ON sales(customer_id, date, total_amount) WHERE status IN ('INVOICED', 'PAID');
CREATE INDEX IF NOT EXISTS idx_purchases_vendor_period ON purchases(vendor_id, date, total_amount) WHERE status = 'APPROVED';

-- Cash flow indexes
CREATE INDEX IF NOT EXISTS idx_cash_bank_transactions_flow ON cash_bank_transactions(transaction_date, amount);
CREATE INDEX IF NOT EXISTS idx_payments_cash_flow ON payments(date, method, amount) WHERE status = 'COMPLETED';

-- Aging analysis indexes
CREATE INDEX IF NOT EXISTS idx_sales_aging ON sales(due_date, outstanding_amount) WHERE outstanding_amount > 0;
CREATE INDEX IF NOT EXISTS idx_purchases_aging ON purchases(due_date, outstanding_amount) WHERE outstanding_amount > 0;

-- STEP 3: Validation Constraints
-- Journal entry validation constraints
ALTER TABLE journal_entries 
DROP CONSTRAINT IF EXISTS chk_journal_entries_balanced,
ADD CONSTRAINT chk_journal_entries_balanced 
CHECK (ABS(total_debit - total_credit) < 0.01);

-- Account balance constraints
ALTER TABLE accounts
DROP CONSTRAINT IF EXISTS chk_accounts_type_valid,
ADD CONSTRAINT chk_accounts_type_valid 
CHECK (type IN ('ASSET', 'LIABILITY', 'EQUITY', 'REVENUE', 'EXPENSE'));

-- Amount validation constraints
ALTER TABLE journal_lines
DROP CONSTRAINT IF EXISTS chk_journal_lines_amount_positive,
ADD CONSTRAINT chk_journal_lines_amount_positive 
CHECK (debit_amount >= 0 AND credit_amount >= 0 AND (debit_amount > 0 OR credit_amount > 0));

-- Date validation constraints - Relaxed to allow future period closing
ALTER TABLE journal_entries
DROP CONSTRAINT IF EXISTS chk_journal_entries_date_valid,
ADD CONSTRAINT chk_journal_entries_date_valid 
CHECK (entry_date >= '2000-01-01' AND entry_date <= '2099-12-31');

-- Status validation constraints
ALTER TABLE journal_entries
DROP CONSTRAINT IF EXISTS chk_journal_entries_status_valid,
ADD CONSTRAINT chk_journal_entries_status_valid 
CHECK (status IN ('DRAFT', 'POSTED', 'REVERSED'));

-- STEP 4: Audit Trail Enhancements
-- Audit log indexes for better query performance
CREATE INDEX IF NOT EXISTS idx_audit_logs_table_record_action ON audit_logs(table_name, record_id, action);
CREATE INDEX IF NOT EXISTS idx_audit_logs_user_timestamp ON audit_logs(user_id, created_at);
CREATE INDEX IF NOT EXISTS idx_audit_logs_timestamp_action ON audit_logs(created_at, action);
CREATE INDEX IF NOT EXISTS idx_audit_logs_critical ON audit_logs(created_at) WHERE action IN ('DELETE', 'UPDATE') AND table_name IN ('journal_entries', 'accounts', 'transactions');

-- STEP 5: Audit Summary Views
-- Create audit summary view
CREATE OR REPLACE VIEW audit_trail_summary AS
SELECT 
    DATE(created_at) as audit_date,
    table_name,
    action,
    user_id,
    COUNT(*) as action_count,
    MIN(created_at) as first_action,
    MAX(created_at) as last_action
FROM audit_logs
WHERE created_at >= CURRENT_DATE - INTERVAL '30 days'
GROUP BY DATE(created_at), table_name, action, user_id
ORDER BY audit_date DESC, action_count DESC;

-- Create critical changes view
CREATE OR REPLACE VIEW critical_audit_changes AS
SELECT 
    al.*,
    u.username,
    TRIM(CONCAT(u.first_name, ' ', u.last_name)) as full_name
FROM audit_logs al
LEFT JOIN users u ON al.user_id = u.id
WHERE al.action IN ('DELETE', 'UPDATE')
    AND al.table_name IN ('journal_entries', 'accounts', 'transactions', 'sales', 'purchases')
    AND al.created_at >= CURRENT_DATE - INTERVAL '7 days'
ORDER BY al.created_at DESC;

-- STEP 6: Account Balance Views
-- Create account balance summary view
CREATE OR REPLACE VIEW account_balance_summary AS
SELECT 
    a.id,
    a.code,
    a.name,
    a.type,
    a.category,
    a.balance as current_balance,
    COALESCE(SUM(jl.debit_amount), 0) as total_debits,
    COALESCE(SUM(jl.credit_amount), 0) as total_credits,
    COALESCE(SUM(jl.debit_amount) - SUM(jl.credit_amount), 0) as calculated_balance,
    ABS(a.balance - COALESCE(SUM(jl.debit_amount) - SUM(jl.credit_amount), 0)) as balance_difference
FROM accounts a
LEFT JOIN journal_lines jl ON a.id = jl.account_id
LEFT JOIN journal_entries je ON jl.journal_entry_id = je.id AND je.status = 'POSTED'
WHERE a.deleted_at IS NULL
GROUP BY a.id, a.code, a.name, a.type, a.category, a.balance
ORDER BY a.code;

-- STEP 7: Financial Reporting Views
-- Trial Balance View
CREATE OR REPLACE VIEW trial_balance AS
SELECT 
    a.code,
    a.name,
    a.type,
    a.category,
    CASE 
        WHEN a.type IN ('ASSET', 'EXPENSE') THEN 
            COALESCE(SUM(jl.debit_amount) - SUM(jl.credit_amount), 0)
        ELSE 0 
    END as debit_balance,
    CASE 
        WHEN a.type IN ('LIABILITY', 'EQUITY', 'REVENUE') THEN 
            COALESCE(SUM(jl.credit_amount) - SUM(jl.debit_amount), 0)
        ELSE 0 
    END as credit_balance
FROM accounts a
LEFT JOIN journal_lines jl ON a.id = jl.account_id
LEFT JOIN journal_entries je ON jl.journal_entry_id = je.id AND je.status = 'POSTED'
WHERE a.deleted_at IS NULL
GROUP BY a.id, a.code, a.name, a.type, a.category
HAVING COALESCE(SUM(jl.debit_amount) - SUM(jl.credit_amount), 0) != 0
ORDER BY a.code;

-- STEP 8: Cash Flow Analysis View
CREATE OR REPLACE VIEW cash_flow_analysis AS
SELECT 
    DATE(cbt.transaction_date) as transaction_date,
    cb.code as account_code,
    cb.name as account_name,
    cb.type as account_type,
    SUM(CASE WHEN cbt.transaction_type = 'INFLOW' THEN cbt.amount ELSE 0 END) as total_inflow,
    SUM(CASE WHEN cbt.transaction_type = 'OUTFLOW' THEN cbt.amount ELSE 0 END) as total_outflow,
    SUM(CASE WHEN cbt.transaction_type = 'INFLOW' THEN cbt.amount ELSE -cbt.amount END) as net_flow
FROM cash_bank_transactions cbt
JOIN cash_banks cb ON cbt.cash_bank_id = cb.id
WHERE cbt.deleted_at IS NULL
    AND cb.deleted_at IS NULL
    AND cbt.transaction_date >= CURRENT_DATE - INTERVAL '1 year'
GROUP BY DATE(cbt.transaction_date), cb.id, cb.code, cb.name, cb.type
ORDER BY transaction_date DESC, cb.code;

-- STEP 9: Sales Analysis View
CREATE OR REPLACE VIEW sales_analysis AS
SELECT 
    DATE(s.date) as sale_date,
    c.name as customer_name,
    s.code as sale_code,
    s.type as sale_type,
    s.status,
    s.subtotal,
    s.discount_amount,
    s.taxable_amount,
    s.ppn,
    s.pph,
    s.total_amount,
    s.paid_amount,
    s.outstanding_amount,
    CASE 
        WHEN s.due_date < CURRENT_DATE AND s.outstanding_amount > 0 THEN 'OVERDUE'
        WHEN s.outstanding_amount = 0 THEN 'PAID'
        WHEN s.outstanding_amount > 0 THEN 'PENDING'
        ELSE 'UNKNOWN'
    END as payment_status
FROM sales s
LEFT JOIN contacts c ON s.customer_id = c.id
WHERE s.deleted_at IS NULL
ORDER BY s.date DESC;

-- STEP 10: Purchase Analysis View
CREATE OR REPLACE VIEW purchase_analysis AS
SELECT 
    DATE(p.date) as purchase_date,
    v.name as vendor_name,
    p.code as purchase_code,
    p.status,
    p.subtotal,
    p.discount_amount,
    p.taxable_amount,
    p.ppn,
    p.pph,
    p.total_amount,
    p.paid_amount,
    p.outstanding_amount,
    CASE 
        WHEN p.due_date < CURRENT_DATE AND p.outstanding_amount > 0 THEN 'OVERDUE'
        WHEN p.outstanding_amount = 0 THEN 'PAID'
        WHEN p.outstanding_amount > 0 THEN 'PENDING'
        ELSE 'UNKNOWN'
    END as payment_status
FROM purchases p
LEFT JOIN contacts v ON p.vendor_id = v.id
WHERE p.deleted_at IS NULL
ORDER BY p.date DESC;

-- STEP 11: Update table statistics for PostgreSQL optimization
ANALYZE accounts;
ANALYZE journal_entries;
ANALYZE journal_lines;
ANALYZE transactions;
ANALYZE sales;
ANALYZE purchases;
ANALYZE cash_bank_transactions;
ANALYZE audit_logs;

-- STEP 12: Create performance monitoring function
CREATE OR REPLACE FUNCTION get_database_performance_stats()
RETURNS TABLE(
    table_name TEXT,
    row_count BIGINT,
    table_size TEXT,
    index_size TEXT,
    total_size TEXT
) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        schemaname||'.'||tablename as table_name,
        n_tup_ins + n_tup_upd as row_count,
        pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) as table_size,
        pg_size_pretty(pg_indexes_size(schemaname||'.'||tablename)) as index_size,
        pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename) + pg_indexes_size(schemaname||'.'||tablename)) as total_size
    FROM pg_stat_user_tables 
    WHERE schemaname = 'public'
        AND tablename IN ('accounts', 'journal_entries', 'journal_lines', 'sales', 'purchases', 'transactions', 'cash_bank_transactions')
    ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC;
END;
$$ LANGUAGE plpgsql;

-- STEP 13: Create system health check function
CREATE OR REPLACE FUNCTION system_health_check()
RETURNS TABLE(
    check_name TEXT,
    status TEXT,
    description TEXT,
    recommendation TEXT
) AS $$
BEGIN
    -- Check for unbalanced journal entries
    RETURN QUERY
    SELECT 
        'Unbalanced Journal Entries'::TEXT,
        CASE WHEN COUNT(*) = 0 THEN 'OK' ELSE 'WARNING' END::TEXT,
        ('Found ' || COUNT(*) || ' unbalanced journal entries')::TEXT,
        CASE WHEN COUNT(*) > 0 THEN 'Review and correct unbalanced entries' ELSE 'No action needed' END::TEXT
    FROM journal_entries 
    WHERE ABS(total_debit - total_credit) >= 0.01 AND status = 'POSTED';
    
    -- Check for orphaned journal lines
    RETURN QUERY
    SELECT 
        'Orphaned Journal Lines'::TEXT,
        CASE WHEN COUNT(*) = 0 THEN 'OK' ELSE 'WARNING' END::TEXT,
        ('Found ' || COUNT(*) || ' orphaned journal lines')::TEXT,
        CASE WHEN COUNT(*) > 0 THEN 'Clean up orphaned journal lines' ELSE 'No action needed' END::TEXT
    FROM journal_lines jl
    LEFT JOIN journal_entries je ON jl.journal_entry_id = je.id
    WHERE je.id IS NULL;
    
    -- Check for accounts without transactions
    RETURN QUERY
    SELECT 
        'Inactive Accounts'::TEXT,
        'INFO'::TEXT,
        ('Found ' || COUNT(*) || ' accounts without any transactions')::TEXT,
        'Consider reviewing unused accounts'::TEXT
    FROM accounts a
    LEFT JOIN journal_lines jl ON a.id = jl.account_id
    WHERE a.deleted_at IS NULL AND jl.account_id IS NULL;
    
    -- Check database size
    RETURN QUERY
    SELECT 
        'Database Size'::TEXT,
        'INFO'::TEXT,
        ('Database size: ' || pg_size_pretty(pg_database_size(current_database())))::TEXT,
        'Monitor database growth regularly'::TEXT;
END;
$$ LANGUAGE plpgsql;

-- STEP 14: Create journal entry validation function
CREATE OR REPLACE FUNCTION validate_journal_entry(entry_id INTEGER)
RETURNS TABLE(
    is_valid BOOLEAN,
    error_message TEXT
) AS $$
DECLARE
    total_debit DECIMAL(15,2);
    total_credit DECIMAL(15,2);
    line_count INTEGER;
BEGIN
    -- Get totals from journal lines
    SELECT 
        COALESCE(SUM(debit_amount), 0),
        COALESCE(SUM(credit_amount), 0),
        COUNT(*)
    INTO total_debit, total_credit, line_count
    FROM journal_lines
    WHERE journal_entry_id = entry_id;
    
    -- Validation checks
    IF line_count = 0 THEN
        RETURN QUERY SELECT FALSE, 'Journal entry has no lines';
        RETURN;
    END IF;
    
    IF ABS(total_debit - total_credit) >= 0.01 THEN
        RETURN QUERY SELECT FALSE, 'Journal entry is not balanced: Debit=' || total_debit || ', Credit=' || total_credit;
        RETURN;
    END IF;
    
    IF total_debit = 0 AND total_credit = 0 THEN
        RETURN QUERY SELECT FALSE, 'Journal entry has zero amounts';
        RETURN;
    END IF;
    
    -- All validations passed
    RETURN QUERY SELECT TRUE, 'Journal entry is valid';
END;
$$ LANGUAGE plpgsql;

-- STEP 15: Create account balance reconciliation function  
CREATE OR REPLACE FUNCTION reconcile_account_balance(account_id INTEGER)
RETURNS TABLE(
    account_code TEXT,
    account_name TEXT,
    current_balance DECIMAL(15,2),
    calculated_balance DECIMAL(15,2),
    difference DECIMAL(15,2),
    needs_adjustment BOOLEAN
) AS $$
DECLARE
    calc_balance DECIMAL(15,2);
    curr_balance DECIMAL(15,2);
    acc_code TEXT;
    acc_name TEXT;
BEGIN
    -- Get account info and current balance
    SELECT a.code, a.name, a.balance 
    INTO acc_code, acc_name, curr_balance
    FROM accounts a 
    WHERE a.id = account_id;
    
    -- Calculate balance from journal lines
    SELECT COALESCE(SUM(jl.debit_amount) - SUM(jl.credit_amount), 0)
    INTO calc_balance
    FROM journal_lines jl
    JOIN journal_entries je ON jl.journal_entry_id = je.id
    WHERE jl.account_id = account_id AND je.status = 'POSTED';
    
    RETURN QUERY SELECT 
        acc_code,
        acc_name,
        curr_balance,
        calc_balance,
        curr_balance - calc_balance,
        ABS(curr_balance - calc_balance) >= 0.01;
END;
$$ LANGUAGE plpgsql;

-- =====================================================
-- Migration completed successfully
-- =====================================================