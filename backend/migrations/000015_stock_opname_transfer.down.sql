-- Reverse stock opname + stock transfer schema. Removes adjustment accounts
-- seeded for 4907 / 5907 only if no journal lines reference them.

DROP TABLE IF EXISTS stock_transfer_lines;
DROP TABLE IF EXISTS stock_transfers;
DROP TABLE IF EXISTS stock_opname_lines;
DROP TABLE IF EXISTS stock_opnames;

DELETE FROM accounts
WHERE code IN ('4907', '5907')
  AND NOT EXISTS (SELECT 1 FROM journal_lines WHERE journal_lines.account_id = accounts.id);
