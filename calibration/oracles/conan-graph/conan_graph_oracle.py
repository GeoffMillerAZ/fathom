#!/usr/bin/env python3
"""conan-graph oracle: the language-breadth harness's tier-A oracle for the conan dialect
(design 1.3, owner decision D4). Writes indexbench lang-oracle's tool-oracle JSON (-tool-oracle,
design 10) for conanfile.py recipes and conanfile.txt consumers:

    <conan venv>/bin/python conan_graph_oracle.py -subject <checkout> -conan <conan> -home <CONAN_HOME>
        -index <conan-center-index checkout> (-recipes name,name,... | -files file) [-scratch dir]
        [-files-out file] [-jobs N] > out.json

The tool is Conan itself (2.x, offline: `-nr`, a CONAN_HOME in scratch). For each recipe or
consumer file the oracle runs

    conan graph info <file or recipe dir> [--version V] --format json -nr  [-s ...] [-o &:...]

and reads the root node: the name conan evaluated and its direct dependencies (requires,
tool_requires, test_requires; python_requires), by name. A graph stops at the first requirement it
cannot resolve, so the closure is exported on demand: each unresolved reference's recipe is
`conan export`ed from the conan-center-index checkout (every version its config.yml lists, newest
three for a range; the label the reference asks for, for a pinned version) and the run repeats.

A recipe is code, so a requirement behind `if self.options.x`, `if self.settings.os == ...` or
`if Version(self.version) < ...` shows only when conan evaluates that branch. The reference set is
therefore the union over a matrix: four profiles (macOS with apple-clang, Linux with gcc, Windows
with msvc, Windows with gcc; the build profile follows the host), for each the versions of the recipe's folder
(newest, oldest and two between) at default options, and, at the newest and the oldest version, each option set to
each of its other values one at a time, and every boolean option all True and all False (options
with more than 8 values and ANY are left out; at most 60 single-option runs per profile and version). A requirement the matrix never activates is a dialect-only reference
(the reading's ref_precision shows it); it is not hidden.

  - definitions: one `package` per attested recipe (the conan-evaluated name, at the line of its
    `name = "..."` literal, found by Python's own `ast`), one per attested conanfile.txt (named
    `conanfile.txt`, line 1).
  - references: (package, dependency name) for every direct dependency of any run.
  - no cross_references: a requirement reaches a recipe folder only through config.yml and
    conan's version resolution, which is no doc-engine edge; the reading does not measure L3.

Bench-only; never a product dependency.
"""
import argparse, ast, json, os, re, subprocess, sys, threading, shutil
from concurrent.futures import ThreadPoolExecutor

try:
    import yaml
except ImportError:  # the conan venv carries PyYAML
    sys.exit("conan-graph oracle: run it with the conan venv's python (PyYAML)")

# host and build machine alike: a recipe's build requirements depend on the build profile too
# (msys2 and ninja behind `self.settings_build.os == "Windows"`)
LINUX = []
WINDOWS = []
for side in ("-s", "-s:b"):
    LINUX += [side, "os=Linux", side, "arch=x86_64", side, "compiler=gcc", side, "compiler.version=13",
              side, "compiler.libcxx=libstdc++11", side, "build_type=Release"]
    WINDOWS += [side, "os=Windows", side, "arch=x86_64", side, "compiler=msvc", side, "compiler.version=193",
                side, "compiler.runtime=dynamic", side, "build_type=Release"]
# Windows with gcc (mingw): the recipes' `not is_msvc(self)` branches (msys2, autotools tool_requires)
MINGW = []
for side in ("-s", "-s:b"):
    MINGW += [side, "os=Windows", side, "arch=x86_64", side, "compiler=gcc", side, "compiler.version=13",
              side, "compiler.libcxx=libstdc++11", side, "build_type=Release"]
PROFILES = {"macos": [], "linux": LINUX, "windows": WINDOWS, "mingw": MINGW}

def retry_eperm(fn, tries=12):
    """A macOS TCC verdict on an external volume can wedge for seconds (EPERM); wait it out."""
    import time
    for i in range(tries):
        try:
            return fn()
        except PermissionError:
            if i == tries - 1:
                raise
            time.sleep(10)


OPTION_CAP = 60
export_lock = threading.Lock()
exported = set()          # (name, version)
failed_exports = {}       # name -> reason
notes = []


