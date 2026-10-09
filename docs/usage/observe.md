### `observe` — file a friction observation

```sh
fathom observe --symptom "..." [--expected "..."] [--actual "..."] [--repro "<verbatim command>"] [--tool <name>] [--root .]
```

Writes one structured entry to its own file, `<root>/.fathom/observations/O<n>.md`,
and prints `recorded O<n> in <path>`. `--symptom` is required. The next id is
minted over the directory and the legacy `<root>/.fathom/observations.md` log
together, each file is created exclusively (a concurrent `observe` that took the
id moves to the next one), and the legacy log is read but never written (EW-32).
Commit `.fathom/observations/` beside the log.
