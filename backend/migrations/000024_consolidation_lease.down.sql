-- Reverse US-110 + US-111

DROP TABLE IF EXISTS lease_payments;
DROP TABLE IF EXISTS lease_contracts;
DROP TABLE IF EXISTS inter_company_transactions;
DROP TABLE IF EXISTS entity_hierarchy;

-- Remove seeded accounts (only those with no journal lines, to be safe).
DELETE FROM accounts
WHERE code IN ('1701', '2301', '5906')
  AND id NOT IN (SELECT account_id FROM journal_lines);
