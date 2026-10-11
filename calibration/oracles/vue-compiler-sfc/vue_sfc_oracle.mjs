// vue_sfc_oracle.mjs — Vue single-file components' tier-A oracle (ruling W4,
// design §1.8; wave lf7, item vuesvelte, K19): @vue/compiler-sfc (MIT), the
// Vue compiler's own SFC parser, reads each .vue file of a subject and writes
// lang-oracle's neutral tool-oracle JSON:
//
//   definitions  the top-level declarations of each <script> and <script
//                setup> block as the compiler's own Babel parse reads them
//                (compileScript's scriptAst and scriptSetupAst, the trees its
//                bindingMetadata is computed from; @babel/parser with the
//                compiler's plugins when compileScript throws): a function
//                declaration "function", a class "class", a const whose
//                initializer is an arrow or function expression "function",
//                any other const declarator "constant" (each name of a
//                destructuring pattern), let and var "variable", TypeScript
//                interface, type alias and enum "interface", "type", "enum",
//                and each import binding "import". Lines are the file's: the
//                name's line, the declarator's (a pattern name's own) or the
//                declaration's last line as end.
//   calls        (path, "<module>", name) for each element of the template the
//                compiler types a component (ElementNode.tagType COMPONENT in
//                descriptor.template.ast): its tag camelized and capitalized
//                as resolveComponent resolves it (el-input -> ElInput), a
//                dotted tag as written. <component :is> (a dynamic component,
//                no name of its own) is counted and left out.
//   call_exclusions  each name a script block calls (a CallExpression,
//                OptionalCallExpression or NewExpression callee's last name)
//                that no template component of the file names, reason
//                "script-call": the script's calls are TypeScript's or
//                JavaScript's own L2, out of the component pair set (design
//                §1.8, html's injected-call precedent), so they leave both
//                sides, counted.
//   imports      each static import or re-export statement of a script block
//                whose specifier is relative (./, ../), resolved to a file of
//                the subject by Vite's rule (the bundler Vue's own tooling
//                runs): the path as written, a .js/.jsx/.mjs/.cjs spec from a
//                TypeScript block first as the .ts/.tsx/.mts/.cts it is emitted
//                from, then Vite's default resolve.extensions (.mjs .js .mts
//                .ts .jsx .tsx .json), then the directory's index with them. An
//                alias specifier (@/, ~/, #, $, /) and a package are counted,
//                never resolved.
//   disclose     the counts a reading names.
//
// Usage: node vue_sfc_oracle.mjs -tools <npm prefix holding @vue/compiler-sfc>
//          -subject <dir> [-out <file>]
// Bench-only: never a product dependency, run in the measuring harness only.
import fs from "node:fs";
import path from "node:path";
import { createRequire } from "node:module";

const MODULE = "<module>";
const ELEMENT = 1, COMPONENT = 1;

export const camelize = (s) => s.replace(/-(\w)/g, (_, c) => (c ? c.toUpperCase() : ""));
export const capitalize = (s) => s.charAt(0).toUpperCase() + s.slice(1);

// componentName is the name resolveComponent resolves a component tag by.
export function componentName(tag) {
  return tag.includes(".") ? tag : capitalize(camelize(tag));
}

// templateComponents walks a template AST: the names of its component
// elements, and the count of dynamic <component> elements left out.
export function templateComponents(ast) {
  const names = new Set();
  let dynamic = 0;
  const walk = (n) => {
    for (const c of n.children || []) {
      if (c.type === ELEMENT) {
        if (c.tagType === COMPONENT) {
          if (c.tag === "component") dynamic++;
          else names.add(componentName(c.tag));
        }
        walk(c);
      } else if (c.branches) {
        for (const b of c.branches) walk(b);
      }
    }
  };
  walk(ast);
  return { names, dynamic };
}

const FUNCTION_INIT = new Set(["ArrowFunctionExpression", "FunctionExpression"]);

