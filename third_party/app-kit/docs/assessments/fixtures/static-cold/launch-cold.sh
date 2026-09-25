#!/bin/sh
set -eu
case "${1-}" in cold-1|cold-2) ;; *) exit 64 ;; esac
[ "$#" -eq 1 ] || exit 64
static_manifest=/Volumes/m2ext/appkit-evidence/appkit-static-cold.7qApnn7w/frozen/pre-execution-v5-sha256.txt
static_expected=0d98d8afc6c480ba735d57f30517476a6bee74ac1f22ad5febbb75dc394bf7c1
static_observed=$(/usr/bin/shasum -a 256 "$static_manifest")
[ "$static_observed" = "$static_expected  $static_manifest" ] || exit 65
/usr/bin/shasum -a 256 -c "$static_manifest" --quiet
printf 'run=%s manifest_sha256=%s\n' "$1" "$static_expected"
exec /opt/homebrew/Cellar/node/26.3.0/bin/node /Volumes/m2ext/appkit-evidence/appkit-static-cold.7qApnn7w/frozen/harness/cold-runner-v5.mjs "$1"
