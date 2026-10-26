# C++ parser population screen

`cpp-screen` measures syntax nodes for the exact file population named by an
`indexbench` C++ reading and its private `-unmatched` receipt. It is a parser
diagnostic, not an oracle or an accuracy grade. Run from the repository root
with a clean binary whose Go VCS revision matches the reading's full
`fathom_commit` and the live checkout. The reading's full subject SHA must
match the subject checkout HEAD:

```sh
bash scripts/fathom-go.sh run ./calibration/oracles/scip-clang/cpp-screen \
  -subject workspace/calibration/subject \
  -subject-name public/repository \
  -reading workspace/artifacts/private-reading.json \
  -unmatched workspace/artifacts/private-unmatched.json \
  -out workspace/artifacts/cpp-screen.json
```

The receipt contains counts and hashes, never file paths, source text, or
per-file results. `written` counts the original tree; `selected` counts the
tree chosen by the pinned product's C++ preparse. Each has ERROR-containing,
MISSING-containing, both, each exclusive part, union, and faults. A failed
receipt has `counts_complete: false`; its partial counters are diagnostic and
must not be used as population rates. The command exits nonzero on any
failure. The source hash binds the probe-time bytes; compare it with the
grade-time source identity separately.
The embedded grammar lock binds the grammar blob hash and upstream commit to
the compiled product revision. A reading's optional grammar fields must agree
with that lock when present; fresh `indexbench` readings omit them.

Inputs are bounded at 256 MiB per JSON, 20,000 files, 16 MiB per source,
and 512 MiB total source. JSON traversal also stops at depth 64 or two
million tokens. Equality is accepted; cap + 1 fails the whole run.
Output must be under an ignored workspace of the main checkout or this
worktree, and cannot overlap an input. Source population paths must be sorted,
unique, relative, and free of symlinks. An existing output file is never
replaced.
