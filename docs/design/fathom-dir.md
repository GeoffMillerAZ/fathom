# The `.fathom/` directory: four zones, one keyed schema, and tech packs

**Status:** accepted 2026-10-19 with amendments A1-A7 (ADR-061); proposed 2026-10-09. Plan: `docs/plans/fathom-dir.md`. Goal G35.

**The ask (owner, 2026-10-09).** "Are some of the patterns or features that we already have better
reworked into a .fathom dir paradigm and we can better organize or merge some of this? … as we add
new capabilities we don't sprawl the config and surface area further than it needs." It arrived with
owner decisions of the same day: **D11, tech packs** (a technology describes itself to fathom,
versioned with itself; the promotion of parked future F5, with the `provider` stratum renamed `tech`),
**K5** (projects describe themselves in `.fathom/` with the same format; committing derived index
snapshots is rejected), **K9** (an authored, anchored orientation entry set in the project pack,
readable without fathom, instead of maps a fresh clone cannot have) and **K10** ("pack" means too many
things: the consolidation absorbs the custodian's packs as built-in tech packs and app-kit's fathom-pack
as the first kit-authored one; one noun, one format).

This document inventories every file fathom reads or writes per repository and per user, names what
the inventory shows, and proposes a layout that reorganizes without breaking: four zones, one schema
entry point that new capabilities extend by key instead of by file, six merges, and the tech pack v1
format bound by adoption. Nothing here is built; every row of the plan is open.

## 1. Before designing (intent check)

- **Fired triggers.** F5 *Promote when* ("a named consumer plus one concrete provider an indexed
  service needs") fired 2026-10-09: the consumer is fathom (it vendors app-kit and agentic-kit today
  and adopts env-kit in the kits wave), the providers are app-kit and env-kit. ADR-024 *Revisit when*
  (1) fired with it. F5's *Drop if* ("platform modules cover the need") does not hold: a module is
  compiled once with model spend and mounted on a daemon (§7.5); a kit's deterministic guidance
  versioned with the kit needs neither. F27 (a committed team snapshot) is reframed by K5 (§12).
- **Decisions honored.** ADR-011 (org-specific content is configuration; a tech pack is the vehicle,
  fathom ships none). ADR-024 and its phase-O amendment (strata; platform modules are bound in
  `.fathom/modules.cue`, "no org rule reaches a repo that did not bind it"). ADR-048 decision 5 and
  I55 (a repository can neither install nor trust a dialect pack: a tech pack's `formats` only
  *references* a dialect). ADR-053 and law 4 (no net tool growth: no MCP tool is added). Law 10 (a
  capability is a verb first: the harness is a subcommand of an existing verb). Law 13 (user scope adds
  defaults; only project scope governs). ADR-018 (CUE is never a plan authoring surface or an engine:
  `.fathom/*.cue` is authored config, which ADR-018 decision 3 explicitly permits).
