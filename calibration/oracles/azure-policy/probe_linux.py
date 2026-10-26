"""Pinned, credential-free Linux x64 startup/API probe. No calibration claim.

Preparation may use the network. Execution requires a fresh network namespace;
only its loopback device is enabled. Refusals produce receipts, never fallback.
"""
import argparse
import copy
import hashlib
import http.client
import json
import os
import platform
import queue
import resource
import shutil
import signal
import socket
import struct
import subprocess
import sys
import threading
import time
from pathlib import Path

from audit_extension import HERE
from prepare_native import prepare

LIMIT = 8 * 1024**2
TIMEOUT = 180
PIN = HERE / 'native-linux-payload-r1.json'
INPUT = HERE / 'probe-linux-inputs-r1.json'


def digest(value):
    return hashlib.sha256(value).hexdigest()


def payload_identity(payload):
    manifest, size = [], 0
    for path in sorted(payload.rglob('*'), key=lambda path: path.relative_to(payload).as_posix()):
        if path.is_symlink():
            raise ValueError('payload symlink refused')
        if not path.is_file():
            continue
        if len(manifest) >= 512 or path.stat().st_size > 64 * 1024**2:
            raise ValueError('payload count/size cap exceeded')
        value = path.read_bytes()
        size += len(value)
        if size > 160 * 1024**2:
            raise ValueError('payload total size cap exceeded')
        manifest.append([path.relative_to(payload).as_posix(), digest(value)])
    raw = (json.dumps(manifest, ensure_ascii=True, separators=(',', ':')) + '\n').encode()
    return dict(files=len(manifest), bytes=size, manifest_sha256=digest(raw),
                apphost_sha256=digest((payload / 'PolicyLanguageServer').read_bytes()))


def verify(payload, pin):
    actual = payload_identity(payload)
    if any(actual[key] != pin[key] for key in actual):
        raise ValueError('pinned Linux payload identity mismatch')
    return actual


def evaluation_inputs(source):
    results = []
    for variant in source['variants']:
        policy = copy.deepcopy(source['policy'])
        if variant == 'unknown_parameter':
            policy['properties']['policyRule']['if']['equals'] = "[parameters('absent')]"
        elif variant == 'changed_input_fact':
            policy['properties']['policyRule']['if']['equals'] = 'different-resource-type'
        elif variant != 'valid':
            raise ValueError('unknown authored control')
        results.append((variant, dict(policyAssignment=json.dumps(source['assignment']), policyDefinitions=[json.dumps(policy)],
                                     policyInitiative='', resources=[json.dumps(source['resource'])],
                                     resourceGroup='', relatedResource='', parameterValues='')))
    return results


def receipt_base():
    return dict(schema_version=1, candidate='AzurePolicy.azurepolicyextension/0.2.6',
                rid='linux-x64', status='pending', native_executed=False,
                script_sha256=digest(Path(__file__).read_bytes()),
                inputs_sha256=digest(INPUT.read_bytes()),
                payload_manifest_sha256=json.loads(PIN.read_text())['manifest_sha256'],
                isolation='fresh Linux network namespace; loopback only; empty private HOME; clean environment',
                calibration_measured=False, tier_granted=False)


def enable_loopback():
    import fcntl
    with socket.socket(socket.AF_INET, socket.SOCK_DGRAM) as sock:
        flags = fcntl.ioctl(sock.fileno(), 0x8913, struct.pack('16sH14x', b'lo', 0))
        old = struct.unpack('16sH14x', flags)[1]
        fcntl.ioctl(sock.fileno(), 0x8914, struct.pack('16sH14x', b'lo', old | 1))


class LSP:
    def __init__(self, process, raw):
        self.process, self.raw, self.messages = process, raw, queue.Queue()
        self.responses = {}
        self.bytes = 0
        self.thread = threading.Thread(target=self.read, daemon=True)
        self.thread.start()

    def read(self):
        try:
            while True:
                length, count = None, 0
                while True:
                    line = self.process.stdout.readline(8193)
                    self.bytes += len(line)
                    count += len(line)
                    if not line:
                        raise EOFError('native stdout closed')
                    if self.bytes > LIMIT or count > 8192:
                        raise ValueError('LSP header/output cap exceeded')
                    if line == b'\r\n':
                        break
                    if line.lower().startswith(b'content-length:'):
                        length = int(line.split(b':', 1)[1])
                if length is None or length < 0 or self.bytes + length > LIMIT:
                    raise ValueError('LSP body/output cap exceeded')
                data = self.process.stdout.read(length)
                if len(data) != length:
                    raise EOFError('incomplete native LSP body')
                self.bytes += length
                self.raw.write(data + b'\n')
                self.raw.flush()
                self.messages.put(json.loads(data))
        except Exception as error:
            self.messages.put(error)

    def send(self, method, params=None, ident=None):
        value = dict(jsonrpc='2.0', method=method)
        if params is not None:
            value['params'] = params
        if ident is not None:
            value['id'] = ident
        body = json.dumps(value).encode()
        self.process.stdin.write(('Content-Length: %d\r\n\r\n' % len(body)).encode() + body)
        self.process.stdin.flush()

    def response(self, ident, seconds=20):
        end = time.monotonic() + seconds
        while ident not in self.responses:
            remaining = end - time.monotonic()
            if remaining <= 0:
                raise TimeoutError('native LSP response timeout')
            value = self.messages.get(timeout=remaining)
            if isinstance(value, Exception):
                raise value
            if 'id' in value and ('result' in value or 'error' in value):
                self.responses[value['id']] = value
        return self.responses[ident]


