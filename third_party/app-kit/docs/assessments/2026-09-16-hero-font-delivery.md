# P01 font delivery: identical tested design, smaller cold payload

Date: 2026-09-16. Private G1/G4 experiment, not a certified target or consumer upgrade.
The candidate passes all eight cold performance observations at499,059 decoded-body
bytes, versus the original658,700:159,641 fewer bytes (24.2%). It stays25,229 bytes
below the unchanged524,288-byte budget. All eight warm observations still fail the
request-bound cached-font body check. No budget, original asset or baseline is replaced.

## Implementation and preservation

The [retained-source runner](fixtures/hero-adapter/FONT.md) uses a private pinned
FontTools4.65.0/Brotli1.2.0 environment. Installed HarfBuzz14.3.0 cannot decode this
WOFF2 input through either tested loader; its `hb-shape` instead receives TTF copies
produced from the original and derived fonts. Browsers receive WOFF2, not those TTFs.
No Python/client dependency is added to the kit or any application.

| Property | Evidence |
|---|---|
| Original controlled Inter | 352,240 bytes; SHA `693b77d4f32ee9b8bfc995589b5fad5e99adf2832738661f5402f9978429a8e3` |
| Candidate WOFF2 | 192,380 bytes; SHA `a328e08221a9a70a27c193d3a2fc2fbcfa5b238d8360c3cc796341130b8b1eb0`;45.4% smaller |
| Metadata | All90 name records retained, including feature/license records |
| Repertoire | 725 selected cmap entries;2,127 original-only entries retain full-font fallback |
| Glyphs | 1,629 of2,937 retained, including layout closure; outlines, advances/bearings, hints and variation deltas checked |
| Variation | Weight100–900 and optical size14–32; fvar/avar/STAT/MVAR exact; other original layout/variation table presence retained |
| Repeatability | Two fresh builds produce identical WOFF2, original/subset TTFs and manifests |
| Shaping | 70 original/subset HarfBuzz pairs have equal glyph names, clusters, advances and offsets for the selected text cases |

The original face remains unrestricted and first. The bounded subset face comes last,
with the same family/style/weight/display descriptors and its selected unicode ranges.
Only these face declarations change; the HTML, rest of the CSS, three image assets,
font outlines and application-script policy remain unchanged. This is a separate
delivery candidate, not a replacement for the original or a general font API.
Mixed-script pages may load both font files, costing more than the original alone;
each real locale/content workload needs its own budget. This candidate is not a
blanket recommendation to subset every catalog font in this way.

## Browser and negative-control evidence

Chrome152.0.7977.84 runs in a fresh isolated profile. Four paired hero views cover
390/1440px, light/dark, height1000, DPR1 and reduced motion. Paired PNG bytes, visible
text and element geometry match exactly; native Tab order and body identities pass.
This is viewport fidelity, not a complete accessibility or portal-baseline claim.

The100-paragraph fixture combines ten strings with five weights and two optical
sizes. It includes ligatures/kerning, precomposed/decomposed accents, dotted/dotless
letters, sharp-S case boundaries, IPA, Greek/Cyrillic combining clusters and enclosing
marks. Its original/candidate full-page PNG and geometry are identical. Every tested
text parent uses custom Inter, not a system substitute. Separate pages request only
the subset for Latin and only the original for Greek; all actual bodies match their pins.

The missing-subset control returns one exact404. It fails its ordinary asset gate,
loads the exact full original as fallback, and preserves all other response identities.
Detection passes; the faulted lane remains explicitly non-OK. Positive lanes never
permit the extra full-font request for the fixed Latin hero. Source-identity and
existing-output refusal controls also pass. The two new helper suites add five Node
tests to `task check`; the runner executes those plus the16 unchanged performance tests.

## Retained failures and reviewed instrument correction

- `build-v1`: FontTools removed feature-name297 despite all-name options. The
  corrected generator copies the full original name table before serialization and
  retains exact post-serialization equality; it does not waive the check.
- `font-proof-lfxukU`: the disjoint unicode-range complement passed all four hero
  pairs but forced an accented IPA cluster onto Lucida Grande. It remains failed.
- `font-proof-QJ0ydo`: the unrestricted fallback repaired all100 typography cases,
  but the instrument still rejected unused eligible font faces through `check()`.

