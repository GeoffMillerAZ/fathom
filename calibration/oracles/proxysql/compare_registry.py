"""Compare literal configuration keys with native registry names; never adjudicates."""
import argparse
import hashlib
import json
import subprocess
from pathlib import Path
import proxysql_oracle

if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('-subject', type=Path, required=True)
    parser.add_argument('-registry', type=Path, required=True)
    args = parser.parse_args()
    pin = '7ddb3dc015d1440e8a0fb0bbafa2df9aaaa2669d'
    if subprocess.check_output(['git', '-C', str(args.subject), 'rev-parse', 'HEAD'], text=True).strip() != pin:
        raise SystemExit('source identity mismatch')
    if subprocess.check_output(['git', '-C', str(args.subject), 'status', '--porcelain', '--', 'src/proxysql.cfg'], text=True).strip():
        raise SystemExit('configuration content must be clean')
    registry = json.loads(args.registry.read_text())
    source = proxysql_oracle.parse((args.subject / 'src/proxysql.cfg').read_text()).value
    comparisons = []
    for prefix in ('mcp', 'genai'):
        names = set(registry[prefix])
        for key, value in source[prefix + '_variables'].value.items():
            literal = prefix + '-' + key
            shortened = prefix + '-' + (key[len(prefix) + 1:] if key.startswith(prefix + '_') else key)
            comparisons.append({'group': prefix, 'key': key, 'line': value.line,
                                'literal_native_name': literal, 'literal_in_registry': literal in names,
                                'after_underscore_prefix_removal': shortened,
                                'shortened_in_registry': shortened in names})
    summary = {}
    for prefix in ('mcp', 'genai'):
        rows = [row for row in comparisons if row['group'] == prefix]
        summary[prefix] = {'literals': len(rows), 'literal_matches': sum(row['literal_in_registry'] for row in rows),
                           'hypothetical_shortened_matches': sum(row['shortened_in_registry'] for row in rows)}
    print(json.dumps({'source_sha': pin, 'groups': summary,
                      'comparisons_sha256': hashlib.sha256(json.dumps(comparisons, sort_keys=True).encode()).hexdigest()}, indent=2))
