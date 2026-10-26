#!/usr/bin/env bash
# Disposable, offline proof of the generated quality-gate Git/Lefthook path.
set -euo pipefail

for tool in git go cue lefthook awk; do
  command -v "$tool" >/dev/null 2>&1 || { printf 'missing tool: %s\n' "$tool" >&2; exit 2; }
done

source_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
scratch_parent="${TMPDIR:-/tmp}"
mkdir -p "$scratch_parent"
scratch="$(mktemp -d "$scratch_parent/quality-gate-smoke.XXXXXX")"
trap 'rm -rf "$scratch"' EXIT
fixture="$scratch/repo"
remote="$scratch/remote.git"
bin="$scratch/bin"
mkdir -p "$bin"
export PATH="$bin:$PATH"
export GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=/dev/null
unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE

fail() {
  printf 'FAIL: %s\n' "$1" >&2
  if [[ -f "$scratch/last.log" ]]; then tail -30 "$scratch/last.log" >&2; fi
  exit 1
}

(cd "$source_root" && go build -o "$bin/quality-gate" ./cmd/quality-gate)
git init -q -b main "$fixture"
git -C "$fixture" config --local user.name 'Quality Gate Smoke'
git -C "$fixture" config --local user.email 'quality-gate-smoke@example.invalid'
quality-gate init --dir "$fixture/.quality-gate" >"$scratch/last.log" 2>&1 || fail 'init'
quality-gate validate --policy-file "$fixture/.quality-gate/policy.json" >"$scratch/last.log" 2>&1 || fail 'policy validation'
cue vet -d '#Policy' "$fixture/.quality-gate/schema.cue" "$fixture/.quality-gate/policy.json" >"$scratch/last.log" 2>&1 || fail 'CUE policy validation'
cmp "$source_root/cmd/quality-gate/lefthook.yml" "$fixture/.quality-gate/lefthook.yml" || fail 'fragment bytes changed'
printf '{"ok":true}\n' >"$fixture/data.json"
git -C "$fixture" add .quality-gate data.json
git -C "$fixture" commit -qm 'seed committed policy and valid JSON'

# Merge the emitted fragment with existing pre-commit and commit-msg jobs.
printf 'pre-commit:\n  commands:\n    existing-precommit:\n      run: "printf p >> precommit-witness.txt"\n' >"$fixture/lefthook.yml"
awk '
  /^commit-msg:$/ { in_commit=1; print; next }
  in_commit && /^  commands:$/ {
    print
    print "    existing-commit-msg:"
    print "      run: \"printf w >> witness.txt\""
    in_commit=0
    next
  }
  { print }
' "$fixture/.quality-gate/lefthook.yml" >>"$fixture/lefthook.yml"
(cd "$fixture" && lefthook dump) >"$scratch/effective.yml" 2>"$scratch/last.log" || fail 'Lefthook effective config'
grep -q 'existing-precommit:' "$scratch/effective.yml" || fail 'existing pre-commit job lost'
grep -q 'existing-commit-msg:' "$scratch/effective.yml" || fail 'existing commit-msg job lost'
grep -q 'quality-gate-snapshot:' "$scratch/effective.yml" || fail 'quality-gate hook job missing'
(cd "$fixture" && lefthook install) >"$scratch/last.log" 2>&1 || fail 'Lefthook install'
[[ -x "$fixture/.git/hooks/pre-commit" && -x "$fixture/.git/hooks/commit-msg" && -x "$fixture/.git/hooks/pre-push" ]] || fail 'Git hooks not installed'

# The index contains invalid JSON; the working file contains valid JSON.
printf '{"ok":' >"$fixture/data.json"
git -C "$fixture" add data.json
printf '{"ok":true}\n' >"$fixture/data.json"
if git -C "$fixture" commit -m 'must reject staged JSON' >"$scratch/last.log" 2>&1; then fail 'staged violation committed'; fi
grep -q 'json-valid' "$scratch/last.log" || fail 'commit failed for a reason other than quality-gate'
[[ -s "$fixture/witness.txt" ]] || fail 'existing hook job did not run'
[[ -s "$fixture/precommit-witness.txt" ]] || fail 'existing pre-commit job did not run'
quality-gate check --repo "$fixture" --boundary pre-commit --staged --observe --policy-file "$fixture/.quality-gate/policy.json" >"$scratch/observe.log" 2>&1 || fail 'explicit observe should exit zero'
grep -q 'quality-gate observed' "$scratch/observe.log" || fail 'observe receipt missing'
if quality-gate check --repo "$fixture" --boundary pre-commit --staged --policy-ref HEAD --policy-path missing/policy.json >"$scratch/last.log" 2>&1; then fail 'missing policy passed'; else code=$?; fi
[[ "$code" -eq 2 ]] || fail 'missing policy did not return unknown/invalid'

