"""Tests of the oracle's own libconfig scanner and claim rule (LB-Q-proxysql)."""

import os
import sys
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import proxysql_oracle as oracle  # noqa: E402


class Scanner(unittest.TestCase):
    def test_lines_and_values(self):
        src = 'datadir="/x"\nmysql_servers =\n(\n\t{ address="h" , port=3306 },\n\t{\n\t\taddress="i"\n\t}\n)\n'
        root = oracle.parse(src)
        ms = root.value["mysql_servers"]
        self.assertEqual((ms.line, ms.end, len(ms.value)), (2, 8, 2))
        self.assertEqual([(i.line, i.end) for i in ms.value], [(4, 4), (5, 7)])
        self.assertEqual(oracle.sval(ms.value[0], "port"), "3306")

    def test_refuses_what_libconfig_refuses(self):
        for bad in ("a = GET", "a = (1,", 'a = "open', "= 1"):
            with self.assertRaises((ValueError, IndexError)):
                oracle.parse(bad)

    def test_claim(self):
        cnf = 'datadir="/x"\n'
        self.assertTrue(oracle.claimed("etc/proxysql.cnf", cnf))
        self.assertTrue(oracle.claimed("t/proxysql01.cfg", cnf))
        self.assertFalse(oracle.claimed("etc/my.cnf", cnf))
        self.assertFalse(oracle.claimed("etc/proxysql.cnf", "[mysqld]\nport=1\n"))


if __name__ == "__main__":
    unittest.main()
