# Cold-arm harness

```bash
scripts/gdev/cold-arm.sh --subject /path/to/repo --service subject \
  --plans /path/to/plans.txt --ids /path/to/ids.txt --budget 536870912 \
  --out /path/to/new-arm --fathom /path/to/fathom
```

Requires Bash, Python 3 (standard library), Git, Go and curl supporting
`--fail-with-body`. Nothing is installed or fetched. Output must be a new
directory outside the subject. Lists have one entry per line, ignoring blank
lines and `#` comments. Plans are subject-relative paths. IDs must list every
checkbox-row ID in those plans exactly once. The corpus is never filtered.

Each invocation creates `<out>/home` as a fresh `FATHOM_HOME` and an empty store.
Inherited `FATHOM_*` settings are cleared. The harness writes only
`[tenancy.unmanaged].budget = "<bytes>B"` in that home's `config.toml`, the
[documented unmanaged budget](../../docs/configuration.md#the-budget-tenancy-user-scope-only).
It changes no admission mode, ladder, mark, host trigger or GC policy. The
daemon binds `127.0.0.1:0`; its private PID-matched record supplies the port.
The installed daemon is never addressed. SIGTERM stops the child. Current
unmanaged Fathom handles that signal and deliberately exits 143, recorded as-is.
Startup is bounded at 60 seconds, each request at 900 seconds, async ingest at
900 one-second polls, and shutdown at 30 seconds before a recorded forced stop.

**Use a new output for the second cold arm. Warmed reruns are not cold arms.
Do not raise the budget.** Keep binary, corpus and budget identical across arms.
Fresh Fathom state does not flush OS filesystem caches.

## Request order

1. Cold `ingest` on `/rpc/extended`, inside the measured daemon. Poll a returned
   run ID without requeueing. Degradation/refusal remains visible.
2. `verify --strict --json --url <private URL>` for every listed plan, in order.
3. Four CLI reads: `export receipt --symbol` (location, private store),
   `plans context` (neighborhood via daemon), `plans status` (all listed plans
   via daemon), `obligations --json` (private daemon's store).
4. Raw JSON-RPC on `/rpc/agent`: `initialize`, `tools/list`, then four tool calls:
   `symbol_context`, `review_diff` with `since`, `obligations`, `verify_plan`.
   Protocol setup is not counted as tool calls. Verification receives all
   supplied plans as combined Markdown.

The first plan-order `symbol:` anchor supplies the symbol probes. A scratch
`<out>/context.md` supplies that symbol as a `ctx:` pointer for CLI context,
without changing acceptance plans. No anchor means a named missing-symbol
probe. Review uses the parent commit and its changed paths. Initial/empty-diff
commits use HEAD and all tracked files, explicitly labeled in the receipt.

## Evidence

`arm.json` has fixed key order: `schema_version`, `identity`, `inputs`, `daemon`,
`requests`, `physical_footprint`, `estimated_memory`, `outcome`. It records binary
version/hash, subject commit/tree/dirty status, OS/arch, Go version, runtime
overrides, exact inputs and plan hashes. Every control retains argv/request,
exit, uncut stdout/stderr, bytes/hash and wall duration. Readiness attempts and
resource reads have separate ledgers and raw files. Named response refusals and
admission fields are retained, alongside the latest sampled tenancy state
(labeled as a sample, not a per-call admission decision). Complete, truncated,
refused, error and unknown are distinct; a complete failed gate can exit 1.

`resources --json --url <private URL>` is polled every 0.25 seconds from record
publication through shutdown, plus a final sequential read. The governor's
underlying cadence is two seconds. Daemon `footprint.now` and ring
`footprint.max` (including children) produce separate **sampled** peaks. macOS
uses `proc_pid_rusage` physical footprint; Linux uses the tenancy reader's
RSS-based measure. Pre-record startup and sub-sample spikes are unobserved.
Unknown samples, including a zero max with zero ring samples, become `null`.
Estimated/memo/heap fields exposed by `resources` are copied separately with raw
pointers. Absent estimates remain unknown. Budget proof uses live/ring readings,
never the post-exit config fallback.

`outcome: complete` means the control sequence returned fully with successful
exits and known physical samples. It does **not** qualify GDEV-01 or prove budget
compliance. Inspect observed budgets, peak, truncation, refusals and ingest
quality. Request failures stay in the receipt and do not stop later controls.
Harness operational failures/interruption exit nonzero and retain a receipt.

## Fathom snapshot and checks

The supplied lists select the 15 largest active plans by checkbox row count,
path order breaking ties, and every ID sorted. This HEAD has **1,852 IDs**,
rather than the historical 152. Regenerate from the repository root:

```bash
python3 - <<'PY'
from pathlib import Path
import re
ranked = []
for path in Path('docs/plans').glob('*.md'):
    if path.name == 'README.md':
        continue
    rows = [s for s in path.read_text().splitlines()
            if re.match(r'^\s*- \[[ xX~>!-]\] ', s)]
    ranked.append((-len(rows), str(path), rows))
selected = sorted(ranked)[:15]
ids = [re.search(r'\bid:\s*([^,}\s]+)', row)[1]
       for _, _, rows in selected for row in rows]
assert len(ids) == len(set(ids))
Path('scripts/gdev/fathom-15-plans.txt').write_text(
    '\n'.join(path for _, path, _ in selected) + '\n')
Path('scripts/gdev/fathom-ids.txt').write_text('\n'.join(sorted(ids)) + '\n')
PY
```

`bash scripts/gdev/cold-arm_test.sh` is Go-free and runs in `task scripts-check`.
It checks order, isolation, raw bytes, refusal/truncation, unknown samples,
injected reporting clocks and invalid/reused inputs using a fake daemon and a
generated Git subject. Real-binary smoke uses a generated three-row subject at
512 MiB. Sandbox typed-ingest/OS-reader degradation remains incomplete evidence.
The controller runs full Fathom cold arms outside the sandbox.
