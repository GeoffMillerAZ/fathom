// Tests for svelte_oracle.mjs (node --test). The compiler half needs the
// tools directory that holds node_modules/svelte (README):
//
//	SVELTE_TOOLS=<tools> node --test calibration/oracles/svelte-compiler/
import { test } from "node:test";
import assert from "node:assert/strict";
import { createRequire } from "node:module";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { run } from "./svelte_oracle.mjs";

const tools = process.env.SVELTE_TOOLS;

test("the compiler's components, declarations and script calls", { skip: !tools }, () => {
  const compiler = createRequire(path.join(path.resolve(tools), "package.json"))("svelte/compiler");
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "svelte-oracle-"));
  fs.writeFileSync(path.join(root, "A.svelte"), "<script lang=\"ts\">\n  import B from './B.svelte';\n  let { x } = $props();\n  const f = () => go();\n</script>\n\n<B />\n<Card.Root />\n<svelte:self />\n<div></div>\n");
  fs.writeFileSync(path.join(root, "B.svelte"), "<p>b</p>\n");
  const out = run(compiler, "test", root);
  assert.deepEqual(out.calls.map((c) => c.to), ["B", "Card.Root"]);
  assert.deepEqual(out.definitions.map((d) => [d.name, d.line, d.kind]), [["B", 2, "import"], ["x", 3, "variable"], ["f", 4, "function"]]);
  assert.deepEqual(out.call_exclusions.map((c) => c.to), ["$props", "go"]);
  assert.deepEqual(out.imports, [{ path: "A.svelte", line: 2, to_path: "B.svelte", module: "./B.svelte" }]);
});
