#!/usr/bin/env bash
# run.sh — scip-typescript over EVERY TS/JS file of a calibration subject
# (tier A; language-breadth §1.3, ADR-048 amendment D2). Bench-only: node
# is an instrument, never a product dependency.
#
#   run.sh <tools-dir> <subject-dir> <out.scip> <include-glob>...
#
# A temporary tsconfig.json (allowJs, the given include globs, node_modules
# and dist excluded) stands in for the subject's own for the run, so the index
# covers the same files fathom reads rather than only the subject's build set
# (nest's own tsconfig leaves sample/ out: 971 of 1,673 files; axios's covers
# 7 of 158). It EXTENDS the subject's own tsconfig.json when there is one
# (O441, wave le): the subject's paths, baseUrl, module resolution and custom
# conditions are how its imports resolve (nest's `@nestjs/*` paths, zod's
# `zod/v4/core` self-references), and a run that dropped them resolved none.
# OWN_TSCONFIG=no indexes without it, for a subject whose tsconfig.json does not
# describe the sources measured (axios's is its d.ts type test's: no allowJs,
# module node16, types []). Run it in an oracle checkout whose dependencies are
# installed (README), never the measured one. The subject's tsconfig is restored
# afterwards.
set -euo pipefail
tools=$1 subj=$2 out=$3; shift 3
bin=$tools/scip-ts/node_modules/.bin/scip-typescript
inc=$(printf '"%s",' "$@"); inc="[${inc%,}]"
cd "$subj"
if [ -f tsconfig.json ] && [ "${OWN_TSCONFIG:-yes}" != no ]; then
  mv tsconfig.json .tsconfig.scip-base.json
  head='"extends": "./.tsconfig.scip-base.json",'; extra=''
else
  head=''; extra=', "target": "ES2021", "module": "commonjs", "baseUrl": "."'
fi
if [ -f tsconfig.json ] && [ -z "$head" ]; then mv tsconfig.json .tsconfig.scip-base.json; fi
restore() {
  rm -f tsconfig.json
  if [ -f .tsconfig.scip-base.json ]; then mv .tsconfig.scip-base.json tsconfig.json; fi
}
trap restore EXIT
cat > tsconfig.json <<JSON
{ $head
  "compilerOptions": { "allowJs": true, "checkJs": false, "noEmit": true, "skipLibCheck": true, "composite": false,
    "incremental": false, "experimentalDecorators": true, "emitDecoratorMetadata": true $extra },
  "include": $inc, "exclude": ["**/node_modules/**", "dist/**", "**/dist/**"] }
JSON
nice -n 10 perl -e 'alarm 1800; exec @ARGV' node --max-old-space-size=8192 "$bin" index --output "$out"
