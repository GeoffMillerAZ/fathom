// json_schema_oracle.mjs - the language-breadth harness's oracle for the json-schema
// doc-engine dialect (design section 1.3, 13.4; LB-Q-json-schema).
//
//   node json_schema_oracle.mjs -tools <npm prefix> -subject <checkout> -out <file> \
//        [-xref-out <file>] [-files-out <file>] [-summary-out <file>]
//
// <npm prefix> is the directory `npm --prefix` installed @apidevtools/json-schema-ref-parser
// and yaml into (nothing is installed globally or into the subject).
//
// Tier A: @apidevtools/json-schema-ref-parser (MIT), the reference $ref resolver that
// swagger-parser, openapi tooling and many validators bundle. For each .json of the subject:
//   - `$RefParser.parse(file)` reads the document (its own JSON reader); a file it cannot
//     read is counted with its error and left out. A document is a JSON Schema when its root
//     is a mapping and its `$schema` is a string naming json-schema.org or its base name ends
//     in `.schema.json` (the claim set, -files-out).
//   - `$RefParser.resolve(file)` resolves every `$ref` the document carries, local pointers
//     and files; `$refs.exists(ref)` is the tool's word that a pointer names something and
//     `$refs.paths()` the files it read. A reference the tool cannot resolve (a missing file,
//     a pointer to nothing, a URL, which http:false leaves unfetched) is in no truth.
//
// Emitted (definitions and references, tier A):
//   - definitions: a `package` per schema (named by the file, at its root mapping's line); a
//     `type_alias` per entry of the root `$defs` and `definitions`, whatever it holds (a
//     boolean schema is one); a `field` per entry of the root `properties`. A definition sits
//     at its key's line.
//   - references: from each entity above, every `$ref` under it whose value is a local JSON
//     Pointer the tool resolves into `#/$defs/<n>`, `#/definitions/<n>` or `#/properties/<n>`
//     naming an entry the document declares, the first segment after the section unescaped
//     (~1, ~0, percent-decoding). A `$ref` outside every entity (a root allOf, a root `$ref`)
//     has no source and is not in the truth (counted in the summary).
// Tier A (-xref-out, written only when there is one): file granularity, {path, from, to_path},
// `from` the document's root entity (its file name): every `$ref` anywhere in a document whose
// value has a file part (the text before `#`, a relative path from the document's directory, a
// leading / from the subject's root) that the tool read (`$refs.paths()`) and that is a file
// of the subject. The target need not itself be a schema: the harness scores only edges
// between files the dialect owns and counts the rest.
// Directories the connector never reads (testdata, vendor, node_modules, generated, .git and
// the editor directories) are not walked. Bench-only; never a product dependency.

import fs from "node:fs";
import path from "node:path";
import { createRequire } from "node:module";

const argv = process.argv.slice(2);
function flag(name, dflt = "") {
  const i = argv.indexOf("-" + name);
  return i >= 0 && i + 1 < argv.length ? argv[i + 1] : dflt;
}
const toolsDir = path.resolve(flag("tools", process.env.JSON_SCHEMA_ORACLE_HOME || "."));
const outFile = flag("out");
const xrefOut = flag("xref-out");
const filesOut = flag("files-out");
const summaryOut = flag("summary-out");
if (!flag("subject") || !outFile) {
  console.error("usage: node json_schema_oracle.mjs -tools <npm prefix> -subject <checkout> -out <file> [-xref-out f] [-files-out f] [-summary-out f]");
  process.exit(2);
}
const subject = path.resolve(flag("subject"));
const req = createRequire(path.join(toolsDir, "package.json"));
const refParserMod = req("@apidevtools/json-schema-ref-parser");
const $RefParser = refParserMod.$RefParser || refParserMod.default || refParserMod;
const YAML = req("yaml");
const version = req("@apidevtools/json-schema-ref-parser/package.json").version;

const SKIP_DIRS = new Set(["vendor", ".git", "testdata", "generated", "node_modules", ".idea", ".vscode", ".claude"]);
const URL_RE = /^[A-Za-z][A-Za-z0-9+.-]*:\/\//;
const META_RE = /^https?:\/\/json-schema\.org\//;

