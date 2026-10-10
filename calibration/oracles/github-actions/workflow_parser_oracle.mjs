// workflow_parser_oracle.mjs: the language-breadth harness's tier-A oracle for
// the github-actions doc-engine dialect (README.md "Why this is tier A").
//
// Workflows go through GitHub's own parser, @actions/workflow-parser (MIT,
// github.com/actions/languageservices: the parser behind GitHub's Actions
// language server and its VS Code extension): parseWorkflow validates each file
// against the workflow schema and convertWorkflowTemplate evaluates it (job ids,
// needs, the needs graph, reusable-workflow references), each token carrying the
// parser's own range. Action metadata (action.yml, action.yaml) goes through
// the same package's parseAction and convertActionTemplate. Nothing is read off
// the YAML by this program: every entity, name and line is a parser token.
//
// stdout (tier A): definitions (a `target` per workflow or action name, a `job`
// per job, a `step` per step with a written id, a `resource` per step `uses`, a
// `variable_decl` per action input and an `output` per action output) and
// references (a job to each job its needs names, when the converter accepts the
// needs). -xref-out (tier C): cross_references, a local `uses: ./path` at file
// granularity: a job's reusable workflow when the converter loaded it through the
// file provider (GitHub's own ./ rule, parseFileReference), and a step's action
// directory resolved by this program to the action.yml or action.yaml in it that
// parseAction accepts (the directory rule is GitHub's documented one, but no
// offline GitHub tool resolves it: the C rung). Counts go to stderr.
//
//	node workflow_parser_oracle.mjs -tools <dir holding node_modules> -subject <checkout> \
//	  [-xref-out <file>] [-files-out <file>] > <out.json>
//
// -files-out names the files the parser accepted, one per line: the reading's
// `lang-oracle -only` scope (a file the parser rejects is attested by nothing,
// so it is named on stderr and left out of the measurement, never scored).
import { readFileSync, readdirSync, statSync, writeFileSync } from "node:fs";
import { registerHooks } from "node:module";
import path from "node:path";
import { pathToFileURL } from "node:url";

const TOOL = "@actions/workflow-parser (parseWorkflow + convertWorkflowTemplate; parseAction + convertActionTemplate)";

// The package is published for bundlers: its schema is imported as
// `import WorkflowSchema from "../workflow-v1.0.min.json"` with no import
// attribute, which Node refuses. This load hook serves each of the package's
// own .json files as a module whose default export is the parsed file,
// byte for byte the schema the package ships.
let hooked = false;
function hookJSON(dist) {
  if (hooked) return;
  hooked = true;
  const prefix = pathToFileURL(dist).href;
  registerHooks({
    load(url, context, nextLoad) {
      if (url.startsWith(prefix) && url.endsWith(".json")) {
        const body = readFileSync(new URL(url), "utf8");
        return { format: "module", source: `export default ${body};`, shortCircuit: true };
      }
      return nextLoad(url, context);
    },
  });
}

async function load(tools) {
  const dist = path.join(tools, "node_modules", "@actions", "workflow-parser", "dist");
  hookJSON(dist);
  const imp = (p) => import(pathToFileURL(path.join(dist, p)).href);
  const [idx, act, conv] = await Promise.all([imp("index.js"), imp("actions/index.js"), imp("model/convert.js")]);
  const pkg = JSON.parse(readFileSync(path.join(dist, "..", "package.json"), "utf8"));
  return { ...idx, ...act, ErrorPolicy: conv.ErrorPolicy, version: pkg.version };
}

const line = (tok) => (tok && tok.range ? tok.range.start.line : 0);
const sameRange = (a, b) =>
  a && b && a.start.line === b.start.line && a.start.column === b.start.column &&
  a.end.line === b.end.line && a.end.column === b.end.column;
const str = (tok) => (tok && typeof tok.value === "string" ? tok.value : "");
const find = (map, key) => (map && typeof map.find === "function" ? map.find(key) : undefined);
const items = (tok) => (tok && typeof tok[Symbol.iterator] === "function" ? [...tok] : []);

// walk lists the subject's workflow files (directly under .github/workflows,
// GitHub's rule) and action metadata files, skipping .git and node_modules.
function walk(root) {
  const workflows = [];
  const actions = [];
  const rec = (dir) => {
    for (const e of readdirSync(dir, { withFileTypes: true }).sort((a, b) => (a.name < b.name ? -1 : 1))) {
      const abs = path.join(dir, e.name);
      if (e.isDirectory()) {
        if (e.name !== ".git" && e.name !== "node_modules") rec(abs);
        continue;
      }
      if (!e.isFile()) continue;
      const rel = path.relative(root, abs).split(path.sep).join("/");
      if (path.posix.dirname(rel) === ".github/workflows" && /\.ya?ml$/.test(e.name)) workflows.push(rel);
      else if (e.name === "action.yml" || e.name === "action.yaml") actions.push(rel);
    }
  };
  rec(root);
  return { workflows, actions };
}

