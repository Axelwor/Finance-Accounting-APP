# NextReport Rendering Sidecar

Zero-dependency Node.js service that renders NextReport YAML document
templates to HTML (and minimal PDF). It implements the contract expected by
the Go backend (`backend/internal/reports/templates.go`, env `NEXTREPORT_URL`,
default `http://localhost:3100`).

No npm packages are required — the server uses only the Node standard library.
The YAML parser is a small built-in parser for the documented template
subset (see below).

## Run locally

```sh
node nextreport/server.js        # listens on :3100 (override with PORT=...)
```

## Docker

```sh
docker build -t nextreport ./nextreport
docker run -p 3100:3100 nextreport
```

In `docker-compose.yml` the service is named `nextreport` and the api uses
`NEXTREPORT_URL=http://nextreport:3100`.

## HTTP contract

### GET /health

```json
{ "status": "ok", "version": "1.0.0", "uptime": 123 }
```

### POST /render

Request JSON body:

```json
{
  "template_yaml": "title: Invoice\nsubtitle: ...\nheader_fields:\n  - number: Invoice Number\nsections:\n  - title: Line Items\n    table: rows\n    columns:\n      - code: Item Code\n",
  "data": {
    "number": "INV-0001",
    "rows": [{ "code": "SKU-1", "qty": 2 }]
  },
  "format": "html"
}
```

| Field           | Type   | Description                                            |
|-----------------|--------|--------------------------------------------------------|
| `template_yaml` | string | NextReport YAML template (subset below)                |
| `data`          | object | Values bound into header fields and table rows         |
| `format`        | string | `html` (default) or `pdf`                              |

Responses:

- `format=html` → `200 text/html; charset=utf-8` with the rendered document.
- `format=pdf` → `200 application/pdf` with a real, valid PDF.
- `400` with `{ "error": "...", "message": "..." }` on invalid JSON, missing
  `title`, or unsupported format; `500` on render failure.

## PDF strategy

PDFs are produced by a minimal built-in PDF 1.4 writer (Helvetica, multi-page,
text-only). The rendered HTML is flattened to text lines and laid out on
Letter pages. This is a **dependency-free, print-oriented text PDF**: it is a
valid, openable PDF but does not carry CSS styling, tables-as-graphics, or
fonts beyond WinAnsi text. If richer PDF output is needed later, swap in
Puppeteer/wkhtmltopdf behind the same endpoint — the request/response contract
does not change.

## Template YAML subset

```yaml
title: Document Title          # required scalar
subtitle: Optional subtitle    # optional scalar

header_fields:                 # key/value pairs bound to data keys
  - number: Number             # label defaults to the key
  - date: Date
  - customer_name: Customer

sections:                      # ordered body sections
  - title: Line Items          # section heading
    table: rows                # data key holding an array of row objects
    columns:                   # columns bound to row keys
      - code: Item Code        # "key: Label"
      - qty: Quantity
      - amount: Amount

  - title: Notes               # static-text section alternative
    content: Free text shown as-is.

footer_text: Footer line       # optional scalar
```

Binding rules:

- `header_fields` entries map a data key to a display label; the value is
  `data[key]`, or `—` when absent.
- `sections[].table` names the `data` key holding the row array (default
  `rows` when `table:` has no value).
- `columns` entries map a row object key to a column label.
- Scalar values may be quoted; `#` starts a comment; blank lines are ignored.
  Anchors, multi-document streams, and block scalars are not supported.

## Templates

`nextreport/templates/` ships one template per `document_type` enum value:

invoice, purchase_order, delivery_order, tax_invoice, withholding_slip,
customer_statement, supplier_statement, payment_voucher, receipt_voucher,
journal_voucher, stock_card, trial_balance, profit_loss, balance_sheet,
cash_flow, ar_aging, ap_aging, asset_register, stock_opname.

The same YAML is seeded into the `report_templates` table by migration
`backend/migrations/000038_seed_report_templates.up.sql`.
