# scripts/lib/build-cache.sh
#
# Project-local build-cache policy. Source this file, then call
# fathom_cache_configure before invoking Go or uv. An unset
# FATHOM_CACHE_ROOT preserves the caller's normal toolchain environment.

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
    local size_kib
    size_kib="$(du -sk "$GOCACHE" 2>/dev/null | awk 'NR == 1 { print $1 }')"
    if [ -z "$size_kib" ]; then
      echo "fathom cache: warning: could not measure $GOCACHE; size guard skipped" >&2
    elif [ "$size_kib" -gt "$((max_mib * 1024))" ]; then
      local lock="$root/.go-build-prune.lock"
      if mkdir "$lock" 2>/dev/null; then
        (
          trap 'rmdir "$lock" 2>/dev/null || true' EXIT
          echo "fathom cache: Go build cache exceeds ${max_mib} MiB; pruning $GOCACHE" >&2
          command "${FATHOM_GO_BINARY:-go}" clean -cache
        ) || return 2
      else
        echo "fathom cache: prune already in progress; continuing" >&2
      fi
    fi
  fi

  export FATHOM_CACHE_CONFIGURED=1
}
