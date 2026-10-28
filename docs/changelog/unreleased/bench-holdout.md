### Added

- `scenariobench parity -corpus <path> [-baseline <arm>] [-task <id>] [-tree <dir>] [-json]` renders every arm into a scratch copy of one local tree and reads PASS, FAIL (an undeclared difference in prompt, tools, settings, notes file, turn cap, model or tree) or INCOMPLETE per arm, exiting 0, 1 or 2. Scenario corpus arms accept an optional `declares` list of intended differences; an arm without it marshals unchanged.
- `docs/bench/heldout-reads.md` records dev sets, held-out splits and held-out reads, and declares the 13-task home corpus a dev set by the sha256 of its sorted task ids.
