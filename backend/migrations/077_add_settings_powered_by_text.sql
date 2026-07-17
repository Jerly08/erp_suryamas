ALTER TABLE settings ADD COLUMN IF NOT EXISTS powered_by_text VARCHAR(160);
UPDATE settings
SET powered_by_text = 'Powered by PT SURYA HEBAT BERBAKAT'
WHERE powered_by_text IS NULL
   OR BTRIM(powered_by_text) = ''
   OR powered_by_text = 'Powered by';
ALTER TABLE settings ALTER COLUMN powered_by_text SET DEFAULT 'Powered by PT SURYA HEBAT BERBAKAT';
ALTER TABLE settings ALTER COLUMN powered_by_text SET NOT NULL;
