/**
 * Codemod: raw <button className="btn ..."> → m3 Button / IconButton wrappers.
 *
 * AST-based via @babel/parser (TSX) — safe against arrow functions and
 * multi-line JSX attributes (regex approaches break on `=>` inside handlers).
 *
 * Usage: node scripts/migrate-buttons.cjs [--write]
 *        (default: dry-run report to /tmp/btn-codemod-report.txt)
 */
const parser = require("@babel/parser");
const fs = require("node:fs");
const path = require("node:path");

const WRITE = process.argv.includes("--write");
const SRC = "src";
const M3_MODULE = path.join(SRC, "components/m3");

function walk(dir, out = []) {
  for (const name of fs.readdirSync(dir)) {
    const p = path.join(dir, name);
    const st = fs.statSync(p);
    if (st.isDirectory()) {
      if (p === path.join(SRC, "components", "m3")) continue; // the wrappers themselves
      walk(p, out);
    } else if (name.endsWith(".tsx") && !name.endsWith(".test.tsx")) out.push(p);
  }
  return out;
}

const CONSUMED = new Set([
  "btn",
  "btn--primary",
  "btn--secondary",
  "btn--ghost",
  "btn--danger",
  "btn--negative",
  "btn--ink",
  "btn--success",
  "btn--sm",
  "btn--xs",
  "btn--full",
  "btn--icon",
  "btn--disabled",
]);

function classify(classes) {
  const set = new Set(classes);
  if (!set.has("btn")) return null;
  const isIcon = set.has("btn--icon");
  let variant;
  let danger = false;
  let success = false;
  if (set.has("btn--primary")) variant = "filled";
  else if (set.has("btn--secondary")) variant = "outlined";
  else if (set.has("btn--ghost")) variant = "text";
  else if (set.has("btn--danger") || set.has("btn--negative")) {
    variant = "outlined";
    danger = true;
  } else if (set.has("btn--ink")) variant = "tonal";
  else if (set.has("btn--success")) {
    variant = "filled";
    success = true;
  } else variant = "tonal";
  const size = set.has("btn--xs") ? "xs" : set.has("btn--sm") ? "sm" : null;
  const fullWidth = set.has("btn--full");
  const leftover = classes.filter((c) => !CONSUMED.has(c));
  return { isIcon, variant, danger, success, size, fullWidth, leftover };
}

/** Identifiers declared at module scope (imports + top-level bindings). */
function moduleScopeNames(ast) {
  const declared = new Set();
  function collectNames(node) {
    if (!node) return;
    if (node.type === "Identifier") declared.add(node.name);
    else if (node.type === "ObjectPattern") node.properties.forEach((p) => collectNames(p.value || p.argument));
    else if (node.type === "ArrayPattern") node.elements.forEach((e) => collectNames(e));
  }
  for (const st of ast.program.body) {
    if (st.type === "ImportDeclaration" && st.specifiers) {
      for (const spec of st.specifiers) declared.add(spec.local.name);
    } else if (st.type === "FunctionDeclaration" && st.id) {
      declared.add(st.id.name);
    } else if (st.type === "ClassDeclaration" && st.id) {
      declared.add(st.id.name);
    } else if (st.type === "VariableDeclaration") {
      for (const d of st.declarations) collectNames(d.id);
    }
  }
  return declared;
}

/** Depth-first traversal that visits every AST node (skips loc/comments). */
function visitAll(node, fn) {
  fn(node);
  for (const key in node) {
    if (key === "loc" || key === "start" || key === "end" || key === "leadingComments" || key === "trailingComments" || key === "extra") continue;
    const child = node[key];
    if (child && typeof child.type === "string") visitAll(child, fn);
    else if (Array.isArray(child)) {
      for (const c of child) {
        if (c && typeof c.type === "string") visitAll(c, fn);
      }
    }
  }
}

const report = [];
let totalConverted = 0;
let totalFiles = 0;
const skipped = [];
const refSites = [];
const unsupportedAttrs = new Map();
const spreadAttrSites = [];

