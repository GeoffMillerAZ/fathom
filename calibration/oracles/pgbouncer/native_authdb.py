"""Native auth_dbname probe and a minimal PostgreSQL v3 admin client.

Protocol: https://www.postgresql.org/docs/18/protocol-message-formats.html
Only admin queries run. Backend endpoints are isolated to a nonexistent local
socket directory; positive probes disable the target before client startup.
"""
import json
import re
import socket
import struct
import subprocess
import time
from pathlib import Path
from pgbouncer_oracle import scan, params, sanitized, free_port


class PGError(RuntimeError):
    def __init__(self, fields):
        self.fields = fields
        super().__init__(fields.get('M', 'native protocol error'))


def error_fields(body):
    fields = {}
    i = 0
    while i < len(body) and body[i]:
        end = body.index(0, i + 1)
        fields[chr(body[i])] = body[i + 1:end].decode('utf8')
        i = end + 1
    return fields


def data_row(body):
    count, = struct.unpack_from('!H', body)
    values, offset = [], 2
    for _ in range(count):
        size, = struct.unpack_from('!i', body, offset)
        offset += 4
        if size < -1 or offset + max(size, 0) > len(body):
            raise ValueError('invalid native row length')
        values.append(None if size == -1 else body[offset:offset + size].decode('utf8'))
        offset += max(size, 0)
    if offset != len(body):
        raise ValueError('native row trailing data')
    return values


def read_exact(stream, size):
    chunks = bytearray()
    while len(chunks) < size:
        part = stream.recv(size - len(chunks))
        if not part:
            raise EOFError('native connection closed')
        chunks.extend(part)
    return bytes(chunks)


def message(stream):
    header = read_exact(stream, 5)
    size, = struct.unpack('!i', header[1:])
    if not 4 <= size <= 1024 * 1024:
        raise ValueError('invalid native message size')
    return header[:1], read_exact(stream, size - 4)


def startup(stream, user, database):
    body = struct.pack('!I', 196608) + b'user\0' + user.encode() + b'\0database\0' + database.encode() + b'\0\0'
    stream.sendall(struct.pack('!I', len(body) + 4) + body)
    while True:
        kind, body = message(stream)
        if kind == b'E':
            raise PGError(error_fields(body))
        if kind == b'R' and body != struct.pack('!I', 0):
            raise RuntimeError('unexpected authentication challenge')
        if kind == b'Z':
            return


def query(port, sql):
    with socket.create_connection(('127.0.0.1', port), timeout=3) as stream:
        startup(stream, 'oracle', 'pgbouncer')
        body = sql.encode() + b'\0'
        stream.sendall(b'Q' + struct.pack('!I', len(body) + 4) + body)
        rows = []
        while True:
            kind, body = message(stream)
            if kind == b'E':
                raise PGError(error_fields(body))
            if kind == b'D':
                rows.append(data_row(body))
            if kind == b'Z':
                return rows


def attested(target, error):
    return error.get('M') == 'authentication database "' + target + '" is disabled'


def enumerate_bindings(instance):
    """Sweep native database members, independently of source-scanner pairs.

    The database being probed stays enabled; every other database is disabled.
    An external authentication database is therefore named before backend SQL.
    Every response must have a disclosed classification, including no binding.
    """
    names = sorted({row[0] for row in query(instance.port, 'SHOW DATABASES')} - {'pgbouncer'})
    if any(not re.fullmatch('[A-Za-z0-9_-]+', name) for name in names):
        raise RuntimeError('native database name needs a quoted-name enumeration protocol')
    pairs, audit = [], []
    for name in names:
        query(instance.port, 'DISABLE ' + name)
    try:
        for owner in names:
            query(instance.port, 'ENABLE ' + owner)
            try:
                error = instance.probe(owner)
                message = error.get('M', '')
                match = re.fullmatch(r'authentication database "([^"]+)" is disabled', message)
                if match:
                    target = match.group(1)
                    if target not in names or target == owner:
                        raise RuntimeError('native target is outside enumerated local members')
                    pairs.append({'from': owner, 'to': target})
                    outcome = 'external-authentication-database'
                elif message in ('"trust" authentication failed', 'no such user'):
                    outcome = 'no-auth-user'
                elif message in ('query_wait_timeout', 'server login has been failing, try again later (server_login_retry)'):
                    outcome = 'no-external-authentication-database'
                else:
                    raise RuntimeError('unclassified native startup outcome: ' + json.dumps(error))
                audit.append(dict(source=owner, outcome=outcome, native_error=error))
            finally:
                query(instance.port, 'DISABLE ' + owner)
    finally:
        for name in names:
            query(instance.port, 'ENABLE ' + name)
    return pairs, audit