printf '{"ok":true,"rev":2}\n' >"$fixture/data.json"
git -C "$fixture" add data.json
git -C "$fixture" commit -m 'commit valid staged JSON' >"$scratch/last.log" 2>&1 || fail 'valid commit rejected'
before="$(wc -c <"$fixture/witness.txt")"
git -C "$fixture" commit --allow-empty -m 'probe empty commit hook' >"$scratch/last.log" 2>&1 || fail 'empty commit rejected'
after="$(wc -c <"$fixture/witness.txt")"
(( after > before )) || fail 'empty commit skipped commit-msg witness job'
grep -q 'quality-gate pass' "$scratch/last.log" || fail 'empty commit skipped quality-gate'
quality-gate check --repo "$fixture" --boundary pre-commit --staged --policy-ref HEAD --policy-path .quality-gate/policy.json >"$scratch/empty-gate.log" 2>&1 || fail 'direct empty-index gate invocation'
grep -q 'quality-gate pass' "$scratch/empty-gate.log" || fail 'empty-index gate receipt missing'

git init -q --bare "$remote"
git -C "$fixture" remote add origin "$remote"
git -C "$fixture" push origin HEAD:refs/heads/main >"$scratch/last.log" 2>&1 || fail 'initial full-tree pre-push rejected'
clean_oid="$(git -C "$fixture" rev-parse HEAD)"
git -C "$fixture" push origin "$clean_oid:refs/heads/by-oid" >"$scratch/last.log" 2>&1 || fail 'valid raw-OID push rejected'
[[ "$(git --git-dir="$remote" rev-parse refs/heads/by-oid)" == "$clean_oid" ]] || fail 'raw-OID push reached wrong commit'
git -C "$fixture" branch clean
printf '{"ok":' >"$fixture/data.json"
git -C "$fixture" add data.json
# Intentional invalid fixture commit only. The installed hook remains active.
git -C "$fixture" -c core.hooksPath=/dev/null commit -qm 'seed invalid push fixture'
bad_oid="$(git -C "$fixture" rev-parse HEAD)"
if git -C "$fixture" push origin refs/heads/clean:refs/heads/clean refs/heads/main:refs/heads/main >"$scratch/last.log" 2>&1; then fail 'bad committed revision pushed'; fi
grep -q 'json-valid' "$scratch/last.log" || fail 'push rejected for a reason other than quality-gate'
[[ "$(git --git-dir="$remote" rev-parse refs/heads/main)" == "$clean_oid" ]] || fail 'remote main changed on rejected push'
if git --git-dir="$remote" show-ref --verify --quiet refs/heads/clean; then fail 'clean ref pushed despite multi-ref rejection'; fi

printf 'refs/heads/clean %s refs/heads/clean %040d\nrefs/heads/main %s refs/heads/main %s\n' "$clean_oid" 0 "$bad_oid" "$clean_oid" >"$scratch/updates.txt"
if quality-gate pre-push --repo "$fixture" --policy-ref HEAD --format json <"$scratch/updates.txt" >"$scratch/multiref.json" 2>"$scratch/last.log"; then fail 'multi-ref policy passed'; else code=$?; fi
[[ "$code" -eq 1 ]] || fail 'multi-ref policy did not hold'
[[ "$(grep -c '"ref":' "$scratch/multiref.json")" -eq 2 ]] || fail 'pre-push did not report both ref rows'
if printf 'malformed row\n' | quality-gate pre-push --repo "$fixture" --policy-ref HEAD >"$scratch/last.log" 2>&1; then fail 'malformed hook input passed'; else code=$?; fi
[[ "$code" -eq 2 ]] || fail 'malformed hook input did not return unknown/invalid'

printf 'PASS quality-gate smoke: %s\n' "$(quality-gate --help | head -1)"
printf 'PASS staged-invalid/worktree-valid commit refused; existing hook job ran\n'
printf 'PASS empty commit invoked commit-msg witness and quality-gate; direct gate: pass\n'
printf 'PASS valid commit and initial full-tree push; invalid multi-ref push refused\n'
printf 'PASS real Git raw-OID push source accepted\n'
printf 'PASS observe, missing policy, CUE schema, and malformed stdin cases\n'
printf 'VERSIONS %s | %s | %s | lefthook %s\n' "$(git --version)" "$(go version)" "$(cue version | head -1)" "$(lefthook version)"
printf 'SOURCE HEAD %s (binary built from working tree)\n' "$(git -C "$source_root" rev-parse --short=12 HEAD)"
