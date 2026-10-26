"""Authored controls for target-only mutation and native error attribution."""
import unittest
import os
import tempfile
from pathlib import Path
from native_upstream import rename_target, attests_error, copy_subject, measure


class NativeUpstreamTest(unittest.TestCase):
    def test_only_selected_declaration_changes(self):
        source = '# upstream app {\nhttp {\n  upstream app { server 127.0.0.1:9010; }\n  server { location / { proxy_pass http://app; } }\n}\n'
        changed = rename_target(source, 'app', 3)
        self.assertEqual(changed, source.replace('  upstream app {', '  upstream zz_oracle_unreferenced {'))
        self.assertEqual(changed.count('\n'), source.count('\n'))

    def test_wrong_line_and_ambiguous_declarations_are_refused(self):
        source = 'upstream app { server 127.0.0.1; }\n'
        with self.assertRaises(ValueError):
            rename_target(source, 'app', 2)
        with self.assertRaises(ValueError):
            rename_target(source + source, 'app', 1)

    def test_native_error_must_name_target_and_queried_directive(self):
        message = 'nginx: [emerg] host not found in upstream "app" in sub/site.conf:17'
        self.assertTrue(attests_error(message, 'app', 'sub/site.conf', {17}))
        self.assertFalse(attests_error(message, 'different', 'sub/site.conf', {17}))
        self.assertFalse(attests_error(message, 'app', 'other/site.conf', {17}))
        self.assertFalse(attests_error(message, 'app', 'sub/site.conf', {18}))
        self.assertFalse(attests_error('syntax error in sub/site.conf:17', 'app', 'sub/site.conf', {17}))

    def test_native_population_can_attest_removed_named_upstream_without_default_port(self):
        message = 'nginx: [emerg] no port in upstream "app" in sub/site.conf:17'
        self.assertFalse(attests_error(message, 'app', 'sub/site.conf', {17}))
        self.assertTrue(attests_error(message, 'app', 'sub/site.conf', {17}, allow_no_port=True))
        self.assertFalse(attests_error(message, 'app', 'sub/site.conf', {18}, allow_no_port=True))
        self.assertFalse(attests_error(message, 'other', 'sub/site.conf', {17}, allow_no_port=True))


class ScratchBoundsTest(unittest.TestCase):
    def test_scratch_cannot_recurse_into_subject(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            with self.assertRaises(ValueError):
                copy_subject(root, root / 'scratch')
            with self.assertRaises(ValueError):
                measure(root, [], Path('/not-needed'), root / 'scratch')


@unittest.skipUnless(os.environ.get('NGINX_NATIVE'), 'set NGINX_NATIVE for authored native control')
class NativeControlTest(unittest.TestCase):
    def test_relative_scratch_matches_absolute_scratch(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / 'subject').mkdir()
            (root / 'work').mkdir()
            (root / 'subject/nginx.conf').write_text('events {}\nhttp { server { listen 127.0.0.1:19000; } }\n')
            previous = Path.cwd()
            try:
                os.chdir(root)
                definitions, references, audit = measure(Path('subject'), ['nginx.conf'], Path(os.environ['NGINX_NATIVE']), Path('work'))
                self.assertTrue(definitions)
                self.assertEqual(references, [])
            finally:
                os.chdir(previous)

    def test_actual_loader_distinguishes_target_only_ablation(self):
        with tempfile.TemporaryDirectory() as tmp:
            subject = Path(tmp) / 'subject'
            subject.mkdir()
            source = ('events {}\nhttp {\n  upstream oracle.invalid { server 127.0.0.1:19001; }\n'
                      '  server {\n    listen 127.0.0.1:19000;\n    location / {\n'
                      '      proxy_pass http://oracle.invalid;\n    }\n  }\n}\n')
            (subject / 'nginx.conf').write_text(source)
            work = Path(tmp) / 'work'
            work.mkdir()
            definitions, references, audit = measure(subject, ['nginx.conf'], Path(os.environ['NGINX_NATIVE']), work)
            self.assertEqual(len(references), 1)
            self.assertEqual(references[0]['to'], 'oracle.invalid')
            self.assertEqual(len(audit), 1)
            self.assertEqual((subject / 'nginx.conf').read_text(), source)
