#!/usr/bin/env python3
"""Terminate only a process group whose live leader is this supervisor."""
import argparse,json,os
from pathlib import Path
import signal,subprocess


def main():
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument('--status',type=Path,required=True)
    p.add_argument('--timeout',type=float,required=True)
    p.add_argument('command',nargs=argparse.REMAINDER)
    args=p.parse_args()
    if os.getpgrp()!=os.getpid():raise SystemExit('requires an owned new-session group leader')
    command=args.command[1:] if args.command[:1]==['--'] else args.command
    if not command or args.status.exists():raise SystemExit('fresh status and command required')
    proc=subprocess.Popen(command)
    timed=False
    try:code=proc.wait(timeout=args.timeout)
    except subprocess.TimeoutExpired:timed=True;code=None
    status={'driver_exit':code,'timeout':timed,'supervisor_group_leader':os.getpid(),
            'cleanup':'self-owned live process group, including supervisor; no stale group identifier'}
    temporary=args.status.with_suffix('.pending')
    temporary.write_text(json.dumps(status,sort_keys=True)+'\n')
    temporary.replace(args.status)
    # This process is still the live leader: the identifier cannot have been
    # recycled to another job. The supervisor dies with its entire group.
    os.killpg(os.getpgrp(),signal.SIGKILL)


if __name__=='__main__':main()
