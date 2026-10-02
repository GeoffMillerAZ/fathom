// convert.js <js-callgraph.json> <subject-root> <version>: the tool's edge list
// as the neutral tool-oracle JSON (tier B) on stdout. An edge is kept when the
// caller is a named function in a file under the subject and the callee is a
// named function the tool resolved to a file under the subject: the tool's
// `global` (module-level code), `anon` callers and `Native` targets are not
// definitions fathom names a pair by.
const fs = require("fs");
const path = require("path");
const [, , cgPath, rootArg, version] = process.argv;
const root = fs.realpathSync(rootArg);
const edges = JSON.parse(fs.readFileSync(cgPath, "utf8"));
const rel = (f) => {
  if (!f || f === "Native") return null;
  const r = path.relative(root, f);
  return r.startsWith("..") || path.isAbsolute(r) ? null : r.split(path.sep).join("/");
};
const seen = new Set();
const calls = [];
for (const e of edges) {
  const from = e.source && e.source.label, to = e.target && e.target.label;
  const p = rel(e.source && e.source.file), q = rel(e.target && e.target.file);
  if (!p || !q || !from || !to || from === "global" || from === "anon" || to === "anon") continue;
  const k = p + "\0" + from + "\0" + to;
  if (seen.has(k)) continue;
  seen.add(k);
  calls.push({ path: p, from, to });
}
calls.sort((a, b) => (a.path + a.from + a.to < b.path + b.from + b.to ? -1 : 1));
process.stdout.write(JSON.stringify({
  // No `source`: that field marks a third-party query (tier C, ADR-048 D5);
  // this is a tool run unmodified, tier B. Provenance: README.md.
  tool: "js-callgraph", version, tier: "B", revision: "7abc6bb04fed9d82640716cc410db21fe1e9eb00",
  licence: "EPL-2.0", calls,
}, null, 1) + "\n");
