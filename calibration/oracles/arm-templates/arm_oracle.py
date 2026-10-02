"""arm_oracle.py - the language-breadth harness's oracle for the arm-templates
doc-engine dialect (design section 1.3, 13.4 step 19; LB-Q-arm-templates).

    python3 arm_oracle.py -subject <checkout> -bicep <bicep CLI> -scratch <dir> \
        -out <file> [-xref-out <file>] [-files-out <file>] [-summary-out <file>] [-jobs N]

Tier A: the Bicep CLI's decompiler (`bicep decompile`, `bicep decompile-params`;
Azure/bicep, MIT), which reads a template with Azure's own template and expression
libraries (Azure.Deployments.Templates, Azure.Deployments.Expression) and writes
every parameter, variable, user function, resource and output as a Bicep
declaration, resolving each expression's parameters('x'), variables('y'),
reference(...) and resourceId(...) to the declaration it names, and a linked
template's templateLink (a relativePath, or a uri relative to the template's own
link) to a module path. A file is an ARM file when its root `$schema` names a
deployment template (deploymentTemplate.json and its subscription-,
managementGroup- and tenant-level variants) or a parameters file
(deploymentParameters.json); a template the decompiler refuses is counted with
its error and left out.

The adapter (bench-only, standard library only) reads each file with an
independent JSONC reader of its own (comments and trailing commas, as ARM
accepts them) for every entity's name as written and its first and last line,
and joins the decompiler's declarations to them: parameters, variables and
outputs by name (the decompiler's identifier rule: a character outside
[A-Za-z0-9_] is `_`, a clash takes a suffix), user functions by
`<namespace>_<member>`, resources and deployments in document order by type (a
nested child resource follows its parent; a Microsoft.Resources/deployments is
a module). An entity the decompiler does not declare is left out and counted.
An inline nested template the decompiler writes as its own module file is read
the same way (its inner-scope declarations; an outer-scope one's synthesized
parameters stand for the outer names they carry).

  - definitions: a `package` per template (named by the file, at its root),
    and a `variable_decl` per parameter (a parameters file's entries too), a
    `variable` per variable (a `copy` variable by its name), a `function` per
    user function member, a `resource` per resource (named by its `name` as
    written; a languageVersion 2.0 resource by its symbolic name) and an
    `output` per output, inline nested templates' included;
  - references: within one decompiled file, every declaration a declaration's
    body names (a Bicep identifier that is a parameter, variable, user
    function, resource or module of the file: not a property key, a member
    after `.`, a loop or lambda variable, a builtin call or a string's text);
  - cross_references (-xref-out): a module whose path is a local file of the
    subject (the decompiler's resolution of templateLink.relativePath and of
    uri(deployment().properties.templateLink.uri, '<path>')), from the
    deployment to that template, file granularity. Every other templateLink.uri
    (an _artifactsLocation, a variable, an https URL) is counted, never drawn.

Directories the connector never reads (testdata, vendor, node_modules,
generated, .git and the editor directories) are not walked.
"""

import argparse
import bisect
import concurrent.futures
import json
import os
import posixpath
import re
import shutil
import subprocess
import sys

SKIP_DIRS = {"vendor", ".git", "testdata", "generated", "node_modules", ".idea", ".vscode", ".claude"}
SNIFF = re.compile(r'"\$schema"\s*:\s*"[^"]*(?i:deploymenttemplate|deploymentparameters)\.json')
TEMPLATE_RE = re.compile(r"(?i)deploymenttemplate\.json")
PARAMS_RE = re.compile(r"(?i)deploymentparameters\.json")
DEPLOYMENTS = "microsoft.resources/deployments"
IDENT = re.compile(r"[A-Za-z_][A-Za-z0-9_]*")
KEYWORDS = {"param", "var", "resource", "module", "output", "func", "type", "metadata", "existing", "for",
            "in", "if", "true", "false", "null", "targetScope", "import", "using", "with", "as", "extension",
            "assert", "test"}

# ---------------------------------------------------------------- JSONC reader


class Node:
    __slots__ = ("kind", "value", "keys", "vals", "items", "line", "end")

    def __init__(self, kind, line):
        self.kind, self.line, self.end = kind, line, line
        self.value, self.keys, self.vals, self.items = None, [], [], []

    def get(self, k):
        if self.kind != "map":
            return None
        for kk, v in zip(self.keys, self.vals):
            if kk == k:
                return v
        return None

    def str(self):
        return self.value if self.kind == "str" else None


