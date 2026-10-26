import tempfile
import unittest
from pathlib import Path
from instrument_source import instrument


class NativeObserverPatchTest(unittest.TestCase):
    def source(self, root):
        for domain in ('http', 'stream'):
            file = root / 'src' / domain / ('ngx_' + domain + '_upstream.c')
            file.parent.mkdir(parents=True)
            file.write_text('ngx_' + domain + '_upstream_srv_conf_t *\nngx_' + domain +
                            '_upstream_add(ngx_conf_t *cf, ngx_url_t *u, ngx_uint_t flags)\n{\n    return native_result;\n}\n')

    def test_original_native_bodies_are_preserved_and_both_domains_observed(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory); self.source(root)
            receipt = instrument(root)
            self.assertEqual(2, len(receipt['sources']))
            for row in receipt['sources']:
                text = (root / row['path']).read_text()
                self.assertIn('{\n    return native_result;\n}', text)
                self.assertIn('ORACLE_NGINX_UPSTREAM_V1', text)
                self.assertIn('if (ngx_test_config)', text)
                self.assertNotEqual(row['original_sha256'], row['observed_sha256'])

    def test_double_instrumentation_is_refused(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory); self.source(root)
            instrument(root)
            with self.assertRaises(ValueError):
                instrument(root)
