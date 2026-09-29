#!/usr/bin/env python3
"""swift_parse.py -- a tier-B Swift L1/L2 oracle from the Swift compiler's own parser.

`swiftc -frontend -parse-as-library -dump-parse <file>` prints the syntax tree the compiler
parses (no name lookup, no type checking, no dependencies to build) with a source range for
every declaration. This script reads that dump and writes `indexbench lang-oracle
-tool-oracle`'s neutral JSON: the definitions the swift dialect declares (with the name's
row and the declaration's last row) and the (enclosing definition, callee) call pairs.

Bench-only, standard library only, never a product dependency. See README.md.

    python3 swift_parse.py --root <subject dir> [--swiftc <path>] [--jobs N] > out.json
"""
import argparse, concurrent.futures, json, os, re, shutil, subprocess, sys, tempfile, threading

SKIP_DIRS = {".build", ".git", "node_modules", "Pods", "Carthage", "DerivedData", ".swiftpm"}

TOKEN = re.compile(
    r'\s*(?:'
    r'(?P<open>\()|(?P<close>\))'
    r'|(?P<str>"(?:[^"\\]|\\.)*")'
    r'|(?P<attr>[A-Za-z_][\w.$@]*=(?:"(?:[^"\\]|\\.)*"|\[[^\]]*\]|<[^>]*>|[^\s()"]+))'
    r'|(?P<angle><[^>]*>)|(?P<brack>\[[^\]]*\])'
    r'|(?P<word>[^\s()"]+)'
    r')')

def unquote(tok):
    """A dump string token's text: JSON escapes where valid, else the raw text (the dump writes
    Swift escapes and literal control characters in string literals' values)."""
    try:
        return json.loads(tok, strict=False)
    except ValueError:
        return tok[1:-1]

class Node:
    __slots__ = ("head", "attrs", "strs", "words", "kids", "parent")
    def __init__(self, head, parent=None):
        self.head, self.attrs, self.strs, self.words, self.kids, self.parent = head, {}, [], [], [], parent

def parse(text):
    """The dump as a tree of Nodes (iterative: dumps nest hundreds deep).

    The dump is pretty-printed, one node per line, two spaces per level, so a node's parent is
    the last node one level up. The parentheses are not trusted for structure: the compiler
    leaves one open after a function type (`typealias F = (Int) -> Void`), which would nest
    every later declaration of the file inside that alias."""
    root = Node("root")
    cur = root
    by_depth = []
    pos, n = 0, len(text)
    expect_head = False
    while pos < n:
        m = TOKEN.match(text, pos)
        if not m or m.end() == pos:
            break
        start = m.start(m.lastgroup)
        pos = m.end()
        if m.group("open"):
            nl = text.rfind("\n", 0, start)
            if text[nl + 1:start].strip(" ") == "":
                depth = (start - nl - 1) // 2
                depth = min(depth, len(by_depth))
                parent = by_depth[depth - 1] if depth > 0 else root
                del by_depth[depth:]
            else:  # an inline node: a child of the open one
                parent = cur
                depth = len(by_depth)
            node = Node("", parent)
            parent.kids.append(node)
            by_depth.append(node)
            cur = node
            expect_head = True
        elif m.group("close"):
            if cur.parent is not None:
                cur = cur.parent
            expect_head = False
        else:
            tok = m.group(m.lastgroup)
            if expect_head:
                # `original_init=call_expr` and `result=type_ident` name the child's role, then its kind;
                # `processed_init=...` repeats `original_init=...` and is dropped.
                cur.head = tok if tok.startswith("processed_init=") else tok.split("=", 1)[-1]
                expect_head = False
            elif m.group("str"):
                cur.strs.append(unquote(tok))
            elif m.group("attr"):
                k, _, v = tok.partition("=")
                if v.startswith('"'):
                    v = unquote(v)
                cur.attrs[k] = v
            else:
                cur.words.append(tok)
    return root

RANGE = re.compile(r':(\d+):(\d+)')
def rng(node):
    r = node.attrs.get("range")
    if not r:
        return None
    m = RANGE.findall(r)
    if len(m) < 2:
        return None
    return int(m[0][0]), int(m[1][0])

