# Wrapped plan rows re-verified after G2-5 (G2-7)

G2-7 reading: 20 wrapped rows re-verified; all 20 were no-contract before G2-5; after it 3 are proven, 13 disproven, 3 still unproven and 1 is still no-contract (DP-9 names no symbol).

Plain summary: before G2-5 the parser read only the checkbox line, so every row whose annotation sat on a
continuation line verified as a positional `md-NNN` row with no contract: nothing about it was checked, and
the default gate let a claimed one through. After the fix the same rows carry their contracts. In this
repository's feature map (FM-1 to FM-9, all open) every row now fails honestly: `internal/featuremap` does not
exist yet. In agentic-kit's decision port (DP-1 to DP-11, nine claimed) three claimed rows now prove, three
claimed rows read library-only under the wired check, two claimed rows are disproven (no test reaches the
symbol), the two open rows fail their `reachable_from` edge, and DP-9 stays no-contract because its
annotation names a package and no symbol. Eight of the nine claimed rows the gate could not check before are
checked now.

Date: 2026-09-29. Binaries: the base (4ae46d03, before G2-5) and the G2-5 line (e92022bd), same store for both.

## Method

- Stores: a fresh scratch ingest of this repository at e92022bd (service `fathom`), and a fresh scratch ingest of
  a clone of agentic-kit at 32b1786 (service `agentic-kit`). Neither is the shared daemon's store.
- For each plan, `fathom verify --format json --store <scratch> --service <svc> --plan <plan>` with the base
  binary and with the G2-5 binary; rows matched by their checkbox line (the base reads them as `md-NNN`).
- Buckets: no-contract (label `no-contract`), proven (status pass), disproven (status fail), unproven (status
  unknown with a contract, here `library-only`).

## Moves

| Row | State | Before G2-5 | After G2-5 | What the check now says |
|---|---|---|---|---|
| FM-1 to FM-9 | open | no-contract (md-001 to md-009) | disproven (claimed-but-absent) | none of the nine symbols is under `internal/featuremap` yet |
| DP-1 | claimed | no-contract | proven | |
| DP-2 | claimed | no-contract | proven | |
| DP-3 | claimed | no-contract | unproven (library-only) | no product entry point reaches `BandOf` |
| DP-4 | claimed | no-contract | proven | |
| DP-5 | claimed | no-contract | unproven (library-only) | no product entry point reaches `NewLLM` |
| DP-6 | claimed | no-contract | unproven (library-only) | no product entry point reaches `Records` |
| DP-7 | claimed | no-contract | disproven (present-but-untested) | no Test* statically reaches `MockDecider` |
| DP-8 | claimed | no-contract | disproven (present-but-untested) | no Test* statically reaches its symbol |
| DP-9 | claimed | no-contract | no-contract | the annotation names `package:` and no `symbol:` |
| DP-10 | open | no-contract | disproven (edge-not-wired) | `TestTypeSafe_Decide` does not reach `NewTypeSafe` |
| DP-11 | open | no-contract | disproven (edge-not-wired) | `TestValidate` does not reach `Validate` |

Counts: feature map 9 no-contract to disproven. Decision port 3 to proven, 3 to unproven, 4 to disproven, 1
unchanged. The agentic-kit rows are that repository's to fix or re-anchor; this reading records them only.
