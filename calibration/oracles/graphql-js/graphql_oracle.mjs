// graphql_oracle.mjs - the language-breadth harness's oracle for the graphql tags dialect
// (design section 1.3, 13.4; LB-Q-graphql).
//
//   node graphql_oracle.mjs -tools <npm prefix> -subject <checkout> -out <file> [-summary-out <file>]
//
// <npm prefix> is the directory `npm --prefix` installed graphql into (nothing is installed globally
// or into the subject).
//
// Tier A: graphql-js (MIT), GraphQL's reference implementation. Every .graphql and .gql file of the
// subject is read with graphql-js `parse`, the parser `buildSchema` itself calls; a file it rejects is
// counted with its error and left out. Cross-check: each file that holds type-system definitions and
// no operation is also given to `buildSchema(src, {assumeValidSDL: true})`, and the type names that
// schema registers must be the file's non-extension type definitions (a file that names a type it
// does not define cannot be built alone and is counted, not compared).
//
// Emitted (definitions and references, tier A), lines from the tokens' own positions:
//   - definitions, at the name's line, ending at the definition's last token: `type` per object,
//     interface, union, enum, scalar and input definition; `extension` per `extend type|interface|...`
//     (named by its target); `field` per field of an object, interface or input type or of their
//     extensions, named Type.field; `enum_value` per enum value, named Enum.VALUE; `directive` per
//     directive definition, named without the `@`; `operation` per NAMED operation; `fragment` per
//     fragment. A schema definition, a schema extension and an anonymous operation are no definition.
//   - references: (enclosing definition, named type) for every type an implements list, a union's
//     member list, a field's or argument's or input field's type, a directive definition's argument
//     type, a variable's type, a type condition (fragment or inline fragment) names, and
//     (enclosing operation or fragment, fragment) for every fragment spread. The five built-in scalars
//     are no reference; a reference inside an anonymous operation or outside every definition (a
//     schema definition's root types) has no source and is not in the truth.
//   - cross_references (file granularity): a reference whose type is defined (not extended) by exactly
//     one file of the subject, a different one. A name two files define (independent schemas under one
//     repository) is ambiguous and is counted, not written.
// Directories the connector never reads (testdata, vendor, node_modules, generated, .git and the
// editor directories) are not walked. Bench-only; never a product dependency.

import fs from "node:fs";
import path from "node:path";
import { createRequire } from "node:module";

const argv = process.argv.slice(2);
function flag(name, dflt = "") {
  const i = argv.indexOf("-" + name);
  return i >= 0 && i + 1 < argv.length ? argv[i + 1] : dflt;
}
const toolsDir = path.resolve(flag("tools", process.env.GRAPHQL_ORACLE_HOME || "."));
const subject = path.resolve(flag("subject"));
const outFile = flag("out");
const summaryOut = flag("summary-out");
if (!flag("subject") || !outFile) {
  console.error("usage: node graphql_oracle.mjs -tools <npm prefix> -subject <checkout> -out <file> [-summary-out <file>]");
  process.exit(2);
}
const req = createRequire(path.join(toolsDir, "package.json"));
const gql = req("graphql");
const version = req("graphql/package.json").version;

const SKIP_DIRS = new Set(["vendor", ".git", "testdata", "generated", "node_modules", ".idea", ".vscode", ".claude"]);
const EXTS = new Set([".graphql", ".gql"]);
const BUILTIN = new Set(["String", "Int", "Float", "Boolean", "ID"]);

function findFiles(root) {
  const found = [];
  (function walk(dir) {
    const ents = fs.readdirSync(dir, { withFileTypes: true }).sort((a, b) => (a.name < b.name ? -1 : 1));
    for (const e of ents) {
      const full = path.join(dir, e.name);
      if (e.isDirectory()) {
        if (!SKIP_DIRS.has(e.name)) walk(full);
      } else if (e.isFile() && EXTS.has(path.extname(e.name).toLowerCase())) {
        found.push(path.relative(root, full).split(path.sep).join("/"));
      }
    }
  })(root);
  return found;
}

