"""walk.py: the language-breadth harness's tier-C oracle for the simulink native dialect
(LB-Q-simulink; design section 1.3). It shares no code with fathom's reader
(internal/extractor/simulink.go): a .slx is opened with Python's zipfile, each part is read
with xml.parsers.expat (start and end lines from the parser's own line counter; xml.etree keeps
none), and a classic .mdl is read by a brace-line reader of its own.

    python3 -I walk.py -subject <checkout> -out <json>

What it reports, in the neutral oracle shape (calibration/oracles/README.md):

  - definitions: one `model` per model file (named by its base name, line 1), then each block of
    the diagram's System in depth-first order, named by its full path from the model (a `/` in a
    block name doubled, Simulink's escape; a line break read as one space), kind `subsystem`
    when the block owns a System, else `block`; lines are the block element's (or brace node's)
    in the part that holds it.
  - .slx layouts: blocks inline in simulink/blockdiagram.xml (before R2020a); from R2020a one
    simulink/systems/system_<n>.xml per system, a SubSystem's `<System Ref="..."/>` naming the
    part read in its place (system_root first, then each part no part names, then any part
    still unread; each part read once).
  - references: a signal or connection line from its source block to each destination block
    (its own Dst, then each Branch's, depth first), an endpoint written as a SID (`7#out:1`)
    or as a block name in the line's system (a .mdl's SrcBlock and DstBlock); an endpoint that
    names no block is dropped. The L2 family.
  - cross_references: each ModelReference block (its ModelNameDialog, else ModelName, an .slx
    or .mdl extension dropped) to the one model file of that name elsewhere in the subject; a
    name several model files share joins none (MATLAB's path order is not modelled), and one
    no model file holds joins none. The L3 family. Read from the blocks, not from the
    package's graphicalInterface part fathom's reader uses, so the two sources cross-check.
  - BlockDefaults and BlockParameterDefaults are template blocks, never read. A text-package
    .mdl (`# MathWorks OPC Text Package`, `# Simulink Text Package`) and a file that does not
    open are skipped and counted on stderr.

Tier C: an independent parser of a format whose own tool (MATLAB's find_system) is not run
here (owner ruling 2026-10-24 (d)); it caps every metric at good. Bench-only.
"""

import argparse
import json
import os
import sys
import xml.parsers.expat
import zipfile

TOOL = "walk.py: zipfile and xml.parsers.expat over .slx parts, a brace-line reader over .mdl"
VERSION = "1"
SKIP_DIRS = {".git"}


def flat(name):
    return name.replace("\r\n", " ").replace("\r", " ").replace("\n", " ")


def join(parent, name):
    return parent + "/" + name.replace("/", "//")


class Node:
    __slots__ = ("tag", "attrs", "kids", "line", "end", "text")

    def __init__(self, tag, attrs, line):
        self.tag, self.attrs, self.kids, self.line, self.end, self.text = tag, attrs, [], line, line, ""


def parse_part(data):
    root = Node("#doc", {}, 0)
    stack = [root]
    p = xml.parsers.expat.ParserCreate()

    def start(tag, attrs):
        n = Node(tag, attrs, p.CurrentLineNumber)
        stack[-1].kids.append(n)
        stack.append(n)

    def end(tag):
        stack[-1].end = p.CurrentLineNumber
        stack.pop()

    def text(data):
        if stack[-1].tag == "P":
            stack[-1].text += data

    p.StartElementHandler = start
    p.EndElementHandler = end
    p.CharacterDataHandler = text
    p.Parse(data, True)
    return root


def kids(n, tag):
    return [k for k in n.kids if k.tag == tag]


def model_ref(block_type, name):
    """The model a ModelReference block names, its .slx or .mdl extension dropped."""
    if block_type != "ModelReference" or not name:
        return ""
    name = name.strip()
    for ext in (".slx", ".mdl"):
        if name.endswith(ext):
            name = name[: -len(ext)]
    return name


def prop(n, name):
    for k in n.kids:
        if k.tag == "P" and k.attrs.get("Name") == name:
            return k.text
    return ""


def xml_dsts(n, out):
    for k in n.kids:
        if k.tag == "P" and k.attrs.get("Name") == "Dst":
            out.append(("sid", k.text.split("#", 1)[0].strip()))
        elif k.tag == "P" and k.attrs.get("Name") == "DstBlock":
            out.append(("name", k.text))
        elif k.tag == "Branch":
            xml_dsts(k, out)
    return out


