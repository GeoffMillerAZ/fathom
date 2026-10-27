# Instrument only; no replacement source builtin or subject test globals.
if [[ "${ORACLE_FUNCTION_ONLY:-0}" == 1 ]]; then
  if [[ "${ORACLE_FULLPATH:-0}" == 1 ]]; then
    builtin shopt -s bash_source_fullpath 2>/dev/null || :
  fi
  __oracle_fullpath=0
  if builtin shopt -q bash_source_fullpath 2>/dev/null; then __oracle_fullpath=1; fi
  builtin printf 'VERSION\0%s\0%s\0' "$BASH_VERSION" "$__oracle_fullpath" >> "$ORACLE_TRACE_FILE"
else
  source "${BASH_ENV%/*}/trace_hook.sh"
fi
if [[ -n "${ORACLE_CALL_TRACE_FILE-}" ]]; then
  source "${BASH_ENV%/*}/function_hook.sh"
fi
