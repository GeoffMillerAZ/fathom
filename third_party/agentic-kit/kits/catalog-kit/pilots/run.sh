#!/usr/bin/env bash
set -euo pipefail

# Git's repository-local environment can override -C in agent hooks.
while IFS= read -r git_variable; do
  unset "$git_variable"
done < <(git rev-parse --local-env-vars)

kit_root=$(cd "$(dirname "$0")/../../.." && pwd -P)
consumer_root=${KIT_PLATFORM_DIR:-}
review_path=${KIT_PLATFORM_REVIEW_PATH:-}
env_path=${KIT_PLATFORM_ENV_PATH:-}
review_module=${KIT_PLATFORM_REVIEW_MODULE:-}
if [[ -z "$consumer_root" || ! -d "$consumer_root" ]]; then
  echo "catalog-pilots: KIT_PLATFORM_DIR not set or not a directory — skipping"
  exit 0
fi
if [[ -z "$review_path" || -z "$env_path" || -z "$review_module" ]]; then
  echo "catalog-pilots: KIT_PLATFORM_REVIEW_PATH, KIT_PLATFORM_ENV_PATH and KIT_PLATFORM_REVIEW_MODULE must be set — skipping"
  exit 0
fi
consumer_root=$(cd "$consumer_root" && pwd -P)
revision=${CONSUMER_REVISION-821c509035a8bbd98907c83b54372bec70116425}
if [[ ! "$revision" =~ ^[0-9a-fA-F]{40}$ ]]; then
  echo "consumer revision must be a full 40-character commit SHA" >&2
  exit 1
fi
if [[ $(git -C "$consumer_root" cat-file -t "$revision" 2>/dev/null || true) != commit ]]; then
  echo "consumer commit $revision is unavailable in $consumer_root" >&2
  exit 1
fi
scratch_parent=${PILOT_SCRATCH_ROOT:-${TMPDIR:-/tmp}}
if [[ ! -d "$scratch_parent" ]]; then
  echo "scratch parent must exist: $scratch_parent" >&2
  exit 1
fi
scratch_parent=$(cd "$scratch_parent" && pwd -P)
if [[ "$scratch_parent" == "$kit_root" || "$scratch_parent" == "$kit_root/"* || "$scratch_parent" == "$consumer_root" || "$scratch_parent" == "$consumer_root/"* ]]; then
  echo "scratch parent must be outside kit and consumer checkouts: $scratch_parent" >&2
  exit 1
fi
scratch=$(mktemp -d "$scratch_parent/agentic-kit-catalog-pilots-XXXXXXXX")
echo "snapshot=$scratch revision=$revision"
git -C "$consumer_root" archive "$revision" | tar -x -C "$scratch"
sed -e "s#__REVIEW_MODULE__#$review_module#g" -e "s#__REVIEW_PATH__#$review_path#g" \
  "$kit_root/kits/catalog-kit/pilots/review_test.go.txt" > "$scratch/$review_path/internal/standard/catalog_kit_pilot_test.go"
cp "$kit_root/kits/catalog-kit/pilots/env_test.go.txt" "$scratch/$env_path/internal/catalog/catalog_kit_pilot_test.go"
(
  cd "$scratch"
  go work init "./$review_path" "./$env_path" "$kit_root"
  export GOWORK="$scratch/go.work"
  export GOTOOLCHAIN=local
  cd "$review_path"
  go test -race ./internal/standard -run 'TestCatalogKitPilot|TestBuildStandardRequirementSemanticFloors|TestBuildStandardConceptAndFindingFloors|TestBuildStandardRefusesMinTierLoosening|TestLoadStandardWithSourcesPinsDefinitionBytesAndLines' -count=1 -v
  cd "$scratch/$env_path"
  go test -race ./internal/catalog -run 'TestCatalogKitPilot|TestLoadWithUnifiesDeclarationRows' -count=1 -v
)