def slx_blocks(path, model, lines):
    z = zipfile.ZipFile(path)
    names = set(z.namelist())
    out = []
    systems = {}
    for nm in sorted(names):
        if nm.startswith("simulink/systems/") and nm.endswith(".xml") and "/" not in nm[len("simulink/systems/"):]:
            doc = parse_part(z.read(nm))
            for s in kids(doc, "System"):
                systems[nm[len("simulink/systems/"):-4]] = s

    def walk(system, parent, descend):
        for b in kids(system, "Block"):
            name = flat(b.attrs.get("Name", ""))
            full = join(parent, name)
            inner = kids(b, "System")
            out.append({"name": full, "kind": "subsystem" if inner else "block", "line": b.line, "end": b.end,
                        "sid": b.attrs.get("SID", ""),
                        "mref": model_ref(b.attrs.get("BlockType", ""), prop(b, "ModelNameDialog") or prop(b, "ModelName"))})
            for s in inner:
                if "Ref" in s.attrs:
                    descend(s.attrs["Ref"], full)
                else:
                    walk(s, full, descend)
        for ln in kids(system, "Line"):
            src = ("sid", prop(ln, "Src").split("#", 1)[0].strip())
            if prop(ln, "SrcBlock"):
                src = ("name", prop(ln, "SrcBlock"))
            lines.append((parent, src, xml_dsts(ln, [])))

    if systems:
        seen = set()

        def descend(ref, parent):
            if ref in seen or ref not in systems:
                return
            seen.add(ref)
            walk(systems[ref], parent, descend)

        named = set()

        def refs(n):
            for k in n.kids:
                if k.tag == "System" and "Ref" in k.attrs:
                    named.add(k.attrs["Ref"])
                refs(k)

        for s in systems.values():
            refs(s)
        order = (["system_root"] if "system_root" in systems else []) + \
            [r for r in sorted(systems) if r != "system_root" and r not in named] + sorted(systems)
        for r in order:
            descend(r, model)
    elif "simulink/blockdiagram.xml" in names:
        doc = parse_part(z.read("simulink/blockdiagram.xml"))
        diagram = None
        for info in kids(doc, "ModelInformation"):
            for k in info.kids:
                if k.tag in ("Model", "Library") and diagram is None:
                    diagram = k
        if diagram is not None:
            for s in kids(diagram, "System"):
                walk(s, model, lambda r, p: None)
    return out


def unquote_all(s):
    """The concatenation of the quoted strings opening s ("a" "b")."""
    out = []
    i = 0
    while i < len(s) and s[i] == '"':
        i += 1
        while i < len(s) and s[i] != '"':
            c = s[i]
            if c == "\\" and i + 1 < len(s):
                i += 1
                out.append({"n": "\n", "t": "\t", "r": "\r"}.get(s[i], s[i]))
            else:
                out.append(c)
            i += 1
        i += 1
        while i < len(s) and s[i] in " \t":
            i += 1
    return "".join(out)


def mdl_dsts(n, out):
    if n["props"].get("Dst"):
        out.append(("sid", n["props"]["Dst"].split("#", 1)[0].strip()))
    elif n["props"].get("DstBlock"):
        out.append(("name", n["props"]["DstBlock"]))
    for b in [k for k in n["kids"] if k["tag"] == "Branch"]:
        mdl_dsts(b, out)
    return out