def strip_jsonc(text):
    """Comments become spaces (newlines kept), then a trailing comma before a
    closing bracket is removed: two passes, so a comment between them counts
    as space."""
    out = list(text)
    i, n, in_str = 0, len(text), False
    while i < n:
        c = text[i]
        if in_str:
            if c == "\\":
                i += 2
                continue
            if c == '"':
                in_str = False
        elif c == '"':
            in_str = True
        elif text.startswith("//", i):
            while i < n and text[i] != "\n":
                out[i] = " "
                i += 1
            continue
        elif text.startswith("/*", i):
            j = text.find("*/", i + 2)
            j = n if j < 0 else j + 2
            for k in range(i, j):
                if text[k] != "\n":
                    out[k] = " "
            i = j
            continue
        i += 1
    t = "".join(out)
    out = list(t)
    i, in_str = 0, False
    while i < n:
        c = t[i]
        if in_str:
            if c == "\\":
                i += 2
                continue
            if c == '"':
                in_str = False
        elif c == '"':
            in_str = True
        elif c == ",":
            k = i + 1
            while k < n and t[k] in " \t\r\n":
                k += 1
            if k < n and t[k] in "}]":
                out[i] = " "
        i += 1
    return "".join(out)


class Reader:
    def __init__(self, text):
        self.s = strip_jsonc(text.lstrip("﻿"))
        self.nl = [i for i, c in enumerate(self.s) if c == "\n"]
        self.i = 0

    def line(self, pos):
        return bisect.bisect_left(self.nl, pos) + 1

    def ws(self):
        s, n = self.s, len(self.s)
        while self.i < n and s[self.i] in " \t\r\n":
            self.i += 1

    def string(self):
        s, i = self.s, self.i + 1
        buf = []
        while True:
            c = s[i]
            if c == '"':
                self.i = i + 1
                return "".join(buf)
            if c == "\\":
                e = s[i + 1]
                if e == "u":
                    buf.append(chr(int(s[i + 2:i + 6], 16)))
                    i += 6
                    continue
                buf.append({"n": "\n", "t": "\t", "r": "\r", "b": "\b", "f": "\f"}.get(e, e))
                i += 2
                continue
            buf.append(c)
            i += 1

    def value(self):
        self.ws()
        s, start = self.s, self.i
        c = s[start]
        if c == "{":
            n = Node("map", self.line(start))
            self.i += 1
            self.ws()
            if s[self.i] == "}":
                n.end = self.line(self.i)
                self.i += 1
                return n
            while True:
                self.ws()
                if s[self.i] != '"':
                    raise ValueError("key expected at line %d" % self.line(self.i))
                k = self.string()
                self.ws()
                if s[self.i] != ":":
                    raise ValueError("':' expected at line %d" % self.line(self.i))
                self.i += 1
                v = self.value()
                n.keys.append(k)
                n.vals.append(v)
                self.ws()
                if s[self.i] == ",":
                    self.i += 1
                    continue
                if s[self.i] == "}":
                    n.end = self.line(self.i)
                    self.i += 1
                    return n
                raise ValueError("',' or '}' expected at line %d" % self.line(self.i))
        if c == "[":
            n = Node("seq", self.line(start))
            self.i += 1
            self.ws()
            if s[self.i] == "]":
                n.end = self.line(self.i)
                self.i += 1
                return n
            while True:
                n.items.append(self.value())
                self.ws()
                if s[self.i] == ",":
                    self.i += 1
                    continue
                if s[self.i] == "]":
                    n.end = self.line(self.i)
                    self.i += 1
                    return n
                raise ValueError("',' or ']' expected at line %d" % self.line(self.i))
        if c == '"':
            n = Node("str", self.line(start))
            n.value = self.string()
            return n
        m = re.compile(r"-?[0-9][0-9.eE+-]*|true|false|null").match(s, start)
        if not m:
            raise ValueError("value expected at line %d" % self.line(start))
        self.i = m.end()
        n = Node("null" if m.group(0) == "null" else "lit", self.line(start))
        n.value = m.group(0)
        return n


