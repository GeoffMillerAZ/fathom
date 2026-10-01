### `claim`

Human-judgment triage on a compiled semantic-layer claim, store-direct only (v0):

```sh
fathom claim add          --store <path> --service <id> --scope <glob> [--scope <glob> ...] --rule "..." [--remedy "..."] [--cite path#Lstart-Lend] [--json]
fathom claim refute       --store <path> --service <id> --claim <claim-id> --note "..." [--author <name>] [--dry-run] [--json]
fathom claim corroborate  --store <path> --service <id> --claim <claim-id> --note "..." [--author <name>]
fathom claim show         <claim-id> [--service <id>] [--json] [--store <path> | --url <daemon>]
fathom claim list         [--layer <name>] [--service <id>] [--status current|fresh|stale|contradicted|superseded|refuted|all] [--page N] [--json] [--store <path> | --url <daemon>]
fathom claim effectiveness [--service <id>] [--class ignored|contested|never_triggered|unclassified|effective|all] [--page N] [--json] [--store <path> | --url <daemon>]
fathom claim learn        [--service <id>] [--store <path>] [--apply] [--json]
```

`effectiveness` (GL-18, U4) prints, per governing rule (a current claim in a governing layer, not retired), the counts the edit hook's rule-outcome log (`~/.fathom/rule-outcomes.jsonl`, GL-10) holds over the last 30 days (served over 90: served, top changed, still served, blocked, refuted, corroborated; JF-2 renamed `complied` and `violated`, and `--json` carries `top_changed_30d`, `still_served_30d`, `top_changed_rate` and `still_served_rate`), each with its n, and the class each falls in (design `governance-learning.md` §5) with the change the evidence proposes — move an ignored rule to a hook or rewrite it, prune or re-scope a never-triggered one. Every proposal is a proposal: nothing is applied. No log, or one that cannot be read, classifies nothing and says why; a rate needs 20 serves, below which the row shows its n; and never triggered is not judged until the log reaches back 90 days. `--json` is exactly `/rules`' section.

`learn` (GL-13, design `governance-learning.md` §6) promotes a candidate claim to active and retires an active one under the thresholds in the service's `.fathom/learning.cue` (the binary's defaults for any field it omits, GL-12): promotion at 3 corroborations from 2 distinct sessions with no refute over 30 days; retirement at 3 refutes outnumbering corroborations, or no serve for 180 days. It reads the rule-outcome log the hooks and verdicts write (GL-10) and the service's governing claims (current, in a governing layer), and prints each decision with the signals behind it. The default is a dry run; `--apply` persists the lifecycle, and a retirement closes the claim's validity window at that instant (its status is unchanged, and no row is ever deleted). Every applied decision is announced with its signals (GL-14): one `announced:` line each, and one intent-ledger row each, which the session attention line counts (`learning N`) and `fathom intent audit` lists; a dry run announces nothing. A never-triggered retirement is held, and named, while the log does not reach back over its whole window. The store defaults to the one the daemon records, and every project with claims is read unless `--service` names one. The owner runs it: no daemon pass or background job runs it.

`show` (OC-30) prints one claim whole — statement, citation, scope, tier, status,
provenance and every verdict recorded against it — and `--json` is exactly the
console's `/claims/{id}` section. The citation is excerpted with two lines of
context from the service's recorded root; a stale, contradicted, superseded or
refuted claim whose compile-time text was kept shows that text, labeled, never the
current lines as its source (J2) — superseded and refuted are terminal verdicts, so
the sweep never re-anchors their citation and the range freezes while the file moves
on. Without kept text, such a claim's excerpt is the current lines at the range with a
note that they may have moved. `--service` is needed only when two services hold the
same id. It reads the Knowledge store (see `layers strata` below), never writes.

`list` is the rows behind every claim count: the claims of a layer, a project and a
status (`current` — fresh, stale, contradicted — by default), ordered by project,
layer, status and statement, 50 a page, each with its first citation. `--json` is
exactly the console's `/claims` section for the same filter; the strata, coverage,
layer and packs pages link each count to it.

`add` (SU-17, docs/plans/substrate.md §D) mints one content-addressed, human-authored
claim without waiting for a pattern to recur — "remember this" as a single utterance.
Producer is always `human.review`; tier is always capped at `sme-stated` — `--tier` is
parsed only to be refused (`--tier` is not a valid flag on this verb; there is no way to
raise the cap). `PromptVersion` is `human-authored/v1`, its own reconciliation domain:
distinct from native review's `review-verb/v1` and the governed import's
`external-import/v1`, so neither ever supersedes or is superseded by a `claim add` claim.
An `external_findings` row is written alongside it, exactly like a governed import
(`source id human:<sha256(rule)[:12]>`, `source version 1`); scope-fp and cites links are
written with the SAME helpers `fathom findings import` uses. Served by the pre-edit hook
like any finding. `--store`, `--service`, at least one `--scope`, and `--rule` are
required; `--remedy` and `--cite` are optional.

`refute` appends a `ClaimEvent{Verdict:"refuted"}` **and** flips the claim's status to
refuted — suppression is refutation with a note. `--dry-run` (SU-18) previews the
disposition instead of recording it: `would withhold N served row(s) across N file(s):
<first 5 files>` (JSON: `{claim_id, would_withhold, files, truncated}`), computed by
walking the claim's own scope glob(s) against the service's indexed files. Nothing is
written on a dry run; `--note` is not required with `--dry-run`. `corroborate` appends a
`ClaimEvent{Verdict:"corroborated"}` and never changes status or tier — corroboration is
provenance, not evidence-laundering. `--store`, `--service`, `--claim`, and `--note` (the
last waived by `--dry-run`) are all required; `--author` is optional.

---
