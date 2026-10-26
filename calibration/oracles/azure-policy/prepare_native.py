"""Prepare only the pinned ARM64 vendor payload, without executing it."""
import argparse
import hashlib
import json
import platform
import shutil
import struct
import zlib
from pathlib import Path, PurePosixPath

from audit_extension import HERE, LIMIT, directory_rows, fetch_range

PREFIX = 'extension/out/artifacts/osx-arm64/'
MAX_FILES = 512
MAX_SINGLE_BYTES = 64 * 1024**2
MAX_TOTAL_BYTES = 160 * 1024**2
MAX_SPAN_BYTES = 80 * 1024**2


def disk_floor():
    if shutil.disk_usage(Path.home()).free < 20 * 1024**3:
        raise ValueError('native preparation requires at least 20 GiB free')


def unpack_span(data, start, rows, destination):
    if not rows or len(rows) > MAX_FILES:
        raise ValueError('native payload file count exceeds budget')
    if any(row['uncompressed'] > MAX_SINGLE_BYTES for row in rows) or sum(row['uncompressed'] for row in rows) > MAX_TOTAL_BYTES:
        raise ValueError('native payload expansion exceeds budget')
    contents, folded = {}, set()
    for row in rows:
        name = row['name']
        if not name.startswith(PREFIX):
            raise ValueError('unexpected architecture payload')
        relative = name[len(PREFIX):]
        path = PurePosixPath(relative)
        mode = row.get('external_attributes', 0) >> 16
        if (not relative or path.is_absolute() or '..' in path.parts or '\\' in relative or '\0' in relative or
                len(relative) > 1024 or any(len(p) > 255 for p in path.parts) or
                mode & 0o170000 == 0o120000 or relative.casefold() in folded):
            raise ValueError('unsafe or duplicate native payload path')
        folded.add(relative.casefold())
        offset = row['offset'] - start
        if offset < 0 or offset + 30 > len(data):
            raise ValueError('native local header outside bounded span')
        fields = struct.unpack_from('<4s5H3I2H', data, offset)
        begin = offset + 30 + fields[9] + fields[10]
        end = begin + row['compressed']
        if (fields[0] != b'PK\x03\x04' or fields[3] != row['method'] or row['flags'] & 1 or
                row['method'] not in (0, 8) or end > len(data) or
                data[offset + 30:offset + 30 + fields[9]].decode('utf8') != name):
            raise ValueError('native local/central ZIP metadata mismatch')
        raw = data[begin:end]
        if row['method'] == 8:
            inflater = zlib.decompressobj(-15)
            value = inflater.decompress(raw, row['uncompressed'] + 1)
            if not inflater.eof or inflater.unconsumed_tail or inflater.unused_data:
                raise ValueError('incomplete or excessive native deflate stream')
        else:
            value = raw
        if len(value) != row['uncompressed'] or zlib.crc32(value) != row['crc']:
            raise ValueError('native payload CRC/size mismatch')
        contents[relative] = value
    # Verify the entire unit before publishing any extracted file.
    if 'PolicyLanguageServer' not in contents:
        raise ValueError('native apphost is absent')
    if destination.exists():
        raise ValueError('fresh native payload directory required')
    manifest = [[name, hashlib.sha256(contents[name]).hexdigest()] for name in sorted(contents)]
    for name, value in contents.items():
        target = destination / name
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_bytes(value)
        target.chmod(0o700 if name == 'PolicyLanguageServer' else 0o600)
    identity = (json.dumps(manifest, ensure_ascii=True, separators=(',', ':')) + '\n').encode()
    return dict(files=len(contents), bytes=sum(map(len, contents.values())),
                manifest_sha256=hashlib.sha256(identity).hexdigest(),
                apphost_sha256=hashlib.sha256(contents['PolicyLanguageServer']).hexdigest())


def prepare(cache):
    disk_floor()
    cache.mkdir(parents=True, exist_ok=True)
    destination = cache / 'payload'
    if destination.exists():
        raise ValueError('fresh native payload directory required')
    pin = json.loads((HERE / 'extension-source-r1.json').read_text())
    directory = pin['directory']
    raw = fetch_range(directory['offset'], directory['bytes'])
    if hashlib.sha256(raw).hexdigest() != directory['sha256']:
        raise ValueError('native archive directory identity mismatch')
    rows = [row for row in directory_rows(raw, directory['entries']) if row['name'].startswith(PREFIX)]
    rows.sort(key=lambda row: row['offset'])
    if len(rows) != 455:
        raise ValueError('native architecture entry population changed')
    last = rows[-1]
    header = struct.unpack('<4s5H3I2H', fetch_range(last['offset'], 30))
    start = rows[0]['offset']
    end = last['offset'] + 30 + header[9] + header[10] + last['compressed']
    if end - start > MAX_SPAN_BYTES:
        raise ValueError('native compressed span exceeds budget')
    parts = []
    for offset in range(start, end, LIMIT):
        disk_floor()
        parts.append(fetch_range(offset, min(LIMIT, end - offset)))
    try:
        receipt = unpack_span(b''.join(parts), start, rows, destination)
    except Exception:
        shutil.rmtree(destination, ignore_errors=True)
        raise
    receipt.update(version=pin['version'], archive_directory_sha256=directory['sha256'],
                   kind='native preparation only; not executed', native_executed=False)
    (cache / 'preparation-receipt.json').write_text(json.dumps(receipt, indent=2) + '\n')
    return receipt


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('-cache', type=Path, required=True)
    args = parser.parse_args()
    if platform.system() != 'Darwin' or platform.machine() != 'arm64':
        raise SystemExit('this protocol selects macOS ARM64 only')
    print(json.dumps(prepare(args.cache.resolve()), sort_keys=True))
