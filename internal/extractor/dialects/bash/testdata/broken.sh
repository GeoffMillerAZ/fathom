#!/usr/bin/env bash
# dialects/bash/testdata/broken.sh — a syntax error the parser must survive
# (a missing "fi" misparses "broken" itself, as tree-sitter-bash's own
# error recovery does — but "good", before it, and "survivor", after it,
# still extract).

good() {
  echo "fine"
}

broken() {
  if [ -z "$X" ]
    echo "missing fi"
}

survivor() {
  echo "still extracted"
}
