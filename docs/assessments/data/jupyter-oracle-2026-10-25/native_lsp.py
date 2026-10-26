#!/usr/bin/env python3
"""Headless direct-notebook LSP coverage probe; independent analyzer tier C."""
import argparse
import ast
import json
import importlib.util
import os
from pathlib import Path
import queue
import subprocess
import sys
import threading


class Client:
    def __init__(self, binary, root, python):
        self.process = subprocess.Popen([str(Path(binary).resolve()), '--stdio'], stdin=subprocess.PIPE,
                                        stdout=subprocess.PIPE, stderr=subprocess.DEVNULL)
        self.messages = queue.Queue()
        self.counter = 0
        self.python = python
        threading.Thread(target=self.read, daemon=True).start()
        self.capabilities = self.request('initialize', {'processId': os.getpid(), 'rootUri': root.as_uri(),
            'workspaceFolders': [{'uri': root.as_uri(), 'name': 'notebook-probe'}],
            'capabilities': {'workspace': {'configuration': True}, 'notebookDocument': {'synchronization': {'dynamicRegistration': False}},
                             'textDocument': {'definition': {'linkSupport': True}}}})
        self.send('initialized', {})

    def read(self):
        try:
            while True:
                headers = {}
                while True:
                    line = self.process.stdout.readline()
                    if not line: return
                    if line in (b'\r\n', b'\n'): break
                    key, value = line.decode().split(':', 1); headers[key.lower()] = value.strip()
                self.messages.put(json.loads(self.process.stdout.read(int(headers['content-length']))))
        except Exception as error:
            self.messages.put({'reader_error': str(error)})

    def write(self, message):
        data = json.dumps(message).encode()
        self.process.stdin.write(f'Content-Length: {len(data)}\r\n\r\n'.encode() + data)
        self.process.stdin.flush()

    def send(self, method, params):
        self.write({'jsonrpc': '2.0', 'method': method, 'params': params})

    def request(self, method, params):
        self.counter += 1
        expected = self.counter
        self.write({'jsonrpc': '2.0', 'id': expected, 'method': method, 'params': params})
        while True:
            message = self.messages.get(timeout=60)
            if 'reader_error' in message: raise RuntimeError(message)
            if message.get('id') == expected and 'method' not in message:
                if 'error' in message: raise RuntimeError(message['error'])
                return message.get('result')
            if 'id' in message and 'method' in message:
                result = None
                if message['method'] == 'workspace/configuration':
                    result = [{'pythonPath': self.python, 'analysis': {'diagnosticMode': 'openFilesOnly',
                              'autoSearchPaths': True, 'useLibraryCodeForTypes': True}}
                              for _ in message['params']['items']]
                self.write({'jsonrpc': '2.0', 'id': message['id'], 'result': result})

    def close(self):
        try: self.request('shutdown', None); self.send('exit', None)
        finally:
            self.process.terminate()
            self.process.wait(timeout=10)


def cells(path):
    doc = json.loads(path.read_text())
    if 'cells' not in doc: raise ValueError('not nbformat4')
    return doc, [''.join(c['source']) if isinstance(c['source'], list) else c['source']
                 for c in doc['cells'] if c['cell_type'] == 'code']


