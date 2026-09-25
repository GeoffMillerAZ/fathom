# Saved private conversion attempts — 2026-09-13

Scope: the [frozen twelve-case pilot](2026-09-13-conversion-pilot.md), not a bulk
conversion or compatibility release. [Storage contract](../design/html-translation.md#save-and-discover-private-attempts).

## Observed result

The built CLI saved all ten archive-backed cases in the ignored private asset store:
five translated syntax artifacts and five unsupported diagnostic results. Exact get
then reverified all ten originals and returned `replay: matches-current` with
`verification: not-verified`. Unsupported attempts correctly have no generated artifact.
The two MatDash cases remain local source-native references; no archive identity was
fabricated to force them into this store.

Private index revision:
`3f9f1f4eb3bbf9c0022cb71ae6f4a6a44fdbf432bf2a7e6a2d4021acc49dd646`.
Rule digest:
`0db161e2e0e567ae491661614e0115f4042c3ea76b188f867c8242e44e90eb7c`.
Toolchain: Go 1.26.3 darwin/arm64, x/net 0.54.0, required templ 0.3.1020.

| Pilot | Syntax result | Immutable attempt ID |
|---|---|---|
| P01 hero | translated | `syntax@ce8b7273781f1f957b17ae4c2accbf5ee641d46d620969090ddfba77cbf45b3d` |
| P02 pricing | translated | `syntax@902c0695ff832fde0c73bb979cf73aa496098778061dffcff0276c11a6176043` |
| P03 FAQ | unsupported | `syntax@1d8a701acbf534c566328c759fa542bb0e3b8af5ebe07bbb25e5b9f25d34d7f5` |
| P04 table | translated | `syntax@d229374638df455c09200048c34428d2997d4b6e02b7ba1e5631e1f8b5ecd378` |
| P05 form | translated | `syntax@c74d277f872746677804b33b7a5c1b749edfbae86ceb252a390aade99c9dc19d` |
| P06 tabs | translated | `syntax@00a1cfaf1ded78907feec619c5c48f45dc28b293a54bd6ad2ca2f5ba68c61eb7` |
| P07 menu | unsupported | `syntax@c00efb7b1c1002013b8d4a8e81a05c60a87801b938246696ec3dd520f4e56e93` |
| P08 nested drawer/menu | unsupported | `syntax@e1cb57a4c16d71962ad912a78b4162fd4702f62957800d079325295a9680012b` |
| P09 mobile navigation | unsupported | `syntax@a7b9ee5ae6614461eb8629edc1e201840eb402302f1cf99a908702c85717e2ab` |
| P10 Catalyst settings | unsupported | `syntax@a886e200fd5226e4474a5f9f2ba8315c9acd21e739fec60bebcafb2c55e12e15` |

Original catalog file SHA-256 before and after saving:
`23a1c87e21181fb944abf551c293b0154953c682ff915e7965332cf381482b84`.
The original catalog was unchanged; all ten exact reads rechecked original ZIP/member
integrity. Git ignore checks passed for the attempt index and an exact derivative file.
No vendor project, script or dependency installation ran during save or exact access.

## Review and replay evidence

Independent review found two pre-publication issues. Both received synthetic canaries:

- Receipt arrays now have count/depth checks before typed decoding. The same local
  600,887-byte adversarial input containing 200,000 empty mappings was refused with
  9,248 allocated bytes in the review probe, versus 34,936,120 before the fix. This
  measures that decoder refusal only, not UI performance or general memory usage.
- The stored-envelope schema admits historical versions of the named parser/compiler
  modules. Current `Translate`/`VerifyReplay` retain their exact pinned CUE shape.
  Older identities are readable as stale; they cannot be saved as current evidence.
  A different schema/family still requires an explicit reader/migration.

The schema split intentionally changed the rule digest. The full frozen live pilot was
rerun: five outputs compiled/rendered with equal DOM before and after templ formatting,
five archive cases remained unsupported, and two local client specimens remained
source-native. Generated output hashes matched the preceding syntax pilot. No golden,
visual threshold, interaction assertion or compatibility gate was weakened.

Unit/race checks cover origin/replay forgery, immutable conflicts, historical access,
symlinks, abandoned locks, concurrent saves, original preservation, metadata/cursor
integrity, bounded decoding and CLI machine-output compatibility. These do not prove
power-loss durability, rendered fidelity, runtime behavior or production adoption.

The actual MCP stdio boundary then read this saved purchase data: ten list entries,
ten current replay results and ten original-source reads, with text/structured parity
and the same private index revision. No source bodies were logged. The optional-filter
schema initially rejected omitted fields; an adapter-specific optional input fixed it,
with empty-query and limit-only subprocess canaries. Full `task check` and focused
CLI/converter/store/MCP race tests passed. The server binary was rebuilt; no global
Codex/Claude registration was changed or counted as a host-adoption pass.

Read-only replay of the saved-data MCP probe:

```sh
APPKIT_PROBE_SAVED_CONVERSIONS_KIT_ROOT=/absolute/path/to/app-kit go test ./cmd/appkit-mcp -run '^TestSavedConversionsPrivateMCPReadLive$' -count=1 -v
```

## Gallery source comparison evidence

Gallery now reads the same private attempt catalog at `/conversions`, with exact
original/converted source panes, independent line paging, explicit unsupported/stale
states and inert artifact downloads. These are source comparisons, not live previews.
Original-reference pages only construct conversion links; a corrupted attempt index
does not disable them. The [route contract](../design/private-references.md#compare-a-saved-attempt)
records query, privacy and recovery behavior.

The opt-in Gallery handler probe passed against the actual saved purchase pilot:
10 attempts across 3 archives, 5 translated and 5 unsupported; all 10 exact reads
reported current replay and retained `not-verified`. All original sources remained
accessible. Five `text/plain` attachment downloads matched 43,622 total saved artifact
bytes and their hashes. List/exact JSON matched the shared library; HTML parsed with
inert source, one main landmark, unique IDs and the private response headers. The run
took 16.442 seconds for repeated integrity-checked route calls, not a browser benchmark.
No private source bodies were logged, written or executed by the probe.

```sh
APPKIT_PROBE_SAVED_CONVERSIONS_GALLERY_ROOT=/absolute/path/to/app-kit/data/design-assets go test ./apps/gallery -run '^TestSavedConversionsGalleryLive$' -count=1 -v
```

Synthetic tests cover same-origin/loopback denial, GET-only behavior, malformed query
and encoded aliases, missing/corrupt attempts, independent original recovery, historical
receipts, hostile source/path escaping, exact downloads and independent pane context.
Line windows reconstruct newline/CRLF/Unicode source bytes and refuse oversized display
without truncation. Independent source review found no actionable P0–P2 issues.
Full `task check`, Gallery race tests and documentation-link/budget checks passed.

The rebuilt local Gallery also served the real 10-entry list and hero comparison over
loopback HTTP; the hero download hash and original-catalog file hash matched the values
above. Chrome connection retry still failed. No screenshot, keyboard, narrow-screen,
contrast, rendered-fidelity or real-browser claim follows from these HTTP/DOM checks.

## Still unearned

No target is certified. Source-native baselines, browser/task/keyboard/contrast proof,
runtime lifecycle integration, complete-page adaptations and consumer payoff remain
required. Rendered original/adapted comparison and promotion into verified-target discovery are separate
work. Saving a rejected attempt preserves the reason and original fallback; it does
not repair that family or fulfill the full G1–G6 objective.
