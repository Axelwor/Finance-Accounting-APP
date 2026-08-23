-- Rollback 000058_purchase_return_movement: restore the original
-- movement_type CHECK without 'PURCHASE_RETURN'.

ALTER TABLE inventory_movements DROP CONSTRAINT IF EXISTS inventory_movements_movement_type_check;

ALTER TABLE inventory_movements ADD CONSTRAINT inventory_movements_movement_type_check
    CHECK (movement_type IN ('GRN', 'SALES_RETURN', 'DO', 'PRODUCTION_OUT', 'PRODUCTION_IN', 'TRANSFER_IN', 'TRANSFER_OUT', 'OPNAME_IN', 'OPNAME_OUT', 'ADJUSTMENT'));
