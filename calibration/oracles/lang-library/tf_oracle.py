#!/usr/bin/env python3
"""tf_oracle.py - the Terraform oracle for LL-M3: OpenTofu's own reading of each selected root.

For each root of subjects/terraform.json it copies the repository (without .git) into --work, runs
`tofu init -backend=false -input=false` offline against the pinned filesystem provider mirror, then
`tofu show -json -config`. Nothing here reads a Terraform file itself and nothing imports the readers. The
provider schema (`tofu providers schema -json`, one scratch root that requires every pinned provider) says which
names are blocks and which are attributes, which the JSON expressions alone cannot say.

Per resource (managed and data, in the root module) it writes every attribute leaf with the oracle class (a path
that several blocks write has one entry per block):
constant (constant_value), references (references) or empty ({}). A nested block takes [*] on every segment.
tofu drops dynamic blocks, lifecycle and attributes it cannot place, so the oracle has none of those.

env: TOFU (tofu binary, default tofu), TF_CLI_CONFIG_FILE (the mirror configuration).
usage: tf_oracle.py --subjects subjects/terraform.json --clones <dir> --work <dir> --out <file.json>
"""
import argparse, json, os, shutil, subprocess, sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from select_subjects import clone_dir  # noqa: E402  (path helper only)

PROVIDERS = ["aws", "cloudinit", "helm", "http", "local", "null", "time", "tls"]
TOFU = os.environ.get("TOFU", "tofu")


def tofu(cwd, *args, timeout=600):
    env = dict(os.environ, TF_IN_AUTOMATION="1", TF_INPUT="0", CHECKPOINT_DISABLE="1")
    return subprocess.run([TOFU, *args], cwd=cwd, env=env, capture_output=True, text=True, timeout=timeout)


def reduce_block(b):
    out = {"attrs": sorted((b.get("attributes") or {}).keys()), "blocks": {}}
    for name, bt in (b.get("block_types") or {}).items():
        out["blocks"][name] = reduce_block(bt["block"])
    return out


def load_schema(work):
    d = os.path.join(work, "_schema")
    os.makedirs(d, exist_ok=True)
    with open(os.path.join(d, "versions.tf"), "w") as f:
        f.write("terraform {\n  required_providers {\n")
        for p in PROVIDERS:
            f.write('    %s = { source = "hashicorp/%s" }\n' % (p, p))
        f.write("  }\n}\n")
    r = tofu(d, "init", "-backend=false", "-input=false", "-no-color")
    if r.returncode != 0:
        sys.exit("schema init failed: " + r.stderr[-400:] + r.stdout[-400:])
    r = tofu(d, "providers", "schema", "-json")
    if r.returncode != 0:
        sys.exit("schema failed: " + r.stderr[-400:])
    js = json.loads(r.stdout)
    sch = {}
    for pname, p in js["provider_schemas"].items():
        for kind, key in (("managed", "resource_schemas"), ("data", "data_source_schemas")):
            for t, s in (p.get(key) or {}).items():
                sch[kind + "." + t] = reduce_block(s["block"])
    return sch


def klass(v):
    if isinstance(v, dict):
        if "constant_value" in v:
            return "constant", []
        if "references" in v:
            return "references", sorted(set(v["references"]))
    return "empty", []


def leaves(expr, blk, prefix, out):
    for key, val in expr.items():
        if key in blk["blocks"]:
            els = val if isinstance(val, list) else [val]
            for el in els:
                if isinstance(el, dict):
                    leaves(el, blk["blocks"][key], prefix + key + "[*].", out)
            continue
        c, refs = klass(val)
        out.setdefault(prefix + key, []).append({"class": c, "refs": refs})


def read_root(cfg_json, schema):
    rm = cfg_json.get("root_module") or {}
    res = {}
    missing = []
    for r in rm.get("resources") or []:
        mode = r["mode"]
        key = "%s.%s.%s" % (mode, r["type"], r["name"])
        blk = schema.get(mode + "." + r["type"])
        if blk is None:
            missing.append(key)
            continue
        out = {}
        leaves(r.get("expressions") or {}, blk, "", out)
        res[key] = out
    variables = {n: {"has_default": "default" in v} for n, v in (rm.get("variables") or {}).items()}
    return {"resources": res, "variables": variables, "module_calls": len(rm.get("module_calls") or {}), "missing_schema": missing}


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--subjects", required=True)
    ap.add_argument("--clones", required=True)
    ap.add_argument("--work", required=True)
    ap.add_argument("--out", required=True)
    a = ap.parse_args()
    sf = json.load(open(a.subjects))
    os.makedirs(a.work, exist_ok=True)
    schema = load_schema(a.work)
    version = tofu(a.work, "version").stdout.splitlines()[0]
    roots = {}
    copied = {}
    for s in sf["selected"]:
        key = s["repo"] + ":" + s["path"]
        if s["repo"] not in copied:
            dst = os.path.join(a.work, s["repo"].replace("/", "-"))
            shutil.rmtree(dst, ignore_errors=True)
            shutil.copytree(clone_dir(a.clones, sf["tool"], s["repo"]), dst, ignore=shutil.ignore_patterns(".git"), symlinks=True)
            copied[s["repo"]] = dst
        d = os.path.join(copied[s["repo"]], s["path"])
        r = tofu(d, "init", "-backend=false", "-input=false", "-no-color")
        if r.returncode != 0:
            roots[key] = {"ok": False, "stage": "init", "error": (r.stderr or r.stdout).strip().splitlines()[-1][:300] if (r.stderr or r.stdout).strip() else "init failed"}
            continue
        r = tofu(d, "show", "-json", "-config")
        if r.returncode != 0:
            roots[key] = {"ok": False, "stage": "show", "error": (r.stderr or r.stdout).strip().splitlines()[0][:300]}
            continue
        try:
            roots[key] = dict(ok=True, **read_root(json.loads(r.stdout), schema))
        except Exception as e:  # an oracle failure is a rejection, never a guess
            roots[key] = {"ok": False, "stage": "parse", "error": repr(e)[:300]}
    json.dump({"oracle": version + ", tofu show -json -config", "roots": roots}, open(a.out, "w"), indent=1, sort_keys=True)
    json.dump(schema, open(a.out + ".schema.json", "w"), sort_keys=True)


if __name__ == "__main__":
    main()
