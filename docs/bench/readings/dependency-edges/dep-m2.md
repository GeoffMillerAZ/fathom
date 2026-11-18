# DEP-M2 · Python dependency edges

**Admission.** Measured for the built-in reader. B1, B2 and B3 pass on
1,314 files from three public repositories. The result is **sound, low coverage**
under the preregistered bars. 121 files (9.21%) are complete. The reading measures
direct source imports under the modeled search roots. Runtime-dependent imports
remain incomplete.

## Pins and population

The pins, tracked-file rule and native identity were committed before the first
corpus run in `549938c46`. The rule selects every tracked regular `.py` file
at each commit, case-sensitively. It excludes untracked files and submodule
contents. Each subject has zero tracked symlinks and zero gitlinks. Selection
matches the frozen counts. [Pins](../../../../calibration/oracles/deps-python/pins.json)

| Subject | Commit | Native repository roots, in order | Tracked files / bytes | Python files / bytes |
|---|---|---|---:|---:|
| FastAPI | `864b569cf8453654fc3bc2c64108c0f644e2918c` | `.` | 2,557 / 28,365,862 | 1,175 / 3,386,387 |
| Flask | `22d924701a6ae2e4cd01e9a15bbaf3946094af65` | `src`, `.` | 235 / 1,892,083 | 83 / 573,266 |
| attrs | `c8bb9bfaa2843e7844b5bb13bed4370b51feec4f` | `src`, `.` | 141 / 2,076,325 | 56 / 570,649 |

Tracked manifests hash current regular file bytes, link text if present, and
index commit IDs for gitlinks. The before and after manifests matched in each
checkout. The Git status check also counts generated or untracked inputs as a
run error. The tracked manifest SHA-256s were:

| Subject | Before and after, both checkouts |
|---|---|
| FastAPI | `8b81ba1c2bd01ba7a6ad0ccfdef3dcdc234dcc631664a02ca2039b0357e70806` |
| Flask | `d5c195ad1d3a4daad78832c0651e0d63f77f5ac08e2c2e1987b4b627979868cb` |
| attrs | `32459b4905458f516a160fedd54fe12133080747e3e69cc4bdd012f764183cac` |

## Oracle and limits

The oracle is CPython 3.13.17 `modulefinder.ModuleFinder`, with
`modulefinder.py` SHA-256
`5447268e1fe922d334f59e2f370dae0130ab2b9eff48ca7bd71c4ec544443c8b`.
The run used Darwin arm64 and Go 1.27.4. Every entry gets a fresh finder.
Repository roots and the isolated interpreter's standard library supply the
search path. For regular package entries the adapter supplies their file-derived
package identity, so relative imports have the same context as a package entry.
The trace records filenames returned by modulefinder during the entry's
`IMPORT_NAME` processing, including package initializers selected by a dotted
import. Python bytecode source positions identify statements. Imports from
the code of loaded targets do not enter the trace. The adapter runs before
constructing the Go reader.

The oracle does no project build or installation and executes no subject code.
Generated, installed, or runtime-registered inputs can therefore be absent.
There were zero entry compile failures, unsupported native-file bindings or
unexpected finder errors. The oracle reported zero optimized-away imports in
its covered corpus rows. The fixture exercises a compiled-away import and
a no-truth entry with an unresolvable direct import.

| Oracle gap | FastAPI | Flask | attrs | Pooled |
|---|---:|---:|---:|---:|
| Namespace loader | 87 | 5 | 0 | 92 |
| Package attribute or from-list ambiguity | 782 | 25 | 13 | 820 |
| Relative context | 3 | 4 | 0 | 7 |
| Runtime constructs or hooks | 100 | 22 | 13 | 135 |
| Unresolved direct import | 75 | 18 | 11 | 104 |
| **Files without oracle truth** | **1,047** | **74** | **37** | **1,158** |

The reader reports candidate source files for namespace portions and visible
alternate roots where appropriate. Descent through a namespace, a file
binding that depends on an alternate root, or a missing module search
configuration adds `missing-config`. Package imports whose from-list might
be supplied at runtime and known dynamic import mechanisms add
`dynamic-import`. Python strings used as dynamic operands stay withheld.
These handles do not establish runtime observations or infer external versions.

## B1–B5

The first candidate had 130 complete files, nine without native truth. B1 and
B2 were inconclusive and B3 passed. All nine had a regular leaf package under a
namespace ancestor. The focused namespace-ancestor fixture reproduced the
overclaim before its fix. The one correction disclosed the ancestor's missing
configuration and kept the candidate targets. The pins, oracle, selection and
bars were identical between both trials. The oracle trace SHA-256 was
`cc1e2fb770de62c38df9c5daba34dd548123f3cd9f3b1e56834c095994ea3c02`
in both trials and both checkout paths.

