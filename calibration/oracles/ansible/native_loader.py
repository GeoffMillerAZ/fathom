"""Offline ansible-core loader instrument. Raw evidence, never a support grade.

The supervising caller must hold the language measurement lease. Each unit runs
in a bounded macOS sandbox; no inventory or automation execution API is called.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import resource
import subprocess
import sys
import tempfile
from types import SimpleNamespace

MAX_OBJECTS = 10000
MAX_DEPTH = 64
MAX_INPUT_BYTES = 2 * 1024 * 1024
MAX_OUTPUT_BYTES = 8 * 1024 * 1024


class Refused(ValueError):
    pass


def population(root):
    # Caller freezes tracked file names before reading; fixtures can use this
    # filesystem population. Refuse links instead of following external content.
    result = []
    for p in sorted(root.rglob('*')):
        if '.git' in p.relative_to(root).parts:
            continue
        if p.is_symlink():
            raise Refused('symlink population requires explicit scope')
        if p.is_file() and p.suffix in ('.yaml', '.yml'):
            result.append(p.relative_to(root).as_posix())
    return result


def sandbox_profile(work):
    # Plugins can read only this checkout, their Python runtime, and OS files;
    # never the user's credentials. Persistent writes only under private work.
    lane = Path(__file__).resolve().parents[3]
    runtime = Path(sys.base_prefix).resolve()
    quote = lambda p: json.dumps(str(p))
    return f'''(version 1)
(allow default)
(deny network*)
(deny process-fork)
(deny file-write* (require-all (require-not (subpath {quote(work)}))
 (require-not (literal "/dev/null"))))
(deny file-read-data (require-all (subpath "/Users")
 (require-not (subpath {quote(lane)}))
 (require-not (subpath {quote(runtime)}))))
'''


def private_env(work, roles=None, collections=None):
    work.mkdir(parents=True, exist_ok=True)
    config = work / 'ansible.cfg'
    config.write_text('[defaults]\nretry_files_enabled=False\n')
    # Do not pass inherited credentials, inventory, plugin or Python settings.
    return dict(PATH=str(Path(sys.executable).parent) + ':/usr/bin:/bin',
                HOME=str(work), TMPDIR=str(work), XDG_CACHE_HOME=str(work / 'cache'),
                LC_ALL='en_US.UTF-8', LANG='en_US.UTF-8', PYTHONDONTWRITEBYTECODE='1',
                ANSIBLE_HOME=str(work / 'ansible'), ANSIBLE_CONFIG=str(config),
                ANSIBLE_LOCAL_TEMP=str(work / 'tmp'), ANSIBLE_REMOTE_TEMP=str(work / 'remote'),
                ANSIBLE_ROLES_PATH=str(roles or work / 'roles'),
                ANSIBLE_COLLECTIONS_PATH=str(collections or work / 'collections'),
                ANSIBLE_GALAXY_CACHE_DIR=str(work / 'galaxy'),
                ANSIBLE_INVENTORY_ENABLED='ini', ANSIBLE_NOCOLOR='1')


def limits():
    resource.setrlimit(resource.RLIMIT_CPU, (20, 20))
    resource.setrlimit(resource.RLIMIT_FSIZE, (MAX_OUTPUT_BYTES, MAX_OUTPUT_BYTES))
    resource.setrlimit(resource.RLIMIT_NOFILE, (128, 128))


def bounded(command, work, env):
    if not Path('/usr/bin/sandbox-exec').exists():
        raise Refused('required sandbox-exec unavailable; no uncontained fallback')
    profile = work / 'sandbox.sb'
    profile.write_text(sandbox_profile(work))
    with tempfile.TemporaryFile(dir=work) as out, tempfile.TemporaryFile(dir=work) as err:
        try:
            p = subprocess.run(['/usr/bin/sandbox-exec', '-f', str(profile), *command],
                               cwd=work, env=env, stdout=out, stderr=err,
                               timeout=30, preexec_fn=limits)
        except subprocess.TimeoutExpired as exc:
            raise Refused('unit timeout') from exc
        out.seek(0); err.seek(0)
        output, errors = out.read(MAX_OUTPUT_BYTES + 1), err.read(MAX_OUTPUT_BYTES + 1)
        if len(output) > MAX_OUTPUT_BYTES or len(errors) > MAX_OUTPUT_BYTES:
            raise Refused('output cap')
        if p.returncode:
            raise Refused('sandbox child failed: ' + errors.decode(errors='replace')[-2000:])
        try:
            return json.loads(output)
        except (ValueError, UnicodeError, RecursionError) as exc:
            raise Refused('malformed child JSON') from exc


def empty(status, reason):
    return dict(status=status, reason=reason, declarations=[], bindings=[], unresolved=[])


def run_unit(root, path, mode, work, max_objects=MAX_OBJECTS, roles=None, collections=None):
    work = work.resolve()
    try:
        if mode not in ('playbook', 'role', 'tasks', 'handlers'):
            raise Refused('unknown loader mode')
        target = checked_path(root, path, allow_root=mode == 'role')
        root = root.resolve()
        if target.is_file() and target.stat().st_size > MAX_INPUT_BYTES:
            raise Refused('input cap')
        env = private_env(work, roles, collections)
        command = [sys.executable, str(Path(__file__).resolve()), '--child', str(root), path, mode, str(max_objects)]
        return validate_result(bounded(command, work, env), max_objects)
    except Refused as exc:
        return empty('refused', str(exc))


def containment_probe(work, witness=None):
    work = work.resolve()
    env = private_env(work)
    # Probe the three independently enforced sandbox boundaries, no remote peer.
    code = '''import json, socket, sys, os
from pathlib import Path
r = {}
try:
 s=socket.socket(); s.connect(('127.0.0.1',9)); r['network_blocked']=False
except PermissionError: r['network_blocked']=True
try:
 Path('/Users/ansible-oracle-probe').write_text('forbidden'); r['write_blocked']=False
except PermissionError: r['write_blocked']=True
try:
 Path(sys.argv[1]).read_bytes(); r['read_blocked']=False
except PermissionError: r['read_blocked']=True
try:
 pid=os.fork()
 if pid == 0: os._exit(0)
 os.waitpid(pid,0); r['fork_blocked']=False
except PermissionError: r['fork_blocked']=True
print(json.dumps(r))
'''
    # Only an explicitly supplied, caller-owned synthetic canary is read.
    # It must exercise the general boundary, outside both allowed roots.
    witness = Path(witness or os.environ['FATHOM_ANSIBLE_OUTSIDE_CANARY']).resolve()
    lane = Path(__file__).resolve().parents[3]
    if (not witness.is_relative_to(Path.home() / 'fathom-work') or
            witness.is_relative_to(lane) or witness.is_relative_to(Path(sys.base_prefix).resolve())):
        raise Refused('read canary must be owned scratch outside allowed roots')
    return bounded([sys.executable, '-c', code, str(witness)], work, env)


def native(root, path, mode, cap):
    from ansible.parsing.dataloader import DataLoader
    from ansible.playbook import Playbook
    from ansible.playbook.play import Play
    from ansible.playbook.block import Block
    from ansible.playbook.role import Role
    from ansible.playbook.role.include import RoleInclude
    from ansible.playbook.helpers import load_list_of_blocks
    from ansible.plugins.loader import init_plugin_loader
    from ansible.plugins.strategy import StrategyBase
    from ansible.vars.manager import VariableManager
    init_plugin_loader()
    loader = DataLoader()
    loader.set_basedir(str((root / path).parent))
    vm = VariableManager(loader=loader, inventory=None)
    declarations, bindings, unresolved = [], [], []
    objects = 0
    synthetic = 0

    def position(obj):
        loc = obj.get_path()
        filename, sep, line = loc.rpartition(':')
        if not sep or not line.isdecimal():
            raise Refused('native object lacks position')
        p = Path(filename).resolve()
        # Dependency declarations remain native truth but outside subject metrics.
        return dict(path=os.path.relpath(p, root), line=int(line), external=not p.is_relative_to(root))

    def edge(rows, value):
        # Bound every append, including duplicates, before final identity dedup.
        if len(bindings) + len(unresolved) >= cap:
            raise Refused('edge cap')
        rows.append(value)

    def declare(obj, kind, name):
        nonlocal objects
        objects += 1
        if objects > cap:
            raise Refused('object cap')
        d = dict(kind=kind, name=str(name), **position(obj))
        declarations.append(d)
        if '{{' in d['name'] or '{%' in d['name']:
            edge(unresolved, dict(source=d, reason='templated declaration'))
        return d

    def walk(blocks, play, handler=False, depth=0):
        nonlocal synthetic
        if depth > MAX_DEPTH:
            raise Refused('nesting cap')
        for obj in blocks:
            if isinstance(obj, Block):
                for section in ('block', 'rescue', 'always'):
                    walk(getattr(obj, section), play, handler, depth + 1)
                continue
            if obj.implicit:
                synthetic += 1
                continue
            source = declare(obj, 'handler' if handler else 'task', obj.name or obj.action)
            action = obj.action.split('.')[-1]
            if action in ('include_tasks', 'include_role', 'include_vars'):
                # Includes need runtime vars/loops/conditions. Never count visited-
                # only output as complete or turn includes into execution.
                edge(unresolved, dict(source=source, reason='unresolved dynamic ' + action))
            notices = obj.notify or []
            if isinstance(notices, str):
                notices = [notices]
            for notice in notices:
                if '{{' in notice or '{%' in notice:
                    edge(unresolved, dict(source=source, reason='templated notify', notification=notice))
                    continue
                strategy = object.__new__(StrategyBase)
                strategy._variable_manager = vm
                strategy._hosts_cache = []
                strategy._hosts_cache_all = []
                found = list(strategy.search_handlers_by_notification(notice, SimpleNamespace(_play=play)))
                if not found:
                    edge(unresolved, dict(source=source, reason='unbound notify', notification=notice))
                for target in found:
                    edge(bindings, dict(source=source, notification=notice, target=position(target)))

    try:
        if mode == 'playbook':
            plays = Playbook.load(str(root / path), variable_manager=vm, loader=loader).get_plays()
            for play in plays:
                declare(play, 'play', play.name or play.hosts)
                play.handlers = play.compile_roles_handlers() + play.handlers
                for role in play.roles:
                    # Native loader has already resolved roles and dependencies.
                    walk(role.compile(play), play)
                walk(play.handlers, play, True)
                # Handler order is core's reverse-search contract.
                for section in ('pre_tasks', 'tasks', 'post_tasks'):
                    walk(getattr(play, section), play)
        else:
            play = Play.load(dict(hosts='all'), variable_manager=vm, loader=loader)
            if mode == 'role':
                include = RoleInclude.load(str(root / path), play=play, variable_manager=vm, loader=loader)
                role = Role.load(include, play=play)
                play.handlers = role.get_handler_blocks(play)
                walk(play.handlers, play, True)
                walk(role.compile(play), play)
            else:
                data = loader.load_from_file(str(root / path))
                blocks = load_list_of_blocks(data, play=play, use_handlers=mode == 'handlers',
                                             variable_manager=vm, loader=loader)
                if mode == 'handlers':
                    play.handlers = blocks
                walk(blocks, play, mode == 'handlers')
        key = lambda v: json.dumps(v, sort_keys=True)
        # Deduplicate by native identity, not by fathom name/scope.
        unique = lambda xs: [json.loads(s) for s in sorted({key(v) for v in xs})]
        return dict(status='loaded', declarations=unique(declarations),
                    bindings=unique(bindings), unresolved=unique(unresolved), synthetic_objects=synthetic)
    except Refused as exc:
        return empty('refused', str(exc))
    except Exception as exc:
        # A rejection is data. Never retain a partial unit's apparent success.
        return empty('rejected', type(exc).__name__ + ': ' + str(exc))


def checked_path(root, path, allow_root=False):
    """Reject links in consumed paths before resolving or launching a child."""
    root = Path(root).absolute()
    path = Path(path)
    if path.is_absolute() or '..' in path.parts or (str(path) == '.' and not allow_root):
        raise Refused('input outside subject or noncanonical path')
    target = root / path
    for component in [target, *target.parents]:
        if component.is_symlink():
            raise Refused('symlink input requires explicit scope')
    if not target.resolve().is_relative_to(root.resolve()):
        raise Refused('input outside subject')
    if not target.exists():
        raise Refused('input missing')
    return target


def frozen_row(rows, subject, sha=None):
    if not isinstance(rows, list) or any(not isinstance(r, dict) for r in rows):
        raise Refused('malformed frozen population')
    if not isinstance(subject, str) or not subject:
        raise Refused('malformed subject identity')
    matches = [r for r in rows if r.get('subject') == subject]
    if len(matches) != 1:
        raise Refused('missing or duplicate frozen subject')
    row = matches[0]
    files = row.get('yaml')
    identity = row.get('sha')
    if (not isinstance(identity, str) or len(identity) != 40 or
            any(c not in '0123456789abcdef' for c in identity) or
            (sha is not None and sha != identity)):
        raise Refused('frozen subject identity mismatch')
    if not isinstance(files, list) or any(not isinstance(f, str) for f in files):
        raise Refused('malformed frozen YAML inventory')
    if files != sorted(set(files)):
        raise Refused('frozen inventory must be sorted and unique')
    for f in files:
        path = Path(f)
        if (path.is_absolute() or '..' in path.parts or path.as_posix() != f or
                path.suffix not in ('.yml', '.yaml')):
            raise Refused('invalid frozen YAML path')
    return row


def expected_units(row):
    roles = sorted({str(Path(f).parent.parent) for f in row['yaml']
                    if Path(f).name in ('main.yml', 'main.yaml') and Path(f).parent.name == 'tasks'})
    return ([(f, mode) for f in row['yaml'] for mode in ('playbook', 'tasks', 'handlers')] +
            [(role, 'role') for role in roles])


def validate_result(value, cap=MAX_OBJECTS, unit_fields=False):
    """Validate the complete child envelope; never admit a partial refusal."""
    def fail():
        raise Refused('malformed child result schema')
    def position(p):
        if (not isinstance(p, dict) or not isinstance(p.get('path'), str) or not p['path'] or
                type(p.get('line')) is not int or p['line'] < 1 or type(p.get('external')) is not bool):
            fail()
    def declaration(d):
        position(d)
        if d.get('kind') not in ('task', 'play', 'handler') or not isinstance(d.get('name'), str):
            fail()
    if not isinstance(value, dict) or value.get('status') not in ('loaded', 'rejected', 'refused'):
        fail()
    allowed = {'status', 'reason', 'declarations', 'bindings', 'unresolved', 'synthetic_objects'}
    if unit_fields: allowed.update(('input', 'mode'))
    if set(value) - allowed:
        fail()
    for key in ('declarations', 'bindings', 'unresolved'):
        if not isinstance(value.get(key), list):
            fail()
    if len(value['declarations']) > cap or len(value['bindings']) + len(value['unresolved']) > cap:
        raise Refused('child result count cap')
    if value['status'] != 'loaded':
        if (any(value[k] for k in ('declarations', 'bindings', 'unresolved')) or
                not isinstance(value.get('reason'), str)):
            fail()
    if 'synthetic_objects' in value and (type(value['synthetic_objects']) is not int or value['synthetic_objects'] < 0):
        fail()
    for d in value['declarations']:
        declaration(d)
    for b in value['bindings']:
        if not isinstance(b, dict): fail()
        declaration(b.get('source')); position(b.get('target'))
        if not isinstance(b.get('notification'), str): fail()
    for u in value['unresolved']:
        if not isinstance(u, dict): fail()
        declaration(u.get('source'))
        if not isinstance(u.get('reason'), str): fail()
        if u['reason'] in ('templated notify', 'unbound notify') and not isinstance(u.get('notification'), str): fail()
    return value


def prepare_output(out, inputs=(), directories=()):
    """Invalidate only an owned output, after protecting all caller inputs."""
    out = Path(out).absolute()
    if any(p.is_symlink() for p in (out, *out.parents)):
        raise Refused('output symlink alias')
    resolved = out.resolve()
    for source in inputs:
        if source is None: continue
        source = Path(source)
        if resolved == source.resolve() or (out.exists() and source.exists() and out.samefile(source)):
            raise Refused('output aliases caller input')
    for directory in directories:
        if directory is not None and resolved.is_relative_to(Path(directory).resolve()):
            raise Refused('output inside caller input directory')
    if out.exists():
        if out.stat().st_nlink != 1:
            raise Refused('output hardlink alias')
        out.unlink()
    return out


def write_output(out, document):
    # Atomic publication keeps a failed write from leaving a partial JSON artifact.
    payload = json.dumps(document, indent=2, sort_keys=True) + '\n'
    with tempfile.NamedTemporaryFile(mode='w', dir=out.parent, prefix='.ansible-output-', delete=False) as f:
        temporary = Path(f.name)
        try:
            f.write(payload)
            f.flush()
            os.replace(temporary, out)
        finally:
            if temporary.exists(): temporary.unlink()


def main():
    if len(sys.argv) > 1 and sys.argv[1] == '--child':
        print(json.dumps(native(Path(sys.argv[2]), sys.argv[3], sys.argv[4], int(sys.argv[5])), sort_keys=True))
        return
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--subject', type=Path, required=True)
    p.add_argument('--population', type=Path, required=True)
    p.add_argument('--work', type=Path, required=True)
    p.add_argument('--out', type=Path, required=True)
    p.add_argument('--roles', type=Path)
    p.add_argument('--collections', type=Path)
    args = p.parse_args()
    out = prepare_output(args.out, [args.population], [args.subject, args.roles, args.collections])
    rows = json.loads(args.population.read_text())
    if any(p.is_symlink() for p in (args.subject.absolute(), *args.subject.absolute().parents)):
        raise Refused('symlink subject requires explicit scope')
    root = args.subject.resolve()
    row = frozen_row(rows, root.name)
    sha = subprocess.check_output(['git', '-C', str(root), 'rev-parse', 'HEAD'], text=True).strip()
    frozen_row(rows, root.name, sha)
    results = []
    for i, path in enumerate(row['yaml']):
        modes = ['playbook', 'tasks', 'handlers']
        for mode in modes:
            r = run_unit(root, path, mode, args.work / str(i) / mode,
                         roles=args.roles, collections=args.collections)
            results.append(dict(input=path, mode=mode, **r))
    # Native Role.load separately covers dependencies, role vars and handlers.
    roles = sorted({str(Path(f).parent.parent) for f in row['yaml']
                    if Path(f).name in ('main.yml', 'main.yaml') and Path(f).parent.name == 'tasks'})
    for i, role in enumerate(roles):
        results.append(dict(input=role, mode='role', **run_unit(root, role, 'role', args.work / ('role'+str(i)),
                                                             roles=args.roles, collections=args.collections)))
    document = dict(subject=root.name, sha=sha, yaml_candidates=len(row['yaml']), role_candidates=len(roles),
                    tasks_executed=False, results=results)
    write_output(out, document)


if __name__ == '__main__':
    main()
