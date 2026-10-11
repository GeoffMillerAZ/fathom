// Tests for vue_sfc_oracle.mjs (node --test). The compiler half needs the
// tools directory that holds node_modules/@vue/compiler-sfc (README):
//
//	VUE_TOOLS=<tools> node --test calibration/oracles/vue-compiler-sfc/
import { test } from "node:test";
import assert from "node:assert/strict";
import { createRequire } from "node:module";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { componentName, specClass, resolveRelative, run } from "./vue_sfc_oracle.mjs";

const tools = process.env.VUE_TOOLS;

test("a component tag is named as resolveComponent resolves it", () => {
  assert.equal(componentName("el-input"), "ElInput");
  assert.equal(componentName("keep-alive"), "KeepAlive");
  assert.equal(componentName("ElButton"), "ElButton");
  assert.equal(componentName("Foo.Bar"), "Foo.Bar");
});

test("a specifier is relative, an alias or a package", () => {
  assert.equal(specClass("./a.vue"), "relative");
  assert.equal(specClass("../b"), "relative");
  assert.equal(specClass("@/components/X.vue"), "alias");
  assert.equal(specClass("$lib/x"), "alias");
  assert.equal(specClass("@vue/shared"), "package");
  assert.equal(specClass("vue"), "package");
});

test("Vite's rule resolves a relative specifier", () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "vue-oracle-"));
  fs.mkdirSync(path.join(root, "src", "dir"), { recursive: true });
  for (const f of ["src/A.vue", "src/b.ts", "src/dir/index.ts", "src/c.js"]) fs.writeFileSync(path.join(root, f), "");
  assert.equal(resolveRelative(root, "src/X.vue", "./A.vue", false), "src/A.vue");
  assert.equal(resolveRelative(root, "src/X.vue", "./A", false), "");
  assert.equal(resolveRelative(root, "src/X.vue", "./b", true), "src/b.ts");
  assert.equal(resolveRelative(root, "src/X.vue", "./b.js", true), "src/b.ts");
  assert.equal(resolveRelative(root, "src/X.vue", "./c.js", true), "src/c.js");
  assert.equal(resolveRelative(root, "src/X.vue", "./dir", true), "src/dir/index.ts");
  assert.equal(resolveRelative(root, "src/X.vue", "../../out", true), "");
});

test("the compiler's components, declarations and script calls", { skip: !tools }, () => {
  const req = createRequire(path.join(path.resolve(tools), "package.json"));
  const sfc = req("@vue/compiler-sfc");
  const babel = createRequire(req.resolve("@vue/compiler-sfc"))("@babel/parser");
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "vue-oracle-"));
  fs.writeFileSync(path.join(root, "A.vue"), "<script setup lang=\"ts\">\nimport B from './B.vue'\nconst n = ref(0)\nfunction go() { emit('x') }\n</script>\n<template><div><B /><el-input /><component :is=\"x\" /><slot /></div></template>\n");
  fs.writeFileSync(path.join(root, "B.vue"), "<template><i /></template>\n");
  const out = run(sfc, babel, "test", root);
  assert.deepEqual(out.calls.map((c) => c.to), ["B", "ElInput"]);
  assert.deepEqual(out.definitions.map((d) => [d.name, d.line, d.kind]), [["B", 2, "import"], ["n", 3, "constant"], ["go", 4, "function"]]);
  assert.deepEqual(out.call_exclusions.map((c) => c.to), ["emit", "ref"]);
  assert.deepEqual(out.imports, [{ path: "A.vue", line: 2, to_path: "B.vue", module: "./B.vue" }]);
});
