#!/usr/bin/env python3
"""Fetch immutable original public policy trees into a fresh workspace directory."""
import argparse
import io
from pathlib import Path
import sys
import tarfile
import urllib.request

SUBJECTS = {
    'terraform-sentinel-policies': ('hashicorp/terraform-sentinel-policies', 'b0bd9902f1db35f922f7d33ff10c35e88c380f0e'),
    'policy-library-gcp-databases-terraform': ('hashicorp/policy-library-gcp-databases-terraform', '6f779b6ddc3329c967c2bf7a22807f3a16fee9ac'),
}


def main():
    if sys.version_info < (3, 13):
        raise SystemExit('Python 3.13+ is required for safe archive extraction')
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--out', required=True, type=Path)
    args = ap.parse_args()
    args.out.mkdir(parents=True, exist_ok=False)
    for name, (repo, sha) in SUBJECTS.items():
        content = urllib.request.urlopen('https://codeload.github.com/'+repo+'/tar.gz/'+sha).read()
        target = args.out / name
        target.mkdir()
        with tarfile.open(fileobj=io.BytesIO(content), mode='r:gz') as archive:
            prefix = archive.getmembers()[0].name.split('/')[0]+'/'
            for member in archive.getmembers():
                if not member.name.startswith(prefix):
                    continue
                member.name = member.name[len(prefix):]
                if member.name:
                    archive.extract(member, target, filter='data')
        print(repo, sha, flush=True)


if __name__ == '__main__':
    main()