function patternNames(p, out) {
  if (!p) return out;
  switch (p.type) {
    case "Identifier": out.push(p); break;
    case "ObjectPattern": for (const q of p.properties) patternNames(q.type === "RestElement" ? q.argument : q.value, out); break;
    case "ArrayPattern": for (const q of p.elements) patternNames(q && q.type === "RestElement" ? q.argument : q, out); break;
    case "AssignmentPattern": patternNames(p.left, out); break;
    case "RestElement": patternNames(p.argument, out); break;
  }
  return out;
}

// declarations is a block's top-level declarations, lines offset by base
// (the block's first line less one).
export function declarations(body, rel, base) {
  const out = [];
  const def = (name, line, end, kind) => out.push({ path: rel, name, line: line + base, end: end + base, kind });
  for (let n of body) {
    if ((n.type === "ExportNamedDeclaration" || n.type === "ExportDefaultDeclaration") && n.declaration) n = n.declaration;
    switch (n.type) {
      case "FunctionDeclaration":
        if (n.id) def(n.id.name, n.id.loc.start.line, n.loc.end.line, "function");
        break;
      case "ClassDeclaration":
        if (n.id) def(n.id.name, n.id.loc.start.line, n.loc.end.line, "class");
        break;
      case "TSInterfaceDeclaration": def(n.id.name, n.id.loc.start.line, n.loc.end.line, "interface"); break;
      case "TSTypeAliasDeclaration": def(n.id.name, n.id.loc.start.line, n.loc.end.line, "type"); break;
      case "TSEnumDeclaration": def(n.id.name, n.id.loc.start.line, n.loc.end.line, "enum"); break;
      case "VariableDeclaration":
        if (n.declare) break;
        for (const d of n.declarations) {
          if (d.id.type === "Identifier") {
            const kind = d.init && FUNCTION_INIT.has(d.init.type) ? "function" : n.kind === "const" ? "constant" : "variable";
            def(d.id.name, d.id.loc.start.line, d.loc.end.line, kind);
          } else {
            for (const id of patternNames(d.id, [])) def(id.name, id.loc.start.line, id.loc.end.line, n.kind === "const" ? "constant" : "variable");
          }
        }
        break;
      case "ImportDeclaration":
        for (const s of n.specifiers) def(s.local.name, s.local.loc.start.line, s.local.loc.end.line, "import");
        break;
    }
  }
  return out;
}

// calleeNames is every name a block calls: a call's or new's callee, its last
// name (a.b.c() calls c), through non-null and parentheses.
export function calleeNames(body) {
  const out = new Set();
  const name = (c) => {
    while (c && (c.type === "TSNonNullExpression" || c.type === "ParenthesizedExpression" || c.type === "TSAsExpression")) c = c.expression;
    if (!c) return;
    if (c.type === "Identifier") out.add(c.name);
    else if ((c.type === "MemberExpression" || c.type === "OptionalMemberExpression") && !c.computed && c.property.type === "Identifier") out.add(c.property.name);
  };
  const seen = new Set();
  const walk = (n) => {
    if (!n || typeof n !== "object" || seen.has(n)) return;
    seen.add(n);
    if (Array.isArray(n)) { for (const x of n) walk(x); return; }
    if (n.type === "CallExpression" || n.type === "OptionalCallExpression" || n.type === "NewExpression") name(n.callee);
    for (const k of Object.keys(n)) {
      if (k === "loc" || k === "start" || k === "end" || k === "leadingComments" || k === "trailingComments" || k === "extra") continue;
      const v = n[k];
      if (v && typeof v === "object") walk(v);
    }
  };
  walk(body);
  return out;
}

const VITE_EXTS = [".mjs", ".js", ".mts", ".ts", ".jsx", ".tsx", ".json"];
const TS_OUTPUT = { ".js": [".ts", ".tsx"], ".jsx": [".tsx"], ".mjs": [".mts"], ".cjs": [".cts"] };

