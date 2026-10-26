"""Measure native INI inventory and lint loader candidates without running tasks.

All per-file inventories, task names and diagnostics remain scratch artifacts.
The summary contains counts and hashes only; no semantic reading is rewritten.
"""
import argparse
from collections import Counter
import hashlib
import importlib.metadata
import json
import locale
import os
from pathlib import Path
import subprocess
import sys


def utf8_locale():
    for name in ('C.UTF-8', 'en_US.UTF-8'):
        try:
            locale.setlocale(locale.LC_CTYPE, name)
            return name
        except locale.Error:
            pass
    raise RuntimeError('Ansible requires an available UTF-8 locale')


def digest(value):
    return hashlib.sha256(json.dumps(value, sort_keys=True).encode()).hexdigest()


def inventory_candidate(path):
    return path.suffix.lower() == '.ini' or (not path.suffix and path.name in ('inventory', 'hosts'))


def inventory_counts(document):
    groups = {k: v for k, v in document.items() if k != '_meta'}
    hosts = set(document.get('_meta', {}).get('hostvars', {}))
    for group in groups.values():
        hosts.update(group.get('hosts', []))
    return dict(groups=len(groups), hosts=len(hosts),
                children=sum(len(g.get('children', [])) for g in groups.values()),
                memberships=sum(len(g.get('hosts', [])) for g in groups.values()))


