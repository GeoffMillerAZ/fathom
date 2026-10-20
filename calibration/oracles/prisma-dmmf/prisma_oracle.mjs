// prisma_oracle.mjs - the language-breadth harness's oracle for the prisma tags dialect
// (design section 1.3, 13.4; LB-Q-prisma).
//
//   node prisma_oracle.mjs -tools <npm prefix> -subject <checkout> -out <file> [-summary-out <file>]
//
// <npm prefix> is the directory `npm --prefix` installed @prisma/prisma-schema-wasm into (nothing is installed
// globally or into the subject). Runs offline: the wasm is Prisma's own schema engine (the module `getDMMF` in
// @prisma/internals calls), so no query-engine binary is downloaded.
//
// The .prisma files of one directory are one schema (the prismaSchemaFolder layout: every file of the folder is
// handed to the engine together). For each such schema the engine's `get_dmmf` attests, by name (tier A): every
// model, view, composite type and enum, each model's, view's and type's fields, each enum's values, and for each
// field its type, kind (scalar, object, enum, unsupported) and its @relation fields: and references: lists.
// DMMF carries no source position and @mrleebo/prisma-ast neither locates members nor reads a @relation( that
// spans lines, so positions come from this file's own line scan (a block header line, a member's first line at
// bracket depth 0, its last line where the depth returns to 0). Every scanned block and member must be in the
// engine's answer and every engine member must be found by the scan: a miss is counted in `name_disagreements`.
// A schema the engine rejects (it validates providers and feature flags) falls back to the scan alone, tier C,
// and is counted in `engine_rejected`; the reading's tier is C when any schema fell back.
//
// Emitted:
//   - definitions, at the name's line, ending at the block's (or member's) last line: `model`, `view`, `type`
//     (composite), `enum`, `datasource`, `generator`; `field` per member of a model, view or type (Model.field);
//     `enum_value` per enum value (Enum.VALUE).
//   - references: (Model.field, type) for a field whose kind is object (a model, a view or a composite type) or
//     enum, and (Model.field, field name) for each name in its @relation fields: and references: lists. The
//     built-in scalars and Unsupported() are none.
//   - cross_references (file granularity): a type reference (an object or enum field: a model, view, composite
//     type or enum) whose type exactly one other file of the same schema defines, attributed to that file by the
//     scan (LB-F580: an enum field counts, as the engine's own kind says). A name two files of one schema define
//     does not validate, so none is ambiguous inside a schema.
// Directories the connector never reads (testdata, vendor, node_modules, generated, .git and the editor
// directories) are not walked. Bench-only; never a product dependency.

import fs from "node:fs";
import path from "node:path";
import { createRequire } from "node:module";

const argv = process.argv.slice(2);
function flag(name, dflt = "") {
  const i = argv.indexOf("-" + name);
  return i >= 0 && i + 1 < argv.length ? argv[i + 1] : dflt;
}
const toolsDir = path.resolve(flag("tools", process.env.PRISMA_ORACLE_HOME || "."));
const subject = flag("subject") ? path.resolve(flag("subject")) : "";
const outFile = flag("out");
const summaryOut = flag("summary-out");
if (!subject || !outFile) {
  console.error("usage: node prisma_oracle.mjs -tools <npm prefix> -subject <checkout> -out <file> [-summary-out <file>]");
  process.exit(2);
}
const req = createRequire(path.join(toolsDir, "package.json"));
const wasm = req("@prisma/prisma-schema-wasm");
const wasmVersion = req("@prisma/prisma-schema-wasm/package.json").version;

const SKIP_DIRS = new Set(["vendor", ".git", "testdata", "generated", "node_modules", ".idea", ".vscode", ".claude"]);

function findFiles(root) {
  const found = [];
  (function walk(dir) {
    const ents = fs.readdirSync(dir, { withFileTypes: true }).sort((a, b) => (a.name < b.name ? -1 : 1));
    for (const e of ents) {
      const full = path.join(dir, e.name);
      if (e.isDirectory()) {
        if (!SKIP_DIRS.has(e.name)) walk(full);
      } else if (e.isFile() && path.extname(e.name).toLowerCase() === ".prisma") {
        found.push(path.relative(root, full).split(path.sep).join("/"));
      }
    }
  })(root);
  return found;
}