def bare(name):
    """A declaration's own name: a function's `f(_:x:)` is `f`, a nested or extended type's
    `Outer.Inner` is `Inner` (the neutral file carries bare names, design 1.4)."""
    name = name.split("(", 1)[0]
    if name.endswith("]") or name.endswith("?"):
        return name  # an array or optional sugar type an extension extends
    if name and name[0] in "<>-+*/%=!&|^~?.":
        return name  # an operator function: `<`, `..<`, `...`, `==`
    return re.sub(r"<[^<>]*>", "", name).rsplit(".", 1)[-1]

TYPE_HEADS = {"struct_decl": "struct", "enum_decl": "enum", "protocol": "protocol", "extension_decl": "extension"}
WRAPPERS = {"bind_optional_expr", "force_value_expr", "paren_expr", "try_expr", "await_expr",
            "optional_try_expr", "force_try_expr", "unresolved_specialize_expr", "inject_into_optional",
            "optional_evaluation_expr"}

def callee_name(e):
    h = e.head
    if "implicit" in e.words:  # a call the compiler wrote (string interpolation's appendInterpolation)
        return ""
    if h == "unresolved_decl_ref_expr":
        return bare(e.attrs.get("name", ""))
    if h == "unresolved_dot_expr":
        return bare(e.attrs.get("field", ""))
    if h == "unresolved_member_expr":
        return bare(e.attrs.get("name", ""))
    if h in WRAPPERS and e.kids:
        return callee_name(e.kids[0])
    return ""

class Def:
    __slots__ = ("name", "kind", "line", "end", "calls")
    def __init__(self, name, kind, line, end):
        self.name, self.kind, self.line, self.end, self.calls = name, kind, line, end, set()

def extract(path, text):
    """(definitions, call pairs) of one file's dump."""
    root = parse(text)
    defs, holders = [], []

    def add(name, kind, node, end=None):
        r = rng(node)
        if not name or r is None:
            return None
        d = Def(name, kind, r[0], end if end is not None else r[1])
        defs.append(d)
        return d

    def walk_children(node, ctx, enc):
        binding = None  # the last pattern_binding_decl among these siblings
        for k in node.kids:
            if k.head == "pattern_binding_decl" and ctx in ("type", "file"):
                binding = binding_holders(k, enc)
                continue
            if k.head == "var_decl" and ctx in ("type", "file"):
                r = rng(k)
                name = k.strs[0] if k.strs else ""
                end = binding[0] if binding else (r[1] if r else None)
                d = add(name, "property" if ctx == "type" else "variable", k, end)
                if d:
                    if binding:
                        for h in binding[1]:
                            if h["names"] and h["names"][0] == name and h["owner"] is None:
                                h["owner"] = d
                    walk_children(k, "body", d)
                continue
            walk(k, ctx, enc)

    def binding_holders(pbd, enc):
        r = rng(pbd)
        hs = []
        for entry in pbd.kids:
            if entry.head != "pattern_entry":
                continue
            names = []
            collect_names(entry, names)
            h = {"names": names, "owner": None, "calls": set()}
            hs.append(h)
            walk_children(entry, "body", h)
        holders.extend(hs)
        return (r[1] if r else None, hs)

    def collect_names(n, out):
        for k in n.kids:
            if k.head == "pattern_named" and k.strs:
                out.append(k.strs[0])
            elif k.head.startswith("processed_init") or k.head.startswith("original_init"):
                continue
            else:
                collect_names(k, out)

    def walk(node, ctx, enc):
        h = node.head
        if h.startswith("processed_init="):
            return
        name = node.strs[0] if node.strs else ""
        if h == "class_decl":
            d = add(bare(name), "actor" if "actor" in node.words else "class", node)
            walk_children(node, "type", enc)
            return
        if h in TYPE_HEADS:
            add(bare(name), TYPE_HEADS[h], node)
            walk_children(node, "type", enc)
            return
        if h == "typealias":
            add(name, "typealias", node)
            return
        if h == "associated_type_decl":
            add(name, "associatedtype", node)
            return
        if h == "enum_case_decl":
            return
        if h == "pattern_expr":
            # `case .a(let x):` and `if case .a(let x) = e` parse as call expressions inside a
            # pattern; a pattern is not a call.
            return
        if h == "enum_element_decl":
            add(bare(name), "enum_case", node)
            return
        if h == "func_decl":
            if name and not name.startswith("<anonymous") and not name.startswith("$") and "implicit" not in node.words:
                d = add(bare(name), "method" if ctx == "type" else "function", node)
                walk_children(node, "body", d or enc)
            else:
                walk_children(node, "body", enc)
            return
        if h in ("constructor_decl", "destructor_decl", "subscript_decl"):
            kind, nm = {"constructor_decl": ("init", "init"), "destructor_decl": ("deinit", "deinit"),
                        "subscript_decl": ("subscript", "subscript")}[h]
            d = add(nm, kind, node)
            walk_children(node, "body", d or enc)
            return
        if h == "macro_decl":
            add(bare(name), "macro", node)
            return
        if h == "call_expr":
            if node.kids:
                cn = callee_name(node.kids[0])
                if cn and enc is not None:
                    if isinstance(enc, Def):
                        enc.calls.add(cn)
                    else:
                        enc["calls"].add(cn)
            walk_children(node, ctx, enc)
            return
        if h == "accessor_decl":
            walk_children(node, "body", enc)
            return
        if h in ("brace_stmt", "closure_expr", "top_level_code_decl"):
            walk_children(node, "body" if ctx != "file" else "body", enc)
            return
        walk_children(node, ctx, enc)

    for sf in root.kids:
        if sf.head == "source_file":
            walk_children(sf, "file", None)
    for h in holders:
        if h["owner"] is not None:
            h["owner"].calls |= h["calls"]
    pairs = []
    for d in defs:
        for c in sorted(d.calls):
            pairs.append({"path": path, "from": d.name, "to": c})
    outdefs = [{"path": path, "name": d.name, "line": d.line, "end": d.end, "kind": d.kind} for d in defs]
    return outdefs, pairs

