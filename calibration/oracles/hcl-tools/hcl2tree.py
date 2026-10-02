"""hcl2tree — an independent HCL2 reader for the HashiCorp HCL tool oracles
(LB-Q-terragrunt, LB-Q-hcl-tools). Bench-only; never a product dependency.

The parser is python-hcl2 (MIT), a Lark grammar of HCL2 that shares no code with
hashicorp/hcl's hclsyntax, which the fathom dialects use. What this module adds
over its parse tree is only the reading of that tree: top-level blocks and
attributes with their 1-based line ranges, a block's labels, an object
constructor's keys, and the references an expression makes (a traversal whose
root is one of the names asked for, a bare identifier when asked, a function
call by name).

Tier C: an independent parser library (design §1.3), not a tool that reports
the tool format's own reading of the file.
"""
import re
import hcl2
from lark import Tree, Token

CODE_ROOT = re.compile(r"[A-Za-z_][A-Za-z0-9_-]*")


def parse(src):
    """The Lark tree of src; raises on a syntax error."""
    return hcl2.parses_to_tree(src)


def _text(node):
    """The first token text under node (an identifier's name)."""
    if isinstance(node, Token):
        return str(node)
    for c in node.children:
        t = _text(c)
        if t is not None:
            return t
    return None


def string_value(node):
    """The literal text of a `string` node, None when it holds an interpolation."""
    if not isinstance(node, Tree) or node.data != "string":
        return None
    out = []
    for c in node.children:
        if isinstance(c, Tree) and c.data == "string_part":
            for p in c.children:
                if isinstance(p, Tree):
                    return None
                out.append(str(p))
    return "".join(out)


def literal_string(expr):
    """The value of an expression that is a plain quoted string, else None."""
    e = unwrap(expr)
    return string_value(e)


def unwrap(n):
    """Strip single-child expr_term wrappers."""
    while isinstance(n, Tree) and n.data == "expr_term" and len(n.children) == 1:
        n = n.children[0]
    return n


def items(body):
    """Top-level (or nested) items of a body node: ('block', type, labels, body,
    line, end) and ('attr', name, expr, line, end), in source order."""
    out = []
    for c in body.children:
        if not isinstance(c, Tree):
            continue
        if c.data == "attribute":
            ident = c.children[0]
            expr = [x for x in c.children if isinstance(x, Tree)][-1]
            out.append(("attr", _text(ident), expr, c.meta.line, c.meta.end_line))
        elif c.data == "block":
            typ = None
            labels = []
            inner = None
            for x in c.children:
                if isinstance(x, Tree) and x.data == "identifier" and typ is None:
                    typ = _text(x)
                elif isinstance(x, Tree) and x.data == "identifier":
                    labels.append(_text(x))
                elif isinstance(x, Tree) and x.data == "string":
                    labels.append(string_value(x) or "")
                elif isinstance(x, Tree) and x.data == "body":
                    inner = x
            out.append(("block", typ, labels, inner, c.meta.line, c.meta.end_line))
    return out


def top(tree):
    return items(tree.children[0])


def object_items(expr):
    """(key name or None, value expr, line, end) of an object constructor, None
    when expr is not an object."""
    e = unwrap(expr)
    if not isinstance(e, Tree) or e.data != "object":
        return None
    out = []
    for c in e.children:
        if isinstance(c, Tree) and c.data == "object_elem":
            k = c.children[0]
            kn = _text(k)
            ks = unwrap(k.children[0]) if k.children else None
            lit = string_value(ks) if isinstance(ks, Tree) and ks.data == "string" else None
            name = lit if lit is not None else (kn if isinstance(ks, Tree) and ks.data == "identifier" else None)
            val = c.children[-1]
            out.append((name, val, c.meta.line, c.meta.end_line))
    return out


