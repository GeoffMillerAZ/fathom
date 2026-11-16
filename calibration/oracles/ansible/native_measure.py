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
from native_loader import (Refused, frozen_row, expected_units, validate_result,
                           prepare_output, write_output)


def unique(rows):
    return [json.loads(s) for s in sorted({json.dumps(r, sort_keys=True) for r in rows})]


def truth(document, population=None):
    if 'yaml_candidates' in document:
        if population is None:
            raise Refused('frozen inventory required before truth selection')
        validate_native(document, population)
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


def validate_native(document, population):
    if not isinstance(document, dict):
        raise Refused('malformed native document')
    row = frozen_row([population], document.get('subject'), document.get('sha'))
    expected = expected_units(row)
    if (document.get('sha') != row['sha'] or
            type(document.get('yaml_candidates')) is not int or type(document.get('role_candidates')) is not int or
            document.get('yaml_candidates') != len(row['yaml']) or
            document.get('role_candidates') != sum(mode == 'role' for _, mode in expected) or
            document.get('tasks_executed') is not False):
        raise Refused('native inventory header mismatch')
    results = document.get('results')
    if not isinstance(results, list):
        raise Refused('missing native results')
    seen = set()
    statuses = Counter(loaded=0, rejected=0, refused=0)
    for result in results:
        validate_result(result, unit_fields=True)
        unit = (result.get('input'), result.get('mode'))
        if not all(isinstance(v, str) for v in unit) or unit not in expected or unit in seen:
            raise Refused('duplicate or unexpected native unit')
        seen.add(unit)
        statuses[result['status']] += 1
    if seen != set(expected):
        raise Refused('incomplete native collection; no comparison published')
    return dict(status='complete', expected=len(expected), attempted=len(results),
                statuses=dict(sorted(statuses.items())))


def validate_product(product, row):
    if not isinstance(product, dict) or not isinstance(product.get('files'), list):
        raise Refused('malformed product inventory')
    paths = []
    for file in product['files']:
        if not isinstance(file, dict) or not isinstance(file.get('path'), str):
            raise Refused('malformed product file')
        paths.append(file['path'])
        result = file.get('result')
        if result is None: continue
        if not isinstance(result, dict) or not isinstance(result.get('dialect'), str):
            raise Refused('malformed product result')
        for key in ('symbols', 'references'):
            if not isinstance(result.get(key), list) or any(not isinstance(r, dict) for r in result[key]):
                raise Refused('malformed product records')
        if result['dialect'] == 'ansible':
            for symbol in result['symbols']:
                if (not isinstance(symbol.get('name'), str) or type(symbol.get('line')) is not int or
                        symbol['line'] < 1 or not isinstance(symbol.get('kind'), str) or
                        ('key' in symbol and not isinstance(symbol['key'], str))):
                    raise Refused('malformed product symbol')
            for reference in result['references']:
                if not isinstance(reference.get('edge'), str):
                    raise Refused('malformed product reference')
                if reference['edge'] == 'notify' and (not isinstance(reference.get('from'), str) or
                        ('to_key' in reference and not isinstance(reference['to_key'], str))):
                    raise Refused('malformed product notify')
    if len(paths) != len(set(paths)) or set(paths) != set(row['yaml']):
        raise Refused('incomplete, duplicate or unexpected product inventory')


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--native', type=Path, required=True)
    p.add_argument('--product', type=Path, required=True)
    p.add_argument('--out', type=Path, required=True)
    p.add_argument('--population', type=Path, help='original frozen subject/YAML inventory (required for comparison)')
    args = p.parse_args()
    out = prepare_output(args.out, [args.native, args.product, args.population])
    if args.population is None:
        raise Refused('frozen --population is required; no comparison published')
    document = json.loads(args.native.read_text())
    if not isinstance(document, dict):
        raise Refused('malformed native document')
    row = frozen_row(json.loads(args.population.read_text()), document.get('subject'), document.get('sha'))
    collection = validate_native(document, row)
    product = json.loads(args.product.read_text())
    validate_product(product, row)
    t = truth(document, row)
    result = compare(t, product)
    result['collection'] = collection
    result['population_sha256'] = hashlib.sha256(args.population.read_bytes()).hexdigest()
    result.update(subject=document['subject'], sha=document['sha'],
                  native_sha256=hashlib.sha256(args.native.read_bytes()).hexdigest(),
                  product_sha256=hashlib.sha256(args.product.read_bytes()).hexdigest(),
                  truth_sha256=hashlib.sha256(json.dumps(t, sort_keys=True).encode()).hexdigest(),
                  external_declarations=t['external_declarations'],
                  external_bindings=t['external_bindings'],
                  unresolved_classes=dict(sorted(Counter(u['reason'] for u in t['unresolved']).items())))
    write_output(out, result)


if __name__ == '__main__':
    main()
