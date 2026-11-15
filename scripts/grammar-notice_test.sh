#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
mkdir -p workspace/tmp
scratch=$(mktemp -d workspace/tmp/grammar-notice-test.XXXXXX)
trap 'rm -rf "$scratch"' EXIT
export GOPROXY=off GOTOOLCHAIN=local
# The caller holds the shared gate lease for this build and all checks.
go build -o "$scratch/grammarnotice" ./cmd/grammarnotice
export GRAMMAR_NOTICE_BIN="$PWD/$scratch/grammarnotice"
python3 - "$scratch" <<'PY'
import gzip,hashlib,io,json,pathlib,sys,tarfile
root=pathlib.Path(sys.argv[1]); (root/'sources').mkdir()
b=io.BytesIO()
with tarfile.open(fileobj=b,mode='w:gz') as t:
    body=b'Copyright synthetic fixture authors\nSynthetic licence fixture\n'
    h=tarfile.TarInfo('fixture/LICENSE'); h.size=len(body); t.addfile(h,io.BytesIO(body))
raw=b.getvalue(); digest=hashlib.sha256(raw).hexdigest()
(root/'sources'/f'{digest}.tar.gz').write_bytes(raw)
(root/'lock.json').write_text(json.dumps({'grammars':[{'blob':'fixture','repo':'fixture/grammar','commit':'a'*40,'licence':'MIT','tarball_sha256':digest}]}))
PY
bash scripts/grammar-notice.sh generate "$scratch/lock.json" "$scratch/sources" "$scratch/THIRD-PARTY-NOTICES.txt"
# Both publication stubs are behind the same check used by release.yml.
attest() { touch "$scratch/attested"; }
upload() { touch "$scratch/uploaded"; }
release_path() (
 set -e
 bash scripts/grammar-notice.sh check "$scratch/lock.json" "$scratch/sources" "$scratch/release.tar.gz"
 attest
 upload
)
test_release_stops_before_upload_without_notice() {
 rm "$scratch/THIRD-PARTY-NOTICES.txt"
 tar -czf "$scratch/release.tar.gz" -C "$scratch" lock.json
 set +e
 release_path >"$scratch/missing.log" 2>&1
 status=$?
 set -e
 test "$status" -ne 0
 test ! -e "$scratch/attested"
 test ! -e "$scratch/uploaded"
 rg 'release.tar.gz.*missing notice' "$scratch/missing.log"
}
test_release_stops_before_upload_without_notice
bash scripts/grammar-notice.sh generate "$scratch/lock.json" "$scratch/sources" "$scratch/THIRD-PARTY-NOTICES.txt"
tar -czf "$scratch/release.tar.gz" -C "$scratch" THIRD-PARTY-NOTICES.txt
release_path
test -e "$scratch/attested"
test -e "$scratch/uploaded"
# Pin the actual workflow's phase ordering, not merely the stubs above.
python3 - <<'PY'
from pathlib import Path
s=Path('.github/workflows/release.yml').read_text()
assert 'release --clean --skip=publish' in s
assert s.index('scripts/grammar-notice.sh check') < s.index('actions/attest-build-provenance@') < s.index('gh release upload')
assert 'if: always()' not in s
s=Path('.goreleaser.yaml').read_text()
assert 'scripts/grammar-notice.sh generate' in s
assert 'THIRD-PARTY-NOTICES.txt' in s
PY
echo 'PASS: release rejects absent notice before attest/upload; complete notice passes'
