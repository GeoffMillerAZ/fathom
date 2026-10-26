"""Authored native lexer population/range controls; no subject content."""
import os
import resource
import subprocess
import tempfile
import unittest
from pathlib import Path
from native_tokens import MAX_STDERR, decode, trees


def log_limit():
    resource.setrlimit(resource.RLIMIT_CORE, (0, 0))
    resource.setrlimit(resource.RLIMIT_FSIZE, (MAX_STDERR + 1, MAX_STDERR + 1))


@unittest.skipUnless(os.environ.get('NGINX_TOKENS'), 'set NGINX_TOKENS to the pinned native lexer observer')
class NativeLexerControl(unittest.TestCase):
    def observe(self, source, path_bytes=None, extras=None):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / 'logs').mkdir()
            parent = root
            if path_bytes:
                remaining = path_bytes - len(str(parent / 'authored.conf').encode())
                while remaining:
                    length = min(200, remaining - 1)
                    if remaining - length - 1 == 1:
                        length -= 1
                    if length < 1:
                        raise ValueError('cannot form exact authored path length')
                    parent = parent / ('x' * length)
                    parent.mkdir()
                    remaining -= length + 1
            path = parent / 'authored.conf'
            if path_bytes:
                self.assertEqual(path_bytes, len(str(path).encode()))
            path.write_text(source)
            for name, value in (extras or {}).items():
                (root / name).write_text(value)
            with tempfile.TemporaryFile() as stdout, tempfile.TemporaryFile() as stderr:
                process = subprocess.run([os.environ['NGINX_TOKENS'], '-T', '-p', str(root) + '/',
                                          '-c', str(path), '-e', str(root / 'error.log')],
                                         stdout=stdout, stderr=stderr, timeout=15, preexec_fn=log_limit)
                stdout.seek(0); stderr.seek(0)
                raw = stderr.read(MAX_STDERR + 1)
            if process.returncode:
                self.fail('authored native configuration refused: ' + raw.decode(errors='replace')[-600:])
            return raw, str(path)

    def test_multiline_native_headers_have_first_word_and_close_lines(self):
        source = ('# café\nevents {}\nhttp {\n upstream\n  alpha.invalid\n {\n'
                  '  server 127.0.0.1:19001;\n }\n server {\n  listen 127.0.0.1:19000;\n'
                  '  server_name "café.invalid";\n  location\n   /\n  {\n'
                  '   proxy_pass http://alpha.invalid;\n  }\n }\n}\n')
        raw, path = self.observe(source)
        native = trees(decode(raw))[path]
        self.assertEqual(18, native['file_end'])
        upstream = native['tree'][1]['block'][0]
        self.assertEqual(('upstream', 4, 8), (upstream['directive'], upstream['line'], upstream['end']))
        server = native['tree'][1]['block'][1]
        self.assertEqual('café.invalid', server['block'][1]['args'][0])
        location = server['block'][2]
        self.assertEqual((12, 16), (location['line'], location['end']))

    def test_empty_inline_data_blocks_unused_declarations_and_no_final_newline(self):
        source = 'events {}\nhttp { types { text/plain txt; } upstream unused.invalid { server 127.0.0.1:19001; } }'
        raw, path = self.observe(source)
        native = trees(decode(raw))[path]
        self.assertEqual(2, native['file_end'])
        blocks = native['tree'][1]['block']
        self.assertEqual(['types', 'upstream'], [node['directive'] for node in blocks])
        self.assertEqual(['unused.invalid'], blocks[1]['args'])
        self.assertEqual('text/plain', blocks[0]['block'][0]['directive'])

    def test_native_argument_cap_plus_one_refuses_even_successful_configuration(self):
        source = 'events {}\nhttp { server { server_name ' + ' '.join('n%d.invalid' % i for i in range(64)) + '; } }\n'
        raw, _ = self.observe(source)
        with self.assertRaisesRegex(ValueError, 'refused'):
            decode(raw)

    def test_native_word_cap_plus_one_refuses_even_successful_configuration(self):
        source = 'events {}\nhttp { log_format authored "' + 'x' * 513 + '"; }\n'
        raw, _ = self.observe(source)
        with self.assertRaisesRegex(ValueError, 'refused'):
            decode(raw)

    def test_native_source_path_cap_plus_one_refuses_whole_profile(self):
        raw, _ = self.observe('events {}\n', path_bytes=513)
        with self.assertRaisesRegex(ValueError, 'refused'):
            decode(raw)

    def test_native_count_cap_plus_one_refuses_whole_profile(self):
        source = 'events {}\nhttp { server {\n' + 'set $authored 1;\n' * 9994 + '} }\n'
        raw, _ = self.observe(source)
        with self.assertRaisesRegex(ValueError, 'refused'):
            decode(raw)

    def test_identical_native_duplicate_includes_share_source_positions_only(self):
        raw, path = self.observe('events {}\nhttp { include headers.conf; include headers.conf; }\n',
                                 extras={'headers.conf':'add_header X-Probe yes;\n'})
        result = trees(decode(raw))[str(Path(path).parent / 'headers.conf')]
        self.assertEqual(2,result['evaluations'])
        self.assertEqual(1,len(result['tree']))
        self.assertEqual((1,1),(result['tree'][0]['line'],result['tree'][0]['end']))
