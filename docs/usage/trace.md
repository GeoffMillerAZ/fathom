### `trace` — the evidence pack (CX-14..CX-18)

`fathom trace export --standard <id> --out <dir> [--service S] [--store DB]
[--commit REF] [--decision-source PATH ...]` (U8, `docs/design/change-and-compliance.md` §3) writes an
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
| `readings.json` | per-plan verify totals, the certificates joined, any source that could not be read; optional `decision_sources` pins explicit selectors |
| `COMMIT` | the commit |
| `MANIFEST.sha256` | `sha256  file` for every other file |

`fathom trace verify <dir> [--store DB]` recomputes the pack at its own
`COMMIT` and compares every file byte for byte (exit 0), or names each
difference by file and, for the matrix, by `<plan>#<requirement>.<column>`
(exit 1); a pack that no longer matches its own manifest is named too.
Store readings are read as they stand: re-verify against a store indexed at
the pack's commit.

For repositories whose decision records live elsewhere, select committed
Markdown files or directories explicitly:

```sh
fathom trace export --standard example --out workspace/evidence-pack \
  --decision-source INTENT.md --decision-source docs/adr
```

Explicit selectors replace `ADR.md` and `docs/intent/invariants.md`; they do
not add to those defaults. Paths are literal, repository-relative Git paths,
never globs. Harmless `./` and trailing slashes are canonicalized, selectors
are sorted/deduplicated, and overlapping file sets are read/digested once.
Every selector must name a Markdown file or a directory containing Markdown
at the resolved `COMMIT`. Current files, untracked additions and later commits
cannot change that expansion. Missing/non-Markdown inputs, traversal,
absolute/backslash/control paths, and selected trees containing symlinks or
gitlinks refuse before a pack is written.

The limits are 16 distinct selectors, 256 Markdown files, 1 MiB per file,
8 MiB total content, 1 MiB Git tree metadata and 4,096 bytes per path.
Directory selection reads lowercase `.md` files only. Explicit mode recognizes
real Markdown ADR headings such as `# ADR0001`, `# ADR 0001`, and
`## ADR-001`, plus explicit invariant IDs in a table's first column. IDs keep
their exact case, punctuation, spacing and number width. Numbered prose does
not create `I1`; fenced/indented examples and filenames declare no IDs.
Decision cells remain cited or unproven, never compliance or execution proof.

Reverification consumes `decision_sources` from the pack and re-reads at its
`COMMIT`; `trace verify` accepts no selector override. Omit the export flag to
keep legacy paths, parser, missing-source disclosures and pack bytes. This is
deterministic local provenance, not authenticated signing: a completely
reconstructed unsigned pack is not tamper-proof.
