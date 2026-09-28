#!/usr/bin/env bash
# run.sh — scip-typescript over EVERY TS/JS file of a calibration subject
# (tier A; language-breadth §1.3, ADR-048 amendment D2). Bench-only: node
# is an instrument, never a product dependency.
#
#   run.sh <tools-dir> <subject-dir> <out.scip> <include-glob>...
#
# A temporary tsconfig.json (allowJs, the given include globs, node_modules
# and dist excluded) replaces the subject's own for the run, so the index
# covers the same files fathom reads rather than only the subject's build
# set (nest's own tsconfig leaves sample/ out: 971 of 1,673 files; axios's
# covers 7 of 158); the checkout is restored with git afterwards.
set -euo pipefail
tools=$1 subj=$2 out=$3; shift 3
bin=$tools/scip-ts/node_modules/.bin/scip-typescript
inc=$(printf '"%s",' "$@"); inc="[${inc%,}]"
cd "$subj"
cat > tsconfig.json <<JSON
{ "compilerOptions": { "allowJs": true, "checkJs": false, "noEmit": true, "target": "ES2021", "module": "commonjs",
  "experimentalDecorators": true, "emitDecoratorMetadata": true, "skipLibCheck": true, "baseUrl": "." },
  "include": $inc, "exclude": ["**/node_modules/**", "dist/**", "**/dist/**"] }
JSON
rc=0
nice -n 10 perl -e 'alarm 1200; exec @ARGV' node --max-old-space-size=4096 "$bin" index --output "$out" || rc=$?
git checkout -q -- tsconfig.json 2>/dev/null || rm -f tsconfig.json
exit $rc
