#!/usr/bin/env python3
"""miss_hit_dump.py: tier-B MATLAB L1 oracle (syntax only), a MISS_HIT parser dump.

Original driver over miss_hit_core's public parser classes (MATLAB_Lexer, Token_Buffer, MATLAB_Parser,
AST_Visitor). It imports nothing from MISS_HIT's own command-line tools.

    python -I miss_hit_dump.py <subject-dir> [--out FILE] [--only-list FILE]

Writes the neutral oracle JSON {tool,version,tier,definitions,disclose} that `indexbench lang-oracle
-tool-oracle` reads. --only-list writes the comma-joined repo-relative paths MISS_HIT parsed, for `-only`.
"""
import json
import os
import sys

from miss_hit_core import m_ast
from miss_hit_core.config import Config
from miss_hit_core.errors import Message_Handler, Error, ICE
from miss_hit_core.m_language import MATLAB_Latest_Language
from miss_hit_core.m_lexer import MATLAB_Lexer, Token_Buffer
from miss_hit_core.m_parser import MATLAB_Parser
from miss_hit_core.version import VERSION


DECODED = []


class Collect(m_ast.AST_Visitor):
    def __init__(self, tbuf, rel):
        self.tbuf = tbuf
        self.rel = rel
        self.defs = []

    def add(self, name, line, end, kind):
        name = str(name).split(".")[-1]
        self.defs.append({"path": self.rel, "name": name, "line": line, "end": end, "kind": kind})

    def class_end(self, node):
        for tok in self.tbuf.tokens:
            if tok.ast_link is node and tok.kind == "KEYWORD" and tok.value == "end":
                return tok.location.line
        return 0

    def visit(self, node, n_parent, relation):
        if isinstance(node, m_ast.Class_Definition):
            self.add(node.n_name, node.t_classdef.location.line, self.class_end(node), "class")
            for blk in node.l_properties:
                for item in blk.l_items:
                    if isinstance(item, m_ast.Entity_Constraints):
                        self.add(item.n_name, item.n_name.loc().line, 0, "property")
        elif isinstance(node, m_ast.Function_Definition):
            in_methods = isinstance(n_parent, m_ast.Special_Block) and n_parent.kind() == "methods"
            end = node.t_end.location.line if node.t_end is not None else 0
            self.add(node.n_sig.n_name, node.t_fun.location.line, end, "method" if in_methods else "function")
        elif isinstance(node, m_ast.Function_Signature):
            if isinstance(n_parent, m_ast.Special_Block) and n_parent.kind() == "methods":
                self.add(node.n_name, node.n_name.loc().line, 0, "method")


def parse(path, rel):
    with open(path, "rb") as fh:
        raw = fh.read()
    try:
        text = raw.decode("utf-8")
    except UnicodeDecodeError:
        # Lines are what the oracle reports, and every byte that is not UTF-8 is inside a line, so
        # replacing it keeps all line numbers; the count is disclosed.
        text = raw.decode("utf-8", errors="replace")
        DECODED.append(rel)
    text = text.replace("\r\n", "\n")
    mh = Message_Handler("debug")
    mh.register_file(path)
    cfg = Config()
    lexer = MATLAB_Lexer(MATLAB_Latest_Language(), mh, text, path)
    tbuf = Token_Buffer(lexer, cfg)
    cunit = MATLAB_Parser(mh, tbuf, cfg).parse_file()
    c = Collect(tbuf, rel)
    cunit.visit(None, c, "Root")
    return c.defs


def main():
    args = sys.argv[1:]
    root = args[0]
    out = args[args.index("--out") + 1] if "--out" in args else None
    only = args[args.index("--only-list") + 1] if "--only-list" in args else None
    defs, parsed, fail = [], [], {}
    total = 0
    for dp, dns, fns in os.walk(root):
        dns[:] = sorted(d for d in dns if d != ".git")
        for fn in sorted(fns):
            if not fn.endswith(".m"):
                continue
            total += 1
            path = os.path.join(dp, fn)
            rel = os.path.relpath(path, root).replace(os.sep, "/")
            try:
                defs.extend(parse(path, rel))
                parsed.append(rel)
            except Error:
                fail["parse error"] = fail.get("parse error", 0) + 1
            except ICE:
                fail["internal error"] = fail.get("internal error", 0) + 1
    defs.sort(key=lambda d: (d["path"], d["line"], d["kind"], d["name"]))
    disclose = ["miss-hit-core %s parser dump, %d .m files, %d parsed" % (VERSION, total, len(parsed))]
    if DECODED:
        disclose.append("%d files are not UTF-8 and were decoded with replacement characters (line numbers unchanged)" % len(DECODED))
    for k in sorted(fail):
        disclose.append("%d files MISS_HIT rejects (%s), left out with -only" % (fail[k], k))
    doc = {"tool": "miss-hit-core", "version": VERSION, "tier": "B", "definitions": defs, "disclose": disclose}
    fh = open(out, "w") if out else sys.stdout
    json.dump(doc, fh, indent=0, sort_keys=True)
    fh.write("\n")
    if out:
        fh.close()
    if only:
        with open(only, "w") as fh:
            fh.write(",".join(parsed))
    sys.stderr.write("%s: %d files, %d parsed, %d definitions, failures %s\n" % (root, total, len(parsed), len(defs), fail))


if __name__ == "__main__":
    main()