const HEADER = /^(model|view|type|enum|datasource|generator)\s+([A-Za-z_][A-Za-z0-9_]*)\s*\{/;
const codeOf = (text) => text.replace(/\/\/.*$/, "").replace(/"(?:[^"\\]|\\.)*"/g, '""');
const delta = (code) => {
  let d = 0;
  for (const ch of code) {
    if (ch === "(" || ch === "[" || ch === "{") d++;
    else if (ch === ")" || ch === "]" || ch === "}") d--;
  }
  return d;
};

// scanBlocks: the top-level blocks of one file with their members. A member is a line at bracket depth 0 inside a
// block whose first token is an identifier (a field or an enum value); a line starting with @ or // is none. A
// member ends where the bracket depth returns to 0 (a field whose @relation( spans lines).
function scanBlocks(src) {
  const lines = src.split("\n");
  const blocks = [];
  let cur = null;
  let depth = 0;
  let mem = null;
  for (let i = 0; i < lines.length; i++) {
    const text = lines[i];
    const code = codeOf(text);
    const trimmed = text.trim();
    if (!cur) {
      const m = HEADER.exec(trimmed);
      if (m) {
        cur = { type: m[1], name: m[2], line: i + 1, end: i + 1, members: [], text: [] };
        depth = delta(code);
        if (depth <= 0) { cur.end = i + 1; blocks.push(cur); cur = null; }
      }
      continue;
    }
    if (depth === 1 && !mem && trimmed !== "" && !trimmed.startsWith("//") && !trimmed.startsWith("@") && !trimmed.startsWith("}")) {
      const m = /^([A-Za-z_][A-Za-z0-9_]*)/.exec(trimmed);
      if (m) { mem = { name: m[1], line: i + 1, end: i + 1, text: [] }; cur.members.push(mem); }
    }
    if (mem) { mem.text.push(code); mem.end = i + 1; }
    depth += delta(code);
    if (mem && depth <= 1) mem = null;
    if (depth <= 0) { cur.end = i + 1; blocks.push(cur); cur = null; mem = null; }
  }
  return blocks;
}

const defs = [];
const refs = [];
const seenRef = new Set();
const cross = [];
const crossSeen = new Set();
const unparsed = [];
let unsupportedFields = 0;
const agreement = { groups: 0, dmmf: 0, fallback: [], name_disagree: [] };
const files = findFiles(subject);

function addDef(file, name, kind, line, end) {
  defs.push({ path: file, name, line, end, kind });
}
function addRef(file, from, to) {
  const k = file + "\0" + from + "\0" + to;
  if (seenRef.has(k)) return;
  seenRef.add(k);
  refs.push({ path: file, from, to });
}
function addCross(file, from, toPath, to) {
  const k = file + "\0" + from + "\0" + toPath + "\0" + to;
  if (crossSeen.has(k)) return;
  crossSeen.add(k);
  cross.push({ path: file, from, to_path: toPath, to });
}

const groups = new Map();
for (const f of files) {
  const d = path.posix.dirname(f);
  if (!groups.has(d)) groups.set(d, []);
  groups.get(d).push(f);
}

