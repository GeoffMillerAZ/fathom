#!/usr/bin/env python3
"""Build Xdebug 3.5.3 against a matching pinned PHP SDK inside workspace."""
import argparse
import hashlib
import io
import json
import os
from pathlib import Path
import shutil
import subprocess
import tarfile
import sys

from fetch import download, ready


def main():
    if sys.version_info < (3, 13):
        raise SystemExit('Python 3.13+ required before creating any output')
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--tools', required=True, type=Path)
    args = ap.parse_args()
    base = args.tools.resolve()
    php_version = subprocess.check_output([str(base/'runtime/php'), '-n', '-r', 'echo PHP_VERSION;'], text=True)
    php_sources = {'8.5.8': '58910198d19e873048fe87cdfe16bc790025417ede3d1651bfa1c4b533d573f2',
                   '8.4.23': '1ab9f52008414e43bb2427ffa288eff2a4de39e1a830f957e800ba368d887a72'}
    if php_version not in php_sources:
        raise ValueError('unsupported SDK pin')
    sources = base/'build-sources'
    sources.mkdir(exist_ok=True)
    logs = base/'logs'
    logs.mkdir(exist_ok=True)
    receipt_path = base/'xdebug-build-receipt.json'
    receipt = json.loads(receipt_path.read_text()) if receipt_path.exists() else {}

    def source(name, url, expected=None):
        root = sources/name
        if root.is_dir():
            return root
        content = download(url)
        digest = hashlib.sha256(content).hexdigest()
        if expected and expected != digest:
            raise ValueError(name+' source checksum mismatch')
        with tarfile.open(fileobj=io.BytesIO(content), mode='r:*') as archive:
            archive.extractall(sources, filter='data')
        receipt[name] = {'source_sha256': digest}
        receipt_path.write_text(json.dumps(receipt, indent=2, sort_keys=True)+'\n')
        return root

    env = os.environ.copy()
    autoconf_prefix = base/'sdk/autoconf'
    php_prefix = base/'sdk/php'
    env['PATH'] = str(autoconf_prefix/'bin')+os.pathsep+str(base/'runtime')+os.pathsep+env['PATH']
    env['PHP_AUTOCONF'] = str(autoconf_prefix/'bin/autoconf')
    env['PHP_AUTOHEADER'] = str(autoconf_prefix/'bin/autoheader')
    env['PHP_INI_SCAN_DIR'] = ''

    def run(name, command, cwd):
        ready()
        print(name, flush=True)
        with (logs/(name+'.log')).open('w') as output:
            code = subprocess.run([str(x) for x in command], cwd=cwd, env=env,
                                  stdout=output, stderr=subprocess.STDOUT).returncode
        if code:
            raise ValueError(name+' failed; inspect workspace log')

    if not (autoconf_prefix/'bin/m4').is_file():
        root = source('m4-1.4.20', 'https://ftp.gnu.org/gnu/m4/m4-1.4.20.tar.xz',
                      'e236ea3a1ccf5f6c270b1c4bb60726f371fa49459a8eaaebc90b216b328daf2b')
        run('m4-configure', ['./configure', '--prefix='+str(autoconf_prefix)], root)
        run('m4-make', ['make', '-j2'], root)
        run('m4-install', ['make', 'install'], root)
    if not (autoconf_prefix/'bin/autoconf').is_file():
        root = source('autoconf-2.72', 'https://ftp.gnu.org/gnu/autoconf/autoconf-2.72.tar.xz',
                      'ba885c1319578d6c94d46e9b0dceb4014caafe2490e437a0dbca3f270a223f5a')
        run('autoconf-configure', ['./configure', '--prefix='+str(autoconf_prefix)], root)
        run('autoconf-make', ['make', '-j2'], root)
        run('autoconf-install', ['make', 'install'], root)
    if not (php_prefix/'bin/php-config').is_file():
        root = source('php-'+php_version, 'https://www.php.net/distributions/php-'+php_version+'.tar.xz',
                      php_sources[php_version])
        run('php-sdk-configure', ['./configure', '--prefix='+str(php_prefix), '--disable-all',
                                  '--disable-cgi', '--disable-phpdbg', '--enable-cli'], root)
        run('php-sdk-headers', ['make', 'install-headers', 'install-build'], root)
        (php_prefix/'bin').mkdir(exist_ok=True)
        for script in ['phpize', 'php-config']:
            shutil.copy2(root/'scripts'/script, php_prefix/'bin'/script)
            (php_prefix/'bin'/script).chmod(0o755)
        (php_prefix/'bin/php').symlink_to(os.path.relpath(base/'runtime/php', php_prefix/'bin'))
    root = source('xdebug-3.5.3', 'https://xdebug.org/files/xdebug-3.5.3.tgz',
                  'f073de91bea046106abf4d6071c963ea71e58571df6ce58948ceca89d121cb2d')
    run('xdebug-phpize', [php_prefix/'bin/phpize'], root)
    run('xdebug-configure', ['./configure', '--with-php-config='+str(php_prefix/'bin/php-config')], root)
    run('xdebug-make', ['make', '-j2'], root)
    extension = root/'modules/xdebug.so'
    run('xdebug-load', [base/'runtime/php', '-n', '-d', 'zend_extension='+str(extension),
                        '-r', 'if (phpversion("xdebug") !== "3.5.3") { exit(1); } echo phpversion("xdebug"), PHP_EOL;'], base)
    receipt['extension_sha256'] = hashlib.sha256(extension.read_bytes()).hexdigest()
    receipt['loaded_version'] = '3.5.3'
    receipt['php_version'] = php_version
    receipt_path.write_text(json.dumps(receipt, indent=2, sort_keys=True)+'\n')


if __name__ == '__main__':
    main()
