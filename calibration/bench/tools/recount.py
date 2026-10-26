"""Apply D24 units to resolved static call-site receipts; never query fathom."""
import argparse
import json
from pathlib import Path, PurePosixPath
import re
import subprocess

parser = argparse.ArgumentParser(__doc__)
parser.add_argument('language', choices=['Go', 'Python', 'TypeScript'])
parser.add_argument('subject', type=Path)
parser.add_argument('rows', type=Path)
a = parser.parse_args()
paths = subprocess.check_output(['git', '-C', str(a.subject), 'ls-files', '-z'], text=True).split('\0')[:-1]
if a.language == 'Go':
    module = re.search(r'^module\s+(\S+)', (a.subject / 'go.mod').read_text(), re.M)[1]
if a.language == 'Python':
    modules = {p.removeprefix('src/').removesuffix('.py').removesuffix('/__init__').replace('/', '.'): p
               for p in paths if p.endswith('.py')}
result = []
for row in json.loads(a.rows.read_text()) or []:
    symbol = row['symbol']
    if a.language == 'Go':
        package = re.match(r'(.*/[^.]+)\.', symbol)[1].removesuffix('_test')
        if not (package == module or package.startswith(module + '/')):
            raise ValueError('symbol outside subject module: ' + symbol)
        defining = package[len(module):].lstrip('/') or '.'
    elif a.language == 'Python':
        defining = str(PurePosixPath(modules[symbol.rsplit('.', 1)[0]]).parent)
    else:
        defining = str(PurePosixPath(symbol.split('#')[0]).parent)
    sites = sorted({site for site in row['sites']
                    if str(PurePosixPath(site.rsplit(':', 2)[0]).parent) != defining})
    directories = sorted({str(PurePosixPath(site.rsplit(':', 2)[0]).parent) for site in sites})
    units = [(module + '/' + p).rstrip('/') if p != '.' else module for p in directories] if a.language == 'Go' else directories
    result.append({'symbol': symbol, 'defining_unit': package if a.language == 'Go' else defining,
                   'calls': len(sites), 'units': units, 'sites': sites,
                   'qualifies': len(sites) >= 5 and len(units) >= 3})
result.sort(key=lambda r: (-len(r['units']), -r['calls'], r['symbol']))
print(json.dumps(result, indent=2))
