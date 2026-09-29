# Input: one raw native JSONL log, slurped. $fixture is its original project path.
# Projection deliberately excludes model prose/thinking, auth/session metadata,
# sockets, UUIDs, request IDs, model IDs, and duplicate wire payloads.
. as $events |
([.[] | select(.type=="assistant") | .message.content[] |
  select(.type=="tool_use") | .id] | unique) as $tools |
([.[] | select(.subtype=="hook_started") | .hook_id] | unique) as $hooks |
def toolid($id): "tool-" + (($tools | index($id)) + 1 | tostring);
def hookid($id): "hook-" + (($hooks | index($id)) + 1 | tostring);
to_entries[] | .key as $position | .value |
if .subtype=="init" then
  {type:"init",host_version:.claude_code_version,permission_mode:.permissionMode,tools}
elif .subtype=="hook_started" then
  {type:.subtype,hook_id:hookid(.hook_id),hook_name,hook_event}
elif .subtype=="hook_response" then
  {type:.subtype,hook_id:hookid(.hook_id),hook_name,hook_event,stdout,stderr,exit_code,outcome}
elif .subtype=="permission_denied" then
  {type:.subtype,tool_id:toolid(.tool_use_id),tool_name,decision_reason_type,decision_reason}
elif .type=="assistant" then
  .message.content[] | select(.type=="tool_use") |
  {type:"tool_call",tool_id:toolid(.id),tool_name:.name,input}
elif .type=="user" then
  .message.content[] | select(.type=="tool_result") |
  {type:"tool_result",tool_id:toolid(.tool_use_id),is_error:(.is_error // false),content}
elif .type=="result" then
  {type,subtype,is_error,num_turns,total_cost_usd,duration_ms,duration_api_ms}
else empty end |
{original_event_index:$position,session:"session-1"} + . |
walk(if type=="string" then split($fixture) | join("$FIXTURE_ROOT") else . end)
