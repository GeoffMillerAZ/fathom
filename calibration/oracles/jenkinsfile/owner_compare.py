"""Conservative ModelAST projection. Partial diagnostic, never a support reading.

The written-unit contract is OWNER-COMPARISON.md. This module consumes only the
owner receipt for truth; candidate definitions cannot fill owner-model gaps.
"""
import argparse
import collections
import hashlib
import json
import sys
from contextlib import contextmanager
from pathlib import Path
from native_model_probe import summarize

class ComparisonRefused(ValueError):
    """The diagnostic cannot attest a complete input unit."""


@contextmanager
def output_unit(out):
    """Invalidate a previous receipt before reading; never retain partial success."""
    try:
        out.unlink(missing_ok=True)
        yield
    except (OSError, ValueError, TypeError, KeyError, IndexError, RecursionError) as error:
        try:
            out.unlink(missing_ok=True)
        except OSError:
            pass  # The structured refusal still reports an unusable output path.
        raise ComparisonRefused('whole comparison refused') from error


def read_json(path, with_digest=False):
    # A bounded read also covers files which grow after stat; the handle always closes.
    with path.open('rb') as stream:
        data = stream.read(32 * 1024**2 + 1)
    if len(data) > 32 * 1024**2:
        raise ComparisonRefused('JSON parse cap')
    parsed = json.loads(data)
    return (parsed, hashlib.sha256(data).hexdigest()) if with_digest else parsed


def refuse(error):
    print(json.dumps(dict(error='comparison_refused', reason=str(error))), file=sys.stderr)
    return 1


DECLARATIONS = {
    'ModelASTPipelineDef': ('pipeline', 'job'),
    'ModelASTStages': ('stages', 'section'),
    'ModelASTStage': (None, 'stage'),
    'ModelASTAgent': ('agent', 'resource'),
    'ModelASTEnvironment': ('environment', 'variable_decl'),
    'ModelASTPostBuild': ('post', 'section'),
    'ModelASTPostStage': ('post', 'section'),
    'ModelASTBuildParameters': ('parameters', 'section'),
    'ModelASTOptions': ('options', 'section'),
}
CALLS = {'ModelASTStep', 'ModelASTTreeStep', 'ModelASTBuildParameter',
         'ModelASTOption', 'ModelASTTrigger', 'ModelASTWhenCondition',
         'ModelASTMethodCall', 'ModelASTInternalFunctionCall'}
KEYWORDS = {'pipeline', 'agent', 'stages', 'stage', 'steps', 'post',
            'environment', 'parameters', 'options'}
OPAQUE = {'ModelASTScriptBlock', 'ModelASTWhenExpression'}
ARGUMENTS = {'ModelASTKey', 'ModelASTValue$ConstantValue', 'ModelASTValue$GStringValue',
             'ModelASTNamedArgumentList', 'ModelASTSingleArgument',
             'ModelASTKeyValueOrMethodCallPair', 'ModelASTClosureMap'}
STRUCTURE = {'ModelASTParallel', 'ModelASTBuildCondition', 'ModelASTTools',
             'ModelASTTriggers', 'ModelASTWhen'}


def match(truth, candidate):
    """Exact multiset match: duplicate written units retain their multiplicity."""
    a, b = collections.Counter(map(tuple, truth)), collections.Counter(map(tuple, candidate))
    return dict(truth=sum(a.values()), candidate=sum(b.values()), matched=sum((a & b).values()))


def project(raw):
    summarize(raw)  # Strict schema, positions, parents and whole-input caps.
    definitions, pairs = [], []
    accounting, disclosures = collections.Counter(), collections.Counter()
    for row in raw['files']:
        if row['status'] != 'parsed':
            disclosures[row['status'] + '_files'] += 1
            continue
        nodes = row['nodes']
        owners = {}
        blocked = set()
        for i, node in enumerate(nodes):
            kind = node['type']
            parent = node['parent']
            if parent in blocked:
                blocked.add(i)
            if kind in DECLARATIONS:
                name, category = DECLARATIONS[kind]
                if kind == 'ModelASTStage':
                    name = node['name']
                elif kind != 'ModelASTPipelineDef':
                    ancestor = parent
                    while ancestor >= 0:
                        if nodes[ancestor]['type'] == 'ModelASTStage':
                            name = nodes[ancestor]['name'] + '.' + name
                            break
                        ancestor = nodes[ancestor]['parent']
                if not name or node['line'] < 1:
                    disclosures['unpositioned_or_unnamed_declarations'] += 1
                    accounting['unmapped_declarations'] += 1
                    blocked.add(i)
                else:
                    definitions.append((name, category, node['line'], node['end']))
                    owners[i] = name
                    accounting['declarations'] += 1
            elif kind == 'ModelASTBranch':
                accounting['synthetic_branches'] += 1
                blocked.add(i)
                if parent >= 0 and nodes[parent]['type'] == 'ModelASTStage':
                    disclosures['written_steps_unmapped'] += 1
            elif kind in OPAQUE:
                accounting['opaque'] += 1
                disclosures['opaque_bodies'] += 1
                blocked.add(i)
            elif kind in CALLS:
                accounting['calls'] += 1
                if node['name'] in KEYWORDS:
                    disclosures['declaration_keyword_calls'] += 1
                    continue
                owner = parent
                while owner >= 0 and owner not in owners:
                    owner = nodes[owner]['parent']
                if i in blocked or owner < 0 or not node['name'] or node['line'] < 1:
                    disclosures['calls_without_written_owner'] += 1
                else:
                    pairs.append((owners[owner], node['name']))
            elif kind in ARGUMENTS:
                accounting['arguments'] += 1
            elif kind in STRUCTURE:
                accounting['structure_without_call_label'] += 1
                disclosures['structural_call_labels_unmapped'] += 1
            else:
                accounting['unknown'] += 1
                disclosures['unknown_model_classes'] += 1
    # The receipt cannot certify absence of top-level methods or library imports.
    # No partial model is a full written-unit denominator, including empty models.
    return dict(definitions=definitions, pairs=pairs,
                accounting=dict(sorted(accounting.items())),
                disclosures=dict(sorted(disclosures.items())),
                complete_l1=False, complete_l2=False)


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('--raw', type=Path, required=True)
    parser.add_argument('--out', type=Path, required=True)
    args = parser.parse_args()
    try:
        with output_unit(args.out):
            raw = read_json(args.raw)
            result = project(raw)
            # Never publish source/model names or individual positions in the receipt.
            result['declarations'] = len(result.pop('definitions'))
            result['pairs'] = len(result['pairs'])
            result['subjects'] = []
            for subject in sorted({r['subject'] for r in raw['files']}):
                subset = dict(raw, files=[r for r in raw['files'] if r['subject'] == subject])
                p = project(subset)
                result['subjects'].append(dict(subject=subject, files=len(subset['files']),
                    declarations=len(p['definitions']), pairs=len(p['pairs']),
                    accounting=p['accounting'], disclosures=p['disclosures']))
            result.update(tier_granted=False, calibration_measured=False)
            args.out.write_text(json.dumps(result, indent=2) + '\n')
    except ComparisonRefused as error:
        sys.exit(refuse(error))
