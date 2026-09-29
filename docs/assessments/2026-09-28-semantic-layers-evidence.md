# Semantic layers: what the evidence says

Created: 2026-09-28 (summarized into the repository the same day).
Status: an audit of fathom's own layers from the scorecard (2026-09-27) and a copy of the live fleet
store taken 2026-09-28; where the two differ the scorecard figure is quoted and the live figure is
named. Not re-run. Decisions it informs: ADR-016 (claims and layers), ADR-037, ADR-038, ADR-049 decision
9, ADR-053, ADR-056 (O120, F37) and ADR-057 (served is not kept).

## Verdict: keep, then improve

Rules with receipts are cheap to keep and not yet proven to change behavior. A layer turns the
documents that govern a repository (ADRs, CLAUDE.md files, plans) into checked, cited claims and serves
the right ones to an agent at the moment it edits. What worked, measured: 4,025 live claims cost
$27.50 to compile, the gate threw out 56 percent of the claims the model proposed, and 97 percent of
citation drift healed with no recompile. What did not: **served is not the same as obeyed** (in the
second guards pilot, agents given the rules broke them as often as agents without, 4 violations each,
at 1.56 times the tokens per clean success), selection is the bottleneck (about 6 rules served out of
about 140 eligible), a fresh claim is not necessarily in force, and no person has judged a single claim
yet, so nothing learns. Nothing measured shows a layer changing agent behavior on average; eval
precision is measured on tiny fixtures and **no live precision figure exists**.

**The scorecard's calls** (F-26 to F-33, F-64 to F-68): keep the three live packs (obligations,
adr-rules, spec-bindings), the compiler, the edit hook and rules-to-tests; improve the obligations
tool, guards, claim triage, the findings loop and the ui-appkit pack; ditch the expert modules
(SR-17, ADR-053).

## What a layer is (in one paragraph)

A layer is a folder of data (a manifest naming the documents to read, a versioned prompt, eval
fixtures, a kill sheet), not Go code. A model reads each changed section once and proposes claims; a
fixed gate rejects the bad ones (a claim whose cited lines do not re-resolve, whose scope matches no
indexed file, whose scope is over-broad, whose nested scope escapes its directory, or that is
malformed); what survives is served as a lookup with no model in the loop: at the edit (a hook adds at
most 6 rules under a 320-token ceiling), in answers (`obligations`, `staleness_report`, `layer_coverage`,
with raw sections when a compile was cut short, ADR-037) and as guards (`enforce: true` denies a
checkable violation before the edit, ADR-038). A claim carries text, scope globs, a citation (exact
lines, a hash and the literal text), a tier (every live claim is doc-claimed), a status (fresh, stale,
contradicted, superseded, refuted), a watermark, a lifecycle (candidate, active or retired: in code only, not stored on live claims) and a
validity window (the columns exist and are empty on every live row).

## The packs (live copy of 2026-09-28)

| Pack | Rows | Fresh | Stale | Superseded | Services |
|---|---|---|---|---|---|
| obligations | 1,340 | 633 | 11 | 696 | 15 |
| adr-rules | 905 | 852 | 1 | 52 | 7 |
| spec-bindings | 1,780 | 1,582 | 51 | 147 | 7 |
| all three | 4,025 | 3,067 | 63 | 895 | 15 (16 in the live copy) |

The scorecard (2026-09-27) reads 634, 852 and 1,612 fresh. Live totals: 0 refuted. Fathom itself holds
2,026 of the 4,025 claims. Two more packs have no live claims: `findings` (tier sme-stated, the only
drift-tracked layer, imported from an outside review kit) and `ui-appkit` (never compiled). A sixth
layer, certifications, is deterministic (ADR-043) and built from declarations, not a model: 0
declarations live.

## The evidence, graded

Measured figures are from the scorecard and the live store copy; each is a reading at that date.

