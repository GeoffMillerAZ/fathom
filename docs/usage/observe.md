### `observe` — file a friction observation

```sh
fathom observe --symptom "..." [--expected "..."] [--actual "..."] [--repro "<verbatim command>"] [--tool <name>] [--root .]
```

Appends a structured entry to `<root>/.fathom/observations.md`. `--symptom`
is required.
