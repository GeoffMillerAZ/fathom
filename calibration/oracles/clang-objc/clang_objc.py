#!/usr/bin/env python3
"""clang_objc.py — the build's own compiler front end as the Objective-C oracle (tier A).

libclang (the Command Line Tools' own libclang.dylib, driven through the `libclang` 18.1.1 Python
bindings) parses every Objective-C translation unit of a subject with the flags a build would pass
(-fobjc-arc, an SDK, the subject's include roots) and writes the neutral oracle file
`indexbench lang-oracle -tool-oracle` reads, plus a definition-range companion:

  calls          (path, from, to): a message send or a C call written in a definition of a subject file,
                 paired with the innermost enclosing definition. `to` is the selector (`initWithFrame:style:`)
                 or the C callee's name; `from` is the enclosing method's selector or function's name.
  call_exclusions a selector every send of which, in one file, has a receiver of the bare type `id`: the
                 compiler binds no class to it, so neither side's pair counts (counted in the reading).
  imports        (path, line, to_path): every #import / #include / @import the preprocessor entered that
                 resolves to a file of the subject, as the compiler resolved it (a conditional one it skipped
                 is none).
  <name>.ranges.json  the extent of every definition the compiler names in a subject file, for
                 range_agreement beside ctags' lines.

A translation unit that stopped on an error (a missing header, an unknown type) is not trusted: its files are
left out of calls and imports, listed under `errored` in the companion and counted in `disclose`. Bench-only;
never imported by fathom.

  python3 clang_objc.py --subject <checkout> --out <oracle.json> [--include DIR]... [--target mac|catalyst]
"""
import argparse
import collections
import json
import os
import subprocess
import sys

sys.path.insert(0, os.environ.get("CLANG_PYBIND", "/Volumes/m2ext/tmp/fathom-build/toolcache/pyclang"))
import clang.cindex as ci  # noqa: E402

LIBCLANG = os.environ.get("LIBCLANG", "/Library/Developer/CommandLineTools/usr/lib/libclang.dylib")
ci.Config.set_library_file(LIBCLANG)
CK = ci.CursorKind


class _UnknownKind:
    """A cursor kind the 18.1.1 bindings predate (the Command Line Tools' libclang is newer)."""
    name = "UNKNOWN"

    def __init__(self, id_):
        self.value = id_


_from_id = CK.from_id


def _safe_from_id(id_):
    try:
        return _from_id(id_)
    except ValueError:
        return _UnknownKind(id_)


CK.from_id = staticmethod(_safe_from_id)

MSG = {CK.OBJC_MESSAGE_EXPR}
METHODS = {CK.OBJC_INSTANCE_METHOD_DECL, CK.OBJC_CLASS_METHOD_DECL}
# What a reference to a declaration in another file is: a type, class or protocol named, a function, variable or
# enumerator used, a field or method bound, a macro expanded.
REFS = {CK.TYPE_REF, CK.OBJC_CLASS_REF, CK.OBJC_PROTOCOL_REF, CK.OBJC_SUPER_CLASS_REF, CK.DECL_REF_EXPR,
        CK.MEMBER_REF_EXPR, CK.MEMBER_REF, CK.OBJC_MESSAGE_EXPR, CK.MACRO_INSTANTIATION}
CALLABLE_DEFS = METHODS | {CK.FUNCTION_DECL}
SKIP_DIRS = {".git", "Pods", "Carthage", "vendor", "node_modules", "build", "DerivedData"}


def sdk_path():
    return subprocess.check_output(["xcrun", "--show-sdk-path"]).decode().strip()


def candidates(root):
    """Sources the dialect reads: .m and .mm (the sniff is the harness's), and every .h."""
    out = []
    for dp, dns, fns in os.walk(root):
        dns[:] = sorted(d for d in dns if d not in SKIP_DIRS and not d.startswith("."))
        for fn in sorted(fns):
            ext = os.path.splitext(fn)[1]
            if ext in (".m", ".mm", ".h"):
                out.append(os.path.relpath(os.path.join(dp, fn), root))
    return out


