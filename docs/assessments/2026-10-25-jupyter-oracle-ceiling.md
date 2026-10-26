# Jupyter oracle candidates — 2026-10-25

**Measured:** the previous search missed basedpyright's native notebook support.
The [committed reading](../bench/readings/lang/jupyter.json) remains the baseline:
L1 symbol recall .9996 / precision 1 at tier B; L2 reference precision .982 /
recall .9942 at tier C (3,759 matching pairs, 3,828 fathom pairs, 3,781 oracle
pairs). No canonical grade or corpus scope changed. ADR-048 and LB-Q-jupyter-rest
apply; a translated Python export is at most tier B, never tier A.

## Named candidates checked

| Candidate / route | Version | Concrete result |
|---|---|---|
| nbformat | 5.11.1 | Reads/validates the container, not resolved code-cell calls. Reference format parsing does not make the custom call visitor native semantic truth. |
| Existing jupyter_oracle.py | Committed recipe; Python AST + nbformat + ctags 6.2.1 | Exports cells, neutralizes magics and maps positions to raw JSON. Its author-written call visitor is tier C; definitions remain B. |
| scip-python direct batch | 0.6.8; source `3b50a0697bdd` | Exact notebook/control probe indexes Total Project Files 0/1. Batch FileMatcher accepts .py/.pyi; its Pyright cell analysis is a separate entrypoint. |
| scip-python over cell export | 0.6.8 | Viable translated-input route, at most B. No tier-A promise follows from independent position mapping. |
| basedpyright native CLI | 1.40.2, source `344759687c2f` | Exact notebook/control probe reports one analyzed file for each. JSON contains diagnostics/summary, not call/reference pairs. Native notebook discovery works. |
| basedpyright native notebook LSP | 1.40.2 | Scripted notebookDocument/didOpen with unchanged cell texts resolves both probe calls. All 51 nontranslation ML notebooks checked: 893/1,120 enumerable call sites receive definitions after declared ML packages are installed; 227 unresolved sites and six scanner-rejected cells and eight opaque magic lines remain explicit. Coverage only; not resolved pair content. Direct notebook input removes translation, but the independent analyzer remains C under P-T1/OL-R2/OL-R7. |
| Pylance | 2026.4.101 prerelease (public changelog, 2026-10-15); no binary installed | Excluded from the free redistributable offline oracle route. Its proprietary code is unnecessary because basedpyright implements native notebook support. No execution claim. |
| jupytext py:percent | 1.19.6 | Full roundtrip check across both pinned subjects: 113/187 pass; 74 fail (four empty invalid notebooks, 70 readable differences). Cell markers offer a mapping route, but code-source equality fails on 30 readable notebooks. Not asserted lossless or tier A. |

