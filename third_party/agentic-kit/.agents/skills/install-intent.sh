#!/usr/bin/env bash
# Install (or refresh) the intent-* skill family for Claude Code and Codex on this
# machine by symlinking into ~/.agents/skills (read natively by Codex) and
# ~/.claude/skills (read by Claude Code). Idempotent. No copies: the kit checkout is
# the single source. Usage: .agents/skills/install-intent.sh [--uninstall]
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
agents="${HOME}/.agents/skills"
claude="${HOME}/.claude/skills"
mkdir -p "$agents" "$claude"

skills=()
for d in "$here"/intent-*/; do
  [[ -f "$d/SKILL.md" ]] && skills+=("$(basename "$d")")
done
[[ ${#skills[@]} -gt 0 ]] || { echo "no intent-* skills found beside this script" >&2; exit 1; }

if [[ "${1:-}" == "--uninstall" ]]; then
  for s in "${skills[@]}"; do rm -f "$agents/$s" "$claude/$s"; done
  echo "removed ${#skills[@]} skills from $agents and $claude"
  exit 0
fi

for s in "${skills[@]}"; do
  ln -sfn "$here/$s" "$agents/$s"
  ln -sfn "../../.agents/skills/$s" "$claude/$s"
done

fail=0
for s in "${skills[@]}"; do
  [[ -f "$agents/$s/SKILL.md" ]] || { echo "FAIL codex   $s" >&2; fail=1; }
  [[ -f "$claude/$s/SKILL.md" ]] || { echo "FAIL claude  $s" >&2; fail=1; }
done
[[ $fail -eq 0 ]] && echo "installed ${#skills[@]} skills: ${skills[*]}"
echo "Claude Code: /intent-<name>   Codex: \$intent-<name>"
exit $fail
