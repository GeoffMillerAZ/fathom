# scripts/lib/kit-pin.sh
#
# Shared helpers for resolving the commit that go.mod pins for a vendored
# private module. Sourced by sync-kit.sh and vendor-check.sh (and their
# test). Assumes cwd is the repo root whose go.mod is authoritative —
# callers `cd` there first.
#
# Two modules are mirrored (ADR-019, extended 2026-09-01 to app-kit):
# agentic-kit and app-kit. Both are private, and fathom does not build
# without them — so a clone that cannot reach either private repo must
# still find the source in-tree. The functions take the module path so a
# third mirror needs no new copy of this logic.
#
# Not executable on its own; `source` it.

# kit_pin_version [module-path]
# Echoes the require version string go.mod has for module (default
# github.com/geoffmilleraz/agentic-kit), e.g.
# "v0.7.1-0.20260717061147-deae519e21d2"). Empty if go.mod has no such
# requirement.
kit_pin_version() {
  local module="${1:-github.com/geoffmilleraz/agentic-kit}"
  # Matches both the grouped `require (...)` block form (each line just
  # `path version`) and the single-line `require path version` form.
  grep -E "^[[:space:]]*(require[[:space:]]+)?${module//./\\.}[[:space:]]+v" go.mod \
    | head -1 | awk '{print $NF}'
}

# resolve_pin_commit <sibling-repo-dir> [module-path]
# Prints the full commit hash go.mod's requirement for module resolves to,
# validated against the sibling checkout. Returns non-zero (nothing printed)
# if the version can't be parsed or the sibling doesn't have that commit.
resolve_pin_commit() {
  local sibling="$1" module="${2:-github.com/geoffmilleraz/agentic-kit}" version ref
  version=$(kit_pin_version "$module")
  if [ -z "$version" ]; then
    echo "kit-pin: no ${module} requirement in go.mod" >&2
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