function isFile(p) {
  try {
    return statSync(p).isFile();
  } catch {
    return false;
  }
}

export async function run({ tools, subject }) {
  const P = await load(tools);
  const trace = new P.NoOperationTraceWriter();
  const out = { tool: TOOL, version: `@actions/workflow-parser ${P.version}`, tier: "A", definitions: [], references: [] };
  const xref = { tool: TOOL + " + local uses: directory rule", version: out.version, tier: "C", definitions: [], cross_references: [] };
  const stats = {
    workflows: 0, workflowsRejected: 0, actions: 0, actionsRejected: 0, actionsNotComposite: 0,
    needsRejected: 0, unnamedWorkflows: 0, otherConverterErrors: 0, localUses: 0, localUsesUnresolved: 0, remoteReusable: 0,
  };
  const rejected = [];
  const accepted = [];
  const { workflows, actions } = walk(subject);
  const def = (p, name, ln, kind) => name && out.definitions.push({ path: p, name, line: ln, kind });
  const seenX = new Set();
  const addX = (p, from, to) => {
    const k = `${p}\0${from}\0${to}`;
    if (!from || !to || seenX.has(k)) return;
    seenX.add(k);
    xref.cross_references.push({ path: p, from, to_path: to });
  };
  // actionAt resolves a step's local `uses: ./dir` to the metadata file
  // parseAction accepts in that directory (GitHub's directory rule).
  const actionOK = new Map();
  const actionAt = (uses) => {
    if (!uses.startsWith("./")) return "";
    stats.localUses++;
    const dir = path.posix.normalize(uses.slice(2)).replace(/\/$/, "");
    if (dir.startsWith("..")) {
      stats.localUsesUnresolved++;
      return "";
    }
    for (const base of ["action.yml", "action.yaml"]) {
      const rel = dir === "." || dir === "" ? base : `${dir}/${base}`;
      if (!actionOK.has(rel)) {
        let ok = false;
        const abs = path.join(subject, rel);
        if (isFile(abs)) {
          const r = P.parseAction({ name: rel, content: readFileSync(abs, "utf8") }, trace);
          ok = r.value !== undefined && r.context.errors.count === 0;
        }
        actionOK.set(rel, ok);
      }
      if (actionOK.get(rel)) return rel;
    }
    stats.localUsesUnresolved++;
    return "";
  };
  // A run step is an entity named by its position, `<path>#<index>` with the
  // path the dotted keys from the document root to the step list
  // (`jobs.build.steps#2`, `runs.steps#0`): the dialect's `fmt:{$path}#{$index}`.
  const steps = (p, from, seq, where) => {
    let i = -1;
    for (const s of items(seq)) {
      i++;
      def(p, str(find(s, "id")), line(s), "step");
      if (find(s, "run")) def(p, `${where}#${i}`, line(s), "step");
      const uses = str(find(s, "uses"));
      if (uses) {
        def(p, uses, line(s), "resource");
        if (uses.startsWith("./")) addX(p, from, actionAt(uses));
      }
    }
  };

  // The file provider the converter loads a reusable workflow through: a local
  // reference ({path}, repository-root relative by parseFileReference) is read
  // from the checkout; a remote one is not fetched (offline).
  const loaded = new Map();
  const provider = {
    async getFileContent(ref) {
      if ("repository" in ref) {
        stats.remoteReusable++;
        throw new Error("offline");
      }
      const abs = path.join(subject, ref.path);
      const content = readFileSync(abs, "utf8");
      loaded.set(ref.path, true);
      return { name: ref.path, content };
    },
  };

  for (const rel of workflows) {
    stats.workflows++;
    const parsed = P.parseWorkflow({ name: rel, content: readFileSync(path.join(subject, rel), "utf8") }, trace);
    if (!parsed.value || parsed.context.errors.count > 0) {
      stats.workflowsRejected++;
      rejected.push(rel);
      continue;
    }
    accepted.push(rel);
    const root = parsed.value;
    const before = parsed.context.errors.count;
    const tmpl = await P.convertWorkflowTemplate(parsed.context, root, provider, {
      fetchReusableWorkflowDepth: 1,
      errorPolicy: P.ErrorPolicy.TryConversion,
    });
    const errs = parsed.context.errors.getErrors().slice(before);
    // An error's prefix is "<file> (Line: n, Col: m)": the file it is in.
    const inFile = (e) => !e.prefix || e.prefix === rel || e.prefix.startsWith(rel + " (");
    const errAt = (tok) => tok && errs.some((e) => sameRange(e.range, tok.range) && inFile(e));
    let accounted = 0;
    const nameTok = find(root, "name");
    // An unnamed workflow is shown by GitHub under its file path (workflow syntax: "name"). The dialect
    // names it from the file (its base name up to the first dot, `.name | $file cut:.`), a fathom naming
    // convention and not the parser's: counted (unnamedWorkflows) so the reading can disclose it.
    let wfName = str(nameTok);
    if (!wfName) {
      wfName = path.posix.basename(rel).split(".")[0];
      stats.unnamedWorkflows++;
    }
    def(rel, wfName, line(root), "target");
    const jobsTok = find(root, "jobs");
    const jobTokens = new Map(items(jobsTok).map((kv) => [str(kv.key), kv.value]));
    for (const job of tmpl.jobs || []) {
      const id = str(job.id);
      def(rel, id, line(job.id), "job");
      for (const n of job.needs || []) {
        if (errAt(n)) {
          stats.needsRejected++;
          accounted++;
          continue;
        }
        out.references.push({ path: rel, from: id, to: str(n) });
      }
      if (job.type === "reusableWorkflowJob") {
        const ref = str(job.ref);
        if (ref.startsWith("./")) {
          stats.localUses++;
          const target = path.posix.normalize(ref.slice(2));
          if (!errAt(job.ref) && loaded.has(ref.slice(2))) addX(rel, id, target);
          else stats.localUsesUnresolved++;
        }
        if (errAt(job.ref)) accounted += errs.filter((e) => sameRange(e.range, job.ref.range)).length;
      }
      steps(rel, id, find(jobTokens.get(id), "steps"), `jobs.${id}.steps`);
    }
    stats.otherConverterErrors += Math.max(0, errs.length - accounted);
  }

  for (const rel of actions) {
    stats.actions++;
    const parsed = P.parseAction({ name: rel, content: readFileSync(path.join(subject, rel), "utf8") }, trace);
    if (!parsed.value || parsed.context.errors.count > 0) {
      stats.actionsRejected++;
      rejected.push(rel);
      continue;
    }
    const tmpl = P.convertActionTemplate(parsed.context, parsed.value, { errorPolicy: P.ErrorPolicy.TryConversion });
    if (parsed.context.errors.count > 0) {
      stats.actionsRejected++;
      rejected.push(rel);
      continue;
    }
    accepted.push(rel);
    const root = parsed.value;
    const name = tmpl.name || "";
    def(rel, name, line(root), "target");
    for (const [key, kind, list] of [["inputs", "variable_decl", tmpl.inputs], ["outputs", "output", tmpl.outputs]]) {
      const ids = new Set((list || []).map((d) => d.id));
      for (const kv of items(find(root, key))) {
        const id = str(kv.key);
        if (ids.has(id) && kv.value && typeof kv.value.find === "function") def(rel, id, line(kv.value), kind);
      }
    }
    if (tmpl.runs && tmpl.runs.using === "composite") steps(rel, name, find(find(root, "runs"), "steps"), "runs.steps");
    else stats.actionsNotComposite++;
  }

  const cmp = (a, b) => (a < b ? -1 : a > b ? 1 : 0);
  out.definitions.sort((a, b) => cmp(a.path, b.path) || a.line - b.line || cmp(a.kind, b.kind) || cmp(a.name, b.name));
  out.references.sort((a, b) => cmp(a.path, b.path) || cmp(a.from, b.from) || cmp(a.to, b.to));
  xref.cross_references.sort((a, b) => cmp(a.path, b.path) || cmp(a.from, b.from) || cmp(a.to_path, b.to_path));
  accepted.sort();
  rejected.sort();
  return { out, xref, stats, accepted, rejected };
}

async function main(argv) {
  const args = {};
  for (let i = 0; i < argv.length; i += 2) args[argv[i].replace(/^-+/, "")] = argv[i + 1];
  if (!args.tools || !args.subject) {
    process.stderr.write("usage: node workflow_parser_oracle.mjs -tools <dir> -subject <checkout> [-xref-out <file>] [-files-out <file>]\n");
    process.exit(2);
  }
  const r = await run({ tools: args.tools, subject: args.subject });
  process.stdout.write(JSON.stringify(r.out, null, 1) + "\n");
  if (args["xref-out"]) writeFileSync(args["xref-out"], JSON.stringify(r.xref, null, 1) + "\n");
  if (args["files-out"]) writeFileSync(args["files-out"], r.accepted.map((f) => f + "\n").join(""));
  process.stderr.write(`github-actions oracle: ${JSON.stringify(r.stats)}\n`);
  for (const f of r.rejected) process.stderr.write(`  rejected by the parser: ${f}\n`);
}

if (import.meta.url === pathToFileURL(process.argv[1] || "").href) {
  main(process.argv.slice(2)).catch((e) => {
    process.stderr.write(`github-actions oracle: ${e.stack || e}\n`);
    process.exit(1);
  });
}
