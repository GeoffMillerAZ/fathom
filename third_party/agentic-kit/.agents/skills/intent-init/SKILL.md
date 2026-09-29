---
name: intent-init
description: >
  Bootstrap a repo's Project Intent system: docs/intent/ (router, principles,
  invariants, domain, scope, futures, questions, autonomy, incidents), a decisions index,
  CHANGELOG.md, and the CLAUDE.md/AGENTS.md stanzas, populated from evidence in the repo
  and pointing at existing VISION or ADR documents instead of copying them. Use for a
  new repo, an existing repo without docs/intent/, or to add a missing piece to one.
---

# intent-init

Part of the `intent-*` family. Rationale: `agentic-kit/docs/project-intent/README.md`
§10. Templates live beside this file in `templates/`. After bootstrapping, run
`intent-doctor`.

## Rules

- **Point, never copy.** A good existing `VISION.md`, `docs/adr/`, `ARCHITECTURE.md`,
  or `ROADMAP.md` stays where it is; the router names it. Existing ADRs keep their
  directory and numbering; the router says "`ADR-` here means any decision record".
- **Evidence only.** Quote the source's sharp phrasing. Mark inferences `(inferred)`.
  Omit sections with no evidence. Never fill with generic language. What cannot be
  determined goes in `questions.md` for the human.
- **Budgets.** Router ≤ 80 lines, living docs ≤ ~150, records ≤ 80.

## Procedure

1. **Inventory** prior art: `VISION.md`, `README.md`, `docs/adr/`, `docs/decisions/`,
   `ARCHITECTURE.md`, `ROADMAP.md`, specs, plans, `CLAUDE.md`, `AGENTS.md`, tests that
   encode rules, `git log` for reversals and pivots, existing `CHANGELOG.md`.
2. **Select scope before templates.** Root owns repository policy; a component keeps
   only local differences. Do not create component autonomy, duplicate root goals, or
   empty inherited documents. Copy applicable templates into `docs/intent/`. Skip
   `vision.md` if a vision doc exists. Skip
   `decisions/` if a decisions dir exists, but add its `README.md` index there. Copy
   `CHANGELOG.md` to the repo root if none exists.
3. **Populate**, in this order: `invariants.md` (with an honest Enforced-by column:
   `test <name>` / `code` / `stated`), `domain.md`, `scope.md` (goals with `Done when`,
   boundaries from code, compatibility surface from real contracts, non-goals with
   `Revisit when`), `principles.md` (tensions as tensions), `futures.md` (only what the
   repo actually names; confidence and horizon on each), `autonomy.md` (may / ask /
   never, the branch model), `incidents.md` (seed from any documented incident, or
   leave the header), `questions.md`.
4. **Existing decisions**: preserve append-only records. Index existing reconsider,
   defer, or until conditions; add trailers only if the repository allows that edit.
   An authorized change of policy gets a new record and a supersession pointer.
5. **Router**: fill the docs map, the routing table, the sensitive surfaces with the
   evidence each requires, and the `Verified against` footer (tool names and versions).
6. **Wire**: the six-line project stanza and the four-line change-records rule
   (`integration.md` §2, `changes.md` §8) into `CLAUDE.md` and `AGENTS.md`, identical
   text. A three-line autonomy summary next to them. Do not import the tree.
7. **Monorepo**: if the repo holds more than one product, nested modules, or components
   with different evolution paths (`docs/project-intent/monorepo.md` §1), write
   `repo.md` from `templates/repo.md`: why one repository, membership rule, the
   component map with `stays` / `split when` / `merge into` / `retire when` /
   `inbound from` per row, dependency direction, procedures, ecosystem. Give a component
   its own `<component>/docs/intent/` only if it passes the own-intent test (own
   audience, cadence, or roadmap); its router's first paragraph links to root; add the
   nested stanza (`integration.md` §2b) to that component's `CLAUDE.md` and `AGENTS.md`.
8. **New repo**: same, but `vision.md` and `scope.md` come from the brief, `futures.md`
   holds only what the brief names, and `decisions/` starts with the two or three
   choices the brief already made that pass the threshold.
9. Run `intent-doctor`. Report the open questions: five sharp ones beat thirty.

## Team distribution (when requested)

Prefer a reviewed project-local snapshot to global configuration changes. The offline
`scripts/vendor_intent.go` updater and `-check` mode are documented in the practice's
`integration.md` §4. Run its tests explicitly (hidden skill directories are not part
of Go's recursive `./...` discovery). Never fetch, update a pin or overwrite a competing
skill implicitly. Report byte/link integrity separately from native host discovery.
