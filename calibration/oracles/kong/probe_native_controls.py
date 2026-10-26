"""Authored native-field, mutation, cap and offline official-CLI controls."""
import argparse
import hashlib
import json
import os
import resource
import subprocess
from pathlib import Path

HERE = Path(__file__).resolve().parent
SAMPLE = '''_format_version: "3.0"
services:
- name: app
  host: pool
  routes:
  - name: nested
    paths: ["/nested"]
  plugins:
  - name: cors
- name: unused
  host: unused.invalid
routes:
- name: flat
  paths: ["/flat"]
  service:
    name: app
upstreams:
- name: pool
  targets:
  - target: 127.0.0.1:19001
plugins:
- name: key-auth
  route: flat
'''
URL_ONLY = '''_format_version: "3.0"
services:
- name: app
  url: http://pool:19001
upstreams:
- name: pool
'''


def sha(data):
    return hashlib.sha256(data).hexdigest()


def child_limits():
    resource.setrlimit(resource.RLIMIT_CORE, (0, 0))
    resource.setrlimit(resource.RLIMIT_FSIZE, (8 * 1024**2 + 1, 8 * 1024**2 + 1))


def native_query(binary, source, directory):
    directory.mkdir()
    home = directory / 'home'; home.mkdir()
    env = dict(HOME=str(home), PATH=os.defpath, TMPDIR=os.environ['TMPDIR'], DECK_ANALYTICS='false', GOMAXPROCS='2')
    with (directory / 'stdout').open('wb') as stdout, (directory / 'stderr').open('wb') as stderr:
        if os.uname().sysname != 'Darwin':
            raise ValueError('OS network-isolation runner unsupported; refuse unisolated execution')
        command = ['/usr/bin/sandbox-exec', '-p', '(version 1)(allow default)(deny network*)', str(binary)]
        result = subprocess.run(command, input=source.encode(), stdout=stdout, stderr=stderr,
                                timeout=30, env=env, cwd=home, preexec_fn=child_limits)
    raw = (directory / 'stdout').read_bytes()
    body = raw[:-1] if raw.endswith(b'\n') else raw
    if len(body) > 8 * 1024**2 or result.returncode:
        raise ValueError('authored native probe refused')
    output = json.loads(body)
    for key in ('native_entities', 'native_written_fields'):
        if output.get(key) is None:
            output[key] = []  # Native Go nil slices encode an empty family.
    if output['status'] != 'native_authored_probe_complete_ungraded' or output['http_requests']:
        raise ValueError('native probe failed closed/offline contract')
    return output


def go_test(build, overlay, pattern, out):
    command = ['go', 'test', '-race', '-count=1', '-v', '-modfile', str(build / 'probe.mod'),
               '-overlay', str(overlay), '-run', pattern, '.']
    process = subprocess.run(command, cwd=HERE / 'native', stdout=subprocess.PIPE,
                             stderr=subprocess.STDOUT, timeout=180)
    out.write_bytes(process.stdout)
    return process.returncode, process.stdout.decode()