def mutate_authdb(source, owner, target, replacement):
    entries = scan(source).get('databases', {}).get('entries', [])
    matches = [(line, value) for key, line, value in entries if key == owner]
    if len(matches) != 1 or params(matches[0][1]).count(('auth_dbname', target)) != 1:
        raise ValueError('source auth_dbname site is not unique')
    lines = source.splitlines(keepends=True)
    line = matches[0][0] - 1
    lines[line], count = re.subn(r"(\bauth_dbname\s*=\s*)(?:'(?:''|[^'])*'|[^\s]+)",
                                lambda m: m.group(1) + replacement, lines[line])
    if count != 1:
        raise ValueError('ambiguous auth_dbname mutation')
    return ''.join(lines)


def isolate_backends(source, work):
    lines = source.splitlines(keepends=True)
    for _, no, value in scan(source).get('databases', {}).get('entries', []):
        raw = lines[no - 1]
        key = raw[:raw.index('=') + 1]
        items = [(k, v) for k, v in params(value) if k not in ('host', 'port')]
        items += [('host', str(work / 'no-backend')), ('port', '1')]
        lines[no - 1] = key + ' ' + ' '.join(k + "='" + v.replace("'", "''") + "'" for k, v in items) + '\n'
    return ''.join(lines)


class Instance:
    def __init__(self, binary, source, work):
        self.work = work.resolve()
        self.work.mkdir(parents=True, exist_ok=True)
        self.port = free_port()
        # Native logs are evidence: never accept a marker left by an earlier run.
        (self.work / 'native.log').write_text('')
        if (self.work / 'no-backend').exists():
            raise ValueError('isolated backend endpoint must not exist')
        (self.work / 'userlist.txt').write_text('"oracle" ""\n')
        config = sanitized(isolate_backends(source, self.work)) + '\n\n[pgbouncer]\n'
        settings = dict(listen_addr='127.0.0.1', listen_port=str(self.port), unix_socket_dir='',
                        auth_type='trust', auth_file=str(self.work / 'userlist.txt'), admin_users='oracle',
                        logfile=str(self.work / 'native.log'), pidfile=str(self.work / 'native.pid'),
                        query_wait_timeout='1', server_connect_timeout='1')
        config += ''.join(k + ' = ' + v + '\n' for k, v in settings.items())
        file = self.work / 'pgbouncer.ini'
        file.write_text(config)
        self.stderr = (self.work / 'stderr.log').open('w')
        self.process = subprocess.Popen([str(binary.resolve()), str(file)], stdout=self.stderr, stderr=self.stderr)

    def __enter__(self):
        try:
            for _ in range(50):
                if self.process.poll() is not None:
                    raise RuntimeError('native instance exited; inspect scratch stderr')
                try:
                    query(self.port, 'SHOW VERSION')
                    return self
                except (OSError, EOFError):
                    time.sleep(.1)
            raise RuntimeError('native instance did not become ready')
        except BaseException:
            self.__exit__(None, None, None)
            raise

    def __exit__(self, *_):
        self.process.terminate()
        try:
            self.process.wait(timeout=5)
        except subprocess.TimeoutExpired:
            self.process.kill()
            self.process.wait()
        self.stderr.close()

    def membership(self):
        # DISABLE registers a wildcard database without connecting to its backend.
        wildcard = False
        try:
            query(self.port, 'DISABLE zz_oracle_probe')
            wildcard = True
        except PGError:
            pass
        dbs = {r[0] for r in query(self.port, 'SHOW DATABASES')} - {'pgbouncer', 'zz_oracle_probe'}
        if wildcard:
            dbs.add('*')
        return (dbs, {r[0] for r in query(self.port, 'SHOW PEERS')},
                {r[0] for r in query(self.port, 'SHOW USERS')} - {'oracle', 'pgbouncer'},
                {r[0] for r in query(self.port, 'SHOW CONFIG')})

    def probe(self, owner):
        try:
            with socket.create_connection(('127.0.0.1', self.port), timeout=3) as stream:
                startup(stream, 'zz_oracle_unknown_user', owner)
        except PGError as error:
            return error.fields
        raise RuntimeError('probe did not produce a native error')

    def disabled_binding(self, owner, target):
        if not re.fullmatch('[A-Za-z0-9_-]+', target):
            raise ValueError('admin identifier needs a new quoted-name protocol')
        query(self.port, 'DISABLE ' + target)
        error = self.probe(owner)
        if not attested(target, error):
            raise RuntimeError('native binding disagreement: ' + json.dumps(error))
        return error