def read_json(text):
    r = Reader(text)
    n = r.value()
    r.ws()
    if r.i != len(r.s):
        raise ValueError("trailing content at line %d" % r.line(r.i))
    return n


# ---------------------------------------------------------------- the ARM model


class Scope:
    """One template's declarations, as written: the root or an inline nested one."""

    def __init__(self):
        self.params, self.vars, self.funcs, self.resources, self.outputs = [], [], [], [], []
        self.nested = {}  # index into resources -> Scope of its inline template
        self.outer = {}  # index into resources -> True when its inline template is outer-scope


def ent(name, node):
    return {"name": name, "line": node.line, "end": node.end}


def read_scope(t):
    sc = Scope()
    p = t.get("parameters")
    if p is not None and p.kind == "map":
        for k, v in zip(p.keys, p.vals):
            if v.kind == "map" and v.get("type") is not None:
                sc.params.append(ent(k, v))
    v = t.get("variables")
    if v is not None and v.kind == "map":
        for k, vv in zip(v.keys, v.vals):
            if k == "copy":
                if vv.kind == "seq":
                    for it in vv.items:
                        nm = it.get("name")
                        if it.kind == "map" and nm is not None and nm.str():
                            sc.vars.append(ent(nm.str(), it))
                continue
            sc.vars.append(ent(k, vv))
    f = t.get("functions")
    if f is not None and f.kind == "seq":
        for ns in f.items:
            nsn, mem = ns.get("namespace"), ns.get("members")
            if nsn is None or mem is None or mem.kind != "map":
                continue
            for k, vv in zip(mem.keys, mem.vals):
                if vv.kind == "map":
                    e = ent(k, vv)
                    e["bicep"] = sanitize(nsn.str() or "") + "_" + sanitize(k)
                    sc.funcs.append(e)
    r = t.get("resources")

    def add_resources(node, symbolic, parent_type=""):
        if node is None:
            return
        pairs = []
        if node.kind == "seq":
            pairs = [(None, it) for it in node.items]
        elif node.kind == "map" and symbolic:
            pairs = list(zip(node.keys, node.vals))
        for key, it in pairs:
            if it.kind != "map" or it.get("type") is None or it.get("apiVersion") is None:
                continue
            nm = key if key is not None else (it.get("name").str() if it.get("name") is not None else None)
            if not nm:
                continue
            e = ent(nm, it)
            t = it.get("type").str() or ""
            # a nested child's type is relative to its parent's
            e["type"] = parent_type + "/" + t if parent_type and "/" not in t.split(".", 1)[0] and not re.match(r"^[A-Za-z0-9]+\.[A-Za-z0-9.]+/", t) else t
            idx = len(sc.resources)
            sc.resources.append(e)
            props = it.get("properties")
            tmpl = props.get("template") if props is not None else None
            if e["type"].lower() == DEPLOYMENTS and tmpl is not None and tmpl.kind == "map":
                sc.nested[idx] = read_scope(tmpl)
                opts = props.get("expressionEvaluationOptions")
                scope = opts.get("scope").str() if opts is not None and opts.get("scope") is not None else "outer"
                sc.outer[idx] = (scope or "outer").lower() != "inner"
            add_resources(it.get("resources"), False, e["type"])

    add_resources(r, True)
    o = t.get("outputs")
    if o is not None and o.kind == "map":
        for k, vv in zip(o.keys, o.vals):
            if vv.kind == "map":
                sc.outputs.append(ent(k, vv))
    return sc


def sanitize(name):
    s = re.sub(r"[^A-Za-z0-9_]", "_", name)
    if s and s[0].isdigit():
        s = "_" + s
    return s


# ---------------------------------------------------------------- the decompiled Bicep


DECL = re.compile(r"^(param|var|resource|module|output|func|type|metadata)\s+([A-Za-z_][A-Za-z0-9_]*)(.*)$")


def bicep_decls(text):
    """Top-level declarations: (keyword, name, rest-of-header, body text). A
    body runs to the next column-0 decorator or declaration (a decorator
    belongs to the declaration below it)."""
    lines = text.split("\n")
    starts = []
    for i, ln in enumerate(lines):
        m = DECL.match(ln)
        if m:
            starts.append((i, m.group(1), m.group(2), m.group(3)))
    out = []
    for i, kw, name, rest in starts:
        k = i + 1
        while k < len(lines) and not (lines[k].startswith("@") or DECL.match(lines[k])):
            k += 1
        out.append((kw, name, rest, "\n".join(lines[i:k])))
    return out


