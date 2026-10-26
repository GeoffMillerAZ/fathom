"""Decode complete native lexer events; never consult scanner candidates."""
import re
MAX_TOKENS = 10000
MAX_ARGS = 64
MAX_WORD = 512
MAX_STDERR = 8 * 1024**2
HEADER = re.compile(r'ORACLE_NGINX_TOKEN_V1 (\d+) (\w+) (\d+) (\d+) ([01]) (\d+) ([0-9a-f]+)$')
ARG = re.compile(r'ORACLE_NGINX_ARG_V1 (\d+) (\d+) ([0-9a-f]+|-)$')


def word(value):
    data = b'' if value == '-' else bytes.fromhex(value)
    if len(data) > MAX_WORD:
        raise ValueError('native token word/path cap exceeded')
    return data.decode('utf8', errors='strict')


def decode(data):
    if len(data) > MAX_STDERR:
        raise ValueError('native token stderr cap exceeded')
    tokens = []
    for line in data.decode('utf8', errors='strict').splitlines():
        if 'ORACLE_NGINX_TOKEN_REFUSED' in line:
            raise ValueError('native token observer refused whole profile')
        if 'ORACLE_NGINX_TOKEN_V1' in line:
            match = HEADER.search(line)
            if not match or (tokens and len(tokens[-1]['args']) != tokens[-1]['argc']):
                raise ValueError('invalid/incomplete native token header')
            ident, kind, start, end, last_lf, count, path = match.groups()
            ident, start, end, count = map(int, (ident, start, end, count))
            if ident != len(tokens) + 1 or ident > MAX_TOKENS or count > MAX_ARGS:
                raise ValueError('native token identity/count cap exceeded')
            if kind not in ('simple', 'block', 'end', 'file'):
                raise ValueError('unknown or failed native token kind')
            if (kind in ('simple', 'block') and (count < 1 or start < 1 or end < start) or
                    kind in ('end', 'file') and count != 0 or end < 1):
                raise ValueError('invalid native token metadata')
            tokens.append(dict(id=ident, kind=kind, line=start, end=end,
                               last_lf=int(last_lf), argc=count, path=word(path), args=[]))
        elif 'ORACLE_NGINX_ARG_V1' in line:
            match = ARG.search(line)
            if not match or not tokens:
                raise ValueError('unowned native argument')
            ident, index, value = match.groups()
            token = tokens[-1]
            if int(ident) != token['id'] or int(index) != len(token['args']) or int(index) >= token['argc']:
                raise ValueError('native argument identity/order mismatch')
            token['args'].append(word(value))
    if not tokens or len(tokens[-1]['args']) != tokens[-1]['argc']:
        raise ValueError('empty/incomplete native token population')
    return tokens


def trees(tokens):
    """Each source gets its whole native syntax tree and evaluated EOF line."""
    active, complete = {}, {}
    for token in tokens:
        path = token['path']
        if path in complete:
            raise ValueError('repeated native source evaluation needs explicit identity treatment')
        if path not in active:
            active[path] = dict(root=[], stack=[], token_count=0)
        state = active[path]
        state['token_count'] += 1
        kind = token['kind']
        if kind in ('simple', 'block'):
            node = dict(directive=token['args'][0], args=token['args'][1:], line=token['line'], end=token['end'])
            target = state['stack'][-1]['block'] if state['stack'] else state['root']
            target.append(node)
            if kind == 'block':
                node['block'] = []
                state['stack'].append(node)
        elif kind == 'end':
            if not state['stack']:
                raise ValueError('native close block lacks opener')
            node = state['stack'].pop()
            if token['end'] < node['end']:
                raise ValueError('native close block precedes header')
            node['end'] = token['end']
        else:
            if state['stack']:
                raise ValueError('native file ended with an open block')
            complete[path] = dict(tree=state['root'], file_end=max(1, token['end'] - token['last_lf']),
                                  token_count=state['token_count'])
            del active[path]
    if active:
        raise ValueError('native source lacks file-end event')
    return complete
