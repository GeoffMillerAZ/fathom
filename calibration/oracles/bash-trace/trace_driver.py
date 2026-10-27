#!/usr/bin/env python3
"""Prototype native trace decoder; never calls a product extractor."""
import argparse, collections, json, os, pathlib, shlex, subprocess, sys

def events(path, max_bytes=32 << 20, max_fields=1_000_000, max_field_bytes=1 << 20):
    path = pathlib.Path(path)
    if path.stat().st_size > max_bytes:
        raise ValueError('Whole-unit refusal: native trace byte cap')
    content = path.read_bytes()
    if content and not content.endswith(b'\x00'):
        raise ValueError('Malformed native trace: incomplete trailing field')
    if content.count(b'\x00') > max_fields:
        raise ValueError('Whole-unit refusal: native trace field count cap')
    parts = content.split(b'\x00')
    if any(len(part) > max_field_bytes for part in parts):
        raise ValueError('Whole-unit refusal: native trace field byte cap')
    i = 0
    while i < len(parts) - 1:
        kind = parts[i].decode()
        i += 1
        size = {'ROOT': 2, 'ENTRY': 4, 'DEFS': 6, 'DEFS_FULLPATH': 6, 'VERSION': 2}.get(kind)
        if size is None or i + size > len(parts) - 1:
            raise ValueError('Malformed native trace')
        yield (kind, [x.decode('utf-8', 'surrogateescape') for x in parts[i:i + size]])
        i += size

def decode(path, root):
    roots = collections.defaultdict(set)
    rows = []
    root = pathlib.Path(root).resolve()

    def canonical(raw, cwd):
        p = pathlib.Path(raw)
        if not p.is_absolute():
            if not cwd:
                return None
            p = pathlib.Path(cwd) / p
        try:
            return p.resolve(strict=True)
        except OSError:
            return None
    for (kind, fields) in events(path):
        if kind == 'VERSION':
            continue
        if kind == 'ROOT':
            (raw, cwd) = fields
            p = canonical(raw, cwd)
            if p:
                roots[raw].add(p)
        else:
            if kind in ('DEFS','DEFS_FULLPATH'):
                (caller, line, cwd, command, before, after) = fields
                try:
                    words = shlex.split(command)
                except ValueError:
                    continue
                if len(words) < 2 or words[0] not in ('.', 'source'):
                    continue
                literal = words[1]
                if any((c in literal for c in '$`*?[')):
                    continue

                def table(text):
                    result = {}
                    for row in text.splitlines():
                        parts = row.split(maxsplit=2)
                        if len(parts) == 3 and parts[1].isdigit():
                            result[parts[0]] = (parts[1], parts[2])
                    return result
                old = table(before)
                new = table(after)
                changed = {v[1] for (k, v) in new.items() if old.get(k) != v}
                # Definition-only imports may be followed by a caller definition before DEBUG.
                # A changed native definition in the exact literal filename is mandatory.
                if kind == 'DEFS_FULLPATH':
                    targets = changed - {caller}
                    if len(targets) != 1:
                        continue
                    target = next(iter(targets))
                    if not pathlib.Path(target).is_absolute():
                        continue
                else:
                    if literal not in changed or changed - {literal, caller}:
                        continue
                    target = literal
            else:
                (target, caller, line, cwd) = fields
            p = canonical(target, cwd)
            if p:
                roots[target].add(p)
            rows.append((target, caller, int(line), p))
    out = []
    unknown = 0
    for (target, caller, line, p) in rows:
        candidates = roots[caller]
        if len(candidates) != 1 or p is None:
            unknown += 1
            continue
        c = next(iter(candidates))
        try:
            out.append({'path': str(c.relative_to(root)), 'line': line, 'to_path': str(p.relative_to(root))})
        except ValueError:
            continue
    return (sorted({(x['path'], x['line'], x['to_path']) for x in out}), unknown)
if __name__ == '__main__':
    a = argparse.ArgumentParser()
    a.add_argument('--trace', required=True)
    a.add_argument('--subject', required=True)
    n = a.parse_args()
    (rows, unknown) = decode(n.trace, n.subject)
    json.dump({'imports': [{'path': p, 'line': l, 'to_path': t} for (p, l, t) in rows], 'unknown_native_frames': unknown}, sys.stdout, indent=2)
    print()
