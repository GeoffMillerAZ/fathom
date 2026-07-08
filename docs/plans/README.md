# docs/plans — contract-bearing plan files

Plan files here are Markdown checklists where `[x]` claims are gated at
pre-push and CI. The grammar is specified in
`docs/design/coding-agent-excellence.md` §4.2.

## Checkbox semantics

| Checkbox | Meaning | Gated? |
|----------|---------|--------|
| `[ ]`    | Open — work not yet claimed done | Never |
| `[x]`    | Claimed done — MUST verify against the index | Always (--strict in CI) |
| `[-]`    | Skipped or abandoned — exempt | Never |

## Line format

```
- [ ] Title text [SPEC-ID] {symbol: Sym, reachable_from: Entry, min_tier: verified}
```

(The example is deliberately unchecked: the parser is line-based and does not
skip code fences, so a `[x]` example in any `docs/plans/*.md` would gate. The
hooks also exclude this README — plans claim, READMEs describe.)

- **Title**: human description of the work.
- **Spec IDs**: bracketed references (`[US-042]`) linked to spec documents.
- **Annotation block** `{…}`: acceptance contract with optional keys `symbol`,
  `reachable_from`, `min_tier`, `skip_test`, `allow_contradiction`.

A line with no annotation and no spec IDs is `Unknown` — reported honestly,
never a false pass. Under `--strict` (CI), Unknown claimed tasks block.

## Gate commands

```sh
# pre-push (fail-open on exit 2):
bin/fathom verify --service <svc> --plan docs/plans/<slug>.md

# CI (fail-closed, strict):
bin/fathom verify --strict --store <tmpstore> --service <svc> --plan docs/plans/<slug>.md
```

## Recorded demo (2026-07-07, first gated run)

The very first run of the gate against `h1-coding-agent-excellence.md` caught
the plan's author (an AI agent) over-claiming — the exact failure mode the gate
exists for:

```
✗ [md-001] Adoption instrument repaired: … {symbol: session_stats, skip_test: true}
    symbol_present: "session_stats" not found in the index
…
15 tasks total; 9 claimed: 6 verified, 3 failing        → exit 1, push blocked
```

`session_stats` was a Python function inside a bash heredoc — not an
index-provable symbol; the claim was rewritten honestly (non-code deliverables
are recorded in CHANGELOG, not claimed in gated plans). Two further failures
exposed a real recall gap (`test_reaches` misses method-call edges —
`docs/dogfood-observations.md` O6). After correcting the claims:
`8 claimed: 8 verified, 0 failing → exit 0`.

Lesson: contracts must name index-provable symbols; the gate is only as strong
as the honesty of its `skip_test` escapes, so every `skip_test` should cite why
(O6 tracks removing the method-edge class).