def identifiers(body):
    """Bicep identifiers a declaration's body uses as values (see the module
    doc): (name, followed-by-call) pairs, and the body's local names."""
    out, local = [], set()
    i, n = 0, len(body)
    prev_sig = ""  # previous significant character
    line_start = True

    def scan_expr(s, a, b):
        """Scan s[a:b] as an expression (strings' interpolations recursed)."""
        res = []
        j, last = a, ""
        at_line_start = True
        while j < b:
            c = s[j]
            if c == "\n":
                at_line_start = True
                j += 1
                continue
            if c in " \t\r":
                j += 1
                continue
            if s.startswith("//", j):
                while j < b and s[j] != "\n":
                    j += 1
                continue
            if s.startswith("/*", j):
                k = s.find("*/", j + 2)
                j = b if k < 0 else k + 2
                continue
            if s.startswith("'''", j):
                k = s.find("'''", j + 3)
                j = b if k < 0 else k + 3
                last, at_line_start = "'", False
                continue
            if c == "'":
                j += 1
                while j < b and s[j] != "'":
                    if s[j] == "\\":
                        j += 2
                        continue
                    if s.startswith("${", j):
                        depth, k = 1, j + 2
                        while k < b and depth:
                            if s[k] == "{":
                                depth += 1
                            elif s[k] == "}":
                                depth -= 1
                            elif s[k] == "'":  # a nested string inside an interpolation
                                k += 1
                                while k < b and s[k] != "'":
                                    k += 2 if s[k] == "\\" else 1
                            k += 1
                        res.extend(scan_expr(s, j + 2, k - 1))
                        j = k
                        continue
                    j += 1
                j += 1
                last, at_line_start = "'", False
                continue
            m = IDENT.match(s, j)
            if m and (j == 0 or not (s[j - 1].isalnum() or s[j - 1] == "_")):
                name = m.group(0)
                k = m.end()
                while k < b and s[k] in " \t":
                    k += 1
                nxt = s[k] if k < b else ""
                key = at_line_start and nxt == ":"
                res.append((name, last == "." or last == "@", key, nxt == "("))
                j, last, at_line_start = m.end(), "i", False
                continue
            last, at_line_start = c, False
            j += 1
        return res

    toks = scan_expr(body, 0, n)
    for k, (name, member, key, call) in enumerate(toks):
        if name in ("for",) and k + 1 < len(toks):
            local.add(toks[k + 1][0])
    for m in re.finditer(r"\bfor\s*\(\s*([A-Za-z_]\w*)\s*,\s*([A-Za-z_]\w*)\s*\)", body):
        local.update(m.groups())
    for m in re.finditer(r"\(\s*([A-Za-z_]\w*)\s*(?:,\s*([A-Za-z_]\w*)\s*)?\)\s*=>|\b([A-Za-z_]\w*)\s*=>", body):
        local.update(x for x in m.groups() if x)
    for name, member, key, call in toks:
        if member or key or name in KEYWORDS:
            continue
        out.append((name, call))
    return out, local


def strip_params_block(text):
    """A module body less its `params: { ... }` block (the decompiler writes
    it at two spaces' indentation, closed at the same)."""
    lines = text.split("\n")
    out, skip = [], False
    for ln in lines:
        if not skip and re.match(r"^  params: \{\s*$", ln):
            skip = True
            continue
        if skip:
            if ln.startswith("  }"):
                skip = False
            continue
        out.append(ln)
    return "\n".join(out)


# ---------------------------------------------------------------- the join


def align_by_name(arm, decl_names, key=lambda e: sanitize(e["name"])):
    """ARM entity index -> Bicep name: a name equal to the sanitized one, or it
    with the decompiler's clash suffix; the rest in order."""
    out, used = {}, set()
    pool = list(decl_names)
    for i, e in enumerate(arm):
        want = key(e)
        for cand in [want] + [want + s for s in ("_var", "_param", "_resource", "_output", "_module", "_func")]:
            if cand in pool and cand not in used:
                out[i] = cand
                used.add(cand)
                break
    rest = [d for d in pool if d not in used]
    for i, e in enumerate(arm):
        if i not in out and rest:
            out[i] = rest.pop(0)
    return out


