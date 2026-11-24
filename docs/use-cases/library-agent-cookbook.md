# Agent Cookbook for the Language Library

Six jobs a coding agent does over source files, each as one recipe on `fathom-lang`: the exact command, the shape of the output and what unknown means for that job. No store, daemon, index, network or server.

The surfaces these recipes use (`outline`, `at`, `extract -rows`, `schema`) are **provisional**: their shapes may change in a minor release until the LP-8 readings pass and one release has shipped. `deps` and `boundaries` are older and carry their own contracts ([dependency-edges](../contracts/dependency-edges.md)). The reference for every command is [lang-library.md](lang-library.md).

Every command block below marked as a recipe is run by `TestAgentCookbook` in `cmd/fathom-lang`, which builds the binary, runs the exact text against a small tree and validates each JSON output against the binary's own `fathom-lang schema` output where a schema exists (`deps` and `boundaries` have none, so the test checks their fields). The tool definitions in [examples/lang/agenttools/tools.json](../../examples/lang/agenttools/tools.json) are run the same way. A recipe that stops running, or whose output stops validating, fails the gate. LBK-11's cookbook links this chapter when it lands.

## The tree the recipes run on

```
app.py          class Greeter (method hello) and main; imports os and util
util.py         def helper
pkg/a.go        func A calls B; func B
web/index.ts    imports ./lib; function run
web/lib.ts      function add
rules.json      two boundary rules (recipe 5)
```

It is a git repository with one commit and an uncommitted change (recipe 3). Replace these paths with yours.

---

## 1. Shape of a file before reading it

Read a small file whole. Use the outline for a large one. The crossover is a measurement of size, not of agent benefit: the table "By file size" in [the HLP-2 reading](../bench/readings/library-roadmap/hlp-2-file-outline.md) gives the median of outline bytes over file bytes by line count and dialect.

| Lines | Median outline over file, across Go, JavaScript, Python and TypeScript |
|---|---|
| 1 to 49 | 27% (Go) to 84% (Python) |
| 50 to 199 | 15% (JavaScript) to 27% (Python) |
| 200 to 999 | 9% (JavaScript) to 18% (TypeScript) |
| 1,000 and over | 7% (JavaScript) to 12% (Python) |

Below about 50 lines the outline is a large share of the file, so read the file. From a few hundred lines up the outline is a small share. The reading sets no bar by size, and it does not measure whether an agent does better with an outline. The size bar it did set (median at or under 20% per dialect) was missed for Python (58.31%) and TypeScript (24.17%).

```sh recipe=outline
fathom-lang outline app.py
```

```text sample=outline
# app.py dialect=python level=L4
1-9 module <module>
  4-6 class Greeter
    5-6 function Greeter.hello  def hello(self, name):
  8-9 function main  def main():
# symbols=4 references=4 unresolved=0 facts=0 notes=0
```

One header line, one line per symbol (`START-END kind name  signature`, nested by line span) and one footer with counts. The JSON form validates against `fathom-lang schema outline`, and `-max-nodes N` caps the nodes:

```sh recipe=outline-json
fathom-lang outline -format json -max-nodes 50 app.py web/index.ts
```

**Unknown here.** `END` is `?` when the extractor computed no end line, so that symbol's span and its children are not known. `incomplete` or `skipped=REASON` in the header says the read was partial, and symbols may be missing. A footer ending in `omitted=N` says `-max-nodes` dropped N nodes. The outline holds only strings the read carries, never source text, so it cannot show a body: read the lines it names.

## 2. Which function a stack-trace line or a diff hunk belongs to

`at` answers `PATH:LINE` and `PATH:LINE-END`, in argument order, reading each file once.

```sh recipe=at-line
fathom-lang at app.py:6 pkg/a.go:9
```

```json sample-elided=at-line
{"schema_version":1,"queries":[
  {"path":"app.py","start":6,"end":6,"status":"found",
   "enclosing":{"key":"app.Greeter.hello","name":"Greeter.hello",...,"line":5,"end_line":6,...},
   "touched":[...],"complete":true,"error":""},
  ...]}
```

(Abridged, `...` marks what the output has and the sample leaves out, and the output is indented: each symbol carries `key`, `name`, `package`, `kind`, `canonical`, `line`, `end_line`, `signature` and `tier`. The output validates against `fathom-lang schema at`.)

For a range, such as one diff hunk, `enclosing` is the innermost symbol that holds the **whole** range, and `touched` is every symbol the range reaches:

```sh recipe=at-range
fathom-lang at pkg/a.go:7-14
```

Lines 7 to 14 reach both `A` and `B` and no single symbol holds them, so `status` is `none`, `enclosing` is null and `touched` lists `A` and `B`. A range inside one function gives `found`.

**Unknown here.** `status` is `unknown` when the read cannot decide: the file read incomplete or skipped, or a symbol with no end line may hold the line. Then `enclosing` is a hint or null, and `complete` false says `touched` may be missing symbols. `none` means the read was complete and no symbol holds the range, which is a fact about the file, not a failure to look. Act on `found` and `none`. Treat `unknown` as "read the file". Exit 2 is a bad query (a line below 1, an end below the start). Exit 1 is a file that could not be read, with its `error` set.

On Python, JavaScript and TypeScript the reader emits a symbol that spans the whole file, so every line is `found`, often as that module symbol. Check `enclosing.canonical` before you treat `found` as inside a function. On Go, `none` is a line outside every declaration (the package clause, imports, top-level comments). Both are measured in [the HLP-1 reading](../bench/readings/library-roadmap/hlp-1-span-lookups.md).

## 3. Which symbols a diff touches

`git diff -U0` prints one hunk header per change, `@@ -OLD +START,COUNT @@`. The script turns each into a `PATH:START-END` query on the working tree and runs one `at`. Pass a base revision, default `HEAD`.

```sh recipe=diff-symbols
base=${1:-HEAD}
case $base in -*) echo "base must be a revision, not an option" >&2; exit 2 ;; esac
git rev-parse --verify --quiet "$base^{commit}" >/dev/null || { echo "base is not a commit in a git work tree: $base" >&2; exit 2; }
top=$(git rev-parse --show-toplevel) && [ -n "$top" ] && cd "$top" || { echo "not in a git work tree" >&2; exit 2; }
diff=$(git -c core.quotePath=false diff -U0 --inter-hunk-context=0 --submodule=short --no-renames --no-color --no-ext-diff --no-textconv --src-prefix=a/ --dst-prefix=b/ "$base" --) || { echo "git diff failed" >&2; exit 2; }
others=$(git -c core.quotePath=false ls-files --others --exclude-standard) || { echo "git ls-files failed" >&2; exit 2; }
parsed=$(printf '%s\n' "$diff" | awk '
/^diff --git / { if (blk != "" && !said) printf "n change with no changed lines: %s: not queried\n", blk; blk = $0; said = 0; inhdr = 1; file = ""; old = ""; next }
inhdr && /^--- / { old = ($0 == "--- /dev/null") ? "" : substr($0, 7); next }
inhdr && /^\+\+\+ / {
  if ($0 == "+++ /dev/null") { file = ""; said = 1; printf "n deleted file %s: not queried\n", old } else file = substr($0, 7)
  next
}
inhdr && /^Binary files / { said = 1; printf "n binary change: %s: not queried\n", $0; next }
/^@@ / {
  inhdr = 0
  if (file == "") next
  split($3, r, ",")
  start = substr(r[1], 2) + 0
  n = (r[2] == "") ? 1 : r[2] + 0
  said = 1
  if (n == 0) { printf "n deletion-only hunk in %s after line %d: not queried\n", file, start; next }
  printf "q %s:%d-%d\n", file, start, start + n - 1
}
END { if (blk != "" && !said) printf "n change with no changed lines: %s: not queried\n", blk }') || { echo "awk failed" >&2; exit 2; }
parsed=$(printf '%s\n' "$parsed"; [ -z "$others" ] || printf '%s\n' "$others" | sed 's/^/n untracked file /; s/$/: not queried/') || { echo "sed failed" >&2; exit 2; }
notes=$(printf '%s\n' "$parsed" | sed -n 's/^n //p') || { echo "sed failed" >&2; exit 2; }
queries=$(printf '%s\n' "$parsed" | sed -n 's/^q //p') || { echo "sed failed" >&2; exit 2; }
[ -z "$notes" ] || printf '%s\n' "$notes" >&2
if [ -z "$queries" ]; then
  [ -z "$notes" ] || { echo "changes exist but none could be queried" >&2; exit 3; }
  echo "no changes against $base" >&2; exit 0
fi
fathom-lang at -- $queries
```

On the tree above it reports `Greeter.hello` in `app.py`, and `A` and `B` in `pkg/a.go`, each `found`, and it prints a note on stderr for the line removed from `util.py`. The output is the `at` document of recipe 2.