const isFile = (p) => { try { return fs.statSync(p).isFile(); } catch { return false; } };
const isDir = (p) => { try { return fs.statSync(p).isDirectory(); } catch { return false; } };

// resolveRelative is Vite's resolution of a relative spec from a file of root
// (a path relative to root, or "" when nothing is found).
export function resolveRelative(root, importerRel, spec, tsBlock) {
  const abs = path.resolve(root, path.dirname(importerRel), spec.replace(/[?#].*$/, ""));
  const rel = (p) => path.relative(root, p).split(path.sep).join("/");
  if (!abs.startsWith(root + path.sep)) return "";
  const ext = path.extname(abs);
  if (tsBlock && TS_OUTPUT[ext]) {
    for (const e of TS_OUTPUT[ext]) if (isFile(abs.slice(0, -ext.length) + e)) return rel(abs.slice(0, -ext.length) + e);
  }
  if (isFile(abs)) return rel(abs);
  for (const e of VITE_EXTS) if (isFile(abs + e)) return rel(abs + e);
  if (isDir(abs)) for (const e of VITE_EXTS) if (isFile(path.join(abs, "index" + e))) return rel(path.join(abs, "index" + e));
  return "";
}

export function specClass(spec) {
  if (spec.startsWith("./") || spec.startsWith("../") || spec === "." || spec === "..") return "relative";
  if (/^(@\/|~\/|~~\/|@@\/|#|\$|\/)/.test(spec)) return "alias";
  return "package";
}

// importStatements is a block's static imports and re-exports with a source.
export function importStatements(body) {
  return body.filter((n) => (n.type === "ImportDeclaration" || n.type === "ExportAllDeclaration" || (n.type === "ExportNamedDeclaration" && n.source)) && n.source);
}

function walkVue(root, rel = "", out = []) {
  for (const ent of fs.readdirSync(path.join(root, rel), { withFileTypes: true }).sort((a, b) => (a.name < b.name ? -1 : a.name > b.name ? 1 : 0))) {
    if (ent.name === ".git" || ent.name === "node_modules") continue;
    const r = rel ? rel + "/" + ent.name : ent.name;
    if (ent.isDirectory()) walkVue(root, r, out);
    else if (ent.isFile() && ent.name.endsWith(".vue")) out.push(r);
  }
  return out;
}

const isTS = (lang) => lang === "ts" || lang === "tsx";

// blocks is the file's script blocks, each with its Babel body and line base:
// compileScript's ASTs, or the compiler's parser over each block when
// compileScript throws.
function blocks(sfc, babel, descriptor, rel, root, counts) {
  const parts = [descriptor.script, descriptor.scriptSetup].filter(Boolean);
  if (parts.length === 0) return [];
  let r = null;
  try {
    r = sfc.compileScript(descriptor, { id: "oracle", fs: { fileExists: isFile, readFile: (p) => fs.readFileSync(p, "utf8"), realpath: (p) => p } });
  } catch (e) {
    counts.compileScriptThrew++;
  }
  const out = [];
  for (const b of parts) {
    const setup = b === descriptor.scriptSetup;
    let body = r ? (setup ? r.scriptSetupAst : r.scriptAst) : null;
    if (!body) {
      try {
        const plugins = isTS(b.lang) ? ["typescript", ...(b.lang === "tsx" ? ["jsx"] : [])] : ["jsx"];
        body = babel.parse(b.content, { sourceType: "module", plugins }).program.body;
      } catch (e) {
        counts.blocksUnparsed++;
        continue;
      }
    }
    out.push({ body, base: b.loc.start.line - 1, ts: isTS(b.lang) });
  }
  return out;
}

export function run(sfc, babel, version, subject) {
  const out = { tool: "@vue/compiler-sfc", version, tier: "A", definitions: [], calls: [], call_exclusions: [], imports: [], disclose: [] };
  const counts = { files: 0, parseErrors: 0, compileScriptThrew: 0, blocksUnparsed: 0, nonHTMLTemplates: 0, dynamic: 0, components: 0, scriptCallees: 0, relative: 0, resolved: 0, unresolved: 0, alias: 0, pkg: 0 };
  for (const rel of walkVue(subject)) {
    const text = fs.readFileSync(path.join(subject, rel), "utf8");
    counts.files++;
    const { descriptor, errors } = sfc.parse(text, { filename: rel, sourceMap: false });
    if (errors.length) counts.parseErrors++;
    const names = new Set();
    if (descriptor.template) {
      if (descriptor.template.ast && (!descriptor.template.lang || descriptor.template.lang === "html")) {
        const t = templateComponents(descriptor.template.ast);
        counts.dynamic += t.dynamic;
        for (const n of t.names) names.add(n);
      } else counts.nonHTMLTemplates++;
    }
    for (const n of [...names].sort()) out.calls.push({ path: rel, from: MODULE, to: n });
    counts.components += names.size;
    const callees = new Set();
    for (const b of blocks(sfc, babel, descriptor, rel, subject, counts)) {
      out.definitions.push(...declarations(b.body, rel, b.base));
      for (const c of calleeNames(b.body)) callees.add(c);
      for (const s of importStatements(b.body)) {
        const spec = s.source.value;
        const cls = specClass(spec);
        if (cls === "alias") { counts.alias++; continue; }
        if (cls === "package") { counts.pkg++; continue; }
        counts.relative++;
        const to = resolveRelative(subject, rel, spec, b.ts);
        if (!to) { counts.unresolved++; continue; }
        counts.resolved++;
        out.imports.push({ path: rel, line: s.loc.start.line + b.base, to_path: to, module: spec });
      }
    }
    for (const c of [...callees].sort()) {
      if (names.has(c) || [...names].some((n) => n.endsWith("." + c))) continue;
      counts.scriptCallees++;
      out.call_exclusions.push({ path: rel, to: c, reason: "script-call" });
    }
  }
  out.disclose.push(`@vue/compiler-sfc ${version}: ${counts.files} files (${counts.parseErrors} with an SFC parse error, ${counts.nonHTMLTemplates} templates in another language and not read), ${counts.components} (file, component) pairs from the compiler's tagType COMPONENT, ${counts.dynamic} dynamic <component :is> elements left out; ${counts.scriptCallees} (file, name) script calls left out of the component pairs (call_exclusions, script-call); compileScript threw on ${counts.compileScriptThrew} files (their blocks read with the compiler's @babel/parser), ${counts.blocksUnparsed} blocks unparsed; ${counts.relative} relative import statements (${counts.resolved} resolved by Vite's rule, ${counts.unresolved} unresolved), ${counts.alias} alias and ${counts.pkg} package specifiers not resolved`);
  return out;
}

function main(argv) {
  const arg = (k) => { const i = argv.indexOf(k); return i >= 0 ? argv[i + 1] : undefined; };
  const tools = arg("-tools"), subject = arg("-subject"), outPath = arg("-out");
  if (!tools || !subject) { console.error("usage: vue_sfc_oracle.mjs -tools <npm prefix> -subject <dir> [-out <file>]"); process.exit(2); }
  const req = createRequire(path.join(path.resolve(tools), "package.json"));
  const sfc = req("@vue/compiler-sfc");
  const babel = createRequire(req.resolve("@vue/compiler-sfc"))("@babel/parser");
  const version = JSON.parse(fs.readFileSync(path.join(path.resolve(tools), "node_modules", "@vue", "compiler-sfc", "package.json"), "utf8")).version;
  const out = run(sfc, babel, version, path.resolve(subject));
  const text = JSON.stringify(out, null, 2) + "\n";
  if (outPath) fs.writeFileSync(outPath, text); else process.stdout.write(text);
}

if (import.meta.url === `file://${process.argv[1]}`) main(process.argv.slice(2));
