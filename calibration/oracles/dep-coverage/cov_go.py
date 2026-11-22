"""Offline Go native adapter for COV-M3; no product reader is imported.

Raw command streams stay private. Truth requires native membership and a
named-file listing, never a package import union attributed to each member.
"""
import argparse
from collections import Counter
import hashlib
import json
import os
from pathlib import Path
import subprocess


def encoded(value):
    return json.dumps(value, sort_keys=True, separators=(',', ':')).encode()


def digest(value):
    return hashlib.sha256(encoded(value)).hexdigest()


def stream(data):
    decoder = json.JSONDecoder()
    text = data.decode()
    rows = []
    while text.strip():
        text = text.lstrip()
        row, end = decoder.raw_decode(text)
        rows.append(row)
        text = text[end:]
    return rows


def environment(cgo):
    env = {k: v for k, v in os.environ.items() if not k.startswith(('GO', 'CGO_'))}
    env.update(GOENV='off', GOFLAGS='-mod=readonly', GOTOOLCHAIN='local',
               GOPROXY='off', GOSUMDB='off', GOWORK='off', GOOS='darwin',
               GOARCH='arm64', CGO_ENABLED=cgo)
    for key in ('GOTMPDIR', 'GOMODCACHE', 'GOPATH', 'GOCACHE'):
        if key in os.environ:
            env[key] = os.environ[key]
    return env


def native(root, args, cgo, raw):
    proc = subprocess.run(['go', 'list', '-e', *args], cwd=root,
                          env=environment(cgo), capture_output=True, timeout=180)
    rows = stream(proc.stdout)
    raw.append({'args': args, 'stdout': proc.stdout.decode(),
                'stderr': proc.stderr.decode(), 'exit': proc.returncode})
    return rows


def relative(root, full):
    if not full:
        return None
    try:
        return Path(full).resolve().relative_to(root.resolve()).as_posix()
    except (ValueError, TypeError):
        return None


def edge(kind, raw, to, resolution='file'):
    return {'Kind': kind, 'Raw': raw, 'Line': 0, 'Resolution': resolution, 'To': to}


def capture(root, selected, cgo, tracked=None):
    root = root.resolve()
    tracked = set(tracked if tracked is not None else
                  (p.relative_to(root).as_posix() for p in root.rglob('*') if p.is_file()))
    modules = sorted(n for n in tracked if Path(n).name == 'go.mod')
    raw = []
    packages = []
    for mod in modules:
        packages += native(root / Path(mod).parent, ['-deps', '-test', '-json', './...'], cgo, raw)
    by_import = {}
    membership = {}
    excluded = set()
    for p in packages:
        if p.get('ForTest') or p.get('ImportPath', '').endswith('.test'):
            continue
        name = p.get('ImportPath', '')
        if name not in by_import or p.get('Dir'):
            by_import[name] = p
        directory = relative(root, p.get('Dir'))
        if directory is None:
            continue
        for field in ('GoFiles', 'CgoFiles', 'TestGoFiles', 'XTestGoFiles'):
            for base in p.get(field, []):
                file = (Path(directory) / base).as_posix()
                if file in tracked:
                    membership[file] = (p, field)
        for base in p.get('IgnoredGoFiles', []) + p.get('InvalidGoFiles', []):
            excluded.add((Path(directory) / base).as_posix())
    truth, gaps = [], {}
    for file in selected:
        member = membership.get(file)
        if member is None:
            gaps[file] = 'native-build-excluded' if file in excluded else 'native-unlisted'
            continue
        p, field = member
        if field == 'CgoFiles':
            gaps[file] = 'native-cgo-attribution'
            continue
        module_dir = Path((p.get('Module') or {}).get('GoMod', root / 'go.mod')).parent
        named_file = os.path.relpath(root / file, module_dir)
        single = native(module_dir, ['-json', named_file], cgo, raw)
        entries = [q for q in single if q.get('ImportPath') == 'command-line-arguments']
        if len(entries) != 1 or entries[0].get('Error') or entries[0].get('InvalidGoFiles'):
            gaps[file] = 'native-named-file-error'
            continue
        q = entries[0]
        if q.get('CgoFiles'):
            gaps[file] = 'native-cgo-attribution'
            continue
        mod = p.get('Module') or {}
        go_mod = relative(root, mod.get('GoMod'))
        if go_mod not in tracked:
            gaps[file] = 'native-module-unavailable'
            continue
        edges = [edge('config-ref', go_mod, go_mod)]
        go_sum = str(Path(go_mod).with_name('go.sum'))
        if go_sum in tracked:
            edges.append(edge('config-ref', go_sum, go_sum))
        fields = ['GoFiles', 'CgoFiles']
        if field == 'TestGoFiles':
            fields.append('TestGoFiles')
        elif field == 'XTestGoFiles':
            fields = ['XTestGoFiles']
        directory = relative(root, p['Dir'])
        siblings = sorted({(Path(directory) / n).as_posix() for f in fields for n in p.get(f, [])})
        if any(n not in tracked for n in siblings):
            gaps[file] = 'native-generated-input-unavailable'
            continue
        edges += [edge('build-input', n, n) for n in siblings if n != file]
        gap = None
        import_field = {'TestGoFiles': 'TestImports', 'XTestGoFiles': 'XTestImports'}.get(field, 'Imports')
        for operand in q.get(import_field, []):
            mapped = q.get('ImportMap', {}).get(operand, operand)
            target = by_import.get(mapped)
            if target is None or not target.get('Dir') or target.get('Error'):
                gap = 'native-import-unavailable'
                break
            local = relative(root, target['Dir'])
            if local is None:
                edges.append(edge('import', operand, 'go:' + operand, 'external'))
            else:
                edges.append(edge('import', operand, local, 'directory'))
        embed_prefix = {'TestGoFiles': 'Test', 'XTestGoFiles': 'XTest'}.get(field, '')
        patterns = q.get(embed_prefix + 'EmbedPatterns', [])
        if patterns:
            if len(patterns) != 1 or not q.get(embed_prefix + 'EmbedFiles'):
                gap = 'native-embed-attribution'
            else:
                for name in q[embed_prefix + 'EmbedFiles']:
                    to = (Path(file).parent / name).as_posix()
                    if to not in tracked:
                        gap = 'native-generated-input-unavailable'
                    edges.append(edge('embed', patterns[0], to))
        if gap:
            gaps[file] = gap
        else:
            truth.append({'Path': file, 'Edges': sorted(edges, key=encoded)})
    return {'truth': truth, 'gaps': gaps, 'raw': raw}


