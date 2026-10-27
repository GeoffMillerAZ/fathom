"""Bounded, scratch-relative syscall evidence; raw traces stay on the runner.

Observed write bytes count successful write-family syscall returns, not mmap
stores or allocation. Final sizes are a separate post-namespace snapshot. An
EFBIG alone is not attributed to RLIMIT_FSIZE: require SIGXFSZ on that thread.
"""
import os
import re
import stat
import signal
from itertools import islice
from pathlib import Path

MAX_FILES = 64
MAX_DIRECTORIES = 256
MAX_SNAPSHOT_ENTRIES = 4096
MAX_TRACE = 8 * 1024**2
MAX_INT = 2**63 - 1
CALLS = {'write', 'writev', 'pwrite64', 'pwritev', 'pwritev2', 'ftruncate',
         'truncate', 'fallocate', 'copy_file_range', 'sendfile', 'splice', 'execve'}


def path_text(value):
    # strace escapes control/non-ASCII bytes using C escapes. Never evaluate
    # trace text. Only allowlisted scratch-relative HOME/TMP paths may publish.
    def escape(match):
        token = match.group(1)
        if re.fullmatch(rb'[0-7]{1,3}', token):
            return bytes([int(token, 8)])
        known = {b'n': b'\n', b'r': b'\r', b't': b'\t', b'\\': b'\\', b'"': b'"'}
        if token not in known: raise ValueError('unknown path escape')
        return known[token]
    raw = value.encode('utf-8', 'surrogateescape')
    return re.sub(rb'\\([0-7]{1,3}|.)', escape, raw).decode('utf-8', 'surrogateescape')


def arguments(text):
    rows, start, depth, quoted, escaped, fd = [], 0, 0, False, False, False
    for index, char in enumerate(text):
        if escaped:
            escaped = False
        elif char == '\\':
            escaped = True
        elif quoted:
            if char == '"': quoted = False
        elif char == '"': quoted = True
        elif fd:
            if char == '>': fd = False
        elif char == '<': fd = True
        elif char in '[{(': depth += 1
        elif char in ']})': depth -= 1
        elif char == ',' and depth == 0:
            rows.append(text[start:index].strip()); start = index + 1
    if quoted or fd or depth or escaped:
        raise ValueError('unparsed syscall arguments')
    return rows + [text[start:].strip()]


