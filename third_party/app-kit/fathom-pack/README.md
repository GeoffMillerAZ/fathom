# app-kit fathom pack

> The kit ships its law with its code. This pack gives any app-kit-consuming
> repo the UI doctrine as *delivered configuration* — pre-edit guards that
> inject at the moment a UI file is touched, and a compiled-obligations
> layer for the deeper rules — so no project starts from scratch and no
> session "forgets" the CLAUDE.md it read an hour ago.

## What's in it

- `guard-pack.cue` — guards for consumer repos (the **consumer** group) and
  for work inside app-kit itself (the **kit-self** group). Guards are
  deterministic, need no compiled layer and no LLM: the note is declared,
  so they work even where obligations are still dark.
- `layers/ui-appkit/` — the compiled-obligations layer pack (prompt + kill
  gate + manifest) for repos that run `fathom compile`: turns the kit's
  decision docs into cited, glob-scoped claims served on edit.

## Install (consumer repo)

1. Copy the **consumer** group's entries from `guard-pack.cue` into your
   repo's `.fathom/guard.cue` `guards:` list. Keep the provenance line on
   each entry (`// from app-kit fathom-pack vN`) — the `/fathom:activate`
   skill re-syncs by that marker. Narrow the scopes to your repo's real
   paths (the shipped globs are deliberately broad).
2. `fathom layers install <app-kit>/fathom-pack/layers/ui-appkit` — then
   compile when your daemon has an LLM profile. Guards work meanwhile.
3. Prove it by demonstration: edit a scoped UI file and confirm the guard
   note appears; run one deliberate violation (a raw `bg-indigo-600`)
   and confirm the injected rule names the role-token law.

Versioning: this pack's version is the kit's version. A consumer whose
provenance markers lag the kit's current pack shows up in
`/fathom:rewire`'s report as drift — same discipline as hooks.

## Authoring guards responsibly

Guards spend attention as well as tokens; the discipline that keeps them
net-positive:

1. **The bar**: a guard exists because the mistake *already happened* —
   the note cites the incident or the decision. Speculative guards are the
   failure mode: fatigue teaches agents to skim the channel, which is
   worse than having no channel.
2. **Scope hygiene**: globs as narrow as the law allows; consumers narrow
   the shipped globs further on install. A guard firing on files it does
   not govern is spam with a citation.
3. **Budgets**: `max_rules` ≤ 4 per entry, notes under ~150 words. The
   injection channel is token-capped and deduped (repeat edits to the same
   file do not re-inject), but attention is not capped — write for the
   agent that has seen this note before.
4. **Staleness is worse than silence**: every note cites its source, packs
   version with their kit, and `/fathom:activate` re-syncs by provenance
   marker — a lagging pack is drift, and a wrong injected rule actively
   misleads.
5. **Measure or retire**: the hook beats record fire rates; reviews record
   catches. A guard that fires often and catches nothing in a quarter is
   folded into docs or killed — the KILL.md discipline applies to guards
   as much as layers.
6. **Guards warn; gates refuse**: hooks fail open by design (a down daemon
   must never block an edit), so anything that must *hold* belongs in a
   blocking gate (lefthook/CI/verify) — the guard is the warning at the
   moment of the edit, not the enforcement.
