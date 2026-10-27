# Simulink cross-checks (ungraded)

Counts only, no model content. The graded oracle is `walk.py` (tier C); these runs check
fathom's block reading against tools that share no code with it. Neither is graded, neither
is vendored: each runs from `workspace/tools` and is never committed.

## miss-hit-core 0.9.44 (`miss_hit_core.s_parser`, GPL-3), reproduced 2026-10-25

Reproduced in round 3's VM at the round-3 reader (branch `claude/simulink-r3-20261025`), over
the five pinned subjects of `calibration/corpus.d/simulink.yaml`. miss-hit reads `.slx`
only; a block compares by SID and its path local to the model.

| Subject | .slx | parsed | refused | agree | trailing break | other path | missing in fathom | fathom only |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| FMT-Model | 22 | 22 | 0 | 26,122 | 85 | 0 | 0 | 610 |
| LADAC | 97 | 97 | 0 | 5,756 | 13 | 0 | 0 | 2,437 |
| Thermodynamics | 40 | 25 | 15 | 867 | 0 | 0 | 0 | 0 |
| WEC-Sim | 19 | 19 | 0 | 3,353 | 7 | 0 | 0 | 613 |
| etherlab-examples | 0 (.mdl only) | | | | | | | |
| **total** | **178** | **163** | **15** | **36,098** | **105** | **0** | **0** | **3,660** |

Divergence classes: *trailing break* (105), a block name ending in a line break, which
fathom reads as a trailing space (a line break is one space) and miss-hit drops; *fathom
only* (3,660), blocks miss-hit does not report, the Stateflow chart-internal blocks it folds
into the chart's block; *refused* (15, all Thermodynamics: 13 AssertionError, 2 miss-hit
Error), models its parser stops on. No block miss-hit reports is missing from fathom's
reading, and none differs in path beyond the trailing break. No tool checked signal lines.
Round 2's queue text read "148 of 163 parsed"; this run counts 178 `.slx` files, 163
parsed, 15 refused, and the same 36,203 blocks and 3,660 fathom-only blocks.

Commands:

    CALIBRATION_SHARDS=<dir holding only simulink.yaml> task calibrate-clone -- simulink
    python3 -m venv workspace/tools/simulink-xcheck/venv
    workspace/tools/simulink-xcheck/venv/bin/pip install miss_hit_core==0.9.44
    # blocks.jsonl: one {File, Err, Blocks: [{SID, Path}]} per model file under
    # workspace/calibration/simulink, from extractor.ReadSimulinkModel (a scratch Go program)
    workspace/tools/simulink-xcheck/venv/bin/python -I calibration/oracles/simulink-walk/xcheck_misshit.py \
      -subjects workspace/calibration/simulink -fathom <blocks.jsonl> -out <summary.json>

## mathworks/data-explorer-core v1.46.0 (`parseModel`, BSD-3): unreproduced

Round 2 reported, over all 194 models: SID and path agree for the 8,544 blocks it reports
(7,651 byte for byte, 893 differing only by its display labels), and 9 of 9 model
references. The package could not be fetched in round 3's VM (no npm registry entry
resolved), so these figures are unreproduced here and carry round 2's word only.
