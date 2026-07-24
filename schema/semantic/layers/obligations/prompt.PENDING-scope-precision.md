# PENDING prompt.md change — obligations scope-mapping precision (O14)

**Status: NOT LIVE.** This file is a proposal, not a pack input — it is not
named by `layer.cue`'s `prompt_file` (which still points at `prompt.md`
alone) and nothing in `internal/semantic` reads it. It exists so the fix is
on record and reviewable without touching the live prompt.

## Why this can't just be edited into prompt.md today (T5-2b,
docs/plans/trust-and-tokenomics.md)

`promptForLayer` (`internal/semantic/prompts.go`) derives
`PromptVersion = "<layer>/" + sha256(prompt.md bytes)[:8]` — ANY byte changed
in `prompt.md`, including a clearly-marked "pending" section, is automatically
a new compile identity (that is the whole point of content-addressing: no
hand-bumped version const that can silently drift from the text it names).
Landing this change is therefore a **flag-day**, not a patch: it changes what
every future obligations compile produces, and — per the idempotency guard
(A2-1, docs/plans/activation.md) — makes every currently-fresh obligations
claim eligible for recompile under the new identity. That is a deliberate,
scheduled event (the program's next flag-day), not something to trigger as a
side effect of a bug-fix commit. So the fix lives here, inert, until that
event.

## The bug (O14, docs/dogfood-observations.md)

Post-recompile, `obligations(internal/query/engine.go)` served:

> Add new extractor ext→kind mapping in connector.

— a rule compiled from CLAUDE.md's `### Adding things` → **Extractor** bullet
("implement `extractor.Extractor`, register by extension in `fusion.New()`,
add its ext→kind in `connector`, golden-test it."). That bullet only ever
applies to work in `internal/extractor/**`, `internal/fusion/**`, and
`internal/connector/**` — never `internal/query/**`. The model scoped it too
broadly (a catch-all Go glob, e.g. `internal/**` or `**/*.go`) instead of the
several *specific* named packages the rule's own text calls out — noise that
then compounds O14's eviction problem (a wrongly-broad rule competes for the
MaxObligations cap on files it has no business matching).

The existing prompt already says "prefer the NARROWEST glob the document
justifies" and forbids a bare `**`, but has no worked example for the case
that actually broke: a single rule whose text NAMES SEVERAL specific
packages. Left to infer, the model generalizes to one broad glob instead of
listing each named package.

## Proposed change

Append this worked example to the existing `scope:` bullet in `prompt.md`
(after the "Prefer the NARROWEST glob…" / "NEVER emit a bare `**`…"
sentences, before `cite_start/cite_end`):

> When a rule's text names SEVERAL specific packages or paths (e.g. "register
> by extension in `fusion.New()`, add its ext→kind in `connector`"), scope to
> the UNION of exactly those named locations as separate glob entries —
> `["internal/extractor/**", "internal/fusion/**", "internal/connector/**"]`
> — never collapse a multi-package rule into one broad catch-all such as
> `internal/**` or `**/*.go`. A rule about extractors and connectors must
> never match an unrelated package such as `internal/query/**`. If the rule's
> text does not name a specific package, scope to the file class the
> surrounding section is about, per the existing guidance above — do not
> widen to compensate for an unnamed scope.

## Landing mechanism

At the next deliberate flag-day: fold the paragraph above into
`prompt.md`'s `scope:` bullet (replacing this file's job), which bumps
`PromptVersion` for the `obligations` layer. Update `KILL.md`/`eval/
RESULTS.md` if the eval gate needs a re-run against the sharper prompt (a
worked example changing model behavior is exactly the kind of change the
eval gate exists to catch regressions on). Close this file and update
docs/dogfood-observations.md O14 to note the landed identity bump.

See docs/plans/trust-and-tokenomics.md row T5-2b for the decision record.
