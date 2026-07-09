ALTER TABLE settings
ADD COLUMN IF NOT EXISTS company_logo_scale INTEGER NOT NULL DEFAULT 100;

UPDATE settings
SET company_logo_scale = 100
WHERE company_logo_scale < 50 OR company_logo_scale > 180;
