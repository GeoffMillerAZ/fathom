# Unreleased minor: explicit trace decision sources

This opt-in compatibility-surface change requires a minor release under the
pre-1.0 policy. It does not change the standard mapping CUE schema or promote
any decision cell to a certificate, runtime result or compliance approval.

Existing invocations and packs need no migration. Omitting `--decision-source`
retains `ADR.md` plus `docs/intent/invariants.md`, the legacy citation parser,
named unread inputs and byte-compatible output. The new `decision_sources`
field is omitted in that mode.

For project-routed records, use repeatable selectors on export:

```sh
fathom trace export --standard example --out workspace/evidence-pack \
  --decision-source INTENT.md --decision-source docs/adr
```

These replace the legacy inputs. Each selector is a literal committed
Markdown file or directory, relative to the repository. The pack records the
canonical selectors and digests of their selected files. Missing/unsafe inputs
or limit overflow refuse before output is created; see `docs/usage/trace.md`
for the exact limits and path rules.

Explicit mode recognizes real Markdown heading/table declarations and exact
citation forms: `ADR0001`, `ADR 0001` and `ADR-0001` are distinct IDs. Numbered
intent prose does not create named invariants. Code examples and filenames
are not declarations. Cells remain cited/unproven using the existing matrix
join and proof tiers.

Use an updated binary to verify an explicit-source pack. Verify re-expands its
recorded selectors at `COMMIT`; later commits, dirty files and command-line
overrides cannot substitute decision sources. Older export binaries reject
the new flag, and older verifiers cannot reproduce the new readings field
byte-for-byte. Store readings remain live as before: use the store indexed at
the pack's commit. Preserve original packs instead of refreshing their hashes.

This is local deterministic provenance, not a signature or a defense against
a writer reconstructing an entire unsigned pack. No model/provider site or
source egress is added.
