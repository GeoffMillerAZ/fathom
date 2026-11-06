#!/usr/bin/env python3
"""select_subjects.py - preregistered subject selection for the lang-library resource readings (LL-M3, LL-M4, LL-M5).

The rule is mechanical and reads only file text and paths. It never runs a reader and never looks at reader
output. Steps:

  pin     resolve each repo without a sha to its default-branch head (git ls-remote) and write it into the subject file
  clone   fetch each pinned commit (depth 1) into <clones>/<tool>-<owner>-<repo>
  select  apply the rule and write the "selected" list back into the subject file

Candidates are ordered by sha256(repo + "/" + path) and the first `per_repo_cap` that pass the rule are taken, so the
choice does not follow alphabetical order, content or size. Only the oracle may later reject a selected item (it
could not decode or initialise it), and the reading reports each such rejection.

usage: select_subjects.py {pin,clone,select} --subjects <file.json> [--clones <dir>]
"""
import argparse, hashlib, json, os, re, subprocess, sys

TF_PROVIDERS = {"aws", "cloudinit", "helm", "http", "local", "null", "time", "tls"}


def run(*a, cwd=None):
    return subprocess.run(a, cwd=cwd, check=True, capture_output=True, text=True).stdout


def clone_dir(clones, tool, repo):
    return os.path.join(clones, tool + "-" + repo.replace("/", "-"))


def order_key(repo, path):
    return hashlib.sha256((repo + "/" + path).encode()).hexdigest()


def walk(root):
    for d, dirs, files in os.walk(root):
        dirs[:] = sorted(x for x in dirs if x not in (".git", ".terraform", "node_modules"))
        yield os.path.relpath(d, root), sorted(files)


def read(p):
    with open(p, "rb") as f:
        return f.read().decode("utf-8", "replace")


def brace_body(text, start):
    """text[start] is '{'. Return the text up to its matching '}', skipping quoted strings and comments."""
    depth, i, n = 0, start, len(text)
    while i < n:
        c = text[i]
        if c == '"':
            i += 1
            while i < n and text[i] != '"':
                i += 2 if text[i] == "\\" else 1
        elif c == "#" or text.startswith("//", i):
            while i < n and text[i] != "\n":
                i += 1
        elif c == "{":
            depth += 1
        elif c == "}":
            depth -= 1
            if depth == 0:
                return text[start + 1 : i]
        i += 1
    return text[start + 1 :]


def tf_dir_ok(root, rel, files):
    names = [f for f in files if f.endswith(".tf")]
    if not names:
        return False
    if any(f.endswith((".tf.json", ".tofu", ".tofu.json")) or f in ("override.tf",) or f.endswith("_override.tf") for f in files):
        return False
    managed = 0
    for f in names:
        t = read(os.path.join(root, rel, f))
        for m in re.finditer(r'(?m)^\s*(resource|data)\s+"([A-Za-z0-9_-]+)"\s+"[^"]+"\s*{', t):
            if m.group(2).split("_")[0] not in TF_PROVIDERS:
                return False
            if m.group(1) == "resource":
                managed += 1
        for m in re.finditer(r'(?m)^\s*provider\s+"([^"]+)"', t):
            if m.group(1) not in TF_PROVIDERS:
                return False
        for m in re.finditer(r'(?m)^\s*module\s+"[^"]+"\s*{', t):
            body = brace_body(t, m.end() - 1)
            s = re.search(r'(?m)^\s*source\s*=\s*"([^"]*)"', body)
            if not s or not (s.group(1).startswith("./") or s.group(1).startswith("../")):
                return False
        for m in re.finditer(r"required_providers\s*{", t):
            body = brace_body(t, m.end() - 1)
            for s in re.finditer(r'source\s*=\s*"([^"]+)"', body):
                src = s.group(1).split("/")
                if src[-1] not in TF_PROVIDERS or (len(src) > 1 and src[-2] != "hashicorp"):
                    return False
            depth = 0
            for line in body.splitlines():
                k = re.match(r"\s*([A-Za-z0-9_-]+)\s*=", line)
                if depth == 0 and k and k.group(1) not in TF_PROVIDERS:
                    return False
                depth += line.count("{") - line.count("}")
    return managed > 0


