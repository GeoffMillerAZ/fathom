# Jupyter oracle ceiling — 2026-10-25

The current notebook L2 recipe is capped by its syntax-only calls oracle. A semantic
Python indexer is available, but a notebook adapter still needs to be measured.
**Measured:** the existing [reading](../bench/readings/lang/jupyter.json) has L1 symbol
recall .9996 and precision 1; L2 reference precision .982 and recall .9942 (3,759
matching pairs, 3,828 fathom pairs, 3,781 oracle pairs). These are committed baseline
numbers, not a new regrade. [LB-Q-jupyter-rest](../plans/language-breadth.md) records
the present recipe ceiling under ADR-048. No reading, scope or grade changed.

## Existing work checked

- **Claimed by a vendor:** [nbformat](https://nbformat.readthedocs.io/en/latest/api.html)
  reads, converts and validates notebook structure. It does not resolve code-cell calls.
- **Measured, source inspection:** the existing
  [jupyter oracle](../../calibration/oracles/jupyter/jupyter_oracle.py) exports Python
  cells, neutralizes line magics, skips non-Python cell magics, maps positions back to
  raw JSON and attributes syntax calls with a custom Python AST visitor. Its existing
  tier C cannot become tier A merely because Python supplies the AST or nbformat the
  container. [Python ast](https://docs.python.org/3/library/ast.html) is a syntax tree;
  ctags already supplies tier-B definitions on the same export.
- **Measured, source inspection:** [scip-python](https://github.com/sourcegraph/scip-python)
  at `3b50a0697bdd` uses
  `packages/pyright-scip/src/FileMatcher.ts`'s `.py`/`.pyi` filter for batch indexing.
  Its analyzer has editor-supplied notebook-cell handling, but that is a different
  entry point from the batch file matcher. No `.ipynb` reader was found in those paths.

## Offline batch probe

**Measured:** the current npm release, scip-python 0.6.8, was installed only as a
workspace bench instrument. A valid notebook with a function, a builtin call and a
call to that function was indexed with an explicit empty package environment (the
probe uses only builtins), fixed project name/version and workspace-only output.
The batch command exited 0, reported `Total Project Files 0`, and wrote a 116-byte
SCIP file. Adding an identical `control.py` and rerunning exited 0, reported
`Total Project Files 1`, and wrote 612 bytes. Both runs were sequential at niceness
10 after one-minute load fell below 6. A small instrument probe does not establish
semantic accuracy, dependency coverage or a corpus grade.

## Next trigger

**Hypothesis:** scip-python can index independently exported notebook cells, with a
mapping back to notebook definitions and call sites. A usable oracle must preserve
the current canonical cell model, account for magics and position-mapping losses,
install the subjects' declared dependencies, and count unresolved sites rather than
silently dropping them. Native `.ipynb` support in a maintained checker is another
route. The existing exporter makes an adapter plausible; this assessment does not
claim that a free stronger oracle is impossible. Neither a zero-file success nor
an unmeasured Python export justifies raising the notebook grade. The retry condition
is recorded in LB-Q-jupyter-rest.
