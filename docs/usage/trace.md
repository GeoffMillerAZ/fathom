### `trace` — the evidence pack (CX-14..CX-18)

`fathom trace export --standard <id> --out <dir> [--service S] [--store DB]
[--commit REF]` (U8, `docs/design/change-and-compliance.md` §3) writes an
evidence pack, only when asked. It reads, at the commit (default `HEAD`,
via `git show`), every `docs/plans/*.md`, the repository-authored standard
mapping `.fathom/trace/<id>.cue` (shape: `schema/trace/schema.cue` —
`standard`, and `clauses` naming requirement ids or RE2 `patterns`; fathom
ships no standard's clauses, ADR-011) and the decision records (`ADR.md`
headings, `docs/intent/invariants.md` ids), plus the store's verify and
certificate readings for the service. The pack:

| File | Carries |
|---|---|
| `matrix.json`, `matrix.md` | per requirement (a plan row, a foreign spec task, or `<row>/accept`): status, code, test, certificate, attested run, decision — each cell `{state, reading}`; the mapping's clauses with the requirements each resolves and any id it names that the matrix lacks |
| `digests.json` | sha256 of every artifact read at the commit |
| `readings.json` | per-plan verify totals, the certificates joined, any source that could not be read |
| `COMMIT` | the commit |
| `MANIFEST.sha256` | `sha256  file` for every other file |

`fathom trace verify <dir> [--store DB]` recomputes the pack at its own
`COMMIT` and compares every file byte for byte (exit 0), or names each
difference by file and, for the matrix, by `<plan>#<requirement>.<column>`
(exit 1); a pack that no longer matches its own manifest is named too.
Store readings are read as they stand: re-verify against a store indexed at
the pack's commit.
