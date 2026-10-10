// lychee_oracle.mjs — HTML's tier-A L3 oracle (wave lf5, item html; design
// §1.3: tier A only for what it attests). lychee (MIT OR Apache-2.0) run
// --offline over a subject's .html/.htm files checks every local link it
// extracts against the file system; this converter turns the links it found
// (success_map) into lang-oracle's neutral `imports`: (path, line, to_path),
// the line lychee's own span gives, to_path the subject file the link's file://
// URL names with its query and fragment dropped. A directory link counts only
// as lychee was told to accept it (--index-files index.html,index.htm: valid
// when an index file exists), and names the first index file in that order. A
// link lychee could not resolve (error_map), one outside the subject and a
// remote one (--offline excludes it) name no file and are counted in
// `disclose`, never truth.
//
// Usage:
//   node lychee_oracle.mjs -list <subject>  # writes the .html/.htm list lychee reads
//   (cd <subject> && lychee --offline --no-progress -v --format json \
//      --root-dir <abs subject> --index-files index.html,index.htm \
//      --files-from <list> > <lychee.json>)
//   node lychee_oracle.mjs -subject <subject> -lychee <lychee.json> -version <v> [-out <file>]
// Bench-only: never a product dependency, run in the measuring harness only.
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const INDEX = ["index.html", "index.htm"];

export function walkHTML(root, rel = "", out = []) {
  for (const ent of fs.readdirSync(path.join(root, rel), { withFileTypes: true }).sort((a, b) => a.name < b.name ? -1 : a.name > b.name ? 1 : 0)) {
    if (ent.name === ".git" || ent.name === "node_modules") continue;
    const r = rel ? rel + "/" + ent.name : ent.name;
    if (ent.isDirectory()) walkHTML(root, r, out);
    else if (ent.isFile() && /\.html?$/i.test(ent.name)) out.push(r);
  }
  return out;
}

// target is the subject file a resolved file:// URL names, "" when none.
export function target(subject, url) {
  let p;
  try {
    const u = new URL(url);
    if (u.protocol !== "file:") return { to: "", why: "remote" };
    u.search = ""; u.hash = "";
    p = fileURLToPath(u);
  } catch { return { to: "", why: "unparsed" }; }
  const rel = path.relative(subject, p);
  if (rel === "" || rel.startsWith("..") || path.isAbsolute(rel)) return { to: "", why: "outside" };
  let st;
  try { st = fs.statSync(p); } catch { return { to: "", why: "missing" }; }
  if (st.isDirectory()) {
    for (const idx of INDEX) {
      if (fs.existsSync(path.join(p, idx))) return { to: rel.split(path.sep).join("/") + "/" + idx, why: "" };
    }
    return { to: "", why: "directory" };
  }
  return { to: rel.split(path.sep).join("/"), why: "" };
}

export function convert(subject, report, version) {
  const out = { tool: "lychee", version, tier: "A", imports: [], disclose: [] };
  const why = {};
  let links = 0;
  for (const [file, entries] of Object.entries(report.success_map || {}).sort()) {
    const from = file.split(path.sep).join("/");
    for (const e of entries) {
      links++;
      const t = target(subject, e.url);
      if (!t.to) { why[t.why] = (why[t.why] || 0) + 1; continue; }
      const u = new URL(e.url);
      out.imports.push({ path: from, line: (e.span && e.span.line) || 0, to_path: t.to, module: decodeURIComponent(u.pathname.slice(u.pathname.lastIndexOf("/") + 1)) + u.search + u.hash });
    }
  }
  let errors = 0, excluded = 0;
  for (const entries of Object.values(report.error_map || {})) errors += entries.length;
  for (const entries of Object.values(report.excluded_map || {})) excluded += entries.length;
  out.imports.sort((a, b) => a.path < b.path ? -1 : a.path > b.path ? 1 : a.line - b.line || (a.to_path < b.to_path ? -1 : a.to_path > b.to_path ? 1 : 0));
  const parts = Object.keys(why).sort().map((k) => `${why[k]} ${k}`).join(", ");
  out.disclose.push(`lychee ${version} --offline: ${links} local links it resolved, ${out.imports.length} to a file of the subject${parts ? ` (left out: ${parts})` : ""}; ${errors} links it could not resolve and ${excluded} it excluded (remote, mailto and the like) name no file and are no truth`);
  return out;
}

function main(argv) {
  const arg = (k) => { const i = argv.indexOf(k); return i >= 0 ? argv[i + 1] : undefined; };
  if (arg("-list")) { process.stdout.write(walkHTML(path.resolve(arg("-list"))).join("\n") + "\n"); return; }
  const subject = arg("-subject"), lychee = arg("-lychee"), version = arg("-version"), outPath = arg("-out");
  if (!subject || !lychee || !version) { console.error("usage: lychee_oracle.mjs -subject <dir> -lychee <lychee.json> -version <v> [-out <file>] | -list <dir>"); process.exit(2); }
  const out = convert(path.resolve(subject), JSON.parse(fs.readFileSync(lychee, "utf8")), version);
  const text = JSON.stringify(out, null, 2) + "\n";
  if (outPath) fs.writeFileSync(outPath, text); else process.stdout.write(text);
}

if (import.meta.url === `file://${process.argv[1]}`) main(process.argv.slice(2));
