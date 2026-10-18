# Built-in readout rules

Each `*.toml` file here is a built-in rule in the user format (`schema = 1`, `[[rule]]`, design
`docs/design/readouts.md` §7), embedded into the binary and loaded by `readout.LoadRules` before
`~/.fathom/readouts.toml` and `~/.fathom/readouts.d/`. Its fixtures run at load and in the gate; a rule
whose fixture fails is not served. The built-ins themselves are RO-8 to RO-13 (`docs/plans/readouts.md`).
This file is not a rule: only `*.toml` files load.
