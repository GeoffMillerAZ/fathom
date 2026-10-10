// parse5_oracle.mjs — HTML's tier-A oracle (ruling W1, design §1.8; wave lf5,
// item html): parse5 (MIT), the WHATWG HTML parsing algorithm in JavaScript,
// reads each .html/.htm file of a subject with sourceCodeLocationInfo and
// writes lang-oracle's neutral tool-oracle JSON:
//
//   definitions  every element carrying a non-empty `id` attribute (kind "id"),
//                template contents included; line is the id attribute's own line
//                (the dialect names a definition at its name), end is the end
//                tag's last line when an end tag is written, the start tag's
//                last line for a void element, and 0 (no end, not scored by
//                range_agreement) when the parser implied the end.
//   calls        W1's same-document ID references as (path, "<module>", id):
//                every attribute the HTML standard's attribute index types as
//                an ID or a set of IDs, on the elements it names (whatwg/html
//                877d6146487f15b15bc8dd0e3c625063a37b456a: commandfor on button;
//                for on label, a set on output; form on button, fieldset,
//                input, object, output, select, textarea; headers on td and th,
//                a set; itemref on any element, a set; list on input;
//                popovertarget on button and input), W1's ARIA ID references on
//                any element (aria-labelledby, aria-describedby, aria-controls,
//                aria-owns, aria-activedescendant; aria-activedescendant one ID,
//                the rest sets), and href="#id" on a and area (the fragment as
//                written). A set is split on ASCII whitespace. The pair's
//                caller is the document's module (K19, design §1.8): an ID is
//                resolved against the element's tree, never its ancestors.
//   call_exclusions  every id two or more elements of a file carry: a reference
//                to it names no one element, so its pairs leave both sides,
//                counted (W1).
//   disclose     the counts a reading names: references, ids named by no
//                element (kept in the truth: an unmatched reference counts
//                against recall), duplicate ids and the references to them,
//                fragments holding a percent escape (kept as written).
//
// Usage: node parse5_oracle.mjs -tools <npm prefix holding parse5@8.0.1>
//          -subject <dir> [-out <file>]
// Bench-only: never a product dependency, run in the measuring harness only.
import fs from "node:fs";
import path from "node:path";
import { createRequire } from "node:module";

const MODULE = "<module>";
const ASCII_WS = /[\t\n\f\r ]+/;

// attribute -> { elements: Set|null (null: any element), set: bool }
export const ID_ATTRS = new Map([
  ["commandfor", { elements: new Set(["button"]), set: false }],
  ["form", { elements: new Set(["button", "fieldset", "input", "object", "output", "select", "textarea"]), set: false }],
  ["headers", { elements: new Set(["td", "th"]), set: true }],
  ["itemref", { elements: null, set: true }],
  ["list", { elements: new Set(["input"]), set: false }],
  ["popovertarget", { elements: new Set(["button", "input"]), set: false }],
  ["aria-labelledby", { elements: null, set: true }],
  ["aria-describedby", { elements: null, set: true }],
  ["aria-controls", { elements: null, set: true }],
  ["aria-owns", { elements: null, set: true }],
  ["aria-activedescendant", { elements: null, set: false }],
]);

// idTokens is the IDs one attribute of an element names (W1), or null when
// the attribute is no ID reference on that element.
export function idTokens(tag, name, value) {
  if (name === "for") {
    if (tag === "label") return value === "" ? [] : [value];
    if (tag === "output") return value.split(ASCII_WS).filter(Boolean);
    return null;
  }
  if (name === "href") {
    if ((tag === "a" || tag === "area") && value.startsWith("#") && value.length > 1) return [value.slice(1)];
    return null;
  }
  const spec = ID_ATTRS.get(name);
  if (!spec || (spec.elements && !spec.elements.has(tag))) return null;
  if (spec.set) return value.split(ASCII_WS).filter(Boolean);
  return value === "" ? [] : [value];
}

function* elements(node) {
  for (const child of node.childNodes || []) {
    if (child.tagName) {
      yield child;
      yield* elements(child);
      if (child.content) yield* elements(child.content); // <template>
    }
  }
}

