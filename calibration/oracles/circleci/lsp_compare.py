"""Compare every local command/executor source use with the vendor LSP.

The YAML walk proposes positions only. Native definition/reference responses
are recorded without a tier claim, extractor input, exclusions or fallback.
"""
import argparse
import hashlib
import json
import os
import queue
import subprocess
import threading
import time
from pathlib import Path
import yaml


def entries(node):
    return node.value if isinstance(node, yaml.MappingNode) else []


def get(node, name):
    return next((v for k, v in entries(node) if k.value == name), None)


def scalar(node):
    return node.value if isinstance(node, yaml.ScalarNode) else ''


def candidate_sites(source, include_orbs=False):
    root = yaml.compose(source)
    lines = source.splitlines()
    declarations = {}
    for group in ('commands', 'executors'):
        declarations[group] = {}
        for key, value in entries(get(root, group)):
            declarations[group].setdefault(key.value, key)

    def position(node):
        mark = node.start_mark
        prefix = lines[mark.line][:mark.column]
        return {'line': mark.line, 'character': len(prefix.encode('utf-16-le')) // 2}

    sites = []

    def add(owner, node, family):
        name = scalar(node)
        group = 'executors' if family == 'executor' else 'commands'
        target = declarations[group].get(name)
        if target is None and not (include_orbs and '/' in name):
            return
        sites.append({'owner': owner, 'target': name, 'family': family,
                      'position': position(node),
                      'declaration': position(target) if target else None,
                      'scope': 'local' if target else 'orb'})

    def steps(owner, node, ancestors=()):
        if node is None or id(node) in ancestors:
            return
        ancestors = ancestors + (id(node),)
        for key, value in entries(node):
            if key.value == 'steps' and isinstance(value, yaml.SequenceNode):
                for item in value.value:
                    if isinstance(item, yaml.ScalarNode):
                        add(owner, item, 'command')
                    else:
                        for candidate, _ in entries(item):
                            add(owner, candidate, 'command')
            steps(owner, value, ancestors)
        if isinstance(node, yaml.SequenceNode):
            for item in node.value:
                steps(owner, item, ancestors)

    for owner, body in entries(get(root, 'jobs')):
        executor = get(body, 'executor')
        if isinstance(executor, yaml.MappingNode):
            executor = get(executor, 'name')
        if executor is not None:
            add(owner.value, executor, 'executor')
        steps(owner.value, body)
    for owner, body in entries(get(root, 'commands')):
        steps(owner.value, body)
    return sites


def contains(rng, point):
    p = (point['line'], point['character'])
    start, end = rng['start'], rng['end']
    return (start['line'], start['character']) <= p < (end['line'], end['character'])


class Rpc:
    def __init__(self, binary, env, error_log):
        self.process = subprocess.Popen([str(binary), '-stdio', '-debug=false'], stdin=subprocess.PIPE,
                                        stdout=subprocess.PIPE, stderr=error_log, env=env)
        self.messages = queue.Queue()
        self.identifier = 0
        threading.Thread(target=self.read, daemon=True).start()

    def read(self):
        stream = self.process.stdout
        try:
            while True:
                headers = {}
                while True:
                    line = stream.readline()
                    if not line:
                        raise EOFError('server closed stdout')
                    if line == b'\r\n':
                        break
                    key, value = line.decode().split(':', 1)
                    headers[key.lower()] = value.strip()
                size = int(headers['content-length'])
                body = stream.read(size)
                if len(body) != size:
                    raise EOFError('truncated LSP frame')
                self.messages.put(json.loads(body))
        except Exception as error:
            self.messages.put(error)

    def send(self, message):
        body = json.dumps(dict(jsonrpc='2.0', **message)).encode()
        self.process.stdin.write(b'Content-Length: %d\r\n\r\n' % len(body) + body)
        self.process.stdin.flush()

    def notify(self, method, params):
        self.send({'method': method, 'params': params})

    def request(self, method, params):
        self.identifier += 1
        identifier = self.identifier
        self.send({'id': identifier, 'method': method, 'params': params})
        deadline = time.monotonic() + 60
        while True:
            message = self.messages.get(timeout=max(.01, deadline - time.monotonic()))
            if isinstance(message, Exception):
                raise message
            if message.get('id') == identifier and 'method' not in message:
                if 'error' in message:
                    raise RuntimeError(str(message['error']))
                return message.get('result')
            if 'method' in message and 'id' in message:
                result = None
                if message['method'] == 'workspace/configuration':
                    result = [{} for _ in message.get('params', {}).get('items', [])]
                self.send({'id': message['id'], 'result': result})
            # Telemetry and diagnostics are notifications to this local client.

    def close(self):
        try:
            self.request('shutdown', None)
            self.notify('exit', None)
            self.process.wait(timeout=2)
        except Exception:
            self.process.kill()
            self.process.wait()
        self.process.stdin.close()
        self.process.stdout.close()


def locations(response):
    if response is None:
        return []
    if isinstance(response, dict):
        response = [response]
    return [(x.get('uri', x.get('targetUri')),
             x.get('range', x.get('targetSelectionRange', x.get('targetRange')))) for x in response]


def compare_file(binary, file, work):
    source = file.read_text()
    sites = candidate_sites(source, include_orbs=True)
    work.mkdir(parents=True, exist_ok=True)
    home = work / 'home'
    home.mkdir(exist_ok=True)
    env = {key: os.environ[key] for key in ('PATH', 'TMPDIR') if key in os.environ}
    env.update(HOME=str(home.resolve()), XDG_CACHE_HOME=str((home / 'cache').resolve()), GOMAXPROCS='2')
    version = subprocess.check_output([str(binary), '-version'], env=env, text=True).strip()
    if version != '0.54.0':
        raise ValueError('unexpected vendor LSP version: ' + version)
    uri = file.resolve().as_uri()
    results = []
    with (work / 'lsp.log').open('w') as log:
        rpc = Rpc(binary, env, log)
        try:
            initialized = rpc.request('initialize', {'processId': os.getpid(), 'rootUri': file.parent.parent.resolve().as_uri(),
                                                     'capabilities': {}, 'initializationOptions': {'editDebounceMs': 0}})
            rpc.notify('initialized', {})
            rpc.notify('textDocument/didOpen', {'textDocument': {'uri': uri, 'languageId': 'yaml', 'version': 1, 'text': source}})
            # A synchronous native parse request follows didOpen; no sleep-based acceptance.
            symbols = rpc.request('textDocument/documentSymbol', {'textDocument': {'uri': uri}})
            if sites and not symbols:
                raise RuntimeError('native LSP returned no symbols for a document with candidate sites')
            for site in sites:
                response = rpc.request('textDocument/definition', {'textDocument': {'uri': uri}, 'position': site['position']})
                resolved = locations(response)
                local_match = site['declaration'] is not None and any(target == uri and contains(rng, site['declaration']) for target, rng in resolved)
                reference_match = False
                if site['declaration'] is not None:
                    refs = rpc.request('textDocument/references', {'textDocument': {'uri': uri}, 'position': site['declaration'],
                                                                   'context': {'includeDeclaration': False}})
                    reference_match = any(target == uri and contains(rng, site['position']) for target, rng in locations(refs))
                results.append(dict(site, definition_agrees=local_match, references_agree=reference_match,
                                    returned_locations=len(resolved), external_locations=sum(target != uri for target, _ in resolved)))
        finally:
            rpc.close()
    return {'version': version, 'server_info': initialized.get('serverInfo'), 'sites': results}


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('-lsp', type=Path, required=True)
    parser.add_argument('-subject', type=Path, required=True)
    parser.add_argument('-work', type=Path, required=True)
    parser.add_argument('-out', type=Path, required=True)
    parser.add_argument('-yaml-refs', type=Path, required=True)
    args = parser.parse_args()
    sha = subprocess.check_output(['git', '-C', str(args.subject), 'rev-parse', 'HEAD'], text=True).strip()
    if subprocess.check_output(['git', '-C', str(args.subject), 'status', '--porcelain', '--', '.circleci'], text=True).strip():
        raise SystemExit('subject config content must be clean')
    yaml_refs = json.loads(args.yaml_refs.read_text())
    reports = []
    for index, file in enumerate(sorted((args.subject / '.circleci').glob('config.y*ml'))):
        source = file.read_text()
        root = yaml.compose(source)
        owners = {k.value for group in ('jobs', 'commands') for k, _ in entries(get(root, group))}
        targets = {k.value for group in ('executors', 'commands') for k, _ in entries(get(root, group))}
        rel = file.relative_to(args.subject).as_posix()
        expected = {(x['owner'], x['target']) for x in candidate_sites(source)}
        existing = {(x['from'], x['to']) for x in yaml_refs['references'] if x['path'] == rel and x['from'] in owners and x['to'] in targets}
        if expected != existing:
            raise SystemExit('source-site inventory differs from existing yaml.v3 pairs: ' + str({'missing_sites': sorted(existing-expected), 'extra_sites': sorted(expected-existing)}))
        report = compare_file(args.lsp.resolve(), file, args.work / str(index))
        report['yaml_v3_pairs'] = len(existing)
        report['path'] = file.relative_to(args.subject).as_posix()
        reports.append(report)
    if not reports:
        raise SystemExit('no configuration')
    result = {'subject_sha': sha, 'binary_sha256': hashlib.sha256(args.lsp.read_bytes()).hexdigest(), 'files': reports}
    args.out.write_text(json.dumps(result, indent=2) + '\n')
    print('native LSP comparison', len(reports), 'configs', sum(len(x['sites']) for x in reports), 'sites')
