-- Migration: Create tax_account_settings table
-- Description: Create table for storing tax account configuration settings
-- Version: 027
-- Created: 2024-10-03
-- PostgreSQL Compatible Version

-- First check if table exists, if so skip creation
DO $$ 
BEGIN
    IF NOT EXISTS (SELECT FROM pg_tables WHERE tablename = 'tax_account_settings') THEN
        
        CREATE TABLE tax_account_settings (
            id BIGSERIAL PRIMARY KEY,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            deleted_at TIMESTAMP NULL DEFAULT NULL,

            -- Sales Account Configuration (required)
            sales_receivable_account_id BIGINT NOT NULL,
            sales_cash_account_id BIGINT NOT NULL,
            sales_bank_account_id BIGINT NOT NULL,
            sales_revenue_account_id BIGINT NOT NULL,
            sales_output_vat_account_id BIGINT NOT NULL,

            -- Purchase Account Configuration (required)
            purchase_payable_account_id BIGINT NOT NULL,
            purchase_cash_account_id BIGINT NOT NULL,
            purchase_bank_account_id BIGINT NOT NULL,
            purchase_input_vat_account_id BIGINT NOT NULL,
            purchase_expense_account_id BIGINT NOT NULL,

            -- Other Tax Accounts (optional)
            withholding_tax21_account_id BIGINT NULL DEFAULT NULL,
            withholding_tax23_account_id BIGINT NULL DEFAULT NULL,
            withholding_tax25_account_id BIGINT NULL DEFAULT NULL,
            tax_payable_account_id BIGINT NULL DEFAULT NULL,

            -- Inventory Account (optional)
            inventory_account_id BIGINT NULL DEFAULT NULL,
            cogs_account_id BIGINT NULL DEFAULT NULL,

            -- Configuration flags
            is_active BOOLEAN DEFAULT TRUE,
            apply_to_all_companies BOOLEAN DEFAULT TRUE,

            -- Metadata
            updated_by BIGINT NOT NULL,
            notes TEXT NULL
        );

        -- Create indexes
        CREATE INDEX IF NOT EXISTS idx_tax_account_settings_deleted_at ON tax_account_settings (deleted_at);
        CREATE INDEX IF NOT EXISTS idx_tax_account_settings_is_active ON tax_account_settings (is_active);
        CREATE INDEX IF NOT EXISTS idx_tax_account_settings_updated_by ON tax_account_settings (updated_by);

        -- Create indexes for Sales accounts
        CREATE INDEX IF NOT EXISTS idx_tax_settings_sales_receivable ON tax_account_settings (sales_receivable_account_id);
        CREATE INDEX IF NOT EXISTS idx_tax_settings_sales_cash ON tax_account_settings (sales_cash_account_id);
        CREATE INDEX IF NOT EXISTS idx_tax_settings_sales_bank ON tax_account_settings (sales_bank_account_id);
        CREATE INDEX IF NOT EXISTS idx_tax_settings_sales_revenue ON tax_account_settings (sales_revenue_account_id);
        CREATE INDEX IF NOT EXISTS idx_tax_settings_sales_output_vat ON tax_account_settings (sales_output_vat_account_id);

        -- Create indexes for Purchase accounts
        CREATE INDEX IF NOT EXISTS idx_tax_settings_purchase_payable ON tax_account_settings (purchase_payable_account_id);
        CREATE INDEX IF NOT EXISTS idx_tax_settings_purchase_cash ON tax_account_settings (purchase_cash_account_id);
        CREATE INDEX IF NOT EXISTS idx_tax_settings_purchase_bank ON tax_account_settings (purchase_bank_account_id);
        CREATE INDEX IF NOT EXISTS idx_tax_settings_purchase_input_vat ON tax_account_settings (purchase_input_vat_account_id);
        CREATE INDEX IF NOT EXISTS idx_tax_settings_purchase_expense ON tax_account_settings (purchase_expense_account_id);

        -- Create indexes for optional accounts
        CREATE INDEX IF NOT EXISTS idx_tax_settings_withholding_tax21 ON tax_account_settings (withholding_tax21_account_id);
        CREATE INDEX IF NOT EXISTS idx_tax_settings_withholding_tax23 ON tax_account_settings (withholding_tax23_account_id);
        CREATE INDEX IF NOT EXISTS idx_tax_settings_withholding_tax25 ON tax_account_settings (withholding_tax25_account_id);
        CREATE INDEX IF NOT EXISTS idx_tax_settings_tax_payable ON tax_account_settings (tax_payable_account_id);
        CREATE INDEX IF NOT EXISTS idx_tax_settings_inventory ON tax_account_settings (inventory_account_id);
        CREATE INDEX IF NOT EXISTS idx_tax_settings_cogs ON tax_account_settings (cogs_account_id);

        -- Add foreign key constraints
        ALTER TABLE tax_account_settings ADD CONSTRAINT fk_tax_settings_sales_receivable 
            FOREIGN KEY (sales_receivable_account_id) REFERENCES accounts(id) ON DELETE RESTRICT;
        ALTER TABLE tax_account_settings ADD CONSTRAINT fk_tax_settings_sales_cash 
            FOREIGN KEY (sales_cash_account_id) REFERENCES accounts(id) ON DELETE RESTRICT;
        ALTER TABLE tax_account_settings ADD CONSTRAINT fk_tax_settings_sales_bank 
            FOREIGN KEY (sales_bank_account_id) REFERENCES accounts(id) ON DELETE RESTRICT;
        ALTER TABLE tax_account_settings ADD CONSTRAINT fk_tax_settings_sales_revenue 
            FOREIGN KEY (sales_revenue_account_id) REFERENCES accounts(id) ON DELETE RESTRICT;
        ALTER TABLE tax_account_settings ADD CONSTRAINT fk_tax_settings_sales_output_vat 
            FOREIGN KEY (sales_output_vat_account_id) REFERENCES accounts(id) ON DELETE RESTRICT;

        ALTER TABLE tax_account_settings ADD CONSTRAINT fk_tax_settings_purchase_payable 
            FOREIGN KEY (purchase_payable_account_id) REFERENCES accounts(id) ON DELETE RESTRICT;
        ALTER TABLE tax_account_settings ADD CONSTRAINT fk_tax_settings_purchase_cash 
            FOREIGN KEY (purchase_cash_account_id) REFERENCES accounts(id) ON DELETE RESTRICT;
        ALTER TABLE tax_account_settings ADD CONSTRAINT fk_tax_settings_purchase_bank 
            FOREIGN KEY (purchase_bank_account_id) REFERENCES accounts(id) ON DELETE RESTRICT;
        ALTER TABLE tax_account_settings ADD CONSTRAINT fk_tax_settings_purchase_input_vat 
            FOREIGN KEY (purchase_input_vat_account_id) REFERENCES accounts(id) ON DELETE RESTRICT;
        ALTER TABLE tax_account_settings ADD CONSTRAINT fk_tax_settings_purchase_expense 
            FOREIGN KEY (purchase_expense_account_id) REFERENCES accounts(id) ON DELETE RESTRICT;

        ALTER TABLE tax_account_settings ADD CONSTRAINT fk_tax_settings_withholding_tax21 
            FOREIGN KEY (withholding_tax21_account_id) REFERENCES accounts(id) ON DELETE SET NULL;
        ALTER TABLE tax_account_settings ADD CONSTRAINT fk_tax_settings_withholding_tax23 
            FOREIGN KEY (withholding_tax23_account_id) REFERENCES accounts(id) ON DELETE SET NULL;
        ALTER TABLE tax_account_settings ADD CONSTRAINT fk_tax_settings_withholding_tax25 
            FOREIGN KEY (withholding_tax25_account_id) REFERENCES accounts(id) ON DELETE SET NULL;
        ALTER TABLE tax_account_settings ADD CONSTRAINT fk_tax_settings_tax_payable 
            FOREIGN KEY (tax_payable_account_id) REFERENCES accounts(id) ON DELETE SET NULL;
        ALTER TABLE tax_account_settings ADD CONSTRAINT fk_tax_settings_inventory 
            FOREIGN KEY (inventory_account_id) REFERENCES accounts(id) ON DELETE SET NULL;
        ALTER TABLE tax_account_settings ADD CONSTRAINT fk_tax_settings_cogs 
            FOREIGN KEY (cogs_account_id) REFERENCES accounts(id) ON DELETE SET NULL;
        ALTER TABLE tax_account_settings ADD CONSTRAINT fk_tax_settings_updated_by 
            FOREIGN KEY (updated_by) REFERENCES users(id) ON DELETE RESTRICT;

        -- Add table comment
        COMMENT ON TABLE tax_account_settings IS 'Configuration table for tax account mappings used in sales and purchase transactions';
        
        -- Create updated_at trigger function if not exists
        CREATE OR REPLACE FUNCTION update_tax_account_settings_updated_at()
        RETURNS TRIGGER AS $trigger$
        BEGIN
            NEW.updated_at = CURRENT_TIMESTAMP;
            RETURN NEW;
        END;
        $trigger$ LANGUAGE plpgsql;
        
        -- Create trigger for updating updated_at column
        DROP TRIGGER IF EXISTS tax_account_settings_updated_at_trigger ON tax_account_settings;
        CREATE TRIGGER tax_account_settings_updated_at_trigger
            BEFORE UPDATE ON tax_account_settings
            FOR EACH ROW
            EXECUTE FUNCTION update_tax_account_settings_updated_at();
        
        RAISE NOTICE 'Created tax_account_settings table successfully';
    ELSE
        RAISE NOTICE 'tax_account_settings table already exists, skipping creation';
    END IF;
