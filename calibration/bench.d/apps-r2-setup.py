"""Unprivileged GoReleaser tool setup from the round's content-pinned downloads."""
import sys
sys.dont_write_bytecode = True
import hashlib
import os
import json
from pathlib import Path
import shutil
import subprocess
import tarfile
import urllib.request
import time
import zipfile


def install(pins, root=Path('/work/tools')):
    root.mkdir(parents=True, exist_ok=True)
    bindir = Path('/work/bin')
    bindir.mkdir(exist_ok=True)
    for pin in pins:
        name = pin['name']
        executable = root / name / (pin['member'] or name)
        if pin.get('executable_sha256') and executable.is_file() and hashlib.sha256(executable.read_bytes()).hexdigest() == pin['executable_sha256']:
            print('TOOL_REUSE_SHA256', name, pin['version'], pin['sha256'], pin['executable_sha256'], flush=True)
            continue
        print('TOOL_FETCH', name, pin['version'], pin['sha256'], flush=True)
        download = root / (name + '.download')
        if not download.exists():
            for attempt in range(3):
                try:
                    with urllib.request.urlopen(pin['url'], timeout=120) as response, download.open('wb') as out:
                        shutil.copyfileobj(response, out)
                    break
                except OSError:
                    download.unlink(missing_ok=True)
                    if attempt == 2:
                        raise
                    time.sleep(2)
        assert hashlib.sha256(download.read_bytes()).hexdigest() == pin['sha256'], name
        if pin['kind'] == 'rustup-component':
            cache = Path('/work/rustup/downloads')
            cache.mkdir(parents=True, exist_ok=True)
            download.replace(cache / pin['sha256'])
            print('TOOL_SHA256', name, pin['version'], pin['sha256'], flush=True)
            continue
        if pin['kind'] == 'prefetch':
            print('TOOL_SHA256', name, pin['version'], pin['sha256'], flush=True)
            continue  # Subject has no cache lookup; retain the verified preparation input.
        target = root / name
        target.mkdir(exist_ok=True)
        kind = pin['kind']
        if kind in {'tar', 'tree'}:
            with tarfile.open(download) as archive:
                for member in archive.getmembers():
                    assert not member.name.startswith('/') and '..' not in Path(member.name).parts
                archive.extractall(target)
        elif kind == 'zip':
            with zipfile.ZipFile(download) as archive:
                archive.extractall(target)
        elif kind == 'deb':
            subprocess.run(['dpkg-deb', '-x', str(download), str(target)], check=True)
        else:
            shutil.copyfile(download, target / name)
        executable = target / (pin['member'] or name)
        executable.chmod(0o755)
        if name == 'nixfmt' and executable.read_bytes()[18:20] != b'\xb7\x00':
            print('TOOL_UNAVAILABLE nixfmt: release binary is not aarch64; native GHC build exceeds setup/disk scope', flush=True)
            download.unlink()
            continue
        link = bindir / name
        link.unlink(missing_ok=True)
        link.symlink_to(executable)
        print('TOOL_SHA256', name, pin['version'], pin['sha256'], flush=True)
        download.unlink()
    env = dict(os.environ, CARGO_HOME='/work/cargo', RUSTUP_HOME='/work/rustup')
    subprocess.run([str(bindir / 'rustup-init'), '-y', '--no-modify-path', '--profile', 'minimal', '--default-toolchain', '1.90.0', '--target', 'armv7-unknown-linux-gnueabihf'], env=env, check=True)
    # Debian's script hardcodes /usr/share; a bin-directory symlink is insufficient.
    wrapper = bindir / 'makeself'
    wrapper.unlink(missing_ok=True)
    wrapper.write_text('#!/bin/sh\nexec /work/tools/makeself/usr/bin/makeself --header /work/tools/makeself/usr/share/makeself/makeself-header.sh "$@"\n')
    wrapper.chmod(0o755)
    # These tests need a service/sandbox, beyond merely finding a CLI. Probe
    # the unprivileged namespace prerequisite without changing the runtime.
    probe = subprocess.run(['unshare', '-Ur', 'true'], capture_output=True, text=True)
    print('SANDBOX_PROBE', probe.returncode, probe.stderr.strip(), flush=True)
    print('SETUP_UNAVAILABLE docker: no daemon/socket; nested privileged service forbidden', flush=True)
    print('SETUP_UNAVAILABLE flatpak-builder: needs native and cross SDK sandboxes; namespace probe above', flush=True)
    print('SETUP_UNAVAILABLE snapcraft: upstream setup requires privileged snapd and classic snap installation', flush=True)


if __name__ == '__main__':
    install(json.loads(Path('/bench-inputs/apps-r2-tools.json').read_text()))
