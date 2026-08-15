# The doctrine catalog — one registry, many enforcement surfaces

> Binding promotion of vision 23 §2's central refusal: ENGINEERING_PRACTICES
> (and laws, and runbook rules, and ADR assertions) are ACCEPTED as content
> and REFUSED as a fourth rule registry. Rows DP-1..4 in
> `docs/plans/custodian.md`; this document is DP-1.

## 1. The problem it solves

Fathom already has three enforcement systems, each excellent at its moment:
**obligations** (rules injected at edit time), **concerns** (config audited
against contracts), **conformance** (architecture rules checked against the
index). A "practices" registry would make four; a "laws" file five; runbook
checks six. Overlapping registries are the config-rot generator one level
up — the same rule stated three ways drifts three ways, and nobody can say
which statement governs.

The catalog inverts this: **one CUE registry of doctrine entries, where
each entry declares which enforcement surface(s) it compiles to.** The
surfaces stay exactly as built — nothing about how obligations inject or
concerns audit changes; what changes is where their *content* lives.

## 2. The entry model

```cue
doctrine: {
	no_raw_cp_deploy: {
		statement: "deploys go through `task deploy`; a raw cp truncates a running executable and loses the TCC grant"
		why:       "O25: fleet-wide daemon outage"
		law:       15 // optional: cites a numbered law
		enforce: {
			concern:    {kind: "forbidden_pattern", pattern: "cp .*libexec/fathom", where: "agent-config"}
			obligation: {paths: ["Taskfile.yml", "scripts/deploy*"], note: "deploy discipline — see statement"}
		}
	}
	store_pure_go: {
		statement: "internal/store stays pure Go (ADR-020/025) — CGO enters only via tree-sitter"
		adr:       "ADR-020"
		enforce: conformance: {from_pkg: "internal/store", forbid_import: "github.com/mattn/go-sqlite3"}
	}
}
```

- `statement` is the human truth, written once. `why` names the failure
  (an observation ID earns its place). `law`/`adr` cite provenance.
- `enforce` maps surface → surface-native config. Each surface's schema is
  the one it already has — the catalog does not invent a new rule
  language; it *addresses* the existing ones. An entry with an `enforce`
  field the surface cannot parse is refused at load (the custodian's
  uncheckable-wish refusal, applied to doctrine).
- An entry with NO `enforce` is legal but visibly labeled **unenforced
  doctrine** in every rendering — honored prose with its status named,
  never silently equal to the enforced entries (law 3).

## 3. Scoping and rendering

- Law 13, fourth use: `~/.fathom/doctrine.cue` (user-scope defaults,
  "everywhere unless a repo says otherwise") and `<repo>/.fathom/
  doctrine.cue` (governs). Same loader discipline as concerns: three-state
  load, `?:` refusal, all errors at once.
- **Compilation, not duplication:** `fathom doctrine compile` renders each
  entry's `enforce` blocks into the surfaces' own stores/files (obligation
  claims tagged with the entry ID; concerns registered under
  `doctrine:<id>`; conformance rules likewise). Recompile is idempotent
  and digest-gated. The surfaces never know the difference.
- **ENGINEERING_PRACTICES.md is a RENDER** (`fathom doctrine render`),
  marker-owned like every fathom-written file (law 15) — or, where a team
  insists on hand-writing it, the custodian audits it against the catalog
  (statements present, none contradicting). Never a parallel truth.
- Laws and runbook rules are entries like any other; the numbered-law
  citation (`law: N`) gives the observatory a "which laws have teeth"
  view: enforced / unenforced per law, computed.

## 4. What stays out

ADR *documents* stay markdown (context and consequences are prose);
`adr:` citations link entries to them, and DP-2's `fathom adr audit` is
the conformance view over entries carrying an `adr:` cite. GOALS stay
bets (DP-4) — a goal is not doctrine, it is a wager with an instrument.
Vision documents stay in the promotion pipeline (DP-3). No LLM anywhere
in the standing path.

## 5. Falsification, pre-registered

- If, after the catalog ships, most entries land with no `enforce` block
  (measured: unenforced ratio), the catalog is a prose garden with extra
  steps — stop and rethink before adding surfaces.
- If compiling doctrine into surfaces produces rules that drift from
  hand-written siblings (the exact disease it treats), the migration
  ordering is wrong: a surface must be either all-catalog or all-hand per
  repo, never mixed silently — the compiler must refuse mixed ownership
  it can detect.

## 6. Build order (when its implementation rows open)

Loader+schema (reusing the custodian loader wholesale) → `doctrine
compile` for the concern surface first (smallest, newest, cleanest) →
obligations tagging → conformance → the renders. Each step proven by
demonstration in this repo before the fleet sees it.