def align_resources(arm, decls):
    """ARM resource index -> Bicep symbol, in order by type."""
    out = {}
    j = 0
    for i, e in enumerate(arm):
        want = e["type"].lower()
        k = j
        while k < len(decls):
            kw, sym, typ = decls[k]
            have = DEPLOYMENTS if kw == "module" else typ.lower()
            if have == want:
                out[i] = sym
                j = k + 1
                break
            k += 1
    return out


class Result:
    def __init__(self):
        self.defs, self.refs, self.xrefs, self.xref_candidates = [], [], [], []
        self.counts = {}

    def count(self, k, n=1):
        self.counts[k] = self.counts.get(k, 0) + n


def join_scope(rel, sc, bicep_text, work_dir, bicep_dir, res, outer_names=None):
    """Join one scope to one decompiled file; returns nothing (fills res)."""
    decls = bicep_decls(bicep_text)
    by_kw = {}
    for kw, name, rest, body in decls:
        by_kw.setdefault(kw, []).append(name)
    to_arm = {}  # Bicep name -> ARM name
    defs = []
    # parameters (an outer-scope module's synthesized ones carry outer names)
    params = [d for d in by_kw.get("param", [])]
    synth = {}
    if outer_names is not None:
        # an outer-scope template's parameters are the outer names it reads,
        # carried in by the module: `x` for an outer parameter x,
        # `variables_y` for an outer variable y
        for d in list(params):
            m = re.match(r"^(?:parameters|variables)_(.+)$", d)
            if d in outer_names:
                synth[d] = outer_names[d]
            elif m and m.group(1) in outer_names:
                synth[d] = outer_names[m.group(1)]
            else:
                continue
            params.remove(d)
    pm = align_by_name(sc.params, params)
    for i, e in enumerate(sc.params):
        if i in pm:
            to_arm[pm[i]] = e["name"]
            defs.append(("variable_decl", e))
        else:
            res.count("param_unattested")
    to_arm.update(synth)
    vm = align_by_name(sc.vars, by_kw.get("var", []))
    for i, e in enumerate(sc.vars):
        if i in vm:
            to_arm[vm[i]] = e["name"]
            defs.append(("variable", e))
        else:
            res.count("var_unattested")
    fm = align_by_name(sc.funcs, by_kw.get("func", []), key=lambda e: e["bicep"])
    for i, e in enumerate(sc.funcs):
        if i in fm:
            to_arm[fm[i]] = e["name"]
            defs.append(("function", e))
        else:
            res.count("func_unattested")
    rdecls = []
    module_path = {}
    for kw, name, rest, body in decls:
        if kw == "resource":
            m = re.match(r"\s*'([^'@]+)@", rest)
            rdecls.append((kw, name, m.group(1) if m else ""))
        elif kw == "module":
            m = re.match(r"\s*'([^']*)'", rest)
            rdecls.append((kw, name, ""))
            module_path[name] = m.group(1) if m else ""
    rm = align_resources(sc.resources, rdecls)
    for i, e in enumerate(sc.resources):
        if i in rm:
            to_arm[rm[i]] = e["name"]
            defs.append(("resource", e))
        else:
            res.count("resource_unattested")
    om = align_by_name(sc.outputs, by_kw.get("output", []))
    out_arm = {}
    for i, e in enumerate(sc.outputs):
        if i in om:
            out_arm[om[i]] = e["name"]
            defs.append(("output", e))
        else:
            res.count("output_unattested")
    for kind, e in defs:
        res.defs.append({"path": rel, "name": e["name"], "line": e["line"], "end": e["end"], "kind": kind})
    # references within this file; an outer-scope module's params block is
    # the decompiler's carrier for the outer names its template reads, not a
    # use by the deployment (the template's resources are the users)
    outer_modules = {rm[i] for i in sc.nested if sc.outer.get(i) and i in rm}
    declared = set(to_arm)
    for kw, name, rest, body in decls:
        if kw == "output":
            src = out_arm.get(name)
        else:
            src = to_arm.get(name)
        if src is None:
            continue
        text = body[len(kw) + 1:].lstrip()[len(name):]  # the header's keyword and name are not uses
        if kw == "module" and name in outer_modules:
            text = strip_params_block(text)
        ids, local = identifiers(text)
        for ident, call in ids:
            if ident in local:
                continue
            if ident not in declared:
                continue
            if call and ident not in by_kw.get("func", []):
                continue
            if ident == name and kw != "output":
                continue  # a declaration naming itself (the decompiler's BCP079)
            tgt = to_arm[ident]
            res.refs.append({"path": rel, "from": src, "to": tgt})
    # nested scopes and linked templates
    for i, e in enumerate(sc.resources):
        sym = rm.get(i)
        if sym is None or sym not in module_path:
            continue
        mp = module_path[sym]
        if i in sc.nested:
            nf = posixpath.normpath(posixpath.join(bicep_dir, mp))
            try:
                nt = read_text(os.path.join(work_dir, nf))
            except OSError:
                res.count("nested_module_missing")
                continue
            outer = None
            if sc.outer.get(i):
                outer = {sanitize(x): x for x in to_arm.values()}
                outer.update(to_arm)
            join_scope(rel, sc.nested[i], nt, work_dir, posixpath.dirname(nf), res, outer)
            continue
        if not mp or mp == "?" or re.match(r"^(br|ts)[:/]|^[A-Za-z][A-Za-z0-9+.-]*://", mp):
            res.count("module_unresolved" if mp == "?" else "module_external")
            continue
        target = posixpath.normpath(posixpath.join(posixpath.dirname(rel), mp))
        res.xref_candidates.append({"path": rel, "from": e["name"], "to_path": target})


