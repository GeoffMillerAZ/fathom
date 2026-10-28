# BASH_ENV instrumentation, without replacing source or rewriting subject files.
# Nested system-Bash children retain their own version; unavailable fullpath
# support remains legacy evidence, never an invented filename.
if [[ "${ORACLE_FULLPATH-0}" == 1 ]]; then
  builtin shopt -s bash_source_fullpath 2>/dev/null || :
fi
__oracle_fullpath=0
if builtin shopt -q bash_source_fullpath 2>/dev/null; then __oracle_fullpath=1; fi
builtin printf 'VERSION\0%s\0%s\0' "$BASH_VERSION" "$__oracle_fullpath" >> "$ORACLE_TRACE_FILE"
__oracle_trace_event() {
  local __oracle_status="$1" __oracle_line="$2" __oracle_command="$3"
  local __oracle_snapshot __oracle_builtin_source=1
  if builtin declare -F source >/dev/null 2>&1 || builtin declare -F . >/dev/null 2>&1; then
    __oracle_builtin_source=0
  fi
  # A definitions-only source returns without any DEBUG event inside its file.
  # extdebug is enabled only in a metadata-reading subshell, never in subject execution.
  if [[ -n "${__oracle_pending_file-}" ]]; then
    if [[ "$__oracle_builtin_source" == 1 && "$__oracle_status" == 0 && "${BASH_SOURCE[1]-}" == "$__oracle_pending_file" ]]; then
      __oracle_snapshot="$(builtin shopt -s extdebug; builtin declare -F $(builtin compgen -A function))"
      local __oracle_defs_kind=DEFS
      if [[ "$__oracle_fullpath" == 1 ]]; then __oracle_defs_kind=DEFS_FULLPATH; fi
      builtin printf '%s\0%s\0%s\0%s\0%s\0%s\0%s\0' "$__oracle_defs_kind" \
        "$__oracle_pending_file" "$__oracle_pending_line" "$__oracle_pending_pwd" \
        "$__oracle_pending_command" "$__oracle_pending_snapshot" "$__oracle_snapshot" >> "$ORACLE_TRACE_FILE"
    fi
    __oracle_pending_file=
  fi
  if [[ "$__oracle_builtin_source" == 1 && "${FUNCNAME[1]-}" == source && -n "${BASH_SOURCE[2]-}" &&
        "${__oracle_prev_source-}" == 1 &&
        "${__oracle_prev_file-}" == "${BASH_SOURCE[2]}" &&
        "${__oracle_prev_line-}" == "${BASH_LINENO[1]}" ]]; then
    builtin printf 'ENTRY\0%s\0%s\0%s\0%s\0' \
      "${BASH_SOURCE[1]}" "${BASH_SOURCE[2]}" "${BASH_LINENO[1]}" "$__oracle_prev_pwd" >> "$ORACLE_TRACE_FILE"
  fi
  if [[ -z "${BASH_SOURCE[2]-}" && -n "${BASH_SOURCE[1]-}" && "${__oracle_root_file-}" != "${BASH_SOURCE[1]}" ]]; then
    builtin printf 'ROOT\0%s\0%s\0' "${BASH_SOURCE[1]}" "$(builtin pwd -P)" >> "$ORACLE_TRACE_FILE"
    __oracle_root_file="${BASH_SOURCE[1]}"
  fi
  __oracle_prev_source=0
  if [[ "$__oracle_builtin_source" == 1 ]]; then
  case "$__oracle_command" in
    '. '*|'\. '*|'source '*)
      __oracle_pending_file="${BASH_SOURCE[1]-}"
      __oracle_pending_line="$__oracle_line"
      __oracle_pending_pwd="$(builtin pwd -P)"
      __oracle_prev_pwd="$__oracle_pending_pwd"
      __oracle_prev_source=1
      __oracle_pending_command="$__oracle_command"
      __oracle_pending_snapshot="$(builtin shopt -s extdebug; builtin declare -F $(builtin compgen -A function))"
      ;;
  esac
  fi
  __oracle_prev_file="${BASH_SOURCE[1]-}"
  __oracle_prev_line="$__oracle_line"
  # Bash restores the interrupted status after DEBUG. Returning that nonzero
  # status ourselves trips errexit in an otherwise protected condition.
  return 0
}
set -o functrace
trap '__oracle_trace_event "$?" "$LINENO" "$BASH_COMMAND"' DEBUG