| Subject | Files | Oracle covered | Complete | Complete with truth | Complete without truth | Expected local | Observed local | Missing | Wrong | B2 inconclusive | Extra |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| FastAPI | 1,175 | 128 | 114 | 114 | 0 | 23 | 23 | 0 | 0 | 0 | 0 |
| Flask | 83 | 9 | 3 | 3 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |
| attrs | 56 | 19 | 4 | 4 | 0 | 1 | 1 | 0 | 0 | 0 | 0 |
| **Pooled** | **1,314** | **156** | **121** | **121** | **0** | **24** | **24** | **0** | **0** | **0** | **0** |

B1 and B2 compare complete files with unique native truth. Missing, wrong and
extra refer to local target bindings. Flask's complete files have zero local
targets to compare. B3 passed for each subject. The Go reader's JSON was
byte-identical on the initial run, a repeat with reversed input order, and a
second checkout path. The two independent native traces also matched.

| B4 subject | Complete / files | Extra / comparable local edges | Total edges | Unknowns / evidence | `dynamic-import` / evidence | `missing-config` / evidence |
|---|---:|---:|---:|---:|---:|---:|
| FastAPI | 114 / 1,175 (9.70%) | 0 / 95 | 4,817 | 3,291 / 8,108 | 1,159 / 8,108 | 2,132 / 8,108 |
| Flask | 3 / 83 (3.61%) | 0 / 12 | 1,068 | 879 / 1,947 | 198 / 1,947 | 681 / 1,947 |
| attrs | 4 / 56 (7.14%) | 0 / 24 | 449 | 401 / 850 | 133 / 850 | 268 / 850 |
| **Pooled** | **121 / 1,314 (9.21%)** | **0 / 131** | **6,334** | **4,571 / 10,905 (41.92%)** | **1,490 / 10,905 (13.66%)** | **3,081 / 10,905 (28.25%)** |

B4 extra coverage includes incomplete oracle-covered files. Evidence is
edges plus unknowns. Other unknown codes have count zero. B5 admits the
built-in reader after the corrected candidate passed B1 and B2. The shipped
default reproduced the corrected counts without granting explicit trust.

## Reproduce

From a fresh clone of this change with Git, Go 1.27.4 and CPython 3.13.17
available, run the following in Bash. The direct `go test` commands may be
wrapped in the shared package-test lease on a coordinated machine. The native
test is off by default and requires the named tool and both corpora when opted
in. All file lists, raw native output and errors stay in ignored scratch.

```bash
set -euo pipefail
proof="$PWD/workspace/tmp/dep-m2"
mkdir -p "$proof"/{tmp,gotmp,evidence,corpus-a,corpus-b}
export TMPDIR="$proof/tmp" GOTMPDIR="$proof/gotmp"
export FATHOM_DEPS_PYTHON_EXECUTABLE="$(command -v python3.13)"
export FATHOM_DEPS_PYTHON_CORPUS="$proof/corpus-a"
export FATHOM_DEPS_PYTHON_RELOCATED="$proof/corpus-b"
export FATHOM_DEPS_PYTHON_EVIDENCE="$proof/evidence"

"$FATHOM_DEPS_PYTHON_EXECUTABLE" -I -B -S - <<'PY'
import json
import os
from pathlib import Path
import subprocess

pins = json.loads(Path("calibration/oracles/deps-python/pins.json").read_text())
for arm in ("FATHOM_DEPS_PYTHON_CORPUS", "FATHOM_DEPS_PYTHON_RELOCATED"):
    for subject in pins["subjects"]:
        dest = Path(os.environ[arm], subject["name"])
        dest.mkdir()
        subprocess.run(["git", "-C", dest, "init", "-q"], check=True)
        subprocess.run(["git", "-C", dest, "fetch", "-q", "--depth=1",
                        subject["url"], subject["commit"]], check=True)
        subprocess.run(["git", "-C", dest, "checkout", "-q", "--detach", "FETCH_HEAD"], check=True)
PY

"$FATHOM_DEPS_PYTHON_EXECUTABLE" -I -B -S \
  calibration/oracles/deps-python/test_modulefinder_oracle.py
go test -race ./pkg/lang/deps/... -count=1
FATHOM_DEPS_PYTHON_READ=1 go test -race ./pkg/lang/deps \
  -run '^TestPythonDependencyReading$' -count=1 -v | tee "$proof/reading.log"
```

The direct-trace fixture asserts that changing only a transitive import leaves
truth identical, and changing an entry import changes truth. For the scope
mutation, add `or module.__name__ == "app.helper"` to the guard in
`EntryFinder.scan_code` in
`calibration/oracles/deps-python/modulefinder_oracle.py`, then run
`python3.13 -I -B -S calibration/oracles/deps-python/test_modulefinder_oracle.py OracleTests.test_direct_bytecode_and_filename`.
The mutant failed the direct fixture. The restored adapter passed all three
fixtures. The generic reader cap test `TestDependencyReadCapAndCancellation`
refused a 2 MiB + 1 byte input before dialect dispatch in the leased race suite.
