# parse5 — HTML's tier-A definitions and W1 ID references

`parse5_oracle.mjs` reads each `.html`/`.htm` file of a subject with
[parse5](https://github.com/inikulin/parse5) 8.0.1 (MIT; its one dependency,
`entities` 8.1.0, BSD-2-Clause), the WHATWG HTML parsing algorithm in
JavaScript, with `sourceCodeLocationInfo`, and writes `indexbench lang-oracle
-tool-oracle`'s neutral JSON (design §1.8, ruling W1; wave lf5, item html):

- `definitions`: every element carrying a non-empty `id`, template contents
  included, kind `id` (the html manifest's `oracle_kinds.tool` maps it to
  `label`); `line` is the id attribute's line, `end` the end tag's last line
  when one is written, a void element's start tag's last line, and `0` (no
  end, not scored by `range_agreement`) when the parser implied the end.
- `calls`: W1's same-document ID references as `(path, "<module>", id)`, one
  per distinct pair: the attributes the HTML standard's attribute index types
  as an ID or a set of IDs, on the elements it names, read at whatwg/html
  `877d6146487f15b15bc8dd0e3c625063a37b456a` (`commandfor`, `for`, `form`,
  `headers`, `itemref`, `list`, `popovertarget`), W1's ARIA ID references on
  any element (`aria-labelledby`, `aria-describedby`, `aria-controls`,
  `aria-owns`, `aria-activedescendant`) and `href="#id"` on `a` and `area`.
  A set is split on ASCII whitespace. The caller is the document's module
  (K19): an ID is resolved against the element's tree, never its ancestors.
- `call_exclusions`: every id two or more elements of a file carry (W1: its
  references name no one element), taken out of both sides by the harness.
- `disclose`: the counts the reading names (references, unmatched references,
  duplicate ids, percent-escaped fragments).

Tier A: the HTML standard's own parser decides every element, attribute and
location. Element names are compared whatever their namespace, as the dialect
reads tags by name.

```sh
npm --prefix <tools> install parse5@8.0.1
node calibration/oracles/parse5/parse5_oracle.mjs -tools <tools> -subject <dir> -out <subject>.parse5.json
PARSE5_TOOLS=<tools> node --test calibration/oracles/parse5/
```

Run in the measuring harness only, never by fathom.
