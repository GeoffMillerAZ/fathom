### `retro` — did the firings actually help? (mechanical, free)

```sh
fathom retro [--session S | (latest stamped)] [--project DIR] [--json] [--write]
```

Joins a session's intervention firings (guard fires, obligation
injections, custodian attention) against what is on disk afterward:
committed / modified-uncommitted / untouched per fired file, concern
remediation now-state, silent-on-clean counts. Unjoinable (pre-stamp)
firings are counted and named; OFF-arm windows say channels were silent
by design. Outcome-shaped evidence, never causality (the arm carries
that — law 12). `--write` lands the retro under
`workspace/artifacts/retros/`. The priced judgment lane is `assess
--interventions` (below). Design: `docs/design/intervention-outcomes.md`.