def git(root, *args):
    return subprocess.check_output(['git', '-C', str(root), *args])


def freeze(root, pin):
    if git(root, 'rev-parse', 'HEAD').decode().strip() != pin['commit']:
        raise ValueError('subject commit differs from preregistration')
    if git(root, 'status', '--porcelain', '--untracked-files=all', '--ignored=matching'):
        raise ValueError('subject checkout is not clean')
    tracked, selected, manifest, modes = [], [], [], Counter()
    for row in git(root, 'ls-tree', '-rz', 'HEAD').split(b'\0'):
        if not row:
            continue
        meta, name = row.split(b'\t', 1)
        mode, kind, oid = meta.decode().split()
        name = name.decode()
        modes[mode] += 1
        if mode in ('100644', '100755'):
            data = (root / name).read_bytes()
            tracked.append(name)
            if name.endswith('.go'):
                selected.append(name)
        elif mode == '120000':
            data = os.readlink(root / name).encode()
        else:
            data = oid.encode()
        manifest.append([name, mode, hashlib.sha256(data).hexdigest()])
    selected.sort()
    selection = hashlib.sha256(b''.join(n.encode() + b'\0' for n in selected)).hexdigest()
    if len(selected) != pin['files'] or selection != pin['selection_sha256']:
        raise ValueError('selection differs from preregistration')
    return tracked, selected, {'selection_sha256': selection, 'manifest_sha256': digest(manifest), 'modes': dict(modes)}


def campaign(sources, output):
    pins = json.loads(Path(__file__).with_name('cov_go_pins.json').read_text())
    go = Path(subprocess.check_output(['which', 'go']).decode().strip())
    if hashlib.sha256(go.read_bytes()).hexdigest() != pins['go_sha256']:
        raise ValueError('native executable digest mismatch')
    if subprocess.check_output(['go', 'version']).decode().split()[2] != pins['go_version']:
        raise ValueError('native version mismatch')
    output.mkdir(parents=True, exist_ok=False)
    for pin in pins['subjects']:
        name = pin['subject']
        subject = output / name
        subject.mkdir()
        captures = []
        receipts = []
        for location in ('primary', 'relocated'):
            dest = subject / location
            subprocess.run(['git', 'clone', '--quiet', '--no-hardlinks', '--no-checkout', str(sources / name), str(dest)], check=True)
            subprocess.run(['git', '-C', str(dest), 'checkout', '--quiet', '--detach', pin['commit']], check=True)
            tracked, selected, before = freeze(dest, pin)
            if location == 'primary':
                for context in pins['contexts']:
                    result = capture(dest, selected, context['CGO_ENABLED'], tracked)
                    raw = result.pop('raw')
                    raw_bytes = encoded(raw)
                    (subject / ('native-cgo' + context['CGO_ENABLED'] + '.json')).write_bytes(raw_bytes)
                    (subject / ('truth-cgo' + context['CGO_ENABLED'] + '.json')).write_bytes(encoded(result))
                    captures.append({'cgo': context['CGO_ENABLED'], 'raw_sha256': hashlib.sha256(raw_bytes).hexdigest(), 'truth_sha256': digest(result)})
            _, _, after = freeze(dest, pin)
            if before != after:
                raise ValueError('native run changed tracked inputs')
            receipts.append(before)
        if receipts[0] != receipts[1]:
            raise ValueError('relocation inputs differ')
        (subject / 'receipt.json').write_bytes(encoded({'inputs': receipts[0], 'captures': captures}))
        print(name + ': native capture preserved', flush=True)


if __name__ == '__main__':
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument('sources', type=Path)
    ap.add_argument('output', type=Path)
    args = ap.parse_args()
    campaign(args.sources.resolve(), args.output.resolve())
