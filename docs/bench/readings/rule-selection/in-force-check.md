# RS-M3: in-force hand check of 20 claims (2026-10-21, $0)

Plan row `RS-M3` in `docs/plans/rule-selection.md`, design ADR-077 d3, code `claimInForce` (`internal/semantic/inforce.go`). Question: does withholding a claim that cites a fully superseded ADR section ever withhold a rule that is still in force? Bar: 0 wrong withholds over 20 hand-checked claims, including every claim citing ADR-003, ADR-004 and ADR-013.

## Verdict

RS-M3 verdict: PASS

0 wrong withholds. Of the 526 served-status claims of service `fathom` that cite `ADR.md`, 10 are withheld (ADR-003: 6, ADR-013: 4), 6 stay tagged `partly_superseded` (ADR-004), 510 are in force. Every withheld claim's citation lies in a section whose own status line says fully superseded, and each rule is either replaced by its superseding record or still served from another in-force or tagged claim. One withhold loses part of a restatement (claim 10 below): a finding, not a wrong withhold.

## Method (re-computable)

1. Data: the read-only `.backup` copy of the shared daemon's store used by RS-M1 (`workspace/db/fleet.sqlite`, 2026-10-21), service `fathom`, its `service_meta.path` pointed at this branch's clone so `ADR.md` is read at HEAD. Nothing was written to the shared store; no model call.
2. Listing: `FATHOM_RS_STORE=<copy> go test ./internal/semantic -run TestRSM3InForceListing -v` lists every served-status claim (fresh, stale, contradicted) whose primary citation is in `ADR.md`, with the standing `claimInForce` gives it (the resolver serving uses, `internal/adr.ParseSections` over the file).
3. Hand check: for each claim, the cited lines, the section they fall in, that section's `**Status.**` paragraph, and whether the rule as stated still binds today. A wrong withhold is a withheld claim whose section is not fully superseded by its own status line, or whose rule binds today with no other served claim carrying it.

## The 20 claims

| # | Citation | Record (status line) | Rule (abridged) | Standing | Hand verdict |
|---:|---|---|---|---|---|
| 1 | `ADR.md#L43-L43` (obligations) | ADR-003: "Superseded by ADR-020 … and ADR-025 (DuckDB deleted outright)" | Persist observations in DuckDB | withheld | right: DuckDB is deleted (ADR-025) |
| 2 | `ADR.md#L43-L43` (adr-rules) | ADR-003 | same rule, other layer | withheld | right |
| 3 | `ADR.md#L52-L52` | ADR-003 | Run tests and CI with CGO_ENABLED=1 | withheld | right: serving it now would be wrong (ADR-066, CGO_ENABLED=0) |
| 4 | `ADR.md#L52-L53` (adr-rules) | ADR-003 | Store package must hide DuckDB behind a Store interface | withheld | right: names the deleted backend; the Store seam binds through ADR-020 (claims 19, 20) |
| 5 | `ADR.md#L53-L53` | ADR-003 | Hide DuckDB behind a Store interface | withheld | right, as 4 |
| 6 | `ADR.md#L53-L54` | ADR-003 | Hide DuckDB behind a Store interface (unit-testability) | withheld | right, as 4 |
| 7 | `ADR.md#L237-L239` | ADR-013: "Superseded by ADR-066 … the last CGO binding left `go.mod`" | Use tree-sitter via the official CGO bindings | withheld | right: the CGO bindings are gone |
| 8 | `ADR.md#L237-L243` (adr-rules) | ADR-013 | General-language extraction runs tree-sitter via CGO, not wazero | withheld | right |
| 9 | `ADR.md#L241-L241` | ADR-013 | Use go/parser for Go | withheld | right: ADR-013 only restates it ("Go still uses go/parser"); claims 13 and 14 (ADR-004, tagged) still serve it |
| 10 | `ADR.md#L241-L243` | ADR-013 | Use native pure-Go parsers for the Track A config/IaC formats | withheld | right by the record, lossy: claims 15 and 16 (ADR-004) serve HCL, k8s YAML and CUE, but ADR-013's list also named proto, gqlparser, goldmark, go-toml, now served by no claim |
| 11 | `ADR.md#L63-L65` | ADR-004: "Superseded in mechanism by ADR-013; revived in part by ADR-021 §4" | Run tree-sitter grammars as WASM in wazero | tagged | right: wazero is revived for Python/JS/TS/Ruby (ADR-021 §4), so it binds in part |
| 12 | `ADR.md#L63-L66` | ADR-004 | same, multi-language extraction | tagged | right, as 11 |
| 13 | `ADR.md#L65-L66` | ADR-004 | Go parsing uses stdlib go/parser and go/ast | tagged | right: binds today |
| 14 | `ADR.md#L65-L66` | ADR-004 | Go extraction must use go/parser and go/ast | tagged | right |
| 15 | `ADR.md#L66-L66` | ADR-004 | HCL, k8s YAML and CUE use their native Go parsers | tagged | right: binds today |
| 16 | `ADR.md#L66-L66` | ADR-004 | same, "must" form | tagged | right |
| 17 | `ADR.md#L1031-L1042` (adr-rules) | ADR-025: accepted | DuckDB backend, its test, MigrateNonDerivable and migrate-store are deleted | in force | right: ADR-025 supersedes ADR-003 and stands |
| 18 | `ADR.md#L1031-L1042` | ADR-025 | Delete DuckStore and its migration verb | in force | right |
| 19 | `ADR.md#L751-L756` | ADR-020: "Accepted (2026-07-22)" | SQLite backend, modernc.org/sqlite, pure Go, WAL | in force | right |
| 20 | `ADR.md#L754-L756` | ADR-020 | Use store.BackendForPath for extension-based selection | in force | right |

## Notes

- `internal/adr.ParseSections` alone reads ADR-004 as `superseded`: `statusFromText` matches the leading word. `claimInForce` reads the whole status paragraph and keeps a record partly in force when it says "in part", "partly", "partially", "in mechanism" or "revived". Every section of `ADR.md` carries a status line today, so the "no status line parses as accepted" case is pinned by `TestClaimCitingSupersededADRIsWithheldAndCounted`'s fixture, not by this corpus.
- Claim 10 is the one loss: a superseded record restating a rule that still binds through no other claim. It is the d3 rule working as written (the status line is the oracle, DM2), and it is fixed by a recompile of the rule from a record in force, not by serving the superseded one.
- The store holds the daemon's last compile, not a fresh one of the committed HEAD; the 12+ claims ADR-077's context counts read here as 16 (10 withheld, 6 tagged).