DIRECTIVE = re.compile(r"^[ \t]*#(?:if|elseif|else|endif)\b.*$", re.M)

def dump(swiftc, root, rel, tmp):
    """The compiler's dump of one file. `swiftc -dump-parse` prints nothing of an `#if` block
    (the compiler picks a branch by the build configuration and the dump omits the block), so
    the directive lines are blanked first, keeping every line number: every branch's
    declarations are parsed, as the tree-sitter grammar reads them."""
    with open(os.path.join(root, rel), "rb") as f:
        src = f.read().decode("utf-8", errors="replace")
    src = DIRECTIVE.sub("", src)
    tf = os.path.join(tmp, "%d_%s" % (threading.get_ident(), os.path.basename(rel)))
    with open(tf, "w", encoding="utf-8") as f:
        f.write(src)
    try:
        p = subprocess.run([swiftc, "-frontend", "-parse-as-library", "-dump-parse", tf],
                           capture_output=True, text=True, errors="replace")
    finally:
        os.unlink(tf)
    return rel, p.stdout, "error:" in p.stderr

def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--root", required=True)
    ap.add_argument("--swiftc", default="swiftc")
    ap.add_argument("--jobs", type=int, default=4)
    ap.add_argument("--only", action="append", default=[], help="a repo-relative file to read (repeatable); default every .swift file")
    a = ap.parse_args()
    root = os.path.abspath(a.root)
    files = list(a.only)
    if not files:
        for dp, dns, fns in os.walk(root):
            dns[:] = sorted(d for d in dns if d not in SKIP_DIRS)
            for f in sorted(fns):
                if f.endswith(".swift"):
                    files.append(os.path.relpath(os.path.join(dp, f), root).replace(os.sep, "/"))
    ver = subprocess.run([a.swiftc, "--version"], capture_output=True, text=True).stdout.splitlines()[0]
    m = re.search(r"Swift version ([0-9][^ ]*)", ver)
    version = m.group(1) if m else ver
    defs, calls, errs = [], [], 0
    sys.setrecursionlimit(100000)
    threading.stack_size(256 * 1024 * 1024)
    tmp = tempfile.mkdtemp(prefix="swift-parse-")
    with concurrent.futures.ThreadPoolExecutor(max_workers=a.jobs) as ex:
        for rel, out, had_err in ex.map(lambda f: dump(a.swiftc, root, f, tmp), files):
            errs += 1 if had_err else 0
            d, c = extract(rel, out)
            defs += d
            calls += c
    json.dump({"tool": "swiftc -frontend -dump-parse", "version": version, "tier": "B",
               "definitions": defs, "calls": calls}, sys.stdout, indent=None, separators=(",", ":"))
    shutil.rmtree(tmp, ignore_errors=True)
    print(f"files {len(files)} with_syntax_errors {errs} definitions {len(defs)} calls {len(calls)}", file=sys.stderr)

if __name__ == "__main__":
    main()
