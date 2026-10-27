# simulink -- provenance and divergences

LB-Q-simulink, design `docs/design/language-breadth.md` sections 13 and 14. A native dialect
(`internal/extractor/simulink.go`, LB-F587): `archive/zip` and `encoding/xml` over a `.slx`
package in memory, a brace-line reader over a classic `.mdl`. No grammar, no incumbent, no
code copied: the layout eras were read from MathWorks' data-explorer-core v1.46.0 (BSD-3)
`ModelStructureScan.ts` and `SlxParser.ts` comments, and every fixture was written for this
dialect.

## The binary gate

A `.slx` is an OPC zip, which the NUL sniff calls binary. The manifest's `match.container`
(`opc-zip`, `.slx`) is what lets it through `fusion.binaryExcluded` (ADR-021 decision (1),
amended 2026-10-24), which checks the four-byte zip signature only; the reader's caps refuse
the rest, each enforced while it reads (4,096 entries; 32 MiB inflated; 200 bytes out per
compressed byte consumed; 1,024 levels deep; 524,288 elements and 65,536 blocks; 64 KiB per
kept value; ADR-021 lists them). Ingest's 2 MiB cap applies to the compressed bytes, so a large library
(WEC-Sim's MOST_Lib.slx, 5.97 MB) is counted oversize.

## What it models

- **Symbols**: the model (`module`, named by its file's base name, line 1), then each block of
  the diagram's System, depth first, named by its full path from the model as Simulink's
  getfullname writes it (a `/` in a name doubled); a block that owns a System is a subsystem
  (`element`, a container), any other a block (`element`). The signature is the block type.
  Names are compared whole (`name_match: identity`): they hold spaces and `/`; a line break
  (`&#xA;`, `\n`) reads as one space so a symbol name stays one line.
- **Lines**: a block's element (or brace node) in the part that holds it: `blockdiagram.xml`
  before R2020a, `systems/system_<n>.xml` from R2020a. The range is part-relative, not a line
  of the `.slx` file, which has none.
- **References (L2)**: each signal or connection line from its source block to each
  destination block, branches included, as the source block's calls.
- **Cross references (L3)**: each model reference (graphicalInterface.json from R2024b,
  graphicalInterface.xml, or the diagram's inline interface) as a `module-source` reference by
  the model's name, which the doc-xref pass joins to the one model of that name in another
  file. Several models of one name (FMT-Model's Controller.slx in seven directories) join
  none: MATLAB's path order decides there, and fathom never guesses it.

## Not read

- Template blocks (BlockDefaults, BlockParameterDefaults), Stateflow charts' contents
  (`stateflow.xml`), masks, parameters, configuration sets, data dictionaries.
- A text-package `.mdl` (`# MathWorks OPC Text Package`, `# Simulink Text Package`): disclosed,
  read as nothing (2 of etherlab-examples' 16).
- The MATLAB code a model calls (MATLAB Function blocks, callbacks, S-functions): a handoff to
  the matlab dialect, not this one.

## Reading (2026-10-26)

Measured at 01b45866, tier C only (owner ruling 2026-10-24 (d)): `calibration/oracles/simulink-walk/walk.py`,
a zipfile and expat walk that reads model references from the ModelReference blocks rather than the
graphicalInterface part. It mirrors the reader's decisions, so its agreement is capped at good: every
level is good at most. Five subjects (WEC-Sim v7.1.0, Thermodynamics v1.3.1, LADAC, FMT-Model,
etherlab-examples); the queue's sixth, PID_MATLAB_Learn, did not resolve to a repository and is not
measured. Throughput was re-timed with `indexbench lang-retime` (3 runs, median 21.69 MB/CPU-s on
darwin/arm64, the machine class the reading's note names; the VM's 9.53 is withdrawn).

Cross-checks (ungraded, `calibration/oracles/simulink-walk/CROSSCHECK.md`): miss-hit-core 0.9.44
reproduced 2026-10-25 in round 3's VM, 163 of 178 `.slx` parsed (15 Thermodynamics models refused by its
parser), every block it reports agrees (105 of them only by a trailing line break the reader keeps as a
space), and the reader adds 3,660 chart-internal blocks; data-explorer-core v1.46.0 is unreproduced (round
2's figures only); rustylink and SLX2MDL did not run. No tool checks the signal lines, the whole of L2's
reference family. L3 rests on 2 resolvable model references in one subject (the bar is 20 across 2), so the
level stays L2.

Caps: every cap enforced while the reader reads, the input as tested, the test, the refusal. Each test is
in `internal/extractor/` and runs as `go test -race -count=1 -run '^<Test>$' ./internal/extractor`
(`GOWORK=off`); `(cap+1)` marks an input exactly one past the cap, any other input is further past it, and a
cap with no case at all is named under Gaps.

- Caps: entries, 4,096; 4,097 parts (cap+1); `simulink_test.go` `TestReadSimulinkModelCaps/entries`; ErrSimulinkCap `4097 entries, cap 4096`.
- Caps: inflated bytes, 32 MiB; 33 parts of about 1 MiB; `TestReadSimulinkModelCaps/inflated_bytes`; ErrSimulinkCap `parts inflate past`.
- Caps: inflate ratio, 200:1; a 4 MiB run of spaces (about 1,000:1) and a false declared size; `TestReadSimulinkModelCaps/ratio` and `TestReadSimulinkModelRealRatio`; ErrSimulinkCap `inflates past 200:1`.
- Caps: nesting depth, 1,024; 1,025 mdl levels (cap+1), 1,048,576 nested slx elements, a spliced chain deeper than any part; `TestReadSimulinkModelStructureCaps` (`deep mdl`, `deep slx`, `wide spliced`); ErrSimulinkCap `nest past`.
- Caps: elements, 524,288; 524,289 mdl nodes (cap+1) and `flat flood` slx; `TestReadSimulinkModelStructureCaps/flat_mdl`, `TestReadSimulinkModelAdversarial/flat_flood`; ErrSimulinkCap `past 524288 elements`.
- Caps: blocks, 65,536; the block floods read at the cap (`TestReadSimulinkModelHeapBound`, no case past it); see Gaps.
- Caps: kept value, 64 KiB; an 8 MiB attribute, 8 MiB text and a 1 MiB mdl attribute; `TestReadSimulinkModelAdversarial` (`huge attribute`, `huge kept text`, `huge mdl attribute`), and 64 KiB + 1 in `TestReadSimulinkModelRefCaps`; ErrSimulinkCap `is longer than 65536 bytes`.
- Caps: start tag, 1 MiB; a 1 MiB attribute value (a tag a few bytes past 1 MiB); `TestReadSimulinkModelAdversarial/long_start_tag`; ErrSimulinkCap `a start tag spans past`.
- Caps: attributes per start tag, 256; 257 (cap+1); `TestReadSimulinkModelAdversarial/attribute_flood`, `TestSimulinkDecoderStrict` (bare names); ErrSimulinkCap `a start tag carries past`.
- Caps: block path, 16 KiB; 16,385 bytes (cap+1); `simulink_shapes_test.go` `TestReadSimulinkModelPathCap`; ErrSimulinkCap `a block path is longer than`.
- Caps: bytes of block paths, 32 MiB; 2,040 children of a 16,000-byte name read at the cap (`TestReadSimulinkModelHeapBound`, `long names at the cap`), no case past it; see Gaps.
- Caps: bytes of signal endpoint paths, 32 MiB; 64 children of a 16,000-byte parent, every pair joined; `TestReadSimulinkModelSignalBytes`; ErrSimulinkCap `signals hold past`.
- Caps: bytes of emitted Call paths, 32 MiB; 3 sources named alike over 1,000 long destinations; `TestSimulinkExtractDocOutputBound`; `ErrModelRefused` wrapping ErrSimulinkCap `emitted calls hold past`.
- Caps: distinct signals, 65,536; 257 short children (65,792 signals) and 700; `simulink_shapes_test.go` `TestReadSimulinkModelCountCaps`; ErrSimulinkCap `past 65536 signals`, asserted; 256 children (65,536) read in full.
- Caps: emitted calls, 65,536; 64 sources x 1,024 destinations = 65,536 reads in full, one more line of one call = 65,537 (cap+1) is refused; `TestReadSimulinkModelCountCaps`; ErrSimulinkCap `past 65536 emitted calls`, asserted, with no symbols.
- Caps: model references, 4,096; 4,097 (cap+1) from the json, the xml, the inline interface and an mdl; `TestReadSimulinkModelRefCaps`; ErrSimulinkCap `past 4096 model references`.
- Caps: graphicalInterface.json strings and depth, 64 KiB per string and 1,024 deep; a 64 KiB + 1 BlockPath, ModelName and skipped string, and 1,025 nested arrays; `TestReadSimulinkModelRefCaps`; ErrSimulinkCap `a string is longer than` and `JSON values nest past`.
- Caps: model references spend the shared element budget, 524,288; 2 references read and 3 are refused after a part that leaves room for 2 (cap+1); `TestReadSimulinkModelJSONRefsSpendElementBudget`; ErrSimulinkCap `past 524288 elements`.
- Caps: a model-references part that does not parse or has data after its top-level value refuses the whole model; `TestReadSimulinkModelRefsPartRefused`; ErrSimulinkCap naming the part.
- Caps: mdl line, 4 MiB; a Note line of 4 MiB of text plus its key (7 bytes past); `TestReadSimulinkModelAdversarial/long_line_mdl`; ErrSimulinkCap (the scanner's token-too-long).
- Gaps: no test refuses a model past the block cap (65,536) or the block-path byte cap, and none sits exactly one past the value, start-tag, signal-byte, call-byte or mdl-line caps (the inputs above are further past), so an off-by-one at those caps is not killed by a test.

Mutation: `internal/extractor/simulink.go` `resolveSignals`, the signal dedupe `seen` keyed on the endpoints'
paths (`map[string]bool` keyed on `Path+"\x00"+Path`) in place of the index pair, run at d3af7eff6 in a
scratch worktree: `GOWORK=off go test -p 2 -count=1 -v -run '^TestSimulinkResolveHashWork$' ./internal/extractor`
FAILs: `simulink_heap_test.go:307: a 16 KiB path took 9.820338s of CPU to resolve, a one-byte one 31.831ms: want at most 113.662ms, so path length does not reach the dedupe` and `--- FAIL: TestSimulinkResolveHashWork (15.10s)`; restored, `--- PASS: TestSimulinkResolveHashWork (0.10s)` and `ok github.com/geoffmilleraz/fathom/internal/extractor 0.556s`.

Mutation: three more at d3af7eff6, each killed then restored green: no trailing-data check in
`jsonModelRefs` (`TestReadSimulinkModelRefsPartRefused` FAILs on `json data after the object`, `json second value` and
`json data after a non-object`); `budget.take` removed from `jsonRefList` (`TestReadSimulinkModelJSONRefsSpendElementBudget`
FAILs `3 references: 3 model refs, <nil>; want ErrSimulinkCap past 524288 elements`); the call cap moved to
`calls > SimulinkMaxCalls+1` (`TestReadSimulinkModelCountCaps` FAILs `65,537 calls: 1089 blocks, 1025 signals, 1090 nodes, <nil>; want ErrSimulinkCap`).

Not measured: L3 rests on 2 resolvable references; no independent tool checks the signal lines; every
heap figure in ADR-021 is from one Linux VM (VmHWM) except the macOS arm64 re-measure it names.
