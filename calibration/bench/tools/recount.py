"""Apply D24 units and D26 product-only callers to resolved static receipts."""
import argparse
import json
from pathlib import Path, PurePosixPath
import re
import subprocess

# Test/support directories are never product units, irrespective of language.
NONPRODUCT = re.compile(
    r'^(?:tests?|testing|specs?|bench(?:marks?)?|examples?|samples?|docs?|'
    r'documentation|tools?|scripts?|testdata|fixtures?|mocks|typing[-_]tests?|'
    r'type[-_]tests?|performance|perf(?:[-_]testing)?|performance[-_]tests?|'
    r'e2e|integration[-_]tests?|website|www|site|codegen|storybook)$', re.I)


def product_path(path):
    parts = PurePosixPath(path).parts
    if any(part.startswith('.') or NONPRODUCT.fullmatch(part.strip('_')) for part in parts[:-1]):
        return False
    name = parts[-1]
    return not bool(re.search(
        r'(^test_|_test\.py$|_test\.go$|(?:^|[._-])(?:test|spec|bench|benchmark)(?:-d)?\.[cm]?[jt]sx?$|^conftest\.py$)',
        name, re.I))


def recount(language, subject, rows, policy='D26'):
    paths = subprocess.check_output(['git', '-C', str(subject), 'ls-files', '-z'], text=True).split('\0')[:-1]
    if language == 'Go':
        module = re.search(r'^module\s+(\S+)', (subject / 'go.mod').read_text(), re.M)[1]
    if language == 'Python':
        modules = {p.removeprefix('src/').removesuffix('.py').removesuffix('/__init__').replace('/', '.'): p
                   for p in paths if p.endswith('.py')}
    result = []
    for row in rows or []:
        symbol = row['symbol']
        if language == 'Go':
            package = re.match(r'(.*/[^.]+)\.', symbol)[1].removesuffix('_test')
            if not (package == module or package.startswith(module + '/')):
                raise ValueError('symbol outside subject module: ' + symbol)
            defining = package[len(module):].lstrip('/') or '.'
        elif language == 'Python':
            defining = str(PurePosixPath(modules[symbol.rsplit('.', 1)[0]]).parent)
        else:
            defining = str(PurePosixPath(symbol.split('#')[0]).parent)
        sites = set(row['sites'])
        outside = {site for site in sites if str(PurePosixPath(site.rsplit(':', 2)[0]).parent) != defining}
        product = {site for site in outside if product_path(site.rsplit(':', 2)[0])}
        selected = sorted(product if policy == 'D26' else outside)
        directories = sorted({str(PurePosixPath(site.rsplit(':', 2)[0]).parent) for site in selected})
        units = [(module + '/' + p).rstrip('/') if p != '.' else module for p in directories] if language == 'Go' else directories
        result.append({'symbol': symbol, 'defining_unit': package if language == 'Go' else defining,
                       'calls': len(selected), 'units': units, 'sites': selected,
                       'qualifies': len(selected) >= 5 and len(units) >= 3,
                       **({'outside_sites_before_product_filter': len(outside),
                           'excluded_nonproduct_sites': sorted(outside - product)} if policy == 'D26' else {})})
    result.sort(key=lambda r: (-len(r['units']), -r['calls'], -r.get('outside_sites_before_product_filter', 0), r['symbol']))
    return result


if __name__ == '__main__':
    parser = argparse.ArgumentParser(__doc__)
    parser.add_argument('language', choices=['Go', 'Python', 'TypeScript'])
    parser.add_argument('subject', type=Path)
    parser.add_argument('rows', type=Path)
    parser.add_argument('--policy', choices=['D24', 'D26'], default='D26')
    args = parser.parse_args()
    print(json.dumps(recount(args.language, args.subject, json.loads(args.rows.read_text()), args.policy), indent=2))