def looks_objc(src):
    import re
    return re.search(r"^\s*(@(interface|protocol|implementation|class|import)\b|#import\b)", src, re.M) is not None


CONFIGS = {
    "mac": ["-isysroot", "{sdk}"],
    "catalyst": ["-target", "arm64-apple-ios14.0-macabi", "-isysroot", "{sdk}", "-iframework",
                 "{sdk}/System/iOSSupport/System/Library/Frameworks"],
}


MODCACHE = os.environ.get("CLANG_MODULE_CACHE", "")


def parse(index, path, root, lang_args, includes, config, modules):
    sdk = sdk_path()
    args = [a.format(sdk=sdk) for a in CONFIGS[config]]
    args += ["-fobjc-arc", "-w"]
    if modules and MODCACHE:
        # `@import Module;` needs modules; the cache lives in the run's scratch dir.
        args += ["-fmodules", "-fimplicit-module-maps", "-fmodules-cache-path=" + MODCACHE]
    args += lang_args
    args += ["-I" + os.path.join(root, d) for d in includes]
    args += ["-I" + os.path.dirname(path)]
    return index.parse(path, args=args, options=ci.TranslationUnit.PARSE_DETAILED_PROCESSING_RECORD)


def errors_of(tu, root):
    errs = []
    for d in tu.diagnostics:
        if d.severity >= ci.Diagnostic.Error:
            loc = d.location
            errs.append((loc.file.name if loc.file else "", loc.line, d.spelling))
    return errs


_SRC = {}


def source_of(name):
    if name not in _SRC:
        try:
            with open(name, "rb") as fh:
                _SRC[name] = fh.read()
        except OSError:
            _SRC[name] = b""
    return _SRC[name]


def written_at(c, want):
    """True when the text at the cursor's extent start is `want` (a name or '[') as written in the file, not a
    macro's expansion: an expansion's cursors sit at the macro invocation, whose first token is the macro's."""
    start = c.extent.start
    if start.file is None:
        return False
    src = source_of(start.file.name)
    off = start.offset
    if want == "[":
        return src[off:off + 1] == b"["
    return src[off:off + len(want)] == want.encode()


def call_shaped(c):
    """A macro invocation written like a call (`NSParameterAssert(x)`, `MIN(a, b)`)."""
    start = c.extent.start
    if start.file is None:
        return False
    src = source_of(start.file.name)
    end = start.offset + len(c.spelling)
    while end < len(src) and src[end:end + 1] in b" \t":
        end += 1
    return src[start.offset:start.offset + len(c.spelling)] == c.spelling.encode() and src[end:end + 1] == b"("


def rel(root, name):
    try:
        r = os.path.relpath(os.path.realpath(name), os.path.realpath(root))
    except ValueError:
        return None
    return None if r.startswith("..") else r


