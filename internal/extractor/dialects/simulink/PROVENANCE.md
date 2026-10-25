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
amended 2026-10-24); the reader's caps (4,096 entries, 32 MiB inflated, 200:1 per part) refuse
the rest. Ingest's 2 MiB cap applies to the compressed bytes, so a large library
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

## Reading (2026-10-25)

Tier C only (owner ruling 2026-10-24 (d)): `calibration/oracles/simulink-walk/walk.py`, an
independent zipfile and expat walk that reads model references from the ModelReference
blocks rather than the graphicalInterface part. Every level is capped at good. Five subjects
(WEC-Sim v7.1.0, Thermodynamics v1.3.1, LADAC, FMT-Model, etherlab-examples); the queue's
sixth, PID_MATLAB_Learn, did not resolve to a repository and is not measured. L3 rests on 2
joined model references (FMT-Model's harnesses; 7 more are ambiguous by name), a thin
denominator. The queue's cross-checks (data-explorer-core, miss-hit-core, rustylink,
SLX2MDL) were not run in this reading.