| # | Finding | Figures | Grade |
|---|---|---|---|
| 1 | **Compile is cheap and scales with documentation change, not code change** | $27.50 for 4,025 claims over 82 runs on Haiku 4.5 (7.37M input and 4.02M output tokens). Per claim: adr-rules $0.013 (17 runs, $11.82), obligations $0.007 (49 runs, $9.60), spec-bindings $0.0034 (16 runs, $6.08). The fleet recompile CV-M8 made 2,798 claims for $7.05 across 26 compiles in 12 services. Code commits cost $0 to recompile: claims anchor to slow-changing documents and join to code by glob at query time | measured |
| 2 | **The compile gate rejects 56 percent of proposals** | 9,219 proposed: 4,051 kept, 5,168 rejected. Of 4,936 rejections with a recorded cause: over-broad 2,240 (2,197 from adr-rules), scope-unverifiable 1,886, malformed 800 (721 from spec-bindings), scope-out-of-subtree 10 | measured |
| 3 | **Freshness heals itself** | 2,056 re-anchor events against 63 contradictions: 97 percent healed with no recompile; the scorecard held 1,843 sweep events on 2026-09-27. A token-free sweep re-hashes each citation after every ingest; a query-time flag serves stale claims flagged, never hidden (an earlier bug rendered stale claims as binding, O88, fixed) | measured |
| 4 | **Eval fixtures pass, and the fixtures are tiny** | Precision 1.00 on all four packs; recall obligations 0.86 (it missed no-secret-logging in both 2026-09-24 runs; its 2026-07-17 runs scored 0.71 and 0.86 and an unpinned model 0.57), adr-rules 1.00, spec-bindings 1.00, findings 1.00. Each fixture is 6 or 7 rules plus 2 traps; the gate is precision 0.90 and recall 0.75; Haiku 4.5, two runs each. These are eval scores, not live precision | measured (fixtures), unmeasured (live precision) |
| 5 | **Serving reaches the agent** | 26,118 `obligations` calls (22,560 from the edit hook, 1,182 from agents); the hook injected on 7,605 of about 110,000 edit events in 14 days (6.9 percent). Payload median 131, p90 315, max 378 tokens (n = 1,918); 47 percent exceeded the old 150-token ceiling, so ADR-016 raised it to 320. The live log later read 27,345 calls, 23,726 from the hook. The channel is forced by configuration, not agent choice | measured |
| 6 | **Served is not obeyed** | vs-pilot-5: rule served in 0 of 4 trials. Guards pilot 6: served in 2 of 8 tasks, 7 violations wired against 6 control, 1.60 times the tokens per clean success. Guards pilot 8: served in 4 of 8 tasks, 4 against 4 (a tie), 1.56 times (10.53M against 6.77M tokens); the wired arm had 7 of 11 runs clean, the control 10 of 14; the bar of at most 1.0 times fails. On task i28 the rule was served before the edit in both trials as a raw section, yet both trials violated it and both rewrote or renamed the test that pinned the rule; on i19 it was served only at doc-claimed tier and both violated. Deny-at-edit (ADR-038) followed; the scorecard records 8 denials in 14 days, all inside bench subjects, and replaying rules over 200 commits caught nothing (VB-16) | measured |
| 7 | **Selection is the bottleneck** | About 140 rules are eligible for a typical edit and 6 are served. 53 percent of obligations claims (344 of 644; fathom alone 212 of 316) carry a repo-wide glob, adr-rules 5 percent, spec-bindings 0 percent; a repo-wide scope makes a claim eligible for almost every edit and crowds out specific rules. Raising the ceiling did not fix it (ADR-016: the right rule can be missing regardless of headroom). In vs-pilot-5, 14 of 60 sections hit `budget_stopped` and ADR-037's raw-section fallback answered them. On the research day the hook on one shell command served 6 of 177 eligible rules, none related to the command | measured |
| 8 | **Fresh does not mean in force** | At least 12 fresh fathom claims cite ADR-003 (DuckDB) or ADR-004, whose headers read Superseded, and none of the 4,025 rows has a `valid_to`; for example "persist observations and derived tables in DuckDB" (scope `internal/store/**`) is still fresh although ADR-025 deleted DuckDB. Freshness only checks that the cited text is unchanged. The fix is validity windows (N8.2, GL-15 to GL-17) | measured |
| 9 | **Churn and waste** | 895 rows (22 percent) superseded (obligations 696, 52 percent of the pack), 17 runs `budget_stopped`, 5 refused, 101 adr-rules sections failed; recompiles triggered by prompt changes were paid for and then retired; CV-M8's dry run priced every layer at $0.00 until it was fixed | measured |
| 10 | **Nothing learns yet** | 0 human verdicts on any claim; 0 per-claim serve counts; the rule-outcome ledger did not exist on the reference machine. SR-61's `claim_id` grew obligations payloads 10 to 13 percent (904 to 1,021 bytes, 701 to 779, 760 to 838); a comment claiming the payload was byte-identical was wrong and was corrected. Also, the events recorded turned out not to measure compliance (O120, JF-1) | measured |
| 11 | **Packs without a live consumer** | findings: 0 claims, 0 external findings, 0 `findings_near` calls (the tracer passed 2026-09-18; three later runs were lost to faults in the measuring setup); ui-appkit: never compiled (capped at 40k tokens and $1); expert-module tools: 0 MCP calls | measured |
| 12 | **One task where layers helped** | Guards pilot 8, task i2 (the store must stay pure Go): both wired trials passed (0.57M and 0.53M tokens); the control violated once (8.15M tokens) and then passed (0.77M). Serving explains only half: in wired trial 1 the rule was pushed out of the top 6 and the agent found `ADR.md` itself; in trial 2 the agent asked the tool. The arms tied overall, 4 against 4 | measured (n = 1 task) |

