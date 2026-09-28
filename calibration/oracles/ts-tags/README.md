# tags_oracle.py — tier-C oracle: an upstream tags.scm on an independent runtime

ADR-048 amendment D5: a language dialect with no SCIP producer (bash)
measures L2 call pairs against its grammar's upstream tags query, run by
py-tree-sitter (the C runtime) — never fathom's own wazero engine. Tier C
caps the level at good.

tree-sitter/tree-sitter-bash v0.25.1 (the lock's pin, a06c2e44) ships no
queries/tags.scm (only highlights.scm), so `bash-tags.scm` is the nearest
licence-allowed upstream: Aider-AI/aider's bash-tags.scm @ ff191aff
(Apache-2.0), verbatim under a provenance header. Disclosure: the bash
dialect's own tags.scm was derived from the same file, so the oracle is
independent in runtime, not in query shape.

    python3 -m venv <tools>/pyts && <tools>/pyts/bin/pip install tree-sitter==0.25.2 tree-sitter-bash==0.25.1
    <tools>/pyts/bin/python tags_oracle.py --lang bash --query bash-tags.scm --ext .sh --ext .bash --shebang bash --shebang sh <clone>/bats-core > out/bash-bats-core.json
    ... --defs ...   # definitions instead of calls (a secondary L1 check)

Call pairs: each `@reference.call` command name with the innermost
enclosing `@definition.function` (top-level calls skipped; a variable
assignment is not a callable and never encloses). 2026-09-28: bats-core
70 candidate files / 725 pairs, nvm 277 / 1,770 (extensionless shebang
test scripts included; the harness drops files fathom does not read).
