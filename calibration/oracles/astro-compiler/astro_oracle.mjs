// astro_oracle.mjs — Astro's tier-A oracle (owner ruling W4, design §1.8's Astro paragraph; wave lf7, item
// angularastro, LB-F546). @astrojs/compiler (MIT) is Astro's own parser: `parse` gives the component's
// frontmatter (its text and lines) and every template node with the type the compiler gives it (`component`,
// `fragment`, `custom-element`, `element`, `expression`). Astro compiles the frontmatter as the component's
// module script, which is TypeScript, so what a frontmatter or a JavaScript <script> declares, calls and
// imports is read with TypeScript's own parser (typescript, Apache-2.0: `createSourceFile`) over the text the
// compiler delimits, at host lines. Each file of the subject named `*.astro` gives lang-oracle's neutral
// tool-oracle JSON:
//
//   definitions  the ranges' definitions, on the TypeScript dialect's kinds as fathom names them (a hosted
//                range keeps its own kinds, F2): a function declaration (an overload signature included,
//                at any depth), a top-level const, let or var bound to an arrow or function expression
//                (function), a class (any depth), a method, constructor or accessor of a class and a method
//                signature of an interface (method), an interface, a type alias, an enum, and each name a
//                top-level `const` binds otherwise (constant: an identifier, a destructured name, an array
//                pattern's identifier). line is the node's first line, end its last.
//   calls        (path, caller, callee): a component tag is a call of the component's module symbol
//                "<module>" (the compiler's `component` node, M2: the callee is the name written, which the
//                harness reads by its last segment); a range's call, `new`, tagged template, `super(...)`
//                and `import(...)` are owned by the innermost enclosing definition above (a destructuring
//                const's initializer and anything outside every definition: the module symbol, K18 and F2),
//                a decorator's by what it decorates; the callee is the last name of the callee expression
//                (a subscript or a called result names none and is not written, as the dialect stores none).
//   imports      every import declaration, `import x = require(...)` and `export ... from` a range writes,
//                with a relative specifier, resolved as Vite resolves a relative path (the file named, else
//                the spec plus .ts, .tsx, .js, .jsx, .mjs, .mts or .json, else its index file); to_path is
//                subject-relative, "" when no file of the subject is named.
//   disclose     the counts the reading names (design §1.8): files, components and fragments, template
//                expressions (not injected: out of both sides), styles (CSS: their pairs out of fathom's side),
//                scripts read and left (a non-JavaScript type), imports by kind (relative, alias, package),
//                and files the compiler or TypeScript reported a syntax error in (still read).
//
// Usage: node astro_oracle.mjs -tools <npm prefix holding @astrojs/compiler and typescript>
//          -subject <dir> [-under <subdir>] [-out <file>]
// -under reads only the files below one subdirectory (a subject's subpath), paths still subject-relative.
// Bench-only: never a product dependency, run in the measuring harness only.
import fs from "node:fs";
import path from "node:path";
import { createRequire } from "node:module";

export const MODULE = "<module>";

// JS_TYPES are the <script> types the dialect reads as JavaScript (dialects/astro/injections.scm).
export const JS_TYPES = new Set(["module", "text/javascript", "application/javascript"]);

// VITE_EXTS is the extension search order for a relative spec with no file of its own name.
export const VITE_EXTS = [".ts", ".tsx", ".js", ".jsx", ".mjs", ".mts", ".json"];

// isComponentName is the compiler's own rule, read from its `component` nodes and stated for the tests: a tag
// whose name starts A to Z or holds a ".", Fragment excepted (the compiler's `fragment`).
export function isComponentName(name) {
  return name !== "Fragment" && (/^[A-Z]/.test(name) || name.includes("."));
}

// scriptIsJS reports whether a <script>'s attributes leave it JavaScript: no `type`, or a JavaScript type.
export function scriptIsJS(attrs) {
  const t = (attrs || []).find((a) => a.name === "type");
  if (!t) return true;
  return t.kind === "quoted" || t.kind === "empty" ? JS_TYPES.has(String(t.value).trim()) : false;
}

