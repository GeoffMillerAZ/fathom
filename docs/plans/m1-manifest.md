# M1 — the manifest goes live (docs/vision/02 §3; instantiates ADR-016's revival)

schema/semantic/schema.cue stops being documentation: the runtime loads it,
the Go mirrors die, prompt identity becomes content-addressed via
agentic-kit v0.7.0 prompttemplate.TextHash, and the compile adopts
llmclient/structured so truncation is a typed budget signal. FLAG-DAY
RECOMPILE acknowledged: PromptVersion changes form, every claim recompiles
once — fold with the O8 serve-time precision prompt revision so the fleet
pays a single recompile, and never land this between a compile and its
measurement window.

## Load-bearing manifest

- [x] Embedded schema.cue is parsed at init; layer definitions (corpus globs, tier, budgets, serving caps) come from the manifest {id: M1-1, symbol: LoadLayer, package: internal/semantic}
- [x] The hand-synced Go mirrors are deleted: ObligationsInclude and the systemPrompt const no longer exist as authoritative sources {id: M1-2, symbol: LoadLayer, skip_test: true, skip: "deletion proven by grep acceptance below; loader behavior is M1-1's test"}
- [x] compile_layer and fathom compile accept any manifest-listed layer instead of hard-rejecting non-obligations {id: M1-3, symbol: cmdCompile, package: cmd/fathom}

## Content-addressed prompt identity

- [x] The obligations prompt lives as a prompttemplate.Template; PromptVersion derives from TextHash (obligations/hash8) so any prompt edit is automatically a new compile identity {id: M1-4, symbol: obligationsPromptVersion, package: internal/semantic}
- [x] The digest gate keys on the derived identity; a template-text edit forces recompile of every section (the v1-to-v2 bug class dies) {id: M1-5, symbol: allDigestsMatch, package: internal/semantic}

## Typed compile failures + enforced budgets

- [x] Section completion goes through structured.Complete: ErrTruncated and ErrMalformed drive the existing skip-section policy; errBadResponse hand-classification retires {id: M1-6, symbol: compileSection, package: internal/semantic}
- [x] Run-level budgets from the manifest are enforced: estimated cost above max_usd_per_run refuses to start; max_tokens_per_run hard-stops mid-run; aborts are resumable via the digest gate; refusals recorded in compile_runs {id: M1-7, symbol: enforceBudgets, package: internal/semantic}

## Tier widening (amends ADR-016 tier gating)

- [x] #Layer.tier admits verified for deterministic Model-empty compile paths; schema testdata updated; the amendment note lands in ADR.md {id: M1-8, symbol: LoadLayer, skip_test: true, skip: "schema+docs deliverable anchored to the loader that enforces it"}

## Acceptance

grep acceptance for M1-2: no definition of ObligationsInclude or systemPrompt
remains under internal/semantic (loader-owned values only) — verified.
task validate green — verified. **One fleet recompile executed with the
O8-revised prompt is NOT part of this slice**: this branch shipped the
mechanism only (no ANTHROPIC_API_KEY, no LLM calls, all tests mocked, per
the slice's own constraints). The next real `compile_layer` / `fathom
compile --dry-run=false` run against the fleet IS that flag-day recompile
(every existing claim's PromptVersion mismatches the new content-addressed
identity) — sequence it deliberately, ideally folded with O8, never
mid-measurement-window. Obligations serving unchanged in shape (6-rule cap,
zero-on-clean) — verified (internal/semantic/serve.go untouched).
