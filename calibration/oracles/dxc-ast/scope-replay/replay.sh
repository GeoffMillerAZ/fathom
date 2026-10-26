#!/usr/bin/env bash
# Reproduce the HLSL reading from one exact main checkout. All scratch data is
# private under that checkout's workspace/; only the two sanitized JSON files
# in workspace/artifacts/hlsl-scope-replay are publishable.
set -euo pipefail

SOURCE_COMMIT=6db920e72898abb20bf86c22652b401902e34598
DXC_RELEASE=v1.9.2602
DXC_ASSET=linux_dxc_2026_02_20.x86_64.tar.gz
DXC_SHA256=a1d3e3b5e1c5685b3eb27d5e8890e41d87df45def05112a2d6f1a63a931f7d60
DXC_URL="https://github.com/microsoft/DirectXShaderCompiler/releases/download/$DXC_RELEASE/$DXC_ASSET"

check_platform() {
  local kernel=${1:-$(uname -s)} machine=${2:-$(uname -m)}
  if [ "$kernel" != Linux ] || [ "$machine" != x86_64 ]; then
    echo "error: DXC replay requires Linux x86_64; refusing $kernel $machine without executing DXC" >&2
    return 1
  fi
}

check_runner_tools() {
  local tool yq_version jq_version jq_number newest
  for tool in awk cmp curl getconf git jq objdump sed sha256sum sort tar yq; do
    command -v "$tool" >/dev/null || {
      echo "error: runner is missing required tool $tool" >&2; return 1;
    }
  done
  yq_version=$(yq --version)
  case "$yq_version" in *mikefarah*"version v4."*) ;; *)
    echo "error: replay requires mikefarah yq v4; found $yq_version" >&2; return 1;;
  esac
  jq_version=$(jq --version)
  jq_number=${jq_version#jq-}
  newest=$(printf '%s\n' 1.6 "$jq_number" | sort -V | tail -n 1)
  if [ "$newest" != "$jq_number" ]; then
    echo "error: replay requires jq 1.6 or newer; found $jq_version" >&2
    return 1
  fi
}

require_glibc() {
  local required=$1 runner=$2 newest
  newest=$(printf '%s\n' "$required" "$runner" | sort -V | tail -n 1)
  if [ "$newest" != "$runner" ]; then
    echo "error: pinned DXC requires glibc $required, but the runner provides glibc $runner" >&2
    return 1
  fi
}

run_reading() {
  local source_root=$1 clones=$2 clang_bin=$3 raw=$4 private_log=$5
  mkdir -p "$(dirname "$raw")" "$(dirname "$private_log")"
  if ! (cd "$source_root" && \
    HLSL_CLONES="$clones" PATH="$clang_bin:$PATH" \
    bash calibration/oracles/dxc-ast/reading.sh "$raw") >"$private_log" 2>&1; then
    echo "error: HLSL reading stage failed; details remain in the private workspace log" >&2
    return 1
  fi
}

pin_checkout() {
  local checkout=$1 want=$2 got
  got=$(git -C "$checkout" rev-parse HEAD)
  if [ "$got" != "$want" ]; then
    git -C "$checkout" fetch --quiet --depth 1 origin "$want"
    git -C "$checkout" checkout --quiet --detach FETCH_HEAD
  fi
  got=$(git -C "$checkout" rev-parse HEAD)
  [ "$got" = "$want" ] || {
    echo "error: subject checkout is $got, want pinned $want" >&2; return 1;
  }
}

verify_source() {
  local source_root=$1 got
  got=$(git -C "$source_root" rev-parse HEAD)
  [ "$got" = "$SOURCE_COMMIT" ] || {
    echo "error: source checkout is $got, want $SOURCE_COMMIT" >&2; return 1;
  }
  verify_checkout_clean "$source_root" source
  local committed current
  committed=$(git -C "$source_root" show "$SOURCE_COMMIT:calibration/oracles/dxc-ast/reading.sh" | sha256sum | cut -d' ' -f1)
  current=$(sha256sum "$source_root/calibration/oracles/dxc-ast/reading.sh" | cut -d' ' -f1)
  [ "$current" = "$committed" ] || {
    echo "error: reading.sh differs from the committed main driver" >&2; return 1;
  }
}

