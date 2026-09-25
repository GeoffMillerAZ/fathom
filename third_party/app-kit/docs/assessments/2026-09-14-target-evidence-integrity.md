# Target-evidence integrity — 2026-09-14

## Result and boundary

The separate `internal/designreceipts` CUE/Go checker matches a real retrospective
P06/static-web Links ledger against ten retained artifact identities, while preserving
all nine full gates as `not-run`. Source/report byte-tampering canaries are rejected.
This closes an integrity-layer gap, not the broader target-receipt/certification goal.
Legacy totals stay **439/657 compiler/DOM, five typed candidates, zero certified targets**.

An all-passed synthetic ledger also returns `verification=not-verified`. The checker
cannot authenticate the truth of arbitrary reports, reviewer identity, archive membership,
complete dependency closure or current compatibility. [ADR-0042](../adr/0042-separate-evidence-integrity-from-certification.md)
records that boundary; [the guide](../design/target-evidence.md) describes caller obligations.

## Reproduction and evidence

```sh
APPKIT_P06_RECEIPT_ROOT=data/design-assets/navigation-typed.FGyrmV \
  GOWORK=off GOPROXY=off go test -race -count=1 ./internal/designreceipts
GOWORK=off GOPROXY=off go test -race -count=1 ./cmd/appkit-mcp
```

Both packages pass together with the explicit private root: 2.769s and 21.217s.
The focused retained-artifact run separately confirms ten matches and nine incomplete
gates. Source-only runs keep metadata tests but explicitly skip the private read; an
explicit empty/missing private root fails, never silently skips. No browser was rerun
for this ledger import, and no private source/output bytes were copied into tracked files.

The retained P06 report contains 115 scoped checks, 23 screenshots, actual200% zoom
and two retained source failures, under `passed-with-source-limitations`/`not-verified`.
It predates this contract. Its full interaction gate remains not-run, including touch
and screen-reader gaps. Named cold/warm latency metrics and real-app integration remain
unproven. The original [adapter assessment](2026-09-14-navigation-typed-adapter.md) stands.

Tests exercise canonical ordering and raw ambiguous JSON, exact nine-gate schemas,
independently supplied expected pins, input/evidence role separation, missing/extra/
tampered content, count/size/text limits, N/A reason/review/applicability consistency,
shared slots and incomplete/all-pass non-certification. Malformed JSON tests directly
exercise decoding so a later stale-binding error cannot hide parser acceptance.
The first valid-fixture run caught an invalid2048-repeat regexp; CUE `strings.MaxRunes`
replaces it and has positive/overlength regression coverage. Schema compilation alone
had not exercised that validator. No production consumer was exposed to this code.

MCP now routes `get_kit_decision("target-evidence")` and `kit://target-evidence` to the
same authored guide. The in-memory protocol roundtrip checks text/structured bodies,
resource identity/size and guidance routing. It does not serve or certify target records.

## Exact identities

| Artifact | SHA-256 |
|---|---|
| Canonical P06 contract | `854eaeb9483e5df1422a46fd6ab9b5a44e7ba070212dd519914a6baf257430ee` |
| Canonical P06 receipt | `4e5df913973635c9e6d51ec6ee184bc4dc790e6c2a7d7492331f3c9e26e44c91` |
| `internal/designreceipts/check.go` | `81888a638942d5da590840f6477252c036b5c3015ee032820a8ca95be18b049b` |
| `internal/designreceipts/schema.cue` | `7112614b414d979f37105277342f3693c8de3f7f1dea5fb61aceaf64a109a550` |
| `internal/designreceipts/check_test.go` | `43d5fbbe32a3b76f361f0db54afda73b62bd6c54f12c30098170c0ca27e0cab0` |
| `internal/designreceipts/p06_retained_test.go` | `2c2c7787f7c68a7bd403367f960bb4ec6ce3c987c17d99b300da4a8265135ebf` |

## Remaining work

Freeze a prospective, complete candidate contract before new semantic runs; earn each
applicable gate with independent oracles and reviewed exclusions. Add consumer history,
safe private persistence and identical Gallery/MCP target-record projection without
changing original access. A complete integrity inventory is necessary but insufficient
for compatibility. This package is internal, not a new standalone CLI/store or runtime.
