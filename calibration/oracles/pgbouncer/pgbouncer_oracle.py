"""pgbouncer_oracle: the language-breadth harness's oracle for the pgbouncer
doc-engine dialect (design section 1.3; LB-Q-pgbouncer). Bench-only.

    python3 pgbouncer_oracle.py -subject <checkout> -out <json> \
        [-pgbouncer <bin>] [-psql <bin>] [-work <scratch dir>]

PgBouncer itself loads every claimed file on a scratch instance (a copy of the
file with the settings that name this machine - paths, addresses, ports,
authentication - replaced by the scratch instance's own) and answers over its
admin console:

  - SHOW DATABASES: the [databases] entries it created; `*` is attested by a
    connection to a database no entry names, which the fallback entry creates.
  - SHOW PEERS: the [peers] entries. SHOW USERS: the [users] entries.
  - SHOW CONFIG: every setting PgBouncer knows; a setting a file writes in
    [pgbouncer] is a definition only when it is one of them.

What PgBouncer does not report, this program's own line scanner (a reader
written here, never fathom's decode) supplies: every line, and the pair a
database's `auth_dbname` parameter writes (the console does not show a
per-database auth_dbname). So the file is tier C - the pairs cap the level at
good (section 1.3) - and the agreement of the scanner's entity set with
PgBouncer's is counted on stderr and in `disclose`: a file PgBouncer refuses is
read by the scanner alone and counted.

A file is claimed as the dialect claims it: a pgbouncer.ini, or any file named
pgbouncer*.ini, or an .ini under a pgbouncer directory, or any other .ini
whose first 16 KiB opens a [databases] or [pgbouncer] section.
"""

import argparse
import fnmatch
import json
import os
import re
import shutil
import signal
import socket
import subprocess
import sys
import tempfile
import time

TOOL = "pgbouncer"
SKIP_DIRS = {"vendor", ".git", "testdata", "generated", "node_modules", ".idea", ".vscode", ".claude"}
SNIFF = re.compile(r"^[ \t]*\[(?:databases|pgbouncer)\][ \t]*\r?$", re.M)
PARAM_SECTIONS = ("databases", "users", "peers")
# [pgbouncer] settings that name this machine: replaced on the scratch instance.
ENV_SETTINGS = {"logfile", "pidfile", "listen_addr", "listen_port", "unix_socket_dir", "unix_socket_mode",
                "unix_socket_group", "auth_type", "auth_file", "auth_hba_file", "auth_ident_file", "admin_users",
                "stats_users", "syslog", "syslog_ident", "syslog_facility", "user", "so_reuseport", "pkt_buf",
                "max_packet_size", "peer_id", "disable_pqexec", "logfile", "dns_zone_check_period"}


def claimed(rel, src):
    base = os.path.basename(rel)
    if not base.lower().endswith(".ini"):
        return False
    if (base == "pgbouncer.ini" or fnmatch.fnmatch(base, "pgbouncer*.ini") or "/pgbouncer/" in "/" + rel):
        return bool(SNIFF.search(src))
    return bool(SNIFF.search(src[:16384]))


def strip_key(k):
    k = k.strip()
    if len(k) >= 2 and k[0] == '"' and k[-1] == '"':
        return k[1:-1]
    return k


def params(value):
    """key=value pairs of a connect string; a value is bare or 'quoted' ('' is a quote)."""
    out, i, n = [], 0, len(value)
    while i < n:
        while i < n and value[i] in " \t":
            i += 1
        j = i
        while i < n and value[i] not in "= \t":
            i += 1
        key = value[j:i]
        while i < n and value[i] in " \t":
            i += 1
        if not key or i >= n or value[i] != "=":
            continue
        i += 1
        while i < n and value[i] in " \t":
            i += 1
        val = []
        if i < n and value[i] == "'":
            i += 1
            while i < n:
                if value[i] == "'":
                    if i + 1 < n and value[i + 1] == "'":
                        val.append("'")
                        i += 2
                        continue
                    i += 1
                    break
                val.append(value[i])
                i += 1
        else:
            while i < n and value[i] not in " \t":
                val.append(value[i])
                i += 1
        out.append((key, "".join(val)))
    return out


