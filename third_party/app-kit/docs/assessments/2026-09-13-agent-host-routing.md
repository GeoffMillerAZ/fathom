# Fresh agent-host routing trial — 2026-09-13

## Result and boundary

Codex CLI 0.154.0 and Claude Code 2.1.270 each completed a fresh, read-only planning
session for the same synthetic Workshop Console consumer. Both retrieved the consumer
contract, identical MCP intent bytes, the same two capability contracts and actual source.
Both kept routes/authorization app-owned and explicitly withheld UI/test/delivery claims.
This advances [G4](../intent/scope.md#g4--make-adoption-and-improvement-propagation-demonstrable);
it does not finish it. One trial per host is not a reliability or fleet adoption rate.

The [receipt](fixtures/host-routing/receipt.json) binds document/source/transcript hashes,
catalog identity, process completion, observed MCP calls and reported usage. Raw traces,
invocations, prompt, frozen first-party snapshot and the two binaries remain private in
`data/adoption-evidence/2026-09-13-host-routing-eetm7M/` (gitignored; back up separately).
The original task directory was `/tmp/appkit-host-proof.eetm7M`.

## Frozen inputs

- Consumer selects `go-web`, L1/L2/L3b, `shell.layout` and `shell.appearance`.
  It describes an equipment-utilization task, quiet graphite visual direction, narrow
  navigation and app-owned permission policy; no runtime/account backend is selected.
- Identical 746-byte AGENTS.md/CLAUDE.md contain the canonical adoption router plus
  read-only trial boundaries. docs/app-kit.md is 2,010 bytes. The prompt asks for a
  source-grounded shell plan, without naming the contract path or MCP tool sequence.
- Catalog revision: `6cb2a851ee7cdcf9bb84f32956085a4ab632e9ce53585b70b276e5585a0c74f5`.
  Frozen intent SHA-256: `9fa40fd38601b3eb912e16c0754023864c156e1d0e9bd5321564405184c1de41`.
- Go 1.26.3 builds come from dirty base `d22871195401594f1083e2952d938ec5dfd655d3`.
  Binary hashes identify the actual trial artifacts; the base commit is not their full
  source identity. The unpublished snapshot is not a released consumer dependency.
- MCP runs under `env -i` with only a PATH, explicit binary and frozen kit root.
  The copied kit has no commercial corpus, embeddings or private design archives.
  Runtime discovery serves first-party metadata and docs; it does not execute an app.

## Actual host observations

| Observation | Codex | Claude |
|---|---|---|
| Explicit full consumer contract, manifest and AGENTS read | yes | yes |
| MCP guidance and byte-identical intent router | yes | yes |
| Both exact selected capability contracts/revision | yes; also bounded shell search | yes; selected IDs already in contract |
| Full shell.go, nav.go and appearance.go source | yes; also appearance.js | yes; not appearance.js |
| Successful final response and process exit 0 | 60,768 ms | 76,533 ms |
| Test-allowlist denials | none observed | design-asset search and CSS setup |

Automatic CLAUDE.md loading is an inference, not an independently recorded event:
Claude's first action opens the contract absent from the task prompt. Its final claim
to have checked CLAUDE.md exceeds explicit-read evidence. Both hosts explicitly read
the identical AGENTS router; the receipt claims that narrower fact.

Claude's two denied tools were advertised but not preapproved by this trial's allowlist.
The final answer disclosed the gaps. They are not successful MCP calls or kit defects.
A future CSS/reference trial should explicitly authorize those read-only tools; this
trial neither supplies licensed references nor tests their rendering.

Codex reported 226,693 aggregate input tokens, including 176,128 cached, and 1,415 output
tokens. Claude reported $0.7400445 with detailed token/cache accounting in the receipt.
These host accounting conventions are not directly comparable or a cost improvement.
Codex read several long design/ADR documents: scoped retrieval remains an optimization
opportunity, not a reason to bypass binding context or add another MCP server.

## Invocation scope

Codex used `exec --ignore-user-config --ephemeral --sandbox read-only --json`, no model
override, disabled hooks/plugins/apps/memory/browser/computer/delegation features, no
login shells, and invocation-local MCP configuration with four enabled read-only tools.
It retained existing authentication; no credentials were inspected or copied.
[Official non-interactive options](https://learn.chatgpt.com/docs/non-interactive-mode),
[configuration reference](https://learn.chatgpt.com/docs/config-file/config-reference).

Claude used `--setting-sources project`, strict invocation-local MCP configuration,
Read/Glob/Grep built-ins only, exact preapproved MCP reads, `dontAsk`, no permission
prompts, hook/automatic-memory disabling and no session persistence. Initialization
reported only appkit connected, no plugins, and model `claude-opus-5[1m]`.
`--restricted` was not used: installed-version source inspection found it suppresses
project instruction loading, which would confound this trial. `--bare` also changes
instruction/authentication behavior. [Official CLI reference](https://code.claude.com/docs/en/cli-reference),
[instruction loading](https://code.claude.com/docs/en/memory#how-claudemd-files-load).

Local managed-setting file checks and selected MDM-key type probes were negative;
that is not proof of absence of all managed/server policy. Hook-disabling flags cannot
override managed policy. No trace shows writes, app launches, browsing, installs or
delegation; incidental host authentication/cache activity is not claimed absent.
No real consumer or global configuration file was intentionally edited.
[Managed hook boundary](https://code.claude.com/docs/en/hooks#disable-or-remove-hooks).

## Separate deterministic checks and replay

The operator, not either host, ran `appkit adoption check --json` with explicit roots:
the positive specimen passed; a separate copy missing required shell.go failed with
exit 1; another copy selecting portable while retaining shell.layout failed with exit 1.
Neither failure was turned into a skip, and no live consumer was changed.

Replay the recorded observations without invoking a model or any network:

```sh
node docs/assessments/fixtures/host-routing/verify.mjs \
  data/adoption-evidence/2026-09-13-host-routing-eetm7M
```

The verifier checks full returned source, both MCP intent bodies/contracts, completion,
paired instruction bytes, explicit operator refusal reports and Claude connection state.
It is a local recorded-evidence checker, not transcript authentication, a hostile-input
sandbox or an automated fleet gate. Inspect raw commands/results when reviewing a claim.
The retained launcher records the original absolute paths; relocate/configure it explicitly
for a new trial. Replaying evidence does not rerun or re-prove today's host versions.

Verification: the frozen trial replays successfully. Independent copied-fixture controls
reject changed source bytes, a missing Codex final recommendation, an unrelated report
consumer/catalog and an altered reported file hash. MCP/adoption/CLI package tests pass;
the scoped doc check reports 54 documents, 329 relative links and zero failures, with
17 inherited oversized ADRs preserved. This is not a full fleet/intent wiring audit.

Remaining G4 evidence includes real consumer host setup, delivered pins, second-consumer
upgrade propagation and the release candidate. Browser/visual quality remains separate.
