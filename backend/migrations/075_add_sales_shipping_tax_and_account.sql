ALTER TABLE sales
    ALTER COLUMN shipping_taxable SET DEFAULT TRUE;

ALTER TABLE tax_account_settings
    ADD COLUMN IF NOT EXISTS sales_shipping_revenue_account_id BIGINT NULL;

CREATE INDEX IF NOT EXISTS idx_tax_account_settings_sales_shipping_revenue
    ON tax_account_settings(sales_shipping_revenue_account_id);

UPDATE tax_account_settings tas
SET sales_shipping_revenue_account_id = account.id,
    updated_at = NOW()
FROM accounts account
WHERE tas.sales_shipping_revenue_account_id IS NULL
  AND account.code = '4101-010'
  AND account.type = 'REVENUE'
  AND account.is_active = TRUE
  AND account.is_header = FALSE
  AND account.deleted_at IS NULL;
