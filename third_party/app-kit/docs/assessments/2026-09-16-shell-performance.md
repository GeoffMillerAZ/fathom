# Core shell: local performance baseline

This is bounded G2/core work under ADR-0069, not a new renderer, production SLO,
consumer upgrade or conversion-certification gate. The corrected v3 local baseline
passes; v1/v2 and the first v3 failures remain retained. The full standard stays partial.

## Contract fixed before measurement

The existing Go/HTML Gallery shell uses system fonts and local assets. For this
profile, low payload and short local page loads are an appropriate architectural
constraint. The contract selects Profile and Settings: ready state, simulated
load failure and native retry back to the ready form. App-state assertions are
prerequisites, not optional checks after timing passes.

| Observation, per document navigation | Ceiling |
|---|---:|
| Sum of decoded delivered response bodies, including HTML/CSS/scripts |131,072 bytes|
| Navigation Timing `responseEnd - startTime` |100 ms|
| Navigation Timing `domContentLoadedEventEnd - startTime` |250 ms|
| Navigation Timing `loadEventEnd - startTime` |500 ms|

These are declared local engineering ceilings, not results inferred from observations.
They replace an earlier unexecuted, looser proposal before acquisition. A failed
measurement must stay failed; investigate or explicitly revise a separate contract,
never widen these thresholds silently. CUE is the maintained contract source in
`fixtures/shell-budget/contract.cue`; the runner freezes its concrete JSON export.

Two lanes:1440×1000/light/scripts enabled and320×900/dark/scripts disabled, each with
three repetitions of both page journeys. HTTP cache is explicitly disabled for
resource accounting. Use the installed Chrome with an isolated profile, an owned
ephemeral loopback Gallery and fictional session data. No remote service, licensed
source, real account, installed binary or existing preview is changed.

The selected existing binary is
`/Volumes/m2ext/appkit-gallery-quality.spQ1NymK/gallery`, SHA-256
`001225811396ef0dd2b525cf9d5ab17f4274f7fb0c0036e777ca94888c2b6631`.
Its embedded CSS is
`0bb39edec5eb03cfe1a79ddf9ca1559399b56b3312e550b5782b115423b19ff8`.
No new build or prior acceptance replay is needed merely to collect these new metrics.
Host: Apple M4 (`Mac16,10`), 32 GiB RAM, macOS 26.6.2 (25G83); installed Chrome,
headless Playwright, unthrottled local loopback. Each receipt records browser version.

## Retained failure and explicit accounting revision

The v1 run at `/private/tmp/appkit-shell-budget.idvAw8/shell-budget-V96O3E`
stopped when the harness tried selecting a fixture control inside a closed native
Laboratory disclosure. Its receipt SHA-256 is
`3794de6463118ac46fea1a5a96f61aad36e7ae713cf4c2d6967323534891f023`.
It contains three partial ready-page observations, not a completed performance pass.
The executed harness, CUE/JSON contract and exact binary/CSS remain beside the receipt.

Independent Sol review found additional harness defects before the v2 run: aborted
forbidden requests could escape the failure list, ready-page assertions were too weak,
late response bodies could be omitted, zero load timing could pass and cleanup was
asserted rather than proven. Terra repaired those defects with negative canaries.
No application or CSS change is required by this failure.

The reviewed `shell-budget-v2` clarification keeps every numeric ceiling unchanged.
Each initiated document navigation is a separate observation: an action's
POST → redirect → final GET and that document's assets belong together; failure and
retry are distinct observations. The original harness incorrectly combined two
documents' resources with only the last document's timing. The complete journey must
still reach the visible error state and recover to the usable form.

V2 distinguishes a read response body from an affirmatively protocol-declared empty
redirect: actual `Content-Length: 0` and absent transfer/content encoding. This is
response-framing evidence, not an independently observed body read. Available body
reads count actual bytes; a body-read error alone never means zero. Unknown or
nonzero unreadable redirect bodies and cache-served responses fail; no refetch or
proxy supplies substitute bytes/timings. This is an explicit evidence-rule revision,
not a retroactive v1 pass. Global request attempts and terminal states must be accounted
for, app postconditions must hold and owned browser/server/port cleanup must be proven.

## V2 failure: source assumptions were wrong

