-- Fix module_permissions unique constraint untuk mendukung UPSERT (ON CONFLICT)
-- Partial index tidak bisa di-target oleh ON CONFLICT, jadi kita perlu unique constraint biasa

-- Hapus semua data soft-deleted terlebih dahulu agar tidak konflik dengan constraint baru
DELETE FROM module_permissions WHERE deleted_at IS NOT NULL;

-- Hapus partial index lama yang tidak bisa digunakan oleh ON CONFLICT
DROP INDEX IF EXISTS idx_module_permission_user_module_active;

-- Hapus index non-unique lama juga
DROP INDEX IF EXISTS idx_module_permission_user_module;

-- Hapus unique constraint lama jika ada
ALTER TABLE module_permissions DROP CONSTRAINT IF EXISTS uq_module_permission_user_module;
ALTER TABLE module_permissions DROP CONSTRAINT IF EXISTS uni_module_permission_records_user_id_module;
ALTER TABLE module_permissions DROP CONSTRAINT IF EXISTS uni_module_permissions_user_id_module;

-- Buat UNIQUE CONSTRAINT biasa pada (user_id, module)
-- Ini diperlukan agar ON CONFLICT (user_id, module) dapat bekerja
ALTER TABLE module_permissions
    ADD CONSTRAINT uq_module_permission_user_module UNIQUE (user_id, module);

-- Buat index untuk performa query
CREATE INDEX IF NOT EXISTS idx_module_permission_user_module
    ON module_permissions(user_id, module);
