# Repository charter

<!-- Only for a repo holding more than one product, or components with different
     evolution paths (agentic-kit/docs/project-intent/monorepo.md §1). Intent about the
     container, not any product. Rarely changes. Under 120 lines. -->

## Why one repository

<!-- What would be lost by splitting: one gate, one audit, one compatibility promise,
     atomic cross-component change. Not "convenience". -->

## What belongs here, what does not

<!-- The membership rule and the exit rule. -->

- Belongs when: <rule, e.g. "it has a named consumer">.
- Leaves when: <rule, e.g. "zero consumers verified at a wave end">.
- Never here: <the kinds of thing that go elsewhere, and where>.

## Component map

<!-- Kind: product | kit | library | tooling | example | contract.
     Intent: root | <path>/docs/intent. Evolution vocabulary: stays ·
     split when: <trigger> · merge into <target> when: <trigger> ·
     retire when: <trigger> · inbound from <repo> when: <trigger>. -->

| Component | Kind | Intent | Consumers | Evolution |
|---|---|---|---|---|
| `<dir>` | kit | root | <named consumers> | stays |

## Dependency direction

<!-- One sentence a new package must not violate, and what enforces it. -->

## Adding, promoting, extracting, retiring

- **Add**: <the artifacts that change together>.
- **Promote**: <the one-way event and where it is recorded>.
- **Extract**: <nested module first; own tag cadence; the trigger that turns it into a repo>.
- **Retire**: <zero consumers verified; changelog Removed; tracking disposition>.

## Ecosystem

- Downstream: <consumer repos and how they are checked>.
- Upstream: <where doctrine or decisions flow in from>.
- Shared decisions inherited: <`<repo> D<n>` citations; never renumbered>.