def measure(subject, work):
    from ansiblelint.app import App
    from ansiblelint.config import Options
    from ansiblelint.file_utils import Lintable
    from ansiblelint.rules import RulesCollection
    from ansiblelint.runner import Runner
    from ansiblelint.utils import is_playbook, parse_yaml_linenumbers, task_in_list
    from ansible.plugins.loader import init_plugin_loader
    init_plugin_loader()
    options = Options(project_dir=str(work), cwd=work, offline=True)
    app = App(options)
    runner = Runner(rules=RulesCollection(app=app, options=options), project_dir=str(subject))
    inventory, lint = [], []
    binary = Path(sys.executable).parent / 'ansible-inventory'
    for path in sorted(subject.rglob('*')):
        if not path.is_file() or '.git' in path.relative_to(subject).parts:
            continue
        rel = path.relative_to(subject).as_posix()
        if inventory_candidate(path):
            result = subprocess.run([str(binary), '-i', str(path), '--list'],
                                    cwd=work, capture_output=True, text=True, timeout=60)
            record = dict(path=rel, source_sha256=hashlib.sha256(path.read_bytes()).hexdigest(),
                          returncode=result.returncode, stdout=result.stdout, stderr=result.stderr)
            if result.returncode == 0:
                record['counts'] = inventory_counts(json.loads(result.stdout))
            inventory.append(record)
        if path.suffix.lower() not in ('.yml', '.yaml'):
            continue
        record = dict(path=rel, tasks=[], children=[], errors=[])
        try:
            file = Lintable(path)
            if file.kind == 'yaml' and is_playbook(str(path)):
                file.kind = 'playbook'
            record['kind'] = file.kind
            if file.kind not in ('playbook', 'tasks', 'handlers'):
                continue
            data = parse_yaml_linenumbers(file)
            for task in task_in_list(data, file, file.kind):
                item = dict(line=task.line, unnamed=not bool(task.name))
                try:
                    item['module'] = task.action
                    item['normalization_error'] = bool(task.error)
                except Exception as error:
                    item['normalization_error'] = type(error).__name__
                record['tasks'].append(item)
            for child in runner.find_children(file):
                candidate = child.path.resolve()
                record['children'].append(dict(path=os.path.relpath(candidate, subject), exists=candidate.exists()))
        except Exception as error:
            record['errors'].append(dict(type=type(error).__name__, message=str(error)))
        lint.append(record)
    return dict(inventory=inventory, lint=lint)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('-subjects', type=Path, required=True)
    parser.add_argument('-work', type=Path, required=True)
    parser.add_argument('-out', type=Path, required=True)
    args = parser.parse_args()
    work = args.work.resolve(); work.mkdir(parents=True, exist_ok=True)
    # Set before importing Ansible: every cache, config and plugin setting is private.
    private = work / 'home'; private.mkdir(exist_ok=True)
    config = work / 'ansible.cfg'; config.write_text('[defaults]\nretry_files_enabled=False\n')
    os.environ.update(LC_ALL=utf8_locale(), LC_CTYPE=utf8_locale(), ANSIBLE_HOME=str(private), ANSIBLE_CONFIG=str(config),
                      ANSIBLE_LOCAL_TEMP=str(work / 'tmp'),
                      ANSIBLE_INVENTORY_UNPARSED_IS_FAILED='true',
                      ANSIBLE_INVENTORY_ANY_UNPARSED_IS_FAILED='true',
                      ANSIBLE_GALAXY_CACHE_DIR=str(work / 'galaxy-cache'),
                      ANSIBLE_ROLES_PATH=str(work / 'roles'),
                      ANSIBLE_COLLECTIONS_PATH=str(work / 'collections'))
    os.environ['PATH'] = str(Path(sys.executable).parent) + os.pathsep + os.environ['PATH']
    import yaml
    shard = yaml.safe_load(Path('calibration/corpus.d/ansible.yaml').read_text())
    raw, summaries = [], []
    for repo in shard['repos']:
        subject = (args.subjects / repo['name'].split('/')[-1]).resolve()
        sha = subprocess.check_output(['git', '-C', str(subject), 'rev-parse', 'HEAD'], text=True).strip()
        if sha != repo['sha'] or subprocess.check_output(['git', '-C', str(subject), 'status', '--porcelain'], text=True).strip():
            raise RuntimeError('subject identity or cleanliness mismatch')
        report = measure(subject, work)
        raw.append(dict(subject=repo['name'], **report))
        inv, lint = report['inventory'], report['lint']
        tasks = [t for f in lint for t in f['tasks']]
        children = [c for f in lint for c in f['children']]
        summary = dict(subject=repo['name'], sha=sha, inventory_candidates=len(inv),
                       inventory_accepted=sum(f['returncode'] == 0 for f in inv),
                       inventory_rejected=sum(f['returncode'] != 0 for f in inv),
                       inventory_totals=dict(Counter({key: sum(f.get('counts', {}).get(key, 0) for f in inv)
                                                      for key in ('groups', 'hosts', 'children', 'memberships')})),
                       lint_files=len(lint), tasks=len(tasks), positioned_tasks=sum(t['line'] > 0 for t in tasks),
                       unnamed_tasks=sum(t['unnamed'] for t in tasks),
                       normalization_errors=sum(bool(t.get('normalization_error')) for t in tasks),
                       children=len(children), existing_children=sum(c['exists'] for c in children),
                       lint_file_errors=sum(bool(f['errors']) for f in lint), evidence_sha256=digest(report))
        summaries.append(summary)
        print(repo['name'], 'inventory', summary['inventory_accepted'], '/',len(inv), 'positioned tasks',summary['positioned_tasks'], flush=True)
    (work / 'raw.json').write_text(json.dumps(raw, indent=2)+'\n')
    summary = dict(versions={name: importlib.metadata.version(name) for name in ('ansible-core', 'ansible-lint', 'ansible-runner')},
                   subjects=summaries, tasks_executed=False, dependencies_installed=False,
                   inventory_authority='A for native groups/hosts/children; source ranges not measured',
                   lint_authority='controller P-T1 ruling pending; candidate measurement, no grade change',
                   raw_sha256=digest(raw))
    args.out.parent.mkdir(parents=True, exist_ok=True)
    args.out.write_text(json.dumps(summary, indent=2)+'\n')


if __name__ == '__main__':
    main()