def select_terraform(root, repo, cap):
    out = []
    for rel, files in walk(root):
        if rel.split(os.sep)[0] in ("wrappers",):
            continue
        if tf_dir_ok(root, rel, files):
            out.append(rel)
    out.sort(key=lambda p: order_key(repo, p))
    return out[:cap]


CFN_EXT = (".yaml", ".yml", ".json")


def cfn_ok(path):
    try:
        if os.path.getsize(path) > 512 * 1024:
            return False
        t = read(path)
    except OSError:
        return False
    return "AWSTemplateFormatVersion" in t and re.search(r'(?m)^\s*"?Resources"?\s*:', t) is not None


def select_cfn(root, repo, cap, skip_dirs):
    cands = []
    for rel, files in walk(root):
        parts = rel.split(os.sep)
        if any(p in skip_dirs for p in parts):
            continue
        for f in files:
            if f.endswith(CFN_EXT):
                p = os.path.normpath(os.path.join(rel, f))
                if cfn_ok(os.path.join(root, p)):
                    cands.append(p)
    cands.sort(key=lambda p: order_key(repo, p))
    return cands[:cap]


def pulumi_ok(path):
    try:
        t = read(path)
    except OSError:
        return False
    if "${PROJECT}" in t or "${DESCRIPTION}" in t:
        return False
    yaml_rt = re.search(r"(?m)^runtime:\s*yaml\s*$", t) or re.search(r"(?m)^runtime:\s*\n\s+name:\s*yaml\s*$", t)
    return bool(yaml_rt) and re.search(r"(?m)^resources:", t) is not None


def select_pulumi(root, repo, cap):
    cands = []
    for rel, files in walk(root):
        for f in files:
            if f in ("Pulumi.yaml", "Pulumi.yml"):
                p = os.path.normpath(os.path.join(rel, f))
                if pulumi_ok(os.path.join(root, p)):
                    cands.append(p)
    cands.sort(key=lambda p: order_key(repo, p))
    return cands[:cap]


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("step", choices=["pin", "clone", "select"])
    ap.add_argument("--subjects", required=True)
    ap.add_argument("--clones")
    a = ap.parse_args()
    sf = json.load(open(a.subjects))
    tool = sf["tool"]
    if a.step == "pin":
        for r in sf["repos"]:
            if not r.get("sha"):
                r["sha"] = run("git", "ls-remote", "https://github.com/" + r["repo"], "HEAD").split()[0]
    elif a.step == "clone":
        for r in sf["repos"]:
            d = clone_dir(a.clones, tool, r["repo"])
            os.makedirs(d, exist_ok=True)
            if not os.path.isdir(os.path.join(d, ".git")):
                run("git", "init", "-q", cwd=d)
                run("git", "remote", "add", "origin", "https://github.com/" + r["repo"], cwd=d)
            run("git", "fetch", "-q", "--depth", "1", "origin", r["sha"], cwd=d)
            run("git", "-c", "advice.detachedHead=false", "checkout", "-q", r["sha"], cwd=d)
    else:
        sel = []
        for r in sf["repos"]:
            root = clone_dir(a.clones, tool, r["repo"])
            cap = sf["per_repo_cap"]
            if tool == "terraform":
                items = select_terraform(root, r["repo"], cap)
            elif tool == "cloudformation":
                items = select_cfn(root, r["repo"], cap, set(sf.get("skip_dirs", [])))
            else:
                items = select_pulumi(root, r["repo"], cap)
            for p in items:
                sel.append({"repo": r["repo"], "path": p})
        sf["selected"] = sel
    json.dump(sf, open(a.subjects, "w"), indent=2)
    open(a.subjects, "a").write("\n")


if __name__ == "__main__":
    main()
