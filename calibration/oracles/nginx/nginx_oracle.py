"""nginx_oracle.py - the language-breadth harness's oracle for the nginx native
dialect (design section 1.3, 13.4 step 19; LB-Q-nginx).

Run it with the interpreter of a venv holding requirements.txt (crossplane):

    <venv>/bin/python nginx_oracle.py -subject <checkout> -out <file> \
        [-routes-out <file>] [-nginx <nginx binary> -scratch <dir>] [-summary-out <file>]

Tier C (-out): nginxinc crossplane 0.5.8 (Apache-2.0), NGINX's own Python parser,
an independent parser library (design 1.3 counts one as tier C). Every file the
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

Tier A cross-check (-nginx, -scratch): `nginx -T` (nginx 1.31.6) on a scratch
copy of each nginx.conf's prefix directory (absolute /etc/nginx/ paths
rewritten to the copy, -e to a scratch error log) validates the configuration
and dumps the files it read ("# configuration file <path>:"); where it passes,
the set is compared with the oracle's include closure from that nginx.conf
(stderr and -summary-out count agreement). It fails where a configuration
names a host that does not resolve here ("host not found in upstream": a
compose service name) or a file that is not in the repository (a certificate,
a log directory); the summary names each failure.

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
COMPOSE = ["compose.yaml", "compose.yml", "docker-compose.yml", "docker-compose.yaml"]


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
        self.rel, self.root = rel, posixpath.basename(rel)
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


def nginx_check(root, prefix, nginx, scratch, fragment):
    """Run nginx -T on a scratch copy of the prefix directory; return (ok, files read, message).

    Absolute configuration paths (/etc/nginx/ and the other prefixes) and the
    pid and log directories (/var/run/, /run/, /var/log/) are rewritten into the
    copy, which also gets a logs/ directory (nginx's default pid and log place).
    A fragment (a server, location or upstream at its top: a conf.d file named
    nginx.conf) is tested inside a wrapper main configuration,
    `events {} http { include nginx.conf; }`."""
    src = os.path.join(root, prefix) if prefix else root
    dst = os.path.join(scratch, "nginx-T", prefix.replace("/", "_") or "_root")
    shutil.rmtree(dst, ignore_errors=True)
    shutil.copytree(src, dst, ignore=shutil.ignore_patterns(".git"), symlinks=False)
    os.makedirs(os.path.join(dst, "logs"), exist_ok=True)
    rewrite = [(pre, dst + "/") for pre in PREFIXES] + [(d, dst + "/logs/") for d in ("/var/run/", "/run/", "/var/log/nginx/", "/var/log/")]
    for d, _, files in os.walk(dst):
        for f in files:
            fp = os.path.join(d, f)
            try:
                text = open(fp, encoding="utf-8").read()
            except (UnicodeDecodeError, OSError):
                continue
            new = text
            for old, repl in rewrite:
                new = new.replace(old, repl)
            if new != text:
                open(fp, "w", encoding="utf-8").write(new)
    main = os.path.join(dst, "nginx.conf")
    if fragment:
        main = os.path.join(dst, "_wrapper.conf")
        with open(main, "w") as f:
            f.write("events {}\nhttp {\n    include nginx.conf;\n}\n")
    err = os.path.join(dst, "logs", "error.log")
    r = subprocess.run([nginx, "-T", "-p", dst + "/", "-c", main, "-e", err],
                       capture_output=True, text=True, timeout=60)
    read = set()
    for m in re.finditer(r"^# configuration file (.+):$", r.stdout, re.M):
        path = m.group(1)
        if path.startswith(dst + "/"):
            rel = path[len(dst) + 1:]
            read.add(posixpath.join(prefix, rel) if prefix else rel)
    read = {x for x in read if posixpath.basename(x) != "_wrapper.conf"}
    msg = ""
    if r.returncode != 0:
        lines = [x for x in r.stderr.splitlines() if "[emerg]" in x]
        msg = (lines[-1] if lines else r.stderr.strip()[-200:]).replace(dst + "/", "")
    return r.returncode == 0, read, msg


def compose_services(root, rel, cache):
    d = posixpath.dirname(rel)
    while True:
        for c in COMPOSE:
            cand = posixpath.join(d, c) if d else c
            if os.path.isfile(os.path.join(root, cand)):
                if cand not in cache:
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
                return cand, cache[cand]
        if not d:
            return None, {}
        d = posixpath.dirname(d)


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


def classify_route(raw, compose, names):
    if "$" in raw or "{" in raw:
        return None, "variable"
    if "unix:" in raw or "unix@" in raw:
        return None, "unix-socket"
    host = route_host(raw)
    try:
        ipaddress.ip_address(host)
        return None, "ip-literal"
    except ValueError:
        pass
    if host == "localhost":
        return None, "ip-literal"
    hits = names.get(host, set()) if compose else set()
    if len(hits) == 1:
        return next(iter(hits)), ""
    if len(hits) > 1:
        return None, "ambiguous"
    return None, "external" if "." in host else "undeclared"


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("-subject", required=True)
    ap.add_argument("-out", required=True)
    ap.add_argument("-routes-out")
    ap.add_argument("-nginx")
    ap.add_argument("-scratch")
    ap.add_argument("-summary-out")
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
    if a.nginx and a.scratch:
        os.makedirs(a.scratch, exist_ok=True)
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
            ok, read, msg = nginx_check(root, prefix, a.nginx, a.scratch, fragment)
            entry = {"passed": ok}
            if ok:
                entry["oracle_closure"] = len(closure)
                entry["nginx_read"] = len(read)
                entry["agree"] = len(closure & read)
                entry["oracle_only"] = sorted(closure - read)
                entry["nginx_only"] = sorted(read - closure)
            else:
                entry["error"] = msg
            summary["nginx_T"][rel] = entry
            print(f"nginx-oracle: nginx -T {rel}: {json.dumps(entry)}", file=sys.stderr)

    with open(a.out, "w") as f:
        json.dump({"tool": "crossplane", "version": crossplane.__version__, "tier": "C",
                   "definitions": defs, "references": refs, "cross_references": xrefs}, f, indent=1, sort_keys=True)
        f.write("\n")

    if a.routes_out:
        upstreams_by_prefix = {}
        for rel, w in walks.items():
            upstreams_by_prefix.setdefault(prefix_of(root, rel), set()).update(w.upstreams)
        resolved, unresolved, cache = [], [], {}
        for rel, w in sorted(walks.items()):
            config_upstreams = upstreams_by_prefix.get(prefix_of(root, rel), set()) | w.upstreams
            compose, names = compose_services(root, rel, cache)
            for owner, raw, _ in w.routes:
                host, port = pass_host(raw)
                if not port and host in config_upstreams:
                    continue  # an upstream of the configuration, declared in another file
                svc, reason = classify_route(raw, compose, names)
                if svc:
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
