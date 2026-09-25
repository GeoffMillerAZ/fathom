#!/bin/sh
set -eu
case "${1-}" in cold-1|cold-2) ;; *) exit 64 ;; esac
[ "$#" -eq 1 ] || exit 64
static_manifest=/Volumes/m2ext/appkit-evidence/appkit-static-replay.ud7EOtjr/frozen/pre-execution-v2-sha256.txt
static_expected=f7ce6d0ba4bac95edadf0479d7912f7e14b6043cae13a46a3d61016ce6b8e6de
static_observed=$(/usr/bin/shasum -a 256 "$static_manifest")
[ "$static_observed" = "$static_expected  $static_manifest" ] || exit 65
/usr/bin/shasum -a 256 -c "$static_manifest" --quiet
printf 'run=%s manifest_sha256=%s\n' "$1" "$static_expected"
exec /opt/homebrew/Cellar/node/26.3.0/bin/node /Volumes/m2ext/appkit-evidence/appkit-static-replay.ud7EOtjr/frozen/cold-runner-v2.mjs "$1"
