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
from receipt_contract import SCHEMA, read_receipt, validate, write_receipt
from filesystem_isolation import isolate_filesystem, restricted_command
from write_tracing import BoundedCapture, trace_identity, trace_command
from write_diagnostics import diagnostics, MAX_TRACE
from vendor_exec import VENDOR_LIMIT, vendor_command

CONTRACT = json.loads(SCHEMA.read_text())

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


def kernel_capabilities():
    values = {}
    for key, path in [('apparmor_restrict_unprivileged_userns', '/proc/sys/kernel/apparmor_restrict_unprivileged_userns'),
                      ('unprivileged_userns_clone', '/proc/sys/kernel/unprivileged_userns_clone'),
                      ('max_user_namespaces', '/proc/sys/user/max_user_namespaces')]:
        try:
            value = int(Path(path).read_text().strip())
            values[key] = value if value >= 0 else None
        except (OSError, ValueError):
            values[key] = None
    return values


def failure_class(error):
    name = type(error).__name__
    return name if name in CONTRACT['properties']['failure_class']['enum'] else 'OtherError'


def receipt_base():
    return dict(schema_version=3, candidate='AzurePolicy.azurepolicyextension/0.2.6',
                rid='linux-x64', status='pending', native_executed=False,
                script_sha256=digest(Path(__file__).read_bytes()),
                filesystem_script_sha256=digest(Path(__file__).with_name('filesystem_isolation.py').read_bytes()),
                inputs_sha256=digest(INPUT.read_bytes()),
                payload_manifest_sha256=json.loads(PIN.read_text())['manifest_sha256'],
                isolation='linux_user_net_pid_mount_namespaces', failed_stage=None,
                kernel_capabilities=kernel_capabilities(),
                calibration_measured=False, tier_granted=False,
                diagnostics_script_sha256=digest(Path(__file__).with_name('write_diagnostics.py').read_bytes()),
                vendor_script_sha256=digest(Path(__file__).with_name('vendor_exec.py').read_bytes()),
                tracing_script_sha256=digest(Path(__file__).with_name('write_tracing.py').read_bytes()),
                vendor_file_limit_bytes=VENDOR_LIMIT, launcher_file_limit_bytes=LIMIT,
                trace_limit_bytes=MAX_TRACE, receipt_limit_bytes=128*1024)


def attach_diagnostics(receipt, trace, scratch, trace_tool):
    """Outer-only evidence; refusal never erases a worker-validated result."""
    receipt.update(trace_tool=trace_tool,trace_within_cap=len(trace.encode('utf-8','surrogateescape'))<=MAX_TRACE)
    try:
        written=diagnostics(trace,scratch)
        if receipt.get('native_executed') is True:
            if not written['vendor_exec_observed'] or written['native_returncode'] is None:
                raise ValueError('trace did not attest vendor exec and exit')
            receipt['trace_writer_hidden_from_vendor']=True
            receipt['native_returncode']=written['native_returncode']
        receipt.update(write_diagnostics=written,diagnostic_status='partial' if written['files_truncated'] or written['snapshot_truncated'] else 'complete')
    except Exception as error:
        receipt.update(write_diagnostics=None,diagnostic_status='refused',diagnostic_failure_class=failure_class(error))
    return receipt


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
    code = value.get('error', {}).get('code') if isinstance(value, dict) else None
    if code is not None and type(code) is not int:
        raise ValueError('native error code must be integer or null')
    return dict(sha256=digest(raw), bytes=len(raw), result_type=type(result).__name__,
                error_code=code,
                result_count=len(result) if isinstance(result, (list, dict)) else None)


def wait_for_port(seconds=10):
    end = time.monotonic() + seconds
    while time.monotonic() < end:
        try:
            with socket.create_connection(('127.0.0.1', 6490), timeout=0.25):
                return True
        except OSError:
            time.sleep(0.1)
    return False


