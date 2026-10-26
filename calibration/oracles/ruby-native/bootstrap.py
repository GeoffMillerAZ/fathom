#!/usr/bin/env python3
"""Install pinned Ruby-owner oracle tools and complete original subjects locally."""
import argparse
import hashlib
import io
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tarfile
import time

SUBJECTS = {
    'sinatra': ('sinatra/sinatra', '599a007a779dc9940e49f34e9077220f4c209f4b'),
    'jekyll': ('jekyll/jekyll', '1f319fb273b6cdf876bc6edd38d7477935cdda8c'),
}
GEMS = {
    'prism': ('2.0.0', 'fd551446f0c6ca71455ef8d1d40bd2ca1538044834fa22df8837e5095f3a5a39'),
    'rbs': ('4.2.0', '51f7b886dcc05bc09e10b901daa6a81829f6adc03101d6ca9ea4aac6103e0674'),
    'typeprof': ('0.33.2', 'aa777c684d578ed27dc3e5d4bc4104c9eb5a76eedf7f5dd3366d571eaa6ca7c3'),
}
RUBY_SHA = '7645e2d653a335798030f6502e7834dfdbeeec5629429a1a34da5dbb2c57d63e'


def ready():
    while True:
        subprocess.run(['df', '-h', str(Path.home())], check=True)
        if shutil.disk_usage(Path.home()).free < 20*(1 << 30):
            raise RuntimeError('owner disk floor: need at least 20 GiB free')
        if os.getloadavg()[0] <= 6:
            return
        print('waiting for load <=6', flush=True)
        time.sleep(15)


def download(url, expected, target):
    ready()
    data = subprocess.check_output(['curl', '-fsSL', '--retry', '2', url])
    if hashlib.sha256(data).hexdigest() != expected:
        raise ValueError('pinned download checksum mismatch')
    target.write_bytes(data)
    return data


def environment(base, ruby):
    env = os.environ.copy()
    env.pop('RUBYOPT', None)
    env.pop('RUBYLIB', None)
    env.pop('GEM_HOME', None)
    env.pop('GEM_PATH', None)
    default = subprocess.check_output([str(ruby), '-e', 'puts Gem.default_dir'], env=env, text=True).strip()
    env.update(GEM_HOME=str(base/'gems'), GEM_PATH=str(base/'gems')+os.pathsep+default,
               GEM_SPEC_CACHE=str(base/'gem-spec-cache'), BUNDLE_USER_HOME=str(base/'bundle-user'),
               BUNDLE_PATH=str(base/'bundle'), BUNDLE_APP_CONFIG=str(base/'bundle-config'),
               PATH=str(ruby.parent)+os.pathsep+env.get('PATH', ''))
    return env


def main():
    if sys.version_info < (3, 13):
        raise SystemExit('Python 3.13+ required before creating output')
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--out', type=Path, required=True)
    ap.add_argument('--only', action='append', choices=['tools', *SUBJECTS])
    args = ap.parse_args()
    if os.getpriority(os.PRIO_PROCESS, 0) < 10:
        raise SystemExit('run with nice -n 10')
    base = args.out.resolve()
    base.mkdir(parents=True, exist_ok=True)
    receipt_path = base/'bootstrap.json'
    receipt = json.loads(receipt_path.read_text()) if receipt_path.exists() else {}

    def save(key, value):
        receipt[key] = value
        receipt_path.write_text(json.dumps(receipt, indent=2, sort_keys=True)+'\n')

    if not args.only or 'tools' in args.only:
        runtime = base/'runtime'
        if not runtime.exists():
            archive = base/'portable-ruby.tar.gz'
            data = download('https://github.com/Homebrew/homebrew-portable-ruby/releases/download/3.3.8/portable-ruby-3.3.8.arm64_big_sur.bottle.tar.gz', RUBY_SHA, archive)
            runtime.mkdir()
            with tarfile.open(fileobj=io.BytesIO(data), mode='r:gz') as source:
                source.extractall(runtime, filter='data')
        ruby = next(runtime.rglob('bin/ruby'))
        assert subprocess.check_output([str(ruby), '-e', 'print RUBY_VERSION'], text=True) == '3.3.8'
        env = environment(base, ruby)
        (base/'downloads').mkdir(exist_ok=True)
        (base/'logs').mkdir(exist_ok=True)
        save('ruby', {'version': '3.3.8', 'archive_sha256': RUBY_SHA,
                      'binary': ruby.relative_to(base).as_posix(),
                      'checksum_source': 'Homebrew/brew tag 4.4.30 Library/Homebrew/vendor/portable-ruby-arm64-darwin'})
        for name, (version, digest) in GEMS.items():
            if receipt.get(name, {}).get('version') == version:
                continue
            path = base/'downloads'/(name+'-'+version+'.gem')
            download('https://rubygems.org/downloads/'+path.name, digest, path)
            ready()
            print('installing', name, version, flush=True)
            with (base/'logs'/(name+'-install.log')).open('w') as log:
                subprocess.run([str(ruby), str(ruby.parent/'gem'), 'install', '--local', str(path),
                                '--no-document', '--ignore-dependencies'], env=env, check=True,
                               stdout=log, stderr=subprocess.STDOUT)
            save(name, {'version': version, 'sha256': digest, 'platform': 'ruby'})
        ready()
        smoke = "require 'typeprof'; require 'json'; abort unless TypeProf::VERSION == '0.33.2'; puts JSON.generate(Gem.loaded_specs.transform_values { |s| s.version.to_s })"
        result = json.loads(subprocess.check_output([str(ruby), '-e', smoke], env=env))
        save('loaded_gems', result)
        print('owner tool smoke', result, flush=True)
    for name, (repo, pin) in SUBJECTS.items():
        if args.only and name not in args.only:
            continue
        if receipt.get(name, {}).get('revision') == pin:
            continue
        ready()
        target = base/name
        subprocess.run(['git', 'init', '--quiet', str(target)], check=True)
        subprocess.run(['git', '-C', str(target), 'remote', 'add', 'origin', 'https://github.com/'+repo], check=True)
        subprocess.run(['git', '-C', str(target), 'fetch', '--quiet', '--depth=1', 'origin', pin], check=True)
        subprocess.run(['git', '-C', str(target), 'checkout', '--quiet', '--detach', 'FETCH_HEAD'], check=True)
        save(name, {'revision': pin, 'mode': 'complete-git-checkout', 'repository': repo})
        print('fetched', name, flush=True)


if __name__ == '__main__':
    main()
