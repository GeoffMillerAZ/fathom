### `layers`

List, show, or install semantic-layer packs — the embedded catalog (`obligations`,
`findings`, `adr-rules`, `spec-bindings`) unioned with a project's own `.fathom/layers/`.
Presence of a pack **is** its enable state for compilation; there is deliberately no
enable/disable verb.

```sh
fathom layers list    [--root <dir>] [--store <path> --service <id>] [--json]
fathom layers show    <name> [--root <dir>]
fathom layers show    <name> --json [--store <path> | --url <daemon>]
fathom layers install <dir>  [--root <dir>] [--force]
fathom layers strata   [--json] [--store <path> | --url <daemon>]
fathom layers coverage [--json] [--store <path> | --url <daemon>]
```

The console's Knowledge verbs (vision 35, OC-27..OC-29) print the projections
`/layers` and `/layers/{layer}` render, fleet-wide: `strata` places every layer in its
stratum (dialect, provider — parked — platform, concern) with current claims, its
lifecycle, real compile runs and their cost, plus fleet totals and the claim-event
lifecycle; `coverage` is current claims per project and layer, a cell nothing compiled
absent ("not compiled", no number in its JSON) and a compile that left no current claim
a measured zero; `show <name> --json` is the layer page — its definition as the fleet
resolves it, every compile run with its kind, cost and accepted/rejected counts, the
rejection reasons, each project's uncompiled scopes with the resume command (`fathom
compile --layer L --service S --store <the store> --continue --dry-run=false`:
`--continue` runs in direct `--store` mode only, and a compile spends only with
`--dry-run=false`), and the dry-run command. A refused run's estimate is never counted
as spend, and a layer with no real compile run has its accepted and rejected counts and
its uncompiled scopes absent (`uncompiled_count`), never a measured 0 or "fully
covered". A layer whose pack could not be read (a broken project pack, or an unreadable
`.fathom/layers` of a project holding it) is listed under `unplaced`, not guessed into a
stratum, and `totals.layers` is a lower bound naming any unreadable catalog. They read the
store the daemon at `--url` recorded when it bound (its `~/.fathom/daemon/<id>/record.json`
record, OC-2, RT-166), else `[daemon].store`, else the workspace default — mounted read-only, and a
missing store is refused rather than created; text output names the store and the rule
that chose it. `show` without `--json` is unchanged.

`list --json`: `{root, store_read, layers: [{name, source, overrides, kind,
tier, stratum, evaled, drift_tracked, corpus_include, load_error?, claims?,
last_compile?}], orphans?}` — `claims`/`last_compile` appear only when
`--store`/`--service` read the store (`store_read: true`); a broken project
pack is an entry with `load_error`, never dropped.

`list` prints every visible pack (name, source, kind, tier, stratum, eval status, drift,
corpus, and — with `--store`/`--service` — compiled claim counts and last-compile date,
so eval-vs-production is never conflated). `show <name>` prints one pack's full resolved
definition and prompt provenance. `install <dir>` validates a directory against `#Layer`
and copies it into `<root>/.fathom/layers/<name>/`; `--force` replaces an
already-installed pack of the same name (whole-directory replace, never a merge). See
[docs/layers.md](../layers.md) for the catalog reference.

---
