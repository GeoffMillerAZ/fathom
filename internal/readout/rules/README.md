# Built-in readout rules

Each `*.toml` file here is a built-in rule file in the user format (`schema = 1`, `[[rule]]`, design
`docs/design/readouts.md` §7), embedded into the binary and loaded by `readout.LoadRules` before
`~/.fathom/readouts.toml` and `~/.fathom/readouts.d/`. Its fixtures run at load and in the gate
(`internal/readout/builtins_test.go`); a rule whose fixture fails is not served. Every built-in ships
in observe. A fixture's `input_file` is a `.txt` file beside its rule.

| File | Rules |
|---|---|
| `ls-long.toml` | `ls-long` |
| `grep-group.toml` | `grep-group`, `git-grep-group` |
| `git.toml` | `git-log`, `git-show`, `git-diff` |
| `go-test.toml` | `go-test` |
| `read-outline.toml` | `read-outline` |

The base stages (ANSI, progress overwrites, repeat collapse) are no rule: `Render` runs them inside
every matched rule. This file is not a rule: only `*.toml` files load.
