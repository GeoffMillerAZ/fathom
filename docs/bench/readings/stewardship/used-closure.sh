#!/usr/bin/env bash
# used-closure.sh: SV-M3's $0 reading (docs/plans/stewardship.md). On a
# SCRATCH worktree of a fathom checkout, reads `fathom report used` for both
# mirrors, compares the Go units with `go list -deps` and the app-kit CSS
# units with scripts/ui-css.sh's declared list, prunes each mirror to its
# closure with prune-to-closure.py (a throwaway script reading the JSON;
# fathom never prunes, SV-X4), then runs `task build` and
# `scripts/ui-css.sh --check-sources` on the pruned tree.
#
#   used-closure.sh CHECKOUT SCRATCH FATHOM_BIN
#
# CHECKOUT is a fathom clone (never touched: the work happens in a detached
# worktree under SCRATCH). Prints the numbers the reading records.
set -euo pipefail
SRC=$(cd "$1" && pwd)
S=$2
F=$3
HERE=$(cd "$(dirname "$0")" && pwd)
mkdir -p "$S"
S=$(cd "$S" && pwd)
T=$S/prune-tree
git -C "$SRC" worktree remove --force "$T" 2>/dev/null || true
git -C "$SRC" worktree add -q --detach "$T" HEAD
cd "$T"
export GOWORK=off GOFLAGS=-mod=readonly GOPROXY=off

mods=(app-kit agentic-kit)
for m in "${mods[@]}"; do
  "$F" report used "third_party/$m" --json >"$S/$m.used.json"
done

echo "## go units vs go list -deps (all main packages, then cmd/fathom)"
mains=$(go list -f '{{if eq .Name "main"}}{{.ImportPath}}{{end}}' ./... | grep -v '/workspace/' | sort)
for m in "${mods[@]}"; do
  mod=$(awk '/^module /{print $2; exit}' "third_party/$m/go.mod")
  # shellcheck disable=SC2086
  go list -deps $mains | grep -E "^${mod}(/|$)" | sort >"$S/$m.golist.txt"
  python3 -c 'import json,sys; r=json.load(open(sys.argv[1])); print("\n".join(sorted(u["id"] for u in r["used"] if u["kind"]=="go-package")))' "$S/$m.used.json" >"$S/$m.units.txt"
  if cmp -s "$S/$m.golist.txt" "$S/$m.units.txt"; then eq=EQUAL; else eq=DIFFER; fi
  "$F" report used "third_party/$m" --main cmd/fathom --json >"$S/$m.fathom.used.json"
  go list -deps ./cmd/fathom | grep -E "^${mod}(/|$)" | sort >"$S/$m.fathom.golist.txt"
  python3 -c 'import json,sys; r=json.load(open(sys.argv[1])); print("\n".join(sorted(u["id"] for u in r["used"] if u["kind"]=="go-package")))' "$S/$m.fathom.used.json" >"$S/$m.fathom.units.txt"
  if cmp -s "$S/$m.fathom.golist.txt" "$S/$m.fathom.units.txt"; then eq2=EQUAL; else eq2=DIFFER; fi
  echo "$m: all mains units $(wc -l <"$S/$m.units.txt" | tr -d ' ') go-list $(wc -l <"$S/$m.golist.txt" | tr -d ' ') $eq; cmd/fathom units $(wc -l <"$S/$m.fathom.units.txt" | tr -d ' ') go-list $(wc -l <"$S/$m.fathom.golist.txt" | tr -d ' ') $eq2"
done

echo "## app-kit CSS units vs scripts/ui-css.sh SOURCE_DECLARATIONS"
awk '/^SOURCE_DECLARATIONS=\(/{p=1; next} p&&/^\)/{exit} p{gsub(/[" ]/,""); print}' scripts/ui-css.sh |
  grep 'third_party/app-kit/' | sed 's#^\.\./\.\./\.\./##' | sort >"$S/css.declared.txt"
python3 -c 'import json,sys; r=json.load(open(sys.argv[1])); print("\n".join(sorted(u["id"] for u in r["used"] if u["kind"] in ("css-source","css-import"))))' "$S/app-kit.used.json" >"$S/css.units.txt"
if cmp -s "$S/css.declared.txt" "$S/css.units.txt"; then echo "css: $(wc -l <"$S/css.units.txt" | tr -d ' ') units EQUAL the declared list"; else echo "css: DIFFER"; diff "$S/css.declared.txt" "$S/css.units.txt" || true; fi
python3 - "$S/app-kit.used.json" <<'PY'
import json, sys
r = json.load(open(sys.argv[1]))
ui = sorted(u["dir"] for u in r["used"] if u["kind"] == "go-package" and "/appkit/ui/" in u["dir"] + "/")
src = sorted(u["id"].rsplit("/", 1)[0] for u in r["used"] if u["kind"] == "css-source")
print("reached ui packages without an @source:", [d for d in ui if d not in src])
print("@source dirs no Go import reaches:", [d for d in src if d not in ui])
PY

echo "## prune, then build"
for m in "${mods[@]}"; do
  python3 "$HERE/prune-to-closure.py" "$T" "third_party/$m" "$S/$m.used.json"
done
set +e
# FATHOM_CACHE_ROOT unset: the build-cache wrapper keeps the caller's Go
# environment (ADR-039), so the warm module cache serves and nothing is fetched.
env -u FATHOM_CACHE_ROOT GOWORK=off task build >"$S/task-build.log" 2>&1
echo "task build: exit $?"
bash scripts/ui-css.sh --check-sources >"$S/ui-css-check.log" 2>&1
echo "ui-css --check-sources: exit $? ($(tail -1 "$S/ui-css-check.log"))"
go vet ./cmd/fathom >"$S/vet.log" 2>&1
echo "go vet ./cmd/fathom (test files compile too): exit $? ($(grep -c 'no required module\|cannot find\|is not in std\|does not contain package' "$S/vet.log") missing-package lines)"
set -e
