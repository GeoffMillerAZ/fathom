# Fathom and Claude OS

Created: 2026-09-27 (summarized into the repository 2026-09-28; owner dispositions of the borrow list
added 2026-09-29).
Status: a reading of one public project's repository and README on 2026-09-27; not re-run. The
decisions it drove are the dated amendment of ADR-049 "The Claude OS comparison (2026-09-27)", futures
F55, F2 and F6 notes, and plan rows RH-11, RH-12, RH-X4, GL-23 and SR-X1.

## Verdict

Neighbours, not competitors. Claude OS helps Claude remember (notes, decisions, documents, found by
meaning); fathom helps an agent know what is true in the code and prove when work is done. They share
some plumbing and one small job, carrying context between sessions (about 25 percent overlap, an
*estimate* from reading both tool lists). Claude OS has no call graph, no rules at the edit and no
gate, and nothing in it threatens the question fathom answers. Keep building fathom, with a sharper
focus: ease of use is Claude OS's advantage and fathom's risk, so a friendlier first run matters more
than new features.

## Facts at a glance (read 2026-09-27)

| | Claude OS | Fathom |
|---|---|---|
| What it is | "Give your AI a memory": tell Claude to remember something and find it later by meaning | A code-intelligence and evidence engine: indexes what the code does, serves rules at the edit, checks that done is backed by the code |
| Language | Python 3.11 to 3.12, React UI | Go, one binary (CLI, daemon, MCP) |
| Runs | MCP server on port 8051, Redis, Ollama or OpenAI | One local daemon; no model or vector service required |
| Storage | SQLite with sqlite-vec (vector search) | Pure-Go SQLite, one store for a fleet of repos |
| MCP tools | About 30, nearly all about knowledge bases, memory and skills | About 25 on the agent surface (8 by default since ADR-053) |
| Code index | Symbols and signatures for Python, Ruby and JavaScript, files linked by shared names, ranked by importance | Symbols plus a call graph; Go calls resolved by the type checker; SCIP import for other languages |
| Gate | Status board only | `fathom verify` fails a push when a done item is not in the code |
| Community | 428 stars, 79 forks, a shared library of 36 or more community skills | Not yet published (F2) |
| Activity | Started October 2025, last change 2026-02-06 | Active daily |
| Licence | README says MIT; the repository has no licence file | FSL-1.1-ALv2 (ADR-052) |

All Claude OS figures are *measured* on 2026-09-27 (repository, README, the MCP tool list and its
structural indexer, and the GitHub API). The "depth of investment by area" bars in the source page are
a reviewer's judgment from reading both projects and are an *estimate*, not a measurement.

## Who does what

| Capability | Claude OS | Fathom | Note |
|---|---|---|---|
| Save a note in plain language | core | none since ADR-053 | The source page listed fathom's memories as "anchored, re-checked"; anchored memory was deleted afterwards (0 memories in any repository, ADR-053 decision 2). `fathom thread note` is the capture path |
| Search your own docs by meaning | embeddings | keyword plus receipts | Fathom chose no embeddings so every answer points at a line (ADR-041) |
| Pick up where the last session left off | yes | yes | The one shared job: both inject a start-of-session summary |
| Clean up old knowledge | yes (dedupe, archive) | staleness flags | Fathom flags stale claims; it does not merge notes |
| Find where a symbol is defined | 3 languages | many | |
| Who calls this, what breaks | no | yes | Claude OS links files by shared names, which is not a call |
| Impact across several repositories | no | yes | |
| Serve the team's rules at the edit | no | yes | ADRs and CLAUDE.md compile into cited rules; enforcement can deny an edit |
| Prove a task is done | status only | gate | Claude OS has a Kanban board |
| Review a diff with evidence | no | yes | |
| Easy first run | demo mode, polished installer | more to learn | Claude OS's advantage |
| Extra services to run | Redis plus a model | none | |
| Community and add-ons | skills library | packs | |

## Strengths

**Fathom is stronger at:** facts about the code; rules that follow you into the edit with citations;
proof, not status; many repositories at once; answers you can check (every answer names its source
line and says when something is unknown); fewer moving parts.

**Claude OS is stronger at:** being easy ("remember this" in plain words); free-form memory for
decisions that never reached code or docs; search by meaning; first impressions (demo mode, a polished
installer, screenshots up front); community.

## Lessons: which databases, and which not

The rule of thumb the reading produced: **one embedded file, many indexes, no extra servers.** When
fathom needs a new kind of search it adds an index inside SQLite (full-text first, vectors only if
measured) rather than start another database. Every extra server makes fathom a heavier guest on a
laptop, the exact problem ADR-047 fixed.

| Candidate | What it would do | Verdict | Why (and where recorded) |
|---|---|---|---|
| SQLite FTS5 | Ranked keyword search over docs, ADRs, plans, memories, sessions | accepted (the winning play) | No new process, answers still cite a line; adopted as N7 (ADR-049 decision 8, KR- rows) |
| sqlite-vec inside SQLite | Find a saved note under different wording | deferred, "maybe, narrowly" | Only for free-form memory, off by default, only if a reading shows it beats FTS; embeddings for code stay removed (ADR-041, vision 31); F13 |
| Redis | Job queue, expiring prompts, live events | refused | Fathom does all three in-process (refresh queue, live event stream to the browser, lifecycle ledger); a second server is one more thing to install, fail and use memory |
| DuckDB as a live store | Fast reports over logs and bench results | refused as a live store | Fathom moved from DuckDB to SQLite (ADR-020, ADR-025); an optional export for one-off analysis is fine |
| A graph database (Neo4j, Kuzu) | Store and query the call graph | refused | A service's graph fits in memory built from SQLite; the graph store keeps one copy per service and version; a graph server adds cost and no new answers |
| A time-series database (Prometheus, InfluxDB) | Track memory and traffic over time | refused, now named | Fathom keeps a 24-hour resource history itself and exports `/metrics` for anyone already running Prometheus. Covered before only by "any second store" in ADR-049 decision 12; the amendment names it |

