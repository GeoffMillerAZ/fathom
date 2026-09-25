# Shared table row-link safety and keyboard targets

## Prospective contract

R1/R4 owner-funded core repair, baseline `a7c4ff3`. `table.Row.Href` is cast directly
to `templ.SafeURL`, bypassing URL sanitization; its small inline Open link has no
minimum target or explicit focus treatment. Fix the kit-managed row destination,
not app-owned query, sort, selection, authorization or custom action behavior.
I4/I7/I12/I17 and [ADR-0080](../adr/0080-harden-table-row-destinations.md) apply.

Preserve accepted relative web references and absolute HTTP(S) exactly. Share the
existing form-button shape predicate internally; preserve all form output. Refused
nonempty destinations become nonfocusable link-role labels, visibly “Open unavailable”,
with aria-disabled and no href or action handler. Empty Href/custom RowAction behavior
stays unchanged. Enabled Open anchors gain44px minimum height/width and explicit
semantic focus styling. Apps still supply permitted destinations and explanations.

Freeze parsed-DOM tests before RED/GREEN. Verify accepted/unsafe shapes, escaping,
inert semantics and untouched custom/no-link output. Run table/form/Gallery/MCP/
qualityplan regressions, pinned templ generation and CSS regeneration. No dependency,
consumer/pin, installed tool, daemon, global configuration or production data changes.

Browser contract: actual table renderer and Gallery CSS, fixed synthetic named rows,
1440×1000/light and390×844/dark, application scripts disabled. Read rows, use native
Tab/Enter to open Alpha and Back, skip the refused destination and open Beta/Back;
measure actual anchor44px targets and focus, no page overflow, exact completed
HTML/CSS and retained captures. Only click the unsafe-row label after proving it
has no href and is disabled; never execute a baseline unsafe destination. Same frozen
oracle for baseline and candidate. Isolated fresh sandboxed Chrome, exact source/
binary/tool identities and bounded owned-process cleanup. No live Gallery host.

Budget: one RED/GREEN sequence, scoped regressions, one baseline and one candidate
browser acquisition with at most one diagnosed harness correction. Stop after this
repair; no whole-table redesign, sorting engine, consumer rollout or full quality
promotion. Retain failures and byte inputs in
`/Volumes/m2ext/appkit-evidence/appkit-table-links.wviVEVTJ` before any tested source is edited.

## Result

Implemented ADR-0080 using the unchanged form-button predicate in private
`ui/internal/webref`. Parsed-DOM tests retain exact relative/HTTP(S) destinations,
HTML escaping and custom-slot/no-Href behavior, and refuse executable/opaque schemes,
controls, ambiguous references and malformed URLs/escapes. The original RED test
bytes/log remain. One test assertion initially required query escaping even for URLs
without a query; its corrected bytes were frozen before GREEN, not substituted for RED.

Table/form/Gallery/qualityplan full race suites pass. MCP's first full race run caught
the new private-helper directory missing from the tier document; the explicit private,
non-rendering row repairs that omission and the full MCP race rerun passes. An initial
command also named nonexistent `internal/vendor`; it is not a pass. The actual CLI
vendor-focused race tests and table/form/helper/Gallery vet pass. Generated templ output
is identical on replay; regenerated Gallery CSS remains byte-identical (46,191 bytes).

The same frozen v2 browser driver runs both source-bound collector builds on macOS,
Go1.26.3, Node26.3.0, Playwright1.62.1 and sandboxed Chrome153.0.8010.53. Both acquisitions
finish the two prescribed no-script lanes with exact completed HTML/CSS, native
Alpha/detail/Back and Beta/detail/Back, no page overflow or unexpected requests,
unchanged input hashes and clean, unforced owned-process exit/output drainage.

| Selected observation | Baseline | Candidate |
|---|---|---|
| Unsafe destination | Native JavaScript href, never activated | Disabled link-role span; guarded pointer click inert |
| Alpha/Beta targets | 45.25×15px,1px browser focus | 45.25×44px,2px semantic focus |
| Native tab order | Alpha → unsafe → Beta | Alpha → Beta |
| Driver result | 82 ordinary checks;10 expected quality failures;0 other errors | 94 checks;0 errors |

The first baseline driver also had two measurement errors: row `textContent` does not
insert separators between cells. Its source/report remain under `harness/` and
`browser-baseline-v1/`. The single permitted correction joins each cell's normalized
text without changing expected labels/values, application bytes or quality requirements.
V2 reruns baseline and runs candidate; no additional acquisitions. Four v2 screenshots
were visually reviewed: readable rows, explicit unavailable state and unclipped focus.
Row heights/column allocation intentionally change; the pre-existing short no-Href row
remains, reinforcing that this is not whole-table layout certification.

## Identity and handoff

Packet manifests retain exact source, collector, response, helper, runtime and driver
hashes; builds use isolated archived module trees, not a sibling checkout. These are
source-bound synthetic executables, not clean-VCS release artifacts.

| Artifact | SHA-256 |
|---|---|
| `baseline.tar` (`a7c4ff3`) | `1e64da04f96676c72962b579171bae75b157872a5fe3c12fd59a0ea94b74c8d2` |
| `candidate.tar` | `01a420dd4fce386ae64a405df1a84c744953d460f6de96a05d4e2c7af31c1824` |
| `harness-v2/table-links/browser.mjs` | `529d1f647a705b1b77dcdeb3b51fbc199260a45cd6cc1699bf2a4323394fcca2` |
| `browser-baseline-v2/report.json` | `453155ea706d19395c680a6dd254150a5ff005ecfb14c99296b5d0aaf2aad0b4` |
| `browser-candidate-v2/report.json` | `a2f4a89fd3da7509271f232b7dd1f3cf115706ac1c6049c812990cb47349a3fb` |

The [collector](fixtures/table-links/fixture.go.txt) and [driver](fixtures/table-links/browser.mjs)
are retained for deliberate replay. Adopters rebuild Go/templ and scanned CSS, review
`data.table` selected metadata changes and table density, then verify their own native
Open/detail/return task. Go delivery must include the imported private helper. No consumer
pin, installed tool, dependency, daemon or default palette changes. Full accessibility,
sorting/selection, muted/activated/custom rows, browser families, real consumer adoption
and core/G3/G4 acceptance remain open; no gate or weighted-category promotion follows.