def evaluate_controls(source, out):
    wait_for_port()
    rows = []
    for variant, body in evaluation_inputs(source):
        row = dict(variant=variant)
        connection = http.client.HTTPConnection('127.0.0.1', 6490, timeout=20)
        try:
            connection.request('POST', '/evaluatePolicy', body=json.dumps(body), headers={'Content-Type': 'application/json'})
            response = connection.getresponse()
            data = response.read(LIMIT + 1)
            if len(data) > LIMIT:
                raise ValueError('evaluation response exceeds cap')
            (out / ('evaluation-' + variant + '.json')).write_bytes(data)
            row.update(http_status=response.status, sha256=digest(data), bytes=len(data))
        except Exception as error:
            row['failure_class'] = failure_class(error)
            with (out / 'evaluation-errors.log').open('a') as private:
                private.write(repr(error) + '\n')
        finally:
            connection.close()
        rows.append(row)
    return rows


def kill_group(process):
    try:
        os.killpg(process.pid, signal.SIGKILL)
    except ProcessLookupError:
        pass


def private_log(path, data):
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    with os.fdopen(fd, 'wb') as target:
        target.write(data)


def read_private_log(path):
    import stat
    fd=os.open(path,os.O_RDONLY|os.O_NOFOLLOW|os.O_NONBLOCK)
    with os.fdopen(fd,'rb') as source:
        if not stat.S_ISREG(os.fstat(source.fileno()).st_mode):
            raise ValueError('private log must be regular')
        value=source.read(LIMIT+1)
    if len(value)>LIMIT: raise ValueError('private log exceeds output cap')
    return value


def log_error(out, error):
    try:
        private_log(out / 'probe-error.log', (repr(error) + '\n').encode())
    except OSError:
        # Log collisions must never divert the schema-validated refusal path.
        pass


