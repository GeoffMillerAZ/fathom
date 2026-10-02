"""bicep_oracle.py - the language-breadth harness's oracle for the bicep tags-engine
dialect (design section 1.3, 13.4 steps 18-19; LB-Q-bicep).

    python3 bicep_oracle.py -subject <checkout> -bicep <bicep CLI> -dotnet <dotnet> \
        -langserver <Bicep.LangServer.dll> -out <file> [-files-out <file>] \
        [-summary-out <file>] [-on-eperm <command>]

Tier A: Azure/bicep's own tools (MIT), the release the dialect is measured
against. The CLI's JSON-RPC server (`bicep jsonrpc --stdio`) compiles each .bicep
(`bicep/compile`) and .bicepparam (`bicep/compileParams`) file; a file it refuses
(an error diagnostic: an intentionally invalid baseline, a registry module the
offline cache does not hold) is counted and left out. Its deployment graph
(`bicep/getDeploymentGraph`) names each module's path. The language server
(`Bicep.LangServer.dll --stdio`, the same release, run by the .NET runtime) is
the semantic model: `textDocument/documentSymbol` lists every declaration with its
range, and `textDocument/references` every place a declaration's symbol is used,
locals (loop and lambda variables, function parameters) being symbols of their
own and so never the file's.

  - definitions: a declaration the language server lists in a compiled file:
    `variable_decl` for a param (a .bicepparam's assignments too), `variable` for
    a var, `resource` for a resource (existing and nested children included),
    `module` for a module, `output`, `type_alias` for a type, `function` for a
    func and `key` for a metadata declaration, each at its name's line, ending
    where the declaration ends;
  - references: for every declaration (and import) symbol of the file, each use
    the language server reports inside another declaration of the same file:
    the innermost enclosing declaration to the symbol's name; and, for a
    module, each key of its `params` object (each one a parameter of the
    module's template, or the compile refuses it) as `<path>.<key>` (the path
    as written), the qualified name the dialect stores;
  - cross_references: a module whose path the CLI resolves to a .bicep or .json
    file of the subject, from the module to that file; and a compile-time
    import of a file of the subject (`import {a} from './t.bicep'`), from every
    declaration that uses an imported name to that file (file granularity).
    A registry module (br:, ts:) and a .bicepparam's `using` are counted.

Directories the connector never reads (testdata, vendor, node_modules,
generated, .git and the editor directories) are not walked.
"""

import argparse
import json
import os
import posixpath
import queue
import re
import subprocess
import sys
import threading
import time

SKIP_DIRS = {"vendor", ".git", "testdata", "generated", "node_modules", ".idea", ".vscode", ".claude"}
KIND_OF = {"param": "variable_decl", "var": "variable", "resource": "resource", "module": "module",
           "output": "output", "type": "type_alias", "func": "function", "metadata": "key"}
ON_EPERM = ""


def retrying(fn, *args):
    for attempt in range(12):
        try:
            return fn(*args)
        except PermissionError:
            if attempt == 11:
                raise
            if ON_EPERM:
                subprocess.run(ON_EPERM, shell=True, capture_output=True)
            time.sleep(1 + attempt)


def read_text(path):
    return retrying(lambda: open(path, encoding="utf-8", errors="replace").read())


# a read the host refused (EPERM), never the file's fault: retried
DENIED = re.compile(r"Access to the path .* is denied|Operation not permitted|Permission denied")


class ServerGone(RuntimeError):
    """A server exited or stopped answering."""


