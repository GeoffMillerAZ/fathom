"""Build a pinned private nginx instrument; no install, daemon or Docker."""
import argparse
import hashlib
import json
import os
import subprocess
import tarfile
import urllib.request
from pathlib import Path

HERE = Path(__file__).resolve().parent

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
            archive.write_bytes(urllib.request.urlopen(item['url'], timeout=60).read())
        if hashlib.sha256(archive.read_bytes()).hexdigest() != item['sha256']:
            raise SystemExit('source archive checksum mismatch')
        if not (cache / item['name']).exists():
            with tarfile.open(archive, 'r:gz') as tar:
                for member in tar.getmembers():
                    dest = (cache / member.name).resolve()
                    if cache not in dest.parents or member.isdev():
                        raise SystemExit('invalid source archive path')
                    if member.issym() or member.islnk():
                        link = ((dest.parent if member.issym() else cache) / member.linkname).resolve()
                        if cache not in link.parents:
                            raise SystemExit('invalid source archive link')
                tar.extractall(cache)
    versions = {item['name'].split('-')[0]: item['name'] for item in protocol['build_inputs']}
    source = cache / versions['nginx']
    features = ['--with-threads', '--with-pcre-jit', '--with-http_ssl_module', '--with-http_v2_module',
                '--with-http_v3_module', '--with-http_realip_module', '--with-http_addition_module',
                '--with-http_sub_module', '--with-http_dav_module', '--with-http_flv_module',
                '--with-http_mp4_module', '--with-http_gunzip_module', '--with-http_gzip_static_module',
                '--with-http_auth_request_module', '--with-http_random_index_module',
                '--with-http_secure_link_module', '--with-http_slice_module', '--with-http_stub_status_module',
                '--with-mail', '--with-mail_ssl_module', '--with-stream', '--with-stream_ssl_module',
                '--with-stream_realip_module', '--with-stream_ssl_preread_module']
    command = ['./configure', '--prefix=' + str(cache / 'prefix'), '--with-pcre=' + str(cache / versions['pcre2']),
               '--with-openssl=' + str(cache / versions['openssl']), *features]
    subprocess.run(command, cwd=source, check=True)
    subprocess.run(['make', '-j2'], cwd=source, check=True)
    binary = source / 'objs/nginx'
    run = subprocess.run([str(binary), '-v'], capture_output=True, text=True, check=True)
    version = (run.stdout + run.stderr).strip()
    if version != 'nginx version: ' + protocol['version']:
        raise SystemExit('native version mismatch')
    receipt = {'version': protocol['version'], 'binary_sha256': hashlib.sha256(binary.read_bytes()).hexdigest(),
               'build_inputs': protocol['build_inputs'], 'configure_features': features,
               'backend_requests_executed': False, 'server_started': False}
    (cache / 'build-receipt.json').write_text(json.dumps(receipt, indent=2) + '\n')
    print(protocol['version'], receipt['binary_sha256'])
