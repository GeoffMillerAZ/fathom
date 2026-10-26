# Sentinel parser feasibility and oracle candidates — 2026-10-25

**Correction (OL-R8):** the earlier “no open parser” ceiling was false.
`glennsarti/sentinel-parser` is Apache-2.0 and supplies a lexer, parser, positioned
AST and walker. The open-parser trigger has fired. It is pinned and measured below;
this assessment no longer treats an absent tree-sitter grammar as absence of a parser.

**Measured, open parser only:** latest main `3a328c0dcfb9` reports files in error
in **30/386** original terraform-sentinel-policies files and **0/9** original
policy-library-gcp-databases-terraform files: **30/395 = 7.59%** pooled, above the
5% full-discovery admission bar. There are zero panics. The split outside mock paths
is **2/87 + 0/3 = 2/90 (2.22%)**; mock paths account for the other 28 failures.
This narrower split passes 5%, so this is a viable policy-body parser route,
not proof that Sentinel cannot be read. No files were excluded or repaired to pass.
The [aggregate receipts](data/sentinel-open-2026-10-25/) contain counts, source
manifest digests and probe identity; the [committed recipe](../../calibration/oracles/sentinel-open/README.md)
replays all original files. A parser's own diagnostics are feasibility evidence,
not independent accuracy or a support reading. No level is claimed.

## Named candidates and concrete outcomes

| Candidate / route | Version / revision checked | Concrete outcome / limit |
|---|---|---|
| glennsarti/sentinel-parser | v0.0.3 `4fd6abca97e7`; main `3a328c0dcfb949ba8e9247b577eb1cac14134a1e` | Apache-2.0, Go. Main measured above; tag-to-main comparison changes only CI and dependency manifests, not parser code. Open product-parser condition is met. Full-discovery error rate is over 5%; non-mock rate is under 5%. It cannot independently grade a product that uses the same parser. |
| clarkmcc/warden-rs | `4e6d612d45cd0b94df69635892c1c5863dfe2b8d` | Toy Rust parser/interpreter; immutable tree has no LICENSE/COPYING file. Rejected for absent reuse licence, not measured or incorporated. |
| Public Sentinel language specification | v0.40.x page dated 2025-09-17 | EBNF can guide a separate reader, but is not an executable oracle. A second dependency-free native reader would require ADR-048's second-language admission review and an independent publishable oracle. |
| Sentinel fmt | Runtime release index 0.41.0; public docs 0.40.x | Formatter/source validation contract, not an open parser API. EULA restricts publishing related benchmark results; OL-R6 forbids a runtime-graded reading in this public repository. No runtime results published. |
| Sentinel apply / test / trace | Same release index/docs scope | Owner evaluation and JSON/trace are a potential technical oracle for executed rules/calls. The same EULA/OL-R6 restriction blocks publication. No runtime execution or trace result is used in this correction. |
| hashicorp/consul Sentinel integration | `a4041a737d744c4f96eea71fa114008ea754457d` | Public `Evaluator` is an interface; non-enterprise `New` returns nil. It is not a usable open evaluator or alternate runtime oracle. |
| sentinel-sdk | 0.5.2 release README | Import/plugin protocol, not a Sentinel-language parser or evaluator. SDK licensing does not license the runtime's implementation. |
| glennsarti/sentinel-lint | `ea798f0b275dce3f85ce1ef52e094516cee2f057` | Open linter imports sentinel-parser v0.0.3. Shares its parser, so not an independent second truth source for that parser. |
| glennsarti/sentinel-utils language server | `2b3039f920d4e1ae5934bc4fb8e6f61f85a35457` | Apache-2.0; includes `language-server`. Initialize/dispatch advertise document sync and linting, not definitions/references. Imports the same sentinel-parser v0.0.3; no independent resolved-call oracle. |
| hashicorp/syntax | `956f4991da90` | TextMate regex/scopes, not a reusable AST or resolved-call oracle. |
| hashicorp/vscode-sentinel | package 0.3.0, `df4f6940ba5d` | Syntax-highlighting packaging; does not supply an independent language AST. |
| ANTLR grammars-v4 Sentinel request | Issue 2322, open; last update 2021-09-19 | Unimplemented grammar request; not a parser to measure. Its absence claim from 2021 is obsolete. |
| tree-sitter-sentinel repository query | 2026-10-25, zero literal-query hits | Bounded negative only; superseded as a general parser argument by the open Go parser above. |

## Search sources (X-R4)

All searches checked 2026-10-25; negatives describe these sources, not universal absence.

