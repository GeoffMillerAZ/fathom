import importlib.util
import json
from pathlib import Path
import shlex
import tempfile
import unittest

from call_sites import collect
from syntax_scope import parse, source_sites


class OriginalScopeTests(unittest.TestCase):
    def setUp(self):
        here = Path(__file__).resolve().parent
        parent = here.parents[2] / 'workspace/tmp'
        parent.mkdir(parents=True, exist_ok=True)
        temporary = tempfile.TemporaryDirectory(dir=parent)
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name)
        spec = importlib.util.spec_from_file_location('bash_syntax_fixture', here.parent / 'bash-source/bash_source_dump.py')
        self.syntax = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(self.syntax)

    @staticmethod
    def call(name, argument, line):
        return dict(Type='CallExpr', Args=[dict(Parts=[dict(Type='Lit', Value=name)], Pos=dict(Line=line, Col=1)),
                                           dict(Parts=[dict(Type='Lit', Value=argument)], Pos=dict(Line=line, Col=8))])

    def parser(self, payload, fail_input=None):
        path = self.root / 'parser'
        body = '#!/bin/sh\n'
        if fail_input:
            body += 'input=$(/bin/cat)\n[ "$input" != ' + shlex.quote(fail_input) + ' ] || exit 2\n'
        body += 'printf %s ' + shlex.quote(payload) + '\n'
        path.write_text(body)
        path.chmod(0o755)
        return str(path)

    def test_external_absolute_source_stays_in_full_population(self):
        tree = [self.call('source', '/external-oracle.sh', 1), self.call('.', 'local.sh', 2)]
        sites, dynamic, external = source_sites(tree, self.root, 'input.sh', self.syntax)
        self.assertEqual([s['value'] for s in sites], ['/external-oracle.sh', 'local.sh'])
        self.assertEqual((dynamic, external), (0, 1))

    def test_source_site_cap_accepts_n_then_refuses_n_plus_one(self):
        tree = [self.call('source', 'one.sh', 1), self.call('.', 'two.sh', 2)]
        self.assertEqual(len(source_sites(tree, self.root, 'input.sh', self.syntax, max_sites=2)[0]), 2)
        with self.assertRaisesRegex(ValueError, 'whole-unit refusal: source site cap'):
            source_sites(tree, self.root, 'input.sh', self.syntax, max_sites=1)

    def test_call_site_cap_stops_before_later_parser_failure(self):
        parser = self.parser(json.dumps(self.call('echo', 'value', 1)), fail_input='invalid')
        for name, content in [('one.sh', 'first'), ('two.sh', 'second'), ('three.sh', 'invalid')]:
            (self.root / name).write_text(content)
        with self.assertRaisesRegex(ValueError, 'literal call-site cap'):
            collect(self.root, ['one.sh', 'two.sh', 'three.sh'], parser, self.syntax, max_sites=1)
        self.assertEqual(len(collect(self.root, ['one.sh', 'two.sh'], parser, self.syntax, max_sites=2)['sites']), 2)

    def test_parser_byte_cap_accepts_n_then_refuses_n_plus_one(self):
        parser = self.parser('{}')
        self.assertEqual(parse(parser, b'', max_bytes=2), {})
        with self.assertRaisesRegex(ValueError, 'parser output byte cap'):
            parse(parser, b'', max_bytes=1)

    def test_parser_nesting_cap_accepts_n_then_refuses_n_plus_one(self):
        parser = self.parser('{"x":[[]]}')
        self.assertEqual(parse(parser, b'', max_depth=3), {'x': [[]]})
        with self.assertRaisesRegex(ValueError, 'parser nesting cap'):
            parse(parser, b'', max_depth=2)

    def test_malformed_output_and_raised_limits_refuse_the_unit(self):
        with self.assertRaisesRegex(ValueError, 'malformed or overnested'):
            parse(self.parser('{'), b'')
        for limits in [dict(max_bytes=0), dict(max_bytes=(32 << 20) + 1), dict(max_depth=257)]:
            with self.subTest(limits=limits), self.assertRaisesRegex(ValueError, 'invalid parser caps'):
                parse(self.parser('{}'), b'', **limits)
