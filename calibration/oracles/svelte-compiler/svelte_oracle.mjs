// svelte_oracle.mjs — Svelte components' tier-A oracle (ruling W4, design
// §1.8; wave lf7, item vuesvelte, K19): svelte/compiler `parse` (MIT), the
// Svelte compiler's own parser, reads each .svelte file of a subject (the
// modern AST) and writes lang-oracle's neutral tool-oracle JSON in the shape
// calibration/oracles/vue-compiler-sfc writes, from the same rules:
//
//   definitions  the top-level declarations of the instance and module
//                <script> (ast.instance.content, ast.module.content: the
//                compiler's ESTree Program, TypeScript read by its own acorn
//                plugin), kinds as the Vue oracle names them (function, class,
//                constant, variable, interface, type, enum, import).
//   calls        (path, "<module>", name) for each template node the compiler
//                types a component (a Component node: a capitalized tag or a
//                dotted one, `<Card.Root>`, its name as written; the pair is
//                compared by its last segment, owner ruling M2). <svelte:
//                component this={...}> and <svelte:self> (no name of their own)
//                are counted and left out.
//   call_exclusions  each name a script calls that no component of the
//                template names, reason "script-call" (the Vue oracle's rule).
//   imports      each static import or re-export of a script with a relative
//                specifier, resolved by Vite's rule (SvelteKit's bundler);
//                alias ($lib, $app, @/) and package specifiers counted.
//   disclose     the counts a reading names.
//
// A file the compiler refuses to parse has no truth: it is counted and named.
//
// Usage: node svelte_oracle.mjs -tools <npm prefix holding svelte> -subject <dir> [-out <file>]
// Bench-only: never a product dependency, run in the measuring harness only.
import fs from "node:fs";
import path from "node:path";
import { createRequire } from "node:module";
import { declarations, calleeNames, importStatements, resolveRelative, specClass } from "../vue-compiler-sfc/vue_sfc_oracle.mjs";

const MODULE = "<module>";

// templateComponents is the names of a fragment's Component nodes, and the
// count of svelte:component and svelte:self nodes left out.
export function templateComponents(fragment) {
  const names = new Set();
  let dynamic = 0;
  const seen = new Set();
  const walk = (n) => {
    if (!n || typeof n !== "object" || seen.has(n)) return;
    seen.add(n);
    if (Array.isArray(n)) { for (const x of n) walk(x); return; }
    if (n.type === "Component") names.add(n.name);
    else if (n.type === "SvelteComponent" || n.type === "SvelteSelf") dynamic++;
    for (const k of Object.keys(n)) {
      if (k === "loc" || k === "metadata" || k === "start" || k === "end") continue;
      const v = n[k];
      if (v && typeof v === "object") walk(v);
    }
  };
  walk(fragment);
  return { names, dynamic };
}

function walkSvelte(root, rel = "", out = []) {
  for (const ent of fs.readdirSync(path.join(root, rel), { withFileTypes: true }).sort((a, b) => (a.name < b.name ? -1 : a.name > b.name ? 1 : 0))) {
    if (ent.name === ".git" || ent.name === "node_modules") continue;
    const r = rel ? rel + "/" + ent.name : ent.name;
    if (ent.isDirectory()) walkSvelte(root, r, out);
    else if (ent.isFile() && ent.name.endsWith(".svelte")) out.push(r);
  }
  return out;
}

const scriptIsTS = (s) => (s.attributes || []).some((a) => a.name === "lang" && Array.isArray(a.value) && a.value.some((v) => v.data === "ts" || v.data === "typescript"));

export function run(compiler, version, subject) {
  const out = { tool: "svelte/compiler parse", version, tier: "A", definitions: [], calls: [], call_exclusions: [], imports: [], disclose: [] };
  const counts = { files: 0, refused: [], dynamic: 0, components: 0, scriptCallees: 0, relative: 0, resolved: 0, unresolved: 0, alias: 0, pkg: 0 };
  for (const rel of walkSvelte(subject)) {
    const text = fs.readFileSync(path.join(subject, rel), "utf8");
    counts.files++;
    let ast;
    try {
      ast = compiler.parse(text, { modern: true, filename: rel });
    } catch (e) {
      counts.refused.push(rel);
      continue;
    }
    const t = templateComponents(ast.fragment);
    counts.dynamic += t.dynamic;
    const names = t.names;
    for (const n of [...names].sort()) out.calls.push({ path: rel, from: MODULE, to: n });
    counts.components += names.size;
    const callees = new Set();
    for (const s of [ast.module, ast.instance].filter(Boolean)) {
      const body = s.content.body;
      const ts = scriptIsTS(s);
      out.definitions.push(...declarations(body, rel, 0));
      for (const c of calleeNames(body)) callees.add(c);
      for (const st of importStatements(body)) {
        const spec = st.source.value;
        const cls = specClass(spec);
        if (cls === "alias") { counts.alias++; continue; }
        if (cls === "package") { counts.pkg++; continue; }
        counts.relative++;
        const to = resolveRelative(subject, rel, spec, ts);
        if (!to) { counts.unresolved++; continue; }
        counts.resolved++;
        out.imports.push({ path: rel, line: st.loc.start.line, to_path: to, module: spec });
      }
    }
    for (const c of [...callees].sort()) {
      if (names.has(c) || [...names].some((n) => n.endsWith("." + c))) continue;
      counts.scriptCallees++;
      out.call_exclusions.push({ path: rel, to: c, reason: "script-call" });
    }
  }
  out.disclose.push(`svelte/compiler ${version} parse (modern AST): ${counts.files} files (${counts.refused.length} the compiler refused to parse, no truth: ${counts.refused.slice(0, 5).join(", ") || "none"}), ${counts.components} (file, component) pairs from its Component nodes, ${counts.dynamic} svelte:component and svelte:self left out; ${counts.scriptCallees} (file, name) script calls left out of the component pairs (call_exclusions, script-call); ${counts.relative} relative import statements (${counts.resolved} resolved by Vite's rule, ${counts.unresolved} unresolved), ${counts.alias} alias and ${counts.pkg} package specifiers not resolved`);
  return out;
}

function main(argv) {
  const arg = (k) => { const i = argv.indexOf(k); return i >= 0 ? argv[i + 1] : undefined; };
  const tools = arg("-tools"), subject = arg("-subject"), outPath = arg("-out");
  if (!tools || !subject) { console.error("usage: svelte_oracle.mjs -tools <npm prefix> -subject <dir> [-out <file>]"); process.exit(2); }
  const req = createRequire(path.join(path.resolve(tools), "package.json"));
  const compiler = req("svelte/compiler");
  const version = JSON.parse(fs.readFileSync(path.join(path.resolve(tools), "node_modules", "svelte", "package.json"), "utf8")).version;
  const out = run(compiler, version, path.resolve(subject));
  const text = JSON.stringify(out, null, 2) + "\n";
  if (outPath) fs.writeFileSync(outPath, text); else process.stdout.write(text);
}

if (import.meta.url === `file://${process.argv[1]}`) main(process.argv.slice(2));
