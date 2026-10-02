# conan — provenance and divergences

LB-Q-conan, design `docs/design/language-breadth.md` §4, §13. A `tags+doc` dialect (the Helm shape): one
manifest owns four files and splits them across the two engines by its doc spec's `detect`. No new grammar:
`conanfile.py` rides the pinned python grammar blob (`tags.scm` below), and `conanfile.txt`, `conan.lock` and
`conandata.yml` are documents. The `tags.scm` is written for this dialect against `indexbench lang-probe`
output over `testdata/` (only the node shapes are tree-sitter-python's, MIT); the fixtures are written for the
dialect, not copied from a project.

## What it models

- **The recipe** (`ConanRecipe`, canonical `package`, callable): a class that derives from `ConanFile`
  (`ConanFile` or `conan.ConanFile`) and writes `name = "<literal>"`, named for that string, at the line of the
  literal, ending with the class. Everything the recipe requires attaches to it, whichever method or class
  attribute it is in.
- **References** (`Calls` on the recipe): the class attributes `requires`, `tool_requires`, `build_requires`,
  `test_requires` and `python_requires` written as a string, a list, a tuple or a bare tuple of strings, and
  `self.requires("x/1.0")`, `self.tool_requires`, `self.test_requires` and `self.build_requires` anywhere in the
  class. The string is a Conan reference, `name/version[@user/channel][#revision]`; the manifest's
  `names.cut_at: "/"` (LB-F124) keeps the name, so `zlib/[>=1.2.11 <2]` is a use of `zlib`.
- **The consumer** (`conanfile.txt`, `ConanConsumer`, canonical `package`, named `conanfile.txt`): the file
  itself, with the names its `[requires]`, `[tool_requires]`, `[test_requires]` and `[build_requires]` sections
  write as references, and one `ConanGenerator`, `ConanOption` (`zlib/*:shared`) and `ConanLayout` entity per
  line of `[generators]`, `[options]` and `[layout]`. The file is decoded as INI (`decode_files`, the `ini` decode,
  LB-F125).
- **The lockfile** (`conan.lock`, `ConanLock`, named `conan.lock`): JSON read by the YAML decode; its
  `requires`, `build_requires` and `python_requires` references (`name/version#rrev%timestamp`) are cut to the
  name by the doc engine's `cut:` value suffix (LB-F126).
- **The recipe data** (`conandata.yml`): `sources.<version>` and `patches.<version>` as `ConanSource` and
  `ConanPatches` keys.

## What it deliberately does not model, and why

- **L3 (a requirement to the recipe folder it resolves to) is not measured, and not drawn.** A reference
  `zlib/[>=1.2.11 <2]` reaches `recipes/zlib/all/conanfile.py` only through `recipes/zlib/config.yml` (a version
  to a folder) and Conan's own version resolution, and the doc-xref pass cannot see a recipe at all: a recipe is
  a tags-engine file (`TagsDocExtractor.ExtractDoc` returns nothing for it), and `fusion.isDocCandidate` admits
  neither `conanfile.py`, `conanfile.txt` nor `conan.lock`. No `cross_file` selector is declared, so nothing
  is claimed (O370, O371). The reading is L2.
- **`test_package` recipes.** `self.requires(self.tested_reference_str)` names no recipe, and a test_package
  `conanfile.py` has no `name`, so it is no entity and what it requires attaches to nothing.
- **A class with a `name` that does not derive from `ConanFile`**, and a `name` that is not a string literal, are
  no recipe.
- **A recipe's other classes, methods and attributes are not symbols.** conanfile.py is claimed by this dialect
  by basename ahead of the python dialect's `.py`, so its classes and methods (`requirements`, `build`,
  `package_info`) are no longer python symbols; only the recipe entity and its references remain. That is a
  regression for code navigation inside recipes, taken because a callable method entity would take the
  `self.requires` calls off the recipe. A second dialect would be the way back, if it matters.
- **Conditional requirements are all read.** The tags query does not look at an `if`: a `self.requires` behind
  `if self.options.with_ssl:` is a reference. The oracle takes the union over a configuration matrix for the
  same reason (below); a requirement the matrix does not activate is a reference the dialect has and the tool
  does not.
- **Dynamic references** (`self.requires(f"{name}/{version}")`, a variable) name no recipe, and an interpolated string
  names what its first literal run says (`f"boost/{self.version}"` is `boost`).
- **`conandata.yml` patch files, `conan.lock` revisions and timestamps, `[options]` values and `[layout]`
  arguments** are not entities or references.
- **Ownership is the basename**, exact case, at any depth: `conanfile.py`, `conanfile.txt`, `conan.lock`,
  `conandata.yml`. A `conandata.yaml`, a `conanfile.pyc` and a `my-conanfile.py` are not claimed.

## The calibration oracle

`calibration/oracles/conan-graph/conan_graph_oracle.py` (stdlib and the conan venv's PyYAML; never a product
dependency; `calibration/oracles/README.md` has the invocation), tier A, Conan 2.33.0 pinned in a venv, a
`CONAN_HOME` in scratch, `-nr` (no remote).

- **Definitions** are the root node of `conan graph info <recipe dir> --version <v> --format json -nr`: the name
  conan evaluated for each recipe (one `package` at the line of the `name = "..."` literal, which Python's own
  `ast` finds) and `conanfile.txt` for a consumer.
- **References** are the root node's direct dependencies (requires, tool_requires, test_requires, python_requires)
  by name. A graph stops at its first unresolvable requirement, so the closure is exported on demand from the
  conan-center-index checkout with `conan export`.
- **A configuration matrix** stands for what the recipe code decides: the default profile (macOS, apple-clang),
  Linux (gcc 13) and Windows (msvc 193), and on Linux each option at each of its other values, one at a time (an
  option with more than 8 values, `ANY` and more than 80 runs are left out).
