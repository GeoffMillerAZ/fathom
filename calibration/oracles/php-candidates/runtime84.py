#!/usr/bin/env python3
"""Install a pinned PHP 8.4 runtime, sharing only existing isolated dependencies."""
import argparse
import hashlib
import io
import json
from pathlib import Path
import tarfile
import sys

from fetch import download


def main():
    if sys.version_info < (3, 13):
        raise SystemExit('Python 3.13+ required before creating any output')
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--tools', required=True, type=Path)
    ap.add_argument('--shared', required=True, type=Path)
    ap.add_argument('--archive', type=Path, help='optional previously downloaded archive, checked identically')
    args = ap.parse_args()
    base, shared = args.tools.resolve(), args.shared.resolve()
    base.mkdir(parents=True, exist_ok=False)
    content = args.archive.read_bytes() if args.archive else download(
        'https://dl.static-php.dev/static-php-cli/common/php-8.4.23-cli-macos-aarch64.tar.gz')
    digest = hashlib.sha256(content).hexdigest()
    if digest != 'bba286e442796dbd420d778a016e2817b31d5036d11b3ba316d19a60de912cdc':
        raise ValueError('PHP 8.4.23 runtime checksum mismatch')
    (base/'runtime').mkdir()
    with tarfile.open(fileobj=io.BytesIO(content), mode='r:gz') as archive:
        archive.extractall(base/'runtime', filter='data')
    for name in ['guzzle', 'monolog', 'callsite-fork']:
        (base/name).symlink_to(shared/name, target_is_directory=True)
    (base/'sdk').mkdir()
    (base/'sdk/autoconf').symlink_to(shared/'sdk/autoconf', target_is_directory=True)
    (base/'runtime-receipt.json').write_text(json.dumps({'php': '8.4.23', 'archive_sha256': digest,
                                                       'shared_subjects_and_dependency_versions': True}, indent=2)+'\n')


if __name__ == '__main__':
    main()