// resolveRelative resolves a relative spec from importer (subject-relative) against the subject's files.
export function resolveRelative(subject, importer, spec, exists) {
  const has = exists || ((p) => { try { return fs.statSync(path.join(subject, p)).isFile(); } catch { return false; } });
  const q = spec.search(/[?#]/);
  if (q >= 0) spec = spec.slice(0, q);
  let cand = path.posix.normalize(path.posix.join(path.posix.dirname(importer), spec));
  if (cand.startsWith("../") || cand === "..") return "";
  if (cand.endsWith("/")) cand = cand.slice(0, -1);
  if (has(cand)) return cand;
  for (const e of VITE_EXTS) if (has(cand + e)) return cand + e;
  for (const e of VITE_EXTS) if (has(cand + "/index" + e)) return cand + "/index" + e;
  return "";
}

// specKind classifies a module spec: relative, alias (a path a tsconfig `paths` key would map: `~/`, `@/`,
// or any spec naming a file by a source extension) or package.
export function specKind(spec) {
  if (spec.startsWith("./") || spec.startsWith("../") || spec === "." || spec === "..") return "relative";
  if (/^[~#]\//.test(spec) || spec.startsWith("@/") || /\.(astro|ts|tsx|js|jsx|mjs|mts|md|mdx|json|css|scss)$/.test(spec)) return "alias";
  return "package";
}

// rangeFacts reads one TypeScript range (text at host line `line0`, the range's first line) with TypeScript's
// parser: its definitions, call pairs and import statements.
export function rangeFacts(ts, rel, text, line0) {
  const sf = ts.createSourceFile(rel + ".ts", text, ts.ScriptTarget.Latest, true, ts.ScriptKind.TS);
  const lineOf = (pos) => sf.getLineAndCharacterOfPosition(pos).line + line0;
  const defs = [];
  const calls = [];
  const imports = [];
  const owner = new Map(); // node -> definition name, for the call walk
  const def = (node, name, kind, startNode) => {
    const start = (startNode || node).getStart(sf);
    defs.push({ path: rel, name, line: lineOf(start), end: lineOf(node.getEnd()), kind });
    owner.set(node, name);
  };
  const isFn = (e) => e && (ts.isArrowFunction(e) || ts.isFunctionExpression(e));
  const top = sf.statements;
  for (const st of top) {
    if (!ts.isVariableStatement(st)) continue;
    const isConst = (st.declarationList.flags & ts.NodeFlags.Const) !== 0;
    for (const d of st.declarationList.declarations) {
      if (ts.isIdentifier(d.name)) {
        if (isFn(d.initializer)) def(d, d.name.text, "function");
        else if (isConst && isComponentAlias(ts, d)) def(d, d.name.text, "function");
        else if (isConst) def(d, d.name.text, "constant");
      } else if (isConst) {
        // Every name a destructuring const binds is one TypeScript declares, a default value or a rest
        // element included (the truth; the dialect's own patterns are measured against it).
        for (const id of boundNames(ts, d.name)) defs.push({ path: rel, name: id.text, line: lineOf(id.getStart(sf)), end: lineOf(id.getEnd()), kind: "constant" });
      }
    }
  }
  const visitDefs = (node) => {
    if (ts.isFunctionDeclaration(node) && node.name) def(node, node.name.text, "function", firstToken(ts, sf, node));
    else if (ts.isClassDeclaration(node) && node.name) def(node, node.name.text, "class", firstToken(ts, sf, node));
    else if ((ts.isMethodDeclaration(node) || ts.isGetAccessorDeclaration(node) || ts.isSetAccessorDeclaration(node)) && ts.isClassLike(node.parent) && memberName(ts, node)) def(node, memberName(ts, node), "method", firstToken(ts, sf, node));
    else if (ts.isConstructorDeclaration(node)) def(node, "constructor", "method", firstToken(ts, sf, node));
    else if (ts.isMethodSignature(node) && memberName(ts, node)) def(node, memberName(ts, node), "method");
    else if (ts.isInterfaceDeclaration(node)) def(node, node.name.text, "interface", firstToken(ts, sf, node));
    else if (ts.isTypeAliasDeclaration(node)) def(node, node.name.text, "type_alias", firstToken(ts, sf, node));
    else if (ts.isEnumDeclaration(node)) def(node, node.name.text, "enum", firstToken(ts, sf, node));
    ts.forEachChild(node, visitDefs);
  };
  visitDefs(sf);
  const visitCalls = (node) => {
    let callee = null;
    if (ts.isCallExpression(node)) callee = node.expression;
    else if (ts.isNewExpression(node)) callee = node.expression;
    else if (ts.isTaggedTemplateExpression(node)) callee = node.tag;
    if (callee) {
      const to = calleeName(ts, callee);
      if (to) calls.push({ path: rel, from: ownerOf(owner, node), to });
    }
    if (ts.isImportDeclaration(node) || (ts.isExportDeclaration(node) && node.moduleSpecifier)) {
      const m = node.moduleSpecifier;
      if (m && ts.isStringLiteral(m)) imports.push({ line: lineOf(node.getStart(sf)), module: m.text });
    } else if (ts.isImportEqualsDeclaration(node) && ts.isExternalModuleReference(node.moduleReference) && ts.isStringLiteral(node.moduleReference.expression)) {
      imports.push({ line: lineOf(node.getStart(sf)), module: node.moduleReference.expression.text });
    }
    ts.forEachChild(node, visitCalls);
  };
  visitCalls(sf);
  const syntaxErrors = sf.parseDiagnostics ? sf.parseDiagnostics.length : 0;
  return { defs, calls, imports, syntaxErrors };
}

// boundNames is every identifier a binding pattern binds, at any depth.
function boundNames(ts, name) {
  if (ts.isIdentifier(name)) return [name];
  const out = [];
  for (const el of name.elements || []) {
    if (ts.isBindingElement(el)) out.push(...boundNames(ts, el.name));
  }
  return out;
}

// firstToken is where the dialect's definition node starts: past a declaration's decorators and its `export`
// or `default` keyword (tree-sitter's declaration node is the one an export_statement wraps), at its own
// first keyword or modifier.
function firstToken(ts, sf, node) {
  const mods = ts.canHaveModifiers(node) ? (ts.getModifiers(node) || []) : [];
  const rest = mods.filter((m) => m.kind !== ts.SyntaxKind.ExportKeyword && m.kind !== ts.SyntaxKind.DefaultKeyword);
  if (rest.length > 0) return rest[0];
  for (const c of node.getChildren(sf)) {
    if (c.kind === ts.SyntaxKind.SyntaxList) continue; // modifiers and decorators
    if (ts.isDecorator(c)) continue;
    return c;
  }
  return node;
}

// memberName is a member's written name (an identifier, a private name, a string or a computed name's text).
function memberName(ts, node) {
  const n = node.name;
  if (!n) return "";
  if (ts.isIdentifier(n) || ts.isPrivateIdentifier(n) || ts.isStringLiteral(n) || ts.isNumericLiteral(n)) return n.text;
  if (ts.isComputedPropertyName(n)) return n.getText();
  return "";
}

// isComponentAlias is the TypeScript dialect's component binding (LB-F377): a PascalCase const bound to
// memo, forwardRef, lazy or a styled template, or to a capitalized member of a capitalized name.
function isComponentAlias(ts, d) {
  const name = d.name.text, e = d.initializer;
  if (!/^[A-Z].*[a-z]/.test(name) || !e) return false;
  if (ts.isCallExpression(e)) return /^((React\.)?(memo|forwardRef|lazy)$|styled\b)/.test(e.expression.getText());
  if (ts.isPropertyAccessExpression(e)) return /^[A-Z][A-Za-z0-9_$]*\.[A-Z][A-Za-z0-9_$]*[a-z][A-Za-z0-9_$]*$/.test(e.getText());
  return false;
}

// calleeName is the last name of a callee expression, "" when it ends in none (a subscript, a call's result).
function calleeName(ts, e) {
  while (ts.isParenthesizedExpression(e) || ts.isNonNullExpression(e) || ts.isAsExpression(e) || ts.isTypeAssertionExpression(e) || (ts.isExpressionWithTypeArguments && ts.isExpressionWithTypeArguments(e))) e = e.expression;
  if (ts.isIdentifier(e) || ts.isPrivateIdentifier(e)) return e.text;
  if (ts.isPropertyAccessExpression(e)) return e.name.text;
  if (e.kind === ts.SyntaxKind.SuperKeyword) return "super";
  if (e.kind === ts.SyntaxKind.ImportKeyword) return "import";
  if (e.kind === ts.SyntaxKind.ThisKeyword) return "this";
  return "";
}

// ownerOf is the name of the innermost enclosing definition of node, else the module symbol.
function ownerOf(owner, node) {
  for (let p = node.parent; p; p = p.parent) {
    if (owner.has(p)) return owner.get(p);
  }
  return MODULE;
}

// readFile is one .astro file's facts: the compiler's AST walked for its ranges and component tags.
export async function readFile(compiler, ts, subject, rel, text) {
  const r = await compiler.parse(text, { position: true });
  const out = { defs: [], calls: [], imports: [], counts: {} };
  const count = (k, n = 1) => { out.counts[k] = (out.counts[k] || 0) + n; };
  if ((r.diagnostics || []).some((d) => d.severity === 1)) count("compiler_errors");
  const ranges = [];
  const walk = (n) => {
    switch (n.type) {
      case "frontmatter":
        ranges.push({ text: n.value, line0: n.position.start.line, kind: "frontmatter" });
        return;
      case "component":
        count("components");
        out.calls.push({ path: rel, from: MODULE, to: n.name });
        break;
      case "fragment":
        count("fragments");
        break;
      case "expression":
        count("expressions");
        break;
      case "element":
        if (n.name === "script") {
          const t = (n.children || []).find((c) => c.type === "text");
          if (scriptIsJS(n.attributes)) {
            count("scripts_read");
            if (t && t.value) ranges.push({ text: t.value, line0: t.position.start.line, kind: "script" });
          } else count("scripts_left");
          return;
        }
        if (n.name === "style") { count("styles"); return; }
        break;
    }
    for (const c of n.children || []) walk(c);
  };
  walk(r.ast);
  for (const rg of ranges) {
    const f = rangeFacts(ts, rel, rg.text, rg.line0);
    if (f.syntaxErrors > 0) count("typescript_errors");
    out.defs.push(...f.defs);
    out.calls.push(...f.calls);
    for (const im of f.imports) {
      const k = specKind(im.module);
      count("imports_" + k);
      if (k !== "relative") continue;
      out.imports.push({ path: rel, line: im.line, to_path: resolveRelative(subject, rel, im.module), module: im.module });
    }
  }
  return out;
}

function walkAstro(root, rel = "", out = []) {
  for (const ent of fs.readdirSync(path.join(root, rel), { withFileTypes: true }).sort((a, b) => (a.name < b.name ? -1 : a.name > b.name ? 1 : 0))) {
    if (ent.name === ".git" || ent.name === "node_modules") continue;
    const r = rel ? rel + "/" + ent.name : ent.name;
    if (ent.isDirectory()) walkAstro(root, r, out);
    else if (ent.isFile() && ent.name.endsWith(".astro")) out.push(r);
  }
  return out;
}

export async function run(compiler, ts, versions, subject, under = "") {
  const out = { tool: "@astrojs/compiler parse + typescript createSourceFile", version: versions, tier: "A", definitions: [], calls: [], imports: [], disclose: [] };
  const counts = {};
  let files = 0;
  for (const rel of walkAstro(subject, under.replace(/\/+$/, ""))) {
    const text = fs.readFileSync(path.join(subject, rel), "utf8");
    const r = await readFile(compiler, ts, subject, rel, text);
    files++;
    out.definitions.push(...r.defs);
    const seen = new Set();
    for (const c of r.calls) {
      const k = c.from + "\u0000" + c.to;
      if (seen.has(k)) continue;
      seen.add(k);
      out.calls.push(c);
    }
    out.imports.push(...r.imports);
    for (const [k, v] of Object.entries(r.counts)) counts[k] = (counts[k] || 0) + v;
  }
  const c = (k) => counts[k] || 0;
  out.disclose.push("units (design §1.8's Astro paragraph, K19): L1 is the frontmatters' and JavaScript scripts' definitions on the TypeScript dialect's kinds, scored under calibration/oracles/astro-compiler/astro.scoring.json (a field and a prop are outside L1, as in typescript.json; the component is its file's module symbol, outside L1); L2 is (file, caller, callee), one pair per distinct triple, graded under §1.2's call thresholds: a component tag (the compiler's component node) is the module symbol <module>'s call, a range's call its innermost enclosing definition's, else the module symbol's; L3 is the relative imports between the measured .astro files, import_resolution and the file-edge truth (LB-F57)");
  out.disclose.push(`@astrojs/compiler and typescript (${versions}): ${files} .astro files, ${out.definitions.length} definitions in their frontmatters and JavaScript scripts, ${out.calls.length} distinct (file, caller, callee) pairs; ${c("components")} component tags (the compiler's component nodes), ${c("fragments")} fragments (no call)`);
  out.disclose.push(`astro_template_expressions: ${c("expressions")} template expressions, whose JavaScript is not injected and is out of both sides (design §1.8)`);
  out.disclose.push(`astro_styles: ${c("styles")} <style> elements, CSS whose pairs are CSS's L2 and leave fathom's side (astro_injected_css_pairs)`);
  out.disclose.push(`astro_scripts: ${c("scripts_read")} JavaScript <script> elements read, ${c("scripts_left")} left (a type naming no JavaScript: data, an import map, a third-party runner)`);
  out.disclose.push(`astro_alias_imports: ${c("imports_relative")} relative imports (graded), ${c("imports_alias")} alias imports (a tsconfig paths name: unjudged, out of both sides), ${c("imports_package")} package imports (no file of the subject)`);
  out.disclose.push(`syntax: ${c("compiler_errors")} files the compiler reported an error in, ${c("typescript_errors")} ranges TypeScript's parser reported an error in (each still read)`);
  return out;
}

async function main(argv) {
  const arg = (k) => { const i = argv.indexOf(k); return i >= 0 ? argv[i + 1] : undefined; };
  const tools = arg("-tools"), subject = arg("-subject"), outPath = arg("-out"), under = arg("-under") || "";
  if (!tools || !subject) { console.error("usage: astro_oracle.mjs -tools <npm prefix> -subject <dir> [-under <subdir>] [-out <file>]"); process.exit(2); }
  const prefix = path.resolve(tools);
  const req = createRequire(path.join(prefix, "package.json"));
  const ts = req("typescript");
  const compiler = await import(path.join(prefix, "node_modules", "@astrojs", "compiler", "dist", "node", "index.js"));
  const ver = (p) => JSON.parse(fs.readFileSync(path.join(prefix, "node_modules", p, "package.json"), "utf8")).version;
  const versions = `@astrojs/compiler ${ver("@astrojs/compiler")}, typescript ${ver("typescript")}`;
  const out = await run(compiler, ts, versions, path.resolve(subject), under);
  const text = JSON.stringify(out, null, 2) + "\n";
  if (outPath) fs.writeFileSync(outPath, text); else process.stdout.write(text);
}

if (import.meta.url === `file://${process.argv[1]}`) await main(process.argv.slice(2));
