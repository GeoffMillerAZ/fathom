# scripts/lib/fleet-services.sh
#
# The fleet scripts' service list. Source this file, then call
# fathom_fleet_services REPO_ROOT: it sets the SERVICES array from the
# gitignored REPO_ROOT/workspace/fleet-services (one service name per line,
# `#` starts a comment, blank lines skipped). The operator's own repositories
# are named in that local file, never in a tracked one. Without the file, or
# with no names in it, the fleet is fathom alone, and stderr says so.

fathom_fleet_services() {
  local file="$1/workspace/fleet-services" line
  SERVICES=()
  if [ -f "$file" ]; then
    while IFS= read -r line || [ -n "$line" ]; do
      line="${line%%#*}"
      line="${line//[[:space:]]/}"
      [ -n "$line" ] && SERVICES+=("$line")
    done <"$file"
  fi
  if [ "${#SERVICES[@]}" -eq 0 ]; then
    echo "fleet-services: no service names in $file (one per line); the fleet is fathom alone" >&2
    SERVICES=(fathom)
  fi
  return 0
}
