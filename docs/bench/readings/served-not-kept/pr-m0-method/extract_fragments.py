#!/usr/bin/env python3
"""PR-M0 part B input: every Edit, Write and MultiEdit fragment in the cell transcripts.

Reads $PRM0_RAW/guards-fathom-r{1,2}/<cell>/attempt-N/session-*.stream.jsonl and
writes one JSON line per proposed file edit to stdout:

  {"run","cell","task","arm","trial","attempt","idx","tool","path","new","old"}

new/old follow the PreToolUse hook's own fragment rule
(internal/harness/codec.go claudeProposedNewContent/OldContent): Write's whole
content with old = "", Edit's new_string/old_string, MultiEdit's
edits[].new_string and edits[].old_string each joined with "\n". A proposal is
counted whether or not the tool call then succeeded, because a PreToolUse check
sees it before it runs. path is made repo-relative by cutting everything up to
the cell worktree directory (".../<subject>-worktrees/<id>/").
"""
import json
import os
import re
import sys

RAW = os.environ["PRM0_RAW"]
RUNS = ("guards-fathom-r1", "guards-fathom-r2")
CELL_RE = re.compile(r"^guards-(i\d+)-.+-(control|full)-t(\d+)$")
WT_RE = re.compile(r"^.*-worktrees/[^/]+/(.*)$")


def rel(p):
    m = WT_RE.match(p or "")
    return m.group(1) if m else (p or "")


def fragment(name, ti):
    if name == "Write":
        return ti.get("content", ""), ""
    if name == "MultiEdit":
        edits = ti.get("edits") or []
        return ("\n".join(e.get("new_string", "") for e in edits),
                "\n".join(e.get("old_string", "") for e in edits))
    return ti.get("new_string", ""), ti.get("old_string", "")


for run in RUNS:
    root = os.path.join(RAW, run)
    for cell in sorted(os.listdir(root)):
        m = CELL_RE.match(cell)
        if not m:
            continue
        task_short, arm, trial = m.group(1), m.group(2), int(m.group(3))
        task = cell[: cell.rfind("-" + arm + "-t")]
        for att in sorted(os.listdir(os.path.join(root, cell))):
            adir = os.path.join(root, cell, att)
            idx = 0
            for fn in sorted(os.listdir(adir)):
                if not fn.endswith(".stream.jsonl"):
                    continue
                with open(os.path.join(adir, fn), encoding="utf-8") as f:
                    for line in f:
                        try:
                            ev = json.loads(line)
                        except ValueError:
                            continue
                        if ev.get("type") != "assistant":
                            continue
                        for c in (ev.get("message") or {}).get("content") or []:
                            if not isinstance(c, dict) or c.get("type") != "tool_use":
                                continue
                            name = c.get("name")
                            if name not in ("Edit", "Write", "MultiEdit"):
                                continue
                            ti = c.get("input") or {}
                            new, old = fragment(name, ti)
                            idx += 1
                            print(json.dumps({
                                "run": run, "cell": cell, "task": task, "task_short": task_short,
                                "arm": arm, "trial": trial, "attempt": att, "idx": idx,
                                "tool": name, "path": rel(ti.get("file_path", "")),
                                "new": new, "old": old,
                            }, sort_keys=True))
