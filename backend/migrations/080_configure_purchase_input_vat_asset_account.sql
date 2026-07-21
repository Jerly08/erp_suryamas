-- Configure PPN Masukan as an asset account for future purchase VAT postings.
-- This migration does not move historical balances or journal lines.

INSERT INTO accounts (
  code,
  name,
  type,
  category,
  parent_id,
  level,
  is_header,
  is_active,
  is_system_critical,
  balance,
  created_at,
  updated_at
)
SELECT
  '1105',
  'PAJAK DIBAYAR DIMUKA',
  'ASSET',
  'CURRENT_ASSET',
  (SELECT id FROM accounts WHERE code = '1100' AND deleted_at IS NULL LIMIT 1),
  3,
  TRUE,
  TRUE,
  TRUE,
  0,
  NOW(),
  NOW()
WHERE NOT EXISTS (
  SELECT 1 FROM accounts WHERE code = '1105' AND deleted_at IS NULL
);

UPDATE accounts
SET
  name = 'PAJAK DIBAYAR DIMUKA',
  type = 'ASSET',
  category = 'CURRENT_ASSET',
  parent_id = COALESCE((SELECT id FROM accounts WHERE code = '1100' AND deleted_at IS NULL LIMIT 1), parent_id),
  level = 3,
  is_header = TRUE,
  is_active = TRUE,
  is_system_critical = TRUE,
  updated_at = NOW()
WHERE code = '1105'
  AND deleted_at IS NULL;

INSERT INTO accounts (
  code,
  name,
  type,
  category,
  parent_id,
  level,
  is_header,
  is_active,
  is_system_critical,
  balance,
  created_at,
  updated_at
)
SELECT
  '1105-005',
  'PPN MASUKAN',
  'ASSET',
  'CURRENT_ASSET',
  parent.id,
  4,
  FALSE,
  TRUE,
  TRUE,
  0,
  NOW(),
  NOW()
FROM accounts parent
WHERE parent.code = '1105'
  AND parent.deleted_at IS NULL
ON CONFLICT (code) WHERE deleted_at IS NULL
DO UPDATE SET
  name = EXCLUDED.name,
  type = EXCLUDED.type,
  category = EXCLUDED.category,
  parent_id = EXCLUDED.parent_id,
  level = EXCLUDED.level,
  is_header = EXCLUDED.is_header,
  is_active = TRUE,
  is_system_critical = TRUE,
  updated_at = NOW();

WITH ppn_masukan_asset AS (
  SELECT id
  FROM accounts
  WHERE code = '1105-005'
    AND type = 'ASSET'
    AND is_active = TRUE
    AND is_header = FALSE
    AND deleted_at IS NULL
  LIMIT 1
)
UPDATE tax_account_settings settings
SET
  purchase_input_vat_account_id = ppn_masukan_asset.id,
  updated_at = NOW(),
  notes = TRIM(BOTH E'\n' FROM CONCAT_WS(
    E'\n',
    NULLIF(settings.notes, ''),
    'System update: PPN Masukan untuk transaksi baru diarahkan ke akun aset 1105-005. Saldo lama tidak dipindahkan otomatis.'
  ))
FROM ppn_masukan_asset
WHERE settings.deleted_at IS NULL
  AND settings.is_active = TRUE
  AND settings.purchase_input_vat_account_id IS DISTINCT FROM ppn_masukan_asset.id;

DO $$
BEGIN
  IF to_regclass('public.tax_configs') IS NOT NULL THEN
    WITH ppn_masukan_asset AS (
      SELECT id
      FROM accounts
      WHERE code = '1105-005'
        AND type = 'ASSET'
        AND is_active = TRUE
        AND is_header = FALSE
        AND deleted_at IS NULL
      LIMIT 1
    )
    UPDATE tax_configs config
    SET
      purchase_ppn_account_id = ppn_masukan_asset.id,
      updated_at = NOW(),
      notes = TRIM(BOTH E'\n' FROM CONCAT_WS(
        E'\n',
        NULLIF(config.notes, ''),
        'System update: Purchase PPN/PPN Masukan default uses asset account 1105-005 for future transactions.'
      ))
    FROM ppn_masukan_asset
    WHERE config.deleted_at IS NULL
      AND config.is_active = TRUE
      AND config.purchase_ppn_account_id IS DISTINCT FROM ppn_masukan_asset.id;
  END IF;
END $$;
