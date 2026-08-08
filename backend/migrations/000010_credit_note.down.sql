DROP TABLE IF EXISTS credit_note_lines;
DROP TABLE IF EXISTS credit_notes;
DELETE FROM accounts WHERE code = '4201' AND account_type = 'CONTRA_REVENUE';
