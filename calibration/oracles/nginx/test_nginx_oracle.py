"""Tests of the compose-project route rule (LB-F169): a compose project is a
directory, a route joins the nearest project that declares its host, one pair
per compose file of that directory that declares the service."""

import os
import shutil
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import nginx_oracle as oracle  # noqa: E402


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
        svc, files, reason = oracle.classify_route("web:80", self.lookup("caddy/c.cfg"))
        self.assertEqual((svc, reason), ("web", ""))
        self.assertEqual(files, ["docker-compose.override.yml", "docker-compose.prod.yml", "docker-compose.yml"])
        svc, files, reason = oracle.classify_route("db:5432", self.lookup("caddy/c.cfg"))
        self.assertEqual((svc, files, reason), ("db", ["docker-compose.yml"], ""))

    def test_the_nearest_project_wins(self):
        svc, files, reason = oracle.classify_route("web:80", self.lookup("app/conf/x.cfg"))
        self.assertEqual((svc, files, reason), ("web", ["app/compose.yaml"], ""))
        # db is not declared in app/: the ancestor project declares it
        svc, files, reason = oracle.classify_route("db:5432", self.lookup("app/conf/x.cfg"))
        self.assertEqual((svc, files, reason), ("db", ["docker-compose.yml"], ""))

    def test_undeclared_and_external(self):
        self.assertEqual(oracle.classify_route("nothing:80", self.lookup("caddy/c.cfg")), (None, [], "undeclared"))
        self.assertEqual(oracle.classify_route("api.example.com:443", self.lookup("caddy/c.cfg")), (None, [], "external"))

    def test_one_name_for_two_services_is_ambiguous(self):
        self.names["docker-compose.yml"] = {"alias": {"a", "b"}}
        self.assertEqual(oracle.classify_route("alias:1", self.lookup("caddy/c.cfg")), (None, [], "ambiguous"))


DUMP = """# configuration file /scratch/p/nginx.conf:
events {}
http {
    include /scratch/p/conf.d/a.conf;
}
# configuration file /scratch/p/conf.d/a.conf:
server { listen 80; }
"""


class AttestationHelpers(unittest.TestCase):
    def test_split_dump_keys_files_by_their_path(self):
        got = oracle.split_dump(DUMP)
        self.assertEqual(sorted(got), ["/scratch/p/conf.d/a.conf", "/scratch/p/nginx.conf"])
        self.assertIn("include /scratch/p/conf.d/a.conf;", got["/scratch/p/nginx.conf"])
        self.assertEqual(got["/scratch/p/conf.d/a.conf"].strip(), "server { listen 80; }")

    def write(self, text):
        d = tempfile.mkdtemp(prefix="oracle-test-")
        with open(os.path.join(d, "x.conf"), "w") as f:
            f.write(text)
        return d

    def test_shape_ignores_rewritten_absolute_paths_and_lines(self):
        a = self.write("events {}\nhttp {\n    include /etc/nginx/mime.types;\n    access_log /var/log/nginx/a.log;\n}\n")
        b = self.write("\n\nevents {}\nhttp {\n    include /scratch/p/mime.types;\n    access_log /scratch/p/logs/a.log;\n}\n")
        pa, _ = oracle.parse(a, "x.conf")
        pb, _ = oracle.parse(b, "x.conf")
        self.assertEqual(oracle.shape(pa), oracle.shape(pb))

    def test_shape_sees_a_different_directive_or_argument(self):
        a = self.write("server { listen 80; server_name example.com; }\n")
        b = self.write("server { listen 80; server_name example.org; }\n")
        c = self.write("server { listen 80; }\n")
        pa, _ = oracle.parse(a, "x.conf")
        self.assertNotEqual(oracle.shape(pa), oracle.shape(oracle.parse(b, "x.conf")[0]))
        self.assertNotEqual(oracle.shape(pa), oracle.shape(oracle.parse(c, "x.conf")[0]))


@unittest.skipUnless(shutil.which("nginx") or os.environ.get("NGINX_BIN"), "no nginx binary")
class AttestAgainstNginx(unittest.TestCase):
    """nginx -T loads a file in the first context it is valid in."""

    def setUp(self):
        self.nginx = os.environ.get("NGINX_BIN") or shutil.which("nginx")
        self.root = tempfile.mkdtemp(prefix="oracle-test-")
        self.scratch = tempfile.mkdtemp(prefix="oracle-test-")
        for rel, text in {
            "http.conf": "server { listen 127.0.0.1:8081; server_name example.com; }\n",
            "loc.conf": "internal;\n",
            "dir.conf": "gzip on;\n",
            "srv.conf": "location /x { return 200 ok; }\n",
            "bad.conf": "server { listen 127.0.0.1:8082; location / { proxy_pass http://backend.invalid; } }\n",
            "broken.conf": "server { listen 80 }\n",
        }.items():
            with open(os.path.join(self.root, rel), "w") as f:
                f.write(text)

    def test_contexts(self):
        got = {}
        for rel in ("http.conf", "srv.conf", "loc.conf", "dir.conf", "bad.conf"):
            ok, read, msg, dump, ctx = oracle.attest(self.root, rel, self.nginx, self.scratch)
            got[rel] = (ok, ctx)
        self.assertEqual(got["http.conf"], (True, "http"))
        self.assertEqual(got["srv.conf"], (True, "server"))
        self.assertEqual(got["loc.conf"], (True, "location"))
        self.assertEqual(got["dir.conf"][0], True)
        self.assertEqual(got["bad.conf"][0], False)

    def test_a_prefix_variable_loads_a_fragment_inside_the_prefix_main_configuration(self):
        root = tempfile.mkdtemp(prefix="oracle-test-")
        with open(os.path.join(root, "nginx.conf"), "w") as f:
            f.write("events {}\nhttp {\n    map $http_x $v { default 1; }\n}\n")
        with open(os.path.join(root, "frag.conf"), "w") as f:
            f.write("add_header X $v;\n")
        ok, read, msg, dump, ctx = oracle.attest(root, "frag.conf", self.nginx, self.scratch)
        self.assertEqual((ok, ctx), (True, "in-http"), msg)


if __name__ == "__main__":
    unittest.main()
