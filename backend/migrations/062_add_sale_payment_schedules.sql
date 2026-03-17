CREATE TABLE IF NOT EXISTS sale_payment_schedules (
    id BIGSERIAL PRIMARY KEY,
    sale_id BIGINT NOT NULL REFERENCES sales(id) ON DELETE CASCADE,
    sequence INTEGER NOT NULL DEFAULT 1,
    percentage DECIMAL(8,4) NOT NULL DEFAULT 0,
    amount DECIMAL(15,2) NOT NULL DEFAULT 0,
    due_date TIMESTAMP NOT NULL,
    description VARCHAR(255) DEFAULT '',
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP NULL
);

CREATE INDEX IF NOT EXISTS idx_sale_payment_schedules_sale_id
    ON sale_payment_schedules(sale_id);

CREATE INDEX IF NOT EXISTS idx_sale_payment_schedules_deleted_at
    ON sale_payment_schedules(deleted_at);

CREATE UNIQUE INDEX IF NOT EXISTS idx_sale_payment_schedules_sale_sequence_active
    ON sale_payment_schedules(sale_id, sequence)
    WHERE deleted_at IS NULL;
