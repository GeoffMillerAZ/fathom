#!/usr/bin/env bash
#
# push-http_test.sh — agentic-kit fathom-pack "Push gates": proves push-http.sh derives a
# smart-HTTP URL from every remote form a clone of this repository is
# pushed from, over disposable temp git repos — never the real checkout,
# never the network (--print-url only). Wired into the kit's `task scripts-check`.
#
# Usage:
#   bash scripts/push-http_test.sh

set -euo pipefail

# A git hook exports GIT_DIR (and, from a linked worktree, more GIT_* paths); inherited here, every
# `git -C <scratch>` below would act on the real repository, not the disposable one.
unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_COMMON_DIR GIT_PREFIX GIT_OBJECT_DIRECTORY GIT_ALTERNATE_OBJECT_DIRECTORIES

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PUSH_HTTP="${SCRIPT_DIR}/push-http.sh"

fail() {
  echo "push-http test: $*" >&2
  exit 1
}

scratch="$(mktemp -d "${TMPDIR:-/tmp}/fathom-push-http-test.XXXXXX")"
trap 'rm -rf "$scratch"' EXIT

case_url() {
  local name="$1" remote_url="$2" want="$3"
  local dir="$scratch/$name"
  mkdir -p "$dir"
  git -C "$dir" init -q
  git -C "$dir" remote add origin "$remote_url"
  local got
  got="$(cd "$dir" && bash "$PUSH_HTTP" --print-url)" || fail "$name: derive failed for '$remote_url'"
  [ "$got" = "$want" ] || fail "$name: '$remote_url' → '$got', want '$want'"
}

case_url scp-like   "git@example.com:owner/repo.git"            "https://example.com/owner/repo.git"
case_url ssh-scheme "ssh://git@example.com/owner/repo.git"      "https://example.com/owner/repo.git"
case_url ssh-port   "ssh://git@example.com:2222/owner/repo.git" "https://example.com/owner/repo.git"
case_url https      "https://example.com/owner/repo.git"        "https://example.com/owner/repo.git"

# A push remote that differs from the fetch remote is the one derived.
dir="$scratch/pushurl"; mkdir -p "$dir"; git -C "$dir" init -q
git -C "$dir" remote add origin "https://example.com/fetch/only.git"
git -C "$dir" remote set-url --push origin "git@example.com:owner/pushed.git"
got="$(cd "$dir" && bash "$PUSH_HTTP" --print-url)"
[ "$got" = "https://example.com/owner/pushed.git" ] || fail "push-url: got '$got'"

# No such remote → exit 2, refused, no network.
dir="$scratch/noremote"; mkdir -p "$dir"; git -C "$dir" init -q
if (cd "$dir" && bash "$PUSH_HTTP" --print-url >/dev/null 2>&1); then
  fail "noremote: expected refusal"
fi
rc=0; (cd "$dir" && bash "$PUSH_HTTP" --print-url >/dev/null 2>&1) || rc=$?
[ "$rc" -eq 2 ] || fail "noremote: exit $rc, want 2"

# A remote form nothing can be derived from → exit 2.
dir="$scratch/local"; mkdir -p "$dir"; git -C "$dir" init -q
git -C "$dir" remote add origin "/srv/git/repo.git"
rc=0; (cd "$dir" && bash "$PUSH_HTTP" --print-url >/dev/null 2>&1) || rc=$?
[ "$rc" -eq 2 ] || fail "local-path: exit $rc, want 2"

echo "push-http test: ok"
