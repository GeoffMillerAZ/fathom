# Native host proof — 2026-09-13

Kit revision: `335bab3de13e8cf7f071881b1bac6553db794fa8`. Host versions: Claude
Code 2.1.270 and Codex CLI 0.154.0. These are bounded historical observations,
not general host certification. No live calls were made to prepare this bundle.

## Result and limits

- Fresh synthetic Go consumer: offline sourced-prose scenario in **105 seconds**;
  both host configuration/runtime receipts in **142 seconds**. Warm local tools,
  expert-agent-assisted setup, including guide reading/build/install; not human
  usability or cold-start timing. Human review time was not measured.
- Two native Claude print invocations, startup then resume: **$0.0461686 +
  $0.014327 = $0.0604956**, each capped at $1. CLI-reported token/list-price cost,
  not an invoice; the first includes auxiliary model cost. Codex model calls: zero.
- Actual hooks denied private Read/Write/Edit and both Bash attempts. Independent
  file hashes remained unchanged and all three prohibited destinations were absent.
  Native SessionStart startup/resume and Stop delivery were observed.
- Public Read worked. Public Edit's hook returned `{}`, but normal native trust
  refused the edit: project permission grants were ignored in the untrusted fixture.
  Allowed editing remains unknown; the strict Claude receipt stays **ready:false**.
- Codex live execution was not attempted: no verified USD hard cap in installed
  exec help, and normal fixture hook trust was not established. No bypass flags
  or global trust edits. Its event controls remain unknown, with the documented
  stdin/hosted-tools/hook-failure-closed limitations unsupported.
- Four actual-consumer receipts are **configuration/runtime only**. Their true
  ready values do not certify live enforcement, semantic truth, or successful edits.
  One startup/resume pair does not measure repeated-run reliability, nested
  instructions, compaction retention, timeout/missing-hook behavior, or sandboxing.

## Evidence and custody

| Files | Meaning / transformation |
| --- | --- |
| `claude-*.events.jsonl`, `sanitize-events.jq` | Whitelisted native event projection; retains tool inputs/results and hook stdout/stderr/exit codes, with original event indices. Drops model prose/thinking/signatures, model IDs, auth/runtime metadata, sockets and UUIDs. Per-file tool/hook IDs are deterministic ordinal labels; the same original session was checked and labeled `session-1`. Fixture paths become `$FIXTURE_ROOT`. |
| `trust-warning.txt` | Native stderr excerpt; fixture path and user configuration home replaced by explicit labels. Not an instruction to change trust. |
| `sentinels-{before,after}.sha256` | Verbatim original independent hash lists, not model output. |
| `sentinels.json` | Explicit independent snapshot summary, including absent destinations and native Edit limitation. |
| `strict-claude-receipt.json`, `synthetic-codex-receipt.json` | Verbatim original receipts. The former references the **original, uncommitted** live-bundle digest; that digest does not describe the sanitized events. |
| `consumer-receipts.json` | Four receipt projections; preserve host/version/time/config/help digests and every control status/basis. Add the reviewed advisory policy scope; omit repetitive detail. No private configuration is included. |
| `cost.json` | Native cost/usage values preserved; session ID omitted and model identities replaced by requested/auxiliary roles. IEEE-754 renderings retain the original numerical precision. |
| `setup-measurement.json`, `codex-live-limits.json` | Verbatim operator measurement/limitations records. |
| `ORIGINAL-SHA256SUMS` | Digest labels for retained original inputs, **not hashes of renamed/sanitized files here**. |
| `SHA256SUMS` | New hashes of this durable bundle; distinct from original input digests. |

Original files remain at `/tmp/native-host-proof.KInSiY/`; nothing was deleted.
Full consumer install/uninstall plans, existing instructions/settings, session
credentials, native model narratives/thinking, and runtime socket/home metadata
are deliberately excluded. SHA-256 proves integrity against a trusted inventory,
not host authenticity, semantic truth, or operator honesty.

## Reproduce without spending

From the kit root:

```sh
sh docs/evidence/native-hosts/2026-09-13/verify.sh
go test -race ./cmd/kit-new ./scenarios/sourced-prose -count=1
```

To regenerate an event projection from retained originals, run inside this folder:

```sh
jq -sc --arg fixture /private/tmp/native-host-proof.KInSiY/consumer \
  -f sanitize-events.jq /tmp/native-host-proof.KInSiY/artifacts/claude-startup.jsonl
```

Use the resume input similarly; compare stdout bytes to the committed event file.
Original sentinel bytes can still be checked using `sentinels.json`'s command in
the retained fixture. The observed native commands used `claude --print --verbose
--output-format stream-json --include-hook-events --setting-sources project
--effort low --model sonnet --max-budget-usd 1`, with tools Read/Edit/Write/Bash,
`--permission-prompts none`, empty strict MCP config, disabled slash commands and
Chrome. Invocation two added `--resume` with the original session ID. **Do not
rerun live commands without fresh spend authorization.**

## Machine-local ownership and reversal

Installed roots are `/Volumes/m2ext/code/geoffmilleraz/throughline` and
`/Volumes/m2ext/code/geoffmilleraz/resume_as_code`. Each has generated portions in
`.claude/settings.json`, `.codex/hooks.json`, appended host-specific stanzas in
`CLAUDE.md`/`AGENTS.md`, and `.agentic-kit/hosts/{claude,codex}/{policy,ownership}.json`.
Policy has empty deny lists and requires configuration/runtime only. Original
instructions and settings were preserved; these machine-local changes are uncommitted.

Binary: `/Volumes/m2ext/code/geoffmilleraz/agentic-kit/bin/kit-new-335bab3`, SHA-256
`d590f411f14058e54ac7dc2e39d7eefc526210b5669a021fa605c28983c88b09`. Do not overwrite
it while installed. [The adoption guide](../../../adopting-hosts.md#remove-or-recover)
describes reviewed `host uninstall -dry-run` then `-expect DIGEST`; recompute plans
after edits. Only owned content is removed; directories/binary/foreign content stay.
Private original plans remain outside this bundle for local recovery. Neither this
proof commit nor its verification commands install, uninstall, or alter configuration.
