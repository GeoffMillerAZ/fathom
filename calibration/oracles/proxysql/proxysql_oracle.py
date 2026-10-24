"""proxysql_oracle: the language-breadth harness's oracle for the proxysql
doc-engine dialect (design section 1.3; LB-Q-proxysql). Bench-only.

    python3 proxysql_oracle.py -subject <checkout> -out <json> [-proxysql <bin>] [-work <scratch dir>]

ProxySQL itself loads every claimed file: `proxysql --initial -f -D <scratch
datadir> -c <file>` parses the libconfig file and writes its configuration into
the on-disk database (proxysql.db), which this program then reads with sqlite3:

  - mysql_servers, pgsql_servers (hostgroup, address, port), mysql_users,
    pgsql_users, mysql_query_rules, pgsql_query_rules, the replication
    hostgroups, scheduler and proxysql_servers rows: the entities the file made.
  - global_variables: every variable ProxySQL knows (admin-, mysql-, pgsql-...);
    a variable a group writes is a definition only when it is one of them.
  - the pairs: a server's hostgroup, a user's default_hostgroup, a rule's
    destination_hostgroup and mirror_hostgroup, a replication entry's writer
    and reader, each where the file writes the key (the table fills the default
    of one it does not) and the hostgroup is one the file's servers or
    replication entries declare.

What ProxySQL does not report is this program's own libconfig scanner (written
here, never fathom's decode): every line, which keys a file writes, and the
top-level sections and settings (ProxySQL keeps no row of them; they are read
by the scanner alone and counted in `disclose`). A file ProxySQL does not load
is left out and counted.

A file is claimed as the dialect claims it: a proxysql.cnf or proxysql.cfg, any
proxysql*.cnf or proxysql*.cfg, or a .cnf under a proxysql directory, whose
content opens a ProxySQL group or the datadir setting.
"""

import argparse
import fnmatch
import json
import os
import re
import shutil
import sqlite3
import subprocess
import sys
import tempfile
import time

TOOL = "proxysql"
SKIP_DIRS = {"vendor", ".git", "testdata", "generated", "node_modules", ".idea", ".vscode", ".claude"}
SNIFF = re.compile(r"^[ \t]*(?:admin_variables|mysql_variables|pgsql_variables|mysql_servers|mysql_users|pgsql_servers|"
                   r"proxysql_servers|datadir)[ \t\r\n]*[=:]", re.M)
PROTOS = ("mysql", "pgsql")


def claimed(rel, src):
    base = os.path.basename(rel)
    named = (base in ("proxysql.cnf", "proxysql.cfg") or fnmatch.fnmatch(base, "proxysql*.cnf")
             or fnmatch.fnmatch(base, "proxysql*.cfg") or (base.endswith(".cnf") and "/proxysql/" in "/" + rel))
    return named and bool(SNIFF.search(src))


# ── a libconfig scanner with lines ────────────────────────────────────────

TOKEN = re.compile(r'\s+|#[^\n]*|//[^\n]*|/\*.*?\*/|@include[^\n]*|"(?:[^"\\]|\\.)*"|[{}()\[\],;=:]|[^\s{}()\[\],;=:"#]+', re.S)


BARE = re.compile(r"^(?:true|false|[-+]?(?:0x[0-9a-f]+|[0-9]+(?:\.[0-9]*)?(?:e[-+]?[0-9]+)?|\.[0-9]+(?:e[-+]?[0-9]+)?)(?:l|ll)?)$", re.I)


class Node:
    def __init__(self, kind, line, end, value=None):
        self.kind, self.line, self.end, self.value = kind, line, end, value  # value: scalar str | dict | list


def tokens(src):
    pos, line, out = 0, 1, []
    while pos < len(src):
        m = TOKEN.match(src, pos)
        if not m:
            raise ValueError("line %d: unexpected %r" % (line, src[pos]))
        t = m.group(0)
        if not (t.isspace() or t.startswith(("#", "//", "/*", "@include"))):
            out.append((t, line))
        line += t.count("\n")
        pos = m.end()
    return out


