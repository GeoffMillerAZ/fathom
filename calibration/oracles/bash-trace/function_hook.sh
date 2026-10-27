# Native RETURN frames witness functions that actually entered execution.
# Source returns may repeat an active function witness; decoder deduplicates.
__oracle_call_dedup=0
__oracle_call_refused=0
__oracle_call_count=0
__oracle_call_cap=${ORACLE_CALL_ROW_CAP:-100000}
if [[ ! "$__oracle_call_cap" =~ ^[0-9]+$ ]] || (( __oracle_call_cap < 1 || __oracle_call_cap > 100000 )); then
  builtin printf 'REFUSED\0' >> "$ORACLE_CALL_TRACE_FILE"
  __oracle_call_refused=1
fi
if (( BASH_VERSINFO[0] >= 4 )); then
  builtin declare -A __oracle_seen_calls
  __oracle_seen_calls=()
  __oracle_call_dedup=1
fi
__oracle_function_return() {
  local __oracle_return_status="$1"
  local __oracle_callee="${FUNCNAME[1]-}" __oracle_caller="${FUNCNAME[2]-}"
  case "$__oracle_callee" in ''|main|source|__oracle_*) return 0;; esac
  case "$__oracle_caller" in ''|main|source) __oracle_caller='<module>';; esac
  if [[ "$__oracle_call_refused" == 1 ]]; then return 0; fi
  if [[ "$__oracle_call_dedup" == 1 ]]; then
    local __oracle_call_key
    builtin printf -v __oracle_call_key '%q,%q,%q,%q,%q' "${BASH_SOURCE[2]-}" "${BASH_LINENO[1]-0}" "$__oracle_caller" "$__oracle_callee" "${BASH_SOURCE[1]-}"
    if [[ -n "${__oracle_seen_calls[$__oracle_call_key]+seen}" ]]; then return 0; fi
    if (( __oracle_call_count >= __oracle_call_cap )); then
      builtin printf 'REFUSED\0' >> "$ORACLE_CALL_TRACE_FILE"
      __oracle_call_refused=1
      return 0
    fi
    __oracle_seen_calls["$__oracle_call_key"]=1
    __oracle_call_count=$(( __oracle_call_count + 1 ))
  fi
  builtin printf 'CALL\0%s\0%s\0%s\0%s\0%s\0' \
    "${BASH_SOURCE[2]-}" "${BASH_LINENO[1]-0}" "$__oracle_caller" "$__oracle_callee" "${BASH_SOURCE[1]-}" >> "$ORACLE_CALL_TRACE_FILE"
  # Native RETURN restores the function's original status itself. A nonzero
  # observer return can fire a subject's inherited ERR trap inside a protected
  # OR-list, even when the original function failure is expected there.
  return 0
}
set -o functrace
trap '__oracle_function_return "$?"' RETURN
