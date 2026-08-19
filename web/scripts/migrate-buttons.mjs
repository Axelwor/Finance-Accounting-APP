/**
 * Codemod: raw <button className="btn ..."> → m3 Button / IconButton wrappers.
 *
 * AST-based via the TypeScript compiler — safe against arrow functions and
 * multi-line JSX attributes (regex approaches break on `=>` inside handlers).
 *
 * Usage: node scripts/migrate-buttons.mjs [--write]
 *        (default: dry-run report to /tmp/btn-codemod-report.txt)
 */
import ts from "typescript";
import { readFileSync, writeFileSync } from "node:fs";
import { readdirSync, statSync } from "node:fs";
import { join, dirname, relative } from "node:path";

const WRITE = process.argv.includes("--write");
const SRC = "src";
const M3_MODULE = join(SRC, "components/m3");

/** Walk .tsx files, skipping tests. */
function walk(dir, out = []) {
  for (const name of readdirSync(dir)) {
    const p = join(dir, name);
    const st = statSync(p);
    if (st.isDirectory()) walk(p, out);
    else if (name.endsWith(".tsx") && !name.endsWith(".test.tsx")) out.push(p);
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

/** Map a class list to wrapper props. Returns null when not a .btn button. */
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

/** Collect identifiers declared at module scope (imports + top-level bindings). */
function moduleScopeNames(sf) {
  const declared = new Set();
  for (const st of sf.statements) {
    if (ts.isImportDeclaration(st) && st.importClause) {
      if (st.importClause.name) declared.add(st.importClause.name.text);
      const nb = st.importClause.namedBindings;
      if (nb && ts.isNamedImports(nb)) {
        for (const el of nb.elements) {
          declared.add(el.name.text);
          if (el.propertyName) declared.add(el.propertyName.text);
        }
      } else if (nb && ts.isNamespaceImport(nb)) {
        declared.add(nb.name.text);
      }
    } else if (ts.isFunctionDeclaration(st) && st.name) {
      declared.add(st.name.text);
    } else if (ts.isClassDeclaration(st) && st.name) {
      declared.add(st.name.text);
    } else if (ts.isVariableStatement(st)) {
      for (const d of st.declarationList.declarations) {
        for (const n of d.name.getText(sf).split(/[^\w$]+/)) {
          if (n) declared.add(n);
        }
      }
    }
  }
  return declared;
}

const report = [];
let totalConverted = 0;
let totalFiles = 0;
const skipped = [];
const refSites = [];

for (const file of walk(SRC)) {
  const source = readFileSync(file, "utf8");
  const sf = ts.createSourceFile(file, source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);

  /** Collect button nodes with classification. */
  const buttons = [];
  function visit(node) {
    if (
      (ts.isJsxElement(node) || ts.isJsxSelfClosingElement(node)) &&
      ts.isIdentifier(node.tagName) &&
      node.tagName.text === "button"
    ) {
      buttons.push(node);
    }
    ts.forEachChild(node, visit);
  }
  visit(sf);

  const classified = [];
  let usesButton = false;
  let usesIconButton = false;

  for (const node of buttons) {
    const opening = ts.isJsxElement(node) ? node.openingElement : node;
    const classNameAttr = opening.attributes.properties.find(
      (p) => ts.isJsxAttribute(p) && p.name.getText(sf) === "className",
    );
    if (!classNameAttr || !ts.isJsxAttribute(classNameAttr)) continue;
    const init = classNameAttr.initializer;
    if (!init || !ts.isStringLiteral(init)) {
      skipped.push(`${file}: dynamic className (line ${sf.getLineAndCharacterOfPosition(node.getStart(sf)).line + 1})`);
      continue;
    }
    const info = classify(init.text.split(/\s+/).filter(Boolean));
    if (!info) continue;

    for (const p of opening.attributes.properties) {
      if (ts.isJsxAttribute(p) && p.name.getText(sf) === "ref") {
        refSites.push(`${file}:${sf.getLineAndCharacterOfPosition(node.getStart(sf)).line + 1}`);
      }
    }

    if (info.isIcon) usesIconButton = true;
    else usesButton = true;
    classified.push({ node, info });
  }

  if (!classified.length) continue;
  totalFiles++;

  // Resolve local names before building replacements.
  const declared = moduleScopeNames(sf);
  const buttonName = declared.has("Button") ? "M3Button" : "Button";
  const iconName = declared.has("IconButton") ? "M3IconButton" : "IconButton";

  const edits = [];
  for (const { node, info } of classified) {
    const opening = ts.isJsxElement(node) ? node.openingElement : node;
    const Target = info.isIcon ? iconName : buttonName;

    const props = [];
    if (!info.isIcon) props.push(`variant="${info.variant}"`);
    if (info.size) props.push(`size="${info.size}"`);
    if (info.danger) props.push("danger");
    if (info.success) props.push("success");
    if (info.fullWidth) props.push("fullWidth");

    for (const p of opening.attributes.properties) {
      if (!ts.isJsxAttribute(p)) continue; // spread — keep verbatim
      const name = p.name.getText(sf);
      if (name === "className") continue;
      if (name === "type") {
        const t = p.initializer && ts.isStringLiteral(p.initializer) ? p.initializer.text : null;
        if (t === "submit") props.push('type="submit"');
        continue;
      }
      if (name === "aria-label" && info.isIcon) {
        const raw = p.initializer ? p.initializer.getText(sf) : '""';
        props.push(`label=${raw}`);
        continue;
      }
      props.push(p.getText(sf));
    }
    if (info.leftover.length) props.push(`className="${info.leftover.join(" ")}"`);

    const indentMatch = source.slice(0, node.getStart(sf)).match(/([ \t]*)$/);
    const indent = indentMatch ? indentMatch[1] : "";
    const multiline = props.length > 2;
    const attrsText = multiline
      ? "\n" + props.map((p) => `${indent}  ${p}`).join("\n") + `\n${indent}`
      : props.length
        ? " " + props.join(" ")
        : "";

    const start = opening.getStart(sf);
    const end = opening.getEnd();
    if (ts.isJsxSelfClosingElement(node)) {
      edits.push({ start, end, text: `<${Target}${attrsText} />` });
    } else {
      edits.push({ start, end, text: `<${Target}${attrsText}>` });
      edits.push({
        start: node.closingElement.tagName.getStart(sf),
        end: node.closingElement.tagName.getEnd(),
        text: Target,
      });
    }
    totalConverted++;
  }

  let out = source;
  const sorted = [...edits].sort((a, b) => b.start - a.start);
  for (const e of sorted) {
    out = out.slice(0, e.start) + e.text + out.slice(e.end);
  }

  // Build the import statement with aliases.
  const importNames = [];
  if (usesButton) importNames.push(buttonName === "Button" ? "Button" : `Button as ${buttonName}`);
  if (usesIconButton) importNames.push(iconName === "IconButton" ? "IconButton" : `IconButton as ${iconName}`);
  const rel = relative(dirname(file), M3_MODULE).replace(/\\/g, "/");
  const importPath = rel.startsWith(".") ? rel : "./" + rel;
  const importStmt = `import { ${importNames.join(", ")} } from "${importPath}";`;

  const lastImport = sf.statements.filter(ts.isImportDeclaration).pop();
  if (lastImport) {
    const pos = lastImport.getEnd();
    out = out.slice(0, pos) + "\n" + importStmt + out.slice(pos);
  } else {
    out = importStmt + "\n" + out;
  }

  report.push(`${WRITE ? "WROTE" : "DRY"} ${file}: ${classified.length} buttons (${importNames.join(", ")})`);
  if (WRITE) writeFileSync(file, out);
}

report.push("");
report.push(`total buttons: ${totalConverted}, files: ${totalFiles}, mode: ${WRITE ? "WRITE" : "DRY RUN"}`);
report.push(`skipped (dynamic className): ${skipped.length}`);
skipped.forEach((s) => report.push("  " + s));
report.push(`ref sites (verify): ${refSites.length}`);
refSites.forEach((s) => report.push("  " + s));

writeFileSync("/tmp/btn-codemod-report.txt", report.join("\n"));
console.log(`done: ${totalConverted} buttons, ${totalFiles} files, mode=${WRITE ? "WRITE" : "DRY RUN"}`);
