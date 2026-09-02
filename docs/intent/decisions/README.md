# Decisions

Index of decision records. The records are the `## ADR-NNN` sections of `ADR.md` at the repo root
(one file, append-only, its own numbering); `ADR-` here means any decision record, not
only architecture. New records append to `ADR.md` with the same shape plus a
`**Revisit when.**` paragraph; each row here is the short pointer, the record holds the
full condition. Superseding edits only the old record's `**Status.**` line.

Record a decision when any holds: reversal is expensive; it constrains code outside its
own package; it creates or removes an invariant, non-goal, or boundary; it accepts a
notable tradeoff or dependency; it defers something significant and names what would
un-defer it; it rejects an alternative someone will propose again; without the record a
future reader would rebuild this differently. Otherwise: a comment, a test, a commit.

Status: proposed · accepted · rejected · deferred · superseded by <id> · deprecated.

| Id | Status | Type | Title | Revisit when |
|---|---|---|---|---|
| ADR-001 | accepted | architecture | Three-layer architecture (connector → fusion → query) | none identified |
| ADR-002 | accepted | architecture | Connector SPI with a deterministic Faux connector | none identified |
| ADR-003 | superseded by ADR-020, ADR-025 | data | Storage: DuckDB | (superseded) |
| ADR-004 | superseded by ADR-013; revived in part by ADR-021 §4 | architecture | CGO-free tree-sitter via wazero; Go via `go/parser` | (superseded) |
| ADR-005 | accepted | architecture | Hash-gated, deterministic 3-pass fusion pipeline | none identified |
| ADR-006 | accepted, amended 2026-08-26 | product | Concern is a required parameter of `Engine.Query`, not a filter every tool honors | a non-engine tool's result sets would halve under scoping; a second concern-shaped filter proposed |
| ADR-007 | accepted | domain | Evidence tiers reused from agentic-kit | none identified |
| ADR-008 | accepted | product | Staleness is always reported | none identified |
| ADR-009 | accepted | dependency | Depend on agentic-kit as the platform substrate | none identified |
| ADR-010 | accepted, amended by ADR-014 | architecture | MCP server: stdio + HTTP, tool groups, profiles | `set_profile` retired by the ADR-014 amendment; the result-cache clause is dead code (`invariants.md` I21) |
| ADR-011 | accepted | strategy | Generalized / org-agnostic, open-source-ready | none identified |
| ADR-012 | accepted | process | Test-driven development with golden fixtures | none identified |
| ADR-013 | accepted | dependency | Tree-sitter via CGO bindings (supersedes ADR-004's mechanism) | a pure-Go runtime matures — partly fired (XT-6b, F1) |
| ADR-014 | accepted, amended 2026-07-16 and 2026-08-26 | architecture | Path-scoped tool surfaces; execute-and-flag; group surfaces deleted (EP-23) | a multi-tenant shape re-arms `strictSurfaces` |
| ADR-015 | accepted | strategy | Rename to fathom; two strata | fleet migrated → remove back-compat shims |
| ADR-016 | accepted, amended 2026-07-16 | architecture | Semantic layers: governed claims beside the index | first non-citable claim kind; deferred pieces' revival evidence; vectors need a SQLite re-check |
| ADR-017 | accepted | data | Born-portable stores: repo-relative anchors, domain-neutral freshness | a non-engineering module needs tier profiles; manifest watermark cost |
| ADR-018 | accepted; pilot KEEP 2026-08-26 | process | Plan grammar v2 + reconciler verbs: SDD state is derived, never driven | machine-evaluated conditions now v1 under ADR-026 (opt-in `{oracle: …}`) |
| ADR-019 | accepted | dependency | In-tree mirror of agentic-kit for kit-free distribution | kit access becomes universal |
| ADR-020 | accepted | data | Store backend: pure-Go SQLite (WAL) replaces DuckDB | none identified |
| ADR-021 | accepted | architecture | Extraction v2: language truth, tool identity, typed edges, tree-sitter on wazero | none identified |
| ADR-022 | accepted | dependency | MCP server stays bespoke, not `kits/mcp-server` | the kit gains surface/group/prompts/resources; any hosted deployment reopens the accepted debt |
| ADR-023 | accepted | dependency | Evaluation stays bespoke, not `kits/eval-runner`/`qa-harness` | an LLM-judged eval lane appears (FA-12) |
| ADR-024 | accepted, amended phase O | architecture | Pack taxonomy adopted as vocabulary, not machinery | provider pack consumer (TX-3); concern pack admission case (TX-4) |
| ADR-025 | accepted | data | DuckDB backend deleted outright (SS-8 executed) | XT-6b fires (CGO drops); `fleet.duckdb` step done |
| ADR-026 | accepted | architecture / process | The evidence plane: intent corpus, unproven claims, change records | any kill clock K0/K1/K3/K5/K-plane; the kit refuses annotations (Q18); the practice ships its own evaluator; a second tool wants intent as data; plan trigger vocabulary repeats (F9); decision latency measures as the bottleneck |
| ADR-027 | accepted | architecture / process | Coverage before conclusion — an instrument may not degrade toward "healthy" | 60 days with no incomplete manifest in a real run, or every incompleteness shares one cause a narrower fix covers |
| ADR-028 | accepted | dependency / strategy | External agent memory is a peer process, never a dependency (OpenViking assessed) | a permissive relicence; the project ships code indexing; an adopter reports a conflict the guide misses; `pack_context` traffic justifies tiering; fathom adopts a root LICENSE |
| ADR-029 | accepted | architecture / strategy | Harness adapters are data behind one seam (`internal/harness`); a second harness's inject tier ships `unverified` behind an explicit `init --harness` flag; measurement stays Claude-only; the kit gets the contract on FA-22 | FA-22 fires; a harness exposes transcripts; Claude Code stops being the interchange envelope; a descriptor reaches 60 days unverified; `hooks_firing` verifies Codex on a real install |
| ADR-030 | accepted | process / architecture | A lexical oracle (`{oracle: lexical, path, pattern}`) proves non-code plan rows with ADR-026's evaluator | Q20 moves the grammar to the kit; >30% of lexical rows point under `docs/vision`/`docs/design`; a second evaluator appears |
| ADR-031 | accepted | process / strategy | Bench credibility: derived third-party Go corpus, a `none` arm with a leak rule, a precision term, paired statistics owned by agentic-kit, a free regression gate | the none arm passes >50% of the home corpus; a public number is requested before the repo is public; SWE-Explore accepts the registration; the kit's stats package gains a third consumer |

## Decisions recorded elsewhere

Pass the threshold, were not written as ADR sections; point, do not duplicate.

| Where | What it settles | Revisit when |
|---|---|---|
| `docs/vision/11` §2, `12`, `13`, `14`, `20` §6 | Doctrine laws 1–15 (summarized in `principles.md`) | each law's own text |
| `docs/design/product-vision.md` §3 | Use-case verdicts and the standing non-goals | the entry conditions in its table |
| `docs/design/semantic-layers.md`, ADR-016 | D-EM2 "the store is the module"; expert-module commitments | ADR-016's trailer |
| `docs/design/coding-agent-excellence.md` §5.3 | Fail-open/fail-closed policy; no grep-blocking, by decision | ≥30% of greps index-answerable |
| `docs/vision/20` §5, `docs/vision/24` | The owner-ask gates: accepted / reframed / rejected with reasons; the dispatch boundary; saved-query router declined; heartbeats refused | never (dispatch boundary) |
| `docs/vision/23` | ADRs are first-class ("decided means honored"); a fourth registry refused | `fathom intent audit` over ADR trailers is `adr audit` (ADR-026, EP-9); the CUE catalog is output only (EP-30) |
| `docs/handoff/repo-strategy.md` | Monorepo now, split on an enforced boundary; the CI/forge repo trigger | first non-fleet consumer |
| `CHANGELOG.md` 2026-08-12 (MI-5), `docs/design/production-tokenomics.md` | The A/B unit is arm-homogeneous; ≥10 turns, ≥20 units per arm | never after data exists |
| `docs/design/scope-and-purge.md` | Six purge/scope defects made structural (V1–V6) | a seventh defect class |
| `docs/design/evidence-plane.md` §4 (D2), ADR-026 | Claimed rows with no contract are honesty debt, listed with a remedy; CI `--strict` follows scope G6 (closes thread `th-c214dd26`) | G6 done |

Session-scale rejections live in plans as `[-]` rows with `disposition: refused|deferred`
and a `trigger:`; they die with the plan.
