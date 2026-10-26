"""Project file-local definition contract from a complete native lexer tree."""
from pathlib import Path, PurePosixPath
from native_tokens import decode, trees

DATA = {'map', 'geo', 'types', 'split_clients', 'charset_map', 'match'}


def definitions(path, native, private_directory='', owner_sites=None):
    output = [dict(path=path, name=PurePosixPath(path).name, kind='package', line=1, end=native['file_end'])]

    def add(name, kind, node):
        if private_directory and private_directory in name:
            raise ValueError('isolation changed a graded native definition name; no guessed inverse')
        output.append(dict(path=path, name=name, kind=kind, line=node['line'], end=node['end']))

    def walk(nodes, server='', upstream='', data=False, owner=PurePosixPath(path).name):
        for node in nodes:
            name, args = node['directive'], node['args']
            block = node.get('block')
            if owner_sites is not None and not data and name in {'proxy_pass','grpc_pass','fastcgi_pass','uwsgi_pass','scgi_pass','memcached_pass'} and args:
                owner_sites.setdefault((node['end'], name, args[0]), set()).add(owner)
            if data or name in DATA and block is not None:
                if block is not None:
                    walk(block, server, upstream, True, owner)
            elif name == 'server' and block is not None and not upstream:
                label = 'server'
                for directive in ['server_name', 'listen']:
                    choices = [child['args'][0] for child in block if child['directive'] == directive and child['args'] and child['args'][0]]
                    if choices:
                        label = choices[0]
                        break
                add(label, 'service', node)
                walk(block, label, '', False, label)
            elif name == 'location' and block is not None:
                label = ' '.join(args)
                label = (server + ' ' if server else '') + label
                add(label, 'route', node)
                walk(block, server, upstream, False, label)
            elif name == 'upstream' and block is not None and args:
                add(args[0], 'service', node)
                walk(block, server, args[0], False, args[0])
            elif name == 'server' and block is None and upstream and args:
                add(upstream + ' ' + args[0], 'target', node)
            elif block is not None:
                walk(block, server, upstream, False, owner)
    walk(native['tree'])
    return output


class DefinitionCapture:
    def __init__(self, target):
        self.target = target
        self.result = None
        self.failed_contexts = 0
        self.owner_sites = {}

    def __call__(self, record):
        if record['target'] != self.target:
            raise ValueError('native definition capture target changed')
        if record['returncode']:
            self.failed_contexts += 1
            return
        if self.result is not None:
            raise ValueError('more than one successful native context')
        directory = Path(record['directory']).resolve()
        prefix = PurePosixPath(record['prefix'])
        if prefix.is_absolute() or '..' in prefix.parts:
            raise ValueError('native definition prefix escaped profile')
        parsed = trees(decode(record['stderr'].encode()))
        located = {}
        for filename, native in parsed.items():
            file = Path(filename)
            resolved = (file if file.is_absolute() else directory / file).resolve()
            try:
                relative = resolved.relative_to(directory).as_posix()
            except ValueError as error:
                raise ValueError('native definition source escaped profile') from error
            if relative == '_wrapper.conf':
                continue
            located[(prefix / relative).as_posix()] = native
        if self.target not in located:
            raise ValueError('native population never parsed the original target')
        native = located[self.target]
        self.result = dict(definitions=definitions(self.target, native, str(directory), self.owner_sites),
                           native_tokens_all_sources=sum(row['token_count'] * row['evaluations'] for row in parsed.values()),
                           native_tokens_target=native['token_count'], native_sources=len(parsed),
                           native_source_evaluations=sum(row['evaluations'] for row in parsed.values()),
                           native_target_evaluations=native['evaluations'],
                           native_target_file_end=native['file_end'], mode=record['mode'])

    def owner(self, event):
        owners = self.owner_sites.get((event['line'], event['command'], event['argument']), set())
        return next(iter(owners)) if len(owners) == 1 else None

    def selected(self):
        if self.result is None:
            raise ValueError('native definitions need a successful whole original profile')
        return self.result
