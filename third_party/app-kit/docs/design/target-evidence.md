# Revision-bound target evidence

[ADR-0042](../adr/0042-separate-evidence-integrity-from-certification.md) implements the
integrity portion of [the conversion contract](conversion-pipeline.md#catalog-and-delivery-contract-to-implement).
[ADR-0043](../adr/0043-private-historical-evidence-catalog.md) adds private historical
persistence and shared CLI/MCP/Gallery metadata; [ADR-0055](../adr/0055-private-retained-image-inspection.md)
adds private PNG inspection. Neither layer certifies a target or introduces a public runtime API.

## Present boundary

`internal/designreceipts` accepts separately authored contracts and observation receipts.
CUE owns their shapes; Go encoders produce exact canonical compact JSON. Map key order
is deterministic. Unknown/duplicate/aliased fields, lossy Unicode, trailing data and
noncanonical encodings are refused before CUE admission. Metadata is limited to 256 KiB
per document; inventories/slots and exact supplied artifact bytes are bounded separately.
Each gate/result permits64 slots/evidence entries under [ADR-0061](../adr/0061-bound-larger-historical-capture-sets.md).
**Reader migration:** rebuild private CLI, Gallery and MCP before selecting packets
with more than32 entries per gate. Old binaries reject those new packets; old packet
bytes/IDs remain unchanged. All document/content/image limits remain in force.

A contract binds candidate, source archive/revision, profile, variant and output hash;
source, transformer, config, patch, runtime, output, fixture, toolchain and baseline
inventories remain separately named. Empty config/patch/runtime lists mean no entries
were declared, not evidence that the dependency closure is complete. The caller reviews
that closure. Source bytes matching a hash do not establish membership in an archive.

All nine gates must be present: reproducibility, structure, visual, interaction,
lifecycle, security, data, performance and integration. Each applicable gate declares
its scope and exact evidence slots with report/image/review/measurement roles. A pass
needs every declared slot; partial observations may retain a subset. Content must match
every declared hash/size; unexpected content is refused. A shared slot must be explicitly
declared in each gate with consistent role and identity; it cannot shadow an input ID.

Not-run/failed observations require reasons. N/A requires a matching contract exclusion
reason and nonempty review assertion, with no evidence slots. This is structural review
metadata, not authenticated independent approval. Full-gate status must not be inferred
from a narrower report; partial historical proof can accompany a not-run full gate.

## Calling the checker

1. Select a trusted contract and expected source/output/contract pins outside the receipt.
2. Encode contract/receipt with `EncodeContract` and `EncodeReceipt` (or retain their exact bytes).
3. Load explicitly authorized private artifacts into a map of logical IDs to bytes.
4. Call `Check(contractBytes, receiptBytes, expected, content)`; no path or command is executed.
5. Display returned claimed statuses separately from `integrity=matched`. Even an all-pass
   ledger returns `verification=not-verified`. An error returns no partial success report.

Never calculate trusted expected pins from an untrusted receipt or newly read artifact.
An in-memory content map provides no filesystem confinement by itself; the separate
store owns bounded reads, provenance admission and private access. This checker performs
no network access, writes, report execution, source adaptation or renderer selection.

## Private historical catalog

`internal/designevidence` uses `evidence/` beneath the existing selected design-assets
root. An immutable canonical bundle contains the exact contract, receipt, declared
source member and supplied content bytes. Its SHA-256 is its `evidence@<sha256>` ID.
Original archive and syntax-attempt identities are unchanged. `catalog.json` holds
bounded CUE-validated metadata; no bundle bytes are returned by discovery or exact get.
Gallery's separate image route is the narrow exception described below; metadata stays byte-free.

Explicit `Publish`/`PublishBundle` requires independent expected contract/source/output
pins, runs `designreceipts.Check`, and verifies retained archive/member membership.
Private 0700 directories/0600 files, anchored regular-file reads, exclusive writer lock,
no-clobber immutable bundles and index-last atomic replacement bound publication.
The owner controls the root. Hostile same-user races, ACL audits and power-loss durability
are not proven. An abandoned lock needs operator review; there is no automatic repair,
overwrite, deletion or garbage collection. Back up this root to retain the evidence.

Encoded bundles are limited to 92 MiB and supplied decoded content to 64 MiB. The index
is limited to 16 MiB/4,096 records. Search defaults to ten results, caps at fifty, and
binds continuation cursors to the exact revision and filters. A missing optional catalog
is empty; damaged configured state is an error. Originals remain separately retrievable.

| Surface | Search recorded metadata | Exact historical recheck |
|---|---|---|
| CLI | `appkit data assets-evidence` | `appkit data assets-evidence-get -id evidence@…` |
| MCP | `search_design_evidence` | `get_design_evidence` |
| Gallery | `/target-evidence` | `/target-evidence/evidence@…` |
| Gallery JSON | `/api/target-evidence` | `/api/target-evidence/evidence@…` |

CLI search accepts `-root`, `-archive`, `-subject`, `-profile`, `-cursor`, `-limit`.
MCP/Gallery use `archive_id`, `subject_id`, `profile`, `cursor`, `limit`; exact lookup
uses the record ID. Gallery also retains its `theme` and native navigation/filter state.
MCP tools reuse the existing configured private root; a running older server must be
rebuilt/restarted before it exposes these additive tools. No host registration is implied.

Save only through explicit CLI `appkit data assets-evidence-save -root ROOT -bundle FILE
-contract-sha SHA -source-sha SHA -artifact-sha SHA` or the internal store API. The CLI
requires a bounded regular, non-symlink bundle input and never derives trusted pins from
it. Argument/operational failures exit 2; successful metadata uses JSON stdout. No
network, conversion, command execution, installation or consumer mutation occurs.

Search reports **recorded** integrity and gate claims; it does not re-read bundles.
Exact get rechecks canonical bundle identity, every supplied artifact, index agreement
and retained original membership. These are historical pins, not independently selected
current transformer or vendor revisions. Neither operation authenticates authored
observations/review, executes semantic reports, or completes missing evidence gates.
Even nine claimed passes remain `verification=not-verified`.

### Inspect retained screenshots

Catalog references link to `/target-evidence?subject_id=EXACT_CAPABILITY_ID`. A record
may have no images or may be absent; neither condition triggers imports or conversion.
In a record, choose **Capture** and optional **Compare below**, then **Show captures**.
Native GET controls preserve filters/theme and stack at most two full-width images.
Gallery theme changes leave captured pixels unchanged. Full-size PNG links retain
browser Back; captures are historical observations, not live specimens or behavior proof.

`/target-evidence/{id}/images/{slot}` returns only receipt-declared PNG image slots after
full historical integrity/original checks plus bounded decoding (8 MiB, 8192-pixel edges,
16 million pixels). Exact retained bytes/hash stay unchanged. Other formats, arbitrary
artifacts and cross-origin access are refused. No image bytes enter MCP or metadata APIs.
Invalid images retain identities/recovery, without a broken image. Successful detail HTML
permits same-origin images; validated raw PNGs alone permit browser-generated inline styling.
Scripts, frames and network proxies stay denied. Retention remains an explicit CLI action.
The [viewer assessment](../assessments/2026-09-15-private-image-inspection.md) records scoped proof and limits.

## Retained evidence and remaining gates

The first consumer is a source-free pinned metadata fixture importing retained P06
static-link evidence. It is explicitly retrospective: no fabricated pre-run freeze or
gate certification. The original and output bytes remain in the private asset store.
The opt-in retained-file check must fail on a missing artifact when requested; absent
private data in an ordinary source-only test run is an explicit skip, not live proof.
[Retained-file evidence and exact identities](../assessments/2026-09-14-target-evidence-integrity.md)
record ten matched artifacts and nine incomplete gates. MCP serves this guide via
`get_kit_decision("target-evidence")` and `kit://target-evidence`; the separate tools above
now expose historical target records without asset bytes.

A separate [P06 prospective task proof](../assessments/2026-09-14-navigation-semantic-proof.md)
now freezes 24 Links input/zoom/theme cases before collecting browser observations.
`internal/navigationproof.Check(expectedContractSHA, contractBytes, reportBytes)` derives
scoped navigation, focus, geometry and contrast outcomes, with planted failures and an
embedded checker/schema digest. It accepts no authored pass flag and always returns
`verification=not-verified`; plausible invented observations are not authenticated proof.
This family-specific checker is separate from the generic integrity layer and does not
automatically complete any full gate or populate target records.

The explicit [P06 retention workflow](../assessments/2026-09-14-private-evidence-catalog.md)
now saves two immutable records: the ten-artifact historical packet and a 41-artifact
aggregate retaining the separate prospective task contract/report, oracle and images.
Both aggregate contracts are retrospective and all nine full gates remain not-run.
The task's pre-run freeze is retained, not retroactively asserted for the aggregate.
CLI/Gallery live metadata and MCP/Gallery protocol parity are separate proofs, not certification.

[P07/P08 retention](../assessments/2026-09-15-overlay-evidence-retention.md) adds two private
packets with 52 menu/44 drawer PNGs: original, typed, failed zoom baseline and repair history.
Those four records retain120 images. Both new packets keep interaction failed and all
nine gates incomplete; neither the image viewer nor metadata tools promote these claims.

[P09 retention](../assessments/2026-09-15-navbar-evidence-retention.md) adds one navbar
packet with114 original/reference, failed-base, runtime-repair and delivered-repair
captures. Five private records now retain234 images. Its54-slot largest gate requires
the ADR-0061 reader; interaction stays failed and all nine gates remain incomplete.

Still required: complete input/dependency closure, prospective full-gate contracts and
independent review, remaining semantic checks, full applicable pilot gates, consumer
history/promotion and certified-target admission. Neither partial checker earns compatibility or certification.
