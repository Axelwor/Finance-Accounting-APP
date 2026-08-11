-- A-01: Track DP consumption per sales order to prevent double-realization.
-- Add dp_consumed_cents column to sales_orders and update invoice logic.

ALTER TABLE sales_orders ADD COLUMN IF NOT EXISTS dp_consumed_cents BIGINT NOT NULL DEFAULT 0 CHECK (dp_consumed_cents >= 0);