**Unknown here.** Everything in recipe 2 applies per hunk: a hunk whose `status` is `unknown` or whose `complete` is false may belong to a symbol the read did not place. Some changes cannot be queried at all, and the script names each one on stderr as `not queried`: a deletion-only hunk (it has no new lines, so to place it run `at` on the base revision's file), a deleted file, a binary change, a change with no changed lines (a mode change, or an empty file added or deleted) and an untracked file (`git diff` does not list it). It pins the diff shape with explicit flags (`--no-textconv`, `--submodule=short`, `--inter-hunk-context=0`), so a diff driver or a diff setting cannot shift, merge or hide the queried lines. A changed submodule is queried at its path and reads as a file that could not be read (exit 1), not as no change.

Exit codes say what empty output means:

| Exit | Meaning |
|---|---|
| 0 | The changes were queried, with stdout the `at` document and the notes above on stderr for what was skipped. Or there were no changes at all: stdout is empty and stderr says `no changes`. |
| 2 | The base is an option (starts with a dash), is not a commit, or a tool failed: git outside a work tree or with no commit yet, or awk or sed. Stdout is empty and stderr says which. Never read this as no changes. |
| 3 | Changes exist but none could be queried, such as a diff that only deletes lines or only changes a mode. Stdout is empty and stderr names each change. Which symbols were touched is **unknown**, not none. |
| 126 | The diff was too large for one `at` call (the system's argument-length limit). Nothing was queried. Split the diff by path. |

Other limits: a very large diff can exceed the system's argument-length limit, and then `at` does not run and the exit is 126 (split the diff by path); paths with spaces or newlines break the word splitting, and a renamed file is read as a deleted file at its old path (noted on stderr, not queried) plus a new file at its new path, queried whole, because the diff runs with `--no-renames`. Paths print unquoted and a path that starts with `-` is passed after `--`, so both are queried as paths.

## 4. What a file depends on, and what is unknown

```sh recipe=deps
fathom-lang deps app.py web/index.ts pkg/a.go
```

```json sample-elided=deps
{"schema_version":1,"files":[
  {"path":"app.py","dialect":"python","measured":true,"trusted":false,"complete":false,
   "edges":[{"from":"app.py","kind":"import","raw":"os","to":"","resolution":"unknown",...,"line":1,...},
            {"from":"app.py","kind":"import","raw":"util","to":"util.py","resolution":"file",...,"line":2,...}],
   "unknowns":[{"from":"app.py","code":"missing-config","raw":"os","line":1,...}, ...]},
  ...]}
```

Paths are repository-relative with forward slashes, and each is read once. An edge has a `resolution` of `file`, `directory`, `external` or `unknown`, and `to` names the target when there is one.

**Unknown here.** A file with any unknown is `complete: false`, and an unmeasured reader (Go today) is always incomplete: the edge list is then a floor, not the whole set. An import with `resolution` `unknown` or a `dynamic-import` unknown is a dependency the reader saw and could not place, not an absent one. `decidable_by` names what would settle it (a config file, a lockfile, a declaration, the toolchain). Read `complete` before skipping work on the strength of an edge list. The full rules are in [dependency-edges](../contracts/dependency-edges.md).

## 5. Does a change break a declared boundary

A rule file declares which files may not import what. For example `rules.json`:

```json
{"schema_version":1,"rules":[
  {"id":"app-not-util","from":["app.py"],"deny":["util.py"],"reason":"app reaches util through its package"},
  {"id":"web-isolated","from":["web/**/*.ts"],"deny":["**/*.py"],"reason":"web stays out of the Python tree"}]}
```

```sh recipe=boundaries
fathom-lang boundaries --rules rules.json app.py web/index.ts web/lib.ts
```

Stdout is one JSON document with a verdict per rule (`clean`, `violated` or `unverified`), the violating edges with their line, and the files whose evidence was incomplete. Findings go to stderr as text. Pass the changed files (`git diff --name-only`) to check a change, or no path to check every file a rule governs. Exit 1 means a violation or a file that could not be read. Here `app-not-util` is violated by `app.py:2 -> util.py` and `web-isolated` is clean, so the exit is 1.

**Unknown here.** `unverified` is not `clean`. It means a rule matched no file, or a matched file was incomplete or read by an unmeasured reader, so a violation could be hiding. Exit 0 covers both `clean` and `unverified`: add `--strict` to fail on `unverified` too, which is the setting for a gate. `--trust-unmeasured go` opts in to the Go reader's edges, and `trusted` records that you did.

## 6. Ad hoc questions over a tree

`extract -rows` prints JSON Lines, one object per line, that SQLite or DuckDB query with no glue code. `-walk` makes every file under the working directory visible as a sibling, which some dialects need to classify a file.

```sh recipe=rows
git ls-files -z -- '*.go' '*.py' '*.ts' | xargs -0 fathom-lang extract -walk -rows
```

Load each line into a table with one text column, `rows(doc TEXT)`, with any loader that puts one line in one row. For a very large tree `xargs` splits the list into several runs, and each prints its own `header` row, which the views below ignore. The views read the row kinds out of that column with SQLite's JSON functions:

```sql recipe=sql-setup
CREATE VIEW file AS SELECT json_extract(doc, '$.path') AS path, json_extract(doc, '$.dialect') AS dialect, json_extract(doc, '$.level') AS level, json_extract(doc, '$.skipped') AS skipped, json_extract(doc, '$.incomplete') AS incomplete, json_extract(doc, '$.error') AS error FROM rows WHERE json_extract(doc, '$.row') = 'file';
CREATE VIEW symbol AS SELECT json_extract(doc, '$.path') AS path, json_extract(doc, '$.i') AS i, json_extract(doc, '$.name') AS name, json_extract(doc, '$.canonical') AS canonical, json_extract(doc, '$.line') AS line, json_extract(doc, '$.end_line') AS end_line FROM rows WHERE json_extract(doc, '$.row') = 'symbol';
CREATE VIEW reference AS SELECT json_extract(doc, '$.path') AS path, json_extract(doc, '$.i') AS i, json_extract(doc, '$.from') AS src, json_extract(doc, '$.to') AS dst, json_extract(doc, '$.to_key') AS to_key, json_extract(doc, '$.line') AS line FROM rows WHERE json_extract(doc, '$.row') = 'reference';
```

Symbols per canonical kind per file:

```sql recipe=sql-symbols-per-kind
SELECT path, canonical, COUNT(*) FROM symbol GROUP BY path, canonical ORDER BY path, canonical;
```

The five longest symbols, among those with a known span:

```sql recipe=sql-longest-symbols
SELECT path, name, end_line - line + 1 AS lines FROM symbol WHERE line > 0 AND end_line >= line ORDER BY lines DESC, path, name, line LIMIT 5;
```

References whose target is not a symbol of the same file, per file:

```sql recipe=sql-refs-without-target
SELECT r.path, f.dialect, COUNT(*) FROM reference r JOIN file f ON f.path = r.path WHERE r.to_key = '' GROUP BY r.path, f.dialect ORDER BY r.path;
```

**Unknown here.** A symbol with `end_line` 0 has no known span: the longest-symbols query leaves it out, so it never ranks, whatever its real size. A reference with an empty `to_key` has a target that is another file, a library, an ambiguous name or a construct the read did not place. It is not a proof the target does not exist, and nothing here joins across files. A `file` row with `incomplete` 1, a `skipped` reason or an `error` is a file whose rows may be partial or absent: `SELECT path, skipped, incomplete, error FROM file WHERE incomplete = 1 OR skipped != '' OR error != ''` lists them, and a result that matters should be read with that list beside it.

---

## Tool definitions

`examples/lang/agenttools/tools.json` holds one definition per job in the common function-calling form: `name`, `description`, `input_schema` (JSON Schema), a `command` argv template, the `output` format and schema, the `exit_codes` and an `example_input`. A `{name}` element is replaced by the parameter (an array parameter becomes one argument per item), and no shell parses a value. The template puts `--` before user-supplied paths so a value that starts with `-` is a path, not a flag. The diff tool runs the recipe 3 script, rejecting a base that starts with `-`. The file is a definition list for your own runner: there is no server and no MCP surface here (ADR-116).

| Job | Tool | Recipe |
|---|---|---|
| 1 | `outline_file` | outline |
| 2 | `enclosing_symbol` | at-line, at-range |
| 3 | `symbols_touched_by_diff` | diff-symbols |
| 4 | `file_dependencies` | deps |
| 5 | `boundary_check` | boundaries |
| 6 | `tree_rows` | rows (the SQL is yours to run) |

## Maintained

This doc covers: `pkg/lang/view`, `cmd/fathom-lang` (its outline, at, deps, boundaries, schema and extract -rows commands as the six recipes use them), and `examples/lang/agenttools/tools.json`, with the HLP-4 plan row that `fathom verify` gates on its reading. Update it when these change.
