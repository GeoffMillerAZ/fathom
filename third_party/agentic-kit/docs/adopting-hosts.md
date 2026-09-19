# Adopt native-host guardrails

This project-local, opt-in CLI supports Claude Code 2.1.270 and Codex CLI 0.154.0.
Hooks are partial guardrails, not a sandbox. Keep native sandbox/permission settings;
the installer does not change them or establish project trust. See the
[contract and limitations](designs/native-hosts.md) before declaring required coverage.

Build an executable at a stable, absolute location you own:

```sh
go build -o /absolute/path/to/kit-new ./cmd/kit-new
```

Create your own policy JSON, using
[`host-policy.json`](../cmd/kit-new/testdata/host-policy.json) as an example. Choose
actual tool names/patterns and project-relative forbidden paths; the example catalog
is synthetic. Any `deny_paths` rule conservatively denies arbitrary shell/stdin and
unknown tool formats. An empty `required_controls` certifies configuration only.
`stdin`, `hosted-tools` on Codex and `hook-failure-closed` on both hosts cannot pass.

Validate policy, inspect the exact before/after plan, then install using its digest:

```sh
cue vet cmd/kit-new/schema.cue /absolute/path/to/policy.json -d '#Policy'
/absolute/path/to/kit-new host plan -host codex -dir /absolute/project \
  -binary /absolute/path/to/kit-new -policy /absolute/path/to/policy.json
/absolute/path/to/kit-new host install -host codex -dir /absolute/project \
  -binary /absolute/path/to/kit-new -policy /absolute/path/to/policy.json \
  -expect DIGEST_FROM_REVIEWED_PLAN
/absolute/path/to/kit-new host doctor -host codex -dir /absolute/project
```

Replace `codex` with `claude` for that host. Plans expose complete existing config:
keep output private. Do not overwrite the pinned executable in place; uninstall and
reinstall around an upgrade. Config changes invalidate prior evidence even when the
installer-owned entries still match. Existing foreign settings and hook entries are
preserved; conflicts stop without adopting or deleting unowned files.

Doctor/probe print a versioned JSON receipt. Exit 0 means every declared required
control passed; exit 1 means incomplete/failed coverage; exit 2 denotes invalid CLI
usage. Expect exit 1 before live tests: local file and version checks do not prove
tool denial or instruction coverage. `probe` is also read-only and free: it executes
only version/help/features checks, never a model request. Use `-host-binary` to select
an explicit host executable. Set project trust through normal host UX after review;
do not use a bypass to make a test pass.

## Gather real observations

With explicit spend authorization, run isolated native sessions against synthetic
data. Preserve host JSON logs and independently inspect sentinel files. Record
allowed and denied tool/edit/shell attempts; startup and resumed sessions; nested
instructions/subagents; compaction; Stop; missing/disabled/malformed/crashing/timed-out
hooks and policy drift. A missing event, timeout, parse failure, or self-reported
success is unknown/failed coverage, not a supported control. Exercise each actual
native entrypoint: a direct `host guard` invocation only tests the handler.

Place bounded artifacts under the installed project and create evidence JSON:

```json
{
  "version": 1,
  "host": "codex",
  "host_version": "codex-cli 0.154.0",
  "config_digest": "REPLACE_WITH_DOCTOR_CONFIG_SHA256",
  "observations": [{
    "control": "tool",
    "status": "supported",
    "method": "live-host",
    "path": "host-evidence/tool-session.jsonl",
    "digest": "REPLACE_WITH_ARTIFACT_SHA256",
    "detail": "Describe invocation, observed native denial and independent sentinel result."
  }]
}
```

Use real 64-character lowercase SHA-256 values, not these placeholders. Keep each
artifact and evidence file below 1 MiB; remove secrets before final hashing. The
receipt labels this supplied evidence, not independently authenticated proof.

```sh
shasum -a 256 /absolute/project/host-evidence/tool-session.jsonl
/absolute/path/to/kit-new host probe -host codex -dir /absolute/project \
  -observations host-evidence/observations.json
```

Only controls actually demonstrated should be supported. Required controls without
evidence remain unknown. Repeat after version/config changes and preserve evidence
for the exact installed configuration, including any native project-local overrides.

## Remove or recover

```sh
/absolute/path/to/kit-new host uninstall -host codex -dir /absolute/project -dry-run
/absolute/path/to/kit-new host uninstall -host codex -dir /absolute/project \
  -expect DIGEST_FROM_REVIEWED_UNINSTALL_PLAN
```

Uninstall removes exact generated entries/content only; foreign instructions/hooks,
the executable, and directories remain. Changed generated content is a conflict:
inspect and reconcile it before retrying. After an interrupted install, doctor fails
and uninstall can remove the identifiable partial installation. Never recursively
delete a native config directory to recover.
