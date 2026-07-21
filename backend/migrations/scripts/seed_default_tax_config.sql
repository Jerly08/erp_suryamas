-- Seed Default Tax Configuration
-- Run this to create default tax config with Indonesia standard rates

-- Insert default tax config
INSERT INTO tax_configs (
    config_name,
    description,
    
    -- Sales tax rates
    sales_ppn_rate,
    sales_pph21_rate,
    sales_pph23_rate,
    sales_other_tax_rate,
    
    -- Sales tax accounts (adjust IDs based on your accounts table)
    sales_ppn_account_id,
    sales_pph21_account_id,
    sales_pph23_account_id,
    sales_other_tax_account_id,
    
    -- Purchase tax rates
    purchase_ppn_rate,
    purchase_pph21_rate,
    purchase_pph23_rate,
    purchase_pph25_rate,
    purchase_other_tax_rate,
    
    -- Purchase tax accounts (adjust IDs based on your accounts table)
    purchase_ppn_account_id,
    purchase_pph21_account_id,
    purchase_pph23_account_id,
    purchase_pph25_account_id,
    purchase_other_tax_account_id,
    
    -- Additional settings
    shipping_taxable,
    discount_before_tax,
    rounding_method,
    
    -- Status flags
    is_active,
    is_default,
    
    -- Metadata
    updated_by,
    created_at,
    updated_at
) VALUES (
    'Indonesia Standard',
    'Standard tax configuration for Indonesia (PPN 11%)',
    
    -- Sales rates
    11.0,  -- PPN Keluaran 11%
    0.0,   -- PPh 21 (default 0, customer bisa potong jika applicable)
    0.0,   -- PPh 23 (default 0, customer bisa potong jika applicable)
    0.0,   -- Other tax
    
    -- Sales accounts (sesuaikan dengan ID account Anda)
    166,   -- 2103 - PPN Keluaran
    254,   -- 1114 - PPh 21 Dibayar Dimuka
    255,   -- 1115 - PPh 23 Dibayar Dimuka
    254,   -- 1116 - Potongan Pajak Lainnya (fallback to 1114)
    
    -- Purchase rates
    11.0,  -- PPN Masukan 11%
    0.0,   -- PPh 21 (default 0)
    2.0,   -- PPh 23 2% (standard untuk jasa)
    0.0,   -- PPh 25
    0.0,   -- Other tax
    
    -- Purchase accounts (sesuaikan dengan ID account Anda)
    COALESCE(
      (SELECT id FROM accounts WHERE code = '1105-005' AND type = 'ASSET' AND is_active = true AND is_header = false AND deleted_at IS NULL LIMIT 1),
      (SELECT id FROM accounts WHERE code = '1240' AND type = 'ASSET' AND is_active = true AND is_header = false AND deleted_at IS NULL LIMIT 1),
      164
    ),     -- 1105-005 - PPN Masukan (asset); 1240 kept as legacy fallback
    167,   -- 2104 - PPh 21 Yang Dipotong
    168,   -- 2105 - PPh 23 Yang Dipotong
    169,   -- 2106 - PPh 25
    170,   -- 2107 - Pemotongan Pajak Lainnya
    
    -- Additional settings
    true,           -- shipping_taxable
    true,           -- discount_before_tax
    'ROUND_HALF_UP', -- rounding_method
    
    -- Status
    true,  -- is_active
    true,  -- is_default
    
    -- Metadata
    1,     -- updated_by (admin user)
    NOW(),
    NOW()
)
ON CONFLICT (config_name) DO UPDATE SET
    is_active = EXCLUDED.is_active,
    is_default = EXCLUDED.is_default,
    updated_at = NOW();

-- Verify
SELECT 
    id,
    config_name,
    sales_ppn_rate,
    purchase_ppn_rate,
    purchase_pph23_rate,
    is_active,
    is_default
FROM tax_configs
WHERE is_active = true AND is_default = true;