verify_checkout_clean() {
  local checkout=$1 label=$2 paths path
  if ! git -C "$checkout" diff --quiet || ! git -C "$checkout" diff --cached --quiet; then
    echo "error: $label checkout is dirty; refusing mixed-source replay" >&2
    return 1
  fi
  paths=$({
    git -C "$checkout" ls-files --others --exclude-standard
    git -C "$checkout" ls-files --others --ignored --exclude-standard
  } | LC_ALL=C sort -u)
  while IFS= read -r path; do
    [ -z "$path" ] && continue
    case "$path" in
      workspace|workspace/*) ;;
      *)
        echo "error: $label checkout has untracked or ignored content outside workspace/" >&2
        return 1
        ;;
    esac
  done <<< "$paths"
}

verify_controller() {
  local controller_root=$1 expected_commit=${2:-} commit rel committed actual
  ACTIVE_WORKFLOW_MODE=manual-no-active-workflow
  ACTIVE_WORKFLOW_PATH=
  ACTIVE_WORKFLOW_SHA256=
  commit=$(git -C "$controller_root" rev-parse HEAD)
  if [ -n "$expected_commit" ] && [ "$commit" != "$expected_commit" ]; then
    echo "error: controller commit does not match the workflow commit" >&2
    return 1
  fi
  verify_checkout_clean "$controller_root" controller
  for rel in \
    calibration/oracles/dxc-ast/scope-replay/replay.sh \
    calibration/oracles/dxc-ast/scope-replay/publish.py \
    calibration/oracles/dxc-ast/scope-replay/hlsl-scope-replay.yml; do
    committed=$(git -C "$controller_root" show "$commit:$rel" | sha256sum | cut -d' ' -f1)
    actual=$(sha256sum "$controller_root/$rel" | cut -d' ' -f1)
    if [ "$actual" != "$committed" ]; then
      echo "error: controller file differs from its committed content" >&2
      return 1
    fi
  done
  if [ "${GITHUB_ACTIONS:-}" = true ]; then
    case "${GITHUB_WORKFLOW_REF:-}" in
      */.github/workflows/hlsl-scope-replay.yml@*) ;;
      *)
        echo "error: active workflow reference is not the expected controller path" >&2
        return 1
        ;;
    esac
    ACTIVE_WORKFLOW_PATH=.github/workflows/hlsl-scope-replay.yml
    if ! cmp -s \
      "$controller_root/calibration/oracles/dxc-ast/scope-replay/hlsl-scope-replay.yml" \
      "$controller_root/$ACTIVE_WORKFLOW_PATH"; then
      echo "error: active workflow does not match the committed controller copy" >&2
      return 1
    fi
    ACTIVE_WORKFLOW_MODE=github-actions-verified-active-copy
    ACTIVE_WORKFLOW_SHA256=$(sha256sum "$controller_root/$ACTIVE_WORKFLOW_PATH" | cut -d' ' -f1)
  fi
}

fetch_dxc() {
  local source_root=$1 archive="$source_root/workspace/tools/$DXC_ASSET"
  local install="$source_root/workspace/tools/dxc" got
  mkdir -p "$source_root/workspace/tools" "$install"
  curl --fail --location --retry 3 --proto '=https' --tlsv1.2 \
    --output "$archive" "$DXC_URL"
  got=$(sha256sum "$archive" | cut -d' ' -f1)
  [ "$got" = "$DXC_SHA256" ] || {
    echo "error: DXC archive sha256 $got, want $DXC_SHA256" >&2; return 1;
  }
  tar -xzf "$archive" -C "$install"
}

inspect_dxc_abi() {
  local source_root=$1 private_receipt=$2 required
  local dxc="$source_root/workspace/tools/dxc/bin/dxc"
  local library="$source_root/workspace/tools/dxc/lib/libdxcompiler.so"
  command -v objdump >/dev/null || { echo "error: objdump is required" >&2; return 1; }
  {
    printf 'archive_sha256 %s\n' "$DXC_SHA256"
    objdump -p "$dxc" "$library" | awk '/NEEDED|GLIBC_|GLIBCXX_|CXXABI_/ {print}' | LC_ALL=C sort -u
  } > "$private_receipt"
  required=$(objdump -p "$dxc" "$library" | sed -n 's/.*GLIBC_\([0-9][0-9.]*\).*/\1/p' | sort -V | tail -n 1)
  [ -n "$required" ] || { echo "error: DXC archive names no GLIBC requirements" >&2; return 1; }
  printf '%s\n' "$required"
}

select_clang() {
  local source_root=$1 selected clang_bin="$source_root/workspace/tools/replay-bin"
  if command -v clang-18 >/dev/null; then
    selected=$(command -v clang-18)
  elif command -v clang >/dev/null; then
    selected=$(command -v clang)
  else
    echo "error: clang is required for the committed clang-includes oracle" >&2
    return 1
  fi
  mkdir -p "$clang_bin"
  ln -sf "$selected" "$clang_bin/clang"
  "$clang_bin/clang" --version | sed -n '1p' > "$source_root/workspace/tmp/hlsl-scope-replay/clang-version.txt"
  printf '%s\n' "$clang_bin"
}