def call(expr):
    """(name, [arg exprs]) of a function call expression, else None."""
    e = unwrap(expr)
    if not isinstance(e, Tree) or e.data != "function_call":
        return None
    name = _text(e.children[0])
    args = []
    for c in e.children:
        if isinstance(c, Tree) and c.data == "arguments":
            args = [x for x in c.children if isinstance(x, Tree)]
    return name, args


def tuple_items(expr):
    e = unwrap(expr)
    if isinstance(e, Tree) and e.data == "tuple":
        return [x for x in e.children if isinstance(x, Tree)]
    return [expr]


def _chain(node):
    """Flatten a get_attr_expr_term chain into (root identifier, [attr names or
    None for an index]) or None."""
    attrs = []
    cur = node
    while isinstance(cur, Tree):
        if cur.data == "expr_term" and len(cur.children) == 1:
            cur = cur.children[0]
            continue
        if cur.data == "get_attr_expr_term":
            acc = cur.children[1]
            ident = [x for x in acc.children if isinstance(x, Tree) and x.data == "identifier"]
            attrs.insert(0, _text(ident[0]) if ident else None)
            cur = cur.children[0]
            continue
        if cur.data in ("index_expr_term", "attr_splat_expr_term", "full_splat_expr_term"):
            attrs.insert(0, None)
            cur = cur.children[0]
            continue
        break
    if isinstance(cur, Tree) and cur.data == "identifier":
        return _text(cur), attrs
    return None


HEREDOC_INTERP = re.compile(r"\$\{([^}]*)\}")
TRAVERSAL = re.compile(r"(?<![A-Za-z0-9_.-])([A-Za-z_][A-Za-z0-9_-]*)((?:\.[A-Za-z_][A-Za-z0-9_-]*|\[[^\]]*\])*)")


def refs(node, roots, bare=False, funcs=()):
    """Every reference under node: (root, [attrs...], line). roots: the
    traversal roots that count; bare: an identifier standing alone counts when
    it is in roots; funcs: function names that count (reported as ('()', [name])).
    """
    out = []

    def add(root, attrs, line):
        out.append((root, attrs, line))

    def walk(n):
        if not isinstance(n, Tree):
            return
        d = n.data
        if d in ("get_attr_expr_term", "index_expr_term", "attr_splat_expr_term", "full_splat_expr_term"):
            ch = _chain(n)
            if ch and ch[0] in roots:
                add(ch[0], ch[1], n.meta.line)
            # index expressions inside the chain
            cur = n
            while isinstance(cur, Tree) and cur.data in ("get_attr_expr_term", "index_expr_term", "attr_splat_expr_term", "full_splat_expr_term", "expr_term"):
                for c in cur.children[1:]:
                    walk(c)
                cur = cur.children[0] if cur.children else None
            if ch is None and isinstance(cur, Tree):
                walk(cur)
            return
        if d == "identifier":
            if bare and _text(n) in roots:
                add(_text(n), [], n.meta.line)
            return
        if d == "function_call":
            name = _text(n.children[0])
            if name in funcs:
                add("()", [name], n.meta.line)
            for c in n.children[1:]:
                walk(c)
            return
        if d == "object_elem":
            walk(n.children[-1])
            return
        if d == "for_intro":
            for c in n.children:
                if isinstance(c, Tree) and c.data != "identifier":
                    walk(c)
            return
        if d in ("heredoc_template", "heredoc_template_trim"):
            tok = str(n.children[0])
            for m in HEREDOC_INTERP.finditer(tok):
                for t in TRAVERSAL.finditer(m.group(1)):
                    root = t.group(1)
                    if root in roots:
                        attrs = [a for a in re.findall(r"\.([A-Za-z_][A-Za-z0-9_-]*)|(\[)", t.group(2))]
                        attrs = [a[0] or None for a in attrs]
                        add(root, attrs, n.meta.line + str(tok)[: m.start()].count("\n"))
            return
        for c in n.children:
            walk(c)

    walk(node)
    return out
