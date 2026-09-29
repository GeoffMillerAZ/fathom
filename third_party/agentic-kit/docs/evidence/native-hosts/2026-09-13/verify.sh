#!/bin/sh
# Offline evidence checks only. Does not invoke a model or install a host.
set -eu
cd "$(dirname "$0")"
shasum -a 256 -c SHA256SUMS
cmp sentinels-before.sha256 sentinels-after.sha256
jq -e 'all(.files[]; .before == .after) and (.absent_after|length)==3' sentinels.json >/dev/null
jq -s -e '
  def response($name; $exit): any(.[]; .type=="hook_response" and .hook_name==$name and .exit_code==$exit);
  def denied($name): any(.[]; .type=="hook_response" and .hook_name==$name and .exit_code==2 and (.stdout|fromjson|.hookSpecificOutput.permissionDecision)=="deny");
  response("SessionStart:startup";0) and response("Stop";0) and
  response("PreToolUse:Edit";0) and
  denied("PreToolUse:Read") and denied("PreToolUse:Write") and denied("PreToolUse:Bash") and
  any(.[]; .type=="permission_denied" and .tool_name=="Edit") and
  ([.[]|select(.type=="tool_call")]|length)==5 and
  ([.[]|select(.type=="tool_result" and .is_error)]|length)==4
' claude-startup.events.jsonl >/dev/null
jq -s -e '
  def response($name; $exit): any(.[]; .type=="hook_response" and .hook_name==$name and .exit_code==$exit);
  def denied($name): any(.[]; .type=="hook_response" and .hook_name==$name and .exit_code==2 and (.stdout|fromjson|.hookSpecificOutput.permissionDecision)=="deny");
  response("SessionStart:resume";0) and response("Stop";0) and
  denied("PreToolUse:Edit") and denied("PreToolUse:Bash") and
  ([.[]|select(.type=="tool_call")]|length)==3 and
  ([.[]|select(.type=="tool_result" and .is_error)]|length)==2
' claude-resume.events.jsonl >/dev/null
jq -e '.ready==false and any(.controls[];.name=="edit" and .status=="unknown")' strict-claude-receipt.json >/dev/null
jq -e '.ready==false and any(.controls[];.name=="tool" and .status=="unknown")' synthetic-codex-receipt.json >/dev/null
jq -e '.model_invocations==0 and .live_event_status=="unknown"' codex-live-limits.json >/dev/null
jq -e 'length==4 and all(.[];.receipt.ready==true and .required_controls==["configuration","runtime"] and .deny_tools==[] and .deny_paths==[])' consumer-receipts.json >/dev/null
jq -e '.offline_elapsed_seconds==105 and .setup_elapsed_seconds==142 and .human_operator_time=="not measured"' setup-measurement.json >/dev/null
jq -e '(.invocations|length)==2 and (.invocations|map(.total_cost_usd*10000000|round)|add)==604956 and .invocation_hard_cap_usd==1 and .authorized_subtask_cap_usd==2' cost.json >/dev/null
printf '%s\n' 'Offline native evidence checks passed; historical observations are not a new live run.'
