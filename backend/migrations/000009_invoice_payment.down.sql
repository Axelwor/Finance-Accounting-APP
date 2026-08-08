DROP TABLE IF EXISTS invoice_payments;
DELETE FROM accounts WHERE code = '2402' AND account_type = 'CUSTOMER_DEPOSIT';
