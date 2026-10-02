"""nginx_oracle.py - the language-breadth harness's oracle for the nginx native
dialect (design section 1.3, 13.4 step 19; LB-Q-nginx).

Run it with the interpreter of a venv holding requirements.txt (crossplane):

    <venv>/bin/python nginx_oracle.py -subject <checkout> -out <file> \
        [-routes-out <file>] [-nginx <nginx binary> -scratch <dir> [-files-out <file>]] [-summary-out <file>]

Tier B (-out), the relabel of crossplane from C (wave ox), and only when -nginx
cross-checks it; a run without -nginx writes tier C. The written reason: on the
ladder in calibration/oracles/envoy/README.md (the reference parser is A, a
widely used third-party parser tool is B, an independent library is C),
crossplane is NGINX Inc's parser tool for the format: nginxinc crossplane 0.5.8
(Apache-2.0) emits nginx's own directive tree with its lines. Design 1.5 D2 also
places an independent parser's syntax dump at B (php-parser, luaparse), where
1.3's table lists an independent parser library under C: this oracle is the
tool and the dump, so B is the definition it meets. It stays below A because
crossplane reads text, it is not nginx's configuration loader: a file is in
scope only where nginx itself loads it and crossplane reads the same directives
from nginx's own dump of it (the cross-check below); a disagreement is excluded
and counted, never trusted. Tier B proves L1 excellent and L2 and L3 good
(langbench.tierCeiling). Every file the
dialect could claim is parsed alone (`crossplane.parse(path, single=True,
check_ctx=False, check_args=False)`: a conf.d fragment is no main context): a
file named nginx.conf, any *.conf, and any file under a sites-enabled/ or
sites-available/ directory; one crossplane rejects (a lexer or brace error) is
counted and left out. Emitted, under the dialect's naming contract (its
PROVENANCE.md):

  - definitions: a `package` per file (its base name, line 1); a `service` per
    server block (its first non-empty server_name, else its first listen
    argument, else "server") and per upstream (its name); a `route` per location
    (its arguments joined by a space, qualified by its server's name and a
    space); a `target` per server of an upstream ("<upstream> <address>").
    Children of map, geo, types, split_clients, charset_map and match blocks are
    data, never entities. crossplane writes no end line, so none is given.
  - references: a proxy_pass, grpc_pass, fastcgi_pass, uwsgi_pass, scgi_pass or
    memcached_pass whose host, written without a port, names an upstream of the
    same file, from its location (else its server, else the file).
  - cross_references (file granularity): an include, resolved as nginx resolves
    it against the configuration prefix: the directory of the nearest nginx.conf
    at or above the including file (a file with none has no prefix and is not
    attributed), an absolute path under /etc/nginx/ (or /usr/local/nginx/conf/,
    /usr/local/etc/nginx/, /opt/homebrew/etc/nginx/) read against that prefix
    (the directory the configuration is installed into), a glob expanded as
    glob(3) does (no leading-dot match); each match that is a file of the
    subject, from the include's location (else server, else upstream, else the
    file's own entity).

Cross-check by nginx itself (-nginx, -scratch; nginx 1.31.6 from brew): `nginx
-T` on a scratch copy of the prefix directory (absolute /etc/nginx/ paths
rewritten to the copy, -e to a scratch error log) validates the configuration
and dumps every file it read ("# configuration file <path>:"). Each nginx.conf
is loaded as the main configuration (a fragment named nginx.conf inside an
http wrapper); every other file nginx has not already read is loaded in the
first context it is valid in: as the main configuration when it holds events or
http, else included inside http, inside a server, inside a location, then the
same three inside its prefix's own nginx.conf (so the maps, upstreams and
variables the prefix defines are in scope). A file no context loads (a host that
does not resolve here, "host not found in upstream": a compose service name; a
certificate or module the repository does not hold; an NGINX Plus directive) is
not in scope. For each file nginx loaded, crossplane parses nginx's dump of it
and the result is compared with its parse of the file (directives, arguments
and blocks; lines, and absolute paths nginx's copy rewrote, ignored): a
difference is excluded. -files-out lists the files in scope, for lang-oracle's
-only; the definitions, references and cross_references are those files' (an
include whose target no nginx run read is left out). stderr and -summary-out
count each exclusion and the include closure agreement of each nginx.conf.

Route targets (-routes-out, bench-only; lang-oracle's tool-oracle decoder
refuses unknown fields, so they are a file of their own): every *_pass target
whose host names no upstream of the configuration (the file and every file
under the same prefix), or names one with a port, and every upstream server's
address, resolved against the nearest compose file at or above the nginx file
(`docker compose config --format json`, Docker Compose 5.1.4, tier A for the
service set): a service name (or a container_name, hostname or network alias
of exactly one service) is a route_reference to that service; otherwise a
route_unresolved with its reason: variable ($ or {), unix-socket, ip-literal
(an IP address or localhost), ambiguous (more than one service), external (a
dotted host no service names) or undeclared (a bare name no service names, or
no compose file).

Directories the connector never reads (testdata, vendor, node_modules,
generated, .git and the editor directories) are not walked. Bench-only; never a
product dependency.
"""