class Rpc:
    """A JSON-RPC peer over stdio with LSP framing (both servers speak it)."""

    def __init__(self, cmd, env):
        self.p = subprocess.Popen(cmd, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, env=env)
        self.q = queue.Queue()
        self.nid = 0
        threading.Thread(target=self._read, daemon=True).start()

    def _read(self):
        out = self.p.stdout
        while True:
            hdr = b""
            while not hdr.endswith(b"\r\n\r\n"):
                c = out.read(1)
                if not c:
                    self.q.put(None)
                    return
                hdr += c
            n = int([l for l in hdr.decode().split("\r\n") if l.lower().startswith("content-length")][0].split(":")[1])
            body = b""
            while len(body) < n:
                chunk = out.read(n - len(body))
                if not chunk:
                    self.q.put(None)
                    return
                body += chunk
            self.q.put(json.loads(body))

    def send(self, msg):
        b = json.dumps(msg).encode()
        try:
            self.p.stdin.write(b"Content-Length: %d\r\n\r\n" % len(b) + b)
            self.p.stdin.flush()
        except OSError as e:  # a broken pipe: the server is gone
            raise ServerGone(str(e))

    def request(self, method, params, timeout=300):
        self.nid += 1
        i = self.nid
        self.send({"jsonrpc": "2.0", "id": i, "method": method, "params": params})
        end = time.time() + timeout
        while True:
            try:
                m = self.q.get(timeout=max(1, end - time.time()))
            except queue.Empty:
                raise ServerGone("no answer to " + method)
            if m is None:
                raise ServerGone("server exited")
            if m.get("id") == i and "method" not in m:
                if "error" in m:
                    raise RuntimeError(json.dumps(m["error"])[:300])
                return m.get("result")
            if "method" in m and "id" in m:
                self.send({"jsonrpc": "2.0", "id": m["id"], "result": None})

    def notify(self, method, params):
        self.send({"jsonrpc": "2.0", "method": method, "params": params})

    def close(self):
        try:
            self.p.kill()
        except OSError:
            pass


def discover(subject):
    def walk():
        out = []

        def fail(e):
            raise e

        for dirpath, dirnames, filenames in os.walk(subject, onerror=fail):
            dirnames[:] = sorted(d for d in dirnames if d not in SKIP_DIRS)
            for fn in sorted(filenames):
                if fn.endswith(".bicep") or fn.endswith(".bicepparam"):
                    out.append(posixpath.relpath(os.path.join(dirpath, fn), subject).replace(os.sep, "/"))
        return out

    return retrying(walk)


def uri_of(path):
    return "file://" + path


def flatten(syms, out, parent=None):
    for s in syms or []:
        s["_parent"] = parent
        out.append(s)
        flatten(s.get("children"), out, s)
    return out


def contains(rng, line, ch):
    s, e = rng["start"], rng["end"]
    if (line, ch) < (s["line"], s["character"]):
        return False
    return (line, ch) <= (e["line"], e["character"])


LOCAL_RES = [
    re.compile(r"\bfor\s+([A-Za-z_][A-Za-z0-9_]*)\s+in\b"),
    re.compile(r"\bfor\s*\(\s*([A-Za-z_][A-Za-z0-9_]*)\s*,\s*([A-Za-z_][A-Za-z0-9_]*)\s*\)\s*in\b"),
    re.compile(r"\(\s*([A-Za-z_][A-Za-z0-9_]*)\s*(?:,\s*([A-Za-z_][A-Za-z0-9_]*)\s*)?(?:,\s*([A-Za-z_][A-Za-z0-9_]*)\s*)?\)\s*=>"),
    re.compile(r"(?<![\w)])\s*([A-Za-z_][A-Za-z0-9_]*)\s*=>"),
]
FUNC_RE = re.compile(r"^\s*func\s+[A-Za-z_][A-Za-z0-9_]*\s*\(([^)]*)\)")


def local_bindings(lines):
    """Candidate binding positions of locals (the language server decides
    which is a symbol: a candidate inside a string or comment resolves to
    nothing)."""
    out = []
    for i, ln in enumerate(lines):
        for rx in LOCAL_RES:
            for m in rx.finditer(ln):
                for g in range(1, (m.lastindex or 0) + 1):
                    if m.group(g):
                        out.append(({"line": i, "character": m.start(g)}, m.group(g)))
        m = FUNC_RE.match(ln)
        if m:
            base = m.start(1)
            for pm in re.finditer(r"([A-Za-z_][A-Za-z0-9_]*)\s+[A-Za-z_]", m.group(1)):
                out.append(({"line": i, "character": base + pm.start(1)}, pm.group(1)))
    return out


