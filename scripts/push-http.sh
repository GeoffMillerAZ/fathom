#!/usr/bin/env bash
#
# push-http.sh — INC-22 (docs/intent/incidents.md), invariant I41: push over
# smart HTTP so no connection is held open across the pre-push gate.
#
# git opens the transport BEFORE it runs the pre-push hook (it needs the
# remote's refs to hand to the hook). Over ssh that means one session sits
# idle for the whole lefthook gate — validate, the hermetic verify, review,
# five to nine minutes on this repository — and on 2026-09-19 two pushes with
# a fully green gate died with SIGPIPE (exit 141) the moment git tried to
# send the pack down a session the remote had already closed; ssh keepalives
# did not help. Smart HTTP has no session to lose: the ref advertisement is
# one request and the pack is another.
#
# The URL is DERIVED from the push remote, never written here (ADR-011:
# nothing org-specific in the tree): git@host:owner/repo.git and
# ssh://[user@]host[:port]/owner/repo.git become https://host/owner/repo.git;
# an http(s) remote is used as-is. Credentials come from the GitHub CLI when
# it is on PATH (`gh auth git-credential`), otherwise from whatever
# credential helper git already has configured — this script never stores a
# token.
#
# Usage:
#   bash scripts/push-http.sh [<refspec>...]      (default: HEAD:main)
#   bash scripts/push-http.sh --print-url         (derive only; no network)
#   REMOTE=<name> bash scripts/push-http.sh ...   (default: origin)
#
# Exit codes: git push's own; 2 for a remote that cannot be derived.

set -euo pipefail

remote="${REMOTE:-origin}"

derive_url() {
  local url="$1"
  case "$url" in
    http://*|https://*) printf '%s\n' "$url" ;;
    ssh://*)
      # ssh://[user@]host[:port]/path → https://host/path
      local rest="${url#ssh://}"
      local hostpart="${rest%%/*}"
      local path="${rest#*/}"
      hostpart="${hostpart##*@}"
      hostpart="${hostpart%%:*}"
      printf 'https://%s/%s\n' "$hostpart" "$path" ;;
    *:*)
      # scp-like: [user@]host:path → https://host/path
      local hostpart="${url%%:*}"
      local path="${url#*:}"
      hostpart="${hostpart##*@}"
      printf 'https://%s/%s\n' "$hostpart" "$path" ;;
    *) return 1 ;;
  esac
}

if ! url_in="$(git remote get-url --push "$remote" 2>/dev/null)"; then
  echo "push-http: refused — no remote named '$remote' in $(pwd)" >&2
  exit 2
fi
if ! url="$(derive_url "$url_in")"; then
  echo "push-http: refused — cannot derive a smart-HTTP URL from '$url_in'" >&2
  exit 2
fi

if [ "${1:-}" = "--print-url" ]; then
  printf '%s\n' "$url"
  exit 0
fi

refspecs=("$@")
[ ${#refspecs[@]} -gt 0 ] || refspecs=("HEAD:main")

cred=()
if command -v gh >/dev/null 2>&1; then
  cred=(-c 'credential.helper=!gh auth git-credential')
fi
exec git "${cred[@]}" push "$url" "${refspecs[@]}"
