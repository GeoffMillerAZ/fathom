#!/usr/bin/env python3
"""Oracle for the jupyter dialect (LB-Q-jupyter, design section 1.3, owner decision D4).

Writes indexbench lang-oracle's tool-oracle JSON (-tool-oracle, design section 10):

    jupyter_oracle.py --subject <checkout> --ctags <universal-ctags> [--files-out FILE]   # definitions, tier B
    jupyter_oracle.py --subject <checkout> --refs                                         # references, tier C

nbformat (the reference reader, nbformat.reads at version 4: a version 3 notebook is
upgraded) supplies each notebook's cells, source and language. The code cells are exported
the way the dialect reads them: one Python module per notebook, each cell's lines in order, a
line magic (`%x`, `!x`) replaced by `pass` at its indent, a cell whose first line is a cell
magic other than the Python-running ones skipped, a notebook whose language is not Python
skipped. ctags (tier B) lists that module's definitions; Python's own `ast` (tier C) lists its
calls. A tag's or call's line in the export is mapped to the line of the notebook file that
holds the source line, by a scan of the raw text in document order for each source string as
JSON writes it, which shares nothing with the dialect's token reader.

Standard library plus nbformat; ctags is an external program. Bench-only.
"""
import argparse
import ast
import json
import os
import subprocess
import sys
import tempfile

import nbformat

SKIP_DIRS = {".git", ".ipynb_checkpoints", "node_modules", "vendor", "testdata", ".idea", ".vscode"}
PYTHON_CELL_MAGICS = {"time", "timeit", "capture", "prun", "debug", "python", "python3", "pypy"}


def notebooks(subject):
    out = []
    for root, dirs, files in os.walk(subject):
        dirs[:] = sorted(d for d in dirs if d not in SKIP_DIRS)
        for f in sorted(files):
            if f.endswith(".ipynb"):
                out.append(os.path.relpath(os.path.join(root, f), subject).replace(os.sep, "/"))
    return out


def language(nb):
    md = nb.get("metadata", {})
    name = (md.get("language_info") or {}).get("name") or (md.get("kernelspec") or {}).get("language") or ""
    return name.lower()


def raw_pieces(raw, nb_src_elems):
    """For each source element (a str, in cell order) the 1-based line of the raw notebook text
    holding its JSON string, scanning forward from the previous one."""
    lines = []
    cursor = 0
    for elem in nb_src_elems:
        tok = json.dumps(elem, ensure_ascii=False)
        pos = raw.find(tok, cursor)
        if pos < 0:
            tok = json.dumps(elem, ensure_ascii=True)
            pos = raw.find(tok, cursor)
        if pos < 0:
            lines.append(None)
            continue
        lines.append(raw.count("\n", 0, pos + len(tok) - 1) + 1)
        cursor = pos + len(tok)
    return lines


def export(path):
    """-> (module text, per-line notebook line numbers) or None when the notebook is not read."""
    with open(path, encoding="utf-8") as fh:
        raw = fh.read()
    try:
        nb = nbformat.reads(raw, as_version=4)
    except Exception as e:  # unreadable notebook: nothing to export
        return None, str(e)
    lang = language(nb)
    if lang and lang != "python":
        return None, "language " + lang
    # The raw JSON writes each source as a string or a list of strings; find each element's line.
    try:
        doc = json.loads(raw)
    except Exception as e:
        return None, str(e)
    cells = doc.get("cells") or []
    elems = []
    for c in cells:
        if c.get("cell_type") != "code":
            continue
        s = c.get("source")
        if isinstance(s, str):
            elems.append(s)
        elif isinstance(s, list):
            elems.extend(x for x in s if isinstance(x, str))
    elem_lines = raw_pieces(raw, elems)
    out_lines, out_map, cell_texts = [], [], []
    k = 0
    for c in cells:
        if c.get("cell_type") != "code":
            continue
        s = c.get("source")
        pieces = [s] if isinstance(s, str) else [x for x in (s or []) if isinstance(x, str)]
        # virtual lines of the cell, each with the raw line of the piece holding its first byte
        full = "".join(pieces)
        starts, pos = [], 0
        for p in pieces:
            starts.append(pos)
            pos += len(p)
        cell_lines = []
        pos = 0
        while pos < len(full):
            nl = full.find("\n", pos)
            text = full[pos:] if nl < 0 else full[pos:nl]
            pi = max(i for i, st in enumerate(starts) if st <= pos)
            cell_lines.append((text, elem_lines[k + pi]))
            pos = len(full) if nl < 0 else nl + 1
        k += len(pieces)
        if not cell_lines:
            continue
        first = cell_lines[0][0].lstrip(" \t")
        if first.startswith("%%"):
            name = first[2:].split()[0] if first[2:].split() else ""
            if name not in PYTHON_CELL_MAGICS:
                continue
            cell_lines[0] = ("", cell_lines[0][1])
        this = []
        for text, ln in cell_lines:
            t = text.lstrip(" \t")
            if t.startswith("%") or t.startswith("!"):
                text = text[: len(text) - len(t)] + "pass"
            out_lines.append(text)
            out_map.append(ln)
            this.append(text)
        cell_texts.append("\n".join(this) + "\n")
    return ("\n".join(out_lines) + "\n", out_map, cell_texts), None