def call_sites(text):
    # This scanner never changes text sent to the oracle. Rejected cells are named.
    lines = text.splitlines()
    if lines and lines[0].lstrip().startswith('%%'):
        magic = lines[0].lstrip()[2:].split()[0]
        if magic not in {'time','timeit','capture','prun','debug','python','python3','pypy'}:
            raise SyntaxError('non-Python cell magic')
        lines[0] = ''
    opaque = 0
    for index, line in enumerate(lines):
        stripped = line.lstrip()
        if stripped.startswith(('%','!')):
            opaque += 1
            lines[index] = line[:len(line)-len(stripped)] + 'pass'
    tree = ast.parse('\n'.join(lines))
    # Reuse the existing public benchmark's declared caller/name projection only;
    # these written pairs remain tier C. Answer presence measures coverage only,
    # never resolved target truth or agreement with the written callee name.
    sys.dont_write_bytecode = True
    recipe = Path(__file__).resolve().parents[4] / 'calibration/oracles/jupyter/jupyter_oracle.py'
    spec = importlib.util.spec_from_file_location('notebook_projection', recipe)
    projection = importlib.util.module_from_spec(spec); spec.loader.exec_module(projection)
    class Walker(projection.CallWalker):
        def __init__(self): super().__init__(); self.sites = []
        def visit_Call(self, node):
            self.sites.append((node, self.owner(), projection.callee_name(node.func)))
            self.generic_visit(node)
    walker = Walker(); walker.visit(tree)
    out = []
    for node, owner, written in walker.sites:
        target = node.func
        if isinstance(target, ast.Attribute):
            name = target.attr
            line = target.end_lineno - 1
            # AST offsets are UTF8 bytes; LSP is UTF16 code units.
            prefix = text.splitlines()[line].encode()[:target.end_col_offset - len(name.encode())].decode()
        elif isinstance(target, ast.Name):
            name = target.id; line = target.lineno - 1
            prefix = text.splitlines()[line].encode()[:target.col_offset].decode()
        else: continue
        out.append({'name': name, 'from': owner, 'to': written, 'line': line, 'character': len(prefix.encode('utf-16-le')) // 2})
    return sorted(out, key=lambda r:(r['line'],r['character'],r['name'])), opaque


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--server', required=True)
    ap.add_argument('--subject', required=True)
    ap.add_argument('--python', default=sys.executable)
    ap.add_argument('--out', required=True)
    a = ap.parse_args()
    root = Path(a.subject).resolve()
    out = Path(a.out); out.parent.mkdir(parents=True, exist_ok=True)
    client = Client(a.server, root, str(Path(a.python).absolute()))
    records = []
    try:
        for path in sorted(root.rglob('*.ipynb')):
            rel = path.relative_to(root).as_posix()
            if 'translations' in path.relative_to(root).parts: continue
            record = {'path': rel, 'calls': [], 'rejected_cells': [], 'opaque_magic_lines': 0}
            records.append(record)
            try: doc, code = cells(path)
            except Exception as e: record['unreadable'] = str(e); continue
            language = (doc.get('metadata',{}).get('language_info') or {}).get('name','python')
            if language.lower() != 'python': record['language'] = language; continue
            uris = ['vscode-notebook-cell:' + path.as_uri().removeprefix('file:') + '#cell' + str(i) for i in range(len(code))]
            client.send('notebookDocument/didOpen', {'notebookDocument': {'uri': path.as_uri(), 'notebookType': 'jupyter-notebook',
                'version': 1, 'cells': [{'kind': 2, 'document': uri} for uri in uris]},
                'cellTextDocuments': [{'uri': uri, 'languageId': 'python', 'version': 1, 'text': text} for uri,text in zip(uris,code)]})
            for index, text in enumerate(code):
                try:
                    sites, opaque = call_sites(text)
                    record['opaque_magic_lines'] += opaque
                except SyntaxError as e:
                    record['rejected_cells'].append({'cell': index, 'line': e.lineno, 'reason': e.msg}); continue
                for site in sites:
                    definitions = client.request('textDocument/definition', {'textDocument': {'uri': uris[index]},
                                                 'position': {'line':site['line'], 'character':site['character']}})
                    answers = []
                    for definition in definitions or []:
                        uri = definition.get('targetUri', definition.get('uri'))
                        category = 'subject' if str(root) in uri else 'external'
                        answers.append({'category':category,
                                        'range':definition.get('targetSelectionRange', definition.get('range'))})
                    record['calls'].append(dict(site, cell=index, definitions=answers))
            client.send('notebookDocument/didClose', {'notebookDocument': {'uri':path.as_uri()},
                'cellTextDocuments':[{'uri':uri} for uri in uris]})
    finally: client.close()
    calls = [c for r in records for c in r['calls']]
    result = {'server': 'basedpyright 1.40.2', 'python':sys.version.split()[0],
              'analysis_python': subprocess.check_output([a.python, '--version'], text=True).strip(),
              'native_notebook_sync': client.capabilities['capabilities'].get('notebookDocumentSync'),
              'files':len(records), 'calls':len(calls), 'resolved_calls':sum(bool(c['definitions']) for c in calls),
              'rejected_cells':sum(len(r['rejected_cells']) for r in records),
              'opaque_magic_lines':sum(r['opaque_magic_lines'] for r in records), 'records':records,
              'analyzer_tier':'C',
              'written_pair_source':'Python AST caller/callee names, not definition target references',
              'resolved_target_truth':False,
              'claim':'direct notebook LSP coverage only; independent analyzer C; no resolved target-pair truth or grade'}
    out.write_text(json.dumps(result, indent=2, sort_keys=True)+'\n')
    print(json.dumps({k:v for k,v in result.items() if k!='records'}, sort_keys=True))


if __name__ == '__main__': main()
