"""Reproduce native truth independence, population mutation and reader caps."""
import argparse
import hashlib
import io
import json
import os
import subprocess
import unittest
from pathlib import Path
from unittest.mock import patch

import native_population
from population import MAX_EVENTS, MAX_STDERR_BYTES


def focused(name):
    output = io.StringIO()
    suite = unittest.defaultTestLoader.loadTestsFromName(name)
    result = unittest.TextTestRunner(stream=output, verbosity=2).run(suite)
    return result, output.getvalue()


def truth(subject, native, work, native_definitions=False):
    definitions, refs, _ = native_population.measure(subject, ['nginx.conf'], native, work, native_definitions)
    if native_definitions:
        refs = dict(definitions=definitions, references=refs)
    return hashlib.sha256(json.dumps(refs, sort_keys=True, separators=(',', ':')).encode()).hexdigest()


def run(native, receipt, out, native_definitions=False):
    native_population.verify_identity(native, json.loads(receipt.read_text()))
    if out.exists():
        raise ValueError('fresh control output required')
    out.mkdir(parents=True)
    subject = out / 'authored'; subject.mkdir()
    source = ('events {}\nhttp {\n upstream alpha.invalid { server 127.0.0.1:19001; }\n'
              ' server { listen 127.0.0.1:19000; location / { proxy_pass http://alpha.invalid; } }\n}\n')
    file = subject / 'nginx.conf'; file.write_text(source)
    nodes = out / 'fathom-nodes.json'
    subprocess.run(['go', 'run', '-race', './calibration/oracles/nginx/extractor-output',
                    '-source', str(file), '-out', str(nodes)], check=True)
    node_count = len(json.loads(nodes.read_text()))
    if not node_count:
        raise ValueError('independence control needs actual fathom output')
    before = truth(subject, native, out / 'before', native_definitions)
    nodes.write_text('[]\n')
    perturbed = truth(subject, native, out / 'after-output-removal', native_definitions)
    if before != perturbed:
        raise ValueError('oracle depends on removed fathom output')
    file.write_text(source.replace('alpha.invalid', 'beta.invalid'))
    changed = truth(subject, native, out / 'after-input-change', native_definitions)
    if before == changed:
        raise ValueError('native truth did not observe changed input fact')

    os.environ['NGINX_OBSERVER'] = str(native.resolve())
    if native_definitions:
        import native_definitions as native_defs
        import nginx_oracle
        os.environ['NGINX_TOKENS'] = str(native.resolve())
        original = native_defs.definitions
        def scanner_selected(path, native_tree, private_directory='', owner_sites=None):
            complete = original(path, native_tree, private_directory, owner_sites)
            scanner = nginx_oracle.FileWalk(path, native_tree['tree']).defs
            signatures = {(row['name'], row['kind'], row['line']) for row in scanner}
            return [row for row in complete if (row['name'], row['kind'], row['line']) in signatures]
        test = 'test_native_definition_integration.NativeDefinitionControl.test_native_definitions_and_owners_survive_independent_walk_omission'
        with patch.object(native_defs, 'definitions', scanner_selected):
            red, red_log = focused(test)
    else:
        original = native_population.local_population
        def scanner_selected(events, path, locate):
            refs, counts = original(events, path, locate)
            return [ref for ref in refs if ref in locate.__self__.refs], counts
        test = 'test_native_population.NativePopulationControl.test_native_pair_survives_an_independent_scanner_omission'
        with patch.object(native_population, 'local_population', scanner_selected):
            red, red_log = focused(test)
    green, green_log = focused(test)
    mutant = 'filter native definitions by independent scanner definitions' if native_definitions else 'filter native refs by independent scanner refs'
    (out / 'mutation.log').write_text('MUTANT: ' + mutant + '\n' + red_log + '\nRESTORED\n' + green_log)
    if red.wasSuccessful() or not red.failures or red.errors or not green.wasSuccessful() or green.skipped:
        raise ValueError('population mutation control did not show FAIL then restored green')
    names = ['test_population.NativePopulationTest.test_event_cap_plus_one_refuses_whole_profile',
             'test_population.NativePopulationTest.test_stderr_cap_plus_one_refuses_whole_profile']
    if native_definitions:
        names += ['test_native_tokens.NativeTokenReaderTest.test_all_reader_caps_refuse_whole_profile',
                  'test_native_token_integration.NativeLexerControl']
    caps = unittest.TestSuite(unittest.defaultTestLoader.loadTestsFromName(name) for name in names)
    log = io.StringIO()
    cap_result = unittest.TextTestRunner(stream=log, verbosity=2).run(caps)
    (out / 'caps.log').write_text(log.getvalue())
    if not cap_result.wasSuccessful():
        raise ValueError('whole-profile cap controls failed')
    summary = dict(native_binary_sha256=hashlib.sha256(native.read_bytes()).hexdigest(),
                   fathom_nodes_before=node_count, fathom_nodes_after=0,
                   fathom_output_race_enabled=True,
                   truth_before_sha256=before, truth_after_fathom_removal_sha256=perturbed,
                   truth_after_input_change_sha256=changed,
                   mutation=dict(mutant=mutant,
                                 failure_line=next(line for line in red_log.splitlines() if line.startswith('FAIL:')),
                                 restored_green=True, native_definitions=native_definitions),
                   caps=dict(events=MAX_EVENTS, event_refusal_input=MAX_EVENTS + 1,
                             stderr_bytes=MAX_STDERR_BYTES, stderr_refusal_input=MAX_STDERR_BYTES + 1,
                             whole_profile_refused=True))
    if native_definitions:
        import native_tokens
        summary['native_lexer_caps'] = dict(tokens=native_tokens.MAX_TOKENS, token_refusal_input=native_tokens.MAX_TOKENS + 1,
                                          arguments=native_tokens.MAX_ARGS, argument_refusal_input=native_tokens.MAX_ARGS + 1,
                                          word_bytes=native_tokens.MAX_WORD, word_refusal_input=native_tokens.MAX_WORD + 1,
                                          path_refusal_input=native_tokens.MAX_WORD + 1, whole_profile_refused=True)
    (out / 'summary.json').write_text(json.dumps(summary, indent=2) + '\n')
    return summary


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    for name in ('nginx', 'build-receipt', 'out'):
        parser.add_argument('-' + name, type=Path, required=True)
    parser.add_argument('-native-definitions', action='store_true')
    args = parser.parse_args()
    print(json.dumps(run(args.nginx.resolve(), args.build_receipt, args.out.resolve(), args.native_definitions), sort_keys=True))
