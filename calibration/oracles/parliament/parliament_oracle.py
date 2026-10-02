"""parliament_oracle.py - the language-breadth harness's oracle for the
aws-iam-policy doc-engine dialect (design section 1.3, 13.4 step 19;
LB-Q-aws-iam-policy).

Run it with the venv that holds parliament (pip install parliament; 1.6.4
measured, BSD-3, duo-labs/parliament at 574ea450):

    python parliament_oracle.py -subject <checkout> -out <file> [-files-out <file>] [-summary-out <file>]

Tier C: parliament is an IAM policy linter, not the format's reference parser,
and its reader is a JSON parser that tracks line numbers (jsoncfg). It reads
.json only, so a YAML policy is outside what this oracle can say. Every .json of
the subject is read the way parliament reads a policy (`jsoncfg.loads_config`,
its own `make_list` and `Statement` classes); a file is a policy document when
its root is an object with `Statement` and a `Version` of 2012-10-17 or
2008-10-17. Emitted:

  - definitions: a `policy` per document (named by the file, at its root
    object's line) and a `rule` per statement (parliament's `Statement` of each
    `make_list(Statement)` object), named by its `Sid`, else its position in
    the list (from 0), at the statement object's line;
  - references, from each statement: every value of Action, NotAction,
    Resource, NotResource, Principal and NotPrincipal as parliament lists them
    (`make_list`; a Principal object's members are listed, one list per
    principal type), as written.

The truth is not parliament's verdict on the policy: a statement it would call
malformed is still a statement. -summary-out counts how many documents
parliament's full analysis accepts and how many it reports MALFORMED, so the
reading can say how far the truth reaches past what the tool approves.

Directories the connector never reads (testdata, vendor, node_modules,
generated, .git and the editor directories) are not walked. Bench-only; never a
product dependency.
"""

import argparse
import json
import os
import posixpath
import sys

import jsoncfg
import parliament
from parliament.misc import make_list

SKIP_DIRS = {"vendor", ".git", "testdata", "generated", "node_modules", ".idea", ".vscode", ".claude"}
VERSIONS = ("2012-10-17", "2008-10-17")
ELEMENTS = ("Action", "NotAction", "Resource", "NotResource", "Principal", "NotPrincipal")


def find_files(root):
    found = []
    for dirpath, dirnames, filenames in os.walk(root):
        dirnames[:] = sorted(d for d in dirnames if d not in SKIP_DIRS)
        for f in sorted(filenames):
            if f.lower().endswith(".json"):
                found.append(os.path.relpath(os.path.join(dirpath, f), root).replace(os.sep, "/"))
    return sorted(found)


def line_of(node):
    return jsoncfg.node_location(node).line


def scalar(node):
    """A scalar node's value as text, None for a null or a non-scalar."""
    if jsoncfg.node_is_scalar(node) and node.value is not None:
        v = node.value
        return v if isinstance(v, str) else json.dumps(v)
    return None


def values(node):
    """The values of an element parliament lists: make_list for Action and Resource;
    a Principal object's members, each a list per principal type."""
    out = []
    if jsoncfg.node_is_object(node):
        for _, member in node:
            out.extend(values(member))
        return out
    for item in make_list(node):
        if jsoncfg.node_is_object(item):
            out.extend(values(item))
        else:
            s = scalar(item)
            if s is not None:
                out.append(s)
    return out


def is_policy(root):
    if not jsoncfg.node_is_object(root):
        return False
    keys = {k for k, _ in root}
    if "Statement" not in keys or "Version" not in keys:
        return False
    return scalar(root["Version"]) in VERSIONS


def main(argv=None):
    ap = argparse.ArgumentParser()
    ap.add_argument("-subject", required=True)
    ap.add_argument("-out", required=True)
    ap.add_argument("-files-out", default="")
    ap.add_argument("-summary-out", default="")
    ap.add_argument("-no-analyze", action="store_true")
    a = ap.parse_args(argv)
    root_dir = os.path.abspath(a.subject)
    defs, refs, files = [], [], []
    stats = {"files": 0, "policies": 0, "not_json": 0, "not_policy": 0, "statements": 0,
             "non_object_statements": 0, "analysis_ok": 0, "analysis_malformed": 0, "analysis_error": 0}
    for rel in find_files(root_dir):
        stats["files"] += 1
        try:
            with open(os.path.join(root_dir, rel), encoding="utf-8-sig") as f:
                text = f.read()
            root = jsoncfg.loads_config(text)
        except Exception:  # pylint: disable=broad-except
            stats["not_json"] += 1
            continue
        if not is_policy(root):
            stats["not_policy"] += 1
            continue
        stats["policies"] += 1
        files.append(rel)
        defs.append({"path": rel, "name": posixpath.basename(rel), "line": line_of(root), "kind": "policy"})
        for i, stmt in enumerate(make_list(root["Statement"])):
            if not jsoncfg.node_is_object(stmt):
                stats["non_object_statements"] += 1
                continue
            stats["statements"] += 1
            sid = scalar(stmt["Sid"]) if "Sid" in {k for k, _ in stmt} else None
            name = sid if sid else str(i)
            defs.append({"path": rel, "name": name, "line": line_of(stmt), "kind": "rule"})
            seen = set()
            for key, member in stmt:
                if key in ELEMENTS:
                    for v in values(member):
                        seen.add(v)
            for v in sorted(seen):
                refs.append({"path": rel, "from": name, "to": v})
        if not a.no_analyze:
            try:
                pol = parliament.analyze_policy_string(text, ignore_private_auditors=True)
                malformed = any(f.issue in ("MALFORMED", "MALFORMED_JSON") for f in pol.findings)
                stats["analysis_malformed" if malformed else "analysis_ok"] += 1
            except Exception:  # pylint: disable=broad-except
                stats["analysis_error"] += 1
    with open(a.out, "w") as f:
        json.dump({"tool": "parliament", "version": parliament.__version__, "tier": "C",
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
    print("parliament-oracle: " + ", ".join(f"{k} {v}" for k, v in stats.items()), file=sys.stderr)
    return 0


if __name__ == "__main__":
    sys.exit(main())
