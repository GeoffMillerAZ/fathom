"""azure_policy_oracle.py - the language-breadth harness's oracle for the
azure-policy doc-engine dialect (design section 1.3, 13.4 step 19;
LB-Q-azure-policy). Python standard library only.

    python azure_policy_oracle.py -subject <checkout> -out <file> [-files-out <file>] [-summary-out <file>]

Tier C: no Azure tool evaluates a policy definition offline. The oracle is a
JSON walk written apart from the dialect: its own JSON reader (a small
recursive-descent parser that records the line each value starts on and each
container ends on; it shares nothing with fathom's decoder), then the
structure the Azure Policy definition schema names, read from the decoded
tree:

  - a policy definition is `properties` holding a `policyRule` object (the ARM
    and Azure Resource Graph shape), a root `policyRule` object, or a root
    `if`/`then` object pair (the `azurepolicy.rules.json` shape) whose `then`
    names an `effect`; an initiative is `properties.policyDefinitions` holding
    at least one object. Any other JSON file is not a policy.
  - definitions: a `policy` per definition or initiative (named by the file, at
    the `properties` object, else the root), a `variable_decl` per parameter
    (`properties.parameters`, else the root's `parameters`, at the entry's
    line), a `rule` per policyRule `then` (named by its effect, as written) and
    a `key` per initiative member (`policyDefinitionReferenceId`, else
    `policyDefinitionId`).
  - references: from each definition and initiative, every `[... parameters('x')
    ...]` written in any string under it (the `properties` object, else the
    root) that names a parameter the file declares.

A parameter named in an expression the file does not declare (the parameters
live in a sibling azurepolicy.parameters.json) is not a reference: it names no
entity of the file. Directories the connector never reads (testdata, vendor,
node_modules, generated, .git and the editor directories) are not walked.
Bench-only; never a product dependency.
"""

import argparse
import json
import os
import posixpath
import re
import sys

SKIP_DIRS = {"vendor", ".git", "testdata", "generated", "node_modules", ".idea", ".vscode", ".claude"}
PARAM_RE = re.compile(r"parameters\('([^']+)'\)")
WS = " \t\r\n"


class Node:
    __slots__ = ("kind", "value", "line", "end")

    def __init__(self, kind, value, line, end):
        self.kind, self.value, self.line, self.end = kind, value, line, end

    def get(self, key):
        if self.kind == "object":
            for k, v in self.value:
                if k == key:
                    return v
        return None


class Reader:
    def __init__(self, text):
        self.s, self.i, self.line = text, 0, 1

    def skip(self):
        while self.i < len(self.s) and self.s[self.i] in WS:
            if self.s[self.i] == "\n":
                self.line += 1
            self.i += 1

    def string(self):
        j, s = self.i + 1, self.s
        while s[j] != '"':
            j += 2 if s[j] == "\\" else 1
        tok = s[self.i:j + 1]
        self.i = j + 1
        return json.loads(tok)

    def value(self, line_override=None):
        self.skip()
        line = self.line
        c = self.s[self.i]
        if c == "{":
            self.i += 1
            members = []
            self.skip()
            if self.s[self.i] == "}":
                self.i += 1
                return Node("object", members, line, self.line)
            while True:
                self.skip()
                kline = self.line
                key = self.string()
                self.skip()
                if self.s[self.i] != ":":
                    raise ValueError("expected :")
                self.i += 1
                v = self.value()
                v.line = kline  # a member starts at its key (the line the reading names)
                members.append((key, v))
                self.skip()
                if self.s[self.i] == ",":
                    self.i += 1
                    continue
                if self.s[self.i] == "}":
                    self.i += 1
                    return Node("object", members, line, self.line)
                raise ValueError("expected , or }")
        if c == "[":
            self.i += 1
            items = []
            self.skip()
            if self.s[self.i] == "]":
                self.i += 1
                return Node("array", items, line, self.line)
            while True:
                items.append(self.value())
                self.skip()
                if self.s[self.i] == ",":
                    self.i += 1
                    continue
                if self.s[self.i] == "]":
                    self.i += 1
                    return Node("array", items, line, self.line)
                raise ValueError("expected , or ]")
        if c == '"':
            return Node("string", self.string(), line, line)
        m = re.compile(r"-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?|true|false|null").match(self.s, self.i)
        if not m:
            raise ValueError("bad value")
        self.i = m.end()
        tok = m.group(0)
        return Node("null" if tok == "null" else "scalar", tok, line, line)

    def document(self):
        v = self.value()
        self.skip()
        if self.i != len(self.s):
            raise ValueError("trailing data")
        return v


def strings(node, out):
    if node.kind == "string":
        out.append(node.value)
    elif node.kind == "object":
        for _, v in node.value:
            strings(v, out)
    elif node.kind == "array":
        for v in node.value:
            strings(v, out)
    return out


