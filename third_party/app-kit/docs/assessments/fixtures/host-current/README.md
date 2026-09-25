# Isolated current-source MCP host trial

Assessment tooling, not an installed onboarding command or a public package.
The [dated contract/results](../../2026-09-23-current-mcp-hosts.md) owns scope,
budget and exact evidence. Nothing here registers or upgrades a host.

## Cheap offline checks

```sh
node --test docs/assessments/fixtures/host-current/verify.test.mjs
node docs/assessments/fixtures/host-current/verify.mjs codex /private/packet/codex-host/stdout.jsonl /private/packet/expected.json
node docs/assessments/fixtures/host-current/verify.mjs claude /private/packet/claude-host/stdout.jsonl /private/packet/expected.json
```

These commands never invoke models, subprocesses or networks. They compare recorded
calls to the frozen SDK oracle, including exact returned documents/source and
independent catalog/inventory guards. They reject unexpected calls, missing
recovery, the synthetic source sentinel in final answers, and inconsistent Claude JSON envelopes.
They do not authenticate transcripts, check current installed registrations, or
prove the current filesystem still matches a historical invocation manifest.

## New acquisition (explicitly inference-bearing)

1. Review authority and budget. Create a new private canonical packet directory
   outside kit/consumer checkout ancestry. Preserve failed packets; never overwrite.
2. Archive the reviewed commit as `source.tar`; extract into `source/`. Build
   `appkit-mcp` there using pinned tools and the reviewed module graph, offline.
   Record source revision and build settings; an archive build has no VCS stamp.
3. Create `kit/` with the archive's `docs/` and `go.mod`. Do not copy `data/`,
   personal config, licensed archives or real consumer data.
4. Copy `collect.go.txt` into `source/cmd/host-trial/main.go`, then run it from
   `source/` with the absolute packet path. It refuses symlinked kit inputs,
   creates one synthetic SQLite source and records collector-time identities
   before its nine SDK calls. `expected.json` appears only after the oracle
   matches source documents/catalog/inventory and known synthetic source bytes.
5. Freeze this fixture directory under `packet/fixtures/host-current/` and
   `../bar-resolution-2026-09-22/browser-owned_process.mjs` at the corresponding
   relative path. Check Node and both installed host executable paths/versions.
6. Run each host once from the frozen runner, supplying absolute paths:

```sh
node /private/packet/fixtures/host-current/run.mjs /private/packet codex /absolute/codex
node /private/packet/fixtures/host-current/run.mjs /private/packet claude /absolute/claude
```

An optional final `RUN_ID` chooses a new exclusive output directory for a separately
approved diagnostic comparison. It does not authorize repeating an unchanged trial.
Retain the original frozen launcher and use a separately frozen directory for a
changed prompt. The current prompt explicitly maps inventory `revision` versus
`catalog_revision`; the dated assessment retains the earlier failed prompt too.

A second optional argument, `no-code-mode-host`, exists **only to retain the failed
Codex presentation experiment**. Do not use it as a workaround: Codex0.155.1 failed
before any MCP call when that host prerequisite was disabled, despite exit0. Normal
acquisition leaves presentation at `default`. See the
[bounded result](../../2026-09-23-codex-tool-presentation.md); do not install or
reconfigure a host automatically on this evidence.

The runner selects Sol/low or Haiku. It keeps existing CLI authentication, ignores
unrelated user MCP configuration, restricts the available/approved MCP tools and
uses a minimal environment for the actual MCP child. Host authentication/cache
bookkeeping is not asserted absent. No policy-bypass flag or automatic installer.

Each acquisition retains prompt/configuration, input/executable hashes, raw trace,
stderr, bounded process/output-drain observations and a source-free review or
failure. Inspect actual final responses and unexpected diagnostics independently.
Host traces stay private. No consumer, installed-binary, automatic instruction-
loading, model-reliability, UI-quality or adoption-acceptance claim follows.
