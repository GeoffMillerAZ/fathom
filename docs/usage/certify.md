### `certify` — the compiled certifications layer (vision 33 / ADR-043 stages 1-3)

```sh
fathom certify status [--service <id>] [--store <path>] [--json]
fathom certify gaps [--kind invariant|incident|spec|port|rule|behaviour] \
  [--service <id>] [--store <path>] [--json]
fathom certify manifest [--out FILE] [--check] [--service <id>] [--store <path>]
fathom certify orphans [--json] [--service <id>] [--store <path>]
fathom certify suggest [--limit N] [--mutation <report>] [--out FILE] [--service <id>] [--store <path>]
```

`certify status` reports the compiled `certifications` layer's target/live/
gap counts (a target is a declared-or-discovered invariant, incident, spec,
rule or port) and the join rate. `certify gaps` lists every declared target
with no live certificate, one line per gap with a remedy; `--kind` narrows
to one target kind; `--json` emits the versioned `fathom.certify.gaps/v1`
shape the Review Kit adapter's `fathom-certify` parses (schema string, field
names, and sort order — kind then id — are the wire contract; do not change
them without a version bump). Each target additionally carries `source`
(`{"file":..., "line":...}`, omitempty) — where the target ITSELF is
declared: a `// certifies:` marker's own comment line, an invariant's row in
`docs/intent/invariants.md`, an incident heading in
`docs/intent/incidents.md`, or a port interface's own declaration — nil when
`Gaps` could not place it. A target source that exists and could not be read
(the intent corpus, `incidents.md`, a port's source file) is named in
`unread` (omitempty; `certify status --json` too, and an INCOMPLETE line in
text): the counts are then a lower bound and no "no manifests found" skip is
claimed. `certify manifest` regenerates
`docs/certifications.md` from the layer; `--check` fails (exit 1) instead of
writing when the committed file differs — a hand edit to that file is a
defect. `certify orphans` (TC-22) lists `Test*` symbols that certify
nothing, reach no symbol changed in the last 180 days, and never failed or
flaked in any ingested run — a list, never an action; nothing is deleted or
flagged for deletion. `certify suggest` (TC-24) asks the decision port
(`[decide] provider = llm|typesafe`) one batched request per chunk of
unregistered `Test*` symbols — a strength choice plus an "asserts a real
behaviour" noul — and writes proceed-band answers to `--out`
(`workspace/artifacts/certify-suggestions.jsonl` by default, regenerated
fresh each run, never appended), rendered as a hint in `certify gaps`'s
table output; a suggestion is never a claim and is never written to the
store, and `provider = none` prints one line and exits 0. The strength
choice offers only the levels the test's evidence permits (JF-9): `asserts`
when the body calls an assertion or hands its `t` to a helper, `golden` when
it reads a golden or `testdata` file, `property` when its file imports a
property library (`testing/quick`, rapid, gopter), `mutation-killed` when
`--mutation` (or `$FATHOM_MUTATION_REPORT`) names a report with a killed
mutant; `smoke` and `conformance` are always offered. An answer outside those
levels is never suggested.

| Flag (`certify gaps`) | Default | Description |
|---|---|---|
| `--kind` | `""` (all kinds) | filter to one target kind |
| `--service` | resolved from cwd | Service ID |
| `--store` | `<repo>/workspace/db/fathom.sqlite` | store path |
| `--json` | `false` | Emit the `fathom.certify.gaps/v1` JSON shape instead of a table |

| Flag (`certify manifest`) | Default | Description |
|---|---|---|
| `--out` | `docs/certifications.md` | path to write the generated artifact |
| `--check` | `false` | exit 1 when the committed file differs, without writing |
| `--service` | resolved from cwd | Service ID |
| `--store` | `<repo>/workspace/db/fathom.sqlite` | store path |

| Flag (`certify suggest`) | Default | Description |
|---|---|---|
| `--limit` | `0` (no cap) | cap the number of unregistered `Test*` symbols considered |
| `--mutation` | `$FATHOM_MUTATION_REPORT` | a mutation report (go-mutesting output or fathom's JSON shape); with a killed mutant, `mutation-killed` is offered |
| `--out` | `workspace/artifacts/certify-suggestions.jsonl` | suggestions artifact path (regenerated fresh, never appended) |
| `--service` | resolved from cwd | Service ID |
| `--store` | `<repo>/workspace/db/fathom.sqlite` | store path |

`certify status` and `certify orphans` also take `--service`/`--store`
(resolved the same way) plus `--json`.