function findFiles(root) {
  const found = [];
  (function walk(dir) {
    const ents = fs.readdirSync(dir, { withFileTypes: true }).sort((a, b) => (a.name < b.name ? -1 : 1));
    for (const e of ents) {
      const full = path.join(dir, e.name);
      if (e.isDirectory()) {
        if (!SKIP_DIRS.has(e.name)) walk(full);
      } else if (e.isFile() && path.extname(e.name) === ".json") {
        found.push(path.relative(root, full).split(path.sep).join("/"));
      }
    }
  })(root);
  return found;
}

const isObj = (v) => v !== null && typeof v === "object" && !Array.isArray(v);

// The key lines: an independent YAML 1.2 reader (the `yaml` package; JSON is YAML) with source
// offsets, so a line is not the dialect's decode's word.
function lineIndex(src) {
  const lc = new YAML.LineCounter();
  const doc = YAML.parseDocument(src, { lineCounter: lc, uniqueKeys: false });
  return { doc, lc };
}
function keyOf(p) { return String(p.key && (p.key.value !== undefined ? p.key.value : p.key.source)); }
function pairLine(idx, node, key) {
  if (!YAML.isMap(node)) return 0;
  for (const p of node.items) if (keyOf(p) === key) return idx.lc.linePos(p.key.range[0]).line;
  return 0;
}
function childNode(node, key) {
  if (!YAML.isMap(node)) return null;
  for (const p of node.items) if (keyOf(p) === key) return p.value;
  return null;
}

// entity sections: [key of the root, kind, group]
const SECTIONS = [
  ["$defs", "type_alias", "def"],
  ["definitions", "type_alias", "def"],
  ["properties", "field", "property"],
];
const POINTERS = { "$defs": "def", "definitions": "def", "properties": "property" };

function unescapePointer(seg) {
  let s = seg;
  try { s = decodeURIComponent(s); } catch { /* keep as written */ }
  return s.replaceAll("~1", "/").replaceAll("~0", "~");
}

function* refsUnder(node) {
  if (Array.isArray(node)) { for (const it of node) yield* refsUnder(it); }
  else if (isObj(node)) {
    for (const [k, v] of Object.entries(node)) {
      if (k === "$ref" && typeof v === "string") yield v;
      yield* refsUnder(v);
    }
  }
}