def params_keys(text):
    """The keys of a module body's first `params: {...}` object (one level)."""
    m = re.search(r"\bparams\s*:\s*\{", text)
    if not m:
        return []
    i, depth, keys = m.end(), 1, []
    at_key = True
    while i < len(text) and depth:
        c = text[i]
        if c == "'":
            if text.startswith("'''", i):
                j = text.find("'''", i + 3)
                i = len(text) if j < 0 else j + 3
                continue
            i += 1
            while i < len(text) and text[i] != "'":
                i += 2 if text[i] == "\\" else 1
            i += 1
            at_key = False
            continue
        if text.startswith("//", i):
            while i < len(text) and text[i] != "\n":
                i += 1
            continue
        if text.startswith("/*", i):
            j = text.find("*/", i + 2)
            i = len(text) if j < 0 else j + 2
            continue
        if c in "{[(":
            depth += 1
        elif c in "}])":
            depth -= 1
        elif c in "\n,":
            at_key = depth == 1
        elif depth == 1 and at_key:
            km = re.match(r"\s*([A-Za-z_][A-Za-z0-9_]*)\s*:", text[i:])
            if km:
                keys.append(km.group(1))
                i += km.end()
                at_key = False
                continue
            if not c.isspace():
                at_key = False
        i += 1
    return keys


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("-subject", required=True)
    ap.add_argument("-bicep", required=True)
    ap.add_argument("-dotnet", required=True)
    ap.add_argument("-langserver", required=True)
    ap.add_argument("-out", required=True)
    ap.add_argument("-files-out", default="")
    ap.add_argument("-summary-out", default="")
    ap.add_argument("-only", default="")
    ap.add_argument("-exclude", default="", help="a regular expression over relative paths left out")
    ap.add_argument("-on-eperm", default="")
    a = ap.parse_args()
    global ON_EPERM
    ON_EPERM = a.on_eperm
    subject = os.path.abspath(a.subject)
    env = dict(os.environ, DOTNET_CLI_TELEMETRY_OPTOUT="1", BICEP_TELEMETRY_DISABLED="1",
               DOTNET_ROOT=os.path.dirname(os.path.abspath(a.dotnet)))
    srv = {}

    def start_servers():
        for old in srv.values():
            old.close()
        for attempt in range(10):
            try:
                cli = Rpc([a.bicep, "jsonrpc", "--stdio"], env)
                srv["version"] = cli.request("bicep/version", {}, timeout=120)["version"]
                ls = Rpc([a.dotnet, a.langserver, "--stdio"], env)
                ls.request("initialize", {"processId": os.getpid(), "rootUri": uri_of(subject),
                                          "capabilities": {"textDocument": {"documentSymbol": {"hierarchicalDocumentSymbolSupport": True}}}}, timeout=300)
                ls.notify("initialized", {})
                srv["cli"], srv["ls"] = cli, ls
                return
            except (ServerGone, queue.Empty, OSError):
                if ON_EPERM:
                    subprocess.run(ON_EPERM, shell=True, capture_output=True)
                time.sleep(2 + attempt)
        raise SystemExit("bicep_oracle: the servers would not start")

    start_servers()
    version = srv.pop("version")

    files = [f for f in discover(subject) if (not a.only or re.search(a.only, f)) and not (a.exclude and re.search(a.exclude, f))]
    all_files = set(files)
    counts = {}
    failures = []

    def count(k, n=1):
        counts[k] = counts.get(k, 0) + n

    defs, refs, xrefs, accepted = [], set(), set(), []
    def one(rel):
        cli, ls = srv["cli"], srv["ls"]
        path = os.path.join(subject, rel)
        is_param = rel.endswith(".bicepparam")
        try:
            if is_param:
                r = cli.request("bicep/compileParams", {"path": path, "parameterOverrides": {}})
            else:
                r = cli.request("bicep/compile", {"path": path})
        except ServerGone:
            raise
        except RuntimeError as e:
            if DENIED.search(str(e)):
                raise ServerGone("a read the host refused: " + str(e)[:120])
            count("compile_failed")
            failures.append((rel, str(e)[:200]))
            return
        errs = [d for d in r.get("diagnostics", []) if d.get("level") == "Error"]
        if any(DENIED.search(d.get("message", "")) for d in errs):
            raise ServerGone("a read the host refused")
        if not r.get("success") or errs:
            count("compile_refused")
            failures.append((rel, (errs[0].get("code", "") + " " + errs[0].get("message", ""))[:200] if errs else "refused"))
            return
        text = read_text(path)
        lines = text.split("\n")
        ls.notify("textDocument/didOpen", {"textDocument": {"uri": uri_of(path), "languageId": "bicep-params" if is_param else "bicep", "version": 1, "text": text}})
        try:
            syms = ls.request("textDocument/documentSymbol", {"textDocument": {"uri": uri_of(path)}}) or []
        except ServerGone:
            raise
        except RuntimeError as e:
            count("symbols_failed")
            failures.append((rel, "documentSymbol: " + str(e)[:200]))
            ls.notify("textDocument/didClose", {"textDocument": {"uri": uri_of(path)}})
            return
        flat = flatten(syms, [])
        decls = []  # (symbol, keyword)
        imports = []
        for s in flat:
            ln = s["selectionRange"]["start"]["line"]
            src = lines[ln] if ln < len(lines) else ""
            m = re.match(r"\s*(param|var|resource|module|output|type|func|metadata|import)\b", src)
            kw = m.group(1) if m else None
            if kw is None and s.get("_parent") is None and re.match(r"\s*@", src):
                kw = None
            if kw == "import" or (kw is None and s["kind"] == 20 and "import" in src):
                imports.append(s)
                continue
            if kw not in KIND_OF:
                count("symbol_unclassified")
                continue
            decls.append((s, kw))
            defs.append({"path": rel, "name": s["name"], "line": ln + 1, "end": s["range"]["end"]["line"] + 1, "kind": KIND_OF[kw]})
        accepted.append(rel)

        def enclosing(line, ch):
            best = None
            for s, kw in decls:
                if contains(s["range"], line, ch):
                    if best is None or contains(best[0]["range"], s["range"]["start"]["line"], s["range"]["start"]["character"]):
                        best = (s, kw)
            return best

        targets = [(s, kw) for s, kw in decls] + [(s, "import") for s in imports]
        # locals: a loop's or a lambda's variables and a function's parameters
        # are symbols of their own; the language server resolves their uses
        # too, which are references of the declaration they sit in
        for pos, name in local_bindings(lines):
            targets.append(({"name": name, "selectionRange": {"start": pos, "end": {"line": pos["line"], "character": pos["character"] + len(name)}}}, "local"))
        import_users = {}  # import symbol name -> set of declaration names using it
        for s, kw in targets:
            try:
                locs = ls.request("textDocument/references", {"textDocument": {"uri": uri_of(path)}, "position": s["selectionRange"]["start"],
                                                               "context": {"includeDeclaration": False}}) or []
            except ServerGone:
                raise
            except RuntimeError:
                count("references_failed")
                continue
            for loc in locs:
                if loc["uri"] != uri_of(path):
                    continue
                st = loc["range"]["start"]
                if contains(s["selectionRange"], st["line"], st["character"]):
                    continue
                enc = enclosing(st["line"], st["character"])
                if enc is None:
                    count("use_outside_declarations")
                    continue
                if enc[0] is s:
                    continue
                refs.add((rel, enc[0]["name"], s["name"]))
                if kw == "import":
                    import_users.setdefault(s["name"], set()).add(enc[0]["name"])
        # modules: their params keys, and the file their path names
        if not is_param:
            try:
                g = cli.request("bicep/getDeploymentGraph", {"path": path})
            except ServerGone:
                raise
            except RuntimeError:
                g = {"nodes": []}
                count("graph_failed")
            mods = {n["name"]: n.get("relativePath", "") for n in g.get("nodes", []) if n.get("type") == "<module>"}
            for s, kw in decls:
                if kw != "module" or s["name"] not in mods:
                    continue
                mp = mods[s["name"]]
                if not mp:
                    continue
                # its params keys: each a parameter of the module's template
                # (the compile refuses another), local or restored from a registry
                r0, r1 = s["range"]["start"]["line"], s["range"]["end"]["line"]
                body = "\n".join(lines[r0:r1 + 1])
                for k in params_keys(body):
                    refs.add((rel, s["name"], mp + "." + k))
                if re.match(r"^(br|ts)[:/]", mp):
                    count("module_registry")
                    continue
                target = posixpath.normpath(posixpath.join(posixpath.dirname(rel), mp))
                if target.startswith("../") or not os.path.isfile(os.path.join(subject, target)):
                    count("module_outside_subject")
                    continue
                xrefs.add((rel, s["name"], target))
            # compile-time imports of a file of the subject
            for s in imports:
                ln = s["selectionRange"]["start"]["line"]
                stmt = " ".join(lines[ln:ln + 6])
                m = re.search(r"\bfrom\s+'([^']+)'", stmt)
                if not m:
                    continue
                target = posixpath.normpath(posixpath.join(posixpath.dirname(rel), m.group(1)))
                if target in all_files or os.path.isfile(os.path.join(subject, target)):
                    for user in import_users.get(s["name"], ()):
                        xrefs.add((rel, user, target))
        elif re.search(r"(?m)^\s*using\s+'", text):
            count("bicepparam_using")
        ls.notify("textDocument/didClose", {"textDocument": {"uri": uri_of(path)}})
    for rel in files:
        snap = (len(defs), set(refs), set(xrefs), len(accepted), dict(counts), len(failures))
        for attempt in range(4):
            try:
                one(rel)
                break
            except ServerGone as e:
                # a server that died mid-file (a read the host refused): undo the
                # file's partial results, restart both servers, read it again
                del defs[snap[0]:]
                refs.clear(); refs.update(snap[1])
                xrefs.clear(); xrefs.update(snap[2])
                del accepted[snap[3]:]
                counts.clear(); counts.update(snap[4])
                del failures[snap[5]:]
                if ON_EPERM:
                    subprocess.run(ON_EPERM, shell=True, capture_output=True)
                start_servers()
                if attempt == 3:
                    count("server_died")
                    failures.append((rel, "server died: " + str(e)[:160]))
    cli, ls = srv["cli"], srv["ls"]
    cli.close()
    ls.close()
    doc = {"tool": "bicep language server and CLI", "version": version, "tier": "A",
           "definitions": sorted(defs, key=lambda d: (d["path"], d["line"], d["name"])),
           "references": [{"path": p, "from": f, "to": t} for p, f, t in sorted(refs)],
           "cross_references": [{"path": p, "from": f, "to_path": t} for p, f, t in sorted(xrefs)]}
    with open(a.out, "w") as f:
        json.dump(doc, f, indent=1, sort_keys=True)
    if a.files_out:
        with open(a.files_out, "w") as f:
            f.write("\n".join(sorted(accepted)) + ("\n" if accepted else ""))
    summary = {"version": version, "files": len(files), "accepted": len(accepted), "counts": dict(sorted(counts.items())),
               "definitions": len(defs), "references": len(refs), "cross_references": len(xrefs),
               "failures": len(failures), "failure_examples": failures[:60]}
    if a.summary_out:
        with open(a.summary_out, "w") as f:
            json.dump(summary, f, indent=1)
    print(json.dumps({k: v for k, v in summary.items() if k != "failure_examples"}, indent=1), file=sys.stderr)


if __name__ == "__main__":
    main()