for (const file of walk(SRC)) {
  const source = fs.readFileSync(file, "utf8");
  let ast;
  try {
    ast = parser.parse(source, { sourceType: "module", plugins: ["typescript", "jsx"] });
  } catch (e) {
    skipped.push(`${file}: PARSE ERROR ${e.message}`);
    continue;
  }

  const buttons = [];
  visitAll(ast, (node) => {
    if (
      node.type === "JSXElement" &&
      node.openingElement &&
      node.openingElement.name.type === "JSXIdentifier" &&
      node.openingElement.name.name === "button"
    ) {
      buttons.push(node);
    }
  });

  const classified = [];
  let usesButton = false;
  let usesIconButton = false;

  for (const node of buttons) {
    const opening = node.openingElement;
    const classNameAttr = opening.attributes.find(
      (a) => a.type === "JSXAttribute" && a.name.name === "className",
    );
    if (!classNameAttr) continue;
    const value = classNameAttr.value;
    if (!value || value.type !== "StringLiteral") {
      skipped.push(`${file}: dynamic className (line ${node.loc.start.line})`);
      continue;
    }
    const info = classify(value.value.split(/\s+/).filter(Boolean));
    if (!info) continue;

    for (const a of opening.attributes) {
      if (a.type === "JSXAttribute" && a.name.name === "ref") {
        refSites.push(`${file}:${node.loc.start.line}`);
      }
      if (a.type === "JSXSpreadAttribute") {
        spreadAttrSites.push(`${file}:${node.loc.start.line}`);
      }
    }

    if (info.isIcon) usesIconButton = true;
    else usesButton = true;
    classified.push({ node, info });
  }

  if (!classified.length) continue;
  totalFiles++;

  const declared = moduleScopeNames(ast);
  const buttonName = declared.has("Button") ? "M3Button" : "Button";
  const iconName = declared.has("IconButton") ? "M3IconButton" : "IconButton";

  const edits = [];
  for (const { node, info } of classified) {
    const opening = node.openingElement;
    const Target = info.isIcon ? iconName : buttonName;

    const props = [];
    if (!info.isIcon) props.push(`variant="${info.variant}"`);
    if (info.size) props.push(`size="${info.size}"`);
    if (info.danger) props.push("danger");
    if (info.success) props.push("success");
    if (info.fullWidth) props.push("fullWidth");

    for (const a of opening.attributes) {
      if (a.type !== "JSXAttribute") {
        // JSXSpreadAttribute — keep verbatim (tracked above)
        props.push(source.slice(a.start, a.end));
        continue;
      }
      const name = a.name.name;
      if (name === "className") continue;
      if (name === "type") {
        if (a.value && a.value.type === "StringLiteral" && a.value.value === "submit") {
          props.push('type="submit"');
        }
        continue;
      }
      if (name === "aria-label" && info.isIcon) {
        props.push(`label=${source.slice(a.value.start, a.value.end)}`);
        continue;
      }
      const supported = new Set([
        "onClick", "disabled", "style", "title", "id", "name", "value", "form", "key",
        "aria-label", "aria-haspopup", "aria-expanded", "aria-describedby", "aria-hidden",
        "role", "hidden",
      ]);
      if (!supported.has(name) && !name.startsWith("data-") && !name.startsWith("aria-")) {
        unsupportedAttrs.set(name, (unsupportedAttrs.get(name) || 0) + 1);
      }
      props.push(source.slice(a.start, a.end));
    }
    if (info.leftover.length) props.push(`className="${info.leftover.join(" ")}"`);

    const start = opening.start;
    const end = opening.end;
    const indent = /([ \t]*)$/.exec(source.slice(0, start))[1];
    const multiline = props.length > 2;
    const attrsText = multiline
      ? "\n" + props.map((p) => `${indent}  ${p}`).join("\n") + `\n${indent}`
      : props.length
        ? " " + props.join(" ")
        : "";

    if (opening.selfClosing) {
      edits.push({ start, end, text: `<${Target}${attrsText} />` });
    } else {
      edits.push({ start, end, text: `<${Target}${attrsText}>` });
      const ct = node.closingElement.name;
      edits.push({ start: ct.start, end: ct.end, text: Target });
    }
    totalConverted++;
  }

  let out = source;
  const sorted = [...edits].sort((a, b) => b.start - a.start);
  for (const e of sorted) {
    out = out.slice(0, e.start) + e.text + out.slice(e.end);
  }

  const importNames = [];
  if (usesButton) importNames.push(buttonName === "Button" ? "Button" : `Button as ${buttonName}`);
  if (usesIconButton) importNames.push(iconName === "IconButton" ? "IconButton" : `IconButton as ${iconName}`);
  const rel = path.relative(path.dirname(file), M3_MODULE).split(path.sep).join("/");
  const importPath = rel.startsWith(".") ? rel : "./" + rel;
  const importStmt = `import { ${importNames.join(", ")} } from "${importPath}";`;

  const lastImport = ast.program.body.filter((s) => s.type === "ImportDeclaration").pop();
  if (lastImport) {
    out = out.slice(0, lastImport.end) + "\n" + importStmt + out.slice(lastImport.end);
  } else {
    out = importStmt + "\n" + out;
  }

  report.push(`${WRITE ? "WROTE" : "DRY"} ${file}: ${classified.length} buttons (${importNames.join(", ")})`);
  if (WRITE) fs.writeFileSync(file, out);
}

report.push("");
report.push(`total buttons: ${totalConverted}, files: ${totalFiles}, mode: ${WRITE ? "WRITE" : "DRY RUN"}`);
report.push(`skipped: ${skipped.length}`);
skipped.forEach((s) => report.push("  " + s));
report.push(`ref sites (verify): ${refSites.length}`);
refSites.forEach((s) => report.push("  " + s));
report.push(`spread-attr sites (verify): ${spreadAttrSites.length}`);
spreadAttrSites.forEach((s) => report.push("  " + s));
report.push(`unsupported attrs seen: ${JSON.stringify(Object.fromEntries(unsupportedAttrs))}`);

fs.writeFileSync("/tmp/btn-codemod-report.txt", report.join("\n"));
console.log(`done: ${totalConverted} buttons, ${totalFiles} files, mode=${WRITE ? "WRITE" : "DRY RUN"}`);