// One reference the tool cannot read (a URL, a missing file) fails the whole resolve, which
// would leave every other reference of the document out of the truth. A stub reader answers an
// unreadable file or a URL with an empty schema, so the tool still resolves the rest and
// $refs.exists() judges each local pointer against the document alone; a file reference is
// judged below by the file's existence in the subject and by $refs.paths().
const STUB_RESOLVE = {
  external: true,
  http: { order: 1, canRead: /^https?:/i, read: () => "{}" },
  file: { order: 2, canRead: true, read: (file) => { try { return fs.readFileSync(file.url.replace(/^file:\/\//, "")); } catch { return "{}"; } } },
};
const resolveErrors = {};
const unresolvedRefs = {};
const stats = { files: 0, schemas: 0, not_schemas: 0, rejected: 0, definitions: 0, refs_internal: 0, refs_internal_in_truth: 0,
  refs_internal_unresolved: 0, refs_not_in_section: 0, refs_outside_entities: 0, refs_file: 0, refs_file_in_subject: 0, refs_file_unresolved: 0,
  refs_url: 0, resolve_errors: 0 };
const rejected = {};
const defs = [], refs = [], xrefs = [], docs = [];

async function main() {
  for (const rel of findFiles(subject)) {
    stats.files++;
    const full = path.join(subject, rel);
    let api;
    try {
      api = await $RefParser.parse(full, { resolve: { external: false, http: false } });
    } catch (e) {
      const why = String(e.message || e).split("\n")[0].replaceAll(subject + path.sep, "").slice(0, 120);
      (rejected[why] ||= []).push(rel);
      stats.rejected++;
      continue;
    }
    const byName = rel.endsWith(".schema.json");
    if (!isObj(api) || !(byName || (typeof api.$schema === "string" && META_RE.test(api.$schema)))) { stats.not_schemas++; continue; }
    stats.schemas++;
    docs.push(rel);
    const src = fs.readFileSync(full, "utf8");
    const idx = lineIndex(src);
    const root = idx.doc.contents;
    const rootLine = root && root.range ? idx.lc.linePos(root.range[0]).line : 1;
    const before = defs.length;
    defs.push({ path: rel, name: path.posix.basename(rel), line: rootLine, kind: "package" });

    const entities = []; // {name, group, value}
    const declared = { def: new Set(), property: new Set() };
    for (const [key, kind, group] of SECTIONS) {
      const sec = api[key];
      if (!isObj(sec)) continue;
      const secNode = childNode(root, key);
      for (const [k, v] of Object.entries(sec)) {
        defs.push({ path: rel, name: String(k), line: pairLine(idx, secNode, k), kind });
        entities.push({ name: String(k), value: v, group });
        declared[group].add(String(k));
      }
    }
    stats.definitions += defs.length - before;

    // the tool's word on every reference
    let $refs = null;
    try {
      $refs = await $RefParser.resolve(full, { resolve: STUB_RESOLVE });
    } catch (e) {
      stats.resolve_errors++;
      (resolveErrors[String(e.message || e).split("\n")[0].slice(0, 120)] ||= []).push(rel);
      $refs = e && e.files && e.files.$refs ? e.files.$refs : null;
    }
    const toolReads = $refs ? new Set($refs.paths().filter((p) => !URL_RE.test(p))) : new Set();
    const exists = (ref) => {
      if (!$refs) return false;
      try { return $refs.exists(full + ref); } catch { return false; }
    };

    const seen = new Set();
    for (const ent of entities) {
      for (const ref of refsUnder(ent.value)) {
        if (!ref.startsWith("#/")) continue;
        stats.refs_internal++;
        if (!exists(ref)) { stats.refs_internal_unresolved++; (unresolvedRefs[rel] ||= []).push(ref); continue; }
        const segs = ref.slice(2).split("/");
        const group = POINTERS[unescapePointer(segs[0])];
        const name = segs.length > 1 ? unescapePointer(segs[1]) : null;
        if (!group || name === null || !declared[group].has(name)) { stats.refs_not_in_section++; continue; }
        stats.refs_internal_in_truth++;
        const key = ent.group + "\u0000" + ent.name + "\u0000" + group + "\u0000" + name;
        if (!seen.has(key)) { seen.add(key); refs.push({ path: rel, from: ent.name, to: name }); }
      }
    }
    const entityRefs = entities.reduce((n, e) => n + [...refsUnder(e.value)].length, 0);
    stats.refs_outside_entities += [...refsUnder(api)].length - entityRefs;

    // file references, anywhere in the document
    const fileRefs = new Set();
    for (const ref of refsUnder(api)) {
      const file = ref.split("#")[0];
      if (!file) continue;
      if (URL_RE.test(file)) { stats.refs_url++; continue; }
      stats.refs_file++;
      const abs = file.startsWith("/") ? path.join(subject, file) : path.resolve(path.dirname(full), file);
      const target = path.relative(subject, abs).split(path.sep).join("/");
      if (target.startsWith("..") || !fs.existsSync(abs) || !toolReads.has(abs)) { stats.refs_file_unresolved++; continue; }
      stats.refs_file_in_subject++;
      fileRefs.add(target);
    }
    for (const target of [...fileRefs].sort()) if (target !== rel) xrefs.push({ path: rel, from: path.posix.basename(rel), to_path: target });
  }

  const sortDefs = defs.sort((a, b) => (a.path < b.path ? -1 : a.path > b.path ? 1 : a.line - b.line || (a.name < b.name ? -1 : 1)));
  fs.writeFileSync(outFile, JSON.stringify({ tool: "@apidevtools/json-schema-ref-parser", version, tier: "A", definitions: sortDefs, references: refs }, null, 1) + "\n");
  if (xrefOut && xrefs.length) {
    fs.writeFileSync(xrefOut, JSON.stringify({ tool: "@apidevtools/json-schema-ref-parser ($refs.paths)", version, tier: "A", cross_references: xrefs }, null, 1) + "\n");
  }
  if (filesOut) fs.writeFileSync(filesOut, docs.sort().map((d) => d + "\n").join(""));
  stats.cross_references = xrefs.length;
  if (summaryOut) fs.writeFileSync(summaryOut, JSON.stringify({ stats, rejected, resolveErrors, unresolvedRefs }, null, 1) + "\n");
  console.error("json-schema-oracle: " + Object.entries(stats).map(([k, v]) => `${k} ${v}`).join(", "));
  for (const why of Object.keys(rejected).sort()) console.error(`  rejected (${why}): ${rejected[why].length} file(s), e.g. ${rejected[why][0]}`);
}
main().catch((e) => { console.error(e); process.exit(1); });
