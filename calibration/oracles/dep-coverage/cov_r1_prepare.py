#!/usr/bin/env python3
"""Prepare only DEP-M1 development pins in a fresh isolated R1 root, offline."""
import argparse
import json
import os
from pathlib import Path
import subprocess


def prepare(source, destination):
    pins = json.loads((Path(__file__).resolve().parent.parent / 'deps-tsjs/pins.json').read_text())['subjects']
    # Refuse replay in place. No campaign manifest or holdout pin is consulted.
    destination.mkdir(parents=True, exist_ok=False)
    for arm in ('primary', 'relocated'):
        for pin in pins:
            origin = source / arm / pin['name']
            target = destination / 'development' / arm / pin['name']
            target.parent.mkdir(parents=True, exist_ok=True)
            subprocess.run(['git', 'clone', '--quiet', '--shared', '--no-checkout', str(origin.resolve(strict=True)), str(target)], check=True)
            subprocess.run(['git', '-C', str(target), 'remote', 'set-url', 'origin', pin['url']], check=True)
            subprocess.run(['git', '-C', str(target), 'checkout', '--quiet', '--detach', pin['commit']], check=True)
    (destination / 'R1-DEVELOPMENT-ONLY').write_text('DEP-M1 development pins only\n')
    print('R1 isolated development checkouts prepared')


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('source', type=Path, help='frozen development directory with primary and relocated arms')
    parser.add_argument('destination', type=Path, help='new isolated development root')
    args = parser.parse_args()
    os.environ.update(GIT_CONFIG_COUNT='2', GIT_CONFIG_KEY_0='core.hooksPath', GIT_CONFIG_VALUE_0='/dev/null',
                      GIT_CONFIG_KEY_1='init.templateDir', GIT_CONFIG_VALUE_1='/dev/null', GIT_TERMINAL_PROMPT='0')
    prepare(args.source, args.destination)
