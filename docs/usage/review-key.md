### `review-key` — one reuse-eligibility token (FL-17/FL-18/FL-19)

```sh
fathom review-key --service <id> (--since <git-ref> | --manifest FILE) [--store DB] [--prior KEY.json] [--json]
```

Computes one sha256 token (`internal/analysis/reviewkey`) over every input a
`fathom review` verdict depends on, so a caller (typically an external
review kit) can decide whether a prior review result is still safe to reuse
without a second bespoke hashing API:

- `manifest` — the changed-path digest set (from `--since`'s git diff,
  hashed off the working tree, or `--manifest FILE`'s own declared
  digests).
- `closure` — the typed import/call closure of the changed files, walked
  forward over the SAME typed call graph `blast_radius`/`review_diff` use
  (never a new import parser), digested by each reached file's store
  fingerprint.
- `rules` — the project's `.fathom/rules/archconformance.cue` +
  `.fathom/guard.cue` + any project-local `.fathom/layers/*/layer.cue`.
- `plans` — every file under `docs/plans/`.
- `tooling` — fathom's own version identity.
- `index` — the service's connector watermark + store watermark.

```json
{ "schema_version": 1,
  "key": "<sha256>",
  "complete": false,
  "unknown": ["dependency closure: pkg/pkg has no typed edges"],
  "components": {
    "manifest": "<sha256>", "closure": "<sha256>", "rules": "<sha256>",
    "plans": "<sha256>", "tooling": "<sha256>", "index": "<sha256>" }
}
```

A component whose input could not be verified (an unreadable manifest path,
a changed package the typed pass never analyzed) never hashes as empty —
it is excluded from that component's own digest and named in `unknown[]`,
and `complete` is false; the key still computes over whatever WAS
resolved, so a caller may reuse narrower results or refuse — fathom only
reports. `--prior KEY.json` diffs a prior key against this run and adds
`changed_since_prior: [component, …]`. Identical inputs produce an
identical key across runs and machines. `pkg/client.ReviewKey` is the same
code path this CLI calls — a Go embedder gets byte-identical answers.
