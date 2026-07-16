# O8 precision + spec-line injection — the serve-time trust slice

O8 (dogfood): v2 obligations inject code-scoped rules ("use the race
detector") on markdown edits and fired for files outside any indexed repo —
trust-fatal noise at serve time. And the killer-feature list (docs/vision/04
§4 item 3) wants the spec's binding line injected when an edited file cites
a spec ID. Both land here; the flag-day recompile (operator, keyed shell)
carries the new prompt.

## Compile-side precision (the O8 work order)

- [x] Prompt v3: rules must scope to the file class they bind (code rules to source globs, docs rules to markdown globs); bare catch-all scopes are forbidden by instruction {id: OP-1, symbol: obligationsPromptVersion, package: internal/semantic, skip_test: true, skip: "prompt text change; identity rotation is the test — TextHash-derived PromptVersion changes"}
- [x] Over-broad-scope guard in compile verification: a draft whose scope matches more than half of the indexed files is rejected and counted, exactly like a failed citation {id: OP-2, symbol: verifyDraft, package: internal/semantic}
- [x] Edit-hook refuses files outside the resolved service root — the out-of-repo injection O8 observed goes silent {id: OP-3, symbol: cmdEditHook, package: cmd/fathom}

## Spec-line injection (knowledge tied to code, in the working channel)

- [x] When the edited file cites bracketed spec IDs, the edit-hook appends a traceability line per ID (where the spec is defined, via the index) beneath the obligations payload — same ≤150-token discipline, silent on none {id: OP-4, symbol: cmdEditHook, package: cmd/fathom}

## Plan authoring at zero marginal effort

- [x] A project skill scaffolds v2-grammar plans (stable ids, contracts, typed dispositions) so plans are born gate-ready {id: OP-5, symbol: cmdEditHook, skip_test: true, skip: "skill markdown deliverable; anchored to the channel that consumes the grammar"}

## Deferred

- [-] The flag-day fleet recompile itself {id: OP-6, disposition: deferred, reason: "operator-side; needs a keyed shell", trigger: "Geoff runs compile --dry-run=false per fleet repo on a binary containing this slice"}