def summary(value):
    raw = json.dumps(value, sort_keys=True, separators=(',', ':')).encode()
    result = value.get('result') if isinstance(value, dict) else value
    return dict(sha256=digest(raw), bytes=len(raw), result_type=type(result).__name__,
                error_code=value.get('error', {}).get('code') if isinstance(value, dict) else None,
                result_count=len(result) if isinstance(result, (list, dict)) else None)


def worker(payload, out):
    # This entry point is invoked only by the successful unshare preflight.
    enable_loopback()
    resource.setrlimit(resource.RLIMIT_CORE, (0, 0))
    resource.setrlimit(resource.RLIMIT_FSIZE, (LIMIT, LIMIT))
    home = out / 'home'
    home.mkdir()
    (home / 'tmp').mkdir()
    env = dict(PATH='/usr/bin:/bin', HOME=str(home), TMPDIR=str(home / 'tmp'),
               DOTNET_CLI_HOME=str(home), DOTNET_MULTILEVEL_LOOKUP='0',
               DOTNET_CLI_TELEMETRY_OPTOUT='1', DOTNET_EnableDiagnostics='0',
               ASPNETCORE_URLS='http://127.0.0.1:6490', LANG='C.UTF-8')
    receipt = receipt_base()
    receipt['status'] = 'native_probe_failed'
    with (out / 'native-stderr.log').open('wb') as stderr, (out / 'lsp-raw.jsonl').open('wb') as raw:
        process = subprocess.Popen([str(payload / 'PolicyLanguageServer')], stdin=subprocess.PIPE,
                                   stdout=subprocess.PIPE, stderr=stderr, cwd=home, env=env)
        receipt['native_executed'] = True
        try:
            lsp = LSP(process, raw)
            lsp.send('initialize', dict(processId=os.getpid(), rootUri=home.as_uri(), capabilities={}), 1)
            initialized = lsp.response(1)
            receipt['initialize'] = summary(initialized)
            capabilities = (initialized.get('result') or {}).get('capabilities', {})
            receipt['capability_keys'] = sorted(capabilities)
            receipt['reference_provider'] = bool(capabilities.get('referencesProvider'))
            receipt['definition_provider'] = bool(capabilities.get('definitionProvider'))
            if 'error' in initialized:
                raise ValueError('native initialize returned an error')
            lsp.send('initialized', {})
            lsp.send('workspace/executeCommand', dict(command='SET_CACHE_FOLDER_PATH', arguments=[str(home)]), 2)
            receipt['cache_command'] = summary(lsp.response(2))
            source = json.loads(INPUT.read_text())
            text = json.dumps(source['policy'], indent=2) + '\n'
            document = home / 'probe.policydefinition.json'
            document.write_text(text)
            lsp.send('textDocument/didOpen', dict(textDocument=dict(uri=document.as_uri(), languageId='json', version=1, text=text)))
            use_line = next(n for n, line in enumerate(text.splitlines()) if "parameters('kind')" in line)
            use_character = text.splitlines()[use_line].index("'kind'") + 1
            declaration_line = next(n for n, line in enumerate(text.splitlines()) if '"kind": {' in line)
            declaration_character = text.splitlines()[declaration_line].index('kind')
            for ident, method, line, char in [(3, 'textDocument/definition', use_line, use_character),
                                               (4, 'textDocument/references', declaration_line, declaration_character)]:
                params = dict(textDocument=dict(uri=document.as_uri()), position=dict(line=line, character=char))
                if ident == 4:
                    params['context'] = dict(includeDeclaration=True)
                lsp.send(method, params, ident)
                receipt[method.split('/')[-1]] = summary(lsp.response(ident))
            receipt['evaluations'] = []
            for variant, body in evaluation_inputs(source):
                connection = http.client.HTTPConnection('127.0.0.1', 6490, timeout=20)
                try:
                    connection.request('POST', '/evaluatePolicy', body=json.dumps(body), headers={'Content-Type': 'application/json'})
                    response = connection.getresponse()
                    data = response.read(LIMIT + 1)
                    if len(data) > LIMIT:
                        raise ValueError('evaluation response exceeds cap')
                    (out / ('evaluation-' + variant + '.json')).write_bytes(data)
                    receipt['evaluations'].append(dict(variant=variant, http_status=response.status,
                                                       sha256=digest(data), bytes=len(data)))
                finally:
                    connection.close()
            receipt['status'] = 'probe_complete_requires_analysis'
            lsp.send('shutdown', None, 5)
            lsp.response(5, 5)
            lsp.send('exit')
        except Exception as error:
            # Avoid publishing paths, subject content, or ambient identities.
            receipt['failure_class'] = type(error).__name__
        finally:
            if process.poll() is None:
                process.terminate()
                try:
                    process.wait(timeout=5)
                except subprocess.TimeoutExpired:
                    process.kill()
                    process.wait()
            receipt['native_returncode'] = process.returncode
    for key, filename in [('stderr', 'native-stderr.log'), ('lsp', 'lsp-raw.jsonl')]:
        value = (out / filename).read_bytes()
        receipt[key + '_sha256'] = digest(value)
        receipt[key + '_bytes'] = len(value)
    (out / 'receipt.json').write_text(json.dumps(receipt, indent=2) + '\n')
    return 0 if receipt['status'] == 'probe_complete_requires_analysis' else 2


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('-cache', type=Path, required=True)
    parser.add_argument('-out', type=Path, required=True)
    parser.add_argument('--namespace-worker', action='store_true', help=argparse.SUPPRESS)
    args = parser.parse_args()
    cache, out = args.cache.resolve(), args.out.resolve()
    payload = cache / 'payload'
    if args.namespace_worker:
        if os.environ.get('FATHOM_PRIVATE_NETNS') != '1':
            raise SystemExit('namespace worker requires the isolated launcher')
        return worker(payload, out)
    if out.exists() or out == cache or cache in out.parents or out in cache.parents:
        parser.error('fresh, separate cache and output directories required')
    out.mkdir(parents=True)
    receipt = receipt_base()
    try:
        if platform.system() != 'Linux' or platform.machine() not in ('x86_64', 'amd64'):
            raise ValueError('Linux x64 runner required')
        if os.getloadavg()[0] > 6:
            raise ValueError('load exceeds 6; retry later')
        if shutil.disk_usage(out.parent).free < 20 * 1024**3:
            raise ValueError('20 GiB free required')
        unshare = shutil.which('unshare')
        if not unshare:
            raise ValueError('util-linux unshare required')
        clean = dict(PATH='/usr/bin:/bin', LANG='C.UTF-8', FATHOM_PRIVATE_NETNS='1', PYTHONDONTWRITEBYTECODE='1')
        namespace = [unshare, '--user', '--map-root-user', '--net', '--']
        preflight = subprocess.run(namespace + [sys.executable, '-B', '-c',
                                  'import sys;sys.path.insert(0,sys.argv[1]);from probe_linux import enable_loopback;enable_loopback()', str(HERE)],
                                  env=clean, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=10)
        if preflight.returncode:
            raise ValueError('runner disallows unprivileged network namespaces; no unisolated fallback')
        os.nice(10)
        pin = json.loads(PIN.read_text())
        if not payload.exists():
            prepare(cache, 'linux-x64', pin)
        verify(payload, pin)
        command = namespace + [sys.executable, '-B', str(Path(__file__).resolve()), '-cache', str(cache), '-out', str(out), '--namespace-worker']
        process = subprocess.Popen(command, env=clean, start_new_session=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        try:
            code = process.wait(timeout=TIMEOUT)
            if (out / 'receipt.json').is_file():
                return code
            receipt.update(status='namespace_worker_failed', native_executed=None, worker_returncode=code)
        except subprocess.TimeoutExpired:
            os.killpg(process.pid, signal.SIGKILL)
            process.wait()
            receipt.update(status='probe_timeout', native_executed=None)
    except Exception as error:
        receipt.update(status='probe_refused', failure_class=type(error).__name__, reason=str(error))
    (out / 'receipt.json').write_text(json.dumps(receipt, indent=2) + '\n')
    return 2


if __name__ == '__main__':
    raise SystemExit(main())
