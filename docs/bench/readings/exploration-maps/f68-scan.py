#!/usr/bin/env python3
"""F68 baseline scan (XM-M10): how often one upstream repository is researched.

Usage: f68-scan.py TRANSCRIPT_ROOT [--days 30] [--now ISO8601] [--names-out FILE]

Read-only over TRANSCRIPT_ROOT (Claude Code's projects directory). Counts, per
session, Bash `git clone` and `gh repo clone` commands and WebFetch calls to
github.com URLs, groups them by normalized upstream host/owner/repo, and folds
each session to its working repository (clones and worktrees to their origin).
Prints AGGREGATE COUNTS as JSON on stdout. It never prints an upstream name,
URL, path, session id or prompt. --names-out writes the upstream keys with
their counts to a file the caller keeps out of the tree (gitignored scratch).
$0: no network, no model.
"""
import argparse, collections, datetime, json, os, re, shlex, subprocess, sys

CLONE_RE = re.compile(r"(?:\bgit\s+(?:-C\s+\S+\s+)?clone|\bgh\s+repo\s+clone)\s+([^\n;&|]*)")
GH_NOT_REPO = {"orgs", "settings", "search", "topics", "marketplace", "features", "sponsors", "apps",
               "login", "notifications", "pulls", "issues", "explore", "about", "pricing", "enterprise",
               "collections", "trending", "readme", "security", "customer-stories", "new", "codespaces",
               "users", "organizations", "site", "contact", "team", "events", "resources", "solutions"}
CLONE_ARG_OPTS = {"-b", "--branch", "--depth", "-c", "--config", "-o", "--origin", "--reference",
                  "--reference-if-able", "--template", "-j", "--jobs", "--filter", "--separate-git-dir",
                  "-u", "--upload-pack", "--server-option", "--shallow-since", "--shallow-exclude",
                  "--recurse-submodules", "--ref-format", "--bundle-uri"}


def norm_url(s):
    """Return host/owner/repo for a remote URL or owner/repo shorthand, else None (a local path)."""
    s = s.strip().strip("'\"")
    if not s:
        return None
    m = re.match(r"^(?:[a-z+]+://)?(?:[^@/]+@)?([A-Za-z0-9.-]+\.[A-Za-z]{2,})[:/]+([^/\s]+)/([^/\s?#]+)", s)
    if m:
        host, owner, repo = m.group(1).lower(), m.group(2), m.group(3)
    elif re.match(r"^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$", s) and not s.startswith((".", "/", "~")):
        host, owner, repo = "github.com", *s.split("/")
    else:
        return None
    repo = re.sub(r"\.git$", "", repo)
    if not owner or not repo or owner.lower() in GH_NOT_REPO and host == "github.com":
        return None
    return "%s/%s/%s" % (host, owner.lower(), repo.lower())


def clone_targets(cmd):
    out = []
    for m in CLONE_RE.finditer(cmd):
        rest = m.group(1)
        try:
            toks = shlex.split(rest)
        except ValueError:
            toks = rest.split()
        i = 0
        while i < len(toks):
            t = toks[i]
            if t.startswith("-"):
                if t in CLONE_ARG_OPTS and "=" not in t:
                    i += 1
                i += 1
                continue
            out.append(norm_url(t))
            break
    return out


def fetch_target(url):
    m = re.match(r"^https?://(?:www\.)?github\.com/([^/\s?#]+)/([^/\s?#]+)", url or "", re.I)
    if not m or m.group(1).lower() in GH_NOT_REPO:
        return None
    return norm_url("github.com/%s/%s" % (m.group(1), m.group(2)))


_origin_cache = {}


