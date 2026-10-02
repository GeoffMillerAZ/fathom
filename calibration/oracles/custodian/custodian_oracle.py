"""custodian_oracle.py - the language-breadth harness's oracle for the
cloud-custodian doc-engine dialect (design section 1.3, 13.4 step 19;
LB-Q-cloud-custodian), and the generator of its CloudFormation type map.

Run it with the venv that holds c7n (pip install c7n, 0.9.53 measured):

    python custodian_oracle.py -subject <checkout> -out <file> [-files-out <file>] [-summary-out <file>]
    python custodian_oracle.py -type-map-out calibration/oracles/custodian/cfn_types.json \
        [-doc-json internal/extractor/dialects/cloud-custodian/doc.json]

Oracle (-out), tier A for what c7n decides, tier C for what it does not:

  - c7n's own loader (`c7n.loader.PolicyLoader.load_file`) reads every .yaml and
    .yml of the subject; a file whose root is a mapping with a `policies` list
    is a Custodian file. `custodian validate`'s check (the loader's schema
    validator over the file's data) is run on each and counted in -summary-out;
    a file that fails it is still listed, because the dialect reads a policy
    whether or not it validates (the summary says how many did not).
  - definitions (tier A: the loader's names and resources): one `policy` per
    policies[] entry the loader returns, named by its `name`, whose `resource`
    c7n resolved (a policy c7n cannot load - an unknown resource, a duplicate
    name - is not listed).
  - lines (tier C, a yaml walk): c7n's loader keeps no source positions, so each
    policy's line is read from an independent yaml composition (PyYAML's
    compose: the start line of the policy's mapping). The reading's
    metric_tiers record A for names and kinds and C for the lines' use.
  - references (tier A: c7n's registry decides whether a type exists): from each
    policy, every filter and action `type` c7n's own parse of the policy
    resolves (an `or`, `and` or `not` block's members included; a bare string
    names its type); a type the resource's registry does not know is not listed.
  - the type map (-type-map-out): for every AWS resource in c7n's registry with a
    `cfn_type`, `aws.<name>` and `<name>` to that CloudFormation type, sorted;
    -doc-json writes the same map into a dialect's doc.json target_map.

Directories the connector never reads (testdata, vendor, node_modules,
generated, .git and the editor directories) are not walked. Bench-only; never a
product dependency.
"""

import argparse
import json
import os
import sys

import yaml

SKIP_DIRS = {"vendor", ".git", "testdata", "generated", "node_modules", ".idea", ".vscode", ".claude"}
EXTS = {".yaml", ".yml"}


def load_registry():
    from c7n.resources import load_available
    load_available()
    from c7n.provider import clouds
    return clouds["aws"].resources


def type_map():
    reg = load_registry()
    out = {}
    for name in sorted(reg.keys()):
        cfn = getattr(reg[name].resource_type, "cfn_type", None)
        if cfn:
            out["aws." + name] = cfn
            out[name] = cfn
    return dict(sorted(out.items()))


def write_type_map(path, doc_json):
    m = type_map()
    with open(path, "w") as f:
        json.dump(m, f, indent=1, sort_keys=True)
        f.write("\n")
    if doc_json:
        with open(doc_json) as f:
            doc = json.load(f)
        for c in doc.get("cross_file", []):
            if c.get("resolve") == "resource-type":
                c["target_map"] = m
        with open(doc_json, "w") as f:
            json.dump(doc, f, indent=1)
            f.write("\n")
    print(f"custodian-oracle: {len(m)} map entries from c7n's registry", file=sys.stderr)


def find_files(root):
    found = []
    for dirpath, dirnames, filenames in os.walk(root):
        dirnames[:] = sorted(d for d in dirnames if d not in SKIP_DIRS)
        for f in sorted(filenames):
            if os.path.splitext(f)[1].lower() in EXTS:
                found.append(os.path.relpath(os.path.join(dirpath, f), root).replace(os.sep, "/"))
    return sorted(found)


def policy_lines(text):
    """name -> [lines] of the policies list entries, from an independent yaml composition."""
    try:
        node = yaml.compose(text, Loader=yaml.SafeLoader)
    except yaml.YAMLError:
        return None
    if not isinstance(node, yaml.MappingNode):
        return None
    for k, v in node.value:
        if getattr(k, "value", None) == "policies" and isinstance(v, yaml.SequenceNode):
            out = []
            for item in v.value:
                name = None
                if isinstance(item, yaml.MappingNode):
                    for ik, iv in item.value:
                        if getattr(ik, "value", None) == "name" and isinstance(iv, yaml.ScalarNode):
                            name = iv.value
                out.append((name, item.start_mark.line + 1, item.end_mark.line))
            return out
    return None