import argparse
import glob
import ipaddress
import json
import os
import posixpath
import re
import shutil
import subprocess
import sys

import crossplane

SKIP_DIRS = {"vendor", ".git", "testdata", "generated", "node_modules", ".idea", ".vscode", ".claude"}
DATA_BLOCKS = {"map", "geo", "types", "split_clients", "charset_map", "match"}
PASS = {"proxy_pass", "grpc_pass", "fastcgi_pass", "uwsgi_pass", "scgi_pass", "memcached_pass"}
PREFIXES = ["/etc/nginx/", "/usr/local/nginx/conf/", "/usr/local/etc/nginx/", "/opt/homebrew/etc/nginx/"]


def candidates(root):
    out = []
    for d, dirs, files in os.walk(root):
        dirs[:] = sorted(x for x in dirs if x not in SKIP_DIRS)
        rel_dir = os.path.relpath(d, root)
        parent = os.path.basename(d)
        for f in sorted(files):
            rel = f if rel_dir == "." else posixpath.join(rel_dir.replace(os.sep, "/"), f)
            if f == "nginx.conf" or f.endswith(".conf") or parent in ("sites-enabled", "sites-available"):
                out.append(rel)
    return out


def parse(root, rel):
    p = crossplane.parse(os.path.join(root, rel), single=True, check_ctx=False, check_args=False,
                         catch_errors=True, comments=False)
    cfg = p["config"][0]
    if p["status"] != "ok" or cfg["errors"]:
        return None, "; ".join(e.get("error", "") for e in (cfg["errors"] or p["errors"]))
    return cfg["parsed"], ""


def pass_host(arg):
    s = arg
    if "://" in s:
        s = s.split("://", 1)[1]
    if s.startswith("unix:"):
        return "", False
    s = re.split(r"[/?]", s, maxsplit=1)[0]
    if s.startswith("["):
        i = s.find("]")
        if i >= 0:
            return s[: i + 1], s[i + 1:].startswith(":")
        return s, False
    if ":" in s:
        return s.rsplit(":", 1)[0], True
    return s, False


def server_name(block):
    for d in block:
        if d["directive"] == "server_name":
            for a in d["args"]:
                if a != "":
                    return a
    for d in block:
        if d["directive"] == "listen" and d["args"]:
            return d["args"][0]
    return "server"


def qualified(q, name):
    return name if not q else q + " " + name


def upstream_names(parsed):
    names = set()

    def walk(ds):
        for d in ds:
            if d["directive"] == "upstream" and "block" in d and d["args"]:
                names.add(d["args"][0])
            if "block" in d and d["directive"] not in DATA_BLOCKS:
                walk(d["block"])
    walk(parsed)
    return names


