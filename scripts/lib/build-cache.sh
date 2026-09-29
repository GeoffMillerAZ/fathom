# scripts/lib/build-cache.sh
#
# Project-local build-cache policy. Source this file, then call
# fathom_cache_configure before invoking Go or uv. An unset
# FATHOM_CACHE_ROOT preserves the caller's normal toolchain environment.

# fathom_cache_trim DIR MAX_MIB: bring a Go build cache under MAX_MIB by
# deleting entries by age, never the whole cache (O124, EW-4). Go stamps an
# entry's mtime whenever a build uses it, so an entry touched inside the last
# day may be in a live build's hands and is never removed; the older ones go
# in three passes (unused for 7 days, then 3, then 1) and the loop stops at
# the first pass that fits. A whole-cache clean deletes what a build that
# started a moment earlier is about to read, which is what failed pre-push
# gates when two builds shared one root. Still over the limit after the last
# pass is reported and the build goes on: a slow start beats a failed one.
# fathom_cache_kib DIR: DIR's size in KiB, or nothing when it cannot be
# measured. Always succeeds: a build writing into DIR can make du exit
# non-zero mid-walk, and a caller under `set -e -o pipefail` must not die on
# that.
fathom_cache_kib() {
  du -sk "$1" 2>/dev/null | awk 'NR == 1 { print $1 }' || true
}

fathom_cache_trim() {
  local dir="$1" max_mib="$2" limit_kib size_kib days
  limit_kib="$((max_mib * 1024))"
  size_kib="$(fathom_cache_kib "$dir")"
  if [ -z "$size_kib" ]; then
    echo "fathom cache: warning: could not measure $dir; size guard skipped" >&2
    return 0
  fi
  [ "$size_kib" -gt "$limit_kib" ] || return 0
  echo "fathom cache: Go build cache exceeds ${max_mib} MiB; trimming entries unused for 7, 3 then 1 days in $dir" >&2
  for days in 7 3 1; do
    # Entries sit two directories down (xx/<hash>-a, xx/<hash>-d); the
    # top-level README and trim.txt are not entries.
    find "$dir" -mindepth 2 -type f -mmin "+$((days * 1440))" -delete 2>/dev/null || true
    size_kib="$(fathom_cache_kib "$dir")"
    if [ -n "$size_kib" ] && [ "$size_kib" -le "$limit_kib" ]; then
      return 0
    fi
  done
  echo "fathom cache: Go build cache is still above ${max_mib} MiB after trimming entries unused for a day; entries used within the last day are never removed (a running build may hold them), so it stays" >&2
  return 0
}

fathom_cache_configure() {
  if [ "${FATHOM_CACHE_CONFIGURED:-}" = "1" ]; then
    return 0
  fi

  local root="${FATHOM_CACHE_ROOT:-}"
  if [ -z "$root" ]; then
    export FATHOM_CACHE_CONFIGURED=1
    return 0
  fi

  case "$root" in
    /*) ;;
    *)
      echo "fathom cache: FATHOM_CACHE_ROOT must be an absolute path: $root" >&2
      return 2
      ;;
  esac

  root="${root%/}"
  if [ -z "$root" ] || [ "$root" = "/" ]; then
    echo "fathom cache: refusing unsafe FATHOM_CACHE_ROOT: ${FATHOM_CACHE_ROOT}" >&2
    return 2
  fi
  if [ ! -d "$root" ]; then
    echo "fathom cache: configured root does not exist: $root" >&2
    echo "fathom cache: create it on the intended volume before running the build" >&2
    return 2
  fi
  if [ ! -w "$root" ]; then
    echo "fathom cache: configured root is not writable: $root" >&2
    return 2
  fi

  export FATHOM_CACHE_ROOT="$root"
  export GOCACHE="$root/build"
  export GOMODCACHE="$root/mod"
  export GOTMPDIR="$root/tmp"
  export TMPDIR="$root/tmp"
  export UV_CACHE_DIR="$root/uv"

  mkdir -p "$GOCACHE" "$GOMODCACHE" "$GOTMPDIR" "$UV_CACHE_DIR" || {
    echo "fathom cache: could not create cache directories under $root" >&2
    return 2
  }

  local max_mib="${FATHOM_CACHE_MAX_MIB:-8192}"
  case "$max_mib" in
    ''|*[!0-9]*)
      echo "fathom cache: FATHOM_CACHE_MAX_MIB must be a non-negative integer: $max_mib" >&2
      return 2
      ;;
  esac

  if [ "$max_mib" -gt 0 ]; then
    fathom_cache_trim "$GOCACHE" "$max_mib"
  fi

  export FATHOM_CACHE_CONFIGURED=1
}
