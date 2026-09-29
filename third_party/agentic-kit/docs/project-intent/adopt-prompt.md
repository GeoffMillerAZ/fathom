# Adopting Project Intent — the prompt to paste into a repo's agent session

Copy everything below the line into a Claude Code or Codex session opened at the root of
the repo you want to adopt the practice. Prefer project-local pinned skills; a personal
installation is optional and needs authorization to change global configuration.

---

Adopt the Project Intent practice in this repository.

Reference: `<path-to-agentic-kit>/docs/project-intent/README.md` (the model),
`changes.md` (commits, changelog, release), `operating.md` (autonomy, incidents,
tripwires), `integration.md` (the exact stanza texts), `examples.md`. Skills: the
`intent-*` family. Resolve the project-local skills first, then any personal install.
If missing, use the reviewed snapshot procedure in `integration.md` §4. Do not change
global configuration, overwrite competing skills or fetch mutable upstream implicitly.

Then run `intent-init` for this repo, honoring these rules:

1. **Point, never copy.** Good existing documents (a vision, ADRs, an architecture doc,
   a roadmap, specs) stay where they are; the router names them. Keep an existing
   decisions directory and its numbering. Do not rename, move, or rewrite what exists.
2. **Evidence only.** Every statement in `docs/intent/` cites a file, a test, a commit,
   or an incident. Quote sharp phrasing. Mark inferences `(inferred)`. Omit a section
   rather than fill it with generic language. What you cannot determine goes in
   `questions.md` for me.
3. **Honest enforcement.** `invariants.md` says for each row whether a test, a lint,
   a gate, or nothing enforces it. Do not upgrade a row you have not verified.
4. **Futures are not requirements.** Only what the repo actually names goes in
   `futures.md`, each with confidence, horizon, `Do not build`, `Preserve`,
   `Promote when`, `Drop if`.
5. **Preserve decision history.** Index existing records and their actual triggers.
   Where local ADR rules are append-only, do not rewrite a historical body to fit the
   template; keep a qualified pointer and record any reopened decision as a successor.
6. **Wire the stanzas** from `integration.md` §2 into `AGENTS.md`, one copy, six plus
   four plus three lines. If a `CLAUDE.md` exists, it gets one line, `@AGENTS.md`, and
   no copy of the stanza; never create one. Do not import the tree.
7. **Autonomy.** Draft `autonomy.md` from how this repo is actually worked: which
   branch agents may push freely, what needs my approval, what is never done. Ask me
   only where the repo gives no evidence.
8. **Change records.** Create `CHANGELOG.md` from the template if none exists; if one
   exists, add the `Release impact` line and apply the two-sentence rule from the next
   entry on, never rewriting released blocks. Fill the compatibility surface in
   `scope.md` from the repo's real contracts.
9. **Incidents.** Seed `incidents.md` from any documented failure; point at a
   predecessor's catalog rather than copying it.
10. **Monorepo.** If this repo holds more than one product, nested modules, or parts
    expected to be split out, merged in, or retired, write `docs/intent/repo.md` (the
    repository charter, `monorepo.md`): why one repository, what belongs, a component
    map with an evolution trigger per row, dependency direction, ecosystem. Give a
    component its own `docs/intent/` only if it has its own audience, cadence, or
    roadmap, and add the nested stanza to its agent files. Route by affected paths,
    including root-started sessions. Inherit autonomy and incident ownership rather
    than creating competing local policies; parent grants are ceilings, not suggestions.
11. Finish with `intent-doctor` and fix what it reports. Then give me: the five
    sharpest open questions, the invariants that nothing enforces, and any place the
    code contradicts a stated rule.

Budgets: router ≤ 80 lines, each living doc ≤ 150, each record ≤ 80. Report structural
checks separately from live-host verification and judgment. Commit only when authorized
by the user and local autonomy; include a `Verified:` line. Preserve unrelated work.
