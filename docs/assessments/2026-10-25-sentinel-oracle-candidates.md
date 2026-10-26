# Sentinel oracle and parser candidates — 2026-10-25

**Source-contract assessment:** Sentinel remains unmeasured and unsupported.
The checked routes below do not supply an allowlisted open product parser. A
separate native evaluator could be an external oracle, but it cannot fill that
product requirement. No executed runtime output, benchmark result or accuracy
claim is included. The owner ruled that this assessment use public source
contracts only.

## Named candidates and concrete outcomes

| Candidate / route | Version / revision checked | Concrete limitation |
|---|---|---|
| Sentinel fmt | Runtime release index 0.41.0; public command docs 0.40.x | Public contract formats policy source to canonical text; it does not publish a reusable open parser or AST API. It might validate syntax externally, but cannot be linked as the product parser. No 0.41.0 runtime behavior inferred from older docs. |
| Sentinel apply / test | Same release index/docs scope | Public apply contract evaluates a policy, with JSON/trace and per-rule output. A potential native external oracle for what executes, not an open product parser or a declared full written-call AST inventory. No runtime results used here. |
| sentinel-sdk | 0.5.2 release README | SDK extends data imports via a plugin protocol/gRPC. It is not a Sentinel-language parser library. The SDK's open license does not make the runtime's internal parser available. |
| hashicorp/syntax | 956f4991da90, README and Sentinel TextMate route | Regex/scopes for highlighting. Explicitly TextMate grammars; no syntax-tree parser.c or reusable language AST. Highlighting is insufficient for the product parse/definition/call contract. |
| hashicorp/vscode-sentinel | package 0.3.0, df4f6940ba5d source | Features list syntax highlighting for .sentinel; editor packaging is not an open language parser. No reusable AST parser established by this interface. |
| ANTLR grammars-v4 Sentinel request | Issue2322, open on 2026-10-25, last update2021-09-19 | An unimplemented request is not a licensed grammar with real-corpus parse measurements. The issue is not proof that no grammar could exist elsewhere. |
| tree-sitter-sentinel repository route | GitHub repository search on2026-10-25, zero results for the named query | Bounded negative search only. No candidate grammar/version found to pin, license-check or measure. Not universal absence proof. |

**Public sources:** [formatter contract](https://developer.hashicorp.com/sentinel/docs/commands/fmt),
[evaluator/JSON contract](https://developer.hashicorp.com/sentinel/docs/commands/apply),
[release index](https://releases.hashicorp.com/sentinel/0.41.0/),
[SDK0.5.2](https://github.com/hashicorp/sentinel-sdk/blob/v0.5.2/README.md),
[highlighting source](https://github.com/hashicorp/syntax/tree/956f4991da90),
[editor source](https://github.com/hashicorp/vscode-sentinel/tree/df4f6940ba5d),
[ANTLR request](https://github.com/antlr/grammars-v4/issues/2322) and
[bounded repository query](https://github.com/search?q=tree-sitter-sentinel&type=repositories).

**Hypothesis / retry:** retain LB-Q-sentinel-rest for these checked candidates.
Reopen when an allowlisted grammar ships src/parser.c or an open reusable Sentinel
parser exists; verify license and measure original real policy files under the
5% parse-error bar before building the dialect. Then separately establish the
external oracle's license, contract and complete population. Formatter success
alone would not prove definitions/calls or a level. No parser was fabricated, no
corpus narrowed, no reading/generated output created, and no plan box changed.
