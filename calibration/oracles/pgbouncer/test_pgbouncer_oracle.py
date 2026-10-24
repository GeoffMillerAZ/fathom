"""Tests of the oracle's own scanner and claim rule (LB-Q-pgbouncer)."""

import os
import sys
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import pgbouncer_oracle as oracle  # noqa: E402


class Scanner(unittest.TestCase):
    def test_params_quote_and_spaces(self):
        got = oracle.params("host=db dbname='a ''b''' pool_size = 5 stray")
        self.assertEqual(got, [("host", "db"), ("dbname", "a 'b'"), ("pool_size", "5")])

    def test_scan_sections_lines_and_wildcard(self):
        src = "; c\n[databases]\napp = host=h auth_dbname=auth\nauth = host=h\n* = host=f\n[pgbouncer]\npool_mode = session\n"
        sec = oracle.scan(src)
        self.assertEqual([e[:2] for e in sec["databases"]["entries"]], [("app", 3), ("auth", 4), ("*", 5)])
        self.assertEqual((sec["databases"]["header"], sec["databases"]["last"]), (2, 5))
        self.assertEqual(sec["pgbouncer"]["entries"], [("pool_mode", 7, "session")])

    def test_claim(self):
        pgb = "[databases]\na = host=h\n"
        self.assertTrue(oracle.claimed("etc/pgbouncer.ini", pgb))
        self.assertTrue(oracle.claimed("test/test.ini", pgb))
        self.assertFalse(oracle.claimed("tox.ini", "[tox]\nenvlist = py3\n"))
        self.assertFalse(oracle.claimed("etc/pgbouncer.ini", "name = x\n"))

    def test_sanitized_drops_machine_settings(self):
        out = oracle.sanitized("[pgbouncer]\nlogfile = /x\npool_mode = session\nclient_tls_key_file = k\n%include a.ini\n")
        self.assertIn("pool_mode", out)
        for gone in ("logfile", "client_tls_key_file", "%include"):
            self.assertNotIn(gone, out)


if __name__ == "__main__":
    unittest.main()
