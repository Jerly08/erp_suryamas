-- Component-based Purchase VAT / DPP Nilai Lain.
-- This migration is additive: purchases created before it retain the existing
-- global PPN calculation and do not need a backfill.

ALTER TABLE purchases
    ADD COLUMN IF NOT EXISTS vat_treatment VARCHAR(40) NOT NULL DEFAULT 'STANDARD';

ALTER TABLE purchase_documents
    ADD COLUMN IF NOT EXISTS reference_number VARCHAR(100) NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS purchase_tax_allocations (
    id BIGSERIAL PRIMARY KEY,
    purchase_id BIGINT NOT NULL,
    purchase_item_id BIGINT NOT NULL,
    tax_treatment VARCHAR(40) NOT NULL DEFAULT 'DPP_NILAI_LAIN',
    source_amount NUMERIC(15,2) NOT NULL DEFAULT 0,
    dpp_amount NUMERIC(15,2) NOT NULL DEFAULT 0,
    ppn_rate NUMERIC(8,4) NOT NULL DEFAULT 0,
    ppn_amount NUMERIC(15,2) NOT NULL DEFAULT 0,
    is_creditable BOOLEAN NOT NULL DEFAULT TRUE,
    tax_invoice_number VARCHAR(100) NOT NULL DEFAULT '',
    tax_invoice_date TIMESTAMPTZ NULL,
    notes TEXT NOT NULL DEFAULT '',
    verified_by BIGINT NULL,
    verified_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ NULL
);

CREATE INDEX IF NOT EXISTS idx_purchase_tax_allocations_purchase
    ON purchase_tax_allocations (purchase_id, id)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_purchase_tax_allocations_item
    ON purchase_tax_allocations (purchase_item_id)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_purchase_tax_allocations_tax_invoice
    ON purchase_tax_allocations (tax_invoice_number)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_purchase_documents_tax_invoice_reference
    ON purchase_documents (purchase_id, document_type, reference_number)
    WHERE deleted_at IS NULL;
