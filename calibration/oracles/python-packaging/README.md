# python-packaging oracle

`pyproject_oracle.py` is the oracle for the `python-packaging` doc-engine dialect (pyproject.toml). It shares no
code with fathom's extractor and is never imported by fathom. Each table of a pyproject.toml is handed to the tool
that owns it, offline, in a venv that holds those tools (no build isolation, no index):

| Table | Evaluated by | Emits |
|---|---|---|
| `[project]` | the file's own build backend (`build-system.build-backend`, `backend-path`; setuptools' legacy backend when none is named, PEP 517) through PyPA's `pyproject_hooks`, `prepare_metadata_for_build_wheel` | a `package` (METADATA `Name`), a `route` per entry of `entry_points.txt` (console_scripts, gui_scripts, every other group) |
| `[dependency-groups]` | PyPA's `dependency-groups`, the PEP 735 reference implementation that pip's `--group` runs: `DependencyGroupResolver.lookup` and `resolve` | a `target` per group it accepts; `-refs-out`: a reference per `include-group` its lookup returns |
| `[tool.*]` | nothing: each tool reads its own table | a `section` per table whose value is a map, named by tomllib |

No tool prints a position, so every line comes from the program's own line scanner (`scan_lines`: table headers,
keys, dotted keys and inline tables; never a TOML library), the compose, envoy, cargo and custodian precedent.

## Tiers (design §1.3)

The ladder: the format's reference parser or the tool's own evaluation is A; a widely used third-party parser tool is
B; an independent library or walk is C.

- `-refs-out` is **tier A**. A dependency group and its `include-group` entries are PEP 735's, and
  `dependency-groups` is the specification's reference implementation (PyPA), the code pip runs for
  `pip install --group`. The references are what its `lookup` returns; a group it refuses (an unknown include, a
  cycle) is counted and left out.
- The definitions file is **tier C**, and the reason is the `[tool.*]` tables. Each package and route is the build
  backend's own evaluation and each group the reference implementation's, which is tier-A evidence, but no tool
  evaluates `[tool.ruff]`, `[tool.mypy]` and the rest together: tomllib names them, and tomllib is TOML's parser, not
  pyproject's (it does not know what a section is), which the ladder puts at C. One oracle file carries one tier,
  and the harness scores one L1 oracle per subject, so the file is C and L1 is capped at good
  (LB-Q-python-packaging-rest). The earlier reading's tier-C oracle (a tomllib walk converted in scratch, never
  committed) is replaced, not relabelled.

A backend that fails rejects its file: the file contributes nothing, is named on stderr and is left out of
`-files-out` (the reading's `lang-oracle -only` scope when one is rejected; none on the corpus subjects). A METADATA
`Name` that differs from `project.name` is counted (0 on the subjects).

## Run

```sh
python3 -m venv <venv>
<venv>/bin/pip download -d <wheels> "flit-core>=3.11,<4" pdm-backend setuptools dependency-groups pyproject_hooks
<venv>/bin/pip install --no-index --find-links <wheels> flit-core pdm-backend setuptools dependency-groups pyproject_hooks
<venv>/bin/python calibration/oracles/python-packaging/pyproject_oracle.py -subject <checkout> \
  -out <scratch>/py-<subject>.json -refs-out <scratch>/py-<subject>-refs.json -files-out <file> -work <scratch dir>
TMPDIR=<scratch> <venv>/bin/python -m unittest discover -s calibration/oracles/python-packaging
```

Pass both JSON files to `lang-oracle -tool-oracle`. fastapi's version is read by pdm-backend from
`fastapi/__init__.py`, outside the corpus subpath: `git -C <checkout> sparse-checkout add fastapi` first.

Versions measured (2026-10-02): python 3.14.7; pyproject_hooks 1.3.3
(5fc53fdac9f7bd63fbcdc868fb5f90b4784d78a53a3d3388cd738b807441a20b), dependency-groups 1.3.2
(8c337f7b92445823b0d0ea507f611390c58dcd2112be00aebcd738a2ca7a1c64), flit_core 3.12.0
(e7a0304069ea895172e3c7bb703292e992c5d1555dd1233ab7b5621b5b69e62c), pdm-backend 2.5.0
(5b8746b567a16a1fd506ce66ee28e89d46374526f52238aa1b96cc2c08245c63), setuptools 84.0.0
(51a52592b3b99e102b609654876bd65f19f999935166d1352678931132b0c670), packaging 26.3
(d7193f7c8e4e93f444fde0262bf90af30e16fa0ad0ad44cb553c87339b23cd1c); wheel sha256 in parentheses.

## Setup.cfg and candidate audit (T-R2, X-R1)

The landed `ini` decoder is Conan's line-oriented reader (`docdecode_ini.go`,
LB-F125), not a general INI key/value parser. It retains each physical line as
an independent `{text,key,value}` map in a section sequence. It does not fold
continuation values or retain option/entry-point group ownership. `decode_files`
can select it by basename, but cannot create those missing semantics. Merely
adding setup.cfg to the basename registry would not implement this format.
The small authored file in `testdata/setupcfg-continuation.cfg` demonstrates
both a continued option and an inline entry-point declaration; it is a contract
example, never calibration truth or a denominator fixture.

