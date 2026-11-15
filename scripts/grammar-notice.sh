#!/usr/bin/env bash
# Offline release-only tool. Cache entries are the source tarballs whose exact
# SHA-256s the lock records, named <tarball_sha256>.tar.gz. Populate the cache
# before release; missing sources fail closed. No network or fallback text.
set -euo pipefail
cd "$(dirname "$0")/.."
export GOPROXY=off GOTOOLCHAIN=local
if [ -n "${GRAMMAR_NOTICE_BIN:-}" ]; then
 exec "$GRAMMAR_NOTICE_BIN" "$@"
fi
exec go run ./cmd/grammarnotice "$@"
