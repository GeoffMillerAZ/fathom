"""Read native plugin variable names; never writes calibration subjects or values."""
import argparse
import json
import os
import subprocess
import time
from pathlib import Path


def registry(binary, plugin, client, work):
    work = work.resolve()
    work.mkdir(parents=True, exist_ok=True)
    config = work / 'registry.cfg'
    socket = work / 'admin.sock'
    config.write_text('plugins=("' + str(plugin.resolve()) + '")\n'
                      'admin_variables={admin_credentials="bench:bench";mysql_ifaces="' + str(socket) + '";}\n'
                      'mysql_variables={threads=1;interfaces="' + str(work / 'mysql.sock') + '";monitor_enabled=false;}\n')
    args = [str(client), '--protocol=SOCKET', '--socket=' + str(socket), '-u', 'bench', '-N', '-s', '--batch']
    env = dict(os.environ, MYSQL_PWD='bench')
    with (work / 'native.log').open('w') as log:
        process = subprocess.Popen([str(binary.resolve()), '--initial', '-f', '-D', str(work), '-c', str(config)], stdout=log, stderr=log)
        try:
            deadline = time.monotonic() + 60
            while True:
                ready = subprocess.run(args + ['-e', 'SELECT 1'], env=env, capture_output=True, text=True)
                if ready.returncode == 0:
                    break
                if process.poll() is not None or time.monotonic() > deadline:
                    raise RuntimeError('native registry unavailable; inspect scratch native.log')
                time.sleep(.2)
            result = {}
            for prefix in ('mcp', 'genai'):
                sql = 'SAVE ' + prefix.upper() + " VARIABLES FROM RUNTIME; SELECT variable_name FROM global_variables WHERE variable_name LIKE '" + prefix + "-%' ORDER BY variable_name"
                output = subprocess.run(args + ['-e', sql], env=env, capture_output=True, text=True, check=True)
                result[prefix] = output.stdout.splitlines()
            return result
        finally:
            if process.poll() is None:
                process.terminate()
            try:
                process.wait(timeout=20)
            except subprocess.TimeoutExpired:
                process.kill()
                process.wait()


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    for arg in ('binary', 'plugin', 'client', 'work', 'out'):
        parser.add_argument('-' + arg, type=Path, required=True)
    args = parser.parse_args()
    names = registry(args.binary, args.plugin, args.client, args.work)
    args.out.write_text(json.dumps(names, indent=2) + '\n')
    print('native registry names', {k: len(v) for k, v in names.items()})