| Candidate | Version / identity | Concrete result or limitation |
|---|---|---|
| Native PEP 517 backend metadata | flit_core 3.12.0, pdm-backend 2.5.0, setuptools 84.0.0 through pyproject_hooks 1.3.3 | Already run in the committed recipe; attests package/entry-point entities, not every unrelated tool namespace. |
| PyPA dependency-groups | 1.3.2 | Already run; native include-group references are tier A. Does not evaluate tool namespaces or setup.cfg entry-point continuations. |
| validate-pyproject | 0.27; wheel SHA256 6ad8a9a80480a5a833753f4e1c1b22cb01b8a5ad0015b1e9dbd4a5f8e898aba0 | Source inspected, not installed: schema declares `tool` merely as an object; `Validator.__call__` validates and returns the supplied mapping. Per-tool plugins add schemas for selected tools, not a native evaluator of arbitrary namespaces. |
| PyPA pyproject-metadata | 0.12.1; wheel SHA256 f7162d580a96386a8eb096da06215f981f547d1490f03055ef99e323bc2da427 | Source inspected, not installed: `StandardMetadata.from_pyproject` reads `[project]` metadata, not unrelated tool-table entities or setup.cfg. |
| pyproject-parser | 0.14.0; wheel SHA256 10adf69e48c2dcfb993f94b4981acfd7d0a201846bce17a67c9736f42505e790 | Source inspected, not installed: `PyProject.tool` is a generic dictionary. Walking it remains independent library evidence C, not native evaluation of each tool's configuration. |
| Python ConfigParser | Python 3.9.6 standard library | Run on the authored example; folds the continued option as one value. It can locate syntax for a future native setuptools oracle but does not fix product decoding. |
| Product Conan INI reader | fathom 5b84aa5b4, LB-F125 | Source inspected: one item per physical line, indentation trimmed. Missing continuation/option semantics require a shared decoder capability and affected-item regrades, held by T-R2. |

Reproduce the syntax requirement without executing package code:

```sh
python3 -c 'import configparser; p=configparser.ConfigParser(); p.read("calibration/oracles/python-packaging/testdata/setupcfg-continuation.cfg"); assert p["options.entry_points"]["console_scripts"].strip()=="demo = package.cli:main"; assert p["options.entry_points"]["app.plugins"]=="plugin = package.plugins:Plugin"'
```

The published candidate versions and wheel digests were checked on 2026-10-25;
source-interface findings above are not runtime failure claims. No claim is
made that a tool is unavailable: native backend metadata already works. The
mixed definitions tier and the setup.cfg decoder capability remain separate
ceilings. Requirements files still need line/requirement grammar support; the
Conan reader drops lines before a section, so it is not that decoder either.
