"""Prepare a pinned vendor architecture payload, without executing it."""
import argparse
import hashlib
import json
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


def disk_floor(cache, min_free_gib=20):
    if shutil.disk_usage(cache).free < min_free_gib * 1024**3:
        raise ValueError('native preparation below configured disk floor')


def native_ranges(rows, central_offset, prefix=PREFIX):
    ordered = sorted(rows, key=lambda row: row['offset'])
    if len({row['offset'] for row in ordered}) != len(ordered):
        raise ValueError('duplicate local ZIP offsets')
    groups = []
    for index, row in enumerate(ordered):
        if not row['name'].startswith(prefix):
            continue
        end = ordered[index + 1]['offset'] if index + 1 < len(ordered) else central_offset
        overhead = end - row['offset'] - row['compressed']
        if overhead < 30 or overhead > 30 + len(row['name'].encode()) + 65535 + 24:
            raise ValueError('native ZIP entry boundary is invalid')
        if groups and groups[-1]['end'] == row['offset']:
            groups[-1]['end'] = end
            groups[-1]['rows'].append(row)
        else:
            groups.append(dict(start=row['offset'], end=end, rows=[row]))
    if sum(group['end'] - group['start'] for group in groups) > MAX_SPAN_BYTES:
        raise ValueError('native compressed span exceeds budget')
    return groups


def unpack_span(data, start, rows, destination, prefix=PREFIX):
    if not rows or len(rows) > MAX_FILES:
        raise ValueError('native payload file count exceeds budget')
    if any(row['uncompressed'] > MAX_SINGLE_BYTES for row in rows) or sum(row['uncompressed'] for row in rows) > MAX_TOTAL_BYTES:
        raise ValueError('native payload expansion exceeds budget')
    contents, folded = {}, set()
    for row in rows:
        name = row['name']
        if not name.startswith(prefix):
            raise ValueError('unexpected architecture payload')
        relative = name[len(prefix):]
        path = PurePosixPath(relative)
        mode = row.get('external_attributes', 0) >> 16
        if (not relative or path.is_absolute() or '..' in path.parts or '\\' in relative or '\0' in relative or
                len(relative) > 1024 or any(len(p) > 255 for p in path.parts) or
                mode & 0o170000 == 0o120000 or relative.casefold() in folded):
            raise ValueError('unsafe or duplicate native payload path')
        folded.add(relative.casefold())
        local = data.get(row['offset']) if isinstance(data, dict) else data
        offset = 0 if isinstance(data, dict) else row['offset'] - start
        if local is None or offset < 0 or offset + 30 > len(local):
            raise ValueError('native local header outside bounded span')
        fields = struct.unpack_from('<4s5H3I2H', local, offset)
        begin = offset + 30 + fields[9] + fields[10]
        end = begin + row['compressed']
        if (fields[0] != b'PK\x03\x04' or fields[3] != row['method'] or row['flags'] & 1 or
                row['method'] not in (0, 8) or end > len(local) or
                local[offset + 30:offset + 30 + fields[9]].decode('utf8') != name):
            raise ValueError('native local/central ZIP metadata mismatch')
        raw = local[begin:end]
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


def prepare(cache, rid="osx-arm64", expected=None, min_free_gib=20):
    if rid not in ("osx-arm64", "linux-x64"):
        raise ValueError("unsupported pinned runtime")
    prefix = "extension/out/artifacts/" + rid + "/"
    cache.mkdir(parents=True, exist_ok=True)
    disk_floor(cache, min_free_gib)
    destination = cache / 'payload'
    if destination.exists():
        raise ValueError('fresh native payload directory required')
    pin = json.loads((HERE / 'extension-source-r1.json').read_text())
    directory = pin['directory']
    raw = fetch_range(directory['offset'], directory['bytes'])
    if hashlib.sha256(raw).hexdigest() != directory['sha256']:
        raise ValueError('native archive directory identity mismatch')
    all_rows = directory_rows(raw, directory['entries'])
    rows = [row for row in all_rows if row['name'].startswith(prefix)]
    rows.sort(key=lambda row: row['offset'])
    if len(rows) != 455:
        raise ValueError('native architecture entry population changed')
    groups = native_ranges(all_rows, directory['offset'], prefix)
    entries = {}
    for group in groups:
        parts = []
        for offset in range(group['start'], group['end'], LIMIT):
            disk_floor(cache, min_free_gib)
            parts.append(fetch_range(offset, min(LIMIT, group['end'] - offset)))
        block = b''.join(parts)
        for index, row in enumerate(group['rows']):
            end = group['rows'][index + 1]['offset'] if index + 1 < len(group['rows']) else group['end']
            entries[row['offset']] = block[row['offset'] - group['start']:end - group['start']]
    try:
        receipt = unpack_span(entries, 0, rows, destination, prefix)
        if expected and any(receipt[key] != expected[key] for key in ("files", "bytes", "manifest_sha256", "apphost_sha256")):
            raise ValueError("native whole-payload SHA-256 identity mismatch")
    except Exception:
        shutil.rmtree(destination, ignore_errors=True)
        raise
    receipt.update(rid=rid, version=pin['version'], archive_directory_sha256=directory['sha256'],
                   fetched_native_ranges=len(groups), fetched_native_bytes=sum(g['end'] - g['start'] for g in groups),
                   kind='native preparation only; not executed', native_executed=False)
    (cache / 'preparation-receipt.json').write_text(json.dumps(receipt, indent=2) + '\n')
    return receipt


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('-cache', type=Path, required=True)
    parser.add_argument('-rid', choices=['osx-arm64', 'linux-x64'], default='osx-arm64')
    args = parser.parse_args()
    print(json.dumps(prepare(args.cache.resolve(), args.rid), sort_keys=True))
