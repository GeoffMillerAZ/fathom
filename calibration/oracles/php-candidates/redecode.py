#!/usr/bin/env python3
"""Re-decode preserved native traces after tightening conservative source attribution."""
import argparse
import json
import os
from pathlib import Path
import subprocess

from fetch import ready
from trace import decode


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--tools', required=True, type=Path)
    ap.add_argument('--traces', required=True, type=Path)
    args = ap.parse_args()
    if os.getpriority(os.PRIO_PROCESS, 0) < 10:
        raise ValueError('run with nice -n 10')
    base = args.tools.resolve()
    for name in ['guzzle', 'monolog']:
        ready()
        out = args.traces/name
        original = json.loads((out/'syntax.json').read_text())
        result = subprocess.check_output([str(base/'runtime/php'), '-n',
            str(Path(__file__).with_name('literal_sites.php').resolve()),
            str(base/'callsite-fork/vendor/autoload.php'), str(base/name)])
        syntax = json.loads(result)
        if original['files'] != syntax['files'] or original['sites'] != syntax['sites']:
            raise ValueError('original literal population changed; cannot reuse old traces')
        old = json.loads((out/'summary.json').read_text())
        oracle, summary = decode(base/name, out, syntax, old['php'])
        for k in ['driver', 'driver_exit', 'php', 'xdebug', 'trace_format', 'trace_bytes', 'trace_complete', 'trace_cap_mib']:
            summary[k] = old[k]
        (out/'syntax.json').write_text(json.dumps(syntax, sort_keys=True)+'\n')
        (out/'oracle.json').write_text(json.dumps(oracle, indent=2, sort_keys=True)+'\n')
        (out/'summary.json').write_text(json.dumps(summary, indent=2, sort_keys=True)+'\n')
        print(name, summary['attested_sites'], '/', summary['literal_sites'], flush=True)


if __name__ == '__main__':
    main()