| Required category | Source / query actually checked | Result |
|---|---|---|
| Owner toolchain | [command docs](https://developer.hashicorp.com/sentinel/docs/commands), [spec](https://developer.hashicorp.com/sentinel/docs/language/spec), [SDK v0.5.2](https://github.com/hashicorp/sentinel-sdk/tree/v0.5.2), [Consul integration](https://github.com/hashicorp/consul/tree/a4041a737d744c4f96eea71fa114008ea754457d/sentinel) | Formatter, evaluator, tracing and import protocol considered separately. Runtime results cannot be published under OL-R6; public integration is a stub. |
| SCIP / LSIF | [Sourcegraph supported-indexer table](https://sourcegraph.com/docs/code-navigation/precise-code-navigation), [SCIP ecosystem list](https://scip-code.org/), [full archived LSIF.dev table](https://lsif.dev/), [GitHub `scip-sentinel`](https://github.com/search?q=scip-sentinel&type=repositories) | No Sentinel entry in the tables; literal GitHub repository query returns zero. The scip README's ten examples are non-exhaustive, not a registry. |
| Language servers | [GitHub `sentinel language-server`](https://github.com/search?q=sentinel+language-server&type=repositories); editor and sentinel-utils source | Literal query's one hit, maru-arch/ast-sentinel, describes a Go CFG analyzer rather than HashiCorp policies. The relevant sentinel-utils LSP was found via the broader parser-author/tool search and checked by its initialize/dispatch/go.mod contracts. |
| Package registries / parsers | [Go package search `sentinel parser`](https://pkg.go.dev/search?q=sentinel+parser), [npm query `hashicorp sentinel parser`](https://www.npmjs.com/search?q=hashicorp%20sentinel%20parser), [crates query `sentinel`](https://crates.io/search?q=sentinel), [GitHub `sentinel hashicorp parser`](https://github.com/search?q=sentinel+hashicorp+parser&type=repositories) | Go search finds the open parser; npm's first 20 results are unrelated generic/Redis parsers; crates' first 100 yield no HashiCorp parser candidate. GitHub returns glennsarti/sentinel-parser and warden-rs, both named above. |
| Runtime call / import tracing | [apply JSON/trace contract](https://developer.hashicorp.com/sentinel/docs/commands/apply), [test contract](https://developer.hashicorp.com/sentinel/docs/commands/test) | Considered as native executed-population evidence, not dismissed for lacking a full written-call index. Publication is barred by the already-decided EULA/OL-R6 condition, not by an invented coverage standard. |

[Parser source](https://github.com/glennsarti/sentinel-parser/tree/3a328c0dcfb949ba8e9247b577eb1cac14134a1e),
[tag comparison](https://github.com/glennsarti/sentinel-parser/compare/v0.0.3...3a328c0dcfb949ba8e9247b577eb1cac14134a1e),
[warden tree](https://github.com/clarkmcc/warden-rs/tree/4e6d612d45cd0b94df69635892c1c5863dfe2b8d),
[linter](https://github.com/glennsarti/sentinel-lint/tree/ea798f0b275dce3f85ce1ef52e094516cee2f057),
[LSP implementation](https://github.com/glennsarti/sentinel-utils/tree/2b3039f920d4e1ae5934bc4fb8e6f61f85a35457/lib/languageserver/internal/langserver),
[highlighting](https://github.com/hashicorp/syntax/tree/956f4991da90),
[editor](https://github.com/hashicorp/vscode-sentinel/tree/df4f6940ba5d),
[ANTLR request](https://github.com/antlr/grammars-v4/issues/2322).

## Current disposition and two independent conditions

The old parser-absence ceiling is withdrawn. The open parser is available; its
present full-discovery result needs repair or an explicitly governed scope decision,
with the passing policy-body split disclosed above. No product dialect was built
or level measured in this correction. Under OL-R6, **no EULA-runtime-graded reading
may be committed**. A publishable oracle therefore independently needs a licence
change, an owner-approved non-runtime route, or a second independent licensed open
parser for tier-C cross-checking under P-T1. Reusing the product parser as its own
truth would not satisfy independence. Without that second condition the honest
implementation target is **dialect built, unmeasured**, never a claimed level.
No runtime probe/output, subject content, file list, canonical corpus addition,
generated surface, extractor change or checkbox flip is part of this correction.

Correcting the queue metadata changes only the generated Sentinel source/licence
row. `TestLanguagesTrackerIsGenerated` and `TestCmdLangTracker` therefore report
that expected drift until controller regeneration; the generated file is untouched.
The open-probe and focused plan/queue race checks, ADR-011, release-note and
merge-debris checks pass.
