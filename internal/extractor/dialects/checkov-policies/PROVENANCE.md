# checkov-policies — provenance and divergences

LB-Q-checkov-policies, design `docs/design/language-breadth.md` §4, §13. A `tags+doc` dialect (the Conan
shape): one manifest owns two kinds of file and splits them across the engines by its doc spec's `detect`.
No new grammar: a Python check rides the pinned python grammar blob (`tags.scm` below, written for this
dialect against `indexbench lang-probe` output over `testdata/`; only the node shapes are
tree-sitter-python's, MIT), and a YAML graph check is a document read by the doc engine (`gopkg.in/yaml.v3`).
The fixtures are written for the dialect, not copied from a project.

## What it models

- **A Python check** (`CheckovCheck`, canonical `policy`, callable): a class that writes its id as a string
  literal in `__init__` and calls `super().__init__(...)`: a local `id = "CKV_X_1"` (the shape checkov's own
  checks use) or the keyword `id="CKV_X_1"`; the shared base classes of a family (the `Abs*` port and role
  checks) name theirs `check_id`, which is read the same way. It is named for the id, at the line of the
  literal, ending with the class. The base is whatever the class derives from (a family's base is itself a
  check class that no name pattern tells from a helper's); a class with no superclass, with no `super().__init__`
  or with an id that is not a literal (`id=check_id`) is no check.
- **Resource types** (`Calls` on the check): the strings of `supported_resources`, `supported_resource`,
  `supported_entities`, `supported_kind`, `supported_provider` and `supported_data` written as a list or a tuple,
  as a local of `__init__` or as a keyword of the `super().__init__` call. `*` is a type like any other.
- **A YAML graph check** (`CheckovGraphCheck`, canonical `policy`): a mapping with `metadata` and `definition`,
  named for `metadata.id`, signed by `metadata.name`; every `resource_types` value anywhere in the definition
  (a list or one string, inside `and`/`or`/`not` and connection blocks) is a reference, in the entity's calls.

## Ownership: a path layout, not content

The manifest claims files by glob alone, and `fusion.checkovDispatch` (ahead of the python and YAML dispatch,
in `resolveExtractor` and `ReingestFile`) takes the classifier's glob verdict as the whole decision:

- Python: `**/checks/{resource,data,provider,module}/**/*.py`
- YAML: `**/checks/graph_checks/**/*.yaml` and `*.yml`

This is checkov's own layout (`checkov/<framework>/checks/...`), so the subject, and any repository that vendors
or copies it, is read. **A user's `--external-checks-dir` of any other name is not claimed**: the engine claims a
tags+doc dialect's YAML only by basename or glob, never by content (`NewTagsDocExtractor` requires them), and
a content claim for every `.py` would take Python symbols from all Python. That is the divergence from the
queue row's "custom policies"; the way back is a content claim in `checkovDispatch` (a `Base*Check` subclass
that writes an id, a `metadata` plus `definition` mapping) with the harness's `byContent` owning `.py`. The
glob claim also costs: a Python file under such a `checks/resource/` of an unrelated project is no longer
read by the python dialect (its classes, methods and functions are not symbols), and a helper module in the
checks tree (`base_resource_check.py`) yields no entity.

## What it deliberately does not model, and why

- **L3 (a check to the resource definitions it names) is not attempted.** The resource types name Terraform,
  CloudFormation, ARM, Kubernetes and other providers' resources; no resolver here reads those as targets of a
  check, and no `cross_file` selector is declared, so no edge is claimed. The reading is L2.
- **`resource_types: all`** is recorded as the reference `all` (the doc engine has no value exclusion on a
  reference); no checkov check in the subject writes it. The oracle never lists it (it expands `all` to a
  provider's types, which no text names).
- **`connected_resource_types`** are not references: the loader's own `resource_types` lists them for one check
  of the subject and not for the others that write them, so counting them costs 157 extra references to
  recover one.
- **Computed ids and resources** (`id=check_id` handed down, `supported_resources=SOME_CONSTANT`, a list built
  by `+`) name nothing; a check whose id is not a literal is no entity.
- **Conditional definitions** are all read; the oracle reads what the loader registers.

## The calibration oracle

`calibration/oracles/checkov/checkov_oracle.py` (stdlib and the checkov venv's PyYAML; never a product
dependency; `calibration/oracles/README.md` has the invocation), checkov 3.2.500 pinned in a venv under
`workspace/tools/checkov-venv`.

- **Tier A for ids, names and resource types:** checkov's own external-checks loaders, the code behind
  `checkov --external-checks-dir`: `BaseCheckRegistry.load_external_checks` over every
  `checks/{resource,data,provider,module}` directory of the subject (each imported file registers its checks
  through `BaseCheck.__init__`), and `checks_infra.registry.Registry(parser=GraphCheckParser()).load_external_checks`
  over every `checks/graph_checks` directory. The tier is established, not assumed: the loader decides which
  checks exist and what each is registered for. The `--external-checks-dir` command line was not driven end to
  end because it also runs a scan; the loaders are the part it delegates to.
- **Tier C for lines:** the loaders keep no source positions, so a Python check's line is the line of its id
  literal and its end the end of its class (Python's own `ast`), and a YAML check's line and end are PyYAML
  `compose`'s mapping.
- **Resource types the file writes:** a registered resource type is a reference of the file only when the
  class (or the YAML) writes it as a string. A framework's base class supplies the entity list of a Kubernetes
  container check (every workload kind, `containers`), which is registered but no text of the file; 937
  such registrations are left out and counted in the summary (`py_refs_unwritten`).
- **The class's file** is the file of its own `__init__` code, not `sys.modules`: the loader keys modules by
  file stem, so two files of one stem swap a module's file.

## Subject

- `bridgecrewio/checkov` at 3.2.500 (f4d437e7, Apache-2.0), the checks directories only (1,497 `.py` and 256
  YAML files checked out; 1,380 Python files and 256 graph checks the loaders list, 1,636 checks, 2,131
  resource-type references). Read, never vendored. L1 sym recall 1, sym precision .997, range agreement .9927;
  ref recall .9986, ref precision 1: **L2 (L1 and L2 excellent)**.
- **What the oracle misses (5 of 1,641 classes the dialect reads, counted against it):** three bicep checks (the
  one tried raises on import under the pinned venv), one module check whose sibling relative import fails under a
  file-stem module name, and CKV_OCI_22, which the loader registers under the abstract base's class and so
  attributes to another file. **What the dialect misses (3 of 2,131 references):** `scope: provider: aws` of two
  graph checks, which the loader folds into `resource_types`, and one connected resource type the loader lists for
  a check that writes it only under `connected_resource_types`. **12 of 1,636 ranges** disagree on the end line
  (not diagnosed).
