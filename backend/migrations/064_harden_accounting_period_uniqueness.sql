-- Migration: Harden accounting_periods uniqueness on normalized period boundaries
-- Purpose:
--   1. Prevent duplicate active periods for the same start/end dates
--   2. Stay safe on legacy databases that may already contain duplicates

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM information_schema.tables
        WHERE table_name = 'accounting_periods'
    ) THEN
        RAISE NOTICE '⏭️ accounting_periods table not found, skipping uniqueness hardening';
        RETURN;
    END IF;

    IF EXISTS (
        SELECT 1
        FROM accounting_periods
        WHERE deleted_at IS NULL
        GROUP BY DATE(start_date), DATE(end_date)
        HAVING COUNT(*) > 1
    ) THEN
        RAISE NOTICE '⚠️ Duplicate accounting_periods detected on canonical dates. Skipping unique index creation until data is cleaned up.';
        RETURN;
    END IF;

    -- Use raw boundary timestamps for uniqueness because the live schema stores
    -- timestamptz columns, and DATE(timestamptz) is not immutable enough for an
    -- index expression. The application write path already normalizes period
    -- boundaries to start-of-day and end-of-day before persisting.
    CREATE UNIQUE INDEX IF NOT EXISTS idx_accounting_periods_unique_boundaries
        ON accounting_periods (start_date, end_date)
        WHERE deleted_at IS NULL;

    RAISE NOTICE '✅ Hardened accounting_periods uniqueness on normalized period boundaries';
END $$;