const K = gql.Kind;
const TYPE_DEFS = new Set([K.OBJECT_TYPE_DEFINITION, K.INTERFACE_TYPE_DEFINITION, K.UNION_TYPE_DEFINITION, K.ENUM_TYPE_DEFINITION, K.SCALAR_TYPE_DEFINITION, K.INPUT_OBJECT_TYPE_DEFINITION]);
const TYPE_EXTS = new Set([K.OBJECT_TYPE_EXTENSION, K.INTERFACE_TYPE_EXTENSION, K.UNION_TYPE_EXTENSION, K.ENUM_TYPE_EXTENSION, K.SCALAR_TYPE_EXTENSION, K.INPUT_OBJECT_TYPE_EXTENSION]);

const nameLine = (n) => n.name.loc.startToken.line;
const endLine = (n) => n.loc.endToken.line;
function namedType(t) {
  while (t.kind === K.NON_NULL_TYPE || t.kind === K.LIST_TYPE) t = t.type;
  return t.name.value;
}

const defs = [];
const refs = [];
const seenRef = new Set();
function addDef(file, name, kind, node) {
  defs.push({ path: file, name, line: nameLine(node), end: endLine(node), kind });
}
function addRef(file, from, to) {
  if (!from || BUILTIN.has(to)) return;
  const k = file + "\0" + from + "\0" + to;
  if (seenRef.has(k)) return;
  seenRef.add(k);
  refs.push({ path: file, from, to });
}
// selection sets: fragment spreads and inline fragments' type conditions.
function walkSelections(file, from, set) {
  if (!set) return;
  for (const s of set.selections) {
    if (s.kind === K.FRAGMENT_SPREAD) addRef(file, from, s.name.value);
    else if (s.kind === K.INLINE_FRAGMENT) {
      if (s.typeCondition) addRef(file, from, s.typeCondition.name.value);
      walkSelections(file, from, s.selectionSet);
    } else walkSelections(file, from, s.selectionSet);
  }
}

const files = findFiles(subject);
const unparsed = [];
const typeDefinedIn = new Map(); // name -> Set(files) of non-extension type definitions
let built = 0, buildAgree = 0, buildIncomplete = 0;
const buildDisagree = [];

for (const file of files) {
  const src = fs.readFileSync(path.join(subject, file), "utf8");
  let doc;
  try {
    doc = gql.parse(src);
  } catch (e) {
    unparsed.push({ path: file, error: String(e.message).split("\n")[0] });
    continue;
  }
  const typeNames = [];
  let hasOp = false, hasTypeSystem = false;
  for (const d of doc.definitions) {
    if (TYPE_DEFS.has(d.kind)) {
      hasTypeSystem = true;
      const t = d.name.value;
      typeNames.push(t);
      if (!typeDefinedIn.has(t)) typeDefinedIn.set(t, new Set());
      typeDefinedIn.get(t).add(file);
      addDef(file, t, "type", d);
      if (d.kind === K.OBJECT_TYPE_DEFINITION || d.kind === K.INTERFACE_TYPE_DEFINITION) {
        for (const i of d.interfaces || []) addRef(file, t, i.name.value);
      }
      if (d.kind === K.UNION_TYPE_DEFINITION) for (const m of d.types || []) addRef(file, t, m.name.value);
      fieldsOf(file, t, d);
      if (d.kind === K.ENUM_TYPE_DEFINITION) for (const v of d.values || []) addDef(file, t + "." + v.name.value, "enum_value", v);
    } else if (TYPE_EXTS.has(d.kind)) {
      hasTypeSystem = true;
      const t = d.name.value;
      addDef(file, t, "extension", d);
      if (d.kind === K.OBJECT_TYPE_EXTENSION || d.kind === K.INTERFACE_TYPE_EXTENSION) {
        for (const i of d.interfaces || []) addRef(file, t, i.name.value);
      }
      if (d.kind === K.UNION_TYPE_EXTENSION) for (const m of d.types || []) addRef(file, t, m.name.value);
      fieldsOf(file, t, d);
      if (d.kind === K.ENUM_TYPE_EXTENSION) for (const v of d.values || []) addDef(file, t + "." + v.name.value, "enum_value", v);
    } else if (d.kind === K.DIRECTIVE_DEFINITION) {
      hasTypeSystem = true;
      const t = d.name.value;
      addDef(file, t, "directive", d);
      for (const a of d.arguments || []) addRef(file, t, namedType(a.type));
    } else if (d.kind === K.OPERATION_DEFINITION) {
      hasOp = true;
      if (!d.name) continue;
      const t = d.name.value;
      addDef(file, t, "operation", d);
      for (const v of d.variableDefinitions || []) addRef(file, t, namedType(v.type));
      walkSelections(file, t, d.selectionSet);
    } else if (d.kind === K.FRAGMENT_DEFINITION) {
      hasOp = true;
      const t = d.name.value;
      addDef(file, t, "fragment", d);
      addRef(file, t, d.typeCondition.name.value);
      walkSelections(file, t, d.selectionSet);
    }
  }
  if (hasTypeSystem && !hasOp) {
    try {
      const schema = gql.buildSchema(src, { assumeValidSDL: true });
      built++;
      const registered = Object.keys(schema.getTypeMap()).filter((n) => !n.startsWith("__") && !BUILTIN.has(n));
      const want = [...new Set(typeNames)].sort();
      const got = registered.sort();
      if (JSON.stringify(want) === JSON.stringify(got)) buildAgree++;
      else buildDisagree.push({ path: file, want, got });
    } catch (e) {
      buildIncomplete++;
    }
  }
}