def find_files(root):
    found = []
    for dirpath, dirnames, filenames in os.walk(root):
        dirnames[:] = sorted(d for d in dirnames if d not in SKIP_DIRS)
        for f in sorted(filenames):
            if f.lower().endswith(".json"):
                found.append(os.path.relpath(os.path.join(dirpath, f), root).replace(os.sep, "/"))
    return sorted(found)


def effect_of(then):
    if then is not None and then.kind == "object":
        e = then.get("effect")
        if e is not None and e.kind == "string" and e.value:
            return e.value
    return None


def read_policy(root):
    """(scope, kind, then, params) when root is a definition or an initiative, else None.
    scope is the node the definition's own line and references are read from."""
    if root.kind != "object":
        return None
    props = root.get("properties")
    if props is not None and props.kind == "object":
        rule = props.get("policyRule")
        if rule is not None and rule.kind == "object" and effect_of(rule.get("then")):
            return props, "definition", rule.get("then"), props.get("parameters")
        members = props.get("policyDefinitions")
        if members is not None and members.kind == "array" and any(m.kind == "object" for m in members.value):
            return props, "initiative", None, props.get("parameters")
    rule = root.get("policyRule")
    if rule is not None and rule.kind == "object" and effect_of(rule.get("then")):
        return root, "definition", rule.get("then"), root.get("parameters")
    if root.get("if") is not None and root.get("then") is not None and effect_of(root.get("then")):
        return root, "definition", root.get("then"), root.get("parameters")
    return None


def main(argv=None):
    ap = argparse.ArgumentParser()
    ap.add_argument("-subject", required=True)
    ap.add_argument("-out", required=True)
    ap.add_argument("-files-out", default="")
    ap.add_argument("-summary-out", default="")
    a = ap.parse_args(argv)
    root_dir = os.path.abspath(a.subject)
    defs, refs, files = [], [], []
    stats = {"files": 0, "policies": 0, "definitions": 0, "initiatives": 0, "not_json": 0, "not_policy": 0,
             "parameters": 0, "members": 0, "references": 0}
    for rel in find_files(root_dir):
        stats["files"] += 1
        try:
            with open(os.path.join(root_dir, rel), encoding="utf-8-sig") as f:
                root = Reader(f.read()).document()
        except Exception:  # pylint: disable=broad-except
            stats["not_json"] += 1
            continue
        pol = read_policy(root)
        if pol is None:
            stats["not_policy"] += 1
            continue
        scope, kind, then, params = pol
        stats["policies"] += 1
        stats["definitions" if kind == "definition" else "initiatives"] += 1
        files.append(rel)
        name = posixpath.basename(rel)
        defs.append({"path": rel, "name": name, "line": scope.line, "end": scope.end, "kind": "policy"})
        declared = set()
        if params is not None and params.kind == "object":
            for k, v in params.value:
                if v.kind == "object":
                    declared.add(k)
                    stats["parameters"] += 1
                    defs.append({"path": rel, "name": k, "line": v.line, "end": v.end, "kind": "variable_decl"})
        if then is not None:
            defs.append({"path": rel, "name": effect_of(then), "line": then.line, "end": then.end, "kind": "rule"})
        if kind == "initiative":
            for m in scope.get("policyDefinitions").value:
                if m.kind != "object":
                    continue
                ref = m.get("policyDefinitionReferenceId")
                ident = m.get("policyDefinitionId")
                nm = ref.value if ref is not None and ref.kind == "string" and ref.value else (
                    ident.value if ident is not None and ident.kind == "string" and ident.value else None)
                if nm:
                    stats["members"] += 1
                    defs.append({"path": rel, "name": nm, "line": m.line, "end": m.end, "kind": "key"})
        used = set()
        for s in strings(scope, []):
            for p in PARAM_RE.findall(s):
                if p in declared:
                    used.add(p)
        for p in sorted(used):
            stats["references"] += 1
            refs.append({"path": rel, "from": name, "to": p})
    with open(a.out, "w") as f:
        json.dump({"tool": "azure-policy JSON walk (the definition schema's structure, an independent JSON reader)",
                   "version": "1", "tier": "C",
                   "definitions": sorted(defs, key=lambda d: (d["path"], d["line"], d["name"])),
                   "references": refs}, f, indent=1)
        f.write("\n")
    if a.files_out:
        with open(a.files_out, "w") as f:
            f.write("".join(r + "\n" for r in files))
    if a.summary_out:
        with open(a.summary_out, "w") as f:
            json.dump({"stats": stats}, f, indent=1, sort_keys=True)
            f.write("\n")
    print("azure-policy-oracle: " + ", ".join(f"{k} {v}" for k, v in stats.items()), file=sys.stderr)
    return 0


if __name__ == "__main__":
    sys.exit(main())
