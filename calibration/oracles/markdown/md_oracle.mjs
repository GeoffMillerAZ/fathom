// md_oracle.mjs -- Markdown's L3 oracle (item markdown, LB-Q-markdown; design
// section 1.3). lychee (MIT OR Apache-2.0) run --offline over a subject's .md
// and .mdx files resolves every local link against the file system; universal
// ctags (Markdown) names the headings. A link lychee resolved to another
// Markdown file of the subject is a cross-file reference (path, from, to_path):
// from is the section holding the link (the last ctags heading at or before
// the link's line, the first heading for a link above all of them), to_path
// the file its file:// URL names with query and fragment dropped. A link to a
// file that is not Markdown, a link lychee could not resolve, a remote one and
// a link within its own file name no cross-file reference and are counted in
// `disclose`, never truth. A link to a heading of its own file (lychee
// --include-fragments checked the fragment exists) is a reference (path, from,
// to): to the heading whose GitHub slug the fragment is, slugged here from
// ctags' heading names (an explicit {#id} wins).
//
// Usage:
//   node md_oracle.mjs -list <subject>            # writes the .md/.mdx list both tools read
//   (cd <subject> && lychee --offline --include-fragments --no-progress -v --format json \
//      --root-dir <abs subject> --files-from <list> > <lychee.json>)
//   (cd <subject> && ctags --languages=Markdown --langmap=Markdown:+.mdx \
//      --output-format=json --fields=+n -L <list> -f - > <ctags.json>)
//   node md_oracle.mjs -subject <subject> -lychee <lychee.json> -ctags <ctags.json> \
//      -lychee-version <v> -ctags-version <v> [-out <file>]
// Bench-only: never a product dependency, run in the measuring harness only.
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const MD = /\.mdx?$/i;
const HEADING_KINDS = ["chapter", "section", "subsection", "subsubsection", "l4subsection", "l5subsection"];

export function walkMD(root, rel = "", out = []) {
  for (const ent of fs.readdirSync(path.join(root, rel), { withFileTypes: true }).sort((a, b) => (a.name < b.name ? -1 : a.name > b.name ? 1 : 0))) {
    if (ent.name === ".git" || ent.name === "node_modules") continue;
    const r = rel ? rel + "/" + ent.name : ent.name;
    if (ent.isDirectory()) walkMD(root, r, out);
    else if (ent.isFile() && MD.test(ent.name)) out.push(r);
  }
  return out;
}

// target is the subject Markdown file a resolved file:// URL names, "" when none.
export function target(subject, url) {
  let p;
  try {
    const u = new URL(url);
    if (u.protocol !== "file:") return { to: "", why: "remote" };
    u.search = "";
    u.hash = "";
    p = fileURLToPath(u);
  } catch {
    return { to: "", why: "unparsed" };
  }
  const rel = path.relative(subject, p);
  if (rel === "" || rel.startsWith("..") || path.isAbsolute(rel)) return { to: "", why: "outside" };
  if (!MD.test(rel)) return { to: "", why: "not markdown" };
  return { to: rel.split(path.sep).join("/"), why: "" };
}

// headingsOf groups ctags' tags by path, ordered by line.
export function headingsOf(ctagsText) {
  const by = {};
  for (const line of ctagsText.split("\n")) {
    if (!line.startsWith("{")) continue;
    const t = JSON.parse(line);
    if (t._type !== "tag" || !HEADING_KINDS.includes(t.kind)) continue;
    (by[t.path] ||= []).push({ name: t.name, line: t.line });
  }
  for (const k of Object.keys(by)) by[k].sort((a, b) => a.line - b.line || (a.name < b.name ? -1 : 1));
  return by;
}

// slugs maps a file's anchors to its heading names: GitHub's rule (lower-case,
// markup and punctuation dropped, spaces to hyphens, a repeat suffixed -1, -2),
// an explicit {#id} winning.
export function slugs(heads) {
  const out = new Map();
  const seen = new Map();
  for (const h of heads) {
    const id = /\s*\{#([^}\s]+)\}\s*$/.exec(h.name);
    if (id) { if (!out.has(id[1].toLowerCase())) out.set(id[1].toLowerCase(), h.name); continue; }
    let t = h.name.replace(/\[([^\]]*)\]\([^)]*\)/g, "$1").replace(/<[^>]*>/g, "").toLowerCase();
    t = t.replace(/[^\p{L}\p{N}\s_-]/gu, "").replace(/\s/g, "-");
    const k = seen.get(t) || 0;
    seen.set(t, k + 1);
    if (k > 0) t = `${t}-${k}`;
    if (!out.has(t)) out.set(t, h.name);
  }
  return out;
}

