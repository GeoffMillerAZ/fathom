# jupyter -- provenance and divergences

LB-Q-jupyter, design `docs/design/language-breadth.md` sections 13 and 14. A native
dialect (`internal/extractor/notebook.go`): `encoding/json` over the notebook and the
python dialect over its code cells. No grammar of its own, no incumbent.

## Engine

The queue named the engine `tags+doc`. The tags+doc engine splits a dialect's files
between two engines by file, and a notebook is one file whose Python lives inside JSON
strings, so neither engine reads it alone. The dialect is `native`, and the queue entry
says so (`retargeted`, wave lang2): a notebook extractor reads the JSON, builds each
code cell's Python, runs the pipeline's python extractor on it and maps the lines back.

## What it models

- **Format 4 notebooks**: `cells[]` of `cell_type: code`, `source` as a string or a list of
  strings. Markdown and raw cells, `outputs` and `metadata` are never read, so an output
  that prints a definition (or a megabyte of base64) costs a token scan and yields nothing.
- **Language**: `metadata.language_info.name`, else `metadata.kernelspec.language`. A
  notebook in another language reads as empty. A notebook that names neither reads as Python.
- **IPython syntax**: a line whose first non-blank character is `%` or `!` becomes `pass` at
  its indent; a cell whose first line is a cell magic other than `time`, `timeit`, `capture`,
  `prun`, `debug`, `python`, `python3` or `pypy` is skipped (`%%bash`, `%%html`,
  `%%writefile`); one of those has its first line blanked. Line counts never change.
- **Definitions and calls**: exactly the python dialect's, per cell (classes, functions,
  methods, module and class variables, type aliases, fields, a `<module>` symbol), with the
  python dialect's source kinds under the notebook's own names (`JupyterClass`,
  `JupyterFunc`, `JupyterVar`, `JupyterTypeAlias`, `JupyterField`, `JupyterModule`: the
  registry holds a source kind to one dialect). Each cell is a separate parse, so a cell
  that does not parse costs only its own symbols. The calls no definition encloses are the
  one `<module>` symbol's, which runs from the first code line to the last.
- **Line mapping**: a source line maps to the line of the notebook file that holds the JSON
  string it begins in, from the decoder's byte offset. A list source gives one line per
  element, a single string gives every line of the cell the line of that string, and an
  element holding several source lines gives all of them its own.

## What it does not model, and why

- **Format 3** (`worksheets`, `input`): read as empty. nbformat upgrades it, the dialect
  does not; none of the calibration notebooks is one.
- **Cross-cell state**: cells share a namespace in Jupyter, and a call from one cell to a
  function another defines is a plain call name here, as it is across files in Python.
- **Imports and file edges** (L3): not attempted. The target is L2.
- **Execution order**: cells are read in document order, not by `execution_count`.
- **Non-Python kernels' syntax** inside a Python notebook (`%%sql` bodies, `%%bash`): skipped.
- **Line magics inside strings or continuations**: a `%` or `!` that starts a line inside a
  triple-quoted string or a multi-line expression is replaced like any other, which only
  changes the text of a string or turns a continuation into a `pass`.

## Oracle

`calibration/oracles/jupyter`: nbformat 5.11.1 (the reference reader, in a venv under the
tool cache) reads each notebook and its language, the code cells are exported as one Python
module in the same convention, universal-ctags 6.2.1 lists its definitions (tier B) and
Python's `ast` its calls (tier C), and each line is mapped back to the notebook file by a
scan of the raw text for each source string as JSON writes it. See
`calibration/oracles/README.md`.