verify_corpus() {
  local source_root=$1 manifest="$source_root/calibration/corpus.d/hlsl.yaml"
  local clones="$source_root/workspace/calibration/hlsl" count i name safe want got want_url got_url
  count=$(yq '.repos | length' "$manifest")
  [ "$count" = 3 ] || { echo "error: HLSL manifest has $count subjects, want 3" >&2; return 1; }
  for i in 0 1 2; do
    name=$(yq -r ".repos[$i].name" "$manifest")
    safe=${name##*/}; safe=${safe%% (*}
    safe=$(printf '%s' "$safe" | tr '[:upper:] ' '[:lower:]-' | tr -cd 'a-z0-9._-')
    want=$(yq -r ".repos[$i].sha" "$manifest")
    want_url=$(yq -r ".repos[$i].url" "$manifest")
    got_url=$(git -C "$clones/$safe" remote get-url origin)
    [ "$got_url" = "$want_url" ] || {
      echo "error: subject $name origin is $got_url, want $want_url" >&2; return 1;
    }
    pin_checkout "$clones/$safe" "$want"
    got=$(git -C "$clones/$safe" rev-parse HEAD)
    [ "$got" = "$want" ] || {
      echo "error: subject $name is $got, want pinned $want" >&2; return 1;
    }
  done
}

main() {
  [ "$#" -eq 1 ] || { echo "usage: replay.sh <exact-main-checkout>" >&2; return 2; }
  local source_root script_dir controller_root controller_commit scratch artifacts baseline raw
  local required runner clang_bin private_log replay_sha publish_sha workflow_sha
  local active_workflow_mode active_workflow_path active_workflow_sha
  source_root=$(cd "$1" && pwd)
  script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
  controller_root=$(git -C "$script_dir" rev-parse --show-toplevel)
  scratch="$source_root/workspace/tmp/hlsl-scope-replay"
  artifacts="$source_root/workspace/artifacts/hlsl-scope-replay"
  baseline="$scratch/baseline.json"
  raw="$scratch/regenerated-private.json"
  private_log="$scratch/reading-private.log"

  check_platform
  check_runner_tools
  verify_controller "$controller_root" "${GITHUB_SHA:-}"
  verify_source "$source_root"
  active_workflow_mode=$ACTIVE_WORKFLOW_MODE
  active_workflow_path=$ACTIVE_WORKFLOW_PATH
  active_workflow_sha=$ACTIVE_WORKFLOW_SHA256
  controller_commit=$(git -C "$controller_root" rev-parse HEAD)
  replay_sha=$(sha256sum "$script_dir/replay.sh" | cut -d' ' -f1)
  publish_sha=$(sha256sum "$script_dir/publish.py" | cut -d' ' -f1)
  workflow_sha=$(sha256sum "$script_dir/hlsl-scope-replay.yml" | cut -d' ' -f1)
  mkdir -p "$scratch" "$artifacts"
  cp "$source_root/docs/bench/readings/lang/hlsl.json" "$baseline"
  fetch_dxc "$source_root"
  required=$(inspect_dxc_abi "$source_root" "$scratch/dxc-abi-private.txt")
  runner=$(getconf GNU_LIBC_VERSION | awk '{print $2}')
  require_glibc "$required" "$runner"

  (cd "$source_root" && bash calibration/clone.sh hlsl)
  verify_corpus "$source_root"
  (cd "$source_root" && go build -o workspace/tools/indexbench ./cmd/indexbench)
  clang_bin=$(select_clang "$source_root")
  run_reading "$source_root" "$source_root/workspace/calibration/hlsl" "$clang_bin" "$raw" "$private_log"

  python3 "$script_dir/publish.py" \
    --baseline "$baseline" \
    --regenerated "$raw" \
    --reading-out "$artifacts/regenerated-reading.json" \
    --diff-out "$artifacts/reading-diff.json" \
    --source-commit "$SOURCE_COMMIT" \
    --dxc-sha256 "$DXC_SHA256" \
    --required-glibc "$required" \
    --runner-glibc "$runner" \
    --go-version "$(go version | awk '{print $3}')" \
    --controller-commit "$controller_commit" \
    --replay-sh-sha256 "$replay_sha" \
    --publish-py-sha256 "$publish_sha" \
    --workflow-sha256 "$workflow_sha" \
    --active-workflow-mode "$active_workflow_mode" \
    --active-workflow-path "$active_workflow_path" \
    --active-workflow-sha256 "$active_workflow_sha"

  [ "$(find "$artifacts" -maxdepth 1 -type f | wc -l | tr -d ' ')" = 2 ] || {
    echo "error: publish directory must contain exactly two files" >&2; return 1;
  }
  printf 'publishable artifacts:\n%s\n%s\n' \
    "$artifacts/regenerated-reading.json" "$artifacts/reading-diff.json"
}

if [ "${REPLAY_LIBRARY_ONLY:-}" != 1 ]; then
  main "$@"
fi
