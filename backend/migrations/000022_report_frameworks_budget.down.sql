-- Reverse of 000022_report_frameworks_budget.up.sql

ALTER TABLE budget_lines DISABLE ROW LEVEL SECURITY;
ALTER TABLE budgets DISABLE ROW LEVEL SECURITY;
ALTER TABLE journal_line_dimensions DISABLE ROW LEVEL SECURITY;
ALTER TABLE dimensions DISABLE ROW LEVEL SECURITY;
ALTER TABLE report_frameworks DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS tenant_isolation_budget_lines ON budget_lines;
DROP POLICY IF EXISTS tenant_isolation_budgets ON budgets;
DROP POLICY IF EXISTS tenant_isolation_journal_line_dimensions ON journal_line_dimensions;
DROP POLICY IF EXISTS tenant_isolation_dimensions ON dimensions;
DROP POLICY IF EXISTS tenant_isolation_report_frameworks ON report_frameworks;

DROP TABLE IF EXISTS budget_lines;
DROP TABLE IF EXISTS budgets;
DROP TABLE IF EXISTS journal_line_dimensions;
DROP TABLE IF EXISTS dimensions;
DROP TABLE IF EXISTS report_frameworks;
