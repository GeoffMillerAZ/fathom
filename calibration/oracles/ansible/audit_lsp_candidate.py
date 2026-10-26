"""Verify and inspect the published vendor LSP; never execute it or an AI feature."""
import argparse
import hashlib
import io
import json
import tarfile
import urllib.request
from pathlib import Path

VERSION = '26.6.0'
URL = 'https://registry.npmjs.org/@ansible/ansible-language-server/-/ansible-language-server-26.6.0.tgz'
INTEGRITY = '0cf4e6724ae73427fc29d792bbda44fe68e7a2f48bdbc2279d08434b696cb83acb23ca1b6a9331014d0a2c36270bef9a4e308af1b6618644791c587794375dab'

if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('-archive', type=Path, required=True)
    parser.add_argument('-download', action='store_true')
    parser.add_argument('-out', type=Path, required=True)
    args = parser.parse_args()
    if args.download and not args.archive.exists():
        args.archive.parent.mkdir(parents=True, exist_ok=True)
        with urllib.request.urlopen(URL, timeout=60) as response:
            args.archive.write_bytes(response.read())
    data = args.archive.read_bytes()
    if hashlib.sha512(data).hexdigest() != INTEGRITY:
        raise SystemExit('archive checksum mismatch')
    with tarfile.open(fileobj=io.BytesIO(data), mode='r:gz') as archive:
        package = json.load(archive.extractfile('package/package.json'))
        server_bytes = archive.extractfile('package/dist/server.js').read()
        provider_bytes = archive.extractfile('package/lib/providers/definitionProvider.js').read()
    if package['version'] != VERSION or package['license'] != 'MIT':
        raise SystemExit('package version/licence mismatch')
    server = server_bytes.decode()
    provider = provider_bytes.decode()
    capabilities = {key: server.count(key + ':') for key in ('definitionProvider', 'referencesProvider', 'documentSymbolProvider')}
    if capabilities != dict(definitionProvider=1, referencesProvider=0, documentSymbolProvider=0):
        raise SystemExit('capability contract changed; investigate')
    if not all(token in provider for token in ('isTaskParam(path)', 'docsLibrary.findModule(', 'module.source', 'module.sourceLineRange')):
        raise SystemExit('definition implementation changed; investigate')
    result = {'candidate': package['name'], 'version': VERSION, 'licence': package['license'],
              'archive_url': URL, 'archive_sha512': INTEGRITY,
              'server_sha256': hashlib.sha256(server_bytes).hexdigest(),
              'definition_provider_sha256': hashlib.sha256(provider_bytes).hexdigest(),
              'capability_field_occurrences': capabilities,
              'result': 'Source inspected, not installed or run: module-key definitions target module implementation source; no symbol/reference provider. Does not attest the measured play/task/variable declarations or handler-reference population.'}
    args.out.write_text(json.dumps(result, indent=2) + '\n')
    print(json.dumps(capabilities, sort_keys=True))
