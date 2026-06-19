ALTER TABLE purchase_request_items
    ADD COLUMN IF NOT EXISTS item_name VARCHAR(255) NOT NULL DEFAULT '';

UPDATE purchase_request_items AS pri
SET item_name = products.name
FROM products
WHERE pri.product_id = products.id
  AND BTRIM(COALESCE(pri.item_name, '')) = '';

ALTER TABLE purchase_request_items
    ALTER COLUMN product_id DROP NOT NULL;
