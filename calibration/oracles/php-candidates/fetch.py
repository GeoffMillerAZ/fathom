#!/usr/bin/env python3
"""Fetch pinned PHP oracle candidates and original subjects into workspace only."""
import argparse
import hashlib
import io
import json
import os
from pathlib import Path
import subprocess
import tarfile
import time
import sys

SOURCES = {
    'callsite-fork': ('michalboryczko/scip-php', '40288605f28ea0111b42f81a8760ad5b79b8c3a0'),
    'php84-fork': ('devmdeniz/scip-php', '9af23514a898dd63a8123b02b567711fbef5dd99'),
    'mago-indexer': ('ArtemisMucaj/scip-php', '04b34f724c9464256891345153b5c2c0a6c23bc8'),
    'guzzle': ('guzzle/guzzle', '7b2f29fe81dc4da0ca0ea7d42107a0845946ea77'),
    'monolog': ('Seldaek/monolog', '10d85740180ecba7896c87e06a166e0c95a0e3b6'),
}


def ready():
    while os.getloadavg()[0] > 6:
        print('waiting for load <=6', flush=True)
        time.sleep(30)


def download(url):
    ready()
    return subprocess.run(['curl', '--fail', '--location', '--silent', '--show-error', '--retry', '2', url],
                          check=True, capture_output=True).stdout


def main():
    if sys.version_info < (3, 13):
        raise SystemExit('Python 3.13+ required before creating any output')
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--out', required=True, type=Path)
    args = ap.parse_args()
    args.out.mkdir(parents=True, exist_ok=True)
    receipt_path = args.out/'fetch-receipt.json'
    receipt = json.loads(receipt_path.read_text()) if receipt_path.exists() else {}

    def record(name, values):
        receipt[name] = values
        receipt_path.write_text(json.dumps(receipt, indent=2, sort_keys=True)+'\n')

    for name, (repo, sha) in SOURCES.items():
        if receipt.get(name, {}).get('mode') == 'complete-git-checkout':
            continue
        target = args.out/name
        target.mkdir(exist_ok=True)
        ready()
        subprocess.run(['git', 'init', '--quiet', str(target)], check=True)
        subprocess.run(['git', '-C', str(target), 'remote', 'add', 'origin', 'https://github.com/'+repo], check=True)
        subprocess.run(['git', '-C', str(target), 'fetch', '--quiet', '--depth=1', 'origin', sha], check=True)
        subprocess.run(['git', '-C', str(target), 'checkout', '--quiet', '--detach', '--force', 'FETCH_HEAD'], check=True)
        tree = subprocess.check_output(['git', '-C', str(target), 'rev-parse', 'HEAD^{tree}'], text=True).strip()
        record(name, {'repository': repo, 'revision': sha, 'tree': tree, 'mode': 'complete-git-checkout'})
        print('fetched', name, flush=True)

    if 'php' not in receipt:
        content = download('https://dl.static-php.dev/static-php-cli/common/php-8.5.8-cli-macos-aarch64.tar.gz')
        if hashlib.sha256(content).hexdigest() != 'cba9bd8b38b51ef2c7cebf02af689e4481bbd07f65edc0b63d3a43b04e0c9db7':
            raise ValueError('PHP 8.5.8 runtime checksum mismatch')
        target = args.out/'runtime'
        target.mkdir(exist_ok=False)
        with tarfile.open(fileobj=io.BytesIO(content), mode='r:gz') as archive:
            archive.extractall(target, filter='data')
        record('php', {'version': '8.5.8', 'archive_sha256': hashlib.sha256(content).hexdigest()})
    if 'composer' not in receipt:
        content = download('https://getcomposer.org/download/2.10.3/composer.phar')
        expected = '7a2d379d5b8ffdaa028580ef26494c36d2feef4b178d3dd1473a4dbc5e17c8d6'
        if hashlib.sha256(content).hexdigest() != expected:
            raise ValueError('Composer checksum mismatch')
        (args.out/'composer.phar').write_bytes(content)
        record('composer', {'version': '2.10.3', 'sha256': expected})
    if 'mago_binary' not in receipt:
        content = download('https://github.com/ArtemisMucaj/scip-php/releases/download/v0.3.0/scip-php-macos-aarch64')
        expected = '27a4788b639a358235d77863231e715576ba91975954e539fdaf7bb5a2e78640'
        if hashlib.sha256(content).hexdigest() != expected:
            raise ValueError('released SCIP producer checksum mismatch')
        binary = args.out/'mago-scip-php'
        binary.write_bytes(content)
        binary.chmod(0o755)
        record('mago_binary', {'version': '0.3.0', 'sha256': expected})


if __name__ == '__main__':
    main()
