# Assessments

Dated, plain-language readings of things fathom does not run itself: the market, a neighbouring
tool, a survey of peer stacks, an owner-verdicted scorecard, a language queue, a video series, an
audit of the semantic layers. One file per assessment, named `YYYY-MM-DD-<slug>.md`; the data an
assessment rests on sits beside it in `data/`. This is the family convention (agentic-kit and
Review Kit keep `docs/assessments/` the same way, ADR-055: one family, one contract).

An assessment is finished only when its summary is here **and** every decision it drove is in the
records (an ADR or a dated amendment, futures, scope, plan rows). The artifact that first showed
the reading is a readable copy, never the record: artifacts move, expire and are edited by their
readers, and nothing in a page can be verified against this repository.

## What lives here, and what does not

| Place | Holds | Why not here |
|---|---|---|
| `docs/assessments/` | A reading of other people's work or of an owner's judgment, and the evidence behind a decision: market maps, vendor documentation, a hand-verdicted scorecard, notes on a video | (this directory) |
| `docs/bench/` and `docs/bench/readings/` | Measurements fathom runs and can re-run: pre-registered bars, derived third-party truth, ledgers (ADR-031 to ADR-034) | An assessment is not re-runnable; filing it as a reading would dilute "a reading is re-runnable". Measurements that come out of a program (the test_ran base rate, language readings, PR-M0 to PR-M2) do go to `docs/bench/readings/` |
| `docs/research/` (not used) | nothing | The name collides with fathom's own vocabulary (the frozen `fathom research` verb, the research bundle of ADR-046 decision 6, `research-flywheel.md`); a directory of that name would be served and searched as if it were that feature |
| `docs/design/`, `docs/vision/`, `ADR.md` | Decisions and their designs | An assessment informs a decision and never makes one; it cites the record that did |

Two older documents of this kind sit at the top of `docs/` (`assessment-claude-code-integration.md`
and `graphify-comparison.html`); they may move in later with link updates.

## Conventions

- **Open with a plain-language verdict**: what the reading says in a few sentences, in words a
  newcomer follows, before any table.
- **Grade every claim.** Use these five words and no others: *measured* (fathom or the author read
  it from a log, a file or an API on a stated date), *estimate* (arithmetic over measured inputs),
  *claimed by a vendor* (their page, their benchmark, their funding figure), *hypothesis* (a
  saving or effect nobody has measured), *unmeasured* (a question with no reading yet). Say what the
  document does **not** claim.
- **Cite every decision it drove by id** (ADR, futures F, scope G, question Q, plan row) and never
  re-decide: the decision lives in the record, the assessment holds the evidence.
- **Never hold a status the product computes** (ADR-054, I54): a language note cites readings and
  `fathom support`; it keeps no level table. The same goes for plan progress (`fathom plans status`).
- **Third-party material is summarized in our own words**, with the source, the commit or read date
  and the licence. Short factual figures are fine; no long quotes, no branding, no course or
  marketing material (the MIT notices of the sources are kept where code or thresholds are reused).
- **Public product names only.** I19's denylist is about private hosts, emails and accounts;
  `TestNoOrgSpecificNames` runs over this directory like every other.

## Index

| Date | File | The verdict in one line | Decisions it drove |
|---|---|---|---|
| 2026-09-27 | [agentic-coding-landscape](2026-09-27-agentic-coding-landscape.md) | The market has many ways to find code and describe work and almost none to prove things about code; keep building and point at that gap, with reach before new engines | ADR-049, ADR-050, ADR-051, vision 38 sections 5 to 7 |
| 2026-09-27 | [fathom-and-claude-os](2026-09-27-fathom-and-claude-os.md) | Neighbours, not competitors (about 25 percent overlap at session-start context); keep building with a sharper focus; one embedded file, many indexes, no extra servers | ADR-049 amendment 2026-09-29, F55, RH-11, RH-12, GL-23, SR-X1, RH-X4 |
| 2026-09-27 | [peer-stacks-and-rust](2026-09-27-peer-stacks-and-rust.md), [data](data/2026-09-27-peer-stacks.json) | Fathom's stack is among the leanest in the field and Go is not the bottleneck; do not rewrite in Rust | ADR-049 decision 11, vision 38 section 8, F29, RH-M5 |
| 2026-09-27 | [feature-scorecard](2026-09-27-feature-scorecard.md), [data](data/2026-09-27-feature-scorecard.json) | Eighty-six features, each scored and verdicted by the owner (30 keep, 24 improve, 12 merge, 10 ditch, 10 new); the strongest is done-means-proven, the weakest is governance that does not bite | ADR-053, ADR-054, FM-M1, F51 |
| 2026-09-28 | [jev-ten-levels](2026-09-28-jev-ten-levels.md) | Level 10 works in fathom if the test run does the asking: compute first, judge only what is left, keep Jev optional per feature | ADR-056, vision 40, F34 to F49, G27 to G32 |
| 2026-09-28 | [semantic-layers-evidence](2026-09-28-semantic-layers-evidence.md) | Keep the layers and improve them: cheap, checked and self-healing, but served is not obeyed and nothing learns yet | ADR-016, ADR-038, ADR-057, GL-10 to GL-18, F53, F7 |
| 2026-09-29 | [language-atlas](2026-09-29-language-atlas.md) | A 142-item queue and a four-level ladder make "supports language X" a reading; the bottleneck moved from dialects to the measuring instruments | ADR-048, ADR-054, G15, vision 37 |
| 2026-10-11 | [reviews-and-incidents](2026-10-11-reviews-and-incidents.md) | Partly yes: fathom supplies deterministic cross-file joins, facts with coverage and receipts behind well-architected, security and incident reviews; Review Kit judges, policy-kit and pdk/risk own rules and tiers, an external agent acts, and fathom never grades, gates or mitigates | ADR-069 decisions 1 to 8, the ADR-067 decision 12 and ADR-068 decision 4 amendments, vision 44, G37, F71, F72, F4, F32, F59, WAE- |
| 2026-10-11 | [index-snapshots](2026-10-11-index-snapshots.md) | Yes as files CI exports and the operator serves, not a server fathom operates; one new subsystem (`import snapshot`); instances ship index snapshots and never call each other; parked until the owner starts it, and the environment daemon waits on the owner's U1 answer | ADR-069 decisions 9 to 15, F3, F27, F70, F72 to F74, G37's IXS-M0, IXS- |
| 2026-10-11 | [making-the-vision-happen](2026-10-11-making-the-vision-happen.md) | Fathom is the center of the evidence plane, not of the platform: the best context source for Review Kit, pdk/risk and incident agents, with no new service; Review Kit's reliability catalog is the critical path, and owner hours bind before worker-days | ADR-069, vision 44, G37 (kill 2027-01-31), owner rulings D1 to D19 |
| 2026-10-16 | [claude-code-mods](2026-10-16-claude-code-mods.md) | Yes, in two steps and only as a display: Go and markdown first, then one small TypeScript module in an optional plugin that draws what fathom's hooks already told the agent; never a gate | ADR-072, the `No Node toolchain` carve-out, ADR-029 amended, F2 in part, G40, Q36, plan MOD- |

## Companion pages

The owner keeps private companion pages for several of these readings; they are not linked here
because readers cannot open them, and the files above are the record.
