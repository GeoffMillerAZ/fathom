# crossplane — provenance and divergences

LB-Q-crossplane, design `docs/design/language-breadth.md` §4, §13; the iac wave (EXCELLENCE T3). A
doc-engine dialect (no `tags.scm`, no upstream grammar, no incumbent). The decode is `gopkg.in/yaml.v3`
through the doc engine. Fixtures are written for this dialect; the shapes follow Crossplane's
documentation and the Upbound reference platforms (Apache-2.0).

## How a file is claimed: every document, never some

`detect.content_sniff` names an `apiVersion` of `apiextensions.crossplane.io/`, `pkg.crossplane.io/` or
`meta.pkg.crossplane.io/` (any version). `fusion.crDispatch` (LB-F149) claims a file only when every
non-empty document passes it and the classifier would otherwise call the file a plain Kubernetes manifest.
A composite resource or claim (a user-defined API group) is Kubernetes, so a file that pairs an XRD with
its example XR is MIXED and stays Kubernetes (`fusion.CRMixed`).

## What it models

- **The object** (`CrossplaneResource`, `resource`): `Kind/<name>` (`Kind/<namespace>/<name>`), the
  Kubernetes dialect's naming: an XRD, a Composition, an EnvironmentConfig, a Provider, Function or
  Configuration package, a package's meta Configuration.
- **An XRD's types** (`CrossplaneCompositeKind`, `CrossplaneClaimKind`, `service`): `spec.names.kind` and
  `spec.claimNames.kind`; **versions** (`CrossplaneVersion`, `section`).
- **A Composition's parts** (`CrossplaneComposedResource`, `element`): each `spec.resources[]` and each
  pipeline step's `input.resources[]`, signed by the base's `apiVersion/kind`; **pipeline steps**
  (`CrossplaneStep`, `step`), signed by their function.
- **References** (`Calls`, L2): `compositeTypeRef.kind`, a step's `functionRef.name`, `dependsOn[]`
  (provider, function, configuration), `defaultCompositionRef` and `enforcedCompositionRef`, as written.
- **Cross-file references** (L3), `service-name`: a Composition's `compositeTypeRef.kind` to the XRD type
  of that name in another file; a step's `functionRef` to the `Function/<name>` object; an XRD's default or
  enforced composition to the `Composition/<name>` object.

## Divergences, each disclosed (and where it is a ceiling, a -rest row)

- **The composite type is matched by kind, not by group and kind.** The engine cannot split an
  `apiVersion` into its group, so two XRDs of one kind in two groups are ambiguous and draw no edge. The
  oracle compares the group too (a collision it would split, the dialect drops).
- **A claim or composite resource to its XRD stays Kubernetes.** It needs a shared Kubernetes-dialect
  join from a group and kind to the CRD or XRD declaring it, coordinated with wave pc's constraint to
  ConstraintTemplate join (LB-Q-crossplane-rest, LB-X row).
- **Managed-resource patches** (`fromFieldPath`, `toFieldPath`) are not read by the product dialect
  (L2 at most; they name fields of resources this dialect does not model). A separate calibration
  probe walks directional field paths at tier C; it creates no product reference or binding edge.
- **Native validation ran in the 2026-10-26 calibration probe.** The official Crossplane CLI 2.5.0
  validated local core and pinned Upbound source CRDs without fetching images. It checked 21
  provider-matched literal bases, all valid; all 21 came from three network subjects selected after
  an initial six-subject probe found zero matches. Their versions had no preregistered selection
  rule, so this is exploratory evidence. Validation did not enumerate nested definitions or bind
  patches. P-T1 grades the full 244-file independent walk at tier C: 428/428 definitions, 86/86
  references and 33/33 cross-file references; L1/L2/L3 are good. Owner-tool probes have not
  established a native L2/L3 binding certificate or an exhaustive ceiling. The old registry-fetch
  refusal is superseded by the owner's calibration-only provider-schema fetch authorization.
  The product still has no downloader.

## Calibration

Subjects in `calibration/corpus.d/crossplane.yaml`; the oracle is `calibration/oracles/crossplane` (an
independent `gopkg.in/yaml.v3` walk). Native probe method and nine-subject assessment:
`docs/assessments/2026-10-26-crossplane-provider-crds.md`.
