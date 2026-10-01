// openapi_oracle.mjs - the language-breadth harness's oracle for the openapi
// doc-engine dialect (design section 1.3, 13.4; LB-Q-openapi, LB-F91).
//
//   node openapi_oracle.mjs -tools <npm prefix> -subject <checkout> -out <file> \
//        [-xref-out <file>] [-files-out <file>] [-summary-out <file>] [-redocly]
//
// <npm prefix> is the directory `npm --prefix` installed @apidevtools/swagger-parser,
// @redocly/cli and yaml into (nothing is installed globally or into the subject).
//
// Tier A: @apidevtools/swagger-parser (MIT), the format's reference parser. For each
// .yaml, .yml and .json of the subject:
//   - `SwaggerParser.parse(file)` reads the document (its own YAML and JSON readers);
//     a file it cannot read is counted with its error and left out, and a file whose
//     root is not a mapping with a top-level `openapi` or `swagger` key is not a
//     document (counted, left out). That is the claim set (-files-out).
//   - `SwaggerParser.resolve(file)` resolves every `$ref` the document carries, local
//     pointers and files; `$refs.exists(ref)` is the tool's word that a pointer names
//     something, and `$refs.paths()` the files it read. A reference the tool cannot
//     resolve (a missing file, a pointer to nothing, a URL, which http:false leaves
//     unfetched) is in no truth.
//
// Emitted (definitions and references, tier A):
//   - definitions: a `package` per document with an `info` (named by the file, at its root
//     mapping's line); a `route` per operation (a method key of a path item under `paths` or
//     `webhooks` whose value is a mapping, named "METHOD /path"); a `type_alias` per
//     entry of `components.schemas` and `definitions`; a `variable_decl` per entry of
//     `components.parameters` and the root `parameters` that is a mapping; a `message`
//     per such entry of `components.responses`, the root `responses` and
//     `components.requestBodies`; a `key` per `components.headers` entry and a `policy`
//     per `components.securitySchemes` and `securityDefinitions` entry. A definition
//     sits at its key's line (the root at its mapping's first line).
//   - references: from each operation and each component entry above (never the
//     document), every `$ref` under it whose value is a local JSON Pointer the tool
//     resolves into a section the document declares an entry of: `#/components/<s>/<n>`
//     and 2.0's `#/definitions|parameters|responses|securityDefinitions/<n>`, the first
//     segment after the section unescaped (~1, ~0, percent-decoding). A `$ref` outside
//     every entity (a path item's shared `parameters`) has no source and is not in the
//     truth.
// Tier C (-xref-out, written only when there is one): file granularity,
// {path, from, to_path}, `from` the document's root entity (its file name): every `$ref` anywhere in a document whose value has a file part
// (the text before `#`, a relative path from the document's directory, a leading / from
// the subject's root) that the tool read (`$refs.paths()`) and that is a file of the
// subject. The target need not itself be a document: the harness scores only edges
// between files the dialect owns and counts the rest.
//
// Cross-check (-redocly): `redocly bundle` (@redocly/cli, MIT) is run on each document
// that has a file `$ref`; the references it cannot resolve are counted against the
// tool's own (swagger-parser's `$refs.exists`), and any disagreement is on stderr.
// Directories the connector never reads (testdata, vendor, node_modules, generated,
// .git and the editor directories) are not walked. Bench-only; never a product
// dependency.

import fs from "node:fs";
import path from "node:path";
import { createRequire } from "node:module";
import { spawnSync } from "node:child_process";

const argv = process.argv.slice(2);
function flag(name, dflt = "") {
  const i = argv.indexOf("-" + name);
  return i >= 0 && i + 1 < argv.length ? argv[i + 1] : dflt;
}
const toolsDir = path.resolve(flag("tools", process.env.OPENAPI_ORACLE_HOME || "."));
const subject = path.resolve(flag("subject"));
const outFile = flag("out");
const xrefOut = flag("xref-out");
const filesOut = flag("files-out");
const summaryOut = flag("summary-out");
const doRedocly = argv.includes("-redocly");
if (!flag("subject") || !outFile) {
  console.error("usage: node openapi_oracle.mjs -tools <npm prefix> -subject <checkout> -out <file> [-xref-out f] [-files-out f] [-summary-out f] [-redocly]");
  process.exit(2);
}
const req = createRequire(path.join(toolsDir, "package.json"));
const SwaggerParser = req("@apidevtools/swagger-parser");
const YAML = req("yaml");
const version = req("@apidevtools/swagger-parser/package.json").version;

const SKIP_DIRS = new Set(["vendor", ".git", "testdata", "generated", "node_modules", ".idea", ".vscode", ".claude"]);
const EXTS = new Set([".yaml", ".yml", ".json"]);
const METHODS = ["get", "put", "post", "delete", "options", "head", "patch", "trace"];
const URL_RE = /^[A-Za-z][A-Za-z0-9+.-]*:\/\//;

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

