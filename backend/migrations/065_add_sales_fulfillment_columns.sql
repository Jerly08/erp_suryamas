-- Ensure sales fulfillment/backorder columns exist on staging/production databases.
-- These columns are used when sales are created with stock shortfall/backorder support.

ALTER TABLE sale_items
  ADD COLUMN IF NOT EXISTS available_quantity INTEGER DEFAULT 0,
  ADD COLUMN IF NOT EXISTS fulfilled_quantity INTEGER DEFAULT 0,
  ADD COLUMN IF NOT EXISTS backorder_quantity INTEGER DEFAULT 0,
  ADD COLUMN IF NOT EXISTS estimated_unit_cost DECIMAL(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS estimated_cogs_amount DECIMAL(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS actual_cogs_amount DECIMAL(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS cogs_variance_amount DECIMAL(15,2) DEFAULT 0,
  ADD COLUMN IF NOT EXISTS fulfillment_status VARCHAR(30) DEFAULT 'IN_STOCK';

UPDATE sale_items
SET
  available_quantity = COALESCE(available_quantity, quantity, 0),
  fulfilled_quantity = COALESCE(fulfilled_quantity, quantity, 0),
  backorder_quantity = COALESCE(backorder_quantity, 0),
  estimated_unit_cost = COALESCE(estimated_unit_cost, 0),
  estimated_cogs_amount = COALESCE(estimated_cogs_amount, 0),
  actual_cogs_amount = COALESCE(actual_cogs_amount, 0),
  cogs_variance_amount = COALESCE(cogs_variance_amount, actual_cogs_amount - estimated_cogs_amount, 0),
  fulfillment_status = COALESCE(NULLIF(fulfillment_status, ''), 'IN_STOCK');

CREATE INDEX IF NOT EXISTS idx_sale_items_fulfillment_status ON sale_items(fulfillment_status);
CREATE INDEX IF NOT EXISTS idx_sale_items_backorder_quantity ON sale_items(backorder_quantity);
