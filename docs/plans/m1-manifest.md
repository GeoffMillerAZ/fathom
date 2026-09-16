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

- [x] Embedded schema.cue is parsed at init; layer definitions (corpus globs, tier, budgets, serving caps) come from the manifest {id: M1-1, symbol: LoadLayer, package: internal/semantic, reachable_from: TestBuild_PropagatesListError}
- [x] The hand-synced Go mirrors are deleted: ObligationsInclude and the systemPrompt const no longer exist as authoritative sources {id: M1-2, symbol: LoadLayer, skip_test: true, skip: "deletion proven by grep acceptance below; loader behavior is M1-1's test"}
- [x] compile_layer and fathom compile accept any manifest-listed layer instead of hard-rejecting non-obligations {id: M1-3, symbol: cmdCompile, package: cmd/fathom}

## Content-addressed prompt identity

- [x] The obligations prompt lives as a prompttemplate.Template; PromptVersion derives from TextHash (obligations/hash8) so any prompt edit is automatically a new compile identity {id: M1-4, symbol: obligationsPromptVersion, package: internal/semantic, reachable_from: TestCompile_HappyPath}
- [x] The digest gate keys on the derived identity; a template-text edit forces recompile of every section (the v1-to-v2 bug class dies) {id: M1-5, symbol: allDigestsMatch, package: internal/semantic, reachable_from: TestBuild_PropagatesListError}

## Typed compile failures + enforced budgets

- [x] Section completion goes through structured.Complete: ErrTruncated and ErrMalformed drive the existing skip-section policy; errBadResponse hand-classification retires {id: M1-6, symbol: compileSection, package: internal/semantic, reachable_from: TestBuild_PropagatesListError}
- [x] Run-level budgets from the manifest are enforced: estimated cost above max_usd_per_run refuses to start; max_tokens_per_run hard-stops mid-run; aborts are resumable via the digest gate; refusals recorded in compile_runs {id: M1-7, symbol: enforceBudgets, package: internal/semantic, reachable_from: TestBuild_PropagatesListError}

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

## Follow-ups from the adversarial review (deferred, typed)

- [-] Pre-flight token gate livelock: a corpus whose estimate exceeds max_tokens_per_run refuses forever instead of start-and-stop {id: M1-F3, disposition: deferred, reason: "latent — the 3-file obligations corpus is far under budget", trigger: "first layer whose corpus estimate approaches max_tokens_per_run"}
- [-] USD gate prices input tokens only; ledger and gate disagree by construction {id: M1-F4, disposition: deferred, reason: "token gate coincidentally backstops worst-case spend near the budget today", trigger: "output-heavy layer or a real over-budget ledger row"}
- [-] PromptVersion hashes the system prompt only — scaffolding and schema changes are identity-invisible {id: M1-F6, disposition: deferred, reason: "sequencing is the protection this release; F1 landed before the flag day", trigger: "next effective-prompt change that is not a Text edit"}
- [-] Layer name validated then discarded — a second manifest layer would silently compile obligations {id: M1-F7, disposition: deferred, reason: "impossible to misfire with a one-layer manifest", trigger: "layer 2 lands in schema.cue"}
- [-] Serving caps decoded from the manifest but not consumed; serve.go constants still hand-mirrored {id: M1-F8, disposition: deferred, reason: "values coincide and serving-unchanged was an acceptance criterion", trigger: "M2 serving-template vocabulary"}
