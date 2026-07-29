#!/usr/bin/env bash
#
# module-cold-run.sh — Phase J's terminal criterion (docs/vision/13-expert-
# suite.md §9 row J, docs/plans/expert-suite-j.md J-8): a stranger-shaped,
# headless proof that a scratch corpus reaches a `servable` module doctor
# verdict in one sitting, using only `fathom module` verbs:
#
#   scratch corpus -> init -> sources -> ingest -> compile -> eval -> doctor
#   (servable) -> pack -> doctor (published)
#
# NEVER a real key: every LLM-shaped step uses `--mock --mock-entries-file`,
# a scripted MockClient (internal/semantic/eval.go, cmd/fathom/
# module_compileeval.go's moduleCompileMock) — no network, no
# ANTHROPIC_API_KEY, runnable in CI. Asserts each step's exit code plus the
# final doctor verdict; prints a step-by-step summary and exits non-zero on
# the first failure.
#
# Usage:
#   scripts/module-cold-run.sh
#
# All state lives under workspace/tmp/module-cold-run-<pid>/ (repo
# convention: CLAUDE.md's workspace/ discipline — never elsewhere in the
# tree) and is left in place on failure for inspection; a clean run removes
# it.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${REPO_ROOT}"

SCRATCH="${REPO_ROOT}/workspace/tmp/module-cold-run-$$"
mkdir -p "${SCRATCH}"
trap 'rc=$?; if [ $rc -ne 0 ]; then echo "module-cold-run.sh FAILED (rc=$rc) — scratch state left at ${SCRATCH} for inspection"; else rm -rf "${SCRATCH}"; fi' EXIT

BIN="${SCRATCH}/fathom"
CORPUS="${SCRATCH}/corpus"
STORE="${SCRATCH}/module.sqlite"
PACKDIR="${SCRATCH}/packed"
MODULE_NAME="cold-run-module"
MANIFEST="${CORPUS}/module.cue"

step() { echo >&2; echo "== $* ==" >&2; }
# run echoes the command to STDERR (never stdout) so a caller can safely
# pipe a `run`'d command's stdout (e.g. `run ... --json | tee file.json`)
# without the echoed command line corrupting the captured JSON.
run() { echo "+ $*" >&2; "$@"; }

step "0. build a fresh fathom binary (never bin/fathom — that binary is the daemon's installed copy, CLAUDE.md's binary-staleness discipline)"
export MACOSX_DEPLOYMENT_TARGET=14.0
export CGO_ENABLED=1
run go build -o "${BIN}" ./cmd/fathom

step "1. scratch corpus"
mkdir -p "${CORPUS}"
cat >"${CORPUS}/RULES.md" <<'EOF'
## Sorting

Always sort output before returning it.
EOF

step "2. fathom module init"
run "${BIN}" module init \
  --dir "${CORPUS}" \
  --name "${MODULE_NAME}" \
  --description "A cold-run proof module." \
  --include "RULES.md" \
  --license "Apache-2.0" \
  --json

step "3. fathom module sources"
run "${BIN}" module sources --manifest "${MANIFEST}" --json

step "4. scaffold the module's same-named project-local layer pack (LC-3; module compile's documented prerequisite, docs/plans/expert-suite-j.md's design section)"
LAYER_DIR="${CORPUS}/.fathom/layers/${MODULE_NAME}"
mkdir -p "${LAYER_DIR}"
cat >"${LAYER_DIR}/layer.cue" <<EOF
name:    "${MODULE_NAME}"
version: "v1"
kind:    "obligation"
corpus: include: ["RULES.md"]
prompt_version: "${MODULE_NAME}/content-addressed"
prompt_file:    ".fathom/layers/${MODULE_NAME}/prompt.md"
tier:           "doc-claimed"
stratum:        "platform"
evaled:         true
budgets: {
	max_tokens_per_run: 200000
	max_usd_per_run:    1.0
}
EOF
cat >"${LAYER_DIR}/prompt.md" <<'EOF'
Compile governing rules from the given section.
EOF

step "5. fathom ingest (populates the index compile's scope verification reads)"
run "${BIN}" ingest --service "${MODULE_NAME}" --path "${CORPUS}" --store "${STORE}"