const isObj = (v) => v !== null && typeof v === "object" && !Array.isArray(v);

// The key lines: an independent YAML 1.2 reader (the `yaml` package; JSON is YAML) with
// source offsets, so a line is not the dialect's decode's word.
function lineIndex(src) {
  const lc = new YAML.LineCounter();
  const doc = YAML.parseDocument(src, { lineCounter: lc, uniqueKeys: false });
  return { doc, lc, errors: doc.errors };
}
function pairLine(idx, node, key) {
  if (!YAML.isMap(node)) return 0;
  for (const p of node.items) {
    const k = p.key && (p.key.value !== undefined ? p.key.value : p.key.source);
    if (String(k) === key) return idx.lc.linePos(p.key.range[0]).line;
  }
  return 0;
}
function childNode(node, key) {
  if (!YAML.isMap(node)) return null;
  for (const p of node.items) {
    const k = p.key && (p.key.value !== undefined ? p.key.value : p.key.source);
    if (String(k) === key) return p.value;
  }
  return null;
}

// entity sections: [path in the document, kind, requireMap]
const SECTIONS = [
  [["components", "schemas"], "type_alias", false, "schema"],
  [["definitions"], "type_alias", false, "schema"],
  [["components", "parameters"], "variable_decl", true, "parameter"],
  [["parameters"], "variable_decl", true, "parameter"],
  [["components", "responses"], "message", true, "response"],
  [["responses"], "message", true, "response"],
  [["components", "requestBodies"], "message", true, "request_body"],
  [["components", "headers"], "key", true, "header"],
  [["components", "securitySchemes"], "policy", true, "security_scheme"],
  [["securityDefinitions"], "policy", true, "security_scheme"],
];
// the pointer prefixes a local $ref may name, and the entity group they are
const POINTERS = {
  "components/schemas": "schema", "components/parameters": "parameter", "components/responses": "response",
  "components/requestBodies": "request_body", "components/headers": "header", "components/securitySchemes": "security_scheme",
  "definitions": "schema", "parameters": "parameter", "responses": "response", "securityDefinitions": "security_scheme",
};

function dig(obj, p) {
  let cur = obj;
  for (const k of p) {
    if (!isObj(cur) || !(k in cur)) return undefined;
    cur = cur[k];
  }
  return cur;
}
function digNode(node, p) {
  let cur = node;
  for (const k of p) {
    cur = childNode(cur, k);
    if (!cur) return null;
  }
  return cur;
}

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

const stats = { files: 0, documents: 0, not_documents: 0, rejected: 0, definitions: 0, refs_internal: 0, refs_internal_in_truth: 0,
  refs_internal_unresolved: 0, refs_not_in_section: 0, refs_outside_entities: 0, refs_file: 0, refs_file_in_subject: 0, refs_file_unresolved: 0,
  refs_url: 0, redocly_runs: 0, redocly_unresolved: 0, redocly_disagree: 0, resolve_errors: 0 };
const rejected = {};
const defs = [], refs = [], xrefs = [], docs = [], disagreements = [];
const tmp = fs.mkdtempSync(path.join(process.env.TMPDIR || "/tmp", "openapi-oracle-"));