The independent review admitted overlapping fallback only under unchanged actual-font,
exact-PNG/geometry and delivery tests. The subsequent readiness correction follows the
[FontFaceSet matching contract](https://www.w3.org/TR/css-font-loading/#font-face-set-check):
`check()` is retained as a diagnostic, not taken as glyph evidence. The candidate must
settle font/layout loading, show actual custom Inter at each visible text parent, and
pass exact response identities. A missing family or failed asset cannot pass those
checks. The negative control demonstrates detection. Original benchmark readiness is
unchanged, and no old failed receipt is relabeled as passed.

## Performance observations and remaining boundary

The same M4/Darwin25.6.0 arm64/Chrome environment, two browser launches,10Mbps down,
80ms latency and4xCPU conditions as [the baseline](2026-09-16-hero-performance.md)
apply. Counts are observations, not averages used to mask a failed cell.

| Candidate measurement | Eight cold observations | Eight warm observations |
|---|---|---|
| Uncached decoded bodies | 499,059 bytes | 2,553 observed HTML bytes |
| FCP | 288–304ms | 128–148ms |
| Sampled LCP | 520–528ms | 128–148ms |
| Readiness upper bound | 618.4–654.1ms | 154.9–173.3ms |
| Instrumented JS heap | 1,315,636–1,315,988 bytes | 2,122,460–2,132,692 bytes |
| Per-cell result | All budgets/identity/tasks pass | All fail cached subset-font body identity |

Warm navigation has six cache hits and no static-asset server requests. Chrome does
not expose the cached subset body through `Network.getResponseBody`; stored bytes,
painted-font facts or cold responses do not substitute for that request-bound proof.
The full performance matrix therefore remains failed. These are local emulated
conditions, not a physical-phone, field-performance or cross-browser guarantee.

The separate [cached-font protocol diagnostic](2026-09-16-cached-font-protocol.md)
tests larger buffers, both durable-message interfaces, request-bound streaming and
raw Fetch response interception. Cold streaming/Fetch returns the exact font; warm
remains unavailable with cache controls intact and no intercepted font response.
This narrows an instrumentation limit without changing the failed performance gate.

## Evidence locations

Private parent: `/Volumes/m2ext/appkit-evidence/appkit-hero-font-candidate.AGTWfHOp/`.
Public tooling wheels and their bootstrap source/lock/receipt stay there; the app
dependency graph and global configuration are unchanged. Licensed input copies stay
inside private proof roots, never these maintained fixture sources.

| Receipt | SHA-256 |
|---|---|
| `font-proof-hCKm6P/font-proof-receipt.json` | `643dd22ebae49a48425cc07ffee94f1c8764cf47e0e933711ea7936be3a2c909` |
| `font-proof-hCKm6P/font-browser-receipt.json` | `32954a644928dd196e3dd4aacd5253691aba311ff2b7e0bc3ab1056e5fbbb579` |
| `hero-performance-YtZ7q6/receipt.json` | `298af19ddb0fdb550130cdb316ef4d758a353742775ca6f35bfad8940218e2cb` |
| `hero-performance-X5MAXO/receipt.json` (current-runner original control) | `4db4e44aebc7e43a0ef02f3943b67ab95743a341ce48fa22d55aee10758868ab` |

The font packet retains ten executed-source/config snapshots, two builds, command
logs and13 captures. Performance retains nine snapshots and16 captures. Normal
browser exits are recorded, and loopback servers close. Full target verification,
real-consumer delivery and G1–G6 acceptance remain open; no promotion follows.

The current-runner original control reproduces eight658,700-byte cold failures and
eight warm font-body failures. Its cold readiness range is766.3–784.3ms; timing/heap
remain within budget. The helper changes did not silently repair the old baseline.
An independent retained-packet audit passes48 checks across executed source, input
copies, both builds,147 commands, raw shaping pairs, captures, browser facts and
owned PID absence. `font-proof-hCKm6P/audit-font.json` SHA:
`74e7591414f2f3fa21f97870c790ecf38562d308b905944013cefbefb53628f6`.
The separate85-check performance audit replays both frozen oracles, verifies
snapshots/inputs/logs/32 captures and confirms four exited browser PIDs. Its
`audit-perf.json` SHA is
`4264f92e95951a0ff0ad7cb96a4bfc9ecdf8d554ea6475fead82e6398913251c`.
Scoped intent checks cover18 documents,432 relative links,64 ADR shapes/index
entries and root/changelog budgets; this is not a full doctor or host-wiring audit.
