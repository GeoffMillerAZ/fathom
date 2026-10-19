import test from "node:test";
import assert from "node:assert/strict";
import { convert, slugs, target } from "./md_oracle.mjs";

const ctags = [
  { _type: "tag", name: "Top", path: "a.md", line: 1, kind: "chapter" },
  { _type: "tag", name: "Install Steps", path: "a.md", line: 5, kind: "section" },
  { _type: "tag", name: "Install Steps", path: "a.md", line: 9, kind: "section" },
  { _type: "tag", name: "Only", path: "b.md", line: 1, kind: "chapter" },
].map((t) => JSON.stringify(t)).join("\n");

test("target keeps only another Markdown file of the subject", () => {
  assert.equal(target("/s", "file:///s/dir/b.md#frag").to, "dir/b.md");
  assert.equal(target("/s", "file:///s/img/x.png").why, "not markdown");
  assert.equal(target("/s", "file:///elsewhere/b.md").why, "outside");
  assert.equal(target("/s", "https://x/b.md").why, "remote");
});

test("slugs follow GitHub: repeats suffixed, an explicit id wins", () => {
  const m = slugs([{ name: "Install Steps" }, { name: "Install Steps" }, { name: "A & B" }, { name: "P {#pid}" }, { name: "Q {/* #qid */}" }]);
  assert.equal(m.get("install-steps"), "Install Steps");
  assert.ok(m.has("install-steps-1"));
  assert.ok(m.has("a--b"));
  assert.ok(m.has("pid"));
});

test("convert maps a link to its section, an in-file anchor to a reference", () => {
  const report = {
    success_map: {
      "a.md": [
        { url: "file:///s/b.md#x", span: { line: 6 } },
        { url: "file:///s/a.md#install-steps-1", span: { line: 2 } },
        { url: "file:///s/img/x.png", span: { line: 3 } },
      ],
    },
    error_map: {},
  };
  const o = convert("/s", report, ctags, { lychee: "t", ctags: "t" });
  assert.deepEqual(o.cross_references, [{ path: "a.md", from: "Install Steps", to_path: "b.md" }]);
  assert.deepEqual(o.references, [{ path: "a.md", from: "Top", to: "Install Steps" }]);
});
