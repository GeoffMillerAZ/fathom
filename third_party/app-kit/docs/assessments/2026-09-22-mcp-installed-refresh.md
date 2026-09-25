# Installed app-kit MCP refresh

Follows the authorized [host setup](2026-09-21-mcp-adoption-setup.md). Both effective
user registrations still select the same absolute stdio executable and `APPKIT_ROOT`:
`/Volumes/m2ext/code/geoffmilleraz/app-kit/bin/appkit-mcp`. Codex reports enabled;
Claude reports user scope and Connected. No host configuration, unrelated server,
consumer pin, secret, global guidance or running host session was changed.

## Earlier installed identity (superseded below)

Replaced the old clean `571068d` build with clean source commit
`c0ab87e38b0a9553aa8f2effedb2d21cb56a03ea`, `vcs.modified=false`, Go1.26.3.
The replacement includes current metadata, canonical capability pins and the signed
analytics capability guidance. Build once, not on normal MCP startup.

| Identity | SHA-256 |
|---|---|
| Installed executable | `57cec30552daa16e486b560a1219f6dd1000741933244694ec2be44aaa4fcca1` |
| Previous executable | `343c401bd3d2c976af4962764b0b9ed032897f26793f8d79d5917fd3ab6b00be` |
| Capability catalog | `212f0881d40d4fb441a3733a27bf5cbc81950211192241fd78024f6d153d40b7` |
| Authored quality inventory | `34c4bc0f7153ecb0397c1b69499d6f73a08744b1490419bf94cd092648dc16ae` |

The old executable is preserved byte-for-byte at
`/Volumes/m2ext/appkit-evidence/appkit-mcp-refresh.CNNg7j/previous-appkit-mcp`.
Replacement verified the old file still matched this backup, copied the candidate
to a private sibling staging file, atomically renamed it and compared installed bytes.
Rollback is an explicit replacement from that retained backup, not an automatic step.

## Proof and limits

The source-free `TestInstalledMCPProbe` passed before and after replacement from a
non-kit working directory, with a restricted child environment and clean shutdown.
It negotiated2026-07-28, discovered22 tools, checked actual build identity, intent
resource/tool bytes, catalog/quality revisions and the canonical capability pin.
Raw logs and candidate binaries remain under the private refresh directory.
No licensed source fetch, embedding request or Jev inference was part of these checks.

These are effective registration and real executable protocol observations, not new
Codex/Claude model-driven adoption trials. The separate September21 host trials
retain their narrower historical scope. Existing sessions may keep their old child
process; reconnect `appkit` or start a new session to load the refreshed executable.
No session was killed. Current host tool availability does not prove consumer UI
quality, source mirrors, adoption pins or any full quality gate.

[Optional Jev research](2026-09-22-jev-adoption-assessment.md) remains a separate,
provider-optional future; the owner's OpenRouter key was not read or used.

## Revision-guard follow-up (completed within scope)

Need: both hosts selected the older clean `c0ab87e` executable, which predates
the catalog revision precondition. The earlier installed probe compared quality
responses with each other, not with the current authored inventory.
Scope: strengthen that probe, demonstrate old-binary refusal and clean-candidate
acceptance, then refresh the same executable with a verified rollback copy.
Acceptance: exact current catalog/quality identity, stale revision refusal without
entry/pin leakage, recovery on the same process, clean shutdown and identical
installed candidate bytes. Budget: one focused test/review/install cycle; no model
trials, licensed retrieval, host configuration changes or session termination.
This is delivery verification, not consumer adoption or a quality-gate promotion.
Evidence: `/Volumes/m2ext/appkit-evidence/appkit-mcp-current.XyzqGs`; the original executable is
preserved there as `previous-appkit-mcp` and the clean `13fc281` source as
`source-before.tar`. Existing setup authorization covers this bounded refresh.

