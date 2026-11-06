"""cfn_reads_oracle.py - the CloudFormation oracle for LL-M4: cfn-lint's own decode of each selected template.

Run it with cfn-lint's interpreter, so the library is the installed tool's (1.52.1):

    <cfn-lint libexec>/bin/python cfn_reads_oracle.py --subjects subjects/cloudformation.json --clones <dir> --out <file>

`cfnlint.decode.decode(path)` reads a template the way cfn-lint does: its YAML loader turns each short form (`!Ref X`) into the
long form, a mapping with one key. Nothing here imports the readers. For each resource with a Type it walks Properties and writes
one leaf per attribute path, with the oracle class (README.md):

  constant    a scalar, or an empty mapping or list (a leaf at its own path); Fn::Sub with nothing left to substitute
  references  an intrinsic function whose argument names a Ref, a GetAtt, an ImportValue or a Sub variable
  empty       an intrinsic function that names none

Paths: a mapping key joins with '.', a list element takes '[*]'. `defaultable` is true when every name the leaf references is a
parameter with a scalar Default whose Type is not an SSM parameter type (such a default is a path, not the value).
"""
import argparse, json, os, re, sys

import cfnlint.decode

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from select_subjects import clone_dir  # noqa: E402  (path helper only)

SUB_VAR = re.compile(r"\$\{([^}]*)\}")


def is_intrinsic(n):
    if isinstance(n, dict) and len(n) == 1:
        k = next(iter(n))
        return k == "Ref" or str(k).startswith("Fn::")
    return False


def sub_parts(arg):
    """(string, map) of a Fn::Sub argument, or None when it has no readable shape."""
    if isinstance(arg, str):
        return str(arg), {}
    if isinstance(arg, list) and len(arg) == 2 and isinstance(arg[0], str) and isinstance(arg[1], dict):
        return str(arg[0]), arg[1]
    return None


def collect(n, refs, consts):
    """refs: names referenced anywhere under n. consts: False when a Fn::Sub still has something to substitute."""
    if isinstance(n, dict):
        if is_intrinsic(n):
            k, v = next(iter(n.items()))
            k = str(k)
            if k == "Ref":
                if isinstance(v, str):
                    refs.add(str(v))
                return
            if k == "Fn::GetAtt":
                if isinstance(v, str):
                    refs.add(str(v))
                elif isinstance(v, list) and all(isinstance(x, str) for x in v) and len(v) == 2:
                    refs.add(".".join(str(x) for x in v))
                else:
                    collect(v, refs, consts)
                return
            if k == "Fn::ImportValue":
                refs.add("export:")
                collect(v, refs, consts)
                return
            if k == "Fn::Sub":
                sp = sub_parts(v)
                if sp is None:
                    collect(v, refs, consts)
                    return
                s, m = sp
                for name in SUB_VAR.findall(s):
                    if name.startswith("!"):
                        continue
                    if name in m:
                        continue
                    refs.add(name)
                for mv in m.values():
                    collect(mv, refs, consts)
                return
            if k == "Fn::If":
                if isinstance(v, list) and v:
                    for x in v[1:]:
                        collect(x, refs, consts)
                    return
        for v in n.values():
            collect(v, refs, consts)
    elif isinstance(n, list):
        for v in n:
            collect(v, refs, consts)


def sub_is_constant(n):
    """Fn::Sub with no free variable and map values that are all constant."""
    sp = sub_parts(n["Fn::Sub"])
    if sp is None:
        return False
    s, m = sp
    for name in SUB_VAR.findall(s):
        if name.startswith("!"):
            continue
        if name not in m:
            return False
        if not node_constant(m[name]):
            return False
    return True


def node_constant(n):
    if isinstance(n, dict):
        if is_intrinsic(n):
            return "Fn::Sub" in n and sub_is_constant(n)
        return all(node_constant(v) for v in n.values())
    if isinstance(n, list):
        return all(node_constant(v) for v in n)
    return True


def classify(n, defaults):
    """Oracle class and defaultable of an intrinsic (or any) node."""
    if node_constant(n) and not is_intrinsic(n):
        return "constant", False
    if is_intrinsic(n) and "Fn::Sub" in n and sub_is_constant(n):
        return "constant", False
    refs, consts = set(), True
    collect(n, refs, consts)
    if refs:
        return "references", all(r in defaults for r in refs)
    return "empty", False


def walk(n, path, defaults, out):
    if is_intrinsic(n):
        c, d = classify(n, defaults)
        out.setdefault(path, []).append({"class": c, "defaultable": d})
    elif isinstance(n, dict):
        if not n:
            out.setdefault(path, []).append({"class": "constant", "defaultable": False})
        for k, v in n.items():
            walk(v, (path + "." if path else "") + str(k), defaults, out)
    elif isinstance(n, list):
        if not n:
            out.setdefault(path, []).append({"class": "constant", "defaultable": False})
        for v in n:
            walk(v, path + "[*]", defaults, out)
    else:
        out.setdefault(path, []).append({"class": "constant", "defaultable": False})


def read_template(tpl):
    defaults = set()
    params = tpl.get("Parameters")
    if isinstance(params, dict):
        for name, p in params.items():
            if not isinstance(p, dict) or "Default" not in p:
                continue
            if isinstance(p["Default"], (dict, list)):
                continue
            if str(p.get("Type", "")).startswith("AWS::SSM::Parameter::Value<"):
                continue
            defaults.add(str(name))
    res = {}
    for name, r in tpl["Resources"].items():
        if not isinstance(r, dict) or not isinstance(r.get("Type"), str):
            continue
        leaves = {}
        props = r.get("Properties")
        if props is None:
            pass
        elif isinstance(props, dict) and not is_intrinsic(props):
            walk(props, "", defaults, leaves)
        else:
            c, d = classify(props, defaults)
            leaves.setdefault("", []).append({"class": c, "defaultable": d})
        res[str(name)] = {"type": str(r["Type"]), "leaves": leaves}
    return res


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--subjects", required=True)
    ap.add_argument("--clones", required=True)
    ap.add_argument("--out", required=True)
    a = ap.parse_args()
    sf = json.load(open(a.subjects))
    out = {}
    for s in sf["selected"]:
        key = s["repo"] + ":" + s["path"]
        path = os.path.join(clone_dir(a.clones, sf["tool"], s["repo"]), s["path"])
        try:
            tpl, matches = cfnlint.decode.decode(path)
        except Exception as e:  # an oracle failure is a rejection, never a guess
            out[key] = {"ok": False, "error": repr(e)[:200]}
            continue
        if matches or tpl is None:
            msg = str(getattr(matches[0], "message", matches[0])).split("\n")[0][:200] if matches else "no document"
            out[key] = {"ok": False, "error": msg}
            continue
        if not isinstance(tpl, dict) or not isinstance(tpl.get("Resources"), dict):
            out[key] = {"ok": False, "error": "not a template: no Resources mapping"}
            continue
        out[key] = {"ok": True, "resources": read_template(tpl)}
    json.dump({"oracle": "cfn-lint " + cfnlint.version.__version__ + " cfnlint.decode.decode", "templates": out}, open(a.out, "w"), indent=1, sort_keys=True)


if __name__ == "__main__":
    import cfnlint.version
    main()
