-- Rollback 000027_output_vat.up.sql

DELETE FROM accounts WHERE code = '2202' AND name LIKE '%Output VAT (PPN Keluaran)%';
