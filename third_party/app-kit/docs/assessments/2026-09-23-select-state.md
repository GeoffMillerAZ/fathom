# Missing select values must not silently become valid choices

## Contract and boundary

R1/R4 shared-core repair, baseline `e5903df`. I4/I7/I12/I15/I17, the native form
contract and ADR-0069's still-open core standards apply. [ADR-0083](../adr/0083-render-missing-select-values-as-unavailable.md)
records a breaking rendering correction, not a new API or runtime.

An absent nonempty value gets a selected disabled empty prompt (caller placeholder
or `Selection unavailable`); a placeholder exclusively owns initial empty selection.
Never reflect the unavailable value or silently choose another valid option. Preserve
exact known/default output, input options, CSS, existing attributes and app-owned
validation. Matching disabled choices intentionally retain native semantics; required
validity alone does not promise a submitted entry. Server validation remains required.

The fixed pre-acquisition cases cover absent required/custom/optional, duplicate empty,
known and native-default selections; initial state, reset, validity, form serialization
and recovery to weekly delivery. No live settings, consumer mutation or persistence.
Budget: one paired browser acquisition and one diagnosed input-method correction;
no repeated key permutations, broader browser matrix or quality promotion.

## Results

Frozen new tests fail on the archived original for the two absent-value variants
and duplicate-empty selection. Unaffected byte and input-immutability controls pass.
The candidate passes form/Gallery/MCP/qualityplan/catalog race suites (1,510 test and
subtest passes;12 existing opt-in/private checks skipped), form/Gallery vet and
isolated archived-candidate form race tests. Final current-source MCP tests also pass.
These are scoped tests, not an all-repository or clean-VCS release claim.

The first browser run reproduces accidental daily selection/submission but fails
native `End` selection, then refuses the unintended `cadence=daily` navigation.
Its exact driver, report and screenshot remain retained. No native-choice pass follows.

One v2 correction uses Playwright `selectOption` for option changes, not key choice.
Both sides use the same fixed expected states, source-generated HTML and actual Gallery
CSS with application JavaScript disabled. Native Tab focus, Reset, required validation
and Enter submission remain actual browser actions. Two lanes:1440×1000/light and
390×844/dark; six cases each. Baseline has165 ordinary checks and16 expected-state
mismatches; candidate has187 checks, zero mismatches/errors and12 completed journeys.
No unexpected requests; exact document/CSS/receipt responses, no horizontal overflow,
unchanged input hashes and owned browser exit/output-drain all pass. Four captures
were reviewed: the candidate replaces misleading `Daily digest` with the readable
`Selection unavailable` prompt without shifting either layout.

Review caught a weak baseline predicate: v2 counted any mismatch. A separate stronger
offline oracle pins the original daily/default and duplicate-empty states. Five tests
pass, auditing all72 initial/reset/weekly states across the two retained reports and
rejecting arbitrary baseline mismatch/candidate regression/mutated-report canaries.
Thus16 historical state defects are separately confirmed, not accepted just because
they differ. The candidate oracle is unchanged. The maintained v3 driver uses that
predicate, but no v3 browser acquisition is claimed.

## Retained identities and replay

Private packet: `/Volumes/m2ext/appkit-evidence/appkit-select-state.lYMKC3iL`.
`baseline-src` retains the archived source plus exact collector/new RED tests;
`candidate-src` retains source plus the form/catalog change and collector.
The candidate archive predates the final documentation-only evidence updates.
Collectors use Go1.26.3 with `-buildvcs=false`, not a clean-VCS build assertion.
Reports bind Node26.3.0, Playwright1.62.1, Chrome153.0.8010.53, source archives,
collector source/binaries/JSON, drivers and lifecycle helper before/after execution.

| Artifact | SHA-256 |
|---|---|
| `baseline.tar` | `30c58c171e7091c09277d337a8bb9c9df2cc7f35acf9bc391b1df484a94f08b1` |
| `controlled-selection/candidate.tar` | `9d91ca21cb41fca43318facde9832d78184c14bd73ba7247858ca8d533757473` |
| `harness/select-state/browser.mjs` (failed native choice) | `929a1a804b71bdff708aa63d9ac083f5f9622847c2efc8cfa5de36ac8533d3e9` |
| `harness-v2/select-state/browser.mjs` | `620f1560a70b46d932313b111054bfe3717b32652600b98731dac4f0c6318ec2` |
| `browser-baseline/report.json` (native failure) | `bcf4aba34763570a9a7e006cba4e6d31e39b5b1e688e5903630fe9f477d4d748` |
| `controlled-selection/browser-baseline/report.json` | `52f47a1f18bcedcd633f8dcd744a544bad051b0941b67f3f2dbb91f132fac55e` |
| `controlled-selection/browser-candidate/report.json` | `a35ec806e1ebb28829f3d0feee26fd616167e666d5f1793547f57742fd5aefb3` |
| `harness-v3/select-state/oracle.mjs` | `0f6f873eb5e2ce8745c3a9f5173d6b56f301289537cc2f1a637c264cc79491b0` |
| `oracle-audit.log` | `7b2379b16e1e4abd255154820ffd1d2fcabc10c70324f321bd67dbed10334ee3` |

Fixture source: [collector](fixtures/select-state/collector.go.txt),
[browser](fixtures/select-state/browser.mjs), [offline oracle tests](fixtures/select-state/oracle.test.mjs).
Replay the latter with `SELECT_STATE_REPORTS` containing the two absolute report paths
separated by the host path delimiter. No browser is launched. Tests do not claim physical input,
screen-reader support, all themes/platforms, perf or persistence. No consumer,
installed binary, dependencies, configuration or service changed. Rebuild and review
the changed `interaction.forms` metadata pin and markup before adopting. Full form,
core, G3/G4 and converted-target certification remain open.