def parse(src):
    toks, i = tokens(src.lstrip("﻿")), 0

    def value():
        nonlocal i
        t, ln = toks[i]
        if t in "{":
            i += 1
            return group(ln, "}")
        if t in ("(", "["):
            closer = ")" if t == "(" else "]"
            i += 1
            items = []
            while True:
                if i >= len(toks):
                    raise ValueError("unterminated list")
                if toks[i][0] == closer:
                    end = toks[i][1]
                    i += 1
                    return Node("list", ln, end, items)
                items.append(value())
                if i < len(toks) and toks[i][0] == ",":
                    i += 1
        if t.startswith('"'):
            s, end = "", ln
            while i < len(toks) and toks[i][0].startswith('"'):
                s += bytes(toks[i][0][1:-1], "utf-8").decode("unicode_escape")
                end = toks[i][1] + toks[i][0].count("\n")
                i += 1
            return Node("scalar", ln, end, s)
        if t in "}),];=:" or not BARE.match(t):
            raise ValueError("line %d: unexpected %r" % (ln, t))
        i += 1
        return Node("scalar", ln, ln, t)

    def group(ln, closer):
        nonlocal i
        d = {}
        while True:
            if i >= len(toks):
                if closer:
                    raise ValueError("unterminated group")
                return Node("group", ln, ln, d)
            t, tl = toks[i]
            if closer and t == closer:
                i += 1
                return Node("group", ln, tl, d)
            if not re.match(r"[A-Za-z*][-\w*]*$", t):
                raise ValueError("line %d: unexpected %r" % (tl, t))
            i += 1
            if i >= len(toks) or toks[i][0] not in ("=", ":"):
                raise ValueError("line %d: %s has no = or :" % (tl, t))
            i += 1
            if i >= len(toks):
                raise ValueError("missing value")
            v = value()
            v.line = tl
            d[t] = v
            if i < len(toks) and toks[i][0] in (";", ","):
                i += 1

    root = group(1, None)
    return root


def sval(node, key):
    n = node.value.get(key) if node.kind == "group" else None
    return n.value if n is not None and n.kind == "scalar" else None


# ── loading ───────────────────────────────────────────────────────────────

def load(proxysql, path, datadir):
    os.makedirs(datadir, exist_ok=True)
    log = open(os.path.join(datadir, "out.log"), "w")
    proc = subprocess.Popen([proxysql, "--initial", "-f", "-D", datadir, "-c", path], stdout=log, stderr=log)
    db = os.path.join(datadir, "proxysql.db")
    ok = False
    try:
        for _ in range(200):
            if proc.poll() is not None:
                break
            time.sleep(0.1)
            try:
                c = sqlite3.connect("file:%s?mode=ro" % db, uri=True)
                n = c.execute("select count(*) from global_variables").fetchone()[0]
                c.close()
                if n > 100:
                    time.sleep(1.5)
                    ok = True
                    break
            except sqlite3.Error:
                continue
    finally:
        if proc.poll() is None:
            proc.terminate()
        try:
            proc.wait(timeout=20)
        except subprocess.TimeoutExpired:
            proc.kill()
            proc.wait()
        log.close()
    if not ok:
        tail = open(os.path.join(datadir, "out.log")).read().strip().splitlines()
        return None, (tail[-1] if tail else "no output")[:160]
    c = sqlite3.connect("file:%s?mode=ro" % db, uri=True)
    return c, ""


def rows(c, sql):
    try:
        return c.execute(sql).fetchall()
    except sqlite3.Error:
        return []