## What not to copy

- **Polling.** Claude OS checks Redis once a second while waiting for a confirmation. Fathom waits on
  events, which avoids idle load on a laptop (consistent with P23, a guest on the operator's machine).
- **A model deciding what is true.** Automatic "insights" from conversations are handy, but a model's
  summary is a claim, not a fact; such items stay marked unverified until the code backs them
  (consistent with P24 and ADR-056: a model answer is never a receipt a stranger can re-derive).
- **Embeddings as the main way to find code.** Fine for notes. For code, exact answers (callers,
  impact, rules) are the reason fathom exists.

## The borrow list, and the owner's disposition of each

The source page listed eight features worth borrowing and rated their value. It recorded no verdicts.
The owner decided each on 2026-09-29 (recorded in the ADR-049 amendment of that date).

| Feature | Value (source) | Disposition | Reason | Record |
|---|---|---|---|---|
| A "remember this" plain-language memory skill that saves a memory attached to a file, symbol or commit | high | **refused** | Anchored memory was deleted for 0 memories in any repository (F-45, ADR-053 decision 2); `fathom thread note` already captures a note against files; a choose-to-save feature repeats the delivery-not-lookup failure | ADR-049 amendment |
| `fathom init --demo` on a sample repo, and `--dry-run` listing every file init would touch | high | **accepted** | Serves goal G16 (reach); `--dry-run` is checked by a test that its list equals the real run's touched set; `--demo` uses the fauxtown fixture fleet and downloads nothing; both are read inside the stranger-time reading RH-M1 | RH-11, RH-12 |
| One knowledge-health report over memories, rules and claims (duplicates, stale entries, things no longer true) | high | **merged** | Law 4: no new verb. It is one console view over instruments that exist or are planned (`staleness_report`, validity windows GL-16 and GL-17, rule effectiveness GL-18) | GL-23 |
| Warnings inside search results when an answer rests on stale or unproven facts | medium | **answered** | Already true: every query result carries a staleness report (I4, ADR-008) and receipts carry tier and drift (ADR-034) | ADR-049 amendment |
| An end-of-session summary offering a four-field thread note | medium | **deferred** | The continuity-thread bet has not read its sample; no prompts are added to a bet under measurement | SR-X1; trigger: SR-52 reads keep |
| "Ready now, deeper later" (a fast first index usable at once while the typed pass runs behind it) | medium | **deferred** | Adds background work before tenancy closes (ADR-049 decision 2) | RH-X4; trigger: RT-M1 PASS and RH-M1 showing cold ingest is at least half of stranger time |
| Console screenshots up front in the README, and MCP directory listings | medium | **deferred** | Publishing is F2 and the owner's Q4 call | F2 note |
| A browsable shared skills and packs library | later | **deferred** | Needs adopters first | F6 note |

## The partnership idea, and its disposition

The source page proposed making fathom the code engine under Claude OS: an adapter answering its
structural-index calls with fathom's JSON, a demo first, measured on a real repository, offered as a
friendly issue and a small pull request, keeping fathom's name, and reused for other memory tools. The
technical fit is real (Claude OS's weakest part is its code index, fathom's strongest).

**Disposition: deferred as a future, F55.** ADR-028 allows an agent-memory tool as a peer process and
never as a dependency. The project has been quiet since 2026-02-06, has no licence file, and fathom
could shrink into a component of a small tool; another daemon (Redis and a model) would sit on the
user's machine beside fathom's own. F55 promotes when such a project with a licence file and a release
in the last 90 days asks for fathom answers, or a user asks; it is dropped if Claude OS stays inactive
past 2027-03-31 and no second tool asks.

## Three things to keep in mind (from the source)

1. Fathom's own benchmarks are modest about savings: on reading tasks it performs about as well as the
   best tools at lower cost, not a large token-saving win. Its real case is the questions search cannot
   answer: impact, rules and proof.
2. Ease of use is the risk. A simple first run would matter more than new features (RH-11, RH-12).
3. Running both costs memory: Claude OS brings Redis and a local model; fathom's daemon stays inside a
   fixed budget (ADR-047), but the combined load on a small machine is *unmeasured*.

## What it drove

| Decision | Where |
|---|---|
| Neighbours, keep going; time-series database named as refused; polling refused; the eight dispositions | ADR-049, amendment 2026-09-29 |
| The fathom-as-code-engine future | F55 |
| Two accepted reach rows and one deferred row | RH-11, RH-12, RH-X4 (`docs/plans/reach.md`) |
| The knowledge-health console panel | GL-23 (`docs/plans/governance-learning.md`) |
| The deferred end-of-session offer | SR-X1 (`docs/plans/surface-reckoning.md`) |
| README screenshots and directory listings wait; skills library waits | F2, F6 notes |

## What this does not claim

That Claude OS is worse or better software; that its figures are current after 2026-09-27; that
fathom's savings exceed any tool's (they do not, per its own benches); that the 25 percent overlap is a
precise number; that any borrowed idea would work here (each is a plan row with its own reading).

## Sources (public, read 2026-09-27)

The Claude OS repository (brobertsaz/claude-os): README, the MCP server's tool list, and the structural
indexer `app/core/tree_sitter_indexer.py`; stars, forks, dates and languages from the GitHub API.
Fathom's facts come from its own repository, plans and benchmark readings.
