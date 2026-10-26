#!/usr/bin/env python3
"""Matching portable-Ruby OpenSSL SDK, installed only inside the tools root."""
import argparse
import io
import json
import os
from pathlib import Path
import subprocess
import sys
import tarfile

from bootstrap import download, ready


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--tools', type=Path, required=True)
    args = ap.parse_args()
    if sys.version_info < (3,13) or os.getpriority(os.PRIO_PROCESS,0) < 10:
        raise SystemExit('requires Python 3.13+ and nice10')
    base = args.tools.resolve()
    prefix = base/'sdk/openssl'
    source = base/'build-sources/openssl-3.4.1'
    if not source.exists():
        source.parent.mkdir(exist_ok=True)
        data = download('https://github.com/openssl/openssl/releases/download/openssl-3.4.1/openssl-3.4.1.tar.gz',
            '002a2d6b30b58bf4bea46c43bdd96365aaf8daa6c428782aa4feee06da197df3',
            base/'downloads/openssl-3.4.1.tar.gz')
        with tarfile.open(fileobj=io.BytesIO(data), mode='r:gz') as archive:
            archive.extractall(source.parent, filter='data')
    commands = [('./Configure','darwin64-arm64-cc','--prefix='+str(prefix),'--openssldir='+str(prefix/'ssl'),
                 '--libdir=lib','no-tests','no-shared'),('make','-j2','build_sw'),('make','install_sw')]
    for number, cmd in enumerate(commands):
        ready()
        with (base/'logs'/('openssl-sdk-'+str(number)+'.log')).open('w') as log:
            subprocess.run(cmd,cwd=source,stdout=log,stderr=log,check=True,timeout=1200)
    (base/'openssl-sdk.json').write_text(json.dumps({'version':'3.4.1','source_sha256':
        '002a2d6b30b58bf4bea46c43bdd96365aaf8daa6c428782aa4feee06da197df3',
        'contract':'same OpenSSL version as Homebrew portable-ruby tag3.3.8; static local SDK; no system install'})+'\n')


if __name__ == '__main__':
    main()