def repo_key(cwd):
    """Fold a working directory to its repository: origin host/owner/repo, else the repo's common root."""
    if not cwd:
        return "unresolved"
    if cwd in _origin_cache:
        return _origin_cache[cwd]
    d = cwd
    while d and not os.path.isdir(d) and d != os.path.dirname(d):
        d = os.path.dirname(d)
    key = None
    try:
        r = subprocess.run(["git", "-C", d, "rev-parse", "--path-format=absolute", "--git-common-dir"],
                           capture_output=True, text=True, timeout=10)
        if r.returncode == 0 and r.stdout.strip():
            common = r.stdout.strip()
            u = subprocess.run(["git", "--git-dir", common, "config", "--get", "remote.origin.url"],
                               capture_output=True, text=True, timeout=10).stdout.strip()
            key = norm_url(u) or "local:" + os.path.dirname(common)
    except Exception:
        pass
    if key is None:
        key = "unresolved"
    _origin_cache[cwd] = key
    return key


def bucket(n, edges):
    for e in edges:
        if n <= e:
            return str(e) if e == edges[0] else "%d-%d" % (prev(edges, e) + 1, e) if prev(edges, e) + 1 != e else str(e)
    return "%d+" % (edges[-1] + 1)


def prev(edges, e):
    i = edges.index(e)
    return edges[i - 1] if i else 0


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("root")
    ap.add_argument("--days", type=int, default=30)
    ap.add_argument("--now", default="")
    ap.add_argument("--names-out", default="")
    a = ap.parse_args()
    now = datetime.datetime.fromisoformat(a.now.replace("Z", "+00:00")) if a.now else datetime.datetime.now(datetime.timezone.utc)
    if now.tzinfo is None:
        now = now.replace(tzinfo=datetime.timezone.utc)
    cutoff = now - datetime.timedelta(days=a.days)
    cut_ts = cutoff.timestamp()

    # session id -> {files, cwd}
    sessions = {}
    for proj in sorted(os.listdir(a.root)):
        pdir = os.path.join(a.root, proj)
        if not os.path.isdir(pdir):
            continue
        for dp, dn, fn in os.walk(pdir):
            for f in fn:
                if not f.endswith(".jsonl"):
                    continue
                p = os.path.join(dp, f)
                rel = os.path.relpath(p, pdir).split(os.sep)
                sid = rel[0][:-6] if len(rel) == 1 else rel[0]
                try:
                    if os.path.getmtime(p) < cut_ts:
                        continue
                except OSError:
                    continue
                s = sessions.setdefault((proj, sid), {"files": [], "cwd": None})
                s["files"].append(p)

    calls = collections.Counter()
    file_found = {}   # path -> list of upstream keys
    file_cwd = {}     # path -> first cwd seen
    for sk, s in sorted(sessions.items()):
        for p in s["files"]:
            found, cwd = [], None
            try:
                fh = open(p, encoding="utf-8", errors="replace")
            except OSError:
                continue
            with fh:
                for n, line in enumerate(fh):
                    if cwd is None and '"cwd"' in line and n <= 400:
                        m = re.search(r'"cwd":"([^"]+)"', line)
                        if m:
                            cwd = m.group(1)
                    if "clone" not in line and "WebFetch" not in line:
                        continue
                    try:
                        o = json.loads(line)
                    except ValueError:
                        continue
                    if o.get("type") != "assistant":
                        continue
                    try:
                        if datetime.datetime.fromisoformat(o.get("timestamp", "").replace("Z", "+00:00")) < cutoff:
                            continue
                    except ValueError:
                        pass
                    c = (o.get("message") or {}).get("content")
                    if not isinstance(c, list):
                        continue
                    for b in c:
                        if not isinstance(b, dict) or b.get("type") != "tool_use":
                            continue
                        inp = b.get("input") or {}
                        if b.get("name") == "Bash":
                            for t in clone_targets(inp.get("command") or ""):
                                calls["clone"] += 1
                                if t:
                                    found.append(t)
                                else:
                                    calls["clone_local"] += 1
                        elif b.get("name") == "WebFetch":
                            t = fetch_target(inp.get("url") or "")
                            if t:
                                calls["webfetch"] += 1
                                found.append(t)
            file_found[p], file_cwd[p] = found, cwd

    # Fold: a resolved cwd gives its origin; an unresolved one (a deleted clone or scratch
    # directory) folds through a mangled-project path component ("-a-b-c", Claude Code's
    # naming of a project directory) to the resolved repository whose path mangles to it.
    mangled = {}
    for cwd in set(file_cwd.values()):
        k = repo_key(cwd)
        if k != "unresolved":
            d = cwd
            while d and not os.path.isdir(d) and d != os.path.dirname(d):
                d = os.path.dirname(d)
            try:
                top = subprocess.run(["git", "-C", d, "rev-parse", "--show-toplevel"], capture_output=True, text=True, timeout=10).stdout.strip()
            except Exception:
                top = ""
            if top:
                mangled["-" + top.strip("/").replace("/", "-").replace(".", "-").replace("_", "-")] = k
    def fold(cwd):
        k = repo_key(cwd)
        if k != "unresolved":
            return k
        for comp in (cwd or "").split("/"):
            if comp.startswith("-") and comp in mangled:
                return mangled[comp]
        return "unresolved"

    sess_repo = {}
    for sk, s in sessions.items():
        cw = [file_cwd.get(p) for p in sorted(s["files"], key=lambda p: (len(p), p)) if file_cwd.get(p)]
        sess_repo[sk] = fold(cw[0]) if cw else "unresolved"
    file_repo = {p: fold(c) for p, c in file_cwd.items()}

    def scheme(units):
        """units: {unit: (repo, [upstream keys])} -> (summary, upstream -> {unit: repo})."""
        def run(excl):
            ups = collections.defaultdict(dict)
            for u, (repo, found) in units.items():
                for t in found:
                    if t != repo and t not in excl:
                        ups[t][u] = repo
            spu, rpu, met = collections.Counter(), collections.Counter(), 0
            for up, per in ups.items():
                ns, nr = len(per), len(set(per.values()))
                spu[bucket(ns, [1, 2, 3, 5, 10])] += 1
                rpu[bucket(nr, [1, 2, 3])] += 1
                met += ns >= 3 and nr >= 2
            return {"upstreams_seen": len(ups),
                    "units_per_upstream": dict(sorted(spu.items())),
                    "repositories_per_upstream": dict(sorted(rpu.items())),
                    "upstreams_meeting_trigger": met}, ups
        own = {r for r, _ in units.values()}
        base, ups = run(set())
        strict, _ = run(own)
        withhit = len({u for per in ups.values() for u in per})
        return {"units": len(units), "units_with_a_hit": withhit, "primary": base,
                "also_excluding_any_session_repository": strict}, ups

    by_session = {sk: (sess_repo[sk], [t for p in s["files"] for t in file_found.get(p, [])]) for sk, s in sessions.items()}
    by_file = {p: (file_repo[p], file_found[p]) for p in file_found}
    res_s, ups = scheme(by_session)
    res_f, ups_f = scheme(by_file)
    repos = collections.Counter(sess_repo.values())
    res = {
        "window_days": a.days,
        "sessions_scanned": len(sessions),
        "transcript_files_scanned": len(file_found),
        "repositories_seen": len(repos),
        "sessions_with_unresolved_repository": repos.get("unresolved", 0),
        "clone_commands": calls["clone"], "clone_commands_from_a_local_path": calls["clone_local"],
        "webfetch_github_repo_calls": calls["webfetch"],
        "a_clone_of_the_sessions_own_repository": "not counted as research",
        "trigger": "an upstream in >= 3 sessions and >= 2 repositories within the window",
        "by_session": res_s,
        "by_transcript_file_each_agent_a_unit": res_f,
        "verdict": "MET" if res_s["primary"]["upstreams_meeting_trigger"] > 0 else "NOT MET",
    }
    if a.names_out:
        with open(a.names_out, "w") as fh:
            for up, per in sorted(ups.items(), key=lambda kv: -len(kv[1])):
                fh.write("%d sessions %d repos\t%s\n" % (len(per), len({sess_repo[sk] for sk in per}), up))
    json.dump(res, sys.stdout, indent=2, sort_keys=True)
    print()


if __name__ == "__main__":
    main()