def types_of(items, registry, out):
    """Filter/action types of a list as c7n reads it: a dict's `type`, a bare string,
    and an or/and/not block's members (recursively); kept only when the registry knows them."""
    if isinstance(items, dict):
        items = [items]
    for it in items if isinstance(items, list) else []:
        if isinstance(it, str):
            if registry is None or it in registry:
                out.append(it)
        elif isinstance(it, dict):
            t = it.get("type")
            if isinstance(t, str) and (registry is None or t in registry):
                out.append(t)
            for block in ("or", "and", "not"):
                if block in it:
                    types_of(it[block], registry, out)


def main(argv=None):
    ap = argparse.ArgumentParser()
    ap.add_argument("-subject")
    ap.add_argument("-out")
    ap.add_argument("-files-out", default="")
    ap.add_argument("-summary-out", default="")
    ap.add_argument("-type-map-out", default="")
    ap.add_argument("-doc-json", default="")
    a = ap.parse_args(argv)
    if a.type_map_out:
        write_type_map(a.type_map_out, a.doc_json)
        if not a.subject:
            return 0
    if not a.subject or not a.out:
        ap.error("-subject and -out are required for an oracle run")

    from c7n.config import Config
    from c7n.loader import PolicyLoader
    from c7n.version import version as c7n_version
    load_registry()  # load_available(): every installed provider (c7n-azure, c7n-tencentcloud)

    root = os.path.abspath(a.subject)
    defs, refs, files = [], [], []
    stats = {"files": 0, "custodian_files": 0, "not_custodian": 0, "load_failed": 0,
             "validate_ok": 0, "validate_failed": 0, "policies": 0, "listed": 0, "unlisted": 0,
             "unmatched_lines": 0, "unlisted_unloadable": 0, "filter_action_types": 0, "unknown_types": 0}
    failed = {}
    for rel in find_files(root):
        stats["files"] += 1
        path = os.path.join(root, rel)
        try:
            with open(path, encoding="utf-8") as f:
                text = f.read()
            data = yaml.safe_load(text)
        except Exception:  # pylint: disable=broad-except
            stats["not_custodian"] += 1
            continue
        if not (isinstance(data, dict) and isinstance(data.get("policies"), list)):
            stats["not_custodian"] += 1
            continue
        stats["custodian_files"] += 1
        files.append(rel)
        loader = PolicyLoader(Config.empty())
        try:
            errors = loader.validator.validate(data)
            ok = not errors
        except Exception as e:  # pylint: disable=broad-except
            ok, errors = False, [str(e)]
        stats["validate_ok" if ok else "validate_failed"] += 1
        if not ok:
            failed.setdefault(str(errors[0])[:100], []).append(rel)
        loaded = []
        try:
            loaded = list(loader.load_data(data, path, validate=False))
        except Exception as e:  # pylint: disable=broad-except
            # c7n loads a file whole: one policy it refuses (an unknown resource or
            # filter type, a duplicate name, a provider that is not installed) fails
            # the file. Each policy is then loaded alone, so the rest are still listed.
            stats["load_failed"] += 1
            failed.setdefault("load: " + str(e)[:90], []).append(rel)
            for item in data["policies"]:
                try:
                    loaded.extend(PolicyLoader(Config.empty()).load_data({"policies": [item]}, path, validate=False))
                except Exception:  # pylint: disable=broad-except
                    stats["unlisted_unloadable"] += 1
        lines = policy_lines(text) or []
        by_name = {}
        for name, line, end in lines:
            by_name.setdefault(name, []).append((line, end))
        stats["policies"] += len(data["policies"])
        for p in loaded:
            name = p.name
            spans = by_name.get(name)
            if not spans:
                stats["unmatched_lines"] += 1
                continue
            line, end = spans.pop(0)
            stats["listed"] += 1
            defs.append({"path": rel, "name": name, "line": line, "end": end, "kind": "policy"})
            seen = []
            mgr = p.resource_manager
            types_of(p.data.get("filters", []), mgr.filter_registry, seen)
            types_of(p.data.get("actions", []), mgr.action_registry, seen)
            for t in sorted(set(seen)):
                refs.append({"path": rel, "from": name, "to": t})
            stats["filter_action_types"] += len(set(seen))
    stats["unlisted"] = stats["policies"] - stats["listed"]
    with open(a.out, "w") as f:
        json.dump({"tool": "c7n (custodian validate's loader)", "version": c7n_version, "tier": "A",
                   "definitions": sorted(defs, key=lambda d: (d["path"], d["line"], d["name"])),
                   "references": sorted(refs, key=lambda r: (r["path"], r["from"], r["to"]))}, f, indent=1)
        f.write("\n")
    if a.files_out:
        with open(a.files_out, "w") as f:
            f.write("".join(r + "\n" for r in sorted(files)))
    if a.summary_out:
        with open(a.summary_out, "w") as f:
            json.dump({"stats": stats, "failures": failed}, f, indent=1, sort_keys=True)
            f.write("\n")
    print("custodian-oracle: " + ", ".join(f"{k} {v}" for k, v in stats.items()), file=sys.stderr)
    return 0


if __name__ == "__main__":
    sys.exit(main())
