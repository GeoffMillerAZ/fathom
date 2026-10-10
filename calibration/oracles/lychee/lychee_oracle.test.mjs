// Tests for lychee_oracle.mjs (node --test): node --test calibration/oracles/lychee/
import { test } from "node:test";
import assert from "node:assert/strict";
import { mkdtempSync, mkdirSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { pathToFileURL } from "node:url";
import { convert } from "./lychee_oracle.mjs";

test("a resolved link names a subject file, its query, fragment and directory index settled", () => {
  const dir = mkdtempSync(path.join(process.env.TMPDIR || tmpdir(), "lychee-oracle-"));
  for (const f of ["a.html", "b.html", "sub/index.html", "s.css"]) {
    mkdirSync(path.join(dir, path.dirname(f)), { recursive: true });
    writeFileSync(path.join(dir, f), "x");
  }
  mkdirSync(path.join(dir, "empty"));
  const url = (p) => pathToFileURL(path.join(dir, p)).href;
  const report = {
    success_map: {
      "a.html": [
        { url: url("b.html") + "#x", span: { line: 3 } },
        { url: url("s.css") + "?v=2", span: { line: 2 } },
        { url: url("sub"), span: { line: 4 } },
        { url: url("empty"), span: { line: 5 } },
        { url: pathToFileURL(path.join(dir, "..", "outside.html")).href, span: { line: 6 } },
      ],
    },
    error_map: { "a.html": [{ url: url("missing.html") }] },
    excluded_map: { "a.html": [{ url: "https://example.com/" }] },
  };
  const out = convert(dir, report, "0.24.2");
  assert.deepEqual(out.imports.map((e) => [e.path, e.line, e.to_path]), [
    ["a.html", 2, "s.css"], ["a.html", 3, "b.html"], ["a.html", 4, "sub/index.html"],
  ]);
  assert.match(out.disclose[0], /left out: 1 directory, 1 outside/);
  assert.match(out.disclose[0], /1 links it could not resolve and 1 it excluded/);
});
