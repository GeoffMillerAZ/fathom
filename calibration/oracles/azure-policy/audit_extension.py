"""Inspect pinned vendor source/metadata through bounded HTTP ZIP ranges.

Does not install the extension, extract native binaries, contact Azure, or
execute any vendor code. This is a candidate audit, not an oracle reading.
"""
import argparse
import hashlib
import json
import struct
import urllib.request
import zlib
from pathlib import Path

HERE = Path(__file__).resolve().parent
URL = ('https://AzurePolicy.gallerycdn.vsassets.io/extensions/azurepolicy/'
       'azurepolicyextension/0.2.6/1769652999395/Microsoft.VisualStudio.Services.VSIXPackage')
TOTAL = 310511962
LIMIT = 4 * 1024**2


def fetch_range(start, size, total=TOTAL):
    if start < 0 or size < 1 or size > LIMIT or start + size > total:
        raise ValueError('invalid or excessive archive range')
    request = urllib.request.Request(URL, headers={'Range': 'bytes={}-{}'.format(start, start + size - 1)})
    with urllib.request.urlopen(request, timeout=30) as response:
        expected = 'bytes {}-{}/{}'.format(start, start + size - 1, total)
        if response.status != 206 or response.headers.get('Content-Range') != expected:
            raise ValueError('server did not honor the bounded range')
        data = response.read(size + 1)
    if len(data) != size:
        raise ValueError('archive range size mismatch')
    return data


def directory_rows(data, expected):
    rows, offset = [], 0
    while offset < len(data):
        if len(data) - offset < 46:
            raise ValueError('truncated ZIP directory')
        fields = struct.unpack_from('<4s6H3I5H2I', data, offset)
        end = offset + 46 + fields[10] + fields[11] + fields[12]
        if fields[0] != b'PK\x01\x02' or end > len(data):
            raise ValueError('invalid ZIP directory record')
        name = data[offset + 46:offset + 46 + fields[10]].decode('utf8')
        rows.append(dict(name=name, flags=fields[3], method=fields[4], crc=fields[7],
                         compressed=fields[8], uncompressed=fields[9], offset=fields[16],
                         external_attributes=fields[15]))
        offset = end
    if len(rows) != expected or len({row['name'] for row in rows}) != len(rows):
        raise ValueError('unexpected ZIP entry population')
    return rows


def extract_metadata(row):
    header = struct.unpack('<4s5H3I2H', fetch_range(row['offset'], 30))
    if header[0] != b'PK\x03\x04' or header[3] != row['method'] or row['flags'] & 1:
        raise ValueError('unsupported ZIP local header')
    if row['uncompressed'] > LIMIT or row['method'] not in (0, 8):
        raise ValueError('metadata exceeds inspection budget')
    name = fetch_range(row['offset'] + 30, header[9]).decode('utf8')
    if name != row['name']:
        raise ValueError('local and central ZIP names disagree')
    raw = fetch_range(row['offset'] + 30 + header[9] + header[10], row['compressed'])
    if row['method'] == 8:
        inflater = zlib.decompressobj(-15)
        data = inflater.decompress(raw, row['uncompressed'] + 1)
        if not inflater.eof or inflater.unconsumed_tail or inflater.unused_data:
            raise ValueError('incomplete or excessive metadata deflate stream')
    else:
        data = raw
    if len(data) != row['uncompressed'] or zlib.crc32(data) != row['crc']:
        raise ValueError('metadata size/CRC mismatch')
    return data


def inspect_claims(bundle, names):
    return dict(arm64_payload_present='extension/out/artifacts/osx-arm64/PolicyLanguageServer' in names,
                loopback_evaluation_route_present='http://localhost:6490/evaluatePolicy' in bundle,
                cache_command_present='SET_CACHE_FOLDER_PATH' in bundle,
                native_executed=False, native_population_measured=False)


def audit():
    pin = json.loads((HERE / 'extension-source-r1.json').read_text())
    directory = pin['directory']
    raw = fetch_range(directory['offset'], directory['bytes'])
    if hashlib.sha256(raw).hexdigest() != directory['sha256']:
        raise ValueError('pinned directory identity mismatch')
    rows = directory_rows(raw, directory['entries'])
    sources = {}
    for name, digest in pin['metadata_sha256'].items():
        row = next(row for row in rows if row['name'] == name)
        data = extract_metadata(row)
        if hashlib.sha256(data).hexdigest() != digest:
            raise ValueError('pinned metadata identity mismatch')
        sources[name] = data.decode('utf8')
    bundle = sources['extension/dist/extension.bundle.js']
    claims = inspect_claims(bundle, [row['name'] for row in rows])
    if not all(claims[key] for key in ('arm64_payload_present', 'loopback_evaluation_route_present', 'cache_command_present')):
        raise ValueError('previously inspected native route changed')
    return dict(version=pin['version'], archive_directory_sha256=directory['sha256'],
                metadata_sha256=pin['metadata_sha256'], observations=claims,
                kind='source-only candidate audit; no reading or tier upgrade')


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('-out', type=Path, required=True)
    args = parser.parse_args()
    result = audit()
    args.out.parent.mkdir(parents=True, exist_ok=True)
    args.out.write_text(json.dumps(result, indent=2) + '\n')
    print(result['version'], result['kind'])
