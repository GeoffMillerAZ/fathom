# opa-parse — tier-A Rego oracle (definitions, calls, imports)

`opa parse` is Rego's reference parser (design §1.3, tier A: the language's own parser, the one OPA evaluates policy
with). This program parses every `.rego` file of a subject on its own with `opa parse --format json --json-include
locations,-comments <file>` (Rego v1, OPA's default) and writes the neutral oracle file `indexbench lang-oracle
-tool-oracle` reads. Its own Go module (standard library only); bench-only, never a product dependency.

    cd calibration/oracles/opa-parse
    go run . -subject <checkout> -opa <opa> [-files <list of subject-relative paths>] > workspace/tmp/oracle/rego-<subject>.json

A file OPA rejects (v0 syntax, a deliberately broken example) is named on stderr and writes nothing.

What it writes, every line from OPA's own locations:

- `definitions`: a `package` per file (its path without `data.`, at the package clause); one definition per rule OPA
  parsed, a `function` when its head has arguments, else a `rule` (complete, partial set, partial object, default and
  test rules alike), named by the head's name or, for a ref head OPA gives no name (`a.b.c := 1`), by the head's static
  path joined with `.`. Several bodies sharing one name are several rules in OPA's AST (`allow if {…}` twice), each its
  own definition at its own line, matched one to one by the harness's nearest-line rule; an `else` chain belongs to the
  rule it follows and extends its range. A rule's last line is its first plus the lines of the source text OPA records
  for it.
- `calls`, attributed to the rule whose head, body or else chain holds them: every call OPA parsed whose operator the
  source spells as its own dotted name (`count(x)`, `object.get(o, k, d)`, `utils.f(x)`, `data.lib.f(x)`), named by its
  last segment. An infix operator (`==`, `:=`, `+`, `in`) is a call in OPA's AST but is not written as one and is left
  out. Also every reference rooted at `data` that is not a call's operator (`data.lib.roles.admins`), named by its last
  static segment. A bare name that refers to a rule of the same package and a reference through an import alias are not
  written: telling them from a local variable or an input path takes OPA's compiler, not its parser.
- `imports`: every `import data.x.y [as z]`, at its line, to each file of the subject that declares `package x.y`, or
  else declares `package x` and a rule named `y`; `import input…`, `import rego.v1` and `import future…` name no file.

First runs (2026-09-29, OPA 1.18.2), for LB-Q-rego's refusal (design §14.2) and its re-plan: StyraInc/regal v2.2.3
`bundle/`, 232 files, all parsed (1,516 definitions: 232 packages, 964 rules, 320 functions; 2,312 call pairs; 1,498
import targets); open-policy-agent/opa v1.18.2 (sparse to `**/*.rego`), 181 files, 138 parsed (43 rejected: v0-syntax
formatter test files and documentation examples written to fail), 996 definitions, 323 call pairs. No reading was taken
with it: the pinned grammar (FallenAngel97/tree-sitter-rego da2a1f63) leaves an ERROR node in 208 of regal's 232 files,
which OPA accepts, so the item was refused before a dialect was built.