# ---------------------------------------------------------------- driver

ON_EPERM = ""  # -on-eperm: a command run before retrying a read the OS refused


def retrying(fn, *args):
    """fn(*args), retried when the OS refuses a read (a host whose volume
    access is revoked mid-run; -on-eperm names the host's repair)."""
    for attempt in range(12):
        try:
            return fn(*args)
        except PermissionError:
            if attempt == 11:
                raise
            if ON_EPERM:
                subprocess.run(ON_EPERM, shell=True, capture_output=True)
            import time
            time.sleep(1 + attempt)


def read_text(path):
    return retrying(lambda: open(path, encoding="utf-8", errors="replace").read())


def read_bytes(path):
    return retrying(lambda: open(path, "rb").read())


def discover(subject):
    def walk():
        out = []

        def fail(e):
            raise e

        for dirpath, dirnames, filenames in os.walk(subject, onerror=fail):
            dirnames[:] = sorted(d for d in dirnames if d not in SKIP_DIRS)
            for fn in sorted(filenames):
                if fn.lower().endswith(".json"):
                    p = os.path.join(dirpath, fn)
                    out.append(posixpath.relpath(p, subject).replace(os.sep, "/"))
        return out

    return retrying(walk)


def run(cmd, env):
    for attempt in range(8):
        try:
            cp = subprocess.run(cmd, capture_output=True, text=True, timeout=300, env=env)
        except subprocess.TimeoutExpired:
            return 124, "", "timeout"
        except PermissionError:
            cp = None
        # a read the host refused, or the runtime aborting on one (a signal,
        # an unhandled exception in Main), is retried; a decompilation the
        # tool refuses says so in its own words
        refused = ("Operation not permitted", "Failed to map file", "Access to the path", "Permission denied",
                   "Bicep.Cli.Program.<Main>", "Fatal error.")
        if cp is not None and cp.returncode >= 0 and not any(r in cp.stderr for r in refused):
            return cp.returncode, cp.stdout, cp.stderr
        if ON_EPERM:
            subprocess.run(ON_EPERM, shell=True, capture_output=True)
        import time
        time.sleep(1 + attempt)
    return cp.returncode if cp else 126, cp.stdout if cp else "", cp.stderr if cp else "permission"


