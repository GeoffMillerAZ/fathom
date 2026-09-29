#!/usr/bin/env python3
"""Drop the bench instrument's own file from a cell's agent.diff (stdin to stdout).

The scenariobench driver writes .claude/scenariobench-mcp.json into the cell
worktree before the session; it lands in agent.diff without a trailing newline
marker, so `git apply` rejects the whole patch as corrupt. It is instrument
wiring, never the agent's change, so it is removed before the replay applies
the diff. Every other file block passes through byte for byte.
"""
import sys

INSTRUMENT = (".claude/scenariobench-mcp.json",)

data = sys.stdin.read()
blocks, cur = [], []
for line in data.splitlines(keepends=True):
    if line.startswith("diff --git ") and cur:
        blocks.append("".join(cur))
        cur = []
    cur.append(line)
if cur:
    blocks.append("".join(cur))

out = []
for b in blocks:
    head = b.split("\n", 1)[0]
    if any(head.endswith(" b/" + p) for p in INSTRUMENT):
        continue
    out.append(b)
sys.stdout.write("".join(out))