## Twenty cousins, and three traditions

Twenty public systems do part of this job. Each teaches something; vendor figures are the vendor's own
(*claimed by a vendor*) and were read on the page or seen only in search summaries as noted in the
source.

| Family | System | What it teaches fathom |
|---|---|---|
| Data layers | Business Objects Universe (US patent 5,555,403, filed 1991, granted 1996) | A layer locked to one consumer does not carry over; a layer should not belong to one consumer |
| Data layers | dbt Semantic Layer | Fix a definition once and it is fixed for every consumer |
| Data layers | Cube (a vendor case names Brex) | Consumer-agnostic and access-scoped: one claim serves a CLI, an MCP client and an editor hook |
| Data layers | Looker and LookML (Google reports up to two thirds fewer errors in its own tests) | Definitions in git with history and review, like `docs/intent/` and ADRs |
| Data layers | AtScale (an anonymous retailer case: 80 percent of queries under 1 second, a 20 TB or larger cube) | One layer, many tools |
| Data layers | Airbnb Minerva (about 12,000 metrics, 4,000 dimensions, 200 producers; a secondary source, its date uncertain) | Define once, use everywhere; the strongest case for persistence: two agents that know different things about the same code |
| Code facts | Meta Glean | Stacked immutable databases make re-indexing cost proportional to the change, which matches hash-gated ingest |
| Code facts | Sourcegraph SCIP (indexers for Java, TypeScript, Rust, C and C++, Ruby, Python, .NET, Dart, PHP) | Cross-repository, version-aware resolution matches the fleet |
| Code facts | Data catalogs and knowledge graphs (Amundsen, DataHub, OpenMetadata) | Relationships are the graph; there is little hard evidence that code knowledge graphs beat plain retrieval for agents |
| Rules as code | Open Policy Agent (large adopters; one integration reported about 400,000 queries a second uncached) | A governed, versioned rule served at decision time: closest to obligations at the edit |
| Rules as code | Semgrep custom rules ("block merge and comment" or "comment only") | Guides versus gates: advisory rules versus loud guards |
| Rules as code | ArchUnit | An invariant that is proven, not just documented: the same shape as done-means-proven |
| Rules as code | dependency-cruiser (error, warn, info severities) | Severity turns a rule into a dial |
| Rules as code | ADRs as fitness functions (Ford and Richards; practitioner posts) | A decision record documents; a fitness function assures. ADR plus invariant plus enforcement column is close |
| Ownership | CODEOWNERS | A scoped claim ("this path is owned by X") served at review time |
| Ownership | Backstage Software Catalog | Claims live next to the code and are harvested into a queryable graph; fathom serves the same kind at decision time |
| Agent memory | AGENTS.md (over 60,000 projects; stewarded under the Linux Foundation) | No citations, no validation, no scope beyond directories: the gap a layer fills |
| Agent memory | Claude Code memory (a hierarchy of instruction files plus path-scoped rules) | Scoping is the same idea; nothing validates content against the code, and the docs treat these files as context, not enforcement (hooks enforce) |
| Agent memory | Cursor rules (four modes) | Staleness is left to the author; the vendor advises short rules and references over duplication |
| Agent memory | GitHub Copilot Memory | The closest public cousin (below) |