END $$;

-- Insert default configuration based on current hardcoded values (PostgreSQL compatible)
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM tax_account_settings WHERE is_active = true) THEN
        INSERT INTO tax_account_settings (
            sales_receivable_account_id,
            sales_cash_account_id,
            sales_bank_account_id,
            sales_revenue_account_id,
            sales_output_vat_account_id,
            purchase_payable_account_id,
            purchase_cash_account_id,
            purchase_bank_account_id,
            purchase_input_vat_account_id,
            purchase_expense_account_id,
            is_active,
            apply_to_all_companies,
            updated_by,
            notes
        ) VALUES (
            -- Sales accounts (based on hardcoded values in services)
            COALESCE((SELECT id FROM accounts WHERE code = '1201' AND is_active = true LIMIT 1), 1),
            COALESCE((SELECT id FROM accounts WHERE code = '1101' AND is_active = true LIMIT 1), 1),
            COALESCE((SELECT id FROM accounts WHERE code = '1102' AND is_active = true LIMIT 1), 
                     (SELECT id FROM accounts WHERE code = '1101' AND is_active = true LIMIT 1), 1), -- fallback to cash account
            COALESCE((SELECT id FROM accounts WHERE code = '4101' AND is_active = true LIMIT 1), 1),
            COALESCE((SELECT id FROM accounts WHERE code = '2103' AND is_active = true LIMIT 1), 1),
            
            -- Purchase accounts (based on hardcoded values in services)
            COALESCE((SELECT id FROM accounts WHERE code = '2001' AND is_active = true LIMIT 1), 1),
            COALESCE((SELECT id FROM accounts WHERE code = '1101' AND is_active = true LIMIT 1), 1),
            COALESCE((SELECT id FROM accounts WHERE code = '1102' AND is_active = true LIMIT 1), 
                     (SELECT id FROM accounts WHERE code = '1101' AND is_active = true LIMIT 1), 1), -- fallback to cash account
            COALESCE(
                (SELECT id FROM accounts WHERE code = '1105-005' AND type = 'ASSET' AND is_active = true AND is_header = false AND deleted_at IS NULL LIMIT 1),
                (SELECT id FROM accounts WHERE code = '1240' AND type = 'ASSET' AND is_active = true AND is_header = false AND deleted_at IS NULL LIMIT 1),
                (SELECT id FROM accounts WHERE code = '2102-002' AND type IN ('LIABILITY', 'ASSET') AND is_active = true AND is_header = false AND deleted_at IS NULL LIMIT 1),
                1
            ), -- PPN Masukan (asset-first)
            COALESCE((SELECT id FROM accounts WHERE code = '6001' AND is_active = true LIMIT 1), 
                     (SELECT id FROM accounts WHERE code = '5101' AND is_active = true LIMIT 1), 1), -- fallback to COGS
            
            -- Configuration
            true,
            true,
            1, -- System user
            'Default configuration based on existing hardcoded values'
        );
        
        RAISE NOTICE 'Inserted default tax account settings successfully';
    ELSE
        RAISE NOTICE 'Tax account settings already exist, skipping default insert';
    END IF;
END $$;

-- Log migration (PostgreSQL compatible)
DO $$
BEGIN
    -- Try to insert migration log, ignore if it already exists
    IF NOT EXISTS (SELECT 1 FROM migrations WHERE migration = '027_create_tax_account_settings.sql') THEN
        INSERT INTO migrations (migration, batch, executed_at) 
        VALUES ('027_create_tax_account_settings.sql', 27, NOW());
    ELSE
        UPDATE migrations 
        SET executed_at = NOW() 
        WHERE migration = '027_create_tax_account_settings.sql';
    END IF;
EXCEPTION
    WHEN OTHERS THEN
        -- migrations table might not exist, create a simple log
        RAISE NOTICE 'Migration log table not available, but tax_account_settings migration completed';
END $$;
