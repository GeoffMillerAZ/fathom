### `readout` — readout rules, offline (ADR-071)

```
fathom readout replay [--days N] [--projects GLOB] [--rules FILE] [--json]
fathom readout explain [--rules FILE] [--mode observe|lean|rich] [--json] '<command>' < output
fathom readout test [file...]
```

None of the three runs a command or installs a rule. Without `--rules` a verb reads the built-in rules and the user's files (`~/.fathom/readouts.toml`, `~/.fathom/readouts.d/*.toml`); with `--rules FILE` it reads that one file alone. Design: [readouts](../design/readouts.md) §8 to §10.

`replay` reads the local Claude Code transcripts (`$CLAUDE_CONFIG_DIR/projects`, else `~/.claude/projects`; every `*.jsonl` under each project directory `--projects` matches, subagent transcripts included) for the last `--days` days (default 30). It pairs each Bash call with its result, counts a tool-use id once (a resumed session copies its history into a new file), reads the harness's persisted output file when the result names one, and renders every call through the engine in observe. It prints aggregates only, never command text, output text, a path, a session id or a project name:

- `bash`: calls, bytes (stdout and stderr), duplicates skipped, unpaired calls, failures, persisted outputs and how many were read;
- `whole_file_reads`: whole-file source reads (`cat`, `sed -n 1,$p`; never `head` or `tail`) over 200 lines, computed by shape whether or not a rule matches, as a share of Bash bytes;
- `rules`, per rule: hits, bytes in, served readouts, would-remove bytes, raw reasons, protected trips by stream (content, diagnostic), the trips a protected-line rule not scoped by stream would have read, and the offline dropped-line reference rate (a later call within five that names a path or identifier only a dropped line carried) as referenced and evaluated counts with the 95% Wilson lower bound that no dropped line was referenced; `eligible` when at least 200 readouts were evaluated and that bound is at least 0.90;
- `eligible`: those rules' would-remove bytes as a share of Bash bytes;
- `shapes`: per command head (the lexer's first command, with the subcommand for git, go and similar tools; the top 25 by bytes, the rest as `(other)`): calls, bytes, share, simple, piped and list counts and the compound share; `tccutil_reset`: its calls and compound share; `reread`: whole-file reads followed by a `sed -n` of the same file within five calls;
- `window`: the first and last counted call's timestamps, from the data.

`explain` renders one command string and its output (stdin) through the engine and prints the lexing verdict, the rule matched, its stages, lossiness and recovery, raw or readout and why, bytes and lines in and out, dropped lines by class, protected trips, records by kind, the input lines not in the view, and the receipt. Exit 0, 1 for a usage error, 2 when the rules cannot load.

`test` loads each file (none: the built-ins and the user's files) and holds every rule to the contract: its fixtures, each run twice for determinism, a protected-line fixture for each stream a lossy rule reaches, and every lossless stage's inverse. It prints each served rule's lossiness, recovery and fixture count, and each refused rule's reason. Exit 0 when every rule is served, 1 when a rule is refused, 2 when a file cannot load.