def entities(rel, root, c):
    defs, refs, notes = [], [], {"scanner_only": 0, "dropped": 0}
    top = root.value

    def add(name, kind, node):
        defs.append({"path": rel, "name": name, "line": node.line, "end": node.end, "kind": kind})

    def ref(frm, to):
        refs.append({"path": rel, "from": frm, "to": to})

    known_vars = {r[0] for r in rows(c, "select variable_name from global_variables")}
    tables = {r[0] for r in rows(c, "select name from sqlite_master where type='table'")}
    for name, node in top.items():
        if node.kind == "scalar":
            add(name, "key", node)
            notes["scanner_only"] += 1
        elif node.kind == "group":
            add(name, "section", node)
            notes["scanner_only"] += 1
            prefix = name[:-len("_variables")] + "-" if name.endswith("_variables") else ""
            for vk, vn in node.value.items():
                if vn.kind != "scalar":
                    continue
                if prefix and (prefix + vk) in known_vars:
                    add(name + "." + vk, "variable_decl", vn)
                else:
                    notes["dropped"] += 1
        else:
            add(name, "section", node)
            notes["scanner_only"] += 1
    for p in PROTOS:
        srv = top.get(p + "_servers")
        db_srv = {(str(r[0]), r[1], str(r[2])) for r in rows(c, "select hostgroup_id, hostname, port from %s_servers" % p)} if p + "_servers" in tables else set()
        for it in (srv.value if srv and srv.kind == "list" else []):
            hg, addr, port = sval(it, "hostgroup"), sval(it, "address"), sval(it, "port")
            if None in (hg, addr, port):
                continue
            if (hg, addr, port) not in db_srv:
                notes["dropped"] += 1
                continue
            add("%s:%s@%s" % (addr, port, hg), "target", it)
    # hostgroup entities, in the order the dialect's spec declares them (all mysql, then all pgsql)
    order = []
    for p in PROTOS:
        srv, rep = top.get(p + "_servers"), top.get(p + "_replication_hostgroups")
        for it in (srv.value if srv and srv.kind == "list" else []):
            if sval(it, "hostgroup") is not None and sval(it, "address") is not None:
                order.append(it.value["hostgroup"])
        for key in ("writer_hostgroup", "reader_hostgroup"):
            for it in (rep.value if rep and rep.kind == "list" else []):
                if sval(it, key) is not None:
                    order.append(it.value[key])
    seen = set()
    for n in order:
        if n.value not in seen:
            seen.add(n.value)
            add(n.value, "label", n)
    for p in PROTOS:
        srv, usr, rul, rep = top.get(p + "_servers"), top.get(p + "_users"), top.get(p + "_query_rules"), top.get(p + "_replication_hostgroups")
        db_usr = {r[0]: str(r[1]) for r in rows(c, "select username, default_hostgroup from %s_users" % p)} if p + "_users" in tables else {}
        for it in (srv.value if srv and srv.kind == "list" else []):
            hg = sval(it, "hostgroup")
            if hg is not None and hg in seen and sval(it, "address") is not None:
                ref("%s:%s@%s" % (sval(it, "address"), sval(it, "port"), hg), hg)
        for it in (usr.value if usr and usr.kind == "list" else []):
            u = sval(it, "username")
            if u is None or u not in db_usr:
                continue
            add("%s/%s" % (p, u), "element", it)
            dh = sval(it, "default_hostgroup")
            if dh is not None and db_usr[u] == dh and dh in seen:
                ref("%s/%s" % (p, u), dh)
        db_rul = {str(r[0]): (str(r[1]), str(r[2])) for r in rows(c, "select rule_id, destination_hostgroup, mirror_hostgroup from %s_query_rules" % p)} if p + "_query_rules" in tables else {}
        for it in (rul.value if rul and rul.kind == "list" else []):
            rid = sval(it, "rule_id")
            if rid is None or rid not in db_rul:
                continue
            add("%s/%s" % (p, rid), "rule", it)
            for k, col in (("destination_hostgroup", 0), ("mirror_hostgroup", 1)):
                v = sval(it, k)
                if v is not None and db_rul[rid][col] == v and v in seen:
                    ref("%s/%s" % (p, rid), v)
        db_rep = {(str(r[0]), str(r[1])) for r in rows(c, "select writer_hostgroup, reader_hostgroup from %s_replication_hostgroups" % p)} if p + "_replication_hostgroups" in tables else set()
        for it in (rep.value if rep and rep.kind == "list" else []):
            w, r = sval(it, "writer_hostgroup"), sval(it, "reader_hostgroup")
            if (w, r) not in db_rep:
                continue
            name = "%s/%s-%s" % (p, w, r)
            add(name, "selector", it)
            for v in (w, r):
                if v in seen:
                    ref(name, v)
    sch = top.get("scheduler")
    db_sch = {str(r[0]) for r in rows(c, "select id from scheduler")}
    for it in (sch.value if sch and sch.kind == "list" else []):
        i = sval(it, "id")
        if i in db_sch:
            add(i, "task", it)
    ps = top.get("proxysql_servers")
    db_ps = {(r[0], str(r[1])) for r in rows(c, "select hostname, port from proxysql_servers")}
    for it in (ps.value if ps and ps.kind == "list" else []):
        h, pt = sval(it, "hostname"), sval(it, "port")
        if (h, pt) in db_ps:
            add("%s:%s" % (h, pt), "service", it)
    return defs, refs, notes