class FileWalk:
    """One file's definitions, references, includes and route targets."""

    def __init__(self, rel, parsed):
        self.rel, self.root, self.parsed = rel, posixpath.basename(rel), parsed
        self.defs = [{"path": rel, "name": self.root, "line": 1, "kind": "package"}]
        self.refs, self.includes, self.routes = [], [], []
        self.upstreams = upstream_names(parsed)
        # A main configuration holds its servers inside http or stream; a
        # fragment (a conf.d file) holds them at its top.
        self.under_http = any(d["directive"] in ("http", "stream", "events") for d in parsed)
        self.walk(parsed, server="", owner=self.root, upstream="", data=False)

    def walk(self, ds, server, owner, upstream, data):
        for d in ds:
            name, args, line = d["directive"], d["args"], d["line"]
            block = d.get("block")
            if name == "include" and args:
                self.includes.append((owner, args[0], line))
            elif data:
                if block is not None:
                    self.walk(block, server, owner, upstream, True)
            elif block is not None and name in DATA_BLOCKS:
                self.walk(block, server, owner, upstream, True)
            elif block is not None and name == "server" and not upstream:
                sn = server_name(block)
                self.defs.append({"path": self.rel, "name": sn, "line": line, "kind": "service"})
                self.walk(block, sn, sn, "", False)
            elif block is not None and name == "location":
                ln = qualified(server, " ".join(args))
                self.defs.append({"path": self.rel, "name": ln, "line": line, "kind": "route"})
                self.walk(block, server, ln, upstream, False)
            elif block is not None and name == "upstream" and args:
                self.defs.append({"path": self.rel, "name": args[0], "line": line, "kind": "service"})
                self.walk(block, server, args[0], args[0], False)
            elif block is None and name == "server" and upstream and args:
                self.defs.append({"path": self.rel, "name": qualified(upstream, args[0]), "line": line, "kind": "target"})
                self.routes.append((upstream, args[0], line))
            elif block is None and name in PASS and args:
                host, port = pass_host(args[0])
                if not port and host in self.upstreams:
                    self.refs.append({"path": self.rel, "from": owner, "to": host})
                else:
                    self.routes.append((owner, args[0], line))
            elif block is not None:
                self.walk(block, server, owner, upstream, False)


def prefix_of(root, rel):
    d = posixpath.dirname(rel)
    while True:
        cand = posixpath.join(d, "nginx.conf") if d else "nginx.conf"
        if os.path.isfile(os.path.join(root, cand)):
            return d
        if not d:
            return None
        d = posixpath.dirname(d)


def resolve_include(root, prefix, raw):
    p = raw
    if p.startswith("/"):
        for pre in PREFIXES:
            if p.startswith(pre):
                p = p[len(pre):]
                break
        else:
            return []
    pattern = os.path.join(root, prefix, p) if prefix else os.path.join(root, p)
    out = []
    for m in sorted(glob.glob(pattern)):
        if os.path.isfile(m):
            rel = os.path.relpath(m, root).replace(os.sep, "/")
            if not rel.startswith("../"):
                out.append(rel)
    return out


def split_dump(stdout):
    """`nginx -T`'s dump as {path: text}: each file it read, introduced by
    "# configuration file <path>:"."""
    parts = re.split(r"^# configuration file (.+):\n", stdout, flags=re.M)
    return {parts[i]: parts[i + 1] for i in range(1, len(parts) - 1, 2)}


def shape(parsed):
    """A parse tree without its lines and absolute paths: nginx -T reads a
    scratch copy whose /etc/nginx/, pid and log paths are rewritten into the
    copy, so an absolute argument is compared as such, never by its value."""
    out = []
    for d in parsed:
        args = tuple("<abs>" if a.startswith("/") else a for a in d["args"])
        out.append((d["directive"], args, shape(d["block"]) if "block" in d else None))
    return out


WRAPPERS = {
    "http": "events {}\nhttp {\n    include %s;\n}\n",
    "server": "events {}\nhttp {\n    server {\n        listen 127.0.0.1:18080;\n        include %s;\n    }\n}\n",
    "location": "events {}\nhttp {\n    server {\n        listen 127.0.0.1:18080;\n        location / {\n            include %s;\n        }\n    }\n}\n",
}