**Three traditions, and where fathom sits.** *Define and serve* (data-world layers: one definition, many
consumers; they know nothing about files or edits). *Enforce* (OPA, Semgrep, ArchUnit: a rule is checked
and a violation stopped; the reason lives in a separate document). *Persist* (agent memory files:
guidance survives between sessions; most carry no citation and nothing checks them against the code).
**Fathom's claim: all three, plus a citation and a staleness check on every claim.**

### The closest public cousin: GitHub Copilot Memory

| Question | Copilot Memory | Fathom layers |
|---|---|---|
| Who writes the fact | The agent, as it works | A model restates documents that people govern, and a gate checks it |
| What is cited | A code location (a file and line) | Exact lines in a governing document, with a hash and the literal text |
| How freshness is kept | Citations checked just before use; a contradicted fact is corrected | A sweep re-hashes after each ingest; a query-time check flags stale claims |
| How long it lives | Repository-scoped; expires after 28 days; storing again resets the clock | Until the source changes or a newer compile replaces it; retirement rules are proposed |
| Who can read it | Vendor-locked | Any agent, through the hook, the CLI or MCP |
| Measured effect | 90 percent PR merge rate with memories against 83 percent without (GitHub's own test; code-review positive feedback 77 against 75 percent) | No behavior measurement yet; guards pilot 8 tied on violations, 4 against 4 |

### Lessons from outside that shaped the design (as recorded in `docs/design/semantic-layers.md` section 2)

Sourcegraph retreated from embeddings-first retrieval to deterministic context (so serving is a lookup,
no model in the loop); DeepWiki serves stale wikis with no indicator (stale is flagged, not hidden);
business-intelligence semantic layers die on upfront modeling (compile from documents that already exist,
redo only changed sections); GraphRAG's $33,000 index made cost the first question (compile cost is
measured per claim and capped per run); `llms.txt` has about 784 sites and no consuming agent while
AGENTS.md has 60,000 or more (delivery goes through a hook into the agent's own context); frontier
models mis-cite about 15 to 20 percent of the time (every citation is re-resolved at compile time);
Moderne showed false positives are trust-fatal (the gate; a live precision figure does not exist yet).

### What the outside evidence cannot tell us

No independent measurement was found that code knowledge graphs beat plain retrieval for agents. Vendor
case studies (AtScale, Looker, Copilot) are self-reported. The Airbnb figures come from a secondary source
with an uncertain date. These lessons were not re-verified for this note.

## Where next (proposals; each has a record)

- **Rules that learn from outcomes** (GL-10, GL-13; thresholds in `.fathom/learning.cue`, GL-12). A rule
  moves candidate (3 or more corroborations from 2 or more sessions and 0 refutes in 30 days), to active,
  to retired (3 or more refutes with refutes above corroborations, or 180 days never triggered). Active
  rules take a class over a 30-day window once served at least 20 times: ignored (violated over served
  at least 0.5), effective (complied over served at least 0.9), never triggered (90 days), contested (3
  or more refutes). Nothing learns until 30 days of signal exist and SR-61 is the first piece; O120 changes
  what violated and complied mean. Retirement without signal is guessing, so collect verdicts first.
- **End dates and time travel** (N8.2, GL-15 to GL-17): `valid_from` and `valid_to` and an `as_of`
  query close the superseded-ADR gap (finding 8).
- **Client rules files as sources** (GL-1 to GL-6): `.cursor/rules`, Copilot instructions, Kiro steering,
  `BUGBOT.md`, `GEMINI.md`, plus SARIF out.
- **Self-invalidating semantic columns: deferred (F37).** A cheap model's judgment about a code unit
  ("validates input: yes or no, with a confidence") is stored as a claim tied to the unit's content hash,
  so it expires when the code changes. Fathom already does this for anchors links (2,798 live) and
  findings drift. A column is a claim kind with no citation, so it meets the promote condition of the
  `ai-derived` tier (F7: its own eval and an ADR). The record of cheap-model judgments is mixed (the
  `route` site 45.7 percent right at confidence 0.9 or more, and no cutoff better than 58.5 percent; the
  within-tier re-rank paired mean -0.19; routing to `localize` on the LCA benchmark 4.6 to 23.4 percent on
  a small sample; see `2026-09-28-jev-ten-levels.md`), so a passing calibration plus an A/B come first,
  and it stays optional.
- **Live maps** (F50): saved questions in the layers' vocabulary, complementary to layers (layers say
  what is true or binding; maps say what code matters for a purpose). The owner recalled about 90 times
  fewer exploration tokens from early benchmarking; that is a *hypothesis* (vision 19) until a bench
  reads it, and this note does not adopt it.
