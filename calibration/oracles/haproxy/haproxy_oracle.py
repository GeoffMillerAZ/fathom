"""haproxy_oracle.py - the language-breadth harness's oracle for the haproxy
native dialect (design section 1.3, 13.4 step 19; LB-Q-haproxy).

    python3 haproxy_oracle.py -subject <checkout> -haproxy <haproxy binary> -out <file> \
        [-routes-out <file>] [-summary-out <file>] [-compose-python <venv python>]

Tier A: HAProxy 3.4.6 itself. `haproxy -c -dr -dC0,line -f <file>` (run in the
file's directory) parses and checks the configuration (-c), does not fail on a
server address it cannot resolve here (-dr: a compose service name), and dumps
every statement it parsed with its source line (-dC0,line: "<line>\\t<section
keyword ...>" and "<line>\\t\\t<keyword ...>", quotes and escapes already
applied, comments and blank lines dropped; HAProxy reads a file twice, so the
first pass is kept). Every file named haproxy.cfg or
*.cfg is tried; one HAProxy rejects (exit 1: an unknown keyword, a missing file
it names) is counted and left out, so a .cfg of another format (setup.cfg) is
never a definition. Emitted, under the dialect's naming contract (its
PROVENANCE.md):

  - definitions: a `service` per frontend, backend and listen section (its
    name, from its keyword's line to its last statement's); a `target` per
    server and server-template of a backend or listen section ("<section>
    <name>"); a `rule` per acl line of a proxy section ("<section> <acl>"); a
    `route` per use_backend and default_backend line of a frontend or listen
    section ("<section> <backend>").
  - references: a route to the backend (or listen section) it names, when the
    file declares one of that name.

Route targets (-routes-out, bench-only; lang-oracle's tool-oracle decoder
refuses unknown fields, so they are a file of their own): each server's
address as HAProxy read it, from its section, resolved against the nearest
compose file at or above the configuration (`docker compose config --format
json`, Docker Compose 5.1.4, tier A for the service set; a version-1 file it
refuses is read with PyYAML from -compose-python's interpreter, tier C): a
service name (or a container_name, hostname or network alias of exactly one
service) is a route_reference; otherwise a route_unresolved with its reason:
variable ($ or %[), unix-socket (unix@, abns@, a path), ip-literal (an IP
address, *, or localhost), ambiguous, external (a dotted host) or undeclared.

Directories the connector never reads (testdata, vendor, node_modules,
generated, .git and the editor directories) are not walked. Bench-only; never a
product dependency.
"""

import argparse
import ipaddress
import json
import os
import posixpath
import re
import subprocess
import sys

SKIP_DIRS = {"vendor", ".git", "testdata", "generated", "node_modules", ".idea", ".vscode", ".claude"}
SECTIONS = {"global", "defaults", "frontend", "backend", "listen", "peers", "resolvers", "userlist", "mailers",
            "cache", "program", "http-errors", "ring", "log-forward", "crt-store", "traces", "acme", "fcgi-app"}
PROXIES = {"frontend": "service", "backend": "service", "listen": "service"}
LINE = re.compile(r"^(\d+)\t(\t?)(.*)$")


def candidates(root):
    out = []
    for d, dirs, files in os.walk(root):
        dirs[:] = sorted(x for x in dirs if x not in SKIP_DIRS)
        rel_dir = os.path.relpath(d, root)
        for f in sorted(files):
            if f.endswith(".cfg"):
                out.append(f if rel_dir == "." else posixpath.join(rel_dir.replace(os.sep, "/"), f))
    return out


def dump(haproxy, root, rel):
    path = os.path.join(root, rel)
    r = subprocess.run([haproxy, "-c", "-dr", "-dC0,line", "-f", path], cwd=os.path.dirname(path),
                       capture_output=True, text=True, timeout=60)
    if r.returncode not in (0, 2):
        err = [x for x in r.stderr.splitlines() if "ALERT" in x]
        return None, (err[0] if err else r.stderr.strip()[:200])
    # HAProxy reads the file twice (a first pass for the global section), so
    # the dump holds the file twice: keep the first pass (its lines ascend).
    out, last = [], 0
    for ln in r.stdout.splitlines():
        m = LINE.match(ln)
        if m:
            n = int(m.group(1))
            if n < last:
                break
            last = n
            out.append((n, m.group(2) == "\t", m.group(3).split()))
    return out, ""


class FileRead:
    def __init__(self, rel, stmts):
        self.rel = rel
        self.defs, self.refs, self.routes = [], [], []
        declared, pending, sec = set(), [], None
        for line, indented, words in stmts:
            if not words:
                continue
            if not indented and words[0] in SECTIONS:
                sec = {"kw": words[0], "name": words[1] if len(words) > 1 else "", "def": None}
                if words[0] in PROXIES and sec["name"]:
                    sec["def"] = {"path": rel, "name": sec["name"], "line": line, "end": line, "kind": "service"}
                    self.defs.append(sec["def"])
                    if words[0] != "frontend":
                        declared.add(sec["name"])
                continue
            if sec is None or sec["def"] is None:
                continue
            sec["def"]["end"] = line
            kw, name = words[0], sec["name"]
            if kw in ("server", "server-template") and sec["kw"] != "frontend":
                addr_at = 3 if kw == "server-template" else 2
                if len(words) > addr_at:
                    self.defs.append({"path": rel, "name": name + " " + words[1], "line": line, "end": line, "kind": "target"})
                    self.routes.append((name, words[addr_at], line))
            elif kw == "acl" and len(words) > 1:
                self.defs.append({"path": rel, "name": name + " " + words[1], "line": line, "end": line, "kind": "rule"})
            elif kw in ("use_backend", "default_backend") and sec["kw"] != "backend" and len(words) > 1:
                route = name + " " + words[1]
                self.defs.append({"path": rel, "name": route, "line": line, "end": line, "kind": "route"})
                pending.append((route, words[1]))
        for route, backend in pending:
            if backend in declared and "%[" not in backend:
                self.refs.append({"path": rel, "from": route, "to": backend})