def diagnostics(trace, root, executable=None):
    if len(trace.encode('utf-8', 'surrogateescape')) > MAX_TRACE:
        raise ValueError('trace cap exceeded')
    root = Path(root).absolute()
    rows, pending, failed, correlated = {}, {}, {}, []
    sigxfsz = False
    omitted_events, snapshot_truncated = 0, False
    largest = None
    executable = str(executable or root/'payload'/'PolicyLanguageServer')
    native_pid, native_returncode = None, None

    def identity(path):
        nonlocal omitted_events
        path = path_text(path)
        deleted = path.endswith(' (deleted)')
        if deleted: path = path[:-10]
        canonical = Path(os.path.normpath(path))
        area, relative, public_path, kind = 'other', None, None, 'generic'
        if path == '/memfd:doublemapper': kind = 'dotnet_doublemapper'
        if path.startswith('/memfd:'): deleted = True
        for candidate,base in [('tmp',root/'home'/'tmp'),('home',root/'home')]:
            try:
                canonical.relative_to(base)
                area = candidate
                relative = canonical.relative_to(root).as_posix()
                break
            except ValueError: pass
        if relative is not None:
            if (len(relative)<=512 and all(part not in ('','.','..') and
                    re.fullmatch(r'[A-Za-z0-9_.-]{1,128}',part) for part in relative.split('/'))):
                public_path = relative
        # Only HOME/TMP have per-path rows. Every other target collapses into a
        # fixed class. No path hash, salt, PID, absolute root or memfd label leaves.
        key = (area,relative if relative is not None else kind)
        if key in rows:
            row=rows[key];row['unlinked'] |= deleted
            return row
        row=dict(area=area,relative_path=public_path,kind=kind,observed_write_bytes=0,
                 largest_requested_extent_bytes=None,exceeds_previous_limit=False,
                 final_size_bytes=None,final_type='absent',unlinked=deleted)
        if len(rows)<MAX_FILES: rows[key]=row
        else: omitted_events+=1
        return row

    def target(arg):
        if re.fullmatch(r'-?\d+',arg): raise ValueError('file descriptor path unavailable')
        match=re.fullmatch(r'-?\d+<((?:\\.|[^>])*)>(\(deleted\))?',arg)
        if match:
            name=match[1]
            if name.startswith(('pipe:[','socket:[','anon_inode:','TCP:','UDP:','UNIX','NETLINK:')): return None,None
            if not name.startswith('/'): raise ValueError('unknown descriptor kind')
            name += ' (deleted)' if match[2] else ''
        elif arg.startswith('"') and arg.endswith('"'):
            name=arg[1:-1]  # relative cwd is unknown: fixed other class, not guessed.
        else: raise ValueError('undecoded syscall target')
        # The private key is only for distinct EFBIG targets. It is never encoded.
        return identity(name),path_text(name)

    for line in trace.splitlines():
        match = re.fullmatch(r'(?:\[pid\s+)?(\d+)(?:\])?\s+(.*)', line)
        if not match: raise ValueError('unparsed trace line')
        pid, body = match.groups()
        if body.startswith('--- SIGXFSZ '):
            sigxfsz = True
            if pid in failed: correlated.append(failed.pop(pid))
            continue
        if body.startswith('+++ '):
            if pid == native_pid:
                exited = re.fullmatch(r'\+\+\+ exited with (\d+) \+\+\+', body)
                killed = re.fullmatch(r'\+\+\+ killed by (SIG[A-Z0-9]+)(?: \(core dumped\))? \+\+\+', body)
                if exited: native_returncode = int(exited[1])
                elif killed and hasattr(signal,killed[1]): native_returncode = -int(getattr(signal,killed[1]))
                else: raise ValueError('unparsed native exit')
            continue
        if body.startswith('--- '): continue
        resumed = re.match(r'<\.\.\. (\w+) resumed>(.*)', body)
        if resumed:
            if pid not in pending or pending[pid][0] != resumed[1]:
                raise ValueError('unmatched resumed syscall')
            _, prefix = pending.pop(pid); body = prefix + resumed[2]
        elif '<unfinished ...>' in body:
            call = re.match(r'(\w+)\(', body)
            if not call or call[1] not in CALLS or pid in pending:
                raise ValueError('unmatched unfinished syscall')
            pending[pid] = (call[1], body.removesuffix('<unfinished ...>')); continue
        call = re.fullmatch(r'(\w+)\((.*)\)\s+=\s+(-?\d+|\?)(?=\s|$)(.*)', body)
        if not call or call[1] not in CALLS: raise ValueError('unparsed watched syscall')
        name, args, result, tail = call.groups()
        args = arguments(args)
        if name == 'execve':
            if args[0].startswith('"') and args[0].endswith('"'):
                if path_text(args[0][1:-1]) == executable and result == '0' and native_pid is None: native_pid = pid
            else: raise ValueError('unparsed exec target')
            continue
        output = {'copy_file_range': 2, 'sendfile': 0, 'splice': 2}.get(name, 0)
        if output >= len(args): raise ValueError('syscall argument absent')
        if result == '-1' and re.search(r'\bEBADF\b',tail): continue
        row, private_key = target(args[output])
        if row is None:
            if 'EFBIG' in tail: raise ValueError('EFBIG target unavailable')
            continue
        count = int(result) if result != '?' else -1
        if count > MAX_INT: raise ValueError('syscall byte cap exceeded')
        extent = None
        if name in ('ftruncate', 'truncate', 'fallocate'):
            values = args[1:] if name != 'fallocate' else args[2:]
            if any(not re.fullmatch(r'\d+', value) for value in values):
                raise ValueError('unparsed allocation extent')
            extent = sum(map(int, values))
        elif name in ('pwrite64', 'pwritev', 'pwritev2'):
            offset = args[3]
            if not re.fullmatch(r'\d+', offset): raise ValueError('unparsed write offset')
            extent = int(offset) + (int(args[2]) if name == 'pwrite64' else max(count, 0))
        if extent is not None and extent > MAX_INT: raise ValueError('extent cap exceeded')
        if extent is not None:
            row['largest_requested_extent_bytes']=max(row['largest_requested_extent_bytes'] or 0,extent)
        if count >= 0 and name not in ('ftruncate', 'truncate', 'fallocate'):
            row['observed_write_bytes'] += count
            if row['observed_write_bytes'] > MAX_INT: raise ValueError('byte sum cap exceeded')
        if count == -1 and re.search(r'\bEFBIG\b', tail):
            failed[pid] = (private_key,dict(area=row['area'],relative_path=row['relative_path'],kind=row['kind'],
                               operation=name,requested_extent_bytes=extent))
        else: failed.pop(pid, None)
    if pending: raise ValueError('unfinished trace refused')

    # Namespace is dead before this snapshot. lstat/scandir never follow a link.
    home = root/'home'
    if home.is_symlink(): raise ValueError('private HOME symlink refused')
    directories = [home] if home.exists() else []
    visited = 0
    snapshot_entries = 0
    while directories:
        directory = directories.pop(); visited += 1
        if visited > MAX_DIRECTORIES:
            snapshot_truncated=True;break
        with os.scandir(directory) as entries:
            bounded_entries=sorted(islice(entries,max(0,MAX_SNAPSHOT_ENTRIES-snapshot_entries)+1),key=lambda item:item.name)
            for entry in bounded_entries:
                snapshot_entries += 1
                if snapshot_entries > MAX_SNAPSHOT_ENTRIES:
                    snapshot_truncated=True;directories.clear();break
                info = entry.stat(follow_symlinks=False)
                if stat.S_ISDIR(info.st_mode):
                    if visited+len(directories)>=MAX_DIRECTORIES:
                        snapshot_truncated=True;continue
                    directories.append(Path(entry.path)); continue
                row = identity(entry.path)
                row['final_size_bytes'] = info.st_size if stat.S_ISREG(info.st_mode) else None
                row['final_type'] = 'regular' if stat.S_ISREG(info.st_mode) else 'symlink' if stat.S_ISLNK(info.st_mode) else 'other'
                row['exceeds_previous_limit']=max(row['final_size_bytes'] or 0,row['largest_requested_extent_bytes'] or 0)>8*1024**2
                if row['final_type']=='regular' and (largest is None or (info.st_size,row['relative_path'] or '')>(largest['final_size_bytes'],largest['relative_path'] or '')):
                    largest=dict(row)
    for row in rows.values():
        row['exceeds_previous_limit']=max(row['final_size_bytes'] or 0,row['largest_requested_extent_bytes'] or 0)>8*1024**2
    unique = dict(correlated)
    return dict(files=sorted(rows.values(),key=lambda r:(r['area'],r['relative_path'] or '',r['kind'],r['final_type'],r['final_size_bytes'] or 0,r['observed_write_bytes'])),
                largest_private_file=largest,files_over_previous_limit=sum(r['exceeds_previous_limit'] for r in rows.values()),
                files_truncated=omitted_events>0,omitted_file_events=omitted_events,snapshot_truncated=snapshot_truncated,
                sigxfsz_observed=sigxfsz,limit_file=next(iter(unique.values())) if len(unique)==1 else None,
                correlated_limit_files=len(unique), vendor_exec_observed=native_pid is not None,
                native_returncode=native_returncode)