**Claimed by a vendor:** basedpyright explicitly documents notebook support in
[both CLI and language server](https://docs.basedpyright.com/latest/benefits-over-pyright/pylance-features/).
**Measured, source inspection:** the release-matched
[notebook LSP handler](https://github.com/DetachHead/basedpyright/blob/v1.40.2/packages/pyright-internal/src/languageServerBase.ts)
accepts notebook cell synchronization and chains cells for analysis. An editor is
not required: the committed client drives that protocol headlessly. The old
assessment's inference from scip-python's batch filter to notebook checkers was
invalid.

## Committed probes and native coverage

**Measured:** [probe.ipynb](data/jupyter-oracle-2026-10-25/probe.ipynb), identical
[control.py](data/jupyter-oracle-2026-10-25/control.py), the explicit empty
[SCIP package environment](data/jupyter-oracle-2026-10-25/environment.json) and
[runner](data/jupyter-oracle-2026-10-25/probe.py) reproduce batch discovery with
Python 3.13.5. The meaningful result is Total Project Files 0/1, not incidental
SCIP byte counts. The native CLI is diagnostic-only. The separate
[native LSP client](data/jupyter-oracle-2026-10-25/native_lsp.py) sends untranslated
notebook cell text; The independent position scanner follows the baseline cell-magic policy, while
raw text sent to the oracle is unchanged. Rejected cells and opaque magic lines
are counted, never erased from the disclosure. Native definition answers are not a
claim that every call is enumerable or resolved.

```sh
nice -n 10 python docs/assessments/data/jupyter-oracle-2026-10-25/probe.py \
  --basedpyright <local-env>/bin/basedpyright \
  --scip-python workspace/tools/jupyter-scip/node_modules/.bin/scip-python \
  --out workspace/tmp/jupyter-probe-replay
nice -n 10 python docs/assessments/data/jupyter-oracle-2026-10-25/native_lsp.py \
  --server <local-env>/bin/basedpyright-langserver \
  --subject workspace/calibration/ML-For-Beginners \
  --python <local-env>/bin/python --out workspace/tmp/jupyter-native-replay.json
```

**Measured:** ML's four unversioned web-app requirements (scikit-learn, pandas,
numpy, flask) and notebook scientific/NLP libraries were installed before the
893/1,120 result; the empty-environment result was only 400/1,097. The interpreter
path retains its virtual environment identity. No unknown site was converted into
a known pair, dropped from the counts or used to raise a grade.

## Scope and roundtrip checks

**Measured:** at ML-For-Beginners `de2d4e12`, Git contains 2,856 notebooks: 2,805
translations and 51 nontranslation notebooks. The old reading's `-only` list names
38 Python notebooks but records 35 measured files. The other 13 nontranslation
files are nine R notebooks and four empty invalid files; they are explicitly
outside that list. The 35-versus-38 measured-file gap is not silently promoted into
full coverage. The new native feasibility probe and jupytext check attempt all
51, naming unreadable and non-Python files. Future canonical remeasurement must
attempt the full nontranslation set and disclose every exclusion, including the
existing discovery gap, rather than inherit the old list silently.

**Measured:** [roundtrip.py](data/jupyter-oracle-2026-10-25/roundtrip.py) executes
`jupytext --test --to py:percent` for every original notebook and asserts the source
file's digest remains unchanged. Across ML (51) and the handbook (136), 113 pass;
74 fail. Of 183 readable files, 153 preserve every code cell's source exactly and
30 do not. Thus some failures are metadata/other cell differences, not necessarily
code loss. Percent markers can map cells, but they do not establish a lossless raw
JSON position map. The existing exporter remains the canonical baseline's explicit
cell policy; a replacement needs measured mapping/loss accounting.

```sh
nice -n 10 python docs/assessments/data/jupyter-oracle-2026-10-25/roundtrip.py \
  --subject ML-For-Beginners=workspace/calibration/ML-For-Beginners \
  --subject PythonDataScienceHandbook=workspace/calibration/PythonDataScienceHandbook \
  --out workspace/tmp/jupyter-roundtrip-replay.json
```

## Environment ruling and next measurement

**Measured:** the pinned handbook `d6623145` declares Python 3.5, NumPy 1.11.1,
pandas 0.18.1, SciPy 0.17.1, scikit-learn 0.17.1 and other historical pins.
A local ARM/Python 3.13 wheel-only dry run rejects NumPy 1.11.1; that is a
platform/environment failure, not proof of universal unavailability. Owner OL-R4
requires preserving this contract and measuring another modern notebook subject
whose own declared environment installs locally. No replacement dependency
contract is imposed on the handbook. The separate modern experiment below now provides a measured comparison.
Holdouts stay outside canonical shards.

**Hypothesis:** independently source-mapped scip-python over exported Python can
improve the evidence to B at most. basedpyright answers remain tier C under
P-T1, as OL-R2/OL-R7; direct notebook reading removes translation but does not
change ownership or tier. L2 can be lifted only by a maintained SCIP indexer
reading untranslated notebooks or an explicit owner ruling. Complete population,
cell/magic and environment accounting would still be required.


## OL-R4 modern-subject measurement

**Measured:** wesm/pydata-book third edition at `a892791c` supplies all 14 notebooks
under its own installable declarations (Python >=3.9, pandas 2.0.3). The selected
Python 3.11.14 / NumPy 1.26.4 environment satisfies those declarations and imports
numpy, pandas, scipy, sklearn and matplotlib successfully. It is separate from the
handbook and the canonical shard; neither original contract changed. The
[full replay recipe](data/jupyter-oracle-2026-10-25/README.md), exact resolved
environments and [input/source manifest](data/jupyter-oracle-2026-10-25/manifest.json)
are committed.

The native notebook LSP receives unchanged cells. It returns definitions at
**1,945/2,071** enumerated call sites; **126** remain unknown. No cell is rejected;
34 magic lines remain opaque under the declared scanner policy. These are
[full-site receipts](data/jupyter-oracle-2026-10-25/native-modern-results.json), not
an inference from a tiny probe. The old receipts retained only answer category/range, not target URI/name. The
1,270 raw pairs (1,094 normalized) instead come from the tier-C AST walk's written
caller/callee names, filtered by answer presence. That filter supplies no resolved
target truth. An external os.path.join answer may contradict a notebook-local
join edge with the same written name; a nonempty answer cannot confirm it.

The [diagnostic comparison](data/jupyter-oracle-2026-10-25/modern-comparison-summary.json)
contains two **tier-C syntax** readings. The answered-site filter reads
**.9327** (1,094/1,173), recall **1**, explicitly a **lower bound; unknown sites
counted as misses**. It is not native binding precision or an error rate. The
complete written-syntax comparison reads **.9940/1** (1,166/1,173). Both stay L2
good. The 72-pair syntax-filter difference and seven other syntax mismatches are
explicit; no unknown is waived by an oracle-blind rule. Legacy native filenames
remain for receipt continuity, with pair source/tier and the absence of resolved
target truth now stated in the JSON. No held-out file enters the canonical corpus.

**Hypothesis:** basedpyright can improve coverage diagnostics, but its custom
exports remain C even if every answer is known. A maintained SCIP indexer reading
untranslated notebooks, or a documented owner amendment, is the retry trigger for
lifting L2. The translated alternative remains at most B. The handbook's declared
environment is preserved, and the modern subject remains a separate diagnostic.

**Review correction:** the earlier A labels were invalid. This change takes the
controller's C option; it does not claim a new resolved-reference oracle. The
[regression](data/jupyter-oracle-2026-10-25/test_comparison.py) prevents the shadowed
name example from becoming an A claim. The pair content is re-emitted from the committed coverage receipt and asserted
unchanged; lang-regrade recomputes the C tier/grade consequences from recorded
counts. The original full LSP/environment probes, extraction and timing are not
claimed as freshly rerun.
