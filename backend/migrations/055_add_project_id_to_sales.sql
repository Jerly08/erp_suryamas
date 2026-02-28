-- Migration: Add project_id to sales and total_sales_revenue to projects
-- This enables linking sales transactions to projects for project profitability tracking

-- Add project_id column to sales table
ALTER TABLE sales ADD COLUMN IF NOT EXISTS project_id INTEGER REFERENCES projects(id);

-- Add index for performance
CREATE INDEX IF NOT EXISTS idx_sales_project_id ON sales(project_id);

-- Add total_sales_revenue column to projects table  
ALTER TABLE projects ADD COLUMN IF NOT EXISTS total_sales_revenue DECIMAL(15,2) DEFAULT 0;