def links(root):
    """Local files a template's deployments link (relativePath and the
    template-relative uri): copied beside it for the decompiler."""
    out = []

    def walk(n):
        if n is None:
            return
        if n.kind == "map":
            for k, v in zip(n.keys, n.vals):
                if k == "templateLink" and v.kind == "map":
                    rp, uri = v.get("relativePath"), v.get("uri")
                    if rp is not None and rp.str() and not rp.str().startswith("["):
                        out.append(rp.str())
                    if uri is not None and uri.str():
                        m = re.match(r"^\[uri\(\s*deployment\(\)\.properties\.templateLink\.uri\s*,\s*'([^']+)'\s*\)\]$", uri.str())
                        if m:
                            out.append(m.group(1))
                walk(v)
        elif n.kind == "seq":
            for it in n.items:
                walk(it)

    walk(root)
    return out


def uri_forms(root, res):
    def walk(n):
        if n is None:
            return
        if n.kind == "map":
            for k, v in zip(n.keys, n.vals):
                if k == "templateLink" and v.kind == "map":
                    uri = v.get("uri")
                    if uri is not None and uri.str() is not None:
                        u = uri.str()
                        if re.match(r"^\[uri\(\s*deployment\(\)\.properties\.templateLink\.uri", u):
                            res.count("uri_template_relative")
                        elif "_artifactsLocation" in u:
                            res.count("uri_artifacts_location")
                        elif u.startswith("http"):
                            res.count("uri_https")
                        else:
                            res.count("uri_other_expression")
                walk(v)
        elif n.kind == "seq":
            for it in n.items:
                walk(it)

    walk(root)


