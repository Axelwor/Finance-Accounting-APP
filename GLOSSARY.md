# Two-Layer Glossary

## Simple Bookkeeping Software with an IFRS/PSAK Accounting Engine

**PRD Appendix** — Simple Bookkeeping Software with an IFRS/PSAK Accounting Engine  
**Version:** 1.3 — Review  
**Date:** 2026-08-07  
**Status:** Review  
**Owner:** Product + Accounting  
**Normative:** Yes for UI and accounting terminology

---

## 1. Purpose

Bridge the **layperson language** (UI layer) with **accounting/PSAK terms** (engine layer). Every term that appears in the UI or technical documents has one consistent definition — one concept, one term.

---

## 2. Two-Layer Term Map

| User Term (UI) | Accounting Term (Engine) | Explanation |
|---|---|---|
| **Money In** | `CASH_IN` / Cash Receipt | Transaction that increases cash/bank (cash sales, receivable collection, capital, loans) |
| **Money Out** | `CASH_OUT` / Cash Payment | Transaction that decreases cash/bank (expenses, payable payments, cash purchases) |
| **Transfer** | `TRANSFER` | Moving cash between CASH/BANK accounts; not a profit/loss transaction |
| **Profit / Loss** | Profit & Loss (Income Statement) | Revenue − expenses for a given period |
| **Bills** | Invoice / Accounts Receivable | Billing document sent to customers |
| **Payables** | Accounts Payable / Liabilities | Obligation to pay suppliers/other parties |
| **Receivables** | Accounts Receivable / Assets | Right to collect from customers |
| **Category** | Account (COA) | Transaction grouping; every category maps to a PSAK account |
| **Goods** | Inventory | Trading stock; valued FIFO/average (PSAK 14) |
| **Low Stock** | Reorder Point / Minimum Stock | Stock threshold that triggers a purchase reminder |
| **Asset** | Fixed Asset | Items with > 1 year life: machinery, vehicles, buildings (PSAK 16) |
| **Depreciation** | Depreciation | Allocating asset cost over its useful life |
| **Opening Balance** | Opening Balance | Account balance when first starting to use the system |
| **Close Books** | Closing Period | Locking the period & moving profit to equity |
| **Down Payment** | Down Payment / Advance Sales | Advance payment; liability (2201) on the selling side |
| **Return** | Credit Note / Purchase Return | Returning goods; reduces revenue/payables |
| **Discount** | Sales / Purchase Discount | Transaction value reduction |
| **Tax** | VAT / Income Tax | Value Added Tax & Income Tax |

---

## 3. Core Technical Terms (Engine)

### 3.1 Journal & Posting
| Term | Definition |
|---|---|
| **Journal Entry** | Double-entry record: at least 2 lines, total debit = total credit |
| **Debit / Credit** | Left/right side of a journal; not "increase/decrease" — depends on account type |
| **Posting** | Process of saving a journal to the ledger |
| **Void** | Cancel a journal with a **reversal journal** (not deletion) — audit trail preserved |
| **Hash Chain** | SHA-256 hash chain between journals — anti-tamper |
| **Source Ref** | Original document number (INV-2026-000123) that triggered the journal |
| **Intent** | Structured transaction type (`SALES_INVOICE`, `CASH_IN`, ...) — basis for automatic journals |
| **Idempotency** | Posting the same intent must not produce duplicate journals |

### 3.2 Accounts & Reports
| Term | Definition |
|---|---|
| **COA (Chart of Accounts)** | Account list; report group (Asset/Liability/Equity/Revenue/Expense) + account type (BANK, AR, AP, INVENTORY, ...) |
| **Group vs Detail Account** | Group = automatic sum of children; only detail accounts may be posted |
| **Trial Balance** | All accounts debit vs credit must balance |
| **Balance Sheet (Financial Position)** | Assets = Liabilities + Equity at a point in time |
| **Profit & Loss** | Revenue − Expenses for a period |
| **Cash Flow** | Cash in/out movements (operating, investing, financing) |
| **ECL** | Expected Credit Loss — allowance for uncollectible receivables (PSAK 71) |
| **NRV** | Net realizable value of inventory (PSAK 14) |
| **OCI** | Other Comprehensive Income (e.g. revaluation surplus) |

### 3.3 Transactions & Documents
| Term | Definition |
|---|---|
| **SQ → SO → DP → DO → INV** | Sales flow: Quotation → Order → Down Payment → Delivery → Invoice |
| **PR → PO → GRN** | Purchase flow: Requisition → Order → Goods Receipt |
| **COGS** | Cost of Goods Sold — cost of goods sold |
| **WIP** | Work In Progress — goods in production |
| **BOM** | Bill of Materials — list of product components |
| **Job Costing** | Cost allocation (material, labor, overhead) per job/order |

### 3.4 Tax & Standards
| Term | Definition |
|---|---|
| **Input VAT** | VAT on purchases — **asset** (creditable), not a payable |
| **Output VAT** | VAT on sales — **liability** (paid to the state) |
| **Final Income Tax for MSMEs** | Final tax under taxpayer scheme and effective regulation; 0.5% and the Rp 4.8 B threshold only apply when meeting the relevant period's conditions |
| **Deferred Tax** | Tax impact of temporary differences (PSAK 46) |
| **SAK EMKM / ETAP / General** | Reporting framework — from a single source of records |

---

## 4. Account Code Map (Summary)

| Code | Account | Related UI Category |
|---|---|---|
| 1101 | Cash | Cash Money In/Out |
| 1102 | Bank | Bank transfers, reconciliation |
| 1201 | Accounts Receivable | Customer bills |
| 1301 | Inventory | Trading stock |
| 1401 | Fixed Assets | Machinery, vehicles, buildings |
| 2101 | Accounts Payable | Supplier bills |
| 2201 | Advance Sales | Customer down payments |
| 2202 | VAT Payable | Output VAT |
| 2203 | Income Tax Payable | Withheld/paid income tax |
| 3101 | Capital | Owner contributions |
| 3301 | Current Earnings | Current period profit/loss |
| 4101 | Sales Revenue | Sale of goods/services |
| 5101 | COGS | Cost of goods sold |
| 5201 | Salary Expense | Employee salaries |

*(Full COA: see [ACCOUNTING_ENGINE.md](ACCOUNTING_ENGINE.md) §3.)*

---

## 5. Terminology Usage Rules

1. **Layperson UI** always uses the user-layer terms — accounting terms do not appear (except in Accountant Mode).
2. **Technical documents** (engine, data model, architecture) use consistent accounting terms — column names & enums in English (`payable_cents`, `receivable_cents`).
3. **One concept = one term** — never switch between synonyms for the same concept (standard: **Payables** = liabilities, **Receivables** = assets).
4. Account codes follow ACCOUNTING_ENGINE.md §3 — single reference, not duplicated in the UI.

---

*This document is a reference for product, UI/UX, and engineering teams. Update it when new terms are introduced.*
