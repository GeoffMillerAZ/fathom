#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
# shellcheck source=scripts/lib/build-cache.sh
source "${SCRIPT_DIR}/lib/build-cache.sh"

fathom_cache_configure
exec "${FATHOM_GO_BINARY:-go}" "$@"