for (const [dir, group] of [...groups.entries()].sort()) {
  agreement.groups++;
  const sources = group.map((f) => [f, fs.readFileSync(path.join(subject, f), "utf8")]);
  const blocks = [];
  for (const [f, src] of sources) for (const b of scanBlocks(src)) blocks.push({ file: f, ...b });
  // engine: names, fields, relations (tier A).
  let dmmf = null;
  try {
    dmmf = JSON.parse(wasm.get_dmmf(JSON.stringify({ prismaSchema: sources, noColor: true }))).datamodel;
    agreement.dmmf++;
  } catch (e) {
    agreement.fallback.push({ dir, error: String(e.message || e).replace(/\s+/g, " ").slice(0, 240) });
  }
  const eng = new Map();
  if (dmmf) {
    for (const m of dmmf.models || []) eng.set(m.name, { fields: new Map(m.fields.map((x) => [x.name, x])) });
    for (const t of dmmf.types || []) eng.set(t.name, { fields: new Map(t.fields.map((x) => [x.name, x])) });
    for (const e of dmmf.enums || []) eng.set(e.name, { values: new Set(e.values.map((v) => v.name)) });
  }
  const typeFile = new Map();
  for (const b of blocks) if (["model", "view", "type", "enum"].includes(b.type)) typeFile.set(b.name, b.file);

  for (const b of blocks) {
    addDef(b.file, b.name, b.type, b.line, b.end);
    if (b.type === "datasource" || b.type === "generator") continue;
    const e = eng.get(b.name);
    if (dmmf && !e) agreement.name_disagree.push({ path: b.file, name: b.name, why: "block not in engine" });
    const seen = new Set();
    if (b.type === "enum") {
      for (const m of b.members) {
        addDef(b.file, b.name + "." + m.name, "enum_value", m.line, m.end);
        seen.add(m.name);
      }
      if (e && e.values) for (const v of e.values) if (!seen.has(v)) agreement.name_disagree.push({ path: b.file, name: b.name + "." + v, why: "engine value not found by scan" });
      continue;
    }
    for (const m of b.members) {
      const q = b.name + "." + m.name;
      seen.add(m.name);
      const f = e && e.fields ? e.fields.get(m.name) : null;
      if (dmmf && !f) {
        // the engine's DMMF leaves out an Unsupported("...") column: a definition with no type to reference.
        if (/\bUnsupported\s*\(/.test(m.text.join(" "))) { addDef(b.file, q, "field", m.line, m.end); unsupportedFields++; }
        else agreement.name_disagree.push({ path: b.file, name: q, why: "scanned field not in engine" });
        continue;
      }
      addDef(b.file, q, "field", m.line, m.end);
      if (f) {
        if ((f.kind === "object" || f.kind === "enum") && f.type) addRef(b.file, q, f.type);
        for (const n of f.relationFromFields || []) addRef(b.file, q, n);
        for (const n of f.relationToFields || []) addRef(b.file, q, n);
        if ((f.kind === "object" || f.kind === "enum") && typeFile.has(f.type) && typeFile.get(f.type) !== b.file) addCross(b.file, q, typeFile.get(f.type), f.type);
      } else {
        // tier C fallback (the engine rejected the schema): the field's type token and its @relation lists, by text.
        const joined = m.text.join(" ");
        const tm = new RegExp("^\\s*" + m.name + "\\s+([A-Za-z_][A-Za-z0-9_]*)").exec(joined);
        if (tm && typeFile.has(tm[1])) {
          addRef(b.file, q, tm[1]);
          if (typeFile.get(tm[1]) !== b.file) addCross(b.file, q, typeFile.get(tm[1]), tm[1]);
        }
        for (const lm of joined.matchAll(/\b(?:fields|references)\s*:\s*\[([^\]]*)\]/g)) {
          for (const n of lm[1].split(",").map((x) => x.trim()).filter(Boolean)) addRef(b.file, q, n);
        }
      }
    }
    if (e && e.fields) for (const n of e.fields.keys()) if (!seen.has(n)) agreement.name_disagree.push({ path: b.file, name: b.name + "." + n, why: "engine field not found by scan" });
  }
}

const byKey = (a, b) => (a.path < b.path ? -1 : a.path > b.path ? 1 : (a.line || 0) - (b.line || 0) || (a.name || a.from || "").localeCompare(b.name || b.from || ""));
const tier = agreement.fallback.length === 0 ? "A" : "C";
const out = {
  tool: "prisma-schema-wasm",
  version: wasmVersion,
  tier,
  definitions: defs.sort(byKey),
  references: refs.sort((a, b) => (a.path < b.path ? -1 : a.path > b.path ? 1 : a.from < b.from ? -1 : a.from > b.from ? 1 : a.to < b.to ? -1 : a.to > b.to ? 1 : 0)),
  cross_references: cross,
};
fs.writeFileSync(outFile, JSON.stringify(out, null, 1) + "\n");
const summary = {
  files: files.length,
  schemas: agreement.groups,
  schemas_validated_by_engine: agreement.dmmf,
  engine_rejected: agreement.fallback,
  unparsed,
  definitions: defs.length,
  references: refs.length,
  cross_references: cross.length,
  unsupported_fields: unsupportedFields,
  name_disagreements: agreement.name_disagree,
};
if (summaryOut) fs.writeFileSync(summaryOut, JSON.stringify(summary, null, 1) + "\n");
console.error(JSON.stringify({ ...summary, unparsed: unparsed.length, name_disagreements: agreement.name_disagree.length, engine_rejected: agreement.fallback.length }));
