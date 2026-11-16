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
(deny file-read-data (literal {quote(work / "private-read-witness")}))
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
        return json.loads(output)


def empty(status, reason):
    return dict(status=status, reason=reason, declarations=[], bindings=[], unresolved=[])


def run_unit(root, path, mode, work, max_objects=MAX_OBJECTS, roles=None, collections=None):
    root, work = root.resolve(), work.resolve()
    if mode not in ('playbook', 'role', 'tasks', 'handlers'):
        raise Refused('unknown loader mode')
    target = (root / path).resolve()
    if not target.is_relative_to(root):
        raise Refused('input outside subject')
    if target.is_file() and target.stat().st_size > MAX_INPUT_BYTES:
        return empty('refused', 'input cap')
    env = private_env(work, roles, collections)
    command = [sys.executable, str(Path(__file__).resolve()), '--child', str(root), path, mode, str(max_objects)]
    try:
        return bounded(command, work, env)
    except Refused as exc:
        return empty('refused', str(exc))


def containment_probe(work):
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
    witness = work / 'private-read-witness'
    witness.write_text('authored containment witness')
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

    def declare(obj, kind, name):
        nonlocal objects
        objects += 1
        if objects > cap:
            raise Refused('object cap')
        d = dict(kind=kind, name=str(name), **position(obj))
        declarations.append(d)
        if '{{' in d['name'] or '{%' in d['name']:
            unresolved.append(dict(source=d, reason='templated declaration'))
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
                unresolved.append(dict(source=source, reason='unresolved dynamic ' + action))
            notices = obj.notify or []
            if isinstance(notices, str):
                notices = [notices]
            for notice in notices:
                if '{{' in notice or '{%' in notice:
                    unresolved.append(dict(source=source, reason='templated notify'))
                    continue
                strategy = object.__new__(StrategyBase)
                strategy._variable_manager = vm
                strategy._hosts_cache = []
                strategy._hosts_cache_all = []
                found = list(strategy.search_handlers_by_notification(notice, SimpleNamespace(_play=play)))
                if not found:
                    unresolved.append(dict(source=source, reason='unbound notify', notification=notice))
                for target in found:
                    bindings.append(dict(source=source, notification=notice, target=position(target)))
                if len(bindings) + len(unresolved) > cap:
                    raise Refused('edge cap')

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
    rows = json.loads(args.population.read_text())
    root = args.subject.resolve()
    row = next(r for r in rows if r['subject'] == root.name)
    sha = subprocess.check_output(['git', '-C', str(root), 'rev-parse', 'HEAD'], text=True).strip()
    if sha != row['sha']:
        raise Refused('frozen subject identity mismatch')
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
    args.out.write_text(json.dumps(document, indent=2, sort_keys=True)+'\n')


if __name__ == '__main__':
    main()
