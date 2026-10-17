### `intent` — the intent corpus, audited (ADR-026, **experimental**)

```sh
fathom intent audit    [--root .] [--json] [--kind conditions|triggers|invariants|unproven] [--fresh] [--top N] [--read-only] [--ledger PATH] [--store DB]
fathom intent doctrine [--root .] [--json] [--kind visions|bets] [--top N]
fathom intent adr-rules [--root .] [--json]
fathom intent export --guard [--root .] [--write | --check]
fathom intent index --check [--root .]
```

`index --check` holds the hand-written decisions index to `ADR.md`: every `## ADR-NNN` heading has exactly
one row in `docs/intent/decisions/README.md` and every row names a heading. Drift exits 1, naming each id and the
side that lacks it; `task validate` runs it. `fathom intent render --playbook` is retired (ADR-074): it prints a
deprecation notice on stderr, writes nothing and exits 0 for one minor release.

Reads a repo's Project Intent corpus (`docs/intent/` router, invariants,
conditions, sensitive surfaces, and the decision records the router points
at) and reports three things: **what it declares**, **what an instrument can
show**, and **what nothing shows**. It never edits the corpus, never
evaluates a condition an author did not annotate, and never scores a
repository.

`--read-only` renders the same audit and writes nothing: no clock, baseline or
debt row and no enforcement-probe cache row in `~/.fathom/intent-ledger.jsonl`,
and no beat. The Observatory's intent tab runs it this way. `--ledger PATH` names the
intent ledger a run appends to instead (`$FATHOM_INTENT_LEDGER`, else
`intent-ledger.jsonl` under `$FATHOM_HOME` or `~/.fathom`), so a scratch clone's audit
never writes the operator's ledger (*Scratch and test runs*).

Three tiers, all normal: a router-bearing repo gets the full audit; a repo
with decision records and no router gets the conditions from their
`Revisit when` trailers; a repo with neither is simply not adopting the
practice, which is a choice.

`audit` reports unproven claims by kind, each with a remedy:

| Kind | What it names |
|---|---|
| U1 / U2 | plan rows the verify gate proved failing, or claimed with no contract (read from the newest recorded gate run — never a fresh verify, which costs ~70 s here) |
| U3 | invariants at rung `VIOLATED`. `stated` and `reviewed` are listed as UNENFORCED, not as debt — turning an honest rung into debt is how rungs get faked |
| U4 | an invariant naming a test or gate step that does not resolve |
| U5 | conditions that fired and were never decided |
| U6 | incidents open past 30 days with no outcome |
| U11 | a record citing a path that has moved |
| U12 | a row claiming done inside a sensitive surface while pinning no named test |
| U14 | a sensitive-surface row whose globs match no file — it has silently stopped guarding |
| U13 | a guard entry whose intent provenance no longer matches the router, or a sensitive surface no guard covers |

**Computable triggers (CX-21, F9's computable subset).** A `Revisit when` or
`Promote when` condition carrying an instrument and a threshold in the bet-block
shape is evaluated by this run — and only by a run: the daemon never evaluates a
trigger on its own.

```markdown
**Revisit when.** the layers erode {instrument: arch_violations, threshold: ">= 3"}
Promote when: the rule holds {instrument: "arch_violations:a -> b", threshold: "== 0"}
```

Instruments: `arch_violations` (the newest ingest-time violation reading, summed
over its deny rules; read from `--store`, default the workspace store),
`arch_violations:<from> -> <to>` (one rule's count), and `plans_failing` (claimed
rows the verify ledger refutes at HEAD). A threshold is a comparison with a number
(`>=`, `>`, `<=`, `<`, `==`, `!=`). Each trigger reads `fired` or `not fired` with
its reading; one whose instrument is unknown, whose threshold is not a comparison,
or whose reading could not be taken reads `unevaluated` with the reason; a prose
trigger is listed as unevaluated, human-judged. `--json` carries them as `triggers`.

Every source it could NOT compute is named with the reason — an audit that
silently yields nothing is indistinguishable from a healthy repo. A file that
cannot be READ (as opposed to one that is absent) makes every count a named
lower bound, `≥ N (k source(s) unreadable: <path>: <errno>)`, and `--json`
carries `coverage.complete` (ADR-027).

`adr-rules` PROPOSES an archconformance deny rule for each dependency statement in
the decision records ("X never imports Y", "X must not import Y", "X calls Y, never
Z", "X imports neither Y nor Z"), each with its ADR, `file:line` and sentence, and
marks the ones `.fathom/rules/archconformance.cue` already holds. A human accepts a
proposal by adding it there; **fathom never writes that file** (CX-19, F24).

`export --guard` PROPOSES `.fathom/guard.cue` entries from the router's
sensitive-surface rows, stamped with per-row provenance so staleness stays
computable. It prints them; **fathom never writes that file** (law 15).

`doctrine` computes the doctrine plane's two views (vision 23 §2; ADR-026 as
amended 2026-09-25). **The vision promotion pipeline:** for every vision in the
series the router's docs map names, its stage (`landed`, `planned`, `accepted`,
`for-review`, `deferred`, `conditional`, `stated`, `superseded`, `rejected`,
`unstated`, or `unreadable`) from its header's `Status:` line or a status its
index entry leads with, the plans its header or index entry declares as its own
(a `Plan:`/`Plans:` label, or "… is its promotion") with their claimed rows
(claimed, not proven — `fathom verify` is the proof), any condition label it
carries, and its newest dated statement. `landed` is computed, never read: every
plan that carries the vision has no open row and at least one has claimed work —
a status word ("shipped"), a plan whose rows are all deferred or refused, and a
plan it merely cites or builds on are not that evidence (a landed claim with no
carrying plan is `stated`). Findings are `unstated`, `undated`, `stale` (for
review past 30 days with no plan; it names any decision the vision cites),
`broken-link` (a markdown link, a plan pointer, or a cited decision id that does
not resolve) and `unreadable` (the vision, or a plan that carries it, exists and
could not be read). Only this metadata is read — never a rule, a condition or a
count for the audit — and only from the series the router names: a
`docs/vision/` the router does not route to is not read.
**Goals as pre-registered bets:** each `**G<n>.**` goal of `scope.md` is a bet
when its `Done when:` carries `{bet: <name>, instrument: "<fathom verb | task
target | repo script>", threshold: "…", review: YYYY-MM-DD}` and the instrument
resolves; otherwise it is refused at load with what it lacks, and a bet whose
review date passed is listed as due. A bet block is not an oracle annotation and
never counts toward K0; one on any condition but a scope goal's is listed as
`misplaced-bet`. A vision home, plan, link target, decisions directory, Taskfile
or script that exists and cannot be read is named on the coverage manifest —
every count becomes a lower bound and `--json` carries `coverage.complete`
(ADR-027) — never an absent series, a broken link or a refused bet. An unknown
`--kind` exits 1 naming the accepted ones.

Three `fathom doctor` rows report the corpus, its debt, and the doctrine plane
(`intent`, `unproven_claims`, `doctrine`), and none can FAIL: not adopting the
practice is a choice, and honest debt is not a broken repo. Switch them off with
the `intent-corpus` capability — the verbs still answer on demand. Full design:
`docs/design/evidence-plane.md`.