def compose_names(root, cand, cache, py):
    """host name -> service names of one compose file, as `docker compose config`
    reads it (a version-1 file it refuses is read with PyYAML, tier C)."""
    if cand in cache:
        return cache[cand]
    names = {}
    r = subprocess.run(["docker", "compose", "-f", os.path.join(root, cand), "config", "--format", "json",
                        "--no-interpolate", "--no-path-resolution", "--no-consistency", "--no-normalize"],
                       capture_output=True, text=True, timeout=60)
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
    elif py:
        v1 = subprocess.run([py, "-c", V1_READER, os.path.join(root, cand)], capture_output=True, text=True)
        for svc, aliases in (json.loads(v1.stdout) if v1.returncode == 0 else {}).items():
            for alias in aliases:
                names.setdefault(alias, set()).add(svc)
        print(f"haproxy-oracle: {cand} read as a version-1 compose file: {len(names)} names", file=sys.stderr)
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


V1_READER = """
import json, sys, yaml
doc = yaml.safe_load(open(sys.argv[1]))
out = {}
if isinstance(doc, dict) and "services" not in doc:
    for svc, spec in doc.items():
        if svc in ("version", "networks", "volumes") or not isinstance(spec, dict):
            continue
        out[svc] = [svc] + [str(a) for a in (spec.get("container_name"), spec.get("hostname")) if a]
print(json.dumps(out))
"""


def classify(raw, lookup):
    """(service, files, reason) of a server address; lookup(host) is the
    nearest compose project's hits."""
    if "$" in raw or "%[" in raw:
        return None, [], "variable"
    if raw.startswith(("unix@", "abns@", "/")):
        return None, [], "unix-socket"
    s = raw
    for pre in ("ipv4@", "ipv6@", "fd@", "sockpair@"):
        if s.startswith(pre):
            s = s[len(pre):]
    if s.startswith("["):
        host = s[1:s.find("]")] if "]" in s else s
    elif s.count(":") > 1:
        host = s  # a bare IPv6 address
    else:
        host = s.rsplit(":", 1)[0] if ":" in s else s
    if host in ("*", "localhost", ""):
        return None, [], "ip-literal"
    try:
        ipaddress.ip_address(host)
        return None, [], "ip-literal"
    except ValueError:
        pass
    svc, files, reason = route_hits(lookup(host))
    if svc or reason:
        return svc, files, reason
    return None, [], "external" if "." in host else "undeclared"


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("-subject", required=True)
    ap.add_argument("-haproxy", required=True)
    ap.add_argument("-out", required=True)
    ap.add_argument("-routes-out")
    ap.add_argument("-summary-out")
    ap.add_argument("-compose-python")
    a = ap.parse_args()
    root = os.path.abspath(a.subject)
    version = subprocess.run([a.haproxy, "-v"], capture_output=True, text=True).stdout.split()[2]

    reads, rejected = {}, {}
    for rel in candidates(root):
        stmts, err = dump(a.haproxy, root, rel)
        if stmts is None:
            rejected[rel] = err
            continue
        reads[rel] = FileRead(rel, stmts)

    defs = [d for r in reads.values() for d in r.defs]
    refs = [x for r in reads.values() for x in r.refs]
    with open(a.out, "w") as f:
        json.dump({"tool": "haproxy", "version": version, "tier": "A", "definitions": defs, "references": refs},
                  f, indent=1, sort_keys=True)
        f.write("\n")

    summary = {"files": len(reads), "rejected": rejected}
    if a.routes_out:
        resolved, unresolved, cache = [], [], {}
        for rel, r in sorted(reads.items()):
            for owner, raw, _ in r.routes:
                svc, files, reason = classify(raw, lambda h: resolve_compose(root, rel, h, lambda c: compose_names(root, c, cache, a.compose_python)))
                if svc:
                    for compose in files:
                        resolved.append({"path": rel, "from": owner, "to_path": compose, "to": svc, "raw": raw})
                else:
                    unresolved.append({"path": rel, "from": owner, "raw": raw, "reason": reason})
        with open(a.routes_out, "w") as f:
            json.dump({"tool": "haproxy+docker-compose", "version": version, "tier": "A",
                       "route_references": resolved, "route_unresolved": unresolved}, f, indent=1, sort_keys=True)
            f.write("\n")
        summary["routes"] = {"resolved": len(resolved), "unresolved": len(unresolved)}

    print(f"haproxy-oracle: {len(reads)} files, {len(rejected)} rejected, {len(defs)} definitions, "
          f"{len(refs)} references", file=sys.stderr)
    for rel, err in sorted(rejected.items()):
        print(f"haproxy-oracle: rejected {rel}: {err[:200]}", file=sys.stderr)
    if a.summary_out:
        with open(a.summary_out, "w") as f:
            json.dump(summary, f, indent=1, sort_keys=True)
            f.write("\n")


if __name__ == "__main__":
    main()
