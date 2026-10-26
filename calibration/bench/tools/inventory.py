"""Read pinned tracked blobs and history, without fetching metadata or executing subject code."""
import argparse
import datetime
import hashlib
import io
import json
from pathlib import Path
import re
import subprocess
import tarfile

parser = argparse.ArgumentParser(__doc__)
parser.add_argument('source', type=Path)
parser.add_argument('language', choices=['Go', 'Python', 'TypeScript'])
a = parser.parse_args()
def git(*args):
    return subprocess.check_output(['git', '-C', str(a.source), *args])
sha = git('rev-parse', 'HEAD').decode().strip()
extensions = {'Go': {'.go'}, 'Python': {'.py'}, 'TypeScript': {'.ts', '.tsx', '.mts', '.cts'}}[a.language]
files = {}
with tarfile.open(fileobj=io.BytesIO(git('archive', sha))) as archive:
    for member in archive.getmembers():
        if member.isfile():
            files[member.name] = archive.extractfile(member).read()
tracked_primary = {name for name in git('ls-tree', '-r', '--name-only', sha).decode().splitlines() if Path(name).suffix in extensions}
assert tracked_primary == {name for name in files if Path(name).suffix in extensions}, 'archive excludes tracked primary files; use direct blob counting for this subject'
lines = {name: data.count(b'\n') + int(bool(data) and not data.endswith(b'\n'))
         for name, data in files.items() if Path(name).suffix in extensions}
licences = []
for name, data in files.items():
    if '/' not in name and re.match(r'licen[cs]e', name, re.I):
        text = data.decode(errors='replace')
        spdx = None
        if 'Permission is hereby granted, free of charge' in text:
            spdx = 'MIT'
        elif 'Apache License' in text and 'Version 2.0' in text:
            spdx = 'Apache-2.0'
        elif 'Redistribution and use in source and binary forms' in text:
            spdx = 'BSD-3-Clause' if re.search('neither the name', text, re.I) else 'BSD-2-Clause'
        licences.append({'path': name, 'spdx': spdx, 'sha256': hashlib.sha256(data).hexdigest()})
def istest(path):
    return bool(re.search(r'(^|/)(tests?|specs?|__tests__)(/|$)|(^|/)(test_[^/]*|[^/]*_test)\.py$|_test\.go$|\.(test|spec)\.[cm]?[jt]sx?$', path))
history = []
for block in git('log', sha, '--since=2024-10-25T00:00:00Z', '--until=2026-10-26T00:00:00Z',
                 '--no-merges', '--format=@@%H%x09%cI%x09%s', '--name-only').decode(errors='replace').split('@@')[1:]:
    entries = block.strip().splitlines()
    commit, date, subject = entries[0].split('\t', 2)
    tests = [p for p in entries[1:] if istest(p)]
    other = [p for p in entries[1:] if p and not istest(p)]
    if tests and other:
        history.append({'sha': commit, 'date': datetime.datetime.fromisoformat(date.replace("Z", "+00:00")).astimezone(datetime.timezone.utc).isoformat().replace('+00:00', 'Z'),
                        'fix_message': bool(re.search(r'\b(fix(es|ed)?|bugs?|regressions?)\b', subject, re.I)),
                        'tests': tests, 'primary_source': [p for p in other if Path(p).suffix in extensions]})
result = {'sha': sha, 'language': a.language, 'primary_lines': sum(lines.values()), 'files': len(lines),
          'licences': licences, 'full_history': git('rev-parse', '--is-shallow-repository').decode().strip() == 'false',
          'c5_window': ['2024-10-25T00:00:00Z', '2026-10-26T00:00:00Z'],
          'co_changes': len(history), 'fix_message_co_changes': sum(x['fix_message'] for x in history),
          'primary_co_changes': sum(bool(x['primary_source']) for x in history),
          'primary_fix_co_changes': sum(bool(x['primary_source']) and x['fix_message'] for x in history), 'replayed': 0,
          'locks': {p: hashlib.sha256(b).hexdigest() for p, b in files.items() if p in ['yarn.lock', 'pnpm-lock.yaml', 'package-lock.json', 'uv.lock', 'poetry.lock', 'go.sum', 'go.mod']},
          'history': history}
print(json.dumps(result, indent=2))