def process(job):
    idx, subject, rel, kind, bicep, scratch, env = job
    res = Result()
    src = read_text(os.path.join(subject, rel))
    try:
        root = read_json(src)
    except (ValueError, IndexError) as e:
        res.count("reader_refused")
        return res, rel, "reader: %s" % e
    work = os.path.join(scratch, "work", "%05d" % idx)
    shutil.rmtree(work, ignore_errors=True)
    dst = os.path.join(work, rel)
    os.makedirs(os.path.dirname(dst), exist_ok=True)
    retrying(shutil.copyfile, os.path.join(subject, rel), dst)
    if kind == "params":
        code, out, err = run([bicep, "decompile-params", dst, "--stdout"], env)
        if "Decompilation failed" in err or not out.strip():
            res.count("decompile_refused")
            return res, rel, err.strip().splitlines()[-1] if err.strip() else "exit %d" % code
        names = set(re.findall(r"^param\s+([A-Za-z_]\w*)", out, re.M))
        p = root.get("parameters")
        if p is not None and p.kind == "map":
            for k, v in zip(p.keys, p.vals):
                if v.kind == "map" and (v.get("value") is not None or v.get("reference") is not None):
                    if sanitize(k) in names or k in names:
                        res.defs.append({"path": rel, "name": k, "line": v.line, "end": v.end, "kind": "variable_decl"})
                    else:
                        res.count("param_value_unattested")
        res.count("params_files")
        return res, rel, ""
    for ln in links(root):
        s = posixpath.normpath(posixpath.join(posixpath.dirname(rel), ln))
        if s.startswith("..") or not os.path.isfile(os.path.join(subject, s)):
            continue
        d = os.path.join(work, s)
        os.makedirs(os.path.dirname(d), exist_ok=True)
        retrying(shutil.copyfile, os.path.join(subject, s), d)
    out_bicep = dst[:-5] + ".oracle.bicep"
    code, out, err = run([bicep, "decompile", dst, "--outfile", out_bicep, "--force"], env)
    # The exit status is non-zero whenever the generated Bicep has an error
    # diagnostic (a self-reference, an unresolved module path): the
    # decompilation itself failed only when the tool says so or wrote nothing.
    if "Decompilation failed" in err or not os.path.isfile(out_bicep):
        res.count("decompile_refused")
        msg = [l for l in err.splitlines() if "Decompilation failed" in l] or [l for l in err.splitlines() if "rror" in l]
        return res, rel, (msg[0] if msg else "exit %d" % code)[:300]
    text = read_text(out_bicep)
    sc = read_scope(root)
    res.defs.append({"path": rel, "name": posixpath.basename(rel), "line": root.line, "end": root.end, "kind": "package"})
    join_scope(rel, sc, text, work, posixpath.dirname(rel), res)
    uri_forms(root, res)
    res.count("templates")
    shutil.rmtree(work, ignore_errors=True)
    return res, rel, ""


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("-subject", required=True)
    ap.add_argument("-bicep", required=True)
    ap.add_argument("-scratch", required=True)
    ap.add_argument("-out", required=True)
    ap.add_argument("-xref-out", default="")
    ap.add_argument("-files-out", default="")
    ap.add_argument("-summary-out", default="")
    ap.add_argument("-jobs", type=int, default=4)
    ap.add_argument("-only", default="", help="a regular expression over relative paths (debugging)")
    ap.add_argument("-on-eperm", default="", help="a shell command run before retrying a read the OS refused")
    a = ap.parse_args()
    global ON_EPERM
    ON_EPERM = a.on_eperm
    subject = os.path.abspath(a.subject)
    env = dict(os.environ, DOTNET_CLI_TELEMETRY_OPTOUT="1", BICEP_TELEMETRY_DISABLED="1")
    ver = subprocess.run([a.bicep, "--version"], capture_output=True, text=True, env=env).stdout.strip()
    vm = re.search(r"version\s+([0-9][^\s]*)", ver)
    version = vm.group(1) if vm else ver
    os.makedirs(a.scratch, exist_ok=True)
    jobs = []
    for rel in discover(subject):
        if a.only and not re.search(a.only, rel):
            continue
        b = read_bytes(os.path.join(subject, rel))
        if not SNIFF.search(b.decode("utf-8", "replace")):
            continue
        try:
            root = read_json(b.decode("utf-8", "replace"))
        except (ValueError, IndexError):
            jobs.append((len(jobs), subject, rel, "template", a.bicep, a.scratch, env))
            continue
        sch = root.get("$schema").str() if root.kind == "map" and root.get("$schema") is not None else None
        if sch is None:
            continue
        if TEMPLATE_RE.search(sch) and root.get("resources") is not None:
            jobs.append((len(jobs), subject, rel, "template", a.bicep, a.scratch, env))
        elif PARAMS_RE.search(sch) and root.get("parameters") is not None:
            jobs.append((len(jobs), subject, rel, "params", a.bicep, a.scratch, env))
    arm_files = {j[2] for j in jobs if j[3] == "template"}
    total = Result()
    failures = []
    files = []
    with concurrent.futures.ThreadPoolExecutor(max_workers=a.jobs) as ex:
        for res, rel, why in ex.map(lambda j: retrying(process, j), jobs):
            total.defs.extend(res.defs)
            total.refs.extend(res.refs)
            for x in res.xref_candidates:
                # the decompiler writes the path whatever is there: a link is
                # in the truth when it names an ARM template of the subject
                if x["to_path"] in arm_files:
                    total.xrefs.append(x)
                else:
                    total.count("module_path_not_a_template")
            for k, v in res.counts.items():
                total.count(k, v)
            if why:
                failures.append((rel, why))
            elif res.defs:
                files.append(rel)
    refs = sorted({(r["path"], r["from"], r["to"]) for r in total.refs})
    doc = {"tool": "bicep decompile", "version": version, "tier": "A",
           "definitions": sorted(total.defs, key=lambda d: (d["path"], d["line"], d["name"], d["kind"])),
           "references": [{"path": p, "from": f, "to": t} for p, f, t in refs]}
    with open(a.out, "w") as f:
        json.dump(doc, f, indent=1, sort_keys=True)
    xr = sorted({(x["path"], x["from"], x["to_path"]) for x in total.xrefs})
    if a.xref_out:
        with open(a.xref_out, "w") as f:
            json.dump({"tool": "bicep decompile", "version": version, "tier": "A",
                       "definitions": [], "references": [],
                       "cross_references": [{"path": p, "from": fr, "to_path": t} for p, fr, t in xr]}, f, indent=1, sort_keys=True)
    if a.files_out:
        with open(a.files_out, "w") as f:
            f.write("\n".join(sorted(files)) + ("\n" if files else ""))
    summary = {"version": version, "files": len(jobs), "counts": dict(sorted(total.counts.items())),
               "definitions": len(total.defs), "references": len(refs), "cross_references": len(xr),
               "failures": len(failures), "failure_examples": failures}
    if a.summary_out:
        with open(a.summary_out, "w") as f:
            json.dump(summary, f, indent=1)
    print(json.dumps({k: v for k, v in summary.items() if k != "failure_examples"}, indent=1), file=sys.stderr)


if __name__ == "__main__":
    main()
