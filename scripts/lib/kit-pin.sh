# scripts/lib/kit-pin.sh
#
# Shared helpers for resolving the commit that go.mod pins for
# github.com/geoffmilleraz/agentic-kit. Sourced by sync-kit.sh and
# vendor-check.sh (and their test). Assumes cwd is the repo root whose
# go.mod is authoritative — callers `cd` there first.
#
# Not executable on its own; `source` it.

# kit_pin_version
# Echoes the require version string go.mod has for
# github.com/geoffmilleraz/agentic-kit (e.g.
# "v0.7.1-0.20260717061147-deae519e21d2"). Empty if go.mod has no such
# requirement.
kit_pin_version() {
  # Matches both the grouped `require (...)` block form (each line just
  # `path version`) and the single-line `require path version` form.
  grep -E '^[[:space:]]*(require[[:space:]]+)?github\.com/geoffmilleraz/agentic-kit[[:space:]]+v' go.mod \
    | head -1 | awk '{print $NF}'
}

# resolve_pin_commit <sibling-repo-dir>
# Prints the full commit hash go.mod's agentic-kit requirement resolves to,
# validated against the sibling checkout. Returns non-zero (nothing printed)
# if the version can't be parsed or the sibling doesn't have that commit.
resolve_pin_commit() {
  local sibling="$1" version ref
  version=$(kit_pin_version)
  if [ -z "$version" ]; then
    echo "kit-pin: no github.com/geoffmilleraz/agentic-kit requirement in go.mod" >&2
    return 1
  fi

  # Pseudo-version (vX.Y.Z-0.<14-digit-timestamp>-<12-hex-commit>, and the
  # +incompatible/prerelease variants) — the trailing 12 hex chars are the
  # abbreviated commit. Anything else (a tagged release, a literal ref) is
  # passed straight to git.
  if [[ "$version" =~ -([0-9a-f]{12})$ ]]; then
    ref="${BASH_REMATCH[1]}"
  else
    ref="$version"
  fi

  git -C "$sibling" rev-parse --verify "${ref}^{commit}" 2>/dev/null
}
