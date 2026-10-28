"""Prepare the checksum-pinned official runtime host; execute no native code."""
import argparse
import hashlib
import io
import json
import tarfile
import urllib.request
from pathlib import Path
from prepare_native import disk_floor

HERE = Path(__file__).resolve().parent
MAX_ARCHIVE = 64 * 1024**2
MAX_EXPANDED = 256 * 1024**2
MAX_FILES = 2048


def validate_members(members):
    if len(members) > MAX_FILES or sum(m.size for m in members) > MAX_EXPANDED:
        raise ValueError('runtime archive count/expansion exceeds budget')
    for member in members:
        path = Path(member.name)
        if path.is_absolute() or '..' in path.parts or member.issym() or member.islnk() or not (member.isfile() or member.isdir()):
            raise ValueError('unsafe runtime archive member')


def prepare(cache):
    disk_floor()
    if cache.exists():
        raise ValueError('fresh runtime host directory required')
    pin = json.loads((HERE / 'native-host-candidates-r1.json').read_text())
    item = pin['runtime']
    with urllib.request.urlopen(item['url'], timeout=60) as response:
        raw = response.read(MAX_ARCHIVE + 1)
    if len(raw) > MAX_ARCHIVE or hashlib.sha512(raw).hexdigest() != item['hash']:
        raise ValueError('official runtime archive size/checksum mismatch')
    with tarfile.open(fileobj=io.BytesIO(raw), mode='r:gz') as archive:
        members = archive.getmembers()
        validate_members(members)
        disk_floor()
        cache.mkdir(parents=True)
        archive.extractall(cache)
    host = cache / 'dotnet'
    if not host.is_file():
        raise ValueError('official runtime host is absent')
    summary = dict(version=pin['version'], runtime_archive_sha512=item['hash'],
                   host_sha256=hashlib.sha256(host.read_bytes()).hexdigest(),
                   files=sum(m.isfile() for m in members), bytes=sum(m.size for m in members),
                   native_executed=False)
    (cache / 'preparation-receipt.json').write_text(json.dumps(summary, indent=2) + '\n')
    return summary


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('-cache', type=Path, required=True)
    args = parser.parse_args()
    print(json.dumps(prepare(args.cache.resolve()), sort_keys=True))