def nginx_check(root, prefix, nginx, scratch, mode, target):
    """Run nginx -T on a scratch copy of the prefix directory; return
    (ok, files read, message, {file: text nginx dumped}).

    mode "main" loads target as the main configuration (-c); "http", "server"
    and "location" load it from a wrapper main configuration, `include`d inside
    that context. Absolute configuration paths (/etc/nginx/ and the other
    prefixes) and the pid and log directories (/var/run/, /run/, /var/log/) are
    rewritten into the copy, which also gets a logs/ directory (nginx's default
    pid and log place)."""
    src = os.path.join(root, prefix) if prefix else root
    tag = re.sub(r"[^A-Za-z0-9._-]", "_", "%s__%s__%s" % (prefix or "_root", mode, target))
    dst = os.path.join(scratch, "nginx-T", tag)
    shutil.rmtree(dst, ignore_errors=True)
    shutil.copytree(src, dst, ignore=shutil.ignore_patterns(".git"), symlinks=False)
    os.makedirs(os.path.join(dst, "logs"), exist_ok=True)
    rewrite = [(pre, dst + "/") for pre in PREFIXES] + [(d, dst + "/logs/") for d in ("/var/run/", "/run/", "/var/log/nginx/", "/var/log/")]
    for d, _, files in os.walk(dst):
        for f in files:
            fp = os.path.join(d, f)
            try:
                with open(fp, encoding="utf-8") as fh:
                    text = fh.read()
            except (UnicodeDecodeError, OSError):
                continue
            new = text
            for old, repl in rewrite:
                new = new.replace(old, repl)
            if new != text:
                with open(fp, "w", encoding="utf-8") as fh:
                    fh.write(new)
    rel_target = posixpath.relpath(target, prefix) if prefix else target
    main = os.path.join(dst, rel_target)
    if mode in ("http", "server", "location"):
        main = os.path.join(dst, "_wrapper.conf")
        with open(main, "w") as f:
            f.write(WRAPPERS[mode] % rel_target)
    elif mode.startswith("in-"):
        # the prefix's own main configuration with the file included inside
        # its http block (in-http), inside a new server (in-server) or a new
        # location (in-location): the maps, upstreams and variables the prefix
        # defines are then in scope for the fragment
        main_rel = posixpath.join(prefix, "nginx.conf") if prefix else "nginx.conf"
        tree, err = parse(root, main_rel)
        if tree is None:
            return False, set(), "no readable nginx.conf: " + err[:100], {}
        inc = {"directive": "include", "args": [rel_target], "line": 1}
        if mode == "in-server":
            inc = {"directive": "server", "args": [], "line": 1, "block": [{"directive": "listen", "args": ["127.0.0.1:18080"], "line": 1}, inc]}
        elif mode == "in-location":
            inc = {"directive": "server", "args": [], "line": 1, "block": [
                {"directive": "listen", "args": ["127.0.0.1:18080"], "line": 1},
                {"directive": "location", "args": ["/"], "line": 1, "block": [inc]}]}
        for d in tree:
            if d["directive"] == "http" and "block" in d:
                d["block"].append(inc)
                break
        else:
            return False, set(), "nginx.conf has no http block", {}
        text = crossplane.build(tree)
        for old, repl in rewrite:
            text = text.replace(old, repl)
        main = os.path.join(dst, "_wrapper.conf")
        with open(main, "w") as f:
            f.write(text)
    err = os.path.join(dst, "logs", "error.log")
    r = subprocess.run([nginx, "-T", "-p", dst + "/", "-c", main, "-e", err],
                       capture_output=True, text=True, timeout=60)
    dump = {}
    for path, text in split_dump(r.stdout).items():
        if path.startswith(dst + "/"):
            rel = path[len(dst) + 1:]
            if posixpath.basename(rel) != "_wrapper.conf":
                dump[posixpath.join(prefix, rel) if prefix else rel] = text
    msg = ""
    if r.returncode != 0:
        lines = [x for x in r.stderr.splitlines() if "[emerg]" in x]
        msg = (lines[-1] if lines else r.stderr.strip()[-200:]).replace(dst + "/", "")
    shutil.rmtree(dst, ignore_errors=True)
    return r.returncode == 0, set(dump), msg, dump