// readFile is one file's definitions, pairs and counts.
export function readFile(parse5, rel, text) {
  const doc = parse5.parse(text, { sourceCodeLocationInfo: true });
  const defs = [];
  const refs = [];
  const ids = new Map();
  for (const el of elements(doc)) {
    const loc = el.sourceCodeLocation;
    const tag = el.tagName.toLowerCase();
    for (const a of el.attrs) {
      const name = a.name.toLowerCase();
      if (name === "id" && a.value !== "" && loc) {
        const at = (loc.attrs && loc.attrs[a.name]) || loc.startTag || loc;
        let end = 0;
        if (loc.endTag) end = loc.endTag.endLine;
        else if (loc.startTag && isVoid(tag)) end = loc.startTag.endLine;
        defs.push({ path: rel, name: a.value, line: at.startLine, end, kind: "id" });
        ids.set(a.value, (ids.get(a.value) || 0) + 1);
        continue;
      }
      const toks = idTokens(tag, name, a.value);
      if (toks) for (const t of toks) refs.push(t);
    }
  }
  return { defs, refs, ids };
}

const VOID = new Set(["area", "base", "br", "col", "embed", "hr", "img", "input", "link", "meta", "source", "track", "wbr"]);
function isVoid(tag) { return VOID.has(tag); }

function walkHTML(root, rel = "", out = []) {
  for (const ent of fs.readdirSync(path.join(root, rel), { withFileTypes: true }).sort((a, b) => a.name < b.name ? -1 : a.name > b.name ? 1 : 0)) {
    if (ent.name === ".git" || ent.name === "node_modules") continue;
    const r = rel ? rel + "/" + ent.name : ent.name;
    if (ent.isDirectory()) walkHTML(root, r, out);
    else if (ent.isFile() && /\.html?$/i.test(ent.name)) out.push(r);
  }
  return out;
}

export function run(parse5, version, subject) {
  const out = { tool: "parse5", version, tier: "A", definitions: [], calls: [], call_exclusions: [], disclose: [] };
  let refs = 0, unmatched = 0, dupIDs = 0, dupRefs = 0, pct = 0, files = 0;
  for (const rel of walkHTML(subject)) {
    const text = fs.readFileSync(path.join(subject, rel), "utf8");
    const r = readFile(parse5, rel, text);
    files++;
    out.definitions.push(...r.defs);
    const dup = new Set([...r.ids].filter(([, n]) => n > 1).map(([k]) => k));
    dupIDs += dup.size;
    for (const id of [...dup].sort()) out.call_exclusions.push({ path: rel, to: id, reason: "duplicate-id" });
    const seen = new Set();
    for (const t of r.refs) {
      refs++;
      if (t.includes("%")) pct++;
      if (dup.has(t)) { dupRefs++; continue; }
      if (!r.ids.has(t)) unmatched++;
      if (seen.has(t)) continue;
      seen.add(t);
      out.calls.push({ path: rel, from: MODULE, to: t });
    }
  }
  out.disclose.push(`parse5 ${version}: ${files} files, ${out.definitions.length} id-carrying elements, ${refs} W1 ID references (${out.calls.length} distinct (document, id) pairs); ${unmatched} references name no id of their document and stay in the truth (W1: an unmatched reference counts against recall); ${dupIDs} ids carried by two or more elements of one document, their ${dupRefs} references excluded from both sides (W1); ${pct} fragments hold a percent escape, compared as written`);
  return out;
}

function main(argv) {
  const arg = (k) => { const i = argv.indexOf(k); return i >= 0 ? argv[i + 1] : undefined; };
  const tools = arg("-tools"), subject = arg("-subject"), outPath = arg("-out");
  if (!tools || !subject) { console.error("usage: parse5_oracle.mjs -tools <npm prefix> -subject <dir> [-out <file>]"); process.exit(2); }
  const req = createRequire(path.join(path.resolve(tools), "package.json"));
  const parse5 = req("parse5");
  const pkg = path.join(path.resolve(tools), "node_modules", "parse5", "package.json");
  const version = JSON.parse(fs.readFileSync(pkg, "utf8")).version;
  const out = run(parse5, version, path.resolve(subject));
  const text = JSON.stringify(out, null, 2) + "\n";
  if (outPath) fs.writeFileSync(outPath, text); else process.stdout.write(text);
}

if (import.meta.url === `file://${process.argv[1]}`) main(process.argv.slice(2));