def mdl_blocks(text, model, lines):
    head = text[:4096]
    if "# MathWorks OPC Text Package" in head or "# Simulink Text Package" in head or \
            "__MWOPC_PACKAGE_BEGIN__" in text or "__MWSL_PART_BEGIN__" in text:
        return None
    root = {"tag": "#doc", "props": {}, "kids": [], "line": 0, "end": 0}
    stack = [root]
    last = None
    for no, raw in enumerate(text.split("\n"), 1):
        t = raw.strip()
        top = stack[-1]
        if not t or t.startswith("#"):
            last = None
        elif t == "}":
            if len(stack) > 1:
                top["end"] = no
                stack.pop()
            last = None
        elif t.endswith("{") and t[:-1].strip() and not any(c in t[:-1].strip() for c in ' \t"['):
            n = {"tag": t[:-1].strip(), "props": {}, "kids": [], "line": no, "end": no}
            top["kids"].append(n)
            stack.append(n)
            last = None
        elif t.startswith('"'):
            if last is not None and last[0] is top:
                top["props"][last[1]] += unquote_all(t)
        else:
            parts = t.split(None, 1)
            key, val = parts[0], (parts[1].strip() if len(parts) > 1 else "")
            if val.startswith('"'):
                val = unquote_all(val)
            if key not in top["props"]:
                top["props"][key] = val
                last = (top, key)
            else:
                last = None
    out = []
    diagram = next((k for k in root["kids"] if k["tag"] in ("Model", "Library")), None)
    if diagram is None:
        return out

    def walk(system, parent):
        for b in [k for k in system["kids"] if k["tag"] == "Block"]:
            full = join(parent, flat(b["props"].get("Name", "")))
            inner = [k for k in b["kids"] if k["tag"] == "System"]
            out.append({"name": full, "kind": "subsystem" if inner else "block", "line": b["line"], "end": b["end"],
                        "sid": b["props"].get("SID", ""),
                        "mref": model_ref(b["props"].get("BlockType", ""), b["props"].get("ModelNameDialog") or b["props"].get("ModelName", ""))})
            for s in inner:
                walk(s, full)
        for ln in [k for k in system["kids"] if k["tag"] == "Line"]:
            src = ("sid", ln["props"].get("Src", "").split("#", 1)[0].strip())
            if ln["props"].get("SrcBlock"):
                src = ("name", ln["props"]["SrcBlock"])
            lines.append((parent, src, mdl_dsts(ln, [])))

    for s in [k for k in diagram["kids"] if k["tag"] == "System"]:
        walk(s, model)
    return out


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("-subject", required=True)
    ap.add_argument("-out", required=True)
    a = ap.parse_args()
    defs, refs, skipped = [], [], 0
    mrefs = []  # (path, block, model name)
    for dirpath, dirnames, filenames in os.walk(a.subject):
        dirnames[:] = sorted(d for d in dirnames if d not in SKIP_DIRS)
        for fn in sorted(filenames):
            ext = os.path.splitext(fn)[1]
            if ext not in (".slx", ".mdl"):
                continue
            full = os.path.join(dirpath, fn)
            rel = os.path.relpath(full, a.subject).replace(os.sep, "/")
            model = os.path.splitext(fn)[0]
            try:
                with open(full, "rb") as fh:
                    data = fh.read()
                lines = []
                if data.startswith(b"PK\x03\x04"):
                    blocks = slx_blocks(full, model, lines)
                else:
                    blocks = mdl_blocks(data.decode("utf-8", "replace"), model, lines)
            except Exception as e:  # a package that does not open, a part that does not parse
                print("walk.py: skipped %s: %s" % (rel, e), file=sys.stderr)
                skipped += 1
                continue
            if blocks is None:
                print("walk.py: skipped %s: text-package .mdl" % rel, file=sys.stderr)
                skipped += 1
                continue
            defs.append({"path": rel, "name": model, "line": 1, "end": 1, "kind": "model"})
            by_sid, paths = {}, set()
            for b in blocks:
                sid = b.pop("sid")
                mref = b.pop("mref")
                if mref:
                    mrefs.append((rel, b["name"], mref))
                if sid and sid not in by_sid:
                    by_sid[sid] = b["name"]
                paths.add(b["name"])
                b["path"] = rel
                defs.append(b)

            def end_of(system, e):
                if e[0] == "sid":
                    return by_sid.get(e[1], "") if e[1] else ""
                p = join(system, flat(e[1]))
                return p if e[1] and p in paths else ""

            seen = set()
            for system, src, dsts in lines:
                s = end_of(system, src)
                if not s:
                    continue
                for d in dsts:
                    t = end_of(system, d)
                    if t and (s, t) not in seen:
                        seen.add((s, t))
                        refs.append({"path": rel, "from": s, "to": t})
    # A model reference names a model by name; it is joined to the one model file of that name
    # elsewhere in the subject, and to none when several share it (MATLAB's path order decides
    # there, which this oracle does not model; counted on stderr).
    files_by_model = {}
    for d in defs:
        if d["kind"] == "model":
            files_by_model.setdefault(d["name"], []).append(d["path"])
    xrefs, ambiguous, missing = [], 0, 0
    for rel, block, name in mrefs:
        cands = [f for f in files_by_model.get(name, []) if f != rel]
        if len(cands) == 1:
            xrefs.append({"path": rel, "from": block, "to_path": cands[0], "to": name})
        elif cands:
            ambiguous += 1
        else:
            missing += 1
    print("walk.py: %d model references joined, %d ambiguous, %d to no model in the subject" % (len(xrefs), ambiguous, missing), file=sys.stderr)
    out = {"tool": TOOL, "version": VERSION, "tier": "C", "definitions": defs, "references": refs, "cross_references": xrefs}
    with open(a.out, "w") as fh:
        json.dump(out, fh, ensure_ascii=False)
    print("walk.py: %d definitions, %d references, %d file(s) skipped" % (len(defs), len(refs), skipped), file=sys.stderr)


if __name__ == "__main__":
    main()
