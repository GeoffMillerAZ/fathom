#!/bin/bash
# gen_compdb.sh <checkout> <build dir> [cmake -D options...]
# Configures a CMake subject with CMAKE_EXPORT_COMPILE_COMMANDS=ON and copies the database it writes to the
# checkout's ROOT (the one place ingest reads a database from: `build/` is excluded by .fathomignore's CT-8
# default, so a database in a build tree is never indexed, and includeRoots reads compile_commands.json and
# build/compile_commands.json only). The copy is GENERATED, never committed with a subject; disclose it in a
# reading. The build directory stays outside the checkout.
set -euo pipefail
src="$(cd "$1" && pwd)"; build="$2"; shift 2
case "$(cd "$(dirname "$build")" && pwd)/$(basename "$build")/" in "$src"/*) echo "gen_compdb: build dir inside the checkout" >&2; exit 2;; esac
cmake -S "$src" -B "$build" -DCMAKE_EXPORT_COMPILE_COMMANDS=ON -Wno-dev "$@" >/dev/null
cp "$build/compile_commands.json" "$src/compile_commands.json"
python3 -c "import json,sys; d=json.load(open(sys.argv[1])); print(len(d), 'entries ->', sys.argv[1])" "$src/compile_commands.json"
