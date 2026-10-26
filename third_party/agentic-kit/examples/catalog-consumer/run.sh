#!/bin/sh
set -eu

here=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd -P)
root=$(CDPATH='' cd -- "$here/../.." && pwd -P)
scratch_parent=${TMPDIR:-/tmp}
if [ ! -d "$scratch_parent" ]; then
  echo "TMPDIR must name an existing directory" >&2
  exit 2
fi
scratch_parent=$(CDPATH='' cd -- "$scratch_parent" && pwd -P)
case "$scratch_parent/" in
  "$root/"*) echo "TMPDIR must be outside the kit checkout" >&2; exit 2 ;;
esac
work=$(mktemp -d "$scratch_parent/catalog-consumer.XXXXXX")
trap 'rm -rf -- "$work"' EXIT HUP INT TERM

export GOWORK=off GOPROXY=off GOSUMDB=off
export GOCACHE="$work/gocache" GOTMPDIR="$work/gotmp"
# Repository-local Git context from the caller must not leak into the fixture.
unset GIT_DIR GIT_WORK_TREE GIT_COMMON_DIR GIT_INDEX_FILE
unset GIT_OBJECT_DIRECTORY GIT_ALTERNATE_OBJECT_DIRECTORIES GIT_NAMESPACE
mkdir -p "$GOCACHE" "$GOTMPDIR"

# CUE validates the authored shape before the Go decoder sees JSON.
(cd "$root" && cue vet ./examples/catalog-consumer/fixtures)
for expression in spec profile policy identity; do
  (cd "$root" && cue export ./examples/catalog-consumer/fixtures -e "$expression") > "$work/$expression.json"
done

cp "$here/main.go" "$work/main.go"
cp "$here/fixtures/definition.txt" "$work/approved-definition.txt"
cat > "$work/go.mod" <<EOF
module example.com/catalog-consumer

go 1.26

require github.com/geoffmilleraz/agentic-kit v0.0.0
EOF
(cd "$work" && go mod edit "-replace=github.com/geoffmilleraz/agentic-kit=$root" && go build -o consumer .)

run_consumer() {
  (cd "$work" && ./consumer -spec "$work/spec.json" -profile "$work/profile.json" \
    -policy "$work/policy.json" -identity "$work/identity.json" \
    -definition "$1" -evidence "$here/fixtures/evidence.txt")
}

run_consumer "$here/fixtures/definition.txt" > "$work/result.json"
if run_consumer "$here/fixtures/changed-definition.txt" > "$work/changed.json" 2> "$work/changed.err"; then
  echo "changed definition unexpectedly prepared" >&2
  exit 1
fi
if [ -s "$work/changed.json" ] || ! grep -q 'criterion definition digest mismatch' "$work/changed.err"; then
  echo "changed definition did not refuse at the binding" >&2
  exit 1
fi
cat "$work/result.json"
