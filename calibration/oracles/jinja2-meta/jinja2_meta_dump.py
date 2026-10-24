# jinja2_meta_dump.py — Jinja's own parse and template lookup as indexbench lang-oracle's
# neutral tool output (LB-Q-jinja2). Tier A for imports: the template names come from
# jinja2.meta.find_referenced_templates (extends, include, import and from-import with a
# constant name) and each is resolved by jinja2.FileSystemLoader over the search path
# this run is given — the importing file's directory, then each ancestor directory up
# to the subject root, the layout of a Flask `templates/` tree, a Kolla `docker/` tree
# and an Ansible role. Tier A for definitions too: macros, blocks and assignments are
# nodes of Jinja's own AST (lines are the opening tag's first line; `end` is the line of its `%}` from Jinja's lexer). Bench-only: never a product dependency.
#
#   python3 -I jinja2_meta_dump.py -subject <checkout> [-pythonpath <dir>] [-out <file>]
#
# Files Jinja cannot parse (a Salt or Ansible extension tag) are counted on stderr.
import json
import os
import sys

args = sys.argv[1:]
subject = None
pythonpath = None
outpath = None
while args:
    a = args.pop(0)
    if a == "-subject":
        subject = args.pop(0)
    elif a == "-out":
        outpath = args.pop(0)
    elif a == "-pythonpath":
        pythonpath = args.pop(0)
    else:
        sys.exit("usage: jinja2_meta_dump.py -subject <dir> [-pythonpath <dir>] [-out <file>] (unknown %s)" % a)
if not subject:
    sys.exit("usage: jinja2_meta_dump.py -subject <dir> [-pythonpath <dir>] [-out <file>]")
if pythonpath:
    sys.path.insert(0, pythonpath)

import jinja2
from jinja2 import Environment, FileSystemLoader, TemplateNotFound, meta, nodes

EXTS = (".j2", ".jinja", ".jinja2")
root = os.path.realpath(subject)

files = []
for dp, dn, fn in os.walk(root):
    dn[:] = sorted(d for d in dn if d != ".git")
    for f in sorted(fn):
        if f.endswith(EXTS):
            files.append(os.path.join(dp, f))

defs, calls, imports = [], [], []
unparsed = 0
dynamic = 0
for p in files:
    rel = os.path.relpath(p, root).replace(os.sep, "/")
    try:
        src = open(p, encoding="utf-8").read()
    except (UnicodeDecodeError, OSError):
        unparsed += 1
        continue
    env = Environment()
    try:
        ast = env.parse(src)
    except jinja2.TemplateSyntaxError:
        unparsed += 1
        continue
    d = os.path.dirname(p)
    roots = []
    while True:
        roots.append(d)
        if d == root:
            break
        d = os.path.dirname(d)
    loader = FileSystemLoader(roots)
    scratch = Environment(loader=loader)

    def resolve(name):
        try:
            _, filename, _ = loader.get_source(scratch, name)
        except TemplateNotFound:
            return None
        return os.path.relpath(os.path.realpath(filename), root).replace(os.sep, "/")

    # The opening tag's own lines, from Jinja's lexer: the line of its `{%` and of its
    # `%}`, keyed by the line and word of the tag's keyword (the AST's lineno).
    tags = {}
    begin = None
    kw = None
    for lineno, tok, value in env.lex(src):
        if tok == "block_begin":
            begin, kw = lineno, None
        elif tok == "name" and begin is not None and kw is None:
            kw = (lineno, value)
        elif tok == "block_end" and begin is not None:
            if kw:
                tags.setdefault(kw, []).append((begin, lineno))
            begin = kw = None

    def span(node, word):
        q = tags.get((node.lineno, word))
        if q:
            return q.pop(0)
        return (node.lineno, node.lineno)

    stmts = [(n.lineno, "macro", n) for n in ast.find_all(nodes.Macro)]
    stmts += [(n.lineno, "block", n) for n in ast.find_all(nodes.Block)]
    stmts += [(n.lineno, "set", n) for n in ast.find_all(nodes.Assign) if isinstance(n.target, nodes.Name)]
    for _, word, n in sorted(stmts, key=lambda t: (t[0], t[1])):
        a, b = span(n, "set" if word == "set" else word)
        name = n.target.name if word == "set" else n.name
        defs.append({"path": rel, "name": name, "line": a, "end": b, "kind": word})

    macros = sorted(((m.lineno, m.name) for m in ast.find_all(nodes.Macro)))
    for n in ast.find_all(nodes.Call):
        callee = None
        if isinstance(n.node, nodes.Name):
            callee = n.node.name
        elif isinstance(n.node, nodes.Getattr):
            callee = n.node.attr
        if callee and callee not in (
            "super", "caller", "range", "dict", "lipsum", "cycler", "joiner", "namespace", "varargs", "kwargs"
        ):
            calls.append({"path": rel, "from": "<module>", "to": callee})

    named = {t for t in meta.find_referenced_templates(ast) if t is not None}
    dynamic += sum(1 for t in meta.find_referenced_templates(ast) if t is None)
    for n in ast.find_all((nodes.Extends, nodes.Include, nodes.Import, nodes.FromImport)):
        if isinstance(n.template, nodes.Const) and n.template.value in named:
            to = resolve(n.template.value)
            if to and to != rel:
                imports.append({"path": rel, "line": n.lineno, "to_path": to, "module": n.template.value})

out = {
    "tool": "jinja2.meta.find_referenced_templates",
    "version": jinja2.__version__,
    "tier": "A",
    "definitions": defs,
    "calls": calls,
    "imports": imports,
}
sink = open(outpath, "w") if outpath else sys.stdout
json.dump(out, sink, indent=1, sort_keys=True)
sink.write("\n")
sys.stderr.write("jinja2-meta: %d files, %d unparsed, %d definitions, %d imports resolved, %d dynamic names\n" % (len(files), unparsed, len(defs), len(imports), dynamic))