def walk(subject):
    for root, dirs, files in os.walk(subject):
        dirs[:] = sorted(d for d in dirs if d not in SKIP_DIRS)
        for f in sorted(files):
            if not f.endswith((".cnf", ".cfg")):
                continue
            full = os.path.join(root, f)
            rel = os.path.relpath(full, subject).replace(os.sep, "/")
            try:
                src = open(full, encoding="utf-8", errors="replace").read()
            except OSError:
                continue
            if claimed(rel, src):
                yield rel, full, src


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("-subject", required=True)
    ap.add_argument("-out", required=True)
    ap.add_argument("-proxysql", default="")
    ap.add_argument("-work", default="")
    a = ap.parse_args()
    pxy = a.proxysql or shutil.which("proxysql") or "proxysql"
    work = a.work or tempfile.mkdtemp(prefix="proxysql-oracle-")
    version = next((ln for ln in subprocess.run([pxy, "--version"], capture_output=True, text=True).stdout.splitlines()
                    if ln.startswith("ProxySQL version")), "ProxySQL").strip()
    defs, refs = [], []
    n_files = n_loaded = n_refused = scanner_only = dropped = 0
    for k, (rel, full, src) in enumerate(walk(os.path.abspath(a.subject))):
        n_files += 1
        try:
            root = parse(src)
        except ValueError as e:
            n_refused += 1
            print("not libconfig: %s: %s" % (rel, e), file=sys.stderr)
            continue
        c, err = load(pxy, full, os.path.join(work, "f%d" % k))
        if c is None:
            n_refused += 1
            print("refused by proxysql: %s: %s" % (rel, err), file=sys.stderr)
            continue
        n_loaded += 1
        d, r, notes = entities(rel, root, c)
        c.close()
        defs += d
        refs += r
        scanner_only += notes["scanner_only"]
        dropped += notes["dropped"]
        if notes["dropped"]:
            print("scanner entries proxysql did not report: %s: %d" % (rel, notes["dropped"]), file=sys.stderr)
    disclose = ["proxysql loaded %d of %d files into its database (%d refused and left out); %d top-level sections and settings are the scanner's alone (the database holds no row of them); %d entries the scanner read and the database did not hold were left out"
                % (n_loaded, n_files, n_refused, scanner_only, dropped)]
    out = {"tool": TOOL, "version": version + " + line scanner", "tier": "A", "definitions": defs, "references": refs,
           "disclose": disclose}
    json.dump(out, open(a.out, "w"), indent=1)
    print("proxysql oracle: %s; %d files (%d loaded, %d refused), %d definitions, %d references"
          % (version, n_files, n_loaded, n_refused, len(defs), len(refs)), file=sys.stderr)


if __name__ == "__main__":
    main()