def attest(root, rel, nginx, scratch):
    """Load rel with nginx -T in the first context it is valid in: as the main
    configuration when it holds events or http, else included inside http, then
    inside a server, then inside a location. Returns (ok, files read, message,
    dump, context); a file no context loads is not attested."""
    prefix = prefix_of(root, rel)
    if prefix is None:
        prefix = posixpath.dirname(rel)
    parsed, err = parse(root, rel)
    if parsed is None:
        return False, set(), err, {}, ""
    top = {d["directive"] for d in parsed}
    modes = (["main"] if top & {"events", "http", "stream"} else []) + ["http", "server", "location"]
    if posixpath.basename(rel) != "nginx.conf" and os.path.isfile(os.path.join(root, prefix, "nginx.conf")):
        modes += ["in-http", "in-server", "in-location"]
    msg = ""
    for mode in modes:
        ok, read, m, dump = nginx_check(root, prefix, nginx, scratch, mode, rel)
        if ok:
            return True, read, "", dump, mode
        msg = msg or m
    return False, set(), msg, {}, ""


def compose_names(root, cand, cache):
    """host name -> service names of one compose file, as `docker compose config`
    reads it (a version-1 file it refuses is read with PyYAML, tier C)."""
    if cand in cache:
        return cache[cand]
    r = subprocess.run(["docker", "compose", "-f", os.path.join(root, cand), "config", "--format", "json",
                        "--no-interpolate", "--no-path-resolution", "--no-consistency", "--no-normalize"],
                       capture_output=True, text=True, timeout=60)
    names = {}
    if r.returncode == 0:
        for svc, spec in json.loads(r.stdout).get("services", {}).items():
            names.setdefault(svc, set()).add(svc)
            for alias in [spec.get("container_name"), spec.get("hostname")]:
                if alias:
                    names.setdefault(alias, set()).add(svc)
            nets = spec.get("networks") or {}
            if isinstance(nets, dict):
                for n in nets.values():
                    for alias in (n or {}).get("aliases") or []:
                        names.setdefault(alias, set()).add(svc)
    else:
        names = compose_v1_services(os.path.join(root, cand))
        print(f"nginx-oracle: docker compose config {cand} refused it ({r.stderr.strip()[:120]}); "
              f"read as a version-1 file: {len(names)} names", file=sys.stderr)
    cache[cand] = names
    return names


COMPOSE_NAME = re.compile(r"^(docker-)?compose([.-].+)?\.ya?ml$", re.I)


def compose_files(root, d):
    """The compose-named files of one directory (compose.y[a]ml, docker-compose.y[a]ml
    and their override or environment variants): a compose project is a directory."""
    try:
        names = sorted(os.listdir(os.path.join(root, d) if d else root))
    except OSError:
        return []
    return [posixpath.join(d, n) if d else n for n in names
            if COMPOSE_NAME.match(n) and os.path.isfile(os.path.join(root, d, n))]


def resolve_compose(root, rel, host, names_of):
    """The nearest compose project that declares host: the directory of the
    configuration, then each ancestor. Every compose file of that directory that
    declares the name is one hit (an override file is the same service, compose
    merges them): a list of (file, set of service names)."""
    d = posixpath.dirname(rel)
    while True:
        hits = []
        for cand in compose_files(root, d):
            svcs = names_of(cand).get(host)
            if svcs:
                hits.append((cand, svcs))
        if hits:
            return hits
        if not d:
            return []
        d = posixpath.dirname(d)


def route_hits(hits):
    """(service, [files], reason) for a host's compose hits: one service (the
    same name in every file) is a pair per file; several services behind one
    name are ambiguous."""
    svcs = set()
    for _, s in hits:
        svcs |= set(s)
    if len(svcs) == 1:
        return next(iter(svcs)), [f for f, _ in hits], ""
    if len(svcs) > 1:
        return None, [], "ambiguous"
    return None, [], ""


def compose_v1_services(path):
    """A version-1 compose file (no services: key, each top-level key a
    service), which Docker Compose 5 no longer accepts: PyYAML 6.0.3 reads
    it (tier C); a service's container_name, hostname and links aliases name
    it too."""
    import yaml
    try:
        with open(path) as f:
            doc = yaml.safe_load(f)
    except Exception:  # noqa: BLE001 - a file YAML cannot read names nothing
        return {}
    names = {}
    if not isinstance(doc, dict) or "services" in doc:
        return names
    for svc, spec in doc.items():
        if svc in ("version", "networks", "volumes") or not isinstance(spec, dict):
            continue
        names.setdefault(svc, set()).add(svc)
        for alias in [spec.get("container_name"), spec.get("hostname")]:
            if alias:
                names.setdefault(str(alias), set()).add(svc)
    return names


