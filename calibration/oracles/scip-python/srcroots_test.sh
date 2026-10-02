#!/usr/bin/env bash
# srcroots_test.sh: srcroots.py names a src layout, a pyproject-declared root and nothing for a flat
# package; no subject is installed or indexed. Run: bash calibration/oracles/scip-python/srcroots_test.sh
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
t=$(mktemp -d "${TMPDIR:-/tmp}/srcroots.XXXXXX"); trap 'rm -rf "$t"' EXIT
mkdir -p "$t/flat/pkg" "$t/srcl/src/pkg" "$t/decl/lib/pkg" "$t/empty/src/notpkg"
touch "$t/flat/pkg/__init__.py" "$t/srcl/src/pkg/__init__.py" "$t/decl/lib/pkg/__init__.py"
printf '[tool.setuptools.package-dir]\n"" = "lib"\n' > "$t/decl/pyproject.toml"
chk() { got=$(python3 "$here/srcroots.py" "$1"); [ "$got" = "$2" ] || { echo "FAIL $1: got '$got' want '$2'" >&2; exit 1; }; }
chk "$t/flat" ""
chk "$t/srcl" "$(cd "$t/srcl/src" && pwd -P)"
chk "$t/decl" "$(cd "$t/decl/lib" && pwd -P)"
chk "$t/empty" ""
echo ok
