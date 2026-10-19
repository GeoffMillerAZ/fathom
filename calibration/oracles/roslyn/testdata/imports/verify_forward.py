"""Future native forward-contract controls. Does not run a loader/compiler."""
import hashlib
import json
from pathlib import Path
import re
import sys


def pe_unsupported(raw, root):
    data = json.loads(Path(raw).read_bytes())
    root = Path(root).resolve()
    assert data['schema'] == 'fathom.roslyn.project-imports/v1'
    assert not data['workspace_failures'] and not data['compile_errors']
    project = next(p for p in data['projects'] if p['name'] == 'MetadataConsumer')
    assert project['project_references'] == []  # no manufactured source reference
    binary = root / 'binary/NativeDependency.dll'
    refs = [r for r in project['native_references'] if r['path'] and Path(r['path']).resolve() == binary]
    assert len(refs) == 1 and refs[0]['reference_kind'] == 'portable-executable'
    hashes = {x['path']: x['sha256'] for x in data['native_input_hashes']}
    assert hashes[str(binary)] == hashlib.sha256(binary.read_bytes()).hexdigest()
    rows = [r for r in data['usings'] if r['path'] == 'src/MetadataConsumer/Consumer.cs.input']
    assert [(r['written_name'], r['alias'], r['is_static']) for r in rows] == [
        ('BoundNamespace', None, False), ('BoundNamespace.Known', 'Alias', False),
        ('BoundNamespace.Known', None, True)]
    identity = 'NativeDependency, Version=1.2.3.4, Culture=neutral, PublicKeyToken=null'
    for row in rows:
        assert row['bound_symbol']['assembly'] == identity
        assert row['status'] == 'native_mapping_unknown'
        assert row['source_contribution_mapping_complete'] is False
        assert row['all_source_target_paths'] == row['selected_target_paths'] == row['measured_cross_file_target_paths'] == []
        for contribution in row['contributions']:
            assert contribution['status'] == 'source_mapping_unknown' and contribution['declarations'] == []
            assert contribution['source_identity_method'].startswith('UNSUPPORTED: PE-to-selected-source association;')
            candidates = contribution['source_identity_candidates']
            assert len(candidates) == 1 and candidates[0]['assembly'] == identity
            assert Path(candidates[0]['path']).parent.name == 'ExactSource'
    print('PE-to-selected-source association explicitly unsupported; all3bound rows UNKNOWN/zero targets. Not native7/7 qualification.')


def diagnostic_repeat(left, right):
    a = Path(left).read_bytes()
    b = Path(right).read_bytes()
    assert a == b, 'new/new stdout differs: strict raw bytes; no normalization or waiver'
    data = json.loads(a)
    assert data['schema'] == 'fathom.roslyn.project-declarations/v1'
    assert not data['workspace_failures']
    rows = data['compile_errors']
    assert len(rows) == 4 and all(r['project'] == 'Error' for r in rows)
    messages = [r['message'] for r in rows]
    # Compare adjacent rendered strings as UTF16 code units; never sort/replace
    # observed output. The four original fixture diagnostics must all remain.
    assert all(x.encode('utf-16-be') <= y.encode('utf-16-be') for x, y in zip(messages, messages[1:]))
    assert [re.search(r'error (CS\d+):', x).group(1) for x in messages] == ['CS0246', 'CS0122', 'CS0433', 'CS7007']
    print('Strict new/new bytes equal; all4original fixture errors retained in declared ordinal order. Historical old/new RED remains.')


if __name__ == '__main__':
    if len(sys.argv) != 4:
        raise SystemExit('verify_forward.py pe-unsupported RAW FIXTURE_ROOT | diagnostic-repeat LEFT RIGHT')
    if sys.argv[1] == 'pe-unsupported':
        pe_unsupported(sys.argv[2], sys.argv[3])
    elif sys.argv[1] == 'diagnostic-repeat':
        diagnostic_repeat(sys.argv[2], sys.argv[3])
    else:
        raise SystemExit('Unknown fixed-purpose control')