async function main() {
  for (const rel of findFiles(subject)) {
    stats.files++;
    const full = path.join(subject, rel);
    let api;
    try {
      api = await SwaggerParser.parse(full, { resolve: { external: false, http: false } });
    } catch (e) {
      const why = String(e.message || e).split("\n")[0].replaceAll(subject + path.sep, "").replace(/^\S*\s+(is not a valid)/, "<file> $1").slice(0, 120);
      (rejected[why] ||= []).push(rel);
      stats.rejected++;
      continue;
    }
    if (!isObj(api) || !(("openapi" in api) || ("swagger" in api))) { stats.not_documents++; continue; }
    stats.documents++;
    docs.push(rel);
    const src = fs.readFileSync(full, "utf8");
    const idx = lineIndex(src);
    const root = idx.doc.contents;
    // the root's line: its mapping's first line (a flow mapping's brace, a block mapping's first key)
    const rootLine = root && root.range ? idx.lc.linePos(root.range[0]).line : 1;
    // the document entity needs an `info` (every document has one, and a tool's own config
    // that has an `openapi:` key does not: redocly.yaml)
    if ("info" in api) defs.push({ path: rel, name: path.posix.basename(rel), line: rootLine, kind: "package" });

    // the entities, each with the subtree its references are read under
    const entities = []; // {group, name, kind, value}
    const declared = { schema: new Set(), parameter: new Set(), response: new Set(), request_body: new Set(), header: new Set(), security_scheme: new Set() };
    for (const sec of ["paths", "webhooks"]) {
      const items = api[sec];
      if (!isObj(items)) continue;
      const itemsNode = childNode(root, sec);
      for (const [pathKey, item] of Object.entries(items)) {
        if (!isObj(item)) continue;
        const itemNode = childNode(itemsNode, pathKey);
        for (const m of METHODS) {
          if (!isObj(item[m])) continue;
          const name = m.toUpperCase() + " " + pathKey;
          defs.push({ path: rel, name, line: pairLine(idx, itemNode, m), kind: "route" });
          entities.push({ name, value: item[m] });
        }
      }
    }
    for (const [p, kind, requireMap, group] of SECTIONS) {
      const sec = dig(api, p);
      if (!isObj(sec)) continue;
      const secNode = digNode(root, p);
      for (const [k, v] of Object.entries(sec)) {
        if (requireMap && !isObj(v)) continue;
        defs.push({ path: rel, name: String(k), line: pairLine(idx, secNode, k), kind });
        entities.push({ name: String(k), value: v, group });
        declared[group].add(String(k));
      }
    }
    stats.definitions += defs.filter((d) => d.path === rel).length;

    // the tool's word on every reference
    let $refs = null;
    try {
      $refs = await SwaggerParser.resolve(full, { resolve: { http: false } });
    } catch (e) {
      stats.resolve_errors++;
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
        if (!exists(ref)) { stats.refs_internal_unresolved++; continue; }
        const segs = ref.slice(2).split("/");
        let group = null, name = null;
        for (const take of [2, 1]) {
          const prefix = segs.slice(0, take).map(unescapePointer).join("/");
          if (POINTERS[prefix] && segs.length > take) { group = POINTERS[prefix]; name = unescapePointer(segs[take]); break; }
        }
        if (!group || !declared[group].has(name)) { stats.refs_not_in_section++; continue; }
        stats.refs_internal_in_truth++;
        const key = ent.name + "\u0000" + name;
        if (!seen.has(key)) { seen.add(key); refs.push({ path: rel, from: ent.name, to: name }); }
      }
    }
    // refs outside every entity: counted, not in the truth
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

    if (doRedocly && fileRefs.size > 0) {
      const bin = path.join(toolsDir, "node_modules", ".bin", "redocly");
      const r = spawnSync(bin, ["bundle", full, "--output", path.join(tmp, "bundled.yaml"), "--format", "json", "--force"], { encoding: "utf8", env: { ...process.env, REDOCLY_TELEMETRY: "off", NO_UPDATE_NOTIFIER: "1" } });
      stats.redocly_runs++;
      const text = (r.stdout || "") + (r.stderr || "");
      const unresolved = (text.match(/Can't resolve \$ref/g) || []).length;
      stats.redocly_unresolved += unresolved;
      const mine = [...refsUnder(api)].filter((x) => !x.startsWith("#/") && !URL_RE.test(x.split("#")[0]) && x.split("#")[0]).filter((x) => {
        const file = x.split("#")[0];
        const abs = file.startsWith("/") ? path.join(subject, file) : path.resolve(path.dirname(full), file);
        return !fs.existsSync(abs);
      }).length;
      if (mine !== unresolved) { stats.redocly_disagree++; disagreements.push(`${rel}: redocly cannot resolve ${unresolved}, the oracle ${mine}`); }
    }
  }
  fs.rmSync(tmp, { recursive: true, force: true });

  const sortDefs = defs.sort((a, b) => (a.path < b.path ? -1 : a.path > b.path ? 1 : a.line - b.line || (a.name < b.name ? -1 : 1)));
  fs.writeFileSync(outFile, JSON.stringify({ tool: "@apidevtools/swagger-parser", version, tier: "A", definitions: sortDefs, references: refs }, null, 1) + "\n");
  if (xrefOut && xrefs.length) {
    fs.writeFileSync(xrefOut, JSON.stringify({ tool: "@apidevtools/swagger-parser ($refs.paths)", version, tier: "A", cross_references: xrefs }, null, 1) + "\n");
  }
  if (filesOut) fs.writeFileSync(filesOut, docs.sort().map((d) => d + "\n").join(""));
  stats.cross_references = xrefs.length;
  if (summaryOut) fs.writeFileSync(summaryOut, JSON.stringify({ stats, rejected, disagreements }, null, 1) + "\n");
  console.error("openapi-oracle: " + Object.entries(stats).map(([k, v]) => `${k} ${v}`).join(", "));
  for (const why of Object.keys(rejected).sort()) console.error(`  rejected (${why}): ${rejected[why].length} file(s), e.g. ${rejected[why][0]}`);
  for (const d of disagreements.slice(0, 10)) console.error("  " + d);
}
main().catch((e) => { console.error(e); process.exit(1); });
