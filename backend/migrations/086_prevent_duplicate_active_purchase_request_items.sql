-- Prevent a Purchase Request item from being present more than once on the
-- same active purchase. Existing historical duplicates are intentionally left
-- untouched; this guard only blocks new duplicates while they are remediated.
CREATE OR REPLACE FUNCTION prevent_duplicate_active_purchase_request_items()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF NEW.purchase_request_item_id IS NULL OR NEW.deleted_at IS NOT NULL THEN
        RETURN NEW;
    END IF;

    -- Serialize concurrent inserts for the same purchase before checking for
    -- an active duplicate. The app-level replacement transaction soft-deletes
    -- old rows first, then creates the submitted item set.
    PERFORM pg_advisory_xact_lock(NEW.purchase_id::BIGINT);

    IF EXISTS (
        SELECT 1
        FROM purchase_items
        WHERE purchase_id = NEW.purchase_id
          AND purchase_request_item_id = NEW.purchase_request_item_id
          AND deleted_at IS NULL
          AND id <> COALESCE(NEW.id, 0)
    ) THEN
        RAISE EXCEPTION USING
            ERRCODE = '23505',
            MESSAGE = format(
                'purchase request item %s is already active on purchase %s',
                NEW.purchase_request_item_id,
                NEW.purchase_id
            );
    END IF;

    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_prevent_duplicate_active_purchase_request_items ON purchase_items;

CREATE TRIGGER trg_prevent_duplicate_active_purchase_request_items
BEFORE INSERT OR UPDATE OF purchase_id, purchase_request_item_id, deleted_at
ON purchase_items
FOR EACH ROW
EXECUTE FUNCTION prevent_duplicate_active_purchase_request_items();