def run_ctags(ctags, module_text):
    with tempfile.TemporaryDirectory() as d:
        p = os.path.join(d, "export.py")
        with open(p, "w", encoding="utf-8") as fh:
            fh.write(module_text)
        r = subprocess.run([ctags, "--output-format=json", "--fields=+nKzSe", "--extras=-{anonymous}", "-f", "-", p],
                           capture_output=True, text=True, check=True)
    tags = []
    for line in r.stdout.splitlines():
        if not line.strip():
            continue
        t = json.loads(line)
        if t.get("_type") == "tag":
            tags.append(t)
    return tags


def callee_name(f):
    """The call's callee as the dialect names it: the receiver chain, argument and index contents dropped."""
    if isinstance(f, ast.Name):
        return f.id
    if isinstance(f, ast.Attribute):
        base = callee_name(f.value)
        return (base + "." + f.attr) if base else None
    if isinstance(f, ast.Call):
        base = callee_name(f.func)
        return (base + "()") if base else None
    if isinstance(f, ast.Subscript):
        return callee_name(f.value)
    return None


class CallWalker(ast.NodeVisitor):
    """Calls as the dialect attributes them: a call inside a function (at any depth) belongs to
    the innermost function, a decorator's or a default's to the function it decorates, a class
    body's statement-level assignment and a module-level assignment to the variable the first
    target names, a class's bases and decorators to the class, and any other module-level
    statement's to <module>. A call is named by its receiver chain with argument and index
    contents dropped."""

    def __init__(self):
        self.pairs = set()
        self.owners = []
        self.in_function = 0

    def owner(self):
        return self.owners[-1] if self.owners else "<module>"

    def visit_FunctionDef(self, n):
        self.owners.append(n.name)
        self.in_function += 1
        for d in n.decorator_list:
            self.visit(d)
        self.visit(n.args)
        if n.returns:
            self.visit(n.returns)
        for b in n.body:
            self.visit(b)
        self.in_function -= 1
        self.owners.pop()

    visit_AsyncFunctionDef = visit_FunctionDef

    def visit_ClassDef(self, n):
        self.owners.append(n.name)
        for d in n.decorator_list:
            self.visit(d)
        for b in n.bases:
            self.visit(b)
        for k in n.keywords:
            self.visit(k)
        self.owners.pop()
        # the body's statements are attributed one by one: an assignment to the variable it
        # defines, anything else to the class
        self.owners.append(n.name)
        for b in n.body:
            self.visit(b)
        self.owners.pop()

    def _assign(self, n, target):
        name = None
        if isinstance(target, ast.Name):
            name = target.id
        elif isinstance(target, (ast.Tuple, ast.List)) and target.elts and isinstance(target.elts[0], ast.Name):
            name = target.elts[0].id
        if name is not None and not self.in_function:
            self.owners.append(name)
            self.generic_visit(n)
            self.owners.pop()
        else:
            self.generic_visit(n)

    def visit_Assign(self, n):
        self._assign(n, n.targets[0])

    def visit_AnnAssign(self, n):
        self._assign(n, n.target)

    def visit_Call(self, n):
        name = callee_name(n.func)
        if name:
            self.pairs.add((self.owner(), name))
        self.generic_visit(n)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--subject", required=True)
    ap.add_argument("--ctags", default="ctags")
    ap.add_argument("--refs", action="store_true")
    ap.add_argument("--files-out")
    a = ap.parse_args()
    res = {"tool": "ctags on the exported cells, nbformat " + nbformat.__version__, "version": "", "tier": "B", "definitions": [], "references": []}
    if a.refs:
        res["tool"], res["tier"] = "python ast on the exported cells, nbformat " + nbformat.__version__, "C"
        res["version"] = "python " + sys.version.split()[0]
    else:
        v = subprocess.run([a.ctags, "--version"], capture_output=True, text=True).stdout.splitlines()[0]
        res["version"] = v
    ok_files, skipped = [], 0
    for rel in notebooks(a.subject):
        got, why = export(os.path.join(a.subject, rel))
        if got is None:
            skipped += 1
            print("skip %s: %s" % (rel, why), file=sys.stderr)
            continue
        text, mapping, cells = got
        ok_files.append(rel)
        if a.refs:
            # each cell is its own parse, as the dialect reads it: a cell that does not parse
            # contributes no call (the dialect's tolerant parse may still read some)
            pairs = set()
            for ct in cells:
                try:
                    tree = ast.parse(ct)
                except SyntaxError as e:
                    print("ast skip a cell of %s: %s" % (rel, e), file=sys.stderr)
                    continue
                w = CallWalker()
                w.visit(tree)
                pairs |= w.pairs
            for frm, to in sorted(pairs):
                res["references"].append({"path": rel, "from": frm, "to": to})
            continue
        for t in run_ctags(a.ctags, text):
            ln = mapping[t["line"] - 1] if 0 < t["line"] <= len(mapping) else None
            if ln is None:
                continue
            end = t.get("end")
            res["definitions"].append({"path": rel, "name": t["name"], "line": ln, "kind": t["kind"],
                                       "end": mapping[end - 1] if end and 0 < end <= len(mapping) and mapping[end - 1] else 0})
    print("notebooks %d, read %d, skipped %d, definitions %d, references %d" % (len(ok_files) + skipped, len(ok_files), skipped, len(res["definitions"]), len(res["references"])), file=sys.stderr)
    if a.files_out:
        with open(a.files_out, "w") as fh:
            fh.write("\n".join(ok_files) + "\n")
    json.dump(res, sys.stdout, indent=1)


if __name__ == "__main__":
    main()
