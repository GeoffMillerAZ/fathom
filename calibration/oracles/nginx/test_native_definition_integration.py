import os
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch


@unittest.skipUnless(os.environ.get('NGINX_TOKENS'), 'set NGINX_TOKENS for native definition controls')
class NativeDefinitionControl(unittest.TestCase):
    def measured(self):
        from native_population import measure
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            subject = root / 'subject'; subject.mkdir()
            (subject / 'nginx.conf').write_text('events {}\nhttp {\n upstream alpha.invalid { server 127.0.0.1:19001; }\n server { listen 127.0.0.1:19000; server_name a.invalid; location / { proxy_pass http://alpha.invalid; } }\n}\n')
            return measure(subject, ['nginx.conf'], Path(os.environ['NGINX_TOKENS']), root / 'work', native_definitions=True)

    def test_native_definitions_and_owners_survive_independent_walk_omission(self):
        import nginx_oracle
        with patch.object(nginx_oracle.FileWalk, 'walk', lambda *args, **kwargs: None):
            definitions, references, evidence = self.measured()
        self.assertEqual(5, len(definitions))
        self.assertEqual([dict(path='nginx.conf', **{'from':'a.invalid /','to':'alpha.invalid'})], references)
        self.assertEqual(1, len(evidence[0]['native_only_pairs']))
        self.assertIn(dict(path='nginx.conf',name='nginx.conf',kind='package',line=1,end=5),definitions)

    def test_all_native_definition_ranges_have_exact_ends(self):
        definitions, _, evidence = self.measured()
        self.assertTrue(all(row['end'] >= row['line'] for row in definitions))
        self.assertEqual(5, evidence[0]['native_positions']['native_target_file_end'])
        self.assertEqual({3,4,5}, {row['end'] for row in definitions})
