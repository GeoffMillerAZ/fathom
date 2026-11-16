"""Compare native positioned truth with public output; diagnostic, no grade.

This is deliberately not a tool-oracle Reading adapter: native end ranges and
several required declaration kinds remain unmeasured. Never promote this partial
vocabulary to the canonical Ansible reading. Call under the language lease.
"""
import argparse
from collections import Counter
import hashlib
import json
from pathlib import Path
import subprocess


def unique(rows):
    return [json.loads(s) for s in sorted({json.dumps(r, sort_keys=True) for r in rows})]


def truth(document):
    loaded = [r for r in document['results'] if r['status'] == 'loaded']
    context = [r for r in loaded if r['mode'] in ('playbook', 'role')]
    covered = {d['path'] for r in context for d in r['declarations']}
    selected = list(context)
    for row in loaded:
        if row['input'] in covered:
            continue
        handler = 'handlers' in Path(row['input']).parts
        if row['mode'] == ('handlers' if handler else 'tasks'):
            selected.append(row)
    ds = unique([d for r in selected for d in r['declarations']])
    bs = unique([b for r in selected for b in r['bindings']])
    us = unique([u for r in selected for u in r['unresolved']])
    return dict(declarations=[d for d in ds if not d['external']],
                bindings=[b for b in bs if not b['source']['external']],
                unresolved=[u for u in us if not u['source']['external']],
                external_declarations=sum(d['external'] for d in ds),
                external_bindings=sum(b['source']['external'] for b in bs),
                external_unresolved=sum(u['source']['external'] for u in us))


def ratio(matched, total):
    return dict(matched=matched, native=total, ratio=matched / total if total else None)


def compare(native, product):
    kinds = dict(task='AnsibleTask', play='AnsibleTask', handler='AnsibleHandler')
    symbols, targets, refs = set(), {}, set()
    statuses = Counter()
    for file in product['files']:
        result = file.get('result') or {}
        statuses[result.get('dialect') or 'unread'] += 1
        if result.get('dialect') != 'ansible':
            continue
        for s in result['symbols']:
            symbols.add((file['path'], s['line'], s['name'], s['kind']))
            targets[(file['path'], s.get('key', s['name']))] = (s['line'], s['kind'])
        for r in result['references']:
            if r['edge'] == 'notify' and r.get('to_key'):
                refs.add((file['path'], r['from'], r['to_key']))
    declarations = native['declarations']
    matched = sum((d['path'], d['line'], d['name'], kinds[d['kind']]) in symbols for d in declarations)
    # Require a native target position. A bare target name in another file is
    # not proof the library bound it there. L3 remains outside this audit.
    matched_bindings = 0
    for b in native['bindings']:
        source, target = b['source'], b['target']
        for path, from_key, to_key in refs:
            if path != source['path'] or target['path'] != path:
                continue
            if targets.get((path, from_key)) != (source['line'], kinds[source['kind']]):
                continue
            if targets.get((path, to_key)) == (target['line'], 'AnsibleHandler'):
                matched_bindings += 1
                break
    misses = len(native['unresolved'])
    return dict(declarations=ratio(matched, len(declarations)),
                bindings=ratio(matched_bindings, len(native['bindings'])),
                unresolved_misses=misses,
                bindings_including_unresolved=ratio(matched_bindings, len(native['bindings']) + misses),
                product_dialects=dict(sorted(statuses.items())),
                authority='native start/name and bound local target diagnostics only; no support grade')


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--native', type=Path, required=True)
    p.add_argument('--product', type=Path, required=True)
    p.add_argument('--out', type=Path, required=True)
    args = p.parse_args()
    document = json.loads(args.native.read_text())
    t = truth(document)
    result = compare(t, json.loads(args.product.read_text()))
    result.update(subject=document['subject'], sha=document['sha'],
                  native_sha256=hashlib.sha256(args.native.read_bytes()).hexdigest(),
                  product_sha256=hashlib.sha256(args.product.read_bytes()).hexdigest(),
                  truth_sha256=hashlib.sha256(json.dumps(t, sort_keys=True).encode()).hexdigest(),
                  external_declarations=t['external_declarations'],
                  external_bindings=t['external_bindings'],
                  unresolved_classes=dict(sorted(Counter(u['reason'] for u in t['unresolved']).items())))
    args.out.write_text(json.dumps(result, sort_keys=True, indent=2)+'\n')


if __name__ == '__main__':
    main()
