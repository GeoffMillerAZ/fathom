# Held-out reads

This file records which task sets are dev (tuning) sets, which are held-out splits, and every read of a held-out split
(`docs/design/bench-prep.md` §13, BX-29). `internal/benchprep` parses it (`ParseHeldOutRecord`): a line that starts
with one of the four prefixes below is a record line and must parse, or the whole record is refused, naming the line;
every other line is prose.

A split sha256 is the sha256 hex of the split's task ids, sorted ascending, each followed by a newline (`SplitSHA256`).

Who writes each line, and its format (placeholders in angle brackets):

- `Dev split sha256: <hex> <repo-relative path> (<n> tasks)`: a task set declared dev. The parser loads the file and
  refuses the record when the file's ids no longer hash to the line. The first line came with this file; the controller
  writes any later one.
- `Held-out split sha256: <hex> <split: held-out|post-cutoff> (<n> tasks)`: a committed held-out split. Only the
  controller writes it, at BX-11.
- `Read: <split sha256> split <name> prereg <id> <prereg sha256> run <run id> commit <sha> at <RFC3339 UTC>`: one read
  of a held-out split, appended by `RecordHeldOutRead` for `scenariobench run` (BX-29). It carries no host or user field.
  A second read of a split sha is refused until the split is demoted.
- `Demoted: <split sha256> <reason>`: the split is a dev set from here on; a held-out read of it is refused. Only the
  controller writes it.

Nobody edits this file by hand otherwise. Reads are appended at the end, so `## Read log` stays the last section.

## Dev sets

Dev split sha256: c0604fba3e8d4f9ee767bed76397e9dd083396a73b2c3526a7be26e4a7362b31 testdata/contextbench/tasks-home.json (13 tasks)

The home corpus (H1-T01 to H1-T13) is a dev set: it was authored on fathom's own repository, the committed home-bench
ledger rows read it (newest `20260914T035806Z`), and it is the tuning set from here on.

## Held-out splits

## Read log