def run(build, deck, deck_receipt, out):
    out.mkdir(parents=True, exist_ok=False)
    identity = json.loads((build / 'build-receipt.json').read_text())
    native = build / 'kong-native'
    if (sha(native.read_bytes()) != identity['binary_sha256'] or
            sha((HERE / 'native/main.go').read_bytes()) != identity['probe_source_sha256'] or
            sha((HERE / 'instrument_native.py').read_bytes()) != identity['instrument_sha256']):
        raise ValueError('native build/source identity mismatch')
    official = json.loads(deck_receipt.read_text())
    if sha(deck.read_bytes()) != official['binary_sha256'] or official['version'] != '1.68.0':
        raise ValueError('official decK identity mismatch')
    overlay = json.loads((build / 'overlay.json').read_text())
    code, log = go_test(build, build / 'overlay.json', '.', out / 'green.log')
    if code or '--- SKIP:' in log:
        raise ValueError('full native race controls failed or skipped')
    count = log.count('--- PASS:')
    if count != 14:
        raise ValueError('unexpected native control count')
    mutations = []
    # Mutate only the read-only native field observation, not owner parsing.
    target, observed = next(iter(overlay['Replace'].items()))
    source = Path(observed).read_text()
    callback = 'OracleServiceObserver(service.Name, service.ID, service.Host, service.URL)'
    if source.count(callback) != 2:
        raise ValueError('native field-observer anchors changed')
    mutant = out / 'types-no-host.go'
    mutant.write_text(source.replace(callback, 'OracleServiceObserver(service.Name, service.ID, nil, service.URL)'))
    mapping = dict(overlay['Replace']); mapping[target] = str(mutant)
    modified = out / 'no-host-overlay.json'; modified.write_text(json.dumps({'Replace': mapping}))
    code, red = go_test(build, modified, 'TestURLDerivedHostDoesNotBecomeWrittenHost', out / 'no-host-red.log')
    if not code or '--- FAIL:' not in red:
        raise ValueError('native Host provenance mutant survived')
    code, green = go_test(build, build / 'overlay.json', 'TestURLDerivedHostDoesNotBecomeWrittenHost', out / 'no-host-green.log')
    if code:
        raise ValueError('native Host provenance restore failed')
    mutations.append(dict(mutant='drop raw native Host observation', file='instrument_native.py',
                          failure_line=next(line for line in red.splitlines() if '--- FAIL:' in line), restored_green=True))
    native_source = (HERE / 'native/main.go').read_text()
    start = native_source.index('\n\tfor _, consumer := range content.Consumers {')
    end = native_source.index('\n\tfor _, upstream := range content.Upstreams {', start)
    mutant = out / 'main-no-consumer-scopes.go'; mutant.write_text(native_source[:start] + native_source[end:])
    mapping = dict(overlay['Replace']); mapping[str(HERE / 'native/main.go')] = str(mutant)
    modified = out / 'no-scopes-overlay.json'; modified.write_text(json.dumps({'Replace': mapping}))
    code, red = go_test(build, modified, 'TestNativePluginScopesUnderConsumerAndGroupAreEnumerated', out / 'no-scopes-red.log')
    if not code or '--- FAIL:' not in red:
        raise ValueError('native scope omission mutant survived')
    code, green = go_test(build, build / 'overlay.json', 'TestNativePluginScopesUnderConsumerAndGroupAreEnumerated', out / 'no-scopes-green.log')
    if code:
        raise ValueError('native scope restore failed')
    mutations.append(dict(mutant='omit native consumer/group plugin populations', file='native/main.go',
                          failure_line=next(line for line in red.splitlines() if '--- FAIL:' in line), restored_green=True))
    authored = out / 'kong.yml'; authored.write_text(SAMPLE)
    nodes = out / 'fathom-nodes.json'
    subprocess.run(['go', 'run', '-race', './calibration/oracles/kong-extractor-output',
                    '-source', str(authored), '-out', str(nodes)], cwd=HERE.parents[2], check=True)
    node_count = len(json.loads(nodes.read_text()))
    if not node_count:
        raise ValueError('tripwire requires actual extractor output')
    before = native_query(native, SAMPLE, out / 'native-before')
    nodes.write_text('[]\n')
    after = native_query(native, SAMPLE, out / 'native-after-output-removal')
    changed = native_query(native, SAMPLE.replace('pool', 'second'), out / 'native-changed')
    if before != after or before == changed:
        raise ValueError('native output tripwire or input-sensitivity control failed')
    cross_checks = []
    for index, source in enumerate((SAMPLE, URL_ONLY, URL_ONLY.replace('  url:', '  host: pool\n  url:'))):
        folder = out / ('deck-control-' + str(index));folder.mkdir()
        path = folder / 'authored.yml';path.write_text(source)
        rendered = folder / 'rendered.json'
        env = dict(HOME=str(folder), PATH=os.defpath, TMPDIR=os.environ['TMPDIR'], DECK_ANALYTICS='false', GOMAXPROCS='2')
        command = ['/usr/bin/sandbox-exec', '-p', '(version 1)(allow default)(deny network*)',
                   str(deck), 'file', 'render', str(path), '--format', 'json', '-o', str(rendered)]
        subprocess.run(command, cwd=folder, env=env, check=True, timeout=30, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        state = json.loads(rendered.read_text())
        observed = native_query(native, source, folder / 'native')
        app = next(item for item in state['services'] if item.get('name') == 'app')
        if app['host'] != 'pool':
            raise ValueError('official host evaluation diverged')
        if index == 0:
            flat = next(route for route in app['routes'] if route['name'] == 'flat')
            if not any(plugin['name'] == 'key-auth' for plugin in flat['plugins']):
                raise ValueError('official route/plugin binding diverged')
        projection = dict(service_host=app['host'])
        if index == 0:
            projection.update(route_service=['flat', 'app'], plugin_route=['key-auth', 'flat'])
        cross_checks.append(dict(control=index, official_render_accepted=True,
                                 native_fields=len(observed['native_written_fields']),
                                 native_entities=len(observed['native_entities']), native_http_requests=observed['http_requests'],
                                 evaluated_projection_sha256=sha(json.dumps(projection, sort_keys=True).encode()),
                                 interpretation='URL-only native written-host population empty despite owner-rendered host' if index == 1 else 'owner evaluated host/bindings agree'))
    summary = dict(kind='authored prototype controls only; no calibration or metric/tier claim',
                   native_binary_sha256=identity['binary_sha256'],
                   native_test_source_sha256=sha((HERE / 'native/main_test.go').read_bytes()),
                   control_script_sha256=sha(Path(__file__).read_bytes()), official_deck_binary_sha256=official['binary_sha256'],
                   race_tests=count, skips=0, mutations=mutations,
                   fathom_nodes_before=node_count, fathom_nodes_after=0,
                   output_removal_interpretation='future hidden-read tripwire; not proof by construction',
                   native_truth_before_sha256=sha(json.dumps(before, sort_keys=True).encode()),
                   native_truth_after_output_removal_sha256=sha(json.dumps(after, sort_keys=True).encode()),
                   native_truth_after_input_change_sha256=sha(json.dumps(changed, sort_keys=True).encode()),
                   native_entities=len(before['native_entities']), native_written_fields=len(before['native_written_fields']),
                   native_resolved_fields=sum(ref['outcome']=='owner_registry_resolved' for ref in before['native_written_fields']),
                   native_http_requests=0, official_cli_network_denied=True, cli_cross_checks=cross_checks,
                   caps=dict(source_bytes=1048576, source_refusal_input=1048577, native_entities_and_fields=10000,
                             population_refusal_input=10001, output_bytes=8388608, output_refusal_input=8388609,
                             whole_input_or_output_refused=True))
    (out / 'summary.json').write_text(json.dumps(summary, indent=2) + '\n')
    return summary


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    for name in ('build', 'deck', 'deck-receipt', 'out'):
        parser.add_argument('-' + name, type=Path, required=True)
    args = parser.parse_args()
    print(json.dumps(run(args.build.resolve(), args.deck.resolve(), args.deck_receipt, args.out.resolve()), sort_keys=True))
