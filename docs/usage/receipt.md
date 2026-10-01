### `receipt` — a stranger's re-derivation (VB-20)

```sh
fathom receipt verify [--repo <path>] <receipt.json>
```

Flags precede the positional argument (Go's `flag` package stops parsing
flags at the first bare argument, the same convention `fleet undo` already
uses) — `--repo` before `<receipt.json>`, never after.

Re-derives a saved agent-surface answer's claim from a bare checkout of the
receipt's own `index_commit` (the envelope field VB-19 adds to `find_callers`,
`references`, `blast_radius`, `symbol_context`, `safe_delete_check`,
`rename_impact`, `test_impact` and `review_diff`) using
`golang.org/x/tools/go/packages` and `git` alone — it never opens `internal/
mcp`, a fathom store, or the daemon, so a stranger with the Go toolchain and
this repository's git history gets the same verdict fathom's own gate does
(ADR-034 receipts; ADR-035 decision 4, `docs/vision/29-verbs-not-lookups.md`
§3 wave E).

A receipt is a small JSON file the caller saves from an answer's envelope —
`kind` (`callers` | `references` | `symbol_location` | `certification`), the
queried `symbol` (bare name or `Recv.Method` — go/types' own vocabulary),
`index_commit`, and the facts to check (`callers`, `files`, `file`/`line`,
or `test`/`reached` for `certification`):

```json
{
  "kind": "callers",
  "tool": "find_callers",
  "symbol": "validateCharge",
  "index_commit": "a1b2c3d...",
  "callers": ["Charge"]
}
```

`--repo` names the git checkout to `git archive` from (default: the current
directory). Exit codes: **0** the claim reproduced; **1** it did not — stdout
names the difference (a missing/extra caller, a file no longer mentioning the
symbol, a moved definition line); **2** it could not run at all — no git
repository at `--repo`, the commit is missing, the checkout has no `go.mod`,
or the receipt file is missing/invalid — an operational refusal, never a
false pass or fail.

**`certification` receipts** (TC-13, vision 33 / ADR-043) carry `test` (the
certifying `Test*` function, `"dir/pkg.Name"`) and `reached` (the symbols its
certification claims that test reaches) instead of `callers`/`files`:

```json
{"kind": "certification", "symbol": "Charge", "test": "internal/billing.TestCharge_Idempotent",
 "index_commit": "a1b2c3d...", "reached": ["Charge", "validateCharge"]}
```

Reach is re-derived independently with go/types alone, exactly like the
other three kinds (`internal/receiptverify` + `internal/contextbench/publicloc`
— never `internal/certify`'s own indexed computation). The run itself is
reported as `pass: unverified (no run)` unless `--run-tests` is given, in
which case `fathom receipt verify` actually executes
`go test -run '^<Test>$' <pkg>` for that one test and reports `pass: true`
or `pass: false` — ADR-043 decision 5: a reached test is not a passing test,
so the verdict is never inferred from reach alone.

| Flag (`receipt verify`) | Default | Description |
|---|---|---|
| `--repo` | current directory | git checkout to re-derive from |
| `--run-tests` | `false` | `certification` kind only: actually run the named test instead of reporting `unverified (no run)` |
