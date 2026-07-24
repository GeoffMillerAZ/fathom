# Wave 5 — Trust bugs + the tokenomics second swing

Owner directive (2026-07-23): fix the bench's trust findings AND take one
more falsifiable swing at pulled-tool tokenomics/discovery — not a retreat
from the home-turf refutation, a causal-fix attempt with a pre-registered
end state either way.

Design (normative):
- **T5-1 O13 — citation drift.** The sweep verifies anchor TEXT, not just
  the line hash at the cited offset: on drift, attempt re-anchor (search
  the corpus for the rule's stored anchor text; single unambiguous match →
  re-cite with an event recording the move, like ADR-017 remap), else flip
  stale. Fixture: upstream-insertion shift (the exact O13 reproduction
  shape). Then the operational pass: a fleet re-sweep flags/heals existing
  wrong citations (recorded counts in this row).
- **T5-2 O14 — eviction visibility.** The obligations serving path reports
  `displaced: N` when the cap drops served-eligible claims, and the
  response labels which layer each served rule came from. Scope-mapping
  noise (mis-scoped rules) is a compile-prompt fix — FILED as the next
  flag-day's change (prompt identity bump), not silently patched.
- **T5-3 refutation propagation.** Every doc asserting the pulled-tool/
  home-turf story gets the verdict with the dated pointer (the H3-1 row's
  owed slice). Sweep: design docs, use-cases, adoption guides, vision.
- **T5-4 read_source.** A line-ranged, size-capped, read-only raw-text tool
  on the agent surface. Kills the T13 class (601k tokens of contortion for
  want of `sed -n`). Doctrine-4 arithmetic stated honestly: the program's
  second net-new tool, justified by measured evidence; the orientation
  quadrant's law-8 clock remains the offset candidate.
- **T5-5 line-granular doc anchors.** DocChunk carries line offsets so
  doc-cited answers can pin lines (kills the T08 class; feeds O13
  precision).
- **T5-6 one-shot completeness.** symbol_context returns the definition's
  source snippet inline (capped) — define-and-show answers in ONE turn.
  Turns are the tokenomics lever; every saved turn ≈ the session baseline.
- **T5-7 forced-channel measurement.** The load-bearing claim gets the
  measure-or-retract discipline: verify_plan/review calls log their verdict
  outcome (pass/fail/catch counts) and `instrument --adoption` reports
  gate catches honestly (replacing the current "not instrumented" marker
  for exactly that field).
- **T5-8 the second swing, pre-registered.** After T5-4/5/6 merge: re-run
  BOTH benches (lookup + home corpora, unchanged criteria, turns reported
  alongside). Outcomes recorded in this row either way. KILL CRITERION,
  BINDING: if the home bench still fails its criterion, the parity
  positioning is PERMANENT — pulled-tool investment drops to maintenance,
  the docs keep the refutation, and no third swing is attempted.

## Tasks

- [ ] Sweep verifies anchor text with single-match re-anchor + event trail; upstream-shift fixture proves heal and stale paths {id: T5-1, symbol: SweepClaims, package: pkg/client}
- [ ] Fleet re-sweep run post-merge: wrong-citation counts found/healed/staled recorded here {id: T5-1b, symbol: SweepClaims, package: pkg/client, skip_test: true, skip: "operational row; evidence = recorded counts + O13 status update"}
- [ ] Obligations serving reports displaced count + per-rule layer labels; eviction test {id: T5-2, symbol: ObligationsFor, package: internal/semantic}
- [ ] Scope-mapping prompt fix filed for the next flag-day (prompt identity bump, NOT compiled now); O14 updated {id: T5-2b, symbol: ObligationsFor, package: internal/semantic, skip_test: true, skip: "filing row — the fix is recompile-gated by design; anchor shared with T5-2"}
- [ ] Refutation propagated to every doc asserting the pulled-tool story; sweep list recorded {id: T5-3, symbol: cmdVerify, package: cmd/fathom, skip_test: true, skip: "docs row anchored to the gate at the story's center; critic pass enforces the sweep"}
- [ ] read_source: line-ranged, size-capped, read-only; on the agent surface; refusal tests for range/size abuse {id: T5-4, symbol: registerReadSource, package: internal/mcp}
- [ ] DocChunk line offsets end-to-end (extractor → store → serving); T08-class fixture {id: T5-5, symbol: registerReadSource, package: internal/mcp, skip_test: true, skip: "anchor shared with T5-4 until landing — update to the real extractor symbol at landing; its own tests are the coverage"}
- [ ] symbol_context inline definition snippet (capped); one-turn define-and-show test {id: T5-6, symbol: registerSymbolContext, package: internal/mcp}
- [ ] verify_plan/review verdict outcomes logged; instrument --adoption reports gate catches (marker replaced with real counts); tests {id: T5-7, symbol: cmdInstrument, package: cmd/fathom}
- [ ] (post-merge) Both benches re-run, unchanged criteria, turns reported; outcome + the binding consequence recorded {id: T5-8, symbol: cmdVerify, package: cmd/fathom, skip_test: true, skip: "measurement row; evidence = committed artifacts + this row's verdict"}