def note(msg):
    notes.append(msg)
    print("note:", msg, file=sys.stderr)


def run(cmd, env, timeout=600):
    import time
    for attempt in range(8):
        p = subprocess.run(cmd, env=env, capture_output=True, text=True, timeout=timeout)
        # a macOS TCC wedge on the external volume fails a process with EPERM; it clears in seconds
        blob = p.stderr + p.stdout
        if p.returncode != 0 and ("Operation not permitted" in blob or "PermissionError" in blob or "[Errno 1]" in blob
                                  or "Traceback" in blob and "no JSON" not in blob and not blob.lstrip().startswith("{")) and attempt < 7:
            time.sleep(10)
            continue
        break
    return p.returncode, p.stdout, p.stderr


def vkey(v):
    parts = re.split(r"[.\-+]", str(v))
    return tuple((0, int(x)) if x.isdigit() else (1, x) for x in parts)


def config_versions(index, name):
    """{version: folder} from recipes/<name>/config.yml, newest first, or None."""
    p = os.path.join(index, "recipes", name, "config.yml")
    if not os.path.exists(p):
        return None
    def read():
        with open(p) as f:
            return yaml.safe_load(f) or {}
    d = retry_eperm(read)
    vs = {str(k): str(v.get("folder", "all")) for k, v in (d.get("versions") or {}).items()}
    return dict(sorted(vs.items(), key=lambda kv: vkey(kv[0]), reverse=True))


def export_recipe(conan, env, index, name, version=None, newest=3):
    """Export recipes/<name> at the version(s) asked for. Returns True when something is in the cache."""
    cv = config_versions(index, name)
    if not cv:
        failed_exports[name] = "no recipes/%s/config.yml in the index" % name
        return False
    if version:
        folder = cv.get(version) or next(iter(cv.values()))
        todo = [(version, folder)]
    else:
        todo = list(cv.items())[:newest]
    ok = False
    for v, folder in todo:
        with export_lock:
            if (name, v) in exported:
                ok = True
                continue
            d = os.path.join(index, "recipes", name, folder)
            rc, out, err = run([conan, "export", d, "--version", v], env)
            if rc != 0:
                failed_exports[name] = (err or out).strip().splitlines()[-1][:200] if (err or out).strip() else "export failed"
                continue
            exported.add((name, v))
            ok = True
    return ok


def graph_info(conan, env, target, version, extra):
    cmd = [conan, "graph", "info", target, "--format", "json", "-nr"]
    if version:
        cmd += ["--version", version]
    cmd += extra
    rc, out, err = run(cmd, env)
    try:
        j = json.loads(out)
    except Exception:
        return None, (err or out)[-300:]
    return j, None


def evaluate(conan, env, index, target, version, extra, tries=80):
    """graph info with on-demand export of unresolved requirements. Returns (root node, why-not)."""
    attempts = {}
    for _ in range(tries):
        j, why = graph_info(conan, env, target, version, extra)
        if j is None:
            return None, "no JSON: %s" % (why or "")
        g = j.get("graph", {})
        err = g.get("error")
        if not err:
            return g["nodes"]["0"], None
        req = (err.get("require") or {}).get("ref") or ""
        m = re.match(r"([^/\[]+)/(.*)$", req)
        if not m:
            return None, "graph error: %s" % str(err)[:200]
        name, ver = m.group(1), m.group(2)
        ver = re.sub(r"[@#].*$", "", ver)
        pinned = None if ver.startswith("[") else ver
        n = attempts.get((name, pinned), 0)
        attempts[(name, pinned)] = n + 1
        if n >= 2:
            return None, "unresolvable %s/%s: %s" % (name, ver, failed_exports.get(name, str(err)[:120]))
        # first try the newest three versions; a range they do not satisfy gets every version
        if not export_recipe(conan, env, index, name, pinned, newest=3 if n == 0 else 10_000):
            return None, "cannot export %s: %s" % (name, failed_exports.get(name, "?"))
    return None, "too many closure steps"


def direct_names(node):
    names = set()
    for dep in (node.get("dependencies") or {}).values():
        if dep.get("direct"):
            ref = dep.get("ref") or dep.get("require") or ""
            names.add(re.split(r"[/\[]", ref, maxsplit=1)[0])
    for pr in (node.get("python_requires") or {}).values():
        ref = pr.get("ref") if isinstance(pr, dict) else str(pr)
        names.add(re.split(r"[/\[]", str(ref), maxsplit=1)[0])
    names.discard("")
    return names


