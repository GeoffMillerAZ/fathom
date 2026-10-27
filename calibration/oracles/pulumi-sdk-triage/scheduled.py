#!/usr/bin/env python3
"""Launch one bench command at the lane's measured load/priority limits."""
import os
import sys

from runner import wait_load


if __name__ == "__main__":
    if len(sys.argv) < 2:
        raise SystemExit("usage: scheduled.py <command> [arguments]")
    load = wait_load()
    os.setpriority(os.PRIO_PROCESS, 0, max(10, os.getpriority(os.PRIO_PROCESS, 0)))
    priority = os.getpriority(os.PRIO_PROCESS, 0)
    if priority < 10:
        raise SystemExit("required nice 10 is not active")
    print("launch load:", load, "actual nice:", priority, file=sys.stderr, flush=True)
    os.execvp(sys.argv[1], sys.argv[1:])
