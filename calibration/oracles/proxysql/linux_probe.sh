#!/bin/sh
# Run only inside a disposable Ubuntu 24.04 container; /probe is lane scratch.
# The subject and this repository are mounted read-only at /repo.
set -eu
case "${1:-}" in
install)
  apt-get update
  DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends python3 libgnutls30t64 libssl3t64 libcurl4t64 libatomic1 libnuma1 ca-certificates
  ;;
measure)
  label=$2
  package=$3
  python3 - "/probe/$package" /repo/calibration/oracles/proxysql/candidates-t-r2.json <<'PYVERIFY'
import hashlib, json, sys
from pathlib import Path
p = Path(sys.argv[1])
metadata = json.loads(Path(sys.argv[2]).read_text())
assets = [a for r in metadata["releases"] for a in r["assets"] if a["name"] == p.name]
if len(assets) != 1 or "sha256:" + hashlib.sha256(p.read_bytes()).hexdigest() != assets[0]["digest"]:
    raise SystemExit("package identity mismatch")
print(p.name, "published digest verified")
PYVERIFY
  mkdir -p "/probe/$label" "/probe/work-$label"
  dpkg-deb --fsys-tarfile "/probe/$package" | tar --no-same-owner -x -C "/probe/$label"
  binary="/probe/$label/usr/bin/proxysql"
  "$binary" --version
  python3 /repo/calibration/oracles/proxysql/proxysql_oracle.py \
    -subject /repo/workspace/calibration/proxysql/proxysql \
    -proxysql "$binary" -work "/probe/work-$label" -out "/probe/$label-oracle.json"
  ;;
*) echo 'usage: linux_probe.sh install | measure LABEL PACKAGE' >&2; exit 2 ;;
esac
