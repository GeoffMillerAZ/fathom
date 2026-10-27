#!/usr/bin/env bash
set -eEuo pipefail
trap 'exit 31' ERR
if [ -z "${BASH_VERSION-}" ] || [[ -z "${BASH_VERSINFO-}" ]] || (( BASH_VERSINFO[0] < 3 )); then
  exit 8
fi
false_function() { false; }
if false_function; then exit 9; fi
conditional_driver() { if false_function || false_function || false_function; then exit 11; fi; :; }
conditional_driver
set +e
trap - ERR
false
original_status=$?
[ "$original_status" = 1 ] || exit 10
printf 'survived\n'
