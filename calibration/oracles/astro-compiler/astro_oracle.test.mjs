// Tests for astro_oracle.mjs (node --test). The parse half needs the tools directory that holds
// node_modules/@astrojs/compiler and node_modules/typescript (README):
//
//	ASTRO_TOOLS=<tools> node --test calibration/oracles/astro-compiler/
import { test } from "node:test";
import assert from "node:assert/strict";
import { createRequire } from "node:module";
import path from "node:path";
import { isComponentName, scriptIsJS, specKind, resolveRelative, readFile, MODULE } from "./astro_oracle.mjs";

const tools = process.env.ASTRO_TOOLS;

test("a component is a tag that starts A to Z or holds a dot, Fragment excepted", () => {
  for (const n of ["Card", "UI.Card", "foo.bar", "Astro.self", "Foo-Bar", "X"]) assert.equal(isComponentName(n), true, n);
  for (const n of ["div", "my-el", "Fragment", "slot", "set:html"]) assert.equal(isComponentName(n), false, n);
});

test("a script is JavaScript with no type or a JavaScript type", () => {
  assert.equal(scriptIsJS([]), true);
  assert.equal(scriptIsJS([{ name: "is:inline", kind: "empty", value: "" }]), true);
  assert.equal(scriptIsJS([{ name: "type", kind: "quoted", value: "module" }]), true);
  assert.equal(scriptIsJS([{ name: "type", kind: "quoted", value: "application/ld+json" }]), false);
  assert.equal(scriptIsJS([{ name: "type", kind: "quoted", value: "text/partytown" }]), false);
});

test("a spec is relative, an alias or a package", () => {
  assert.equal(specKind("./Card.astro"), "relative");
  assert.equal(specKind("../lib/x"), "relative");
  assert.equal(specKind("~/components/Logo.astro"), "alias");
  assert.equal(specKind("@/layouts/Base.astro"), "alias");
  assert.equal(specKind("astro:content"), "package");
  assert.equal(specKind("@astrojs/starlight/components"), "package");
});

test("a relative spec resolves as Vite resolves it", () => {
  const files = new Set(["src/components/Card.astro", "src/lib/format.ts", "src/ui/index.ts"]);
  const has = (p) => files.has(p);
  assert.equal(resolveRelative("", "src/pages/index.astro", "../components/Card.astro", has), "src/components/Card.astro");
  assert.equal(resolveRelative("", "src/pages/index.astro", "../lib/format", has), "src/lib/format.ts");
  assert.equal(resolveRelative("", "src/pages/index.astro", "../ui", has), "src/ui/index.ts");
  assert.equal(resolveRelative("", "src/pages/index.astro", "../components/Card", has), "");
  assert.equal(resolveRelative("", "index.astro", "../outside.astro", has), "");
});

test("definitions, calls and imports of one component, at host lines", { skip: !tools }, async () => {
  const prefix = path.resolve(tools);
  const ts = createRequire(path.join(prefix, "package.json"))("typescript");
  const compiler = await import(path.join(prefix, "node_modules", "@astrojs", "compiler", "dist", "node", "index.js"));
  const text = [
    "---",
    "import Card from './Card.astro';",
    "const { title, size = 2 } = Astro.props;",
    "const posts = await getCollection('blog');",
    "function label(n: number) {",
    "  return fmt(n);",
    "}",
    "track();",
    "---",
    "<Card>{posts.map((p) => <UI.Item p={p} />)}</Card>",
    "<Fragment><my-el /></Fragment>",
    "<script type=\"application/ld+json\">{}</script>",
    "<script>",
    "  boot();",
    "</script>",
    "",
  ].join("\n");
  const r = await readFile(compiler, ts, "/nonexistent", "a.astro", text);
  assert.deepEqual(r.defs, [
    { path: "a.astro", name: "title", line: 3, end: 3, kind: "constant" },
    { path: "a.astro", name: "size", line: 3, end: 3, kind: "constant" },
    { path: "a.astro", name: "posts", line: 4, end: 4, kind: "constant" },
    { path: "a.astro", name: "label", line: 5, end: 7, kind: "function" },
  ]);
  const pairs = r.calls.map((c) => `${c.from}>${c.to}`).sort();
  assert.deepEqual(pairs, [`${MODULE}>Card`, `${MODULE}>UI.Item`, `${MODULE}>boot`, `${MODULE}>track`, "label>fmt", "posts>getCollection"].sort());
  assert.deepEqual(r.imports, [{ path: "a.astro", line: 2, to_path: "", module: "./Card.astro" }]);
});
