-- Reverse US-070..072: Production / Job Order Costing.

DROP TABLE IF EXISTS production_job_costs;
DROP TABLE IF EXISTS production_jobs;
DROP TABLE IF EXISTS bom_lines;
DROP TABLE IF EXISTS bill_of_materials;

-- NOTE: seeded accounts (1303, 1304, 4902, 5901) are intentionally NOT
-- removed here because journal entries may already reference them. They are
-- harmless if left in place; dropping them could break the hash-chain ledger.
