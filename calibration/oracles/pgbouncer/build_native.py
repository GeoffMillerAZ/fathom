"""Build pinned native PgBouncer with a private static libevent, without TLS.

Run under the lane's serial nice/load guard. All outputs stay below -cache.
"""
import argparse
import hashlib
import json
import os
import subprocess
import tarfile
import urllib.request
from pathlib import Path

HERE = Path(__file__).resolve().parent


def run(command, cwd, env=None):
    subprocess.run([str(x) for x in command], cwd=cwd, env=env, check=True)


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('-cache', type=Path, required=True)
    args = parser.parse_args()
    cache = args.cache.resolve()
    cache.mkdir(parents=True, exist_ok=True)
    protocol = json.loads((HERE / 'native-protocol-r1.json').read_text())
    for item in protocol['build_inputs']:
        archive = cache / (item['name'] + '.tar.gz')
        if not archive.exists():
            with urllib.request.urlopen(item['url'], timeout=60) as response:
                archive.write_bytes(response.read())
        if hashlib.sha256(archive.read_bytes()).hexdigest() != item['sha256']:
            raise SystemExit('archive checksum mismatch')
        source = cache / item['name']
        if not source.exists():
            with tarfile.open(archive, 'r:gz') as tar:
                for member in tar.getmembers():
                    if member.name.startswith('/') or '..' in Path(member.name).parts:
                        raise SystemExit('invalid archive path')
                tar.extractall(cache)
    event = cache / 'libevent-2.1.12-stable'
    prefix = cache / 'libevent-prefix'
    if not (prefix / 'lib/libevent.a').exists():
        run(['./configure', '--prefix=' + str(prefix), '--disable-openssl', '--disable-shared',
             '--disable-samples', '--disable-libevent-regress'], event)
        run(['make', '-j2'], event)
        run(['make', 'install'], event)
    source = cache / 'pgbouncer-1.26.1'
    for name, expected in protocol['native_source_sha256'].items():
        if hashlib.sha256((source / name).read_bytes()).hexdigest() != expected:
            raise SystemExit('release differs from the canonical native source: ' + name)
    env = dict(os.environ, LIBEVENT_CFLAGS='-I' + str(prefix / 'include'),
               LIBEVENT_LIBS=str(prefix / 'lib/libevent.a'), CFLAGS='-O2')
    run(['./configure', '--without-openssl', '--without-cares'], source, env)
    run(['make', '-j2', 'pgbouncer'], source, env)
    version_output = subprocess.check_output([str(source / 'pgbouncer'), '-V'], text=True).strip()
    version = version_output.splitlines()[0]
    if version != protocol['version']:
        raise SystemExit('wrong native version: ' + version)
    receipt = {'version': version, 'version_output': version_output, 'binary_sha256': hashlib.sha256((source / 'pgbouncer').read_bytes()).hexdigest(),
               'native_source_sha256': protocol['native_source_sha256'], 'build_inputs': protocol['build_inputs'],
               'tls': False, 'libevent': '2.1.12-stable static'}
    (cache / 'build-receipt.json').write_text(json.dumps(receipt, indent=2) + '\n')
    print(version, receipt['binary_sha256'])