def main():
    sys.setrecursionlimit(20000)
    ap = argparse.ArgumentParser()
    ap.add_argument("--subject", required=True)
    ap.add_argument("--out", required=True)
    ap.add_argument("--include", action="append", default=[])
    ap.add_argument("--all-header-dirs", action="store_true",
                    help="add -I for the subject root and every directory that holds a header (an Xcode project's recursive user header search paths)")
    ap.add_argument("--target", default="auto", choices=["auto", "mac", "catalyst"])
    ap.add_argument("--only", action="append", default=[], help="subject-relative path prefix to parse (default all)")
    ap.add_argument("--list", help="write the measured file list here")
    ap.add_argument("--compdb", help="write a compile_commands.json here (the conventional place is <subject>/build/): one entry per "
                    "source, the -I roots this run gave the compiler, so fathom's include-path resolver searches the same directories")
    a = ap.parse_args()
    root = os.path.realpath(a.subject)
    index = ci.Index.create()

    files = candidates(root)
    if a.all_header_dirs:
        dirs = {"."} | {os.path.dirname(f) or "." for f in files if f.endswith(".h")}
        a.include = sorted(set(a.include) | dirs)
    if a.only:
        files = [f for f in files if any(f.startswith(o) for o in a.only)]

    if a.compdb:
        os.makedirs(os.path.dirname(a.compdb), exist_ok=True)
        entries = []
        for f in files:
            if os.path.splitext(f)[1] in (".m", ".mm"):
                lang = "objective-c++" if f.endswith(".mm") else "objective-c"
                entries.append({"directory": root, "file": f, "arguments": ["clang", "-x", lang, "-fobjc-arc"] + ["-I" + os.path.join(root, d) for d in a.include] + ["-c", f]})
        with open(a.compdb, "w") as fh:
            json.dump(entries, fh, indent=1)
            fh.write("\n")

    defs = {}  # (path, name, kind, line) -> end line
    calls = set()
    sends = collections.defaultdict(list)  # (path, selector) -> [receiver type spellings]
    imports = {}
    xrefs = set()  # (file, file it binds a reference in): the compiler's cross-file reference edges
    usr_refs = set()  # (file, USR of a function it calls that is only declared in the unit)
    definers = collections.defaultdict(set)  # USR -> subject files that define the function
    trusted_files = set()
    errored = {}
    config_of = {}
    units = 0

    def walk(tu, only_files):
        """Collect definitions, calls and imports of the subject files in tu."""
        out_calls = []
        out_sends = []
        out_defs = []
        out_imps = []
        out_refs = []
        out_usr = []
        out_definers = []
        extents = []  # (file, start offset, end offset, name) of every callable definition with a body
        macros = []   # (file, offset, name) of every call-shaped macro invocation

        def visit(c, defn):
            f = c.location.file
            here = None
            if f is not None:
                here = rel(root, f.name)
            k = c.kind
            if k == CK.INCLUSION_DIRECTIVE and here in only_files:
                inc = c.get_included_file()
                if inc is not None:
                    to = rel(root, inc.name)
                    if to is not None:
                        out_imps.append({"path": here, "line": c.location.line, "to_path": to, "module": c.spelling})
            if here is not None and here in only_files and k in REFS:
                try:
                    target = c.referenced
                except Exception:
                    target = None
                if target is not None and target.location.file is not None:
                    there = rel(root, target.location.file.name)
                    if there is not None and there != here:
                        out_refs.append((here, there))
                    if target.kind == CK.FUNCTION_DECL and not target.is_definition():
                        # the file whose definition the call links to is resolved once every unit is read
                        out_usr.append((here, target.get_usr()))
            if here is not None and here in only_files:
                kind = None
                name = c.spelling
                if k == CK.OBJC_INTERFACE_DECL and c.is_definition():
                    kind = "interface"
                elif k == CK.OBJC_IMPLEMENTATION_DECL:
                    kind = "implementation"
                elif k == CK.OBJC_PROTOCOL_DECL and c.is_definition():
                    kind = "protocol"
                elif k == CK.OBJC_CATEGORY_DECL:
                    kind = "category"
                    name = c.spelling
                elif k in METHODS:
                    kind = "method" if k == CK.OBJC_INSTANCE_METHOD_DECL else "class_method"
                elif k == CK.OBJC_PROPERTY_DECL:
                    kind = "property"
                elif k == CK.FUNCTION_DECL and c.is_definition():
                    kind = "function"
                elif k in (CK.STRUCT_DECL, CK.UNION_DECL) and c.is_definition() and name:
                    kind = "struct" if k == CK.STRUCT_DECL else "union"
                elif k == CK.ENUM_DECL and c.is_definition() and name:
                    kind = "enum"
                elif k == CK.TYPEDEF_DECL:
                    kind = "typedef"
                elif k == CK.MACRO_DEFINITION:
                    kind = "macro"
                if kind and name:
                    out_defs.append((here, name, kind, c.location.line, c.extent.end.line))
                    if kind == "function":
                        out_definers.append((c.get_usr(), here))
                # the enclosing definition of what is inside
                if k in CALLABLE_DEFS and any(ch.kind == CK.COMPOUND_STMT for ch in c.get_children()):
                    defn = c.spelling
                elif k == CK.FUNCTION_DECL and c.is_definition():
                    defn = c.spelling
                if defn is not None and k in CALLABLE_DEFS | {CK.FUNCTION_DECL}:
                    extents.append((here, c.extent.start.offset, c.extent.end.offset, defn))
                if defn is not None:
                    if k in MSG:
                        if written_at(c, "["):
                            ch = list(c.get_children())
                            recv = ch[0].type.spelling if ch else ""
                            out_calls.append((here, defn, c.spelling))
                            out_sends.append((here, c.spelling, recv))
                    elif k == CK.CALL_EXPR and c.spelling:
                        if written_at(c, c.spelling):
                            out_calls.append((here, defn, c.spelling))
            for ch in c.get_children():
                visit(ch, defn)

        for top in tu.cursor.get_children():
            f = top.location.file
            if f is not None and rel(root, f.name) in only_files:
                if top.kind == CK.MACRO_INSTANTIATION:
                    here_ = rel(root, f.name)
                    try:
                        mdef = top.referenced
                    except Exception:
                        mdef = None
                    if mdef is not None and mdef.location.file is not None:
                        there = rel(root, mdef.location.file.name)
                        if there is not None and there != here_:
                            out_refs.append((here_, there))
                    if top.spelling and call_shaped(top):
                        macros.append((here_, top.extent.start.offset, top.spelling))
                else:
                    visit(top, None)
        for (mf, off, name) in macros:
            inner = None
            for (ef, lo, hi, dn) in extents:
                if ef == mf and lo <= off < hi and (inner is None or hi - lo < inner[1]):
                    inner = (dn, hi - lo)
            if inner is not None:
                out_calls.append((mf, inner[0], name))
        for usr, f_ in out_definers:
            definers[usr].add(f_)
        return out_calls, out_sends, out_defs, out_imps, out_refs, out_usr

    for rel_path in files:
        path = os.path.join(root, rel_path)
        ext = os.path.splitext(rel_path)[1]
        try:
            src = open(path, encoding="utf-8", errors="replace").read()
        except OSError:
            continue
        if ext == ".h" and not looks_objc(src):
            continue
        if ext == ".m" and not looks_objc(src[:16384]):
            continue
        lang_args = ["-x", "objective-c++" if ext == ".mm" else ("objective-c-header" if ext == ".h" else "objective-c")]
        tried = [a.target] if a.target != "auto" else ["mac", "catalyst"]
        units += 1
        first_errs = None
        for cfg in tried:
            for modules in (False, True):
                tu = parse(index, path, root, lang_args, a.include, cfg, modules)
                errs = errors_of(tu, root)
                # @import needs modules; anything else does not, and a unit that stopped on a missing header
                # is not helped by them.
                if not errs or not any("modules are disabled" in e[2] for e in errs):
                    break
            if errs:
                if first_errs is None:
                    first_errs = errs
                continue
            # Every configuration that parses clean adds its own: a file under `#if TARGET_OS_IOS` is empty
            # under macOS and whole under Mac Catalyst, and fathom reads both branches.
            config_of.setdefault(rel_path, []).append(cfg)
            c, s_, d, i, r, u = walk(tu, {rel_path})
            trusted_files.add(rel_path)
            calls.update(c)
            for p_, sel, recv in s_:
                sends[(p_, sel)].append(recv)
            for t in d:
                defs[t[:4]] = t[4]
            for e in i:
                imports[(e["path"], e["line"], e["to_path"])] = e
            xrefs.update(r)
            usr_refs.update(u)
        if rel_path not in trusted_files and first_errs:
            errored[rel_path] = [{"file": rel(root, e[0]) or e[0], "line": e[1], "msg": e[2]} for e in first_errs[:3]]

    # A call to a function declared in a header links to the file that defines it: a SCIP index puts the
    # reference there, so the compiler's cross-file reference edges do too, beside the declaring header's.
    for (f_, usr) in usr_refs:
        for g_ in definers.get(usr, ()):
            if g_ != f_:
                xrefs.add((f_, g_))

    # The calls of a file are trusted only when its own unit parsed without error.
    direct = {(k[0], k[2]) for k in imports}
    pairs = sorted({(p, f, t) for (p, f, t) in calls if p in trusted_files})
    excl = []
    for (p, sel), recvs in sorted(sends.items()):
        if p in trusted_files and recvs and all(r.strip() == "id" for r in recvs):
            excl.append({"path": p, "to": sel, "reason": "every send of this selector in the file has a receiver of type id"})
    ver = subprocess.run(["xcrun", "clang", "--version"], capture_output=True, text=True).stdout.splitlines()[0]
    doc = {
        "tool": "clang-objc",
        "version": "libclang (Command Line Tools) via libclang==18.1.1 bindings; " + ver,
        "tier": "B",
        "calls": [{"path": p, "from": f, "to": t} for (p, f, t) in pairs],
        "call_exclusions": excl,
        "imports": [imports[k] for k in sorted(imports) if k[0] in trusted_files] + [
            # file-edge truth with no import statement of its own: a reference the compiler binds to a file the
            # importer reaches through an import chain (the harness's field for it is same_package)
            {"path": f, "line": 0, "to_path": g, "same_package": True}
            for (f, g) in sorted(xrefs)
            if f in trusted_files and g in trusted_files and (f, g) not in direct],
        "disclose": [
            "%d translation units parsed, %d without a compile error (%s)" % (
                units, len(trusted_files), ", ".join("%s %d" % kv for kv in sorted(collections.Counter(c for cs in config_of.values() for c in cs).items()))),
            "%d units stopped on an error and are left out of calls and imports" % len(errored),
        ],
    }
    with open(a.out, "w") as f:
        json.dump(doc, f, indent=1, sort_keys=True)
        f.write("\n")
    ranges = {
        "tool": "clang-objc",
        "tier": "B",
        "version": doc["version"],
        "units": units,
        "errored": len(errored),
        "kinds": {"interface": "class", "implementation": "class", "category": "class", "protocol": "interface",
                  "method": "method", "class_method": "method", "property": "property", "function": "function",
                  "struct": "struct", "union": "union", "enum": "enum", "typedef": "type_alias", "macro": "macro"},
        "scope": "the first and last line of every definition the compiler names in a file whose unit parsed without error "
                 "(an @interface or @implementation, a protocol, a category, a method, a function with a body, a struct, union or enum "
                 "with a body, a typedef, a macro, a property)",
        "ranges": [{"path": k[0], "name": k[1], "kind": k[2], "line": k[3], "end": v}
                   for k, v in sorted(defs.items()) if k[0] in trusted_files],
    }
    with open(os.path.splitext(a.out)[0] + ".ranges.json", "w") as f:
        json.dump(ranges, f, indent=1, sort_keys=True)
        f.write("\n")
    with open(os.path.splitext(a.out)[0] + ".errored.json", "w") as f:
        json.dump(errored, f, indent=1, sort_keys=True)
        f.write("\n")
    if a.list:
        with open(a.list, "w") as f:
            f.write("\n".join(sorted(trusted_files)) + "\n")
    print("units %d trusted %d errored %d calls %d imports %d defs %d exclusions %d" % (
        units, len(trusted_files), len(errored), len(pairs), len(imports), len(defs), len(excl)), file=sys.stderr)


if __name__ == "__main__":
    main()
