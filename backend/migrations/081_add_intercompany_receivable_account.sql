-- Add an intercompany receivable COA for non-invoice payment classification.
-- This migration only creates/normalizes the account. It does not move balances
-- or alter historical journals.

DO $$
DECLARE
  parent_current_asset_id BIGINT;
  receivable_parent_id BIGINT;
BEGIN
  SELECT id INTO parent_current_asset_id
  FROM accounts
  WHERE code = '1100' AND deleted_at IS NULL
  ORDER BY id ASC
  LIMIT 1;

  SELECT id INTO receivable_parent_id
  FROM accounts
  WHERE code = '1107' AND deleted_at IS NULL
  ORDER BY id ASC
  LIMIT 1;

  IF receivable_parent_id IS NULL THEN
    INSERT INTO accounts (
      code, name, type, category, parent_id, level, is_header, is_active, balance, created_at, updated_at
    )
    VALUES (
      '1107', 'PIUTANG LAIN-LAIN', 'ASSET', 'CURRENT_ASSET', parent_current_asset_id, 3, true, true, 0, NOW(), NOW()
    )
    RETURNING id INTO receivable_parent_id;
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM accounts WHERE code = '1107-002' AND deleted_at IS NULL
  ) THEN
    INSERT INTO accounts (
      code, name, type, category, parent_id, level, is_header, is_active, balance, created_at, updated_at
    )
    VALUES (
      '1107-002', 'PIUTANG INTERCOMPANY', 'ASSET', 'CURRENT_ASSET', receivable_parent_id, 4, false, true, 0, NOW(), NOW()
    );
  ELSE
    UPDATE accounts
    SET
      name = 'PIUTANG INTERCOMPANY',
      type = 'ASSET',
      category = 'CURRENT_ASSET',
      parent_id = receivable_parent_id,
      level = 4,
      is_header = false,
      is_active = true,
      updated_at = NOW()
    WHERE code = '1107-002'
      AND deleted_at IS NULL;
  END IF;
END $$;