def recipe_name_line(path):
    """(name, line) of the `name = "<literal>"` of the first ConanFile subclass that writes one."""
    def read():
        with open(path, encoding="utf-8", errors="replace") as f:
            return f.read()
    src = retry_eperm(read)
    try:
        tree = ast.parse(src)
    except SyntaxError:
        return None, 0
    for node in ast.walk(tree):
        if not isinstance(node, ast.ClassDef):
            continue
        bases = [b.id if isinstance(b, ast.Name) else b.attr if isinstance(b, ast.Attribute) else "" for b in node.bases]
        if "ConanFile" not in bases:
            continue
        for st in node.body:
            if isinstance(st, ast.Assign) and len(st.targets) == 1 and isinstance(st.targets[0], ast.Name) \
                    and st.targets[0].id == "name" and isinstance(st.value, ast.Constant) and isinstance(st.value.value, str):
                return st.value.value, st.lineno
    return None, 0


def option_runs(node, cap):
    """One run per option value other than the current one, then every boolean option all True and all
    False at once (a requirement nested under two options needs both)."""
    defs = node.get("options_definitions") or {}
    cur = node.get("options") or {}
    runs = []
    bools = []
    for opt, vals in defs.items():
        vals = [str(v) for v in vals]
        if "ANY" in vals or len(vals) > 8 or len(vals) < 2:
            continue
        if sorted(vals) == ["False", "True"]:
            bools.append(opt)
        for v in vals:
            if v != str(cur.get(opt)):
                runs.append(["-o", "&:%s=%s" % (opt, v)])
    runs = runs[:cap]
    for val in ("True", "False"):
        if bools:
            extra = []
            for opt in bools:
                extra += ["-o", "&:%s=%s" % (opt, val)]
            runs.append(extra)
    return runs


def spread(versions, n=4):
    """newest, oldest and evenly spaced ones between, at most n."""
    if len(versions) <= n:
        return versions
    idx = sorted({round(i * (len(versions) - 1) / (n - 1)) for i in range(n)})
    return [versions[i] for i in idx]


def attest_recipe(conan, env, index, rel, version):
    """The root node's direct dependencies over the matrix: for each profile (macOS, Linux, Windows) the
    recipe's versions that share its folder (newest, oldest and two between: a requirement behind
    `Version(self.version) < ...` shows only for them) at default options, and, at the newest version,
    each option at each of its other values one at a time (at most OPTION_CAP runs per profile)."""
    target = os.path.join(index, rel)
    folder = os.path.dirname(target)
    recipe = os.path.relpath(os.path.dirname(folder), os.path.join(index, "recipes"))
    cv = config_versions(index, os.path.basename(os.path.dirname(folder))) or {}
    mine = [v for v, f in cv.items() if f == os.path.basename(folder)] or [version]
    versions = spread(mine)
    name_lit, line = recipe_name_line(target)
    names = set()
    root = None
    errs = []
    for pname, prof in PROFILES.items():
        ends = {}
        for ver in versions:
            node, why = evaluate(conan, env, index, folder, ver, prof)
            if node is None:
                errs.append("%s %s: %s" % (pname, ver, why))
                continue
            root = root or node
            names |= direct_names(node)
            if ver in (versions[0], versions[-1]):
                ends[ver] = node
        for ver, base in ends.items():
            for extra in option_runs(base, OPTION_CAP):
                n2, _ = evaluate(conan, env, index, folder, ver, prof + extra)
                if n2 is not None:
                    names |= direct_names(n2)
    if root is None:
        return None, errs
    return {"path": rel, "name": root.get("name") or name_lit, "line": line, "names": names, "literal": name_lit}, errs