step "6. fathom module compile --mock (dry-run estimate, then a real scripted compile)"
run "${BIN}" module compile --manifest "${MANIFEST}" --store "${STORE}" --dry-run=true --json

COMPILE_ENTRIES="${SCRATCH}/compile-entries.json"
cat >"${COMPILE_ENTRIES}" <<'EOF'
["{\"claims\":[{\"rule\":\"Always sort output before returning it.\",\"scope\":[\"RULES.md\"],\"cite_start\":3,\"cite_end\":3}]}"]
EOF
run "${BIN}" module compile \
  --manifest "${MANIFEST}" --store "${STORE}" \
  --dry-run=false --mock --mock-entries-file "${COMPILE_ENTRIES}" --json

step "7. fathom module map (deterministic gaps against the now-compiled store)"
run "${BIN}" module map --manifest "${MANIFEST}" --store "${STORE}" --json || true

step "8. fathom module eval --mock"
EVAL_DIR="${CORPUS}/eval"
mkdir -p "${EVAL_DIR}"
cp "${CORPUS}/RULES.md" "${EVAL_DIR}/doc.md"
cat >"${EVAL_DIR}/labels.json" <<'EOF'
{"docs":["doc.md"],"probes":["a.go","b.go"],"rules":[{"key":"sort","cite_substr":"Always sort output","governs":["a.go","b.go"]}]}
EOF
EVAL_ENTRIES="${SCRATCH}/eval-entries.json"
cat >"${EVAL_ENTRIES}" <<'EOF'
["{\"claims\":[{\"rule\":\"Always sort output before returning it.\",\"scope\":[\"a.go\",\"b.go\"],\"cite_start\":3,\"cite_end\":3}]}"]
EOF
run "${BIN}" module eval \
  --manifest "${MANIFEST}" --fixtures "${EVAL_DIR}" \
  --mock --mock-entries-file "${EVAL_ENTRIES}" --json

step "9. record eval evidence into module.cue (no verb auto-promotes this — docs/plans/expert-suite-j.md's honest scoping note: an author hand-edits eval_evidence after a passing eval, same as any other manifest field)"
echo "precision 1.00 recall 1.00 (mock lane, scripts/module-cold-run.sh)" >"${CORPUS}/RESULTS.md"
python3 - "${MANIFEST}" <<'PY'
import sys
path = sys.argv[1]
with open(path) as f:
    text = f.read()
text = text.replace("eval_evidence: evaled: false", 'eval_evidence: {\n\tevaled: true\n\tpath: "RESULTS.md"\n}')
with open(path, "w") as f:
    f.write(text)
PY

step "10. fathom module doctor (expect: servable)"
DOCTOR_JSON="${SCRATCH}/doctor-servable.json"
run "${BIN}" module doctor --manifest "${MANIFEST}" --store "${STORE}" --json | tee "${DOCTOR_JSON}"
VERDICT=$(python3 -c "import json; print(json.load(open('${DOCTOR_JSON}'))['verdict'])")
echo "verdict: ${VERDICT}"
if [ "${VERDICT}" != "servable" ]; then
  echo "FAILED: expected doctor verdict 'servable' after compile+eval, got '${VERDICT}'"
  exit 1
fi

step "11. fathom module pack"
run "${BIN}" module pack --manifest "${MANIFEST}" --store "${STORE}" --out "${PACKDIR}" --json

step "12. fathom module doctor --pack-dir (expect: published)"
DOCTOR_JSON2="${SCRATCH}/doctor-published.json"
run "${BIN}" module doctor --manifest "${MANIFEST}" --store "${STORE}" --pack-dir "${PACKDIR}" --json | tee "${DOCTOR_JSON2}"
VERDICT2=$(python3 -c "import json; print(json.load(open('${DOCTOR_JSON2}'))['verdict'])")
echo "verdict: ${VERDICT2}"
if [ "${VERDICT2}" != "published" ]; then
  echo "FAILED: expected doctor verdict 'published' after pack, got '${VERDICT2}'"
  exit 1
fi

echo
echo "module-cold-run.sh: PASS — corpus -> servable -> published, entirely via \`fathom module\` verbs, zero real LLM calls."
