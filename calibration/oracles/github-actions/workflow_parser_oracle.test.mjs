// Tests for workflow_parser_oracle.mjs (node --test). They need the tools
// directory that holds node_modules/@actions/workflow-parser (README):
//
//	GHA_ORACLE_TOOLS=<tools> node --test calibration/oracles/github-actions/
import { test } from "node:test";
import assert from "node:assert/strict";
import { mkdtempSync, mkdirSync, writeFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { run } from "./workflow_parser_oracle.mjs";

const tools = process.env.GHA_ORACLE_TOOLS;

function subject(files) {
  const dir = mkdtempSync(path.join(process.env.TMPDIR || tmpdir(), "gha-oracle-"));
  for (const [rel, body] of Object.entries(files)) {
    mkdirSync(path.join(dir, path.dirname(rel)), { recursive: true });
    writeFileSync(path.join(dir, rel), body);
  }
  return dir;
}

const has = (list, want) => list.some((e) => Object.entries(want).every(([k, v]) => e[k] === v));

test("jobs, needs, steps and uses come from the parser, at its lines", { skip: !tools }, async () => {
  const dir = subject({
    ".github/workflows/ci.yml": [
      "# leading comment",
      "name: CI",
      "on: push",
      "jobs:",
      "  build:",
      "    runs-on: ubuntu-latest",
      "    steps:",
      "      - uses: actions/checkout@v4",
      "      - name: compile",
      "        id: compile",
      "        run: make",
      "  test:",
      "    needs: build",
      "    runs-on: ubuntu-latest",
      "    steps:",
      "      - run: make test",
      "",
    ].join("\n"),
  });
  try {
    const r = await run({ tools, subject: dir });
    assert.equal(r.out.tier, "A");
    const d = r.out.definitions;
    assert.ok(has(d, { name: "CI", kind: "target", line: 2 }), JSON.stringify(d));
    assert.ok(has(d, { name: "build", kind: "job", line: 5 }));
    assert.ok(has(d, { name: "test", kind: "job", line: 12 }));
    assert.ok(has(d, { name: "actions/checkout@v4", kind: "resource", line: 8 }));
    assert.ok(has(d, { name: "compile", kind: "step", line: 9 }));
    // a run step without an id is an entity named by its position, `jobs.<job>.steps#<index>`; one with an id is
    // the step its id names alone (O631, K20); the parser's generated id is not written
    assert.ok(has(d, { name: "jobs.test.steps#0", kind: "step", line: 16 }));
    assert.ok(!d.some((e) => e.name === "jobs.build.steps#1"));
    assert.ok(!d.some((e) => e.name === "jobs.build.steps#0"));
    assert.ok(!d.some((e) => e.kind === "step" && e.name.startsWith("__")));
    // a remote action is also its pin, once per file (LB-F311, K20)
    assert.ok(has(d, { name: "pin:actions/checkout@v4", kind: "resource", line: 8 }));
    assert.deepEqual(r.out.references, [{ path: ".github/workflows/ci.yml", from: "test", to: "build" }]);
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});

test("a needs the parser rejects is not a reference, and is counted", { skip: !tools }, async () => {
  const dir = subject({
    ".github/workflows/ci.yml": [
      "on: push",
      "jobs:",
      "  a:",
      "    runs-on: ubuntu-latest",
      "    steps: [{run: x}]",
      "  b:",
      "    needs: [a, nope]",
      "    runs-on: ubuntu-latest",
      "    steps: [{run: y}]",
      "",
    ].join("\n"),
  });
  try {
    const r = await run({ tools, subject: dir });
    assert.deepEqual(r.out.references, [{ path: ".github/workflows/ci.yml", from: "b", to: "a" }]);
    assert.equal(r.stats.needsRejected, 1);
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});

test("a file the parser rejects contributes nothing and is counted", { skip: !tools }, async () => {
  const dir = subject({
    ".github/workflows/bad.yml": "on: push\njobs:\n  a:\n    runs-on: x\n    stepz: []\n",
    ".github/workflows/ok.yml": "on: push\njobs:\n  a:\n    runs-on: x\n    steps: [{run: y}]\n",
  });
  try {
    const r = await run({ tools, subject: dir });
    assert.ok(!r.out.definitions.some((e) => e.path.endsWith("bad.yml")));
    assert.ok(r.out.definitions.some((e) => e.path.endsWith("ok.yml")));
    assert.equal(r.stats.workflowsRejected, 1);
    // -files-out names the files the parser accepted: the reading's -only scope
    assert.deepEqual(r.accepted, [".github/workflows/ok.yml"]);
    assert.deepEqual(r.rejected, [".github/workflows/bad.yml"]);
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});

test("composite action metadata comes from parseAction; local uses are cross-references", { skip: !tools }, async () => {
  const dir = subject({
    ".github/actions/setup/action.yml": [
      "name: Setup",
      "description: d",
      "inputs:",
      "  version:",
      "    description: v",
      "outputs:",
      "  dir:",
      "    description: o",
      "    value: ${{ steps.s.outputs.dir }}",
      "runs:",
      "  using: composite",
      "  steps:",
      "    - id: s",
      "      run: echo",
      "      shell: bash",
      "    - uses: ./.github/actions/other",
      "",
    ].join("\n"),
    ".github/actions/other/action.yaml": "name: Other\ndescription: o\nruns:\n  using: node20\n  main: index.js\n",
    ".github/workflows/ci.yml": [
      "on: push",
      "jobs:",
      "  a:",
      "    runs-on: x",
      "    steps:",
      "      - uses: ./.github/actions/setup",
      "  call:",
      "    uses: ./.github/workflows/reuse.yml",
      "",
    ].join("\n"),
    ".github/workflows/reuse.yml": "on: workflow_call\njobs:\n  r:\n    runs-on: x\n    steps: [{run: y}]\n",
  });
  try {
    const r = await run({ tools, subject: dir });
    const d = r.out.definitions;
    const a = ".github/actions/setup/action.yml";
    assert.ok(has(d, { path: a, name: "Setup", kind: "target", line: 1 }));
    assert.ok(has(d, { path: a, name: "version", kind: "variable_decl", line: 5 }));
    assert.ok(has(d, { path: a, name: "dir", kind: "output", line: 8 }));
    assert.ok(has(d, { path: a, name: "s", kind: "step", line: 13 }));
    assert.ok(has(d, { path: a, name: "./.github/actions/other", kind: "resource", line: 16 }));
    const x = r.xref.cross_references;
    assert.ok(has(x, { path: ".github/workflows/ci.yml", from: "a", to_path: a }), JSON.stringify(x));
    assert.ok(has(x, { path: ".github/workflows/ci.yml", from: "call", to_path: ".github/workflows/reuse.yml" }));
    assert.ok(has(x, { path: a, from: "Setup", to_path: ".github/actions/other/action.yaml" }));
    assert.equal(r.xref.tier, "C");
    // LB-F457: a subject with local-uses edges carries the written ceiling, one note
    assert.equal(r.xref.disclose.length, 1);
    assert.ok(r.xref.disclose[0].startsWith("L3 ceiling: "), r.xref.disclose[0]);
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});

test("a reusable workflow the converter cannot load is no cross-reference", { skip: !tools }, async () => {
  const dir = subject({
    ".github/workflows/ci.yml": "on: push\njobs:\n  call:\n    uses: ./.github/workflows/missing.yml\n",
  });
  try {
    const r = await run({ tools, subject: dir });
    assert.deepEqual(r.xref.cross_references, []);
    assert.equal(r.stats.localUsesUnresolved, 1);
    assert.equal(r.xref.disclose, undefined); // no edge, no ceiling to write
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});