The single v2 run at `/private/tmp/appkit-shell-budget-v2.W3ak58/shell-budget-RDMTGn`
completed with 30 observations and 87 failure records, exit 1. Receipt SHA-256:
`f803bc3c898269b1fc435e7417abc39aa4e6382d304fc142939383b5ba9653b9`.
Its executed harness SHA-256 is
`a785e8387d173db95ab740394b7cdae91507571834b02e706a85080539303884`;
concrete contract SHA-256 is
`d61e335dd2cc604853f816f94e274d91f1c71e3c4bc1eebb009190a473f90bcd`.

The script-enabled lane attempted 15 requests to the required local
`/static/shell-demo.js`, which the allowlist omitted. Those were correctly refused,
but this invalidated complete-asset measurement. The load-error fixture correctly
returned HTTP 503 while the harness expected 200. Its form assertions also counted
the hidden Resume button and incorrectly required clean-draft Save to be enabled.
Review had checked the harness without grounding these assumptions in the renderer;
the prior GO was insufficient. These are harness defects, not application regressions.

Source-grounded v3 retains the same numeric budgets and binary. It admits and counts
the exact script only in the enabled lane; requires the explicit error 503 followed by
retry 200; checks usable editable fields and the enhancement-appropriate Save state;
and treats document order and independently scheduled exact assets separately.
The source basis is `shell_demo_templates.go`, `shellDemoGET` in
`shell_demo_model.go`, and `shell_demo.js`. V3 does not erase either earlier failure.

The first v3 receipt at
`/private/tmp/appkit-shell-budget-v3.bFRGAt/shell-budget-CCoigN/receipt.json`, SHA-256
`090e443bcdb5f63714527a2a30c437447adee43b5eaac70e9b5155ce127a8ee9`,
retains 30 observations and 12 task failures. Assets, numeric budgets and theme
observations passed; browser/Gallery exited 0 and the port refused. Two selectors
were wrong: Settings uses `shell-timezone`, not `shell-time_zone`; the retry notice's
`Update` heading precedes `Your profile`. The follow-up corrects those selectors,
requires the same intended controls and records individual task facts. It does not
change the v3 contract, target artifact or acceptance budgets.

## Corrected v3 result

`/private/tmp/appkit-shell-budget-v3-followup.VcZgSg/shell-budget-Jr5Ozr/receipt.json`
passes 30 observations with zero failures. Its SHA-256 is
`ad9eaace6e233524eacdac198b4ee19df73e5ec61bcd5e6fe9975bc7cd2ac3eb`.
The frozen harness is `bd36593a0e6db15e96035547b8cfaa1ad9e0331a9d570f99bf3fce4c4981067d`;
concrete contract is `507eb862225cc601b05da4c87e435618bdcbc17605afb8976944ce51c3b214e8`.
Both match the executed inputs; root rechecked the workspace harness identity and Sol
reviewed the retained receipt. Chrome is 152.0.7977.84 on the host named above.

| Maximum per observation | Observed | Ceiling |
|---|---:|---:|
| Decoded response bodies |83,015 bytes|131,072 bytes|
| Response end |18.1 ms|100 ms|
| DOM content loaded |38.3 ms|250 ms|
| Load end |41.4 ms|500 ms|

The ledger accounts for 87 responses, including 12 redirects. The separate route-
interception log has 75 entries; it is not the total network-request count. CSS is
51,597 bytes on all 30 documents; the pinned 4,487-byte shell script is counted on
the 15 JS-on documents and absent from JS-off. Ready/error/retry facts, expected
Save states and computed light/dark polarity pass. No cache/unknown-resource failure
was recorded. Browser PID44486 and Gallery PID44478 exited 0, the owned port refused,
and a separate root PID check found neither process. No app/CSS optimization occurred.
This is app-page traffic evidence, not an OS-level network-sandbox claim.

The final fixture's 14 pure checks pass. The updated readiness inventory passes CUE
vet and the complete `internal/qualityplan` package; targeted MCP/Gallery quality and
intent-document tests pass. Seven changed Markdown files resolve 289 local links and
retain router/scope budgets. These checks are not a full repository or native test run.
The inventory update is source metadata; no installed Gallery/MCP binary was refreshed.

## Evidence boundaries

Decoded body bytes are not encoded network-transfer bytes. Navigation Timing is not
human task time, first useful paint, input latency or Go handler-only latency. Missing
timing/body observations, unexpected resources, failed user-task assertions and
uncertain cleanup are failures, never zero-cost successes.

This lane does not establish warm-cache behavior, cold process startup, degraded
network/CPU budgets, memory bounds, arbitrary devices, full accessibility or native
host performance. Those remain separate applicable work. The shell's full performance
standard cannot pass solely from this bounded baseline.
