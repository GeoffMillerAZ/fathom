### `gen-status-cue`

Render an evidence-backed plan-verification status to a deterministic CUE file. Loads a
plan (`.cue` matching `schema/plan/schema.cue`, `.json`, or a `.md` checklist), verifies
each task against the index (the same checks as the `verify_plan` MCP tool), and writes a
`STATUS.cue`.

The plan file is the human-owned source of truth; `STATUS.cue` is regenerated, never
hand-edited — there is no bidirectional sync and no plan state stored in the database.

```sh
fathom gen-status-cue --plan <file> --service <id> [--store <path>] [--output <path>] [--concern <c>]
```

| Flag | Default | Description |
|---|---|---|
| `--plan` | (required) | Path to a `.cue`, `.json`, or `.md` checklist plan file |
| `--service` | (required) | Service ID to verify against |
| `--store` | `<repo>/workspace/db/fathom.sqlite` (see §CLI subcommands) | store path (.sqlite/.db) |
| `--output` | `workspace/artifacts/STATUS.cue` | Output CUE file path |
| `--concern` | — | Optional concern to scope contradiction detection |

```sh
fathom gen-status-cue --plan plan.cue --service my-service
# wrote workspace/artifacts/STATUS.cue: 4 tasks, 2 verified, 1 failing, 1 unknown
```

Output is `cue vet`-able and deterministic for a given plan, index and set of attested runs,
with one clock-dependent field: a `runtime_pass` criterion's `detail` names the age of the
row's newest attested run, read against the wall clock (`…, at commit 1a2b3c4d5e6f, 3h12m
old)`), so two renders apart in time can differ in that text and nowhere else. A static
check never claims a test passes: `runtime_pass` reads the newest attested run of a row's
certifying tests (TC-29), and is `unknown` when none was ingested.

---