Both registrations were rechecked unchanged: Codex enabled stdio, Claude user scope
Connected, both selecting the same absolute executable. The replacement was built
before any probe/document edits from clean `13fc281849627ace1eaa94667c17932e9c0f24ea`,
Go1.26.3, `vcs.modified=false`. No production source changed in this follow-up.

| Revision-guard delivery identity (superseded below) | SHA-256 |
|---|---|
| Installed executable | `54758ecfaf55138dcc328e67d3ab1b21fe17bc3718c8dd2da74ec2ed607a9ffe` |
| Previous executable (rollback copy above) | `57cec30552daa16e486b560a1219f6dd1000741933244694ec2be44aaa4fcca1` |
| Capability catalog (unchanged) | `212f0881d40d4fb441a3733a27bf5cbc81950211192241fd78024f6d153d40b7` |
| Authored quality inventory | `033f1271d5aa6d825affec5aa7363c66599c17b60c4a40b893b993be4b725b0b` |
| Executed final probe source | `efe1474b2c781c17b8c4cd955be7f0809a0936eb55a1ffec1400309a4777f172` |

The strengthened probe now checks optional `expected_revision` discovery, identical
guarded/unguarded selection, a guaranteed different stale revision, no entry/pin in
structured or raw-text refusal, and same-process recovery. Quality list/detail now
match the source-loaded inventory, not merely each other. The initial RED also exposed
probe cleanup ordering: deferred context cancellation preceded session cleanup on
`Fatal`. Registering cancellation first with `t.Cleanup` closes the session before
canceling. Both executed harness revisions and logs remain retained.

`old-red-v2.log` fails only on the old binary's missing revision schema (exit1), with
no cleanup error. The same final harness passes the clean candidate
(`candidate-green-v2.log`, exit0) and the actual replacement path
(`installed-green-v2.log`, exit0). The latter negotiated2026-07-28, discovered22 tools,
matched current intent/catalog/quality identities and closed cleanly. Final package
race tests pass (`package-race-final.log`,33.435s); independent review found no blocker.
The old executable was checked against its backup immediately before atomic sibling
replacement; installed bytes equal the tested candidate. No licensed opt-in, model
trial, secret read, consumer migration, dependency or configuration change occurred.
Existing sessions still require reconnect/restart; the historical host trials above
are not trials of this new binary. No quality or adoption gate is promoted.

## Static-reading delivery follow-up

The [static discovery and exact-guide repair](2026-09-16-catalog-selection.md#static-reading-discovery-follow-up-2026-09-22)
changes embedded metadata and document routing. The current installed executable is
built from clean `470c9cfee33dae6f86f29f8a6c52c0b6f5dbae47`, Go1.26.3,
`vcs.modified=false`, SHA-256
`d9b56eb1c94df94aa8f393b495df3432b71853e5bee57f486e9e8f2d4b06ede9`.
Its catalog revision is `35110307653541f4d79e8f36e3c56b1f7fa7a435867ce7f43f51c36bd29b861b`;
the quality inventory and intent router identities above remain unchanged.

Evidence: `/Volumes/m2ext/appkit-evidence/appkit-static-discovery.77STFy`. It retains committed source,
build identity, exact binaries and raw probe logs. `candidate-probe.log` and
`installed-probe.log` pass; `previous-probe.log` fails on the old embedded catalog
revision using the same current probe. Both successful runs negotiate2026-07-28,
discover22 tools, match current catalog/quality/intent and close cleanly.
The previous `54758ec…a9ffe` executable remains byte-for-byte as `previous-appkit-mcp`.
Replacement checked that backup against the installed file immediately before an
atomic sibling rename, then verified installed bytes through the probe.

The existing Codex/Claude registrations were rechecked, not edited. No session was
terminated, and existing hosts still need reconnect/restart. New source-level MCP
tool/resource parity proves the guide's exact bytes; this executable probe is not a
new model-driven host trial, consumer adoption or static-host certification. No key,
licensed source, inference request, dependency or consumer pin changed. Root source
and this delivery note are separate commits; this note does not require a binary rebuild.
