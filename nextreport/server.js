'use strict';

// NextReport rendering sidecar — zero-dependency Node service.
// Contract (see README.md):
//   GET  /health                 -> { status, version, uptime }
//   POST /render {template_yaml, data, format}
//     format=html -> text/html (full render)
//     format=pdf  -> application/pdf (minimal hand-rolled PDF, text-only layout)

const http = require('http');

const VERSION = '1.0.0';
const PORT = Number(process.env.PORT || 3100);
const START_TIME = Date.now();

// ---------------------------------------------------------------------------
// Minimal YAML parser for the documented NextReport template subset:
//   - `key: value` scalars
//   - `key:` followed by an indented block:
//       * list of scalars:            `- item`
//       * list of single-key maps:    `- key: Label`
//       * map of scalars:             `key: value`
// Comments (#) and blank lines are ignored. No anchors, no multi-line strings.
// ---------------------------------------------------------------------------
function parseYAML(src) {
  const lines = [];
  for (const raw of String(src || '').split(/\r?\n/)) {
    const noComment = raw.replace(/\s#.*$/, '');
    if (!noComment.trim()) continue;
    if (noComment.trim().startsWith('#')) continue;
    const indent = noComment.match(/^\s*/)[0].replace(/\t/g, '  ').length;
    lines.push({ indent, text: noComment.trim() });
  }

  let pos = 0;
  function parseBlock(indent) {
    // Decide list vs map from first line at this indent.
    if (pos >= lines.length || lines[pos].indent < indent) return null;
    const isList = lines[pos].text.startsWith('- ');
    if (isList) {
      const arr = [];
      while (pos < lines.length && lines[pos].indent === indent && lines[pos].text.startsWith('- ')) {
        const item = lines[pos].text.slice(2).trim();
        pos++;
        const kv = splitKV(item);
        if (kv) {
          // map item; deeper-indented continuation keys merge into this item
          const merged = { [kv.key]: kv.val !== '' ? unquote(kv.val) : '' };
          if (pos < lines.length && lines[pos].indent > indent) {
            const child = parseBlock(lines[pos].indent);
            if (child && typeof child === 'object' && !Array.isArray(child)) {
              Object.assign(merged, child);
            } else if (child !== null) {
              merged[kv.key] = child;
            }
          }
          arr.push(merged);
        } else {
          arr.push(unquote(item));
        }
      }
      return arr;
    }
    const obj = {};
    while (pos < lines.length && lines[pos].indent === indent && !lines[pos].text.startsWith('- ')) {
      const kv = splitKV(lines[pos].text);
      if (!kv) { pos++; continue; }
      pos++;
      if (kv.val === '') {
        const child = (pos < lines.length && lines[pos].indent > indent) ? parseBlock(lines[pos].indent) : null;
        obj[kv.key] = child === null ? '' : child;
      } else {
        obj[kv.key] = unquote(kv.val);
      }
    }
    return obj;
  }

  const root = parseBlock(0);
  return (root && typeof root === 'object') ? root : {};
}

function splitKV(text) {
  const m = text.match(/^([^:\s][^:]*):\s*(.*)$/);
  if (!m) return null;
  return { key: m[1].trim(), val: m[2].trim() };
}

function unquote(v) {
  if ((v.startsWith('"') && v.endsWith('"')) || (v.startsWith("'") && v.endsWith("'"))) {
    return v.slice(1, -1);
  }
  return v;
}

// ---------------------------------------------------------------------------
// Rendering
// ---------------------------------------------------------------------------
function esc(s) {
  return String(s)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}

function lookup(data, key) {
  if (data == null) return '';
  if (typeof data === 'object' && key in data) return data[key];
  return '';
}

// Resolve a header field spec to [label, dataKey].
function fieldSpec(entry) {
  if (typeof entry === 'string') return [entry, entry];
  const key = Object.keys(entry).find((k) => k !== '_label');
  const label = entry._label || (typeof entry[key] === 'string' ? entry[key] : key);
  return [label, key];
}

function columnSpec(entry) {
  if (typeof entry === 'string') return { key: entry, label: entry };
  const key = Object.keys(entry).find((k) => k !== '_label');
  const label = entry._label || (typeof entry[key] === 'string' ? entry[key] : key);
  return { key, label };
}

function tableRows(section, data) {
  const srcKey = (typeof section.table === 'string' && section.table) ? section.table : 'rows';
  const src = lookup(data, srcKey);
  if (!Array.isArray(src)) return [];
  return src;
}

function renderHTML(tpl, data) {
  const title = tpl.title || 'Report';
  const parts = [];
  parts.push('<!DOCTYPE html>');
  parts.push('<html lang="en"><head><meta charset="utf-8">');
  parts.push(`<title>${esc(title)}</title>`);
  parts.push('<style>' +
    'body{font-family:-apple-system,"Segoe UI",Roboto,Helvetica,Arial,sans-serif;margin:32px;color:#1f2937;line-height:1.5}' +
    '.nr-header{border-bottom:2px solid #1f2937;padding-bottom:12px;margin-bottom:20px}' +
    '.nr-title{font-size:24px;font-weight:700;margin:0}' +
    '.nr-subtitle{font-size:13px;color:#6b7280;margin:4px 0 0}' +
    '.nr-fields{display:grid;grid-template-columns:repeat(auto-fit,minmax(220px,1fr));gap:10px;margin:16px 0;background:#f9fafb;padding:14px;border-radius:6px}' +
    '.nr-field-label{font-size:11px;font-weight:600;text-transform:uppercase;color:#6b7280;letter-spacing:.04em}' +
    '.nr-field-value{font-size:14px}' +
    '.nr-section{margin:24px 0}' +
    '.nr-section-title{font-size:16px;font-weight:600;border-left:4px solid #1f2937;padding-left:10px;margin-bottom:10px}' +
    'table.nr-table{width:100%;border-collapse:collapse;font-size:13px}' +
    '.nr-table th{background:#1f2937;color:#fff;padding:8px;text-align:left}' +
    '.nr-table td{border-bottom:1px solid #e5e7eb;padding:8px}' +
    '.nr-table tr:nth-child(even){background:#f9fafb}' +
    '.nr-content{font-size:13px;color:#374151;white-space:pre-wrap}' +
    '.nr-footer{margin-top:36px;border-top:1px solid #e5e7eb;padding-top:10px;font-size:11px;color:#6b7280;text-align:center}' +
    '@media print{body{margin:12mm}}' +
    '</style></head><body>');

  parts.push('<div class="nr-header">');
  parts.push(`<h1 class="nr-title">${esc(title)}</h1>`);
  if (tpl.subtitle) parts.push(`<p class="nr-subtitle">${esc(tpl.subtitle)}</p>`);
  parts.push(`<p class="nr-subtitle">Generated ${new Date().toISOString()}</p>`);
  parts.push('</div>');

  if (Array.isArray(tpl.header_fields) || (tpl.header_fields && typeof tpl.header_fields === 'object')) {
    const entries = Array.isArray(tpl.header_fields)
      ? tpl.header_fields
      : Object.entries(tpl.header_fields).map(([k, v]) => ({ [k]: v }));
    parts.push('<div class="nr-fields">');
    for (const entry of entries) {
      const [label, key] = fieldSpec(entry);
      const val = lookup(data, key);
      const shown = (val === '' || val == null) ? '—' : val;
      parts.push(`<div><div class="nr-field-label">${esc(label)}</div><div class="nr-field-value">${esc(shown)}</div></div>`);
    }
    parts.push('</div>');
  }

  const sections = Array.isArray(tpl.sections) ? tpl.sections : [];
  for (const section of sections) {
    if (!section || typeof section !== 'object') continue;
    parts.push('<div class="nr-section">');
    if (section.title) parts.push(`<div class="nr-section-title">${esc(section.title)}</div>`);
    if (Array.isArray(section.columns)) {
      const cols = section.columns.map(columnSpec);
      const rows = tableRows(section, data);
      parts.push('<table class="nr-table"><thead><tr>');
      for (const c of cols) parts.push(`<th>${esc(c.label)}</th>`);
      parts.push('</tr></thead><tbody>');
      if (rows.length === 0) {
        parts.push(`<tr><td colspan="${cols.length}">No data</td></tr>`);
      }
      for (const row of rows) {
        parts.push('<tr>');
        for (const c of cols) {
          const v = row && typeof row === 'object' ? lookup(row, c.key) : '';
          parts.push(`<td>${esc(v == null ? '' : (typeof v === 'object' ? JSON.stringify(v) : v))}</td>`);
        }
        parts.push('</tr>');
      }
      parts.push('</tbody></table>');
    } else if (section.content) {
      parts.push(`<div class="nr-content">${esc(section.content)}</div>`);
    }
    parts.push('</div>');
  }

  if (tpl.footer_text) parts.push(`<div class="nr-footer">${esc(tpl.footer_text)}</div>`);
  parts.push('</body></html>');
  return parts.join('\n');
}

// ---------------------------------------------------------------------------
// Minimal PDF writer (PDF 1.4, Helvetica, text-only, multi-page).
// Extracts text lines from the rendered HTML so the PDF is dependency-free.
// ---------------------------------------------------------------------------
function htmlToTextLines(html) {
  const text = html
    .replace(/<style[\s\S]*?<\/style>/gi, '')
    .replace(/<\/tr>/gi, '\n')
    .replace(/<\/(td|th)>/gi, '\t')
    .replace(/<(br|\/p|\/div|\/h1|\/h2|\/h3|\/li)[^>]*>/gi, '\n')
    .replace(/<[^>]+>/g, '')
    .replace(/&amp;/g, '&').replace(/&lt;/g, '<').replace(/&gt;/g, '>').replace(/&quot;/g, '"').replace(/&#39;/g, "'")
    .replace(/&nbsp;/g, ' ');
  const lines = [];
  for (const raw of text.split('\n')) {
    const line = raw.replace(/[ \t]+/g, ' ').trim();
    if (line) lines.push(line);
  }
  return lines;
}

function pdfEscape(s) {
  return String(s)
    .replace(/[^\x20-\x7e\u00a0-\u00ff]/g, '?')
    .replace(/\\/g, '\\\\')
    .replace(/\(/g, '\\(')
    .replace(/\)/g, '\\)');
}

function renderPDF(title, lines) {
  const PAGE_W = 612, PAGE_H = 792, MARGIN = 50, SIZE = 10, LEADING = 14;
  const perPage = Math.floor((PAGE_H - 2 * MARGIN) / LEADING);
  const pages = [];
  for (let i = 0; i < lines.length; i += perPage) pages.push(lines.slice(i, i + perPage));
  if (pages.length === 0) pages.push(['(no content)']);

  const objects = []; // each: "N 0 obj ... endobj"
  const pageObjNums = [];
  const contentObjNums = [];
  const fontObjNum = 3;
  const firstPageObj = 4;
  for (let i = 0; i < pages.length; i++) {
    pageObjNums.push(firstPageObj + i * 2);
    contentObjNums.push(firstPageObj + i * 2 + 1);
  }

  objects.push(`1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n`);
  objects.push(`2 0 obj\n<< /Type /Pages /Count ${pages.length} /Kids [${pageObjNums.map((n) => `${n} 0 R`).join(' ')}] >>\nendobj\n`);
  objects.push(`3 0 obj\n<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>\nendobj\n`);

  pages.forEach((pageLines, i) => {
    let stream = `BT\n/F1 ${SIZE} Tf\n${LEADING} TL\n${MARGIN} ${PAGE_H - MARGIN} Td\n`;
    stream += `(${pdfEscape(title)}) Tj T*\n`;
    stream += `() Tj T*\n`;
    for (const line of pageLines) stream += `(${pdfEscape(line)}) Tj T*\n`;
    stream += 'ET\n';
    objects.push(`${pageObjNums[i]} 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 ${PAGE_W} ${PAGE_H}] /Resources << /Font << /F1 ${fontObjNum} 0 R >> >> /Contents ${contentObjNums[i]} 0 R >>\nendobj\n`);
    objects.push(`${contentObjNums[i]} 0 obj\n<< /Length ${Buffer.byteLength(stream)} >>\nstream\n${stream}endstream\nendobj\n`);
  });

  let pdf = '%PDF-1.4\n';
  const offsets = [];
  for (const obj of objects) {
    offsets.push(Buffer.byteLength(pdf));
    pdf += obj;
  }
  const xrefPos = Buffer.byteLength(pdf);
  const count = objects.length + 1;
  pdf += `xref\n0 ${count}\n`;
  pdf += '0000000000 65535 f \n';
  for (const off of offsets) pdf += `${String(off).padStart(10, '0')} 00000 n \n`;
  pdf += `trailer\n<< /Size ${count} /Root 1 0 R >>\nstartxref\n${xrefPos}\n%%EOF\n`;
  return Buffer.from(pdf, 'binary');
}

// ---------------------------------------------------------------------------
// HTTP server
// ---------------------------------------------------------------------------
function sendJSON(res, status, obj) {
  res.writeHead(status, { 'Content-Type': 'application/json' });
  res.end(JSON.stringify(obj));
}

function readBody(req, cb) {
  let body = '';
  let size = 0;
  req.on('data', (chunk) => {
    size += chunk.length;
    if (size > 5 * 1024 * 1024) {
      req.destroy();
      return;
    }
    body += chunk;
  });
  req.on('end', () => cb(null, body));
  req.on('error', (err) => cb(err));
}

const server = http.createServer((req, res) => {
  const u = new URL(req.url, `http://${req.headers.host || 'localhost'}`);

  if (req.method === 'GET' && u.pathname === '/health') {
    sendJSON(res, 200, {
      status: 'ok',
      version: VERSION,
      uptime: Math.floor((Date.now() - START_TIME) / 1000),
    });
    return;
  }

  if (req.method === 'POST' && u.pathname === '/render') {
    readBody(req, (err, body) => {
      if (err) return sendJSON(res, 400, { error: 'read_failed', message: err.message });
      let payload;
      try {
        payload = JSON.parse(body || '{}');
      } catch (e) {
        return sendJSON(res, 400, { error: 'invalid_json', message: e.message });
      }
      const templateYaml = payload.template_yaml || '';
      const data = payload.data || {};
      const format = payload.format || 'html';

      let tpl;
      try {
        tpl = parseYAML(templateYaml);
      } catch (e) {
        return sendJSON(res, 400, { error: 'yaml_parse_failed', message: e.message });
      }
      if (!tpl || typeof tpl !== 'object' || !tpl.title) {
        return sendJSON(res, 400, { error: 'invalid_template', message: 'template_yaml must define at least a title' });
      }

      try {
        const html = renderHTML(tpl, data);
        if (format === 'html') {
          res.writeHead(200, { 'Content-Type': 'text/html; charset=utf-8' });
          res.end(html);
        } else if (format === 'pdf') {
          const pdf = renderPDF(String(tpl.title), htmlToTextLines(html));
          res.writeHead(200, {
            'Content-Type': 'application/pdf',
            'Content-Disposition': 'inline; filename="report.pdf"',
            'X-NextReport-Pdf': 'minimal-text-pdf',
          });
          res.end(pdf);
        } else {
          sendJSON(res, 400, { error: 'unsupported_format', message: `format must be html or pdf, got: ${format}` });
        }
      } catch (e) {
        sendJSON(res, 500, { error: 'render_failed', message: e.message });
      }
    });
    return;
  }

  sendJSON(res, 404, { error: 'not_found', message: `${req.method} ${u.pathname} not found` });
});

server.listen(PORT, () => {
  console.log(`[nextreport] listening on :${PORT} (v${VERSION})`);
});