def publish_receipt(receipt, out, publish, contract):
    try:
        private = out / 'preflight-stderr.log'
        if private.exists():
            fd = os.open(private, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
            with os.fdopen(fd, 'rb') as source:
                import stat
                if not stat.S_ISREG(os.fstat(source.fileno()).st_mode):
                    raise ValueError('preflight log must be regular')
                data = source.read(LIMIT + 1)
            if len(data) > LIMIT:
                raise ValueError('preflight log exceeds cap')
            receipt.update(preflight_stderr_sha256=digest(data), preflight_stderr_bytes=len(data))
        write_receipt(out / 'receipt.json', receipt, contract, public=True)
        if publish:
            write_receipt(publish, receipt, contract, public=True)
    except Exception as error:
        receipt=receipt_base()
        receipt.update(status='receipt_invalid', native_executed=None, failure_class=failure_class(error), failed_stage='receipt')
        log_error(out, error)
        # Even a private-tail failure must publish an outer-generated refusal.
        # Private and public locations are outside the writable worker mount.
        try:
            write_receipt(out / 'receipt.json', receipt, contract, public=True)
        except OSError:
            pass
        if publish:
            write_receipt(publish, receipt, contract, public=True)
    return 0 if receipt['status'] == 'probe_complete_requires_analysis' else 2


def copy_payload(payload, destination, pin):
    shutil.copytree(payload, destination)
    verify(destination, pin)


def worker(payload, out, protected=None, trace_fd=None):
    # Measure isolation before any vendor byte executes, even for manual calls.
    if os.getpid() != 1 or socket.if_nameindex() != [(1, 'lo')]:
        raise ValueError('worker requires PID1 and loopback-only network namespace')
    filesystem_flags = isolate_filesystem(out, protected)
    enable_loopback()
    resource.setrlimit(resource.RLIMIT_CORE, (0, 0))
    resource.setrlimit(resource.RLIMIT_FSIZE, (LIMIT, VENDOR_LIMIT))
    home = out / 'home'
    home.mkdir()
    (home / 'tmp').mkdir()
    env = dict(PATH='/usr/bin:/bin', HOME=str(home), TMPDIR=str(home / 'tmp'),
               DOTNET_CLI_HOME=str(home), DOTNET_MULTILEVEL_LOOKUP='0',
               DOTNET_CLI_TELEMETRY_OPTOUT='1', DOTNET_EnableDiagnostics='0',
               ASPNETCORE_URLS='http://127.0.0.1:6490', LANG='C.UTF-8')
    receipt = receipt_base()
    receipt.update(status='native_probe_failed', netns_loopback_only=True,
                   measured_interfaces=1, shutdown_clean=False, failed_stage='execution')
    receipt.update(filesystem_flags)
    if trace_fd is None or trace_fd < 3:
        raise ValueError('launcher-owned trace pipe required')
    with (out / 'native-stderr.log').open('wb') as stderr, (out / 'lsp-raw.jsonl').open('wb') as raw:
        command = trace_command('/usr/bin/strace',trace_fd,vendor_command(home,payload))
        try:
            process = subprocess.Popen(command, stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                                       stderr=subprocess.PIPE, cwd=home, env=env, start_new_session=True,
                                       pass_fds=(trace_fd,))
        finally:
            os.close(trace_fd)
        errors = BoundedCapture(process.stderr, stderr, LIMIT)
        receipt['native_executed'] = True
        lsp = None
        try:
            lsp = LSP(process, raw)
            receipt['failed_stage'] = 'initialize'
            lsp.send('initialize', dict(processId=os.getpid(), rootUri=home.as_uri(), capabilities={}), 1)
            initialized = lsp.response(1)
            receipt['initialize'] = summary(initialized)
            capabilities = (initialized.get('result') or {}).get('capabilities', {})
            # Only standard capability flag names can leave the runner.
            known = set(CONTRACT['properties']['capability_keys']['items']['enum'])
            receipt['capability_keys'] = sorted(set(capabilities) & known)
            receipt['unknown_capability_keys'] = len(set(capabilities) - known)
            receipt['reference_provider'] = bool(capabilities.get('referencesProvider'))
            receipt['definition_provider'] = bool(capabilities.get('definitionProvider'))
            if 'error' in initialized:
                raise ValueError('native initialize returned an error')
            lsp.send('initialized', {})
            receipt['failed_stage'] = 'cache'
            lsp.send('workspace/executeCommand', dict(command='SET_CACHE_FOLDER_PATH', arguments=[str(home)]), 2)
            receipt['cache_command'] = summary(lsp.response(2))
            source = json.loads(INPUT.read_text())
            text = json.dumps(source['policy'], indent=2) + '\n'
            document = home / 'probe.policydefinition.json'
            document.write_text(text)
            lsp.send('textDocument/didOpen', dict(textDocument=dict(uri=document.as_uri(), languageId='json', version=1, text=text)))
            receipt['failed_stage'] = 'bindings'
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
            receipt['failed_stage'] = 'evaluations'
            receipt['evaluations'] = evaluate_controls(source, out)
            receipt['failed_stage'] = 'shutdown'
            lsp.send('shutdown', None, 5)
            lsp.response(5, 5)
            lsp.send('exit')
            process.wait(timeout=5)
            receipt['shutdown_clean'] = process.returncode == 0
            if receipt['shutdown_clean'] and all('failure_class' not in row for row in receipt['evaluations']):
                receipt.update(status='probe_complete_requires_analysis', failed_stage=None)
            elif not receipt['shutdown_clean']:
                receipt['failed_stage'] = 'shutdown'
            else:
                receipt['failed_stage'] = 'evaluations'
        except Exception as error:
            receipt['failure_class'] = failure_class(error)
            (out / 'worker-error.log').write_text(repr(error) + '\n')
        finally:
            # Kill the normal-path group too. The enclosing PID namespace's
            # init exit then kills descendants that escaped with setsid.
            try:
                process.wait(timeout=1)
            except subprocess.TimeoutExpired:
                pass
            kill_group(process)
            process.wait()
            if lsp:
                lsp.thread.join(timeout=1)
            receipt['tracer_returncode'] = process.returncode
            receipt['native_returncode'] = process.returncode  # outer trace replaces this before publication
            errors.check()
    for key, filename in [('stderr', 'native-stderr.log'), ('lsp', 'lsp-raw.jsonl')]:
        value = read_private_log(out / filename)
        receipt[key + '_sha256'] = digest(value)
        receipt[key + '_bytes'] = len(value)
    write_receipt(out / 'receipt.json', receipt, CONTRACT)
    return 0 if receipt['status'] == 'probe_complete_requires_analysis' else 2


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('-cache', type=Path, required=True)
    parser.add_argument('-out', type=Path, required=True)
    parser.add_argument('-min-free-gib', type=float, default=20)
    parser.add_argument('-max-load', type=float, default=6)
    parser.add_argument('-publish', type=Path, help='separate validated receipt destination')
    parser.add_argument('--namespace-worker', action='store_true', help=argparse.SUPPRESS)
    parser.add_argument('--protected-dir', type=Path, action='append', help=argparse.SUPPRESS)
    parser.add_argument('--trace-fd', type=int, help=argparse.SUPPRESS)
    parser.add_argument('--controls-failed', action='store_true', help=argparse.SUPPRESS)
    args = parser.parse_args()
    cache, out = args.cache.resolve(), args.out.resolve()
    payload = cache / 'payload'
    if args.namespace_worker:
        if os.environ.get('FATHOM_PRIVATE_NETNS') != '1':
            raise SystemExit('namespace worker requires the isolated launcher')
        return worker(payload, out, args.protected_dir, args.trace_fd)
    if (out.exists() or out == cache or cache in out.parents or out in cache.parents or
            args.min_free_gib < 1 or args.max_load <= 0):
        parser.error('fresh separate directories and valid positive resource floors required')
    out.mkdir(parents=True, mode=0o700)
    scratch = out / 'worker'
    scratch.mkdir(mode=0o700)
    receipt = receipt_base()
    publish = None
    # Pin schema and identity in launcher memory before vendor execution.
    contract = copy.deepcopy(CONTRACT)
    identity = {key: receipt[key] for key in ['schema_version', 'candidate', 'rid', 'script_sha256', 'filesystem_script_sha256',
                                            'inputs_sha256', 'payload_manifest_sha256', 'isolation',
                                            'calibration_measured', 'tier_granted', 'diagnostics_script_sha256', 'vendor_script_sha256',
                                            'tracing_script_sha256', 'vendor_file_limit_bytes', 'launcher_file_limit_bytes',
                                            'trace_limit_bytes', 'receipt_limit_bytes']}
    stage = 'platform'
    try:
        if platform.system() != 'Linux' or platform.machine() not in ('x86_64', 'amd64'):
            raise ValueError('Linux x64 runner required')
        stage = 'resources'
        checkout = HERE.parents[2]
        if checkout == cache or checkout in cache.parents or checkout == out or checkout in out.parents:
            raise ValueError('execution data must be outside the checkout')
        cache.mkdir(parents=True, exist_ok=True, mode=0o700)
        if args.publish:
            candidate = args.publish.resolve()
            if candidate == out or out in candidate.parents or candidate == cache or cache in candidate.parents:
                raise ValueError('published receipt must be separate from private data')
            candidate.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
            publish = candidate
        if os.getloadavg()[0] > args.max_load:
            raise ValueError('load exceeds configured maximum')
        if min(shutil.disk_usage(cache).free, shutil.disk_usage(out).free) < args.min_free_gib * 1024**3:
            raise ValueError('disk below configured floor')
        stage = 'unshare'
        unshare = shutil.which('unshare')
        if not unshare:
            raise ValueError('util-linux unshare required')
        clean = dict(PATH='/usr/bin:/bin', LANG='C.UTF-8', FATHOM_PRIVATE_NETNS='1', PYTHONDONTWRITEBYTECODE='1')
        protected = [checkout, Path.home().resolve(), Path(os.environ.get('RUNNER_TEMP', '/tmp')).resolve(), cache]
        userns = [unshare, '--user', '--map-root-user']
        netns = userns + ['--net']
        pidns = netns + ['--pid', '--fork', '--kill-child']
        namespace = pidns + ['--mount', '--mount-proc']
        log_fd = os.open(out / 'preflight-stderr.log', os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
        with os.fdopen(log_fd, 'wb') as stderr:
            for stage, options in [('userns', userns), ('netns', netns), ('pidns', pidns)]:
                result = subprocess.run(options + ['--', '/bin/true'], env=clean, stdout=subprocess.DEVNULL,
                                        stderr=stderr, timeout=10)
                if result.returncode:
                    raise ValueError('namespace preflight refused')
            stage = 'mountns'
            code = ("import sys;sys.path.insert(0,sys.argv[1]);"
                    "exec(\"try:\\n from probe_linux import enable_loopback,socket,isolate_filesystem,Path\\nexcept Exception:\\n sys.exit(40)\\n"
                    "try:\\n isolate_filesystem(Path(sys.argv[2]),[Path(p) for p in sys.argv[3:]])\\nexcept Exception:\\n sys.exit(42)\\n"
                    "try:\\n assert socket.if_nameindex()==[(1,'lo')]\\n enable_loopback()\\nexcept Exception:\\n sys.exit(41)\")")
            result = subprocess.run(namespace + ['--', sys.executable, '-B', '-c', code, str(HERE), str(scratch), *map(str, protected)],
                                    env=clean, stdout=subprocess.DEVNULL, stderr=stderr, timeout=30)
            if result.returncode:
                stage = {40: 'import', 41: 'loopback'}.get(result.returncode, 'mountns')
                raise ValueError('namespace import, filesystem or loopback preflight refused')
        stage = 'filesystem_controls'
        if args.controls_failed:
            raise ValueError('required authored kernel controls failed')
        stage = 'tracing'
        if not Path('/usr/bin/strace').is_file():
            raise ValueError('runner strace required; no install or untraced fallback')
        trace_tool = trace_identity()
        current_nice = os.getpriority(os.PRIO_PROCESS, 0)
        if current_nice < 10:
            os.nice(10 - current_nice)
        stage = 'preparation'
        pin = json.loads(PIN.read_text())
        if not payload.exists():
            prepare(cache, 'linux-x64', pin, min_free_gib=args.min_free_gib)
        stage = 'verification'
        verify(payload, pin)
        copy_payload(payload, scratch / 'payload', pin)
        # Preparation has separately pinned archive/member bounds. From native
        # launch onward, the launcher enforces 8 MiB while its child can raise
        # only to the finite 256 MiB hard ceiling.
        resource.setrlimit(resource.RLIMIT_CORE,(0,0))
        resource.setrlimit(resource.RLIMIT_FSIZE,(LIMIT,VENDOR_LIMIT))
        stage = 'worker'
        command = namespace + ['--', sys.executable, '-B', str(Path(__file__).resolve()), '-cache', str(scratch), '-out', str(scratch), '--namespace-worker']
        for directory in protected:
            command += ['--protected-dir', str(directory)]
        reader, writer = os.pipe()
        command += ['--trace-fd', str(writer)]
        with (out / 'native-trace.log').open('xb') as trace:
            capture = BoundedCapture(os.fdopen(reader, 'rb'), trace, MAX_TRACE)
            try:
                process = subprocess.Popen(command, env=clean, start_new_session=True, stdout=subprocess.DEVNULL,
                                           stderr=subprocess.DEVNULL, pass_fds=(writer,))
            finally:
                os.close(writer)
            try:
                end = time.monotonic() + TIMEOUT
                while True:
                    if capture.overflow or capture.error:
                        stage = 'tracing'
                        raise ValueError('trace capture failed or exceeded cap')
                    remaining = end - time.monotonic()
                    if remaining <= 0: raise subprocess.TimeoutExpired(command, TIMEOUT)
                    try:
                        code = process.wait(timeout=min(0.2, remaining)); break
                    except subprocess.TimeoutExpired:
                        continue
                receipt.update(status='namespace_worker_failed', native_executed=None, worker_returncode=code,
                               failed_stage='worker')
            except subprocess.TimeoutExpired:
                receipt.update(status='probe_timeout', native_executed=None, failed_stage='worker')
            finally:
                kill_group(process)
                process.wait()
                stage = 'tracing'
                capture.check()
        if receipt['status'] != 'probe_timeout' and (scratch / 'receipt.json').exists():
            stage = 'receipt'
            try:
                if scratch.is_symlink():
                    raise ValueError('output directory symlink refused')
                published = read_receipt(scratch / 'receipt.json', contract, identity)
                if (code == 0) != (published['status'] == 'probe_complete_requires_analysis'):
                    raise ValueError('worker status disagrees with exit')
                receipt = published
            except Exception as error:
                receipt.update(status='receipt_invalid', failure_class=failure_class(error), failed_stage='receipt')
                log_error(out, error)
        stage = 'tracing'
        raw_trace = (out / 'native-trace.log').read_bytes()
        attach_diagnostics(receipt,raw_trace.decode('utf-8','surrogateescape'),scratch,trace_tool)
    except Exception as error:
        receipt.update(status='probe_refused', failure_class=failure_class(error), failed_stage=stage)
        log_error(out, error)
    return publish_receipt(receipt, out, publish, contract)


if __name__ == '__main__':
    raise SystemExit(main())