def scan(src):
    """sections: name -> {header, last, entries: [(key, line, value)]}, in file order."""
    src = src.lstrip("﻿")
    sections, cur = {}, None
    for no, raw in enumerate(src.split("\n"), 1):
        text = raw.strip()
        if not text or text[0] in "#;" or text.startswith("%include"):
            continue
        if text[0] == "[" and text.endswith("]"):
            name = text[1:-1].strip()
            cur = sections.setdefault(name, {"header": no, "last": no, "entries": []})
            continue
        if cur is None:
            continue
        if text[0] == '"':
            m = re.match(r'"([^"]*)"\s*=\s*(.*)$', text)
            if not m:
                continue
            key, value = m.group(1), m.group(2)
        else:
            if "=" not in text:
                continue
            k, _, value = text.partition("=")
            key, value = strip_key(k), value.strip()
            if not key:
                continue
        cur["last"] = no
        cur["entries"] = [e for e in cur["entries"] if e[0] != key] + [(key, no, value)]
    return sections


def free_port():
    with socket.socket() as s:
        s.bind(("127.0.0.1", 0))
        return s.getsockname()[1]


def sanitized(src):
    """The file with every machine-naming [pgbouncer] setting and %include dropped."""
    out, in_pgb = [], False
    for raw in src.lstrip("﻿").split("\n"):
        text = raw.strip()
        if text.startswith("%include"):
            continue
        if text.startswith("[") and text.endswith("]"):
            in_pgb = text[1:-1].strip() == "pgbouncer"
        elif in_pgb and text and text[0] not in "#;" and "=" in text:
            key = text.partition("=")[0].strip()
            if key in ENV_SETTINGS or key.endswith(("_file", "_dir")) or key.startswith(("client_tls", "server_tls")):
                continue
        out.append(raw)
    return "\n".join(out)


class Instance:
    def __init__(self, pgbouncer, psql, work):
        self.pgbouncer, self.psql, self.work = pgbouncer, psql, work

    def query(self, port, sql, dbname="pgbouncer"):
        p = subprocess.run([self.psql, "-X", "-At", "-F", "\t", "-h", "127.0.0.1", "-p", str(port), "-U", "oracle",
                            "-d", dbname, "-c", sql], capture_output=True, text=True, timeout=20)
        if p.returncode != 0:
            raise RuntimeError(p.stderr.strip())
        return [ln.split("\t") for ln in p.stdout.splitlines() if ln]

    def load(self, src, tag):
        """Start PgBouncer on the file; return (databases, peers, users, config keys) or an error string."""
        d = os.path.join(self.work, tag)
        os.makedirs(d, exist_ok=True)
        port = free_port()
        with open(os.path.join(d, "userlist.txt"), "w") as f:
            f.write('"oracle" ""\n')
        ini = os.path.join(d, "pgbouncer.ini")
        with open(ini, "w") as f:
            f.write(sanitized(src) + "\n\n[pgbouncer]\n")
            for k, v in (("logfile", d + "/pgb.log"), ("pidfile", d + "/pgb.pid"), ("listen_addr", "127.0.0.1"),
                         ("listen_port", port), ("unix_socket_dir", d), ("auth_type", "trust"),
                         ("auth_file", d + "/userlist.txt"), ("admin_users", "oracle")):
                f.write("%s = %s\n" % (k, v))
        os.chmod(d, 0o777)
        for n in os.listdir(d):
            os.chmod(os.path.join(d, n), 0o666)
        cmd = [self.pgbouncer, "-d"] + (["-u", "postgres"] if os.geteuid() == 0 else []) + [ini]
        p = subprocess.run(cmd, capture_output=True, text=True, timeout=20)
        pidfile = d + "/pgb.pid"
        try:
            for _ in range(40):
                try:
                    self.query(port, "show version")
                    break
                except Exception:
                    time.sleep(0.1)
            else:
                log = ""
                try:
                    log = open(d + "/pgb.log").read().strip().splitlines()[-1]
                except Exception:
                    log = (p.stdout + p.stderr).strip()
                return "does not load: " + log[:160]
            # `*` is attested by a database no entry names: the fallback entry creates it.
            subprocess.run([self.psql, "-X", "-h", "127.0.0.1", "-p", str(port), "-U", "oracle", "-d", "zz_oracle_probe",
                            "-c", "select 1"], capture_output=True, text=True, timeout=20,
                           env=dict(os.environ, PGCONNECT_TIMEOUT="1"))
            dbs = {r[0] for r in self.query(port, "show databases")}
            wildcard = "zz_oracle_probe" in dbs
            dbs -= {"pgbouncer", "zz_oracle_probe"}
            if wildcard:
                dbs.add("*")
            peers = {r[0] for r in self.query(port, "show peers")}
            users = {r[0] for r in self.query(port, "show users")} - {"oracle", "pgbouncer"}
            cfg = {r[0] for r in self.query(port, "show config")}
            return dbs, peers, users, cfg
        finally:
            try:
                os.kill(int(open(pidfile).read().strip()), signal.SIGTERM)
            except Exception:
                pass


