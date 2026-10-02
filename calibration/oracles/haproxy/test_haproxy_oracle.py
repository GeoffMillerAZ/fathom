"""Tests of the compose-project route rule (LB-F169): a compose project is a
directory, a route joins the nearest project that declares its host, one pair
per compose file of that directory that declares the service."""

import os
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import haproxy_oracle as oracle  # noqa: E402


class ComposeProjectRule(unittest.TestCase):
    def setUp(self):
        self.root = tempfile.mkdtemp(prefix="oracle-test-")
        for rel in ("docker-compose.yml", "docker-compose.override.yml", "docker-compose.prod.yml",
                    "app/compose.yaml", "app/conf/x.cfg", "other/notes.yml", "caddy/c.cfg"):
            full = os.path.join(self.root, rel)
            os.makedirs(os.path.dirname(full), exist_ok=True)
            open(full, "w").close()
        # what `docker compose config` would read from each file
        self.names = {
            "docker-compose.yml": {"web": {"web"}, "db": {"db"}},
            "docker-compose.override.yml": {"web": {"web"}},
            "docker-compose.prod.yml": {"web": {"web"}, "cache": {"cache"}},
            "app/compose.yaml": {"web": {"web"}},
        }

    def names_of(self, cand):
        return self.names.get(cand, {})

    def lookup(self, rel):
        return lambda host: oracle.resolve_compose(self.root, rel, host, self.names_of)

    def test_compose_files_are_the_named_files_of_one_directory(self):
        self.assertEqual(oracle.compose_files(self.root, ""),
                         ["docker-compose.override.yml", "docker-compose.prod.yml", "docker-compose.yml"])
        self.assertEqual(oracle.compose_files(self.root, "app"), ["app/compose.yaml"])
        self.assertEqual(oracle.compose_files(self.root, "other"), [])

    def test_a_service_in_override_files_is_one_pair_per_declaring_file(self):
        svc, files, reason = oracle.classify("web:80", self.lookup("caddy/c.cfg"))
        self.assertEqual((svc, reason), ("web", ""))
        self.assertEqual(files, ["docker-compose.override.yml", "docker-compose.prod.yml", "docker-compose.yml"])
        svc, files, reason = oracle.classify("db:5432", self.lookup("caddy/c.cfg"))
        self.assertEqual((svc, files, reason), ("db", ["docker-compose.yml"], ""))

    def test_the_nearest_project_wins(self):
        svc, files, reason = oracle.classify("web:80", self.lookup("app/conf/x.cfg"))
        self.assertEqual((svc, files, reason), ("web", ["app/compose.yaml"], ""))
        # db is not declared in app/: the ancestor project declares it
        svc, files, reason = oracle.classify("db:5432", self.lookup("app/conf/x.cfg"))
        self.assertEqual((svc, files, reason), ("db", ["docker-compose.yml"], ""))

    def test_undeclared_and_external(self):
        self.assertEqual(oracle.classify("nothing:80", self.lookup("caddy/c.cfg")), (None, [], "undeclared"))
        self.assertEqual(oracle.classify("api.example.com:443", self.lookup("caddy/c.cfg")), (None, [], "external"))

    def test_one_name_for_two_services_is_ambiguous(self):
        self.names["docker-compose.yml"] = {"alias": {"a", "b"}}
        self.assertEqual(oracle.classify("alias:1", self.lookup("caddy/c.cfg")), (None, [], "ambiguous"))


if __name__ == "__main__":
    unittest.main()
