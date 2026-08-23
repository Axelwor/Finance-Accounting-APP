-- FIX-WAVE-003 F2 / N2: purchase return (Retur Pembelian) posts
-- inventory_movements rows with movement_type 'PURCHASE_RETURN', symmetric to
-- 'SALES_RETURN' on the sales side. The CHECK constraint created inline by
-- 000007_delivery_order did not include it, so every purchase return failed
-- with SQLSTATE 23514 (inventory_movements_movement_type_check) and the whole
-- transaction rolled back: stock never decreased and the reversal journal was
-- never created.
--
-- Fix: drop and re-add the constraint with 'PURCHASE_RETURN' allowed.

ALTER TABLE inventory_movements DROP CONSTRAINT IF EXISTS inventory_movements_movement_type_check;

ALTER TABLE inventory_movements ADD CONSTRAINT inventory_movements_movement_type_check
    CHECK (movement_type IN ('GRN', 'SALES_RETURN', 'PURCHASE_RETURN', 'DO', 'PRODUCTION_OUT', 'PRODUCTION_IN', 'TRANSFER_IN', 'TRANSFER_OUT', 'OPNAME_IN', 'OPNAME_OUT', 'ADJUSTMENT'));
