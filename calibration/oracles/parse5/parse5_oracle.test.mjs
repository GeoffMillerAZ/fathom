// Tests for parse5_oracle.mjs (node --test). The parse half needs the tools
// directory that holds node_modules/parse5 (README):
//
//	PARSE5_TOOLS=<tools> node --test calibration/oracles/parse5/
import { test } from "node:test";
import assert from "node:assert/strict";
import { createRequire } from "node:module";
import path from "node:path";
import { idTokens, readFile } from "./parse5_oracle.mjs";

const tools = process.env.PARSE5_TOOLS;

test("an attribute is an ID reference only on the elements the index names", () => {
  assert.deepEqual(idTokens("label", "for", "email"), ["email"]);
  assert.deepEqual(idTokens("output", "for", "a \tb\n"), ["a", "b"]);
  assert.equal(idTokens("span", "for", "email"), null);
  assert.deepEqual(idTokens("input", "form", "f"), ["f"]);
  assert.equal(idTokens("div", "form", "f"), null);
  assert.deepEqual(idTokens("td", "headers", "h1 h2"), ["h1", "h2"]);
  assert.deepEqual(idTokens("button", "commandfor", "menu"), ["menu"]);
  assert.deepEqual(idTokens("div", "aria-labelledby", "x  y"), ["x", "y"]);
  assert.deepEqual(idTokens("div", "itemref", "x y"), ["x", "y"]);
  assert.deepEqual(idTokens("a", "href", "#top"), ["top"]);
  assert.equal(idTokens("a", "href", "#"), null);
  assert.equal(idTokens("a", "href", "page.html#top"), null);
  assert.equal(idTokens("link", "href", "#top"), null);
});

test("definitions carry the id's line and an end only where the parser attests one", { skip: !tools }, () => {
  const parse5 = createRequire(path.join(path.resolve(tools), "package.json"))("parse5");
  const r = readFile(parse5, "a.html", "<!DOCTYPE html>\n<div\n  id=\"w\">\n<p id=p>x\n<input id=i>\n</div>\n");
  assert.deepEqual(r.defs, [
    { path: "a.html", name: "w", line: 3, end: 6, kind: "id" },
    { path: "a.html", name: "p", line: 4, end: 0, kind: "id" },
    { path: "a.html", name: "i", line: 5, end: 5, kind: "id" },
  ]);
});