def route_host(raw):
    s = raw
    if "://" in s:
        s = s.split("://", 1)[1]
    s = re.split(r"[/?]", s, maxsplit=1)[0]
    if s.startswith("["):
        return s[1:s.find("]")] if "]" in s else s
    return s.rsplit(":", 1)[0] if ":" in s else s


def classify_route(raw, lookup):
    """(service, files, reason) of a proxy_pass address; lookup(host) is the
    nearest compose project's hits."""
    if "$" in raw or "{" in raw:
        return None, [], "variable"
    if "unix:" in raw or "unix@" in raw:
        return None, [], "unix-socket"
    host = route_host(raw)
    try:
        ipaddress.ip_address(host)
        return None, [], "ip-literal"
    except ValueError:
        pass
    if host == "localhost":
        return None, [], "ip-literal"
    svc, files, reason = route_hits(lookup(host))
    if svc or reason:
        return svc, files, reason
    return None, [], "external" if "." in host else "undeclared"


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("-subject", required=True)
    ap.add_argument("-out", required=True)
    ap.add_argument("-routes-out")
    ap.add_argument("-nginx")
    ap.add_argument("-scratch")
    ap.add_argument("-summary-out")
    ap.add_argument("-files-out")
    a = ap.parse_args()
    root = os.path.abspath(a.subject)

    walks, failed = {}, {}
    for rel in candidates(root):
        parsed, err = parse(root, rel)
        if parsed is None:
            failed[rel] = err
            continue
        walks[rel] = FileWalk(rel, parsed)

    defs, refs, xrefs = [], [], []
    graph = {}
    for rel, w in sorted(walks.items()):
        defs.extend(w.defs)
        refs.extend(w.refs)
        prefix = prefix_of(root, rel)
        if prefix is None:
            continue
        for owner, raw, _ in w.includes:
            for t in resolve_include(root, prefix, raw):
                xrefs.append({"path": rel, "from": owner, "to_path": t})
                graph.setdefault(rel, set()).add(t)

    summary = {"files": len(walks), "parse_failures": failed, "nginx_T": {}}
    attested = None
    if a.nginx and a.scratch:
        os.makedirs(a.scratch, exist_ok=True)
        dumps = {}  # file -> the text nginx -T dumped for it
        for rel in sorted(walks):
            if posixpath.basename(rel) != "nginx.conf":
                continue
            prefix = posixpath.dirname(rel)
            closure, todo = {rel}, [rel]
            while todo:
                for t in graph.get(todo.pop(), ()):
                    if t not in closure:
                        closure.add(t)
                        todo.append(t)
            fragment = any(d["name"] != "nginx.conf" and d["kind"] in ("service", "route") and
                           posixpath.basename(d["path"]) == "nginx.conf" and d["path"] == rel and
                           not walks[rel].under_http for d in walks[rel].defs)
            ok, read, msg, dump = nginx_check(root, prefix, a.nginx, a.scratch, "http" if fragment else "main", rel)
            entry = {"passed": ok}
            if ok:
                entry["oracle_closure"] = len(closure)
                entry["nginx_read"] = len(read)
                entry["agree"] = len(closure & read)
                entry["oracle_only"] = sorted(closure - read)
                entry["nginx_only"] = sorted(read - closure)
                for r in read:
                    dumps.setdefault(r, dump[r])
            else:
                entry["error"] = msg
            summary["nginx_T"][rel] = entry
            print(f"nginx-oracle: nginx -T {rel}: {json.dumps(entry)}", file=sys.stderr)

        # Tier B: a file is in scope when nginx -T loads it (in the main
        # configuration of its prefix, or alone in the first context it is valid
        # in) and crossplane parses the same directives from nginx's dump of it
        # as from the file. A file nginx does not load, or whose dump parses
        # differently, is left out and counted.
        not_loaded, disagree = {}, {}
        for rel in sorted(walks):
            if rel not in dumps:
                ok, read, msg, dump, _ = attest(root, rel, a.nginx, a.scratch)
                if not ok:
                    not_loaded[rel] = msg
                    continue
                for r in read:
                    dumps.setdefault(r, dump[r])
        os.makedirs(os.path.join(a.scratch, "dumps"), exist_ok=True)
        attested = set()
        for rel in sorted(walks):
            if rel not in dumps:
                continue
            dp = os.path.join(a.scratch, "dumps", re.sub(r"[^A-Za-z0-9._-]", "_", rel) + ".conf")
            with open(dp, "w") as fh:
                fh.write(dumps[rel])
            parsed, err = parse(os.path.dirname(dp), os.path.basename(dp))
            if parsed is None:
                disagree[rel] = "crossplane rejects nginx's dump: " + err[:120]
            elif shape(parsed) != shape(walks[rel].parsed):
                disagree[rel] = "crossplane reads different directives from nginx's dump"
            else:
                attested.add(rel)
        summary["attestation"] = {"in_scope": len(attested), "not_loaded": not_loaded, "dump_disagrees": disagree}
        print(f"nginx-oracle: nginx -T loaded {len(attested)} of {len(walks)} files and crossplane read the same "
              f"directives from its dump; {len(not_loaded)} not loaded by nginx, {len(disagree)} dump disagreements "
              f"(left out, counted)", file=sys.stderr)
        for rel, msg in sorted(not_loaded.items()):
            print(f"nginx-oracle: not loaded {rel}: {msg[:160]}", file=sys.stderr)
        for rel, msg in sorted(disagree.items()):
            print(f"nginx-oracle: dump disagrees {rel}: {msg}", file=sys.stderr)
        defs = [d for d in defs if d["path"] in attested]
        refs = [x for x in refs if x["path"] in attested]
        kept = [x for x in xrefs if x["path"] in attested and x["to_path"] in dumps]
        summary["attestation"]["xrefs_target_not_loaded"] = len([x for x in xrefs if x["path"] in attested]) - len(kept)
        xrefs = kept
        if a.files_out:
            with open(a.files_out, "w") as fh:
                fh.write("".join(x + "\n" for x in sorted(attested)))

    with open(a.out, "w") as f:
        json.dump({"tool": "crossplane", "version": crossplane.__version__, "tier": "B" if attested is not None else "C",
                   "definitions": defs, "references": refs, "cross_references": xrefs}, f, indent=1, sort_keys=True)
        f.write("\n")

    if a.routes_out:
        upstreams_by_prefix = {}
        for rel, w in walks.items():
            upstreams_by_prefix.setdefault(prefix_of(root, rel), set()).update(w.upstreams)
        resolved, unresolved, cache = [], [], {}
        for rel, w in sorted(walks.items()):
            if attested is not None and rel not in attested:
                continue
            config_upstreams = upstreams_by_prefix.get(prefix_of(root, rel), set()) | w.upstreams
            for owner, raw, _ in w.routes:
                host, port = pass_host(raw)
                if not port and host in config_upstreams:
                    continue  # an upstream of the configuration, declared in another file
                svc, files, reason = classify_route(raw, lambda h: resolve_compose(root, rel, h, lambda c: compose_names(root, c, cache)))
                if svc:
                    for compose in files:
                        resolved.append({"path": rel, "from": owner, "to_path": compose, "to": svc, "raw": raw})
                else:
                    unresolved.append({"path": rel, "from": owner, "raw": raw, "reason": reason})
        with open(a.routes_out, "w") as f:
            json.dump({"tool": "crossplane+docker-compose", "version": crossplane.__version__, "tier": "C",
                       "route_references": resolved, "route_unresolved": unresolved}, f, indent=1, sort_keys=True)
            f.write("\n")
        summary["routes"] = {"resolved": len(resolved), "unresolved": len(unresolved)}

    print(f"nginx-oracle: {len(walks)} files, {len(failed)} rejected, {len(defs)} definitions, "
          f"{len(refs)} references, {len(xrefs)} cross-references", file=sys.stderr)
    for rel, err in sorted(failed.items()):
        print(f"nginx-oracle: rejected {rel}: {err[:160]}", file=sys.stderr)
    if a.summary_out:
        with open(a.summary_out, "w") as f:
            json.dump(summary, f, indent=1, sort_keys=True)
            f.write("\n")


if __name__ == "__main__":
    main()