def attest_txt(conan, env, index, subject, rel):
    target = os.path.join(subject, rel)
    names = set()
    root = None
    errs = []
    for pname, prof in PROFILES.items():
        node, why = evaluate(conan, env, index, target, None, prof)
        if node is None:
            errs.append("%s: %s" % (pname, why))
            continue
        root = root or node
        names |= direct_names(node)
    if root is None:
        return None, errs
    return {"path": rel, "name": "conanfile.txt", "line": 1, "names": names, "literal": None}, errs


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("-subject", required=True)
    ap.add_argument("-conan", required=True)
    ap.add_argument("-home", required=True)
    ap.add_argument("-index", required=True, help="conan-center-index checkout the closure is exported from")
    ap.add_argument("-recipes", default="", help="comma-separated recipe names (conan-center-index subject)")
    ap.add_argument("-files", default="", help="file of subject-relative conanfile.txt/.py paths (any subject)")
    ap.add_argument("-files-out", default="")
    ap.add_argument("-jobs", type=int, default=4)
    a = ap.parse_args()
    subject = os.path.realpath(a.subject)
    index = os.path.realpath(a.index)
    os.makedirs(a.home, exist_ok=True)
    env = dict(os.environ, CONAN_HOME=a.home, CONAN_NON_INTERACTIVE="1")
    rc, out, err = run([a.conan, "profile", "detect", "--force"], env)
    version = run([a.conan, "--version"], env)[1].strip()

    jobs = []   # (kind, rel, version)
    if a.recipes:
        for n in [x for x in a.recipes.split(",") if x]:
            cv = config_versions(subject, n)
            if not cv:
                note("%s: no config.yml, skipped" % n)
                continue
            ver, folder = next(iter(cv.items()))
            jobs.append(("py", "recipes/%s/%s/conanfile.py" % (n, folder), ver))
    if a.files:
        for ln in open(a.files).read().split():
            jobs.append(("txt" if ln.endswith(".txt") else "pyfile", ln, None))

    results = []
    lock = threading.Lock()

    def work(job):
        kind, rel, ver = job
        try:
            if kind == "txt":
                r, errs = attest_txt(a.conan, env, index, subject, rel)
            else:
                r, errs = attest_recipe(a.conan, env, index, rel, ver) if kind == "py" else attest_pyfile(a.conan, env, index, subject, rel)
        except Exception as e:  # keep the run going; the file is simply not attested
            r, errs = None, ["exception: %r" % e]
        with lock:
            if r is None:
                note("%s: not attested (%s)" % (rel, "; ".join(errs)[:300]))
            else:
                results.append(r)
                if errs:
                    note("%s: attested on %d of 3 profiles (%s)" % (rel, 3 - len(errs), "; ".join(errs)[:200]))

    with ThreadPoolExecutor(a.jobs) as ex:
        list(ex.map(work, jobs))

    results.sort(key=lambda r: r["path"])
    o = {"tool": "conan graph info --format json -nr (macOS, Linux, Windows and mingw host+build profiles; the versions of a recipe folder at default options; each option at each of its other values, and every boolean all True and all False, at the newest and oldest version)",
         "version": version, "tier": "A", "definitions": [], "references": []}
    for r in results:
        if not r["name"]:
            note("%s: attested but conan reports no name" % r["path"])
            continue
        if r["literal"] and r["literal"] != r["name"]:
            note("%s: literal name %r differs from conan's %r" % (r["path"], r["literal"], r["name"]))
        o["definitions"].append({"path": r["path"], "name": r["name"], "line": r["line"], "kind": "package"})
        for d in sorted(r["names"]):
            o["references"].append({"path": r["path"], "from": r["name"], "to": d})
    print("attested %d of %d; closure exported %d recipe versions; not exportable: %s" % (
        len(results), len(jobs), len(exported), sorted(failed_exports)), file=sys.stderr)
    if a.files_out:
        def write_files():
            with open(a.files_out, "w") as f:
                f.write("\n".join(r["path"] for r in results if r["name"]) + "\n")
        retry_eperm(write_files)
    json.dump(o, sys.stdout, indent=1)
    print()


def attest_pyfile(conan, env, index, subject, rel):
    target = os.path.join(subject, rel)
    name_lit, line = recipe_name_line(target)
    names = set()
    root = None
    errs = []
    for pname, prof in PROFILES.items():
        node, why = evaluate(conan, env, index, target, None, prof)
        if node is None:
            errs.append("%s: %s" % (pname, why))
            continue
        root = root or node
        names |= direct_names(node)
    if root is None:
        return None, errs
    return {"path": rel, "name": root.get("name") or name_lit, "line": line, "names": names, "literal": name_lit}, errs


if __name__ == "__main__":
    main()