- **Invariants that bound the design.** I9 and I53 (hooks fail open inside their deadline), I35
  (at most 6 rules per edit, the 320-token payload ceiling), I38 (an unreadable file never looks
  absent), I42 (byte-identical behavior when an optional input is absent: the precedent for "no pack,
  no change"), I56 (fathom never downloads), I63 (fathom never removes a file it does not own), I31
  (`.fathom/` local state is a sanctioned in-tree exception today, which §3 narrows).
- **Principles.** P20 (a named consumer; generalize from two, not one: two providers exist), P22
  (measured: the kill criterion reads existing edit-hook telemetry at zero spend).

## 2. Inventory

Readers are cited as `file:line` in this repository at `a5448438`. "Compat" is the compatibility
surface of `docs/intent/scope.md`; "named 10-09" marks an item §11 adds to it because adopters already
commit it.

### 2.1 Project scope, `<repo>/.fathom/` (committed unless marked)

| Path | Purpose | Format, schema | Authored / generated | Reader | Compat | Overlap or finding |
|---|---|---|---|---|---|---|
| `config.toml` | parameters: daemon, decide, findings, tests, hooks, surface, connector, console, work, zones, profiles | TOML, `schema/config/schema.cue:391` | authored | `internal/config/config.go:427` (`Load`), plus 17 hand joins (e.g. `cmd/fathom/testrunhook.go:90`) | `[[surface.pattern]]` only; whole file named 10-09 | its booleans (`[hooks] test_run`, `[hooks] pin_check`, `[guards] enforce`, `[daemon] auto_refresh`) are a second switchboard (§3 e) |
| `capabilities.cue` | which features are on; list and composite budgets | CUE, embedded `internal/capability/capability_schema.cue:10` | authored | `internal/capability/paths.go:14` | yes | 10 registry ids; the scaffold comment still lists `narrate`, which the registry no longer defines |
| `concerns.cue` | the custodian's contracts over agent config; `packs:` enables shipped custodian packs | CUE, embedded `internal/custodian/custodian_schema.cue:9` | authored | `internal/custodian/load.go:58`, `cmd/fathom/configaudit.go:249` | yes | the shipped packs (`internal/custodian/packs/*.cue`: a `pack: {name, version, about}` header plus `concerns`) are already tech-pack-shaped; K10 absorbs them (§6 M7) |
| `guard.cue` | `guards` (pre-edit notes on path or command scopes) and `layer_rules` (compiled architecture rules) | CUE, `schema/guard/schema.cue:18` | authored; this repo's has 3 entries pasted from a kit | `internal/guardspec/guard.go:99`, `layerrule.go:96`; 8 hand joins (`cmd/fathom/guard.go:28`, `obligations.go:110`, `enforce.go:49`, `rules.go:128`, `doctor_phaseo.go:108`, `intentaudit.go:661`, `refactor_status_blockers.go:84`, `configpanel.go:87`) | yes | `guards` is the same shape a kit's `fathom-pack/guard-pack.cue` ships (§3 a); `layer_rules` overlaps `rules/archconformance.cue` (§3 d) |
| `modules.cue` | platform-module bindings | CUE, `schema/binding/schema.cue:28` | authored | `internal/bindspec/binding.go:110`, `cmd/fathom/doctor_phaseo.go:84`, `capabilityprobe.go:149` | yes | `cmd/fathom/configpanel.go:69` reads `.fathom/bindings.cue`, a name nothing writes (§3 c) |
| `layers/<name>/` | project semantic layer packs (`layer.cue`, `prompt.md`, `KILL.md`, `eval/`) | CUE and Markdown, `schema/semantic/schema.cue` | authored, or staged from a kit by `fathom layers install` | `internal/semantic/packs.go:157` (`loadUnion`) | yes | this repo's `ui-appkit` is a staged kit copy that differs from the pinned kit in all three files (§3 a) |
| `rules/archconformance.cue` | package-boundary rules over call edges (review, `arch_conformance`) | CUE, `schema/rules/archconformance/schema.cue:62` | authored | `internal/projectrules/projectrules.go:47,106` | yes (`rules/`) | second architecture-rule format (§3 d) |
| `risk.cue` | review risk thresholds | CUE, `schema/risk/schema.cue:19` | authored | `cmd/fathom/review.go:436`, `internal/mcp/review_tool.go:218` | named 10-09 | a tuning table outside `config.toml`'s provenance (§3 f) |
| `learning.cue` | rule promote and retire thresholds | CUE, `schema/learning/schema.cue:19` | authored | `cmd/fathom/claim_learn.go:247` | named 10-09 | same (§3 f) |
| `trace/<standard>.cue` | standard-clause mappings for `fathom trace export` | CUE, `schema/trace/schema.cue:17` | authored | `cmd/fathom/trace.go:238` | named 10-09 | knowledge; an org may later ship it in a pack |
| `observations/O<n>.md`, `observations.md` | friction journal (`fathom observe`); the single file is the legacy form | Markdown | written by a verb, then edited | `cmd/fathom/observations.go:23,27` | yes | none |
| `owned.json` | this checkout's ownership map of wired files and regions | JSON | generated | `internal/wiring/ledger.go:25,125` | no | local state in a committed directory; only this repo's root `.gitignore:47` hides it, `fathom init` writes no ignore for adopters |
| `leases.jsonl` | plan-row leases | JSONL | generated | `cmd/fathom/leases.go:40` | no | local state, ignored nowhere |
| `scip/*.scip` | SCIP indexes for typed edges | binary | generated (`init --scip`) | `cmd/fathom/ingest.go:553` (`resolveSCIPPaths`) | no | derived index data; its own `.gitignore` (`init_scip.go:153`) |
| `receipts.jsonl` | legacy: receipts moved to the state directory | JSONL | none now | `internal/wiring/receipt.go:47` writes the statedir | no | `.gitignore:46` is a dead line |
| `memories.jsonl` | retired with `internal/memory` (SR-13); purge refuses a leftover | JSONL | none now | `cmd/fathom/purgecmd.go:169` | no | none |
| `tenancy-allow.txt` | this repository's own tenancy-lint excuses | text | generated by a test | `internal/lint/tenancy/tenancy_test.go:22` | no (not a product file) | a fathom development artifact inside the product's per-repo directory (§3 i) |
| designed, unbuilt | `features.cue` (ADR-055), `incidents.cue` (ADR-058), `licensed.cue` (ADR-060), `doctrine.cue`, `trust.lock` | CUE | authored | none yet | no | five more file, schema, loader and doctor-row sets at today's rate (§3 g) |

### 2.2 Project scope outside `.fathom/`

| Path | Purpose | Reader or writer | Compat | Note |
|---|---|---|---|---|
| `.fathomignore` (root) | index excludes, gitignore syntax | `internal/connector/local.go:25,292` | named 10-09 | stays at the root: the `.gitignore`/`.dockerignore` convention |
| `AGENTS.md` stanza region; `@AGENTS.md` import region in a `CLAUDE.md` | always-loaded routing | `internal/wiring` via `cmd/fathom/init.go:190` | yes (ADR-059) | harness-defined location |
| `.claude/settings.json` hooks (or `~/.claude/settings.json`), `.claude/skills` with `.agents/skills` linked, `.gemini/settings.json` | harness wiring | `cmd/fathom/init.go:712,734,233` | yes (hook verbs) | harness-defined location |
| `docs/plans/*.md` | gated plans, grammar v2 | `internal/verify/markdown.go:72`, `schema/plan/schema.cue` | yes | read in place |
| `docs/intent/**`, `ADR.md` | intent corpus | `internal/intent/corpus.go:233`, `internal/adr/adr.go:85` | no | read in place; a project pack cites these and never copies them |

### 2.3 Per-checkout state outside the tree

`$XDG_STATE_HOME/fathom/<repo-id>/` (`internal/statedir/statedir.go:82,118`): receipts, session and
discovery logs, narrate and retro artifacts, call logs. `FATHOM_STATE_DIR` names it directly;
`[daemon] state_dir = "workspace"` restores the in-tree convention, which this repository uses.

### 2.4 User scope, `~/.fathom/` (`internal/fathomhome/fathomhome.go:39`; `FATHOM_HOME` relocates all of it)

| Kind | Files | Compat |
|---|---|---|
| authored defaults | `config.toml`, `capabilities.cue`, `concerns.cue`, `guard.cue` (notes and commands only, `internal/guardspec/guard.go:214`), `ignore` (`internal/connector/userignore.go:22`), `prices.toml` (`internal/cost/pricetable.go:38`) | `config.toml`, `capabilities.cue`, `concerns.cue` |
| written by verbs | `ignore.d/<service>` (purge), `suppressions.jsonl`, `purges.jsonl` | `ignore.d/<service>` |
| ledgers and logs, each with its own variable (`fathomhome.PerFile`) | `verify-ledger.jsonl`, `session-ledger.jsonl`, `intent-ledger.jsonl`, `assess-ledger.jsonl`, `hooks.jsonl`, `rule-outcomes.jsonl`, `preflight-outcomes.jsonl`, `console-views.jsonl`, `session-ratings.jsonl`, `session-stamps/`, `threads.jsonl`, `arm.json` | `verify-ledger.jsonl`, the telemetry shapes |
| machine state | `db/`, `daemon/<id>/`, `daemons/`, `logs/`, `cache/` (wazero), `fleet/`, `archive/`, trusted dialect packs (ADR-048) | the daemon records via doctor |
| cloud sessions | `~/.fathom-cloud/` logs (`docs/adoption/cloud-sessions.md`) | no |

### 2.5 Kit side

`app-kit/fathom-pack/` (`guard-pack.cue` with 4 consumer and 6 kit-self guards at v0.2, `layers/ui-appkit/`,
`README.md`) and `agentic-kit/fathom-pack/` (`guard-pack.cue`, `layers/agentic-kit-standards/`,
`scripts/`). Both are already on disk in this repository at the pinned commits
(`third_party/app-kit/fathom-pack/`, `third_party/agentic-kit/fathom-pack/`). Nothing reads them
automatically: adoption is "copy only the applicable consumer entries into `.fathom/guard.cue`"
(app-kit's README, step 1). `internal/console/knowledge.go:286` recognizes a staged kit layer by its
prompt path.

## 3. What the inventory shows

- **a. Copy-and-adapt drifts inside one repository.** This repository pins app-kit's pack at v0.2
  (`third_party/app-kit/fathom-pack/guard-pack.cue`), while its own `.fathom/guard.cue` carries three
  entries marked "from app-kit fathom-pack v0.1", and `.fathom/layers/ui-appkit/` differs from the
  pinned `layers/ui-appkit/` in `KILL.md`, `layer.cue` and `prompt.md`. The copy is the drift; the pin
  was current all along.
- **b. Local state has three homes.** Per-checkout state sits in `.fathom/` (`owned.json`,
  `leases.jsonl`, `scip/`), in the state directory (receipts, logs) and in `~/.fathom/` (ledgers keyed
  by root). `statedir.go`'s header (lines 12 to 15) names `owned.json` as in-tree footprint, and
  `fathom init` writes no ignore for it, so an adopter's `git status` shows fathom's state one
  `git add .` from being published (the W4 review finding recorded at `.gitignore:49` for `memories.jsonl`;
  those ignore lines protect this repository only).
- **c. One file, two names.** The literal `.fathom` is spelled by hand in 96 non-test lines across 65
  files; only `internal/capability/paths.go` declares itself "the ONE spelling". The Config panel
  checks `.fathom/bindings.cue` (`cmd/fathom/configpanel.go:69`) while the binding file every writer
  and doctor uses is `modules.cue`, so the panel always reports "no bindings file".
- **d. Two architecture-rule formats.** `guard.cue`'s `layer_rules` (import direction, boundary, no
  cycles, cross-service; directories; served at the edit and compiled to a Go test by `fathom rules
  gen-test`) and `rules/archconformance.cue` (from-package, to-package, allow, over call edges; served by
  review and `arch_conformance`) state the same kind of sentence in two schemas with two loaders.
- **e. Two switchboards.** `capabilities.cue` turns 10 features on and off; four `config.toml`
  booleans turn four more on and off, which `fathom capabilities` cannot show.
- **f. Tuning tables outside the tuning file.** `risk.cue` and `learning.cue` are threshold tables in
  their own CUE files, outside `config.toml`'s precedence, user defaults and `fathom config --explain`
  provenance, and absent from `docs/configuration.md`.
- **g. Growth by file.** Every capability so far has added a file name, a schema (8 directories under
  `schema/` serve `.fathom/` files, plus 2 embedded schemas), a loader, a doctor row and a docs section;
  five designed capabilities queue more of the same.
- **h. The compatibility list is partial.** It names the committed `.fathom/` files but omits
  `config.toml` as a whole, `.fathomignore`, `risk.cue`, `learning.cue` and `trace/`, all of which
  adopters commit.
- **i. A development artifact in the product directory.** `tenancy-allow.txt` serves this
  repository's own lint test; no product code reads it and no adopter has one.
- **j. One noun, many things.** "Pack" names custodian packs (`packs:` in `concerns.cue`), concern
  and project layer packs, kits' `fathom-pack/`, dialect packs (ADR-048) and now tech packs; the
  custodian's shipped packs already have the tech-pack shape (a `pack` header plus one section).
  K10 resolves it: one noun, one format (§6 M7, §7.1).

## 4. The layout: four zones

```
<repo>/
  .fathomignore                  Z1  stays at the root
  .fathom/
    .gitignore                   Z3  written by init (wiring-owned region): the local names below
    config.toml                  Z1  parameters (TOML); gains [risk] and [learning] (H2)
    capabilities.cue             Z1  the one switchboard
    modules.cue                  Z1  bindings: platform modules, plus exclude_packs for tech packs
    guard.cue                    Z2  guards + layer_rules (layer_rules gains call_direction, H2)
    pack.cue                     Z2  generated + formats + orientation (new keys; the file name is convention)
    concerns.cue                 Z2  agent-config contracts
    layers/<name>/               Z2  project semantic layers (platform stratum)
    trace/<standard>.cue         Z2  standard mappings
    observations/O<n>.md         Z2  friction journal
    (legacy, read with one doctor line each: rules/archconformance.cue, risk.cue, learning.cue,
     owned.json, leases.jsonl, scip/)
$XDG_STATE_HOME/fathom/<repo-id>/  Z3  per-checkout state: receipts, logs, and (H2) owned.json, leases.jsonl, scip/
~/.fathom/                       Z4  user scope, unchanged
<pinned kit>/fathom-pack/        the kit's tech pack, read in place (§7)
```

| Zone | Holds | Committed | Who governs |
|---|---|---|---|
| Z1 authored config | how fathom behaves in this repository | yes | the repository (law 13) |
| Z2 authored knowledge | what the repository says about itself: the project's own pack (guards, layer rules, generated, formats), contracts, layers, mappings, journal | yes | the repository |
| Z3 local and derived state | what one checkout accumulated; never shared, never a fact about the project | never | fathom (it owns these files) |
| Z4 user scope | machine defaults and the operator's notes; machine state | n/a | additive only (law 13) |

Rules: Z1 and Z2 are reviewed like code. Z3 is never committed and is either outside the tree or
named by `.fathom/.gitignore`; K5 holds here: no derived index data (snapshots, SCIP files, caches) is
ever committed. Z4 stays as it is: `FATHOM_HOME` already relocates it whole, `verify-ledger.jsonl` is
on the compatibility surface, and thirteen per-file variables address it; moving it buys no reader
anything.

## 5. One schema entry point, extended by key

`schema/project` defines `#Project`: one closed CUE value composing the existing schemas by top-level
key (`capabilities`, `budgets`, `bindings`, `exclude_packs`, `packs`, `concerns`, `guards`,
`layer_rules`, `generated`, `formats`, `orientation`). Today's files already use disjoint keys, so every existing
file is valid as written.

- **One loader, fault-isolated.** `internal/projectdir.Load` parses each `.fathom/*.cue` file on its
  own, validates each top-level key against its sub-schema, and returns one value. A syntax error names
  its file; a failing key names its file and key; every other key still loads (I9, I38). Two files
  setting the same key conflict by name. A key fathom does not know is a doctor warning, never an
  error, so a stray file breaks nothing. `risk.cue` and `learning.cue`, whose content is flat, are
  mapped by file name until H2 retires them.
- **One resolver.** `internal/projectdir` is the twin of `internal/fathomhome`: every project-scope
  name is spelled once, and a test refuses a new hand join. The `bindings.cue` drift (§3 c) cannot
  recur.
- **The keyed rule.** A new capability that needs project configuration adds a key to `#Project` and
  reads it through the loader. It adds no loader, no schema directory, no doctor row of its own (the
  one `project` doctor row reports every key) and no file name fathom must know; `fathom init` may
  scaffold the key into a file by convention. The designed `features`, `incidents` and `licensed`
  keys land this way. A capability that only switches something on adds a `capabilities` id, never a
  `config.toml` boolean.
- **User scope** is the same loader over `~/.fathom/*.cue` against `#UserProject`, which admits only
  the law-13 subset (capability defaults, custodian defaults, notes and command scopes).

## 6. Merges

| # | Merge | Horizon | Old path |
|---|---|---|---|
| M1 | A kit's consumer guards are read from the pinned pack (§7) instead of pasted into `guard.cue`; staged kit layers are read from the pin instead of copied into `layers/` | H1 | pasted entries keep working as project guards; doctor names a pasted entry whose `from <kit> fathom-pack vX` provenance matches a discovered pack |
| M2 | `rules/archconformance.cue` folds into `layer_rules` as kind `call_direction` (from and to package globs over call edges, `allow`), checked by the same code review and `arch_conformance` run today | H2 | read indefinitely; one doctor deprecation line with the equivalent `layer_rules` entry printed |
| M3 | `risk.cue` and `learning.cue` become `config.toml` sections `[risk]` and `[learning]`, gaining user defaults and `config --explain` provenance | H2 | read when the section is absent; a section wins; one doctor line |
| M4 | `owned.json`, `leases.jsonl` and `scip/` move to the state directory (Z3) | H2 | read when the new path is absent, written only to the new path; doctor names the stale old copy; fathom never deletes it |
| M5 | The four `config.toml` booleans become capability ids (`test-run-hook`, `pin-check`, `guard-enforce`, `auto-refresh`) sourced from those same booleans, so `fathom capabilities` shows them | H2 | the booleans stay valid; `capabilities.cue` wins on conflict and doctor names the conflict |
| M6 | `tenancy-allow.txt` moves to `internal/lint/tenancy/allow.txt` | H1 | this repository only; no adopter has the file |
| M7 | The custodian's shipped packs (`claude-code`, `codex`, `lefthook`, `taskfile`) become fathom's built-in tech packs: the same `pack` header and a `concerns` section, validated by `#Pack`; `concerns.cue`'s `packs:` list stays the switch that enables a pack's concerns (K10) | H1 | `packs: ["lefthook"]` keeps its meaning; `fathom config audit` output byte-identical |

Kept apart on purpose: `.fathomignore` (convention), `concerns.cue` as the repository's own contracts
(it governs agent config, not code), `config.toml` as TOML (operators edit it; its provenance
machinery exists), `trace/` and `layers/` as directories (collections), and user scope.

## 7. Tech packs v1: one format, three authors

### 7.1 The format

A tech pack is a directory of CUE data validated by `#Pack` (`schema/techpack`). Its header is the
`pack: {name, version, about}` block the custodian's shipped packs already carry (K10: one noun). The project's own
`.fathom/` is validated by `#Project`, which is `#Pack` without the header plus the config keys: one
format.

```cue
// <kit>/fathom-pack/pack.cue (a kit). A project writes the same keys in .fathom/*.cue, without the header.
pack: {name: "app-kit", version: "0.3.0", about: "Go and templ UI kit"} // required in a kit; version advisory, the pin is the truth
guards: [{                                      // the existing #Guard shape, unchanged
	scopes:    ["**/*.templ"]
	note:      "Role utilities only; tokens carry dark mode. Search the kit's components before writing markup."
	max_rules: 2
}]
generated: [{
	output:     "{dir}/{name}_templ.go"         // placeholders {dir} and {name} bind once, then fill source
	source:     ["{dir}/{name}.templ"]
	regenerate: "templ generate"                // rendered text; fathom never runs it
}]
formats: [{dialect: "templ", globs: ["**/*.templ"]}] // a dialect fathom ships or the user trusts (ADR-048)
// fixtures/ beside this file: guard scope positives and negatives, generated pairs, format samples
```

D11 named the sections formats, generated and rules. The rules section is the existing `guards` key:
the kits already ship it in that shape (`guard-pack.cue`), the edit path already serves it, and
`rules` is taken by `rules/archconformance.cue`. Two more sections have consumers today: `concerns`
(custodian contracts about a technology's own config files, which is what the four shipped custodian
packs are, K10) and `orientation` (§7.6, K9). A pack's `concerns` can fail `fathom config audit`, a
gate, so they apply only when the repository names the pack in `concerns.cue`'s `packs:` list, as
today; its guards, generated lines and formats are advisory and apply by the pin. A kit pack may not
carry `layer_rules`, `capabilities`, `bindings` or `exclude_packs`: a kit advises, only the
repository governs.
A kit's own kit-self guards move to the kit repository's own `.fathom/`, where they always belonged.

Examples the first two providers carry: env-kit `generated: [{output: "env.gen.json", source:
["env.cue"], regenerate: "bosun gen"}]`; app-kit the templ pair above. This repository's own pack
declares `cmd/fathom/ui/app.css` and `cmd/fathom/ui/tokens/theme.css` from `cmd/fathom/ui/input.css`
by `task ui:css`, and the tenancy allow list from its lint test, replacing a prose guard note.

### 7.2 Binding: adoption is the binding

A kit pack applies when the repository's own committed pin puts it on disk. Discovery is a closed set,
deterministic, without network and without `go.work` (which is local; a clean clone and the author's
checkout must read the same packs):

1. **Go modules.** For each `require` in `go.mod`, the module's directory as the go command resolves
   it offline: a `replace` to a local path, else `vendor/<path>` when `vendor/modules.txt` lists it,
   else the module cache at the required version. A `fathom-pack/` there with a `pack` header is
   a pack. A pinned module absent from disk is disclosed ("declared, not on disk"), never fetched (I56).
2. **CUE modules.** `cue.mod/module.cue` dependencies resolved the same way (env-kit arrives through
   an `env.cue` import).

3. **In-tree mirrors (A1, 2026-10-19).** A committed mirror directory carrying a pin file (`.vendor-commit`,
   ADR-070 decision 4), read offline, never through `go.work`. ADR-070 removed every kit `require` and `replace`
   from `go.mod`, so this is how fathom's own kits are discovered. ADR-061's revisit condition 2 is recorded as
   fired by ADR-070.

No other ecosystem in v1. Without a `pack` header a `fathom-pack/` is not a tech pack, so the
existing `guard-pack.cue` files change nothing until their kits opt in. The `tech-packs` capability
turns discovery off; `exclude_packs: ["<name>"]` in `modules.cue` drops one pack. Pack identity is
name, version and the sha256 of its files in sorted order; it tags every served line and joins the
ingest watermark when `formats` applies.

This extends the phase-O doctrine rather than bending it: a binding is a committed, reviewable act,
either a `modules.cue` entry (an org module) or a dependency pin (a kit). Nothing reaches a repository
that did not commit one.

### 7.3 Three authors

| Author | Where | Bound by | Trusted via | Served tag |
|---|---|---|---|---|
| the project | `.fathom/` | being the repository | the repository's own review (platform stratum) | none |
| a kit | `<pin>/fathom-pack/` | the dependency pin | its fixtures pass in fathom's harness, tagged kit-supplied | `[app-kit@0.3.0]` |
| an org | a platform module (compiled once, ADR-024 amendment), or a tech pack for an internal tool shipped as a pinned module | `modules.cue`, or the pin | the module's readiness level; or fixtures | `[module]`, or `[tool@version]` |

Fathom's own built-in packs (the four custodian packs today, M7) are embedded, authored and tested by
fathom like the embedded layers, and enabled by name in `packs:`.

### 7.4 Serving and budget

Pack guards and generated lines are served before the edit (`edit-hook --pre`), inside I35's caps:
the repository's rules first, bound module rules next, tech pack lines last, at most 2 lines per pack.
A displaced line is counted per source and disclosed (the O14 precedent). A generated line reads
"`<output>` is generated from `<source>` by `<regenerate>`: edit the source". Pack lines are advisory:
never `verified`, never verify-gating, never an override of a repository rule (as platform modules,
§3.4 of `docs/design/platform-modules.md`). `internal/analysis/integrity.GeneratedHandEdits` (CV-11)
reads the declarations before its same-directory `.proto`/`.cue`/`.sql` heuristic. `formats` lets
classification map a glob to a dialect id only when that dialect is embedded or trusted at user
scope; an unknown id is disclosed and ignored.

### 7.5 Gates

1. **Data only.** CUE values. Fathom never executes a pack's command (`regenerate` is rendered text,
   as F57's templates), never runs a pack's `scripts/`, never evaluates CUE beyond unification.
2. **Never fetches.** Packs are read from the pinned copy already on disk (I56). Freshness is the pin:
   a pack is exactly as current as the dependency the repository uses, so the upstream watermark the
   parked provider stratum needed (pack taxonomy, insight 2) is unnecessary; upstream drift is the
   dependency's drift (F58), not the pack's.
3. **Fixtures and a harness reading.** Each section ships fixtures; fathom's harness reads them as
   pure data at load (microseconds: glob matching and placeholder binding). A section with no fixtures,
   or a failing one, is not served, and doctor says so. A pack claims no level above its reading,
   tagged kit-supplied: the fixtures are the kit's, and there is no higher level in v1. A kit runs the
   same harness in its own CI with `fathom rules fixtures <dir>`.
4. **Per-edit cap.** At most 2 lines per pack, inside I35; repository rules keep priority.
5. **Byte-identical without packs.** A repository with no discovered pack produces byte-identical
   edit-hook, obligations and ingest output, pinned by a golden test (I42's shape).
6. **Kill criterion from existing telemetry, zero spend.** The rule-outcome log
   (`~/.fathom/rule-outcomes.jsonl`, GL-10) records each generated line as served, and the session's
   next edit as moved to the declared source or back to the output. Pre-registered (FD-M1): over 60
   days of owner traffic, KEEP when at least 20 fires occur and at least half are followed in-session
   by an edit to a declared source with at most a quarter followed by a re-edit of the output; KILL
   otherwise, including fewer than 20 fires (unused). A KILL removes the serving path in the same
   change; the declarations stay as data for integrity. Pack guards are read by the existing
   `RuleEffectiveness` classes: a pack whose every rule reads never-triggered or ignored for 60 days in
   every repository that discovers it is reported to its kit's owner; fathom never edits a kit.

### 7.6 Orientation entries (K9)

Zero exploration maps exist; they live in the local store, so a fresh clone or a cloud session has
none and `AGENTS.md` cannot route to them. The project pack instead carries an authored orientation
set: `orientation: [{anchor: "internal/verify/markdown.go", why: "the plan grammar parser; start here
for any verify question"}]`, where an anchor is a repository path or a `path#Symbol`. It is plain text
in the repository, so any client reads it without fathom; fathom verifies it: `fathom config audit`
fails an anchor that no longer resolves (I38's shape: a moved file is named, never silently dropped),
and the `AGENTS.md` router routes to it with one row (K8). A map earns promotion into the set through
ADR-056's reuse rule; nothing is generated into it. Kit packs carry no orientation in v1.

### 7.7 Not in v1 (futures with triggers)

Verbs a kit adds and what they prove (F62), framework edges as data mapping a call pattern to an
ADR-050 contract key (F63), and pack-carried maps (F64, behind F50's gate). Each waits for its own
named consumer.

## 8. Two horizons

**H1, additive (plan waves 1 and 2).** The resolver and the drift fix; `#Project` and the
fault-isolated loader with existing loaders reading through it, byte-identical; `.fathom/.gitignore`
for Z3 names, written through `internal/wiring` with a receipt and undo; the tenancy allow list leaves
`.fathom/`; the zone table in `docs/configuration.md`; tech pack v1 (schema, discovery, harness,
serving, integrity, formats, layer union with precedence embedded, then tech pack, then project; the
`tech` stratum value; the capability and the exclusion; the doctor row and `layers strata`; the
outcome kinds); the custodian's packs as built-in tech packs (M7, K10); the orientation set (§7.6,
K9); and this repository as consumer #1 (§10). Touches: ADR-024 (provider renamed tech and
promoted), ADR-048 (formats reference dialects only; I55 unchanged), ADR-053 (no tool added), the
phase-O doctrine (bindings by pin), the custodian (CC-12's packs keep their names and `packs:`
switch), ADR-056 (maps are promoted into orientation by its reuse rule, never generated into it),
F5 (promoted), F65 (parked; orientation covers today's need), I35, I42, I56, I9, I38.

**H2, merges with legacy reads (plan wave 3).** M2 to M5, and the designed `features`, `incidents` and
`licensed` capabilities landing as keys. Touches: ADR-044 (review and conservation read the merged
rules), ADR-055, ADR-058 and ADR-060 (their files become keys of `#Project`), ADR-056 and ADR-057 (their
switches surface as capabilities), O63 (state leaves the tree), I31 (narrowed: `.fathom/` holds no local
state once M4 lands), I63 (nothing is deleted).

## 9. Breaking changes

None. Every existing file keeps its name, its location and its meaning; every legacy read continues
(before 1.0 at least, and dropping one is its own breaking decision with a migration note). No
`fathom migrate` verb is proposed: the compatibility surface does not require one while the reads
stay, and each deprecation line prints the exact manual move.

Behavior that does change, none of it breaking: a `.fathom/*.cue` file with an unknown top-level key
draws a doctor warning (it was silently ignored); a repository whose pinned kit adds a `pack`
header gets new advisory lines at the edit (no kit carries one today, so no adopter's output changes on
upgrade; the capability turns them off). The compatibility surface grows by the `#Pack` and `#Project`
keys (including `orientation` and the `pack` header), the `.fathom/.gitignore` region, `fathom rules fixtures`' output, and in H2 the `call_direction`
kind and the `[risk]` and `[learning]` sections.

## 10. Fathom as consumer #1

Fathom already holds both kits' packs at their pins (`third_party/app-kit`, `third_party/agentic-kit`, in-tree
mirrors pinned by each `.vendor-commit`, ADR-070; discovery source 3 of section 7.2). This waits on an app-kit
release whose fathom-pack carries the `pack:` header and fixtures (A7): fathom never edits a kit repository. When app-kit's pack carries a header and fixtures: the three pasted
"v0.1" guards leave `.fathom/guard.cue`, `.fathom/layers/ui-appkit/` is deleted in favor of the pinned
layer, and fathom's own `generated` entries replace the prose "GENERATED by task ui:css" note. env-kit
follows when the kits wave makes it a dependency. The reading FD-M3 records what the edit hook served
before and after.

## 11. Documentation changes in this change

`docs/intent/scope.md` names the whole of `.fathom/config.toml`, `.fathomignore`, `risk.cue`,
`learning.cue` and `trace/` on the compatibility surface (they were already read from adopters' trees)
and adds goal G35; `futures.md` marks F5 promoted, reframes F27, and adds F62 to F65; `domain.md`
renames the stratum and defines tech pack; ADR-024 and the pack taxonomy point here; the decisions
index lists ADR-061.

## 12. Rejected

- **Committing derived index snapshots** (K5): churn, staleness and bloat; fathom's own cold ingest is
  72 s in the cloud setup and the platform snapshot caches it. A shared cache keyed by commit is F27.
- **One `.fathom/fathom.cue` for everything**: merge conflicts, one parse failure takes every
  capability down, and every adopter migrates for no reader gain. Keys give the single entry point
  without the single file.
- **Moving `.fathomignore` into `.fathom/`**, or user-scope state under `~/.fathom/state/`: convention
  and compatibility cost, no reader gain.
- **An explicit kit list in `modules.cue`**: it duplicates the pin and drifts from it, the failure §3 a
  measured; D11 decided adoption is the binding. `exclude_packs` is the only knob.
- **Copy-and-adapt as the adoption path** (`layers install`, pasted guards): §3 a.
- **A registry, a fetch verb, or a pack host**: I56 and F2.
- **A `fathom pack` verb or an MCP tool**: law 4 and law 10 are met by `rules fixtures`, `layers
  strata` and one doctor row.
- **Kit packs carrying layer rules, capabilities or bindings**: a kit advises; only the repository
  governs (law 13's shape).
- **A generated map readable without fathom, now**: parked as F65 until a consumer without MCP and an
  A/B exist; the authored orientation set (§7.6, K9) serves the readable-without-fathom need today.

## 13. Open questions for the owner (answered 2026-10-19, ADR-061 A3)

1. Tech packs on by default when discovered (this design), or off until `capabilities.cue` turns them
   on? Default-on matches "adoption is the binding"; default-off matches I60's posture for
   decision-port sites, which do not apply here (no judgment is made).
2. Per-pack cap of 2 lines: the platform-module default. Raise only on FD-M2 evidence?

Answers (2026-10-19, owner-architect): Q1 on by default when discovered (adoption is the binding; no judgment is
made, so I60 does not apply). Q2 a per-pack cap of 2 lines until FD-M2 evidence says otherwise.

## 14. Amendments at acceptance (2026-10-19)

A1 discovery source 3 (section 7.2). A2 `auto-refresh` stays user-only (I64): FD-34 surfaces `test-run-hook`,
`pin-check` and `guard-enforce` only. A4 pack identity joins the ingest watermark after RT-M1 PASS (FD-17). A5
`[risk]` and `[learning]` fields each get a `config.ScopeOf` class (FD-31). A6 repository-scope readouts rules (ADR-071
decision 10) land as a `#Project` key, not a new TOML file name (section 5). A7 consumer #1 waits on an app-kit release
carrying the `pack:` header and fixtures (section 10). The proposed invariants (a) and (b) are I67 and I68.
