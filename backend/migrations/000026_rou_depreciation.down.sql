DROP TABLE IF EXISTS lease_depreciation_log;

-- Remove seeded accounts (optional — they may be in use after deployment).
DELETE FROM accounts WHERE code IN ('1702', '5209');
