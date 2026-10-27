import hashlib, json, os, pathlib, stat, subprocess
root = pathlib.Path(os.environ['SUBJECT']).resolve()
scope = os.environ['SUBPATH']
out = pathlib.Path(os.environ['RECEIPT'])
top = subprocess.check_output(['git', '-C', str(root), 'rev-parse', '--show-toplevel'])
if pathlib.Path(os.fsdecode(top).rstrip('\n')).resolve() != root:
    raise SystemExit('SUBJECT must be the subject Git checkout root')
def paths(*args):
    return subprocess.check_output(['git', '-C', str(root), 'ls-files', '-z', *args, '--', scope])
entries = []
for record in paths('-s', '--cached').split(b'\0'):
    if not record:
        continue
    meta, name = record.split(b'\t', 1)
    mode, oid, stage = meta.split()
    if stage != b'0':
        raise SystemExit('unmerged index: resolve conflicts first')
    entries.append((name, mode, oid))
if not entries:
    raise SystemExit('empty tracked set: check SUBJECT and SUBPATH')
lines, size = [], 0
counts = dict(files=0, symlinks=0, gitlinks=0)
for name, mode, oid in sorted(entries):
    path = root / os.fsdecode(name)
    if mode in (b'100644', b'100755'):
        if not stat.S_ISREG(path.lstat().st_mode):
            raise SystemExit('tracked regular file has changed type')
        data = path.read_bytes()
        kind, identity = 'file', hashlib.sha256(data).hexdigest()
        counts['files'] += 1
        size += len(data)
    elif mode == b'120000':
        kind, identity = 'symlink', hashlib.sha256(os.readlink(os.fsencode(path))).hexdigest()
        counts['symlinks'] += 1
    elif mode == b'160000':
        kind, identity = 'gitlink', oid.decode('ascii')
        counts['gitlinks'] += 1
    else:
        raise SystemExit('unsupported Git index mode')
    lines.append(json.dumps([os.fsdecode(name), kind, identity],
                           ensure_ascii=True, separators=(',', ':')) + '\n')
manifest = ''.join(lines).encode('utf-8')
out.parent.mkdir(parents=True, exist_ok=True)
pathlib.Path(str(out) + '.manifest').write_bytes(manifest)
pathlib.Path(str(out) + '.untracked').write_bytes(paths('--others'))
summary = dict(sha256=hashlib.sha256(manifest).hexdigest(), entries=len(entries), bytes=size, **counts)
pathlib.Path(str(out) + '.json').write_text(json.dumps(summary, sort_keys=True) + '\n')
print(json.dumps(summary, sort_keys=True))