export function convert(subject, report, ctagsText, versions) {
  const heads = headingsOf(ctagsText);
  const out = { tool: "lychee+universal-ctags", version: `lychee ${versions.lychee}, ctags ${versions.ctags}`, tier: "B", cross_references: [], references: [], disclose: [] };
  const why = {};
  let links = 0;
  const seen = new Set();
  const refSeen = new Set();
  const slugCache = new Map();
  for (const [file, entries] of Object.entries(report.success_map || {}).sort()) {
    const from = file.split(path.sep).join("/");
    for (const e of entries) {
      links++;
      const t = target(subject, e.url);
      if (!t.to) {
        why[t.why] = (why[t.why] || 0) + 1;
        continue;
      }
      const hs = heads[from] || [];
      if (t.to === from) {
        const frag = (() => { try { return decodeURIComponent(new URL(e.url).hash.slice(1)).toLowerCase(); } catch { return ""; } })();
        if (!slugCache.has(from)) slugCache.set(from, slugs(hs));
        const to = frag ? slugCache.get(from).get(frag) : undefined;
        const line = (e.span && e.span.line) || 0;
        let sec = hs.length ? hs[0].name : "";
        for (const h of hs) { if (h.line > line) break; sec = h.name; }
        if (to && sec && to !== sec && !refSeen.has(from + "\0" + sec + "\0" + to)) {
          refSeen.add(from + "\0" + sec + "\0" + to);
          out.references.push({ path: from, from: sec, to });
        } else {
          why["within its own file, not a reference"] = (why["within its own file, not a reference"] || 0) + 1;
        }
        continue;
      }
      if (hs.length === 0) {
        why["file with no heading"] = (why["file with no heading"] || 0) + 1;
        continue;
      }
      const line = (e.span && e.span.line) || 0;
      let sec = hs[0].name;
      for (const h of hs) {
        if (h.line > line) break;
        sec = h.name;
      }
      const key = from + "\0" + sec + "\0" + t.to;
      if (seen.has(key)) continue;
      seen.add(key);
      out.cross_references.push({ path: from, from: sec, to_path: t.to });
    }
  }
  out.cross_references.sort((a, b) => (a.path < b.path ? -1 : a.path > b.path ? 1 : a.from < b.from ? -1 : a.from > b.from ? 1 : a.to_path < b.to_path ? -1 : a.to_path > b.to_path ? 1 : 0));
  out.references.sort((a, b) => (a.path < b.path ? -1 : a.path > b.path ? 1 : a.from < b.from ? -1 : a.from > b.from ? 1 : a.to < b.to ? -1 : a.to > b.to ? 1 : 0));
  let errors = 0;
  for (const entries of Object.values(report.error_map || {})) errors += entries.length;
  const parts = Object.keys(why).sort().map((k) => `${why[k]} ${k}`).join(", ");
  out.disclose.push(`lychee ${versions.lychee} --offline: ${links} local links it resolved, ${out.cross_references.length} distinct (section, file) pairs to another Markdown file of the subject, ${out.references.length} (section, heading) references within a file${parts ? ` (left out: ${parts})` : ""}; ${errors} links it could not resolve are not truth`);
  return out;
}

function main(argv) {
  const arg = (k) => { const i = argv.indexOf(k); return i >= 0 ? argv[i + 1] : undefined; };
  if (arg("-list")) { process.stdout.write(walkMD(path.resolve(arg("-list"))).join("\n") + "\n"); return; }
  const subject = arg("-subject"), lychee = arg("-lychee"), ctags = arg("-ctags"), lv = arg("-lychee-version"), cv = arg("-ctags-version"), outPath = arg("-out");
  if (!subject || !lychee || !ctags || !lv || !cv) {
    console.error("usage: md_oracle.mjs -subject <dir> -lychee <lychee.json> -ctags <ctags.json> -lychee-version <v> -ctags-version <v> [-out <file>] | -list <dir>");
    process.exit(2);
  }
  const out = convert(path.resolve(subject), JSON.parse(fs.readFileSync(lychee, "utf8")), fs.readFileSync(ctags, "utf8"), { lychee: lv, ctags: cv });
  const text = JSON.stringify(out, null, 2) + "\n";
  if (outPath) fs.writeFileSync(outPath, text); else process.stdout.write(text);
}

if (import.meta.url === `file://${process.argv[1]}`) main(process.argv.slice(2));
