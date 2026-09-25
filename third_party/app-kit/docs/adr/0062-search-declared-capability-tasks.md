# ADR-0062: Search declared capability tasks, not explanatory prose

- **Status:** Accepted
- **Type:** architecture
- **Date:** 2026-09-16
- **Extends:** ADR-0020

## Context

G1's broader, source-reviewed evaluation exposed real selection failures: usage
disclaimers recommended an account backend and automatic source patching, while
combining "email field" with "licensed design delivery" invented an email service.
Common supported task phrases were also absent. This fires ADR-0020's bad-selection
revisit trigger; the shared catalog remains the correct boundary.

## Decision

Match every normalized query token within either the entry's combined ID/name or
one authored positive task phrase. Do not combine unrelated tasks. Summary, usage,
proof, gaps, dependencies and app-owned responsibilities explain a contract; they
are not positive search evidence. Exact get still supplies the complete contract.

Keep deterministic case/punctuation normalization, duplicate-token removal and
three-character-or-longer prefix matching. Score the best complete field match;
names/IDs outrank task phrases within the existing native/reference priority groups.
Compatible proven recipes still lead, and empty-query inventory remains ID-sorted.
Profile/layer filters, bounded summaries and read-only/private-source boundaries stay.

Maintain reviewed task vocabulary in the existing CUE records, not a hidden synonym
service. A phrase can describe related facets, such as mobile navigation and current
route, so useful conjunctions need not be enumerated one query at a time. Never merge
unrelated source-delivery and user-workflow claims. This conservative vocabulary is
not unrestricted natural-language search; unknown combinations can remain empty.
One source-free corpus must cover every record, useful task synonyms,
unavailable capabilities, profile/layer refusal and complete pagination. Run the same
expectations through catalog, MCP and Gallery JSON/HTML. Correct an unsupported test
expectation only from source evidence, retaining its initial and corrected results.

## Compatibility and migration

**Breaking search semantics:** prose-only and unrelated-task keyword conjunctions
no longer match. Use a declared task phrase or exact capability name/ID, narrow with
profile/layer, and read `get_capability` for costs and exclusions. Add a task phrase
only when the implementation actually supports that task; do not add whole paragraphs
to recover accidental full-text matches. No IDs, response schemas or maturity change.

Cursor version 2 prevents continuing a version-1 result order, including unchanged
custom catalogs. Discard old cursors and restart the same query. Metadata edits also
change the normal catalog revision. Applications and consumer pins are not upgraded.

## Alternatives rejected

- Index all prose and parse negation: brittle language rules still combine unrelated
  responsibilities. Wins if an independently evaluated parser proves those boundaries.
- Fuzzy/LLM search or a new MCP server: unnecessary runtime and maintenance cost for
  this catalog. Wins if measured real-task recall remains poor after reviewed vocabulary.
- Silently change expected recommendations: converts implementation mistakes into
  an oracle. Source-grounded evaluation corrections remain explicit and replayable.

## Revisit when

- A source-supported consumer task remains undiscoverable despite declared vocabulary.
- Corpus passes while a real task is wrongly recommended or a refusal is misleading.
- Catalog growth makes phrase maintenance or measured retrieval cost unacceptable.
- MCP, Gallery, or a retained continuation disagrees with the shared task contract.
