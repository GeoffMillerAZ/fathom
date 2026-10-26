import hashlib
import json
import os
import tempfile
import unittest
from pathlib import Path
from native_population import verify_identity, measure


class IdentityTest(unittest.TestCase):
    def test_plain_or_mismatched_binary_cannot_claim_population_authority(self):
        with tempfile.TemporaryDirectory() as directory:
            binary = Path(directory) / 'native'
            binary.write_bytes(b'authored identity control')
            receipt = dict(version='nginx/1.31.6', binary_sha256=hashlib.sha256(binary.read_bytes()).hexdigest())
            with self.assertRaisesRegex(ValueError, 'observer'):
                verify_identity(binary, receipt)
            receipt['binary_sha256'] = '0' * 64
            with self.assertRaisesRegex(ValueError, 'identity'):
                verify_identity(binary, receipt)


@unittest.skipUnless(os.environ.get('NGINX_OBSERVER'), 'native observer build required')
class NativePopulationControl(unittest.TestCase):
    def native(self, source):
        with tempfile.TemporaryDirectory() as directory:
            subject = Path(directory) / 'subject'; subject.mkdir()
            (subject / 'nginx.conf').write_text(source)
            return measure(subject, ['nginx.conf'], Path(os.environ['NGINX_OBSERVER']), Path(directory) / 'work')

    def test_native_population_covers_both_domains_unused_and_forward_declarations(self):
        source = ('events {}\nhttp {\n'
                  ' server { listen 127.0.0.1:19000; location / { proxy_pass http://web.invalid; } }\n'
                  ' upstream web.invalid { server 127.0.0.1:19001; }\n'
                  ' upstream unused.invalid { server 127.0.0.1:19002; }\n}\n'
                  'stream {\n upstream tcp.invalid { server 127.0.0.1:19003; }\n'
                  ' server { listen 127.0.0.1:19004; proxy_pass tcp.invalid; }\n}\n')
        definitions, refs, audit = self.native(source)
        self.assertEqual({'web.invalid', 'tcp.invalid'}, {ref['to'] for ref in refs})
        self.assertEqual(3, audit[0]['classification']['declarations'])
        self.assertEqual(2, audit[0]['classification']['native_local_sites'])
        self.assertEqual(2, len(audit[0]['negative_controls']))
        self.assertTrue(any(d['name'] == 'unused.invalid' for d in definitions))

    def test_native_pair_survives_an_independent_scanner_omission(self):
        from unittest.mock import patch
        import nginx_oracle
        original = nginx_oracle.FileWalk.__init__
        def omitted(instance, *args, **kwargs):
            original(instance, *args, **kwargs)
            instance.refs = []
        source = ('events {}\nhttp {\n upstream retained.invalid { server 127.0.0.1:19001; }\n'
                  ' server { listen 127.0.0.1:19000; location / { proxy_pass http://retained.invalid; } }\n}\n')
        with patch.object(nginx_oracle.FileWalk, '__init__', omitted):
            _, refs, audit = self.native(source)
        self.assertEqual(1, len(refs))
        self.assertEqual(1, len(audit[0]['native_only_pairs']))
        self.assertEqual([], audit[0]['scanner_only_pairs'])
