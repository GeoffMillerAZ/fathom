HLP-4 verdict: PASS

# HLP-4: the agent cookbook, what runs and what checks it

Measured at `2a62c36e3` (`cmd/fathom-lang` and the cookbook files at that commit). Toolchain `go1.27.4`, `darwin/arm64`.

PASS means `TestAgentCookbook` passes: it builds `fathom-lang`, makes the small tree of the cookbook (Go, Python and TypeScript files, a boundary rule file, a git repository with one commit and an uncommitted change), extracts the exact text of every recipe from `docs/use-cases/library-agent-cookbook.md` and every definition from `examples/lang/agenttools/tools.json`, runs it, and checks the output. Where the binary prints a schema (`fathom-lang schema extract|rows|at|outline`) the test compiles that output with the CUE module pinned in `go.mod` and validates against it. `deps` and `boundaries` print no schema, so the test decodes their JSON and asserts the fields and verdicts the recipe teaches. There is no bar: the verdict says every recipe and tool runs and validates, and nothing about whether an agent does better with them.

```sh
go test -race -count=1 -run TestAgentCookbook -v ./cmd/fathom-lang
```

The tools the recipes use are `sh`, `git`, `xargs`, `awk` and `sed`. The test fails if any is missing, so no recipe depends on a tool it does not run. SQL runs through `modernc.org/sqlite`, already in `go.mod`. The helper surfaces (`outline`, `at`, `extract -rows`, `schema`) are provisional.

## Recipes

Each row is a block of the guide. `Test` is the subtest of `TestAgentCookbook` that runs its exact text.

| Job | Recipe block | Test | Assertions |
|---|---|---|---|
| 1 Shape of a file | `outline` | `recipe/outline` | exit 0, first line `# PATH dialect=`, last line `# symbols=` |
| 1 | `outline-json` | `recipe/outline-json` | exit 0, output validates against `schema outline` |
| 2 Which function | `at-line` | `recipe/at-line` | exit 0, validates against `schema at`, `app.py:6` is `found` in `Greeter.hello`, `pkg/a.go:9` is `found` in `A` |
| 2 | `at-range` | `recipe/at-range` | exit 0, validates against `schema at`, a range over `A` and `B` is `none` with no enclosing symbol, two touched, complete |
| 3 Symbols a diff touches | `diff-symbols` | `recipe/diff-symbols` | exit 0, validates against `schema at`, `Greeter.hello`, `A` and `B` each `found`, stderr names the deletion-only hunk of `util.py` as not queried |
| 4 Dependencies | `deps` | `recipe/deps` | exit 0, three files, `app.py` imports `util.py` with resolution `file`, and the import of `os` is an unknown |
| 5 Boundary | `boundaries` | `recipe/boundaries` | exit 1, `app-not-util` violated, `web-isolated` clean |
| 6 Ad hoc SQL | `rows` | `recipe/rows` | exit 0, every line validates against `schema rows` |
| 6 | `sql-setup`, `sql-symbols-per-kind`, `sql-longest-symbols`, `sql-refs-without-target` | `recipe/sql` | the rows load into in-memory SQLite, the views build, and each of the three queries returns exactly what the nested `extract` document of the same files gives, computed in Go (at least 6 kind groups, and at least one file with references that have no in-file target) |

`cookbook-names-match-the-test` fails when the guide holds a block the test does not run or the test expects one the guide lacks.

## Tool definitions

| Job | Tool | Test | Assertions |
|---|---|---|---|
| 1 | `outline_file` | `tool/outline_file` | input schema compiles, accepts its `example_input` and rejects `{}`, the expanded command exits 0, the text frame holds |
| 2 | `enclosing_symbol` | `tool/enclosing_symbol` | same input checks, exit 0, validates against `schema at`, `found` in `Greeter.hello` |
| 3 | `symbols_touched_by_diff` | `tool/symbols_touched_by_diff` | same input checks, exit 0, validates against `schema at`, the three symbols and the stderr note |
| 4 | `file_dependencies` | `tool/file_dependencies` | same input checks, exit 0, the `deps` assertions above |
| 5 | `boundary_check` | `tool/boundary_check` | same input checks, exit 1 (declared in `exit_codes`), the verdicts above |
| 6 | `tree_rows` | `tool/tree_rows` | same input checks, exit 0, every line validates against `schema rows` |

`diff-unqueried-changes` runs the diff script on twenty trees: no change (exit 0, stderr says so), only a deleted line, only a deleted file, only an untracked file (each exit 3 with empty stdout and a note per change), a pure rename (exit 0, the old path noted as a deleted file and the new path queried whole), an edit beside a deleted file (exit 0, the edit queried, the note on stderr), an edited file named with a non-ASCII character and one starting with a dash (both queried and `found`), and three git failures (a base that is not a commit, a directory that is not a work tree, a repository with no commit yet), each exit 2 with empty stdout and no `no changes` on stderr; run from a subdirectory, an untracked file at the root (exit 3, noted) and an edit at the root (exit 0, queried); a failing `awk` or `sed` first on the path (exit 2, empty stdout, the failed tool named on stderr); a mode-only change and a deleted empty file (each exit 3, noted as a change with no changed lines); and a `textconv` diff driver that prepends lines (the edit still `found`, because the script passes `--no-textconv`); a mode-only change before an edited file (the edit queried, the mode change noted); a submodule with a dirty worktree under `diff.submodule=log` (queried at its path, exit 1, never "no changes"); and a failing `sed` beside an untracked file (exit 2). `samples` compares the displayed outline output byte for byte with the recipe's output and the abridged `at` and `deps` samples piece by piece, in order. `tools-put-dashdash-before-values` requires every placeholder in a template to follow a literal `--` or `--rules`.

Also asserted for the tools: the exit code each run returns is declared in the tool's `exit_codes` (`tool/*`), `tools/file` (schema_version 1 and provisional), `diff-script-is-one-text` (the script in `tools.json` equals the guide's `diff-symbols` block), `tools-stop-flag-parsing` (`fathom-lang outline -- -not-a-flag.py` reads a path, not a flag, which is why the templates put `--` before a value) and `guide-is-indexed-and-names-the-tools` (the use-case index links the guide, and the guide points at `tools.json`). A command element that is exactly `{name}` is the only substitution, so no value reaches a shell except the diff base, which the script refuses when it starts with `-`.

## What this does not show

- It does not show an agent does better with these recipes. Whether an outline helps an agent belongs to the bench program, as the HLP-2 reading says.
- The tree is small and clean. Recipe 1's crossover is the size table of [the HLP-2 reading](hlp-2-file-outline.md), not something this test measures.
- Recipe 3's script does not query deletion-only hunks, deleted files, binary changes, changes with no changed lines or untracked files (each is named on stderr, and when nothing else was queried the exit is 3, not 0) and splits paths at spaces. The guide says so.
- The SQL load step is left to the reader's loader ("one line, one row"). The test loads the lines with `INSERT`, so no command that the test cannot run appears in the guide.
- `deps` and `boundaries` have no published schema, so their checks are field assertions, not schema validation.
