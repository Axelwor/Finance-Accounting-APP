-- B-04: No-op down migration. Seeded COA accounts may have posted journals
-- referencing them, so deleting them is unsafe on downgrade.
SELECT 1;
