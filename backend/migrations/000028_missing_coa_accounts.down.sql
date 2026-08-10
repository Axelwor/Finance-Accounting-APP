-- Rollback: remove seeded accounts 3105, 4902, 1302
DELETE FROM accounts WHERE code IN ('3105', '4902', '1302');