def entities(rel, sections, attest):
    """(definitions, references, disagreements) of one file."""
    defs, refs, disagree = [], [], []
    dbs, peers, users, cfg = attest if attest else (None, None, None, None)

    def add(name, kind, lo, hi):
        defs.append({"path": rel, "name": name, "line": lo, "end": hi, "kind": kind})

    def keep(name, known, what):
        if known is None or name in known:
            return True
        disagree.append("%s %s" % (what, name))
        return False

    declared = set()
    for sname, sec in sections.items():
        add(sname, "section", sec["header"], sec["last"])
        for key, no, value in sec["entries"]:
            if sname == "databases":
                if keep(key, dbs, "database"):
                    add(key, "target", no, no)
                    declared.add(key)
            elif sname == "peers":
                if keep(key, peers, "peer"):
                    add(key, "service", no, no)
            elif sname == "users":
                if keep(key, users, "user"):
                    add(key, "element", no, no)
            elif sname == "pgbouncer":
                if keep(key, cfg, "setting"):
                    add(key, "key", no, no)
    if dbs is not None:
        declared = declared & dbs
    for sname, sec in sections.items():
        if sname == "databases":
            for key, no, value in sec["entries"]:
                for pk, pv in params(value):
                    if pk == "auth_dbname" and pv in declared and key in declared:
                        refs.append({"path": rel, "from": key, "to": pv})
        elif sname == "pgbouncer":
            for key, no, value in sec["entries"]:
                if key == "auth_dbname" and value in declared:
                    refs.append({"path": rel, "from": "pgbouncer", "to": value})
    return defs, refs, disagree


def walk(subject):
    for root, dirs, files in os.walk(subject):
        dirs[:] = sorted(d for d in dirs if d not in SKIP_DIRS)
        for f in sorted(files):
            full = os.path.join(root, f)
            rel = os.path.relpath(full, subject).replace(os.sep, "/")
            if f.lower().endswith(".ini"):
                try:
                    src = open(full, encoding="utf-8", errors="replace").read()
                except OSError:
                    continue
                if claimed(rel, src):
                    yield rel, src


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("-subject", required=True)
    ap.add_argument("-out", required=True)
    ap.add_argument("-pgbouncer", default="")
    ap.add_argument("-psql", default="")
    ap.add_argument("-work", default="")
    a = ap.parse_args()
    pgb = a.pgbouncer or shutil.which("pgbouncer") or "/usr/sbin/pgbouncer"
    psql = a.psql or shutil.which("psql") or "psql"
    work = a.work or tempfile.mkdtemp(prefix="pgbouncer-oracle-")
    os.makedirs(work, exist_ok=True)
    inst = Instance(pgb, psql, work)
    version = subprocess.run([pgb, "--version"], capture_output=True, text=True).stdout.splitlines()[0].strip()
    defs, refs, notes = [], [], []
    n_files = n_loaded = n_refused = n_dis = 0
    for k, (rel, src) in enumerate(walk(os.path.abspath(a.subject))):
        n_files += 1
        sections = scan(src)
        res = inst.load(src, "f%d" % k)
        attest = None
        if isinstance(res, str):
            n_refused += 1
            print("refused by pgbouncer: %s: %s" % (rel, res), file=sys.stderr)
        else:
            n_loaded += 1
            attest = res
        d, r, dis = entities(rel, sections, attest)
        defs += d
        refs += r
        n_dis += len(dis)
        for x in dis:
            print("scanner and pgbouncer disagree: %s: %s" % (rel, x), file=sys.stderr)
    disclose = ["pgbouncer loaded %d of %d files on a scratch instance (%d read by the line scanner alone); %d entries the scanner read and pgbouncer did not report were left out"
                % (n_loaded, n_files, n_refused, n_dis)]
    out = {"tool": TOOL, "version": version + " + line scanner", "tier": "C", "definitions": defs, "references": refs,
           "disclose": disclose}
    json.dump(out, open(a.out, "w"), indent=1)
    print("pgbouncer oracle: %s; %d files (%d loaded, %d refused), %d definitions, %d references, %d disagreements"
          % (version, n_files, n_loaded, n_refused, len(defs), len(refs), n_dis), file=sys.stderr)


if __name__ == "__main__":
    main()
