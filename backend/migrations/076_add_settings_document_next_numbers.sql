-- Restore settings counters still used by the Settings model and UI payload.
-- Migration 035 removed these as legacy fields, but the settings update flow
-- still persists them when saving system settings.
ALTER TABLE settings
ADD COLUMN IF NOT EXISTS purchase_next_number INTEGER DEFAULT 1,
ADD COLUMN IF NOT EXISTS quote_next_number INTEGER DEFAULT 1;

UPDATE settings
SET purchase_next_number = COALESCE(purchase_next_number, 1),
    quote_next_number = COALESCE(quote_next_number, 1)
WHERE purchase_next_number IS NULL
   OR quote_next_number IS NULL;
