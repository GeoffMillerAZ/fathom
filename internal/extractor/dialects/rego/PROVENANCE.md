# rego — provenance and divergences

LB-Q-rego, design `docs/design/language-breadth.md` §13 (engine `native`, grammar_source
`native:pure-go`), ADR-048's wave-pc amendment. `internal/extractor/regoextract.go` is a reader
written for this dialect from the Rego language reference (openpolicyagent.org/docs/policy-language)
and the shapes OPA's own parser reports for the calibration subjects; no OPA source was copied.

## Grammar and parser considered and not used

- FallenAngel97/tree-sitter-rego (MIT, HEAD `da2a1f63` on 2026-10-01, unchanged since the refusal of
  2026-09-29): leaves an ERROR node in 265 of the 358 OPA-accepted calibration files (LB-Q-rego-rest).
- OPA's parser, `github.com/open-policy-agent/opa/v1/ast` at v1.18.2 (Apache-2.0): parses every file
  the oracle accepts by construction, but adds 6,450,288 bytes to `cmd/fathom` (102,371,538 to
  108,821,826 at eb900e7a), raises `golang.org/x/net` and `google.golang.org/protobuf`, and is the
  tier-A oracle's own parser, so a reading of it would prove the mapping and not the parse.

## What it models

- **The package** (`RegoPackage`, canonical `package`): its path without a leading `data`, string
  segments joined with dots (`package edge["cases"].v1` is `edge.cases.v1`), at the clause's line.
  Every symbol of the file carries it as its `Package`.
- **A rule** (`RegoRule`, `rule`) per head: complete, partial set and object, multi-value, default and
  test rules alike, named as OPA names the head (the variable; `p` for `p[k] := v` and `p.q := 1`;
  `a.b.c` for `a.b.c := 1`, `items.x` for `items[x] if …`), from the head's line to its last token
  (an `else` chain extends its rule). Several bodies of one name are several rules on their lines.
- **A function** (`RegoFunction`, `function`): a head with arguments, signed `name(args)`.

## References

- **L2 (calls)**: a call by written name (`count(x)`, `object.get(…)`, `util.f(x)`, `data.lib.f(x)`),
  stored as written, and a reference rooted at `data` that is not a call's operator, stored up to its
  last static segment (`data.lib.roles[x]` is `data.lib.roles`), attributed to the rule that holds
  it. Infix operators, `set()` and bare names are not calls.
- **L3 (imports)**: `import data.x.y [as z]` is a whole-module import bound to `z` or `y`, resolved by
  the `rego-package` strategy (`internal/fusion/resolve_regopkg.go`) to the files declaring package
  `x.y`, else package `x` with a rule `y`; `import input…`, `rego.v1` and `future…` bind nothing. A
  call or a reference through the bound name (`util.f(x)`, `r.users`) is a file-resolved edge to the
  rule of the package's file that defines it; each file re-exports its own package, so a package
  split across files resolves its members wherever they are defined.

## Not modelled

- A bare name that refers to a rule of the same package, and a reference through an import alias
  that is not a call, as L2 calls (the oracle needs OPA's compiler to tell them from locals).
- Rego v0 syntax (`deny[msg] { … }` without `if` or `contains`) is read by the same rules but not
  measured: OPA 1.18.2 rejects it, so it has no oracle.
