"""Fetch the pinned official decK archive into private scratch; no install."""
import argparse
import hashlib
import io
import json
import shutil
import tarfile
import urllib.request
from pathlib import Path

URL = 'https://github.com/Kong/deck/releases/download/v1.68.0/deck_1.68.0_darwin_all.tar.gz'
SHA256 = '78e1cc4b9257483e84ca7bc391cec9a0a0c47bffc47ed2f1266263205e4a585b'
SIZE = 42678741


def fetch(out):
    if shutil.disk_usage(Path.home()).free < 20 * 1024**3:
        raise ValueError('disk below 20 GiB before official download')
    out.mkdir(parents=True, exist_ok=False)
    with urllib.request.urlopen(URL, timeout=60) as response:
        data = response.read(SIZE + 1)
    if len(data) != SIZE or hashlib.sha256(data).hexdigest() != SHA256:
        raise ValueError('official archive identity mismatch')
    with tarfile.open(fileobj=io.BytesIO(data), mode='r:gz') as archive:
        entries = archive.getmembers()
        members = [entry for entry in entries if entry.name == 'deck' or entry.name == './deck']
        if len(members) != 1 or not members[0].isfile() or members[0].size > 256 * 1024**2:
            raise ValueError('unexpected official binary member')
        raw = archive.extractfile(members[0]).read(256 * 1024**2 + 1)
    (out / 'deck').write_bytes(raw)
    (out / 'deck').chmod(0o700)
    receipt = dict(tool='official Kong decK', version='1.68.0', url=URL, archive_sha256=SHA256,
                   archive_bytes=SIZE, binary_sha256=hashlib.sha256(raw).hexdigest(), binary_bytes=len(raw),
                   checksum_source='https://github.com/Kong/deck/releases/tag/v1.68.0',
                   executed=False)
    (out / 'receipt.json').write_text(json.dumps(receipt, indent=2) + '\n')
    return receipt


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('-out', type=Path, required=True)
    args = parser.parse_args()
    print(json.dumps(fetch(args.out), sort_keys=True))