- **From the evidence to a check** (ADR-057): finding 6 (the rule served and broken anyway) is why the
  commit pin line exists, and why a typed `propose_check` for prose rules is a deferred future (F53).

## What it drove

| Decision | Where |
|---|---|
| Keep the three packs, the compiler, the edit hook and rules-to-tests; ditch expert modules | ADR-053; SR-17 |
| Per-rule counts, certifications on fathom itself, findings loop armed, ui-appkit compiled | SR-61, SR-62, SR-64, SR-66 |
| Rules that learn; validity windows; every client's rules files | ADR-049 decision 9; GL-1 to GL-18 |
| The rule-outcome events renamed for what they measure | JF-1 to JF-3; O120; ADR-056 decision 8 |
| Served is not kept: a lost pin is a fact about the change | ADR-057; `docs/design/served-not-kept.md` |
| Self-invalidating columns wait | F37, F7 |

## What this does not claim

That layers change agent behavior (not measured on average); that eval precision is live precision;
that any cousin's effect transfers (each vendor figure is its own claim); that the store copy is current
after 2026-09-28; that the 90 times figure is real.

## Sources

`docs/design/semantic-layers.md`, `docs/design/governance-learning.md`, `docs/vision/02-shape.md`,
`docs/vision/32-decision-port.md`, ADR-016, ADR-017, ADR-037, ADR-038, the pack eval results under
`schema/semantic/layers/*/eval/RESULTS.md`, and the readings
`docs/bench/readings/guards-pilot-8-second-reading.md`, `decision-port-calibration-2026-09-24.md`,
`decision-port-ab-2026-09-23.md` and `cv-m8-fleet-recompile-2026-09-25.md`; the public pages for the
cousins (vendor documentation and engineering blogs, read or seen in search summaries as marked in the
source page).