function fieldsOf(file, t, d) {
  const list = d.kind === K.INPUT_OBJECT_TYPE_DEFINITION || d.kind === K.INPUT_OBJECT_TYPE_EXTENSION ? d.fields : d.fields;
  for (const f of list || []) {
    const fq = t + "." + f.name.value;
    addDef(file, fq, "field", f);
    addRef(file, fq, namedType(f.type));
    for (const a of f.arguments || []) addRef(file, fq, namedType(a.type));
  }
}

// cross references: file granularity, a type one other file defines.
const cross = [];
let ambiguous = 0;
const crossSeen = new Set();
for (const r of refs) {
  const fs_ = typeDefinedIn.get(r.to);
  if (!fs_) continue;
  if (fs_.size > 1) { ambiguous++; continue; }
  const [to] = [...fs_];
  if (to === r.path) continue;
  const k = r.path + "\0" + r.from + "\0" + to + "\0" + r.to;
  if (crossSeen.has(k)) continue;
  crossSeen.add(k);
  cross.push({ path: r.path, from: r.from, to_path: to, to: r.to });
}

const byKey = (a, b) => (a.path < b.path ? -1 : a.path > b.path ? 1 : (a.line || 0) - (b.line || 0) || (a.name || a.from || "").localeCompare(b.name || b.from || ""));
const out = {
  tool: "graphql-js",
  version,
  tier: "A",
  definitions: defs.sort(byKey),
  references: refs.sort((a, b) => (a.path < b.path ? -1 : a.path > b.path ? 1 : a.from < b.from ? -1 : a.from > b.from ? 1 : a.to < b.to ? -1 : a.to > b.to ? 1 : 0)),
  cross_references: cross,
};
fs.writeFileSync(outFile, JSON.stringify(out, null, 1) + "\n");
const summary = {
  files: files.length,
  parsed: files.length - unparsed.length,
  unparsed,
  definitions: defs.length,
  references: refs.length,
  cross_references: cross.length,
  ambiguous_cross_references: ambiguous,
  build_schema: { built, agree: buildAgree, incomplete: buildIncomplete, disagree: buildDisagree },
};
if (summaryOut) fs.writeFileSync(summaryOut, JSON.stringify(summary, null, 1) + "\n");
console.error(JSON.stringify({ ...summary, unparsed: unparsed.length, build_schema: { built, agree: buildAgree, incomplete: buildIncomplete, disagree: buildDisagree.length } }));
