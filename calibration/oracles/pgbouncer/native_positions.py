"""Native-first PgBouncer explicit tokens and syntax-ablation line evidence."""
import argparse
import hashlib
import json
import os
import re
import resource
import subprocess
import tempfile
from pathlib import Path

HERE = Path(__file__).resolve().parent
MAX_EVENTS = 10000
MAX_LOG = 8 * 1024**2
MAX_SOURCE = 1024**2
STOP = 'zz_oracle_stop'
KINDS = dict(databases='target', peers='service', users='element', pgbouncer='key')


def verify(binary, receipt):
    if hashlib.sha256(binary.read_bytes()).hexdigest() != receipt['binary_sha256']:
        raise ValueError('native binary identity mismatch')
    actual = subprocess.check_output([str(binary), '--version'], text=True, timeout=10)
    if actual.strip() != receipt['version_output'].strip():
        raise ValueError('native version mismatch')


def candidates(source):
    """Locate ablation sites only. These never create oracle definitions."""
    if len(source.encode()) > MAX_SOURCE or '\0' in source or STOP in source:
        raise ValueError('source exceeds cap or contains reserved marker/NUL')
    found, section = [], None
    for line, raw in enumerate(source.splitlines(), 1):
        text = raw.strip()
        if not text or text[0] in '#;':
            continue
        header = re.fullmatch(r'\[([^\]\r\n]+)\]', text)
        key = re.fullmatch(r"(?:'([^'\r\n]+)'|([A-Za-z0-9_.\-*]+))[ \t]*=[ \t]*(.*)", text)
        if header:
            section = header[1]
            found.append(dict(kind='section', name=section, line=line, section=section))
        elif key and section:
            found.append(dict(kind='key', name=key[1] or key[2], value=key[3].rstrip(), line=line, section=section))
        else:
            # Includes/inline headers/unknown syntax may never disappear silently.
            raise ValueError('unsupported source syntax for native line attestation')
    return found


def decode(log):
    if len(log) > MAX_LOG:
        raise ValueError('native log cap exceeded')
    events = []
    for raw in log.decode('utf8', errors='strict').splitlines():
        if ' DEBUG parse_ini_file: ' not in raw:
            continue
        message = raw.split(' DEBUG parse_ini_file: ', 1)[1]
        header = re.fullmatch(r'\[([^\]]+)\]', message)
        key = re.fullmatch(r"'(.*?)' = '(.*)'", message)
        accepted = re.fullmatch(r"'.*?' = '.*' ok:([01])", message)
        if accepted:
            if accepted[1] != '1':
                raise ValueError('native key callback refused')
            continue
        if header:
            events.append(dict(kind='section', name=header[1]))
        elif key:
            events.append(dict(kind='key', name=key[1], value=key[2]))
        else:
            raise ValueError('unknown native callback')
        if len(events) > MAX_EVENTS + int(events[-1] == dict(kind='section', name=STOP)):
            raise ValueError('native event cap exceeded')
    if not events or events[-1] != dict(kind='section', name=STOP):
        raise ValueError('native population lacks terminal boundary')
    return events[:-1]


def reconcile(events, sites):
    if len(events) != len(sites):
        raise ValueError('native population and mutation sites differ')
    for native, site in zip(events, sites):
        if any(site.get(key) != value for key, value in native.items()):
            raise ValueError('native callback cannot be mapped to exact source token')
    return sites


def limits():
    resource.setrlimit(resource.RLIMIT_CORE, (0, 0))
    resource.setrlimit(resource.RLIMIT_FSIZE, (MAX_LOG + 1, MAX_LOG + 1))


def native(binary, source, directory, expected_line=None):
    directory.mkdir(parents=True, exist_ok=True)
    path = directory / 'probe.ini'
    body = source + ('' if source.endswith('\n') else '\n')
    stop_line = body.count('\n') + 1
    profile = []
    section = None
    for raw in body.splitlines(keepends=True):
        header = re.fullmatch(r'\[([^\]]+)\]', raw.strip())
        if header:
            section = header[1]
        if section == 'pgbouncer' and re.match(r"[ \t]*(?:logfile|'logfile')[ \t]*=", raw):
            ending = '\r\n' if raw.endswith('\r\n') else '\n'
            raw = raw.split('=', 1)[0] + '= ' + str(directory / 'private.log') + ending
        profile.append(raw)
    path.write_text(''.join(profile) + '[' + STOP + ']\n')
    with tempfile.TemporaryFile() as stdout, tempfile.TemporaryFile() as stderr:
        process = subprocess.run([str(binary), '-v', '-v', str(path)], stdout=stdout, stderr=stderr,
                                 timeout=10, cwd=directory, preexec_fn=limits)
        stdout.seek(0); stderr.seek(0)
        data = stdout.read(MAX_LOG + 1) + stderr.read(MAX_LOG + 1)
    if len(data) > MAX_LOG:
        raise ValueError('native log cap exceeded')
    (directory / 'native.log').write_bytes(data)
    text = data.decode('utf8', errors='strict')
    if expected_line is None:
        expected = 'invalid section "' + STOP + '" in configuration (' + str(path) + ':' + str(stop_line) + ')'
    else:
        expected = 'syntax error in configuration (' + str(path) + ':' + str(expected_line) + '), stopping loading'
    if process.returncode != 1 or text.count(expected) != 1:
        raise ValueError('native parse did not attest exact boundary/position')
    return data


def measure(source, rel, binary, directory):
    sites = candidates(source)
    if any(site['kind'] == 'key' and site['section'] == 'pgbouncer' and site['name'] == 'syslog' for site in sites):
        raise ValueError('global syslog setting is unsupported by private position protocol')
    # Baseline original values must parse completely, even if they disable logging.
    native(binary, source, directory / 'original')
    lines = source.splitlines(keepends=True)
    observed_lines = list(lines)
    changed = 0
    for site in sites:
        if site['kind'] == 'key' and site['section'] == 'pgbouncer' and site['name'] == 'verbose':
            n = site['line'] - 1
            newline = '\r\n' if lines[n].endswith('\r\n') else '\n' if lines[n].endswith('\n') else ''
            observed_lines[n] = lines[n].split('=', 1)[0] + '= 2' + newline
            site['value'] = '2'
            changed += 1
    observed = ''.join(observed_lines)
    events = decode(native(binary, observed, directory / 'observation'))
    for site in sites:
        if site['kind'] == 'key' and site['section'] == 'pgbouncer' and site['name'] == 'logfile':
            site['value'] = str(directory / 'observation' / 'private.log')
    reconcile(events, sites)
    sections = {}
    for index, (event, site) in enumerate(zip(events, sites)):
        mutated = list(observed_lines)
        line = mutated[site['line'] - 1]
        start = len(line) - len(line.lstrip(' \t'))
        mutated[site['line'] - 1] = line[:start] + '?' + line[start:]
        native(binary, ''.join(mutated), directory / ('token-%d' % index), site['line'])
        if event['kind'] == 'section':
            section = sections.setdefault(event['name'], dict(header=site['line'], last=site['line'], keys={}))
        else:
            section['last'] = site['line']
            section['keys'][event['name']] = site['line']
    definitions = []
    for name, section in sections.items():
        if name not in KINDS:
            raise ValueError('unknown native section family')
        definitions.append(dict(path=rel, name=name, kind='section', line=section['header'], end=section['last']))
        for key, line in section['keys'].items():
            definitions.append(dict(path=rel, name=key, kind=KINDS[name], line=line, end=line))
    return definitions, dict(native_callbacks=len(events), positioned_callbacks=len(events),
                             definitions=len(definitions), verbosity_values_observed=changed,
                             logfile_destinations_redirected=sum(site['kind'] == 'key' and site['section'] == 'pgbouncer' and site['name'] == 'logfile' for site in sites),
                             source_sha256=hashlib.sha256(source.encode()).hexdigest())


def main():
    parser = argparse.ArgumentParser()
    for key in ['subject', 'pgbouncer', 'work', 'out']:
        parser.add_argument('-' + key, type=Path, required=True)
    args = parser.parse_args()
    binary = args.pgbouncer.resolve()
    verify(binary, json.loads((HERE / 'native-position-build-r1.json').read_text()))
    # Native controls are mandatory before A output, not optional environment skips.
    import unittest
    os.environ['PGB_POSITION_BIN'] = str(binary)
    suite = unittest.defaultTestLoader.discover(str(HERE), pattern='test_native_positions.py')
    result = unittest.TextTestRunner().run(suite)
    if not result.wasSuccessful() or result.skipped:
        raise ValueError('native authored position controls incomplete')
    from pgbouncer_oracle import walk
    definitions, receipts = [], []
    for index, (relative, source) in enumerate(walk(str(args.subject.resolve()))):
        found, receipt = measure(source, relative, binary, args.work.resolve() / ('file-%d' % index))
        definitions.extend(found); receipts.append(receipt)
    if len(receipts) != 6:
        raise ValueError('canonical claimed-file population changed')
    args.out.mkdir(parents=True, exist_ok=False)
    output = dict(tool='pgbouncer-native-positions', version='PgBouncer 1.26.1 native cfparser', tier='A', definitions=definitions)
    (args.out / 'defs.json').write_text(json.dumps(output, indent=2) + '\n')
    summary = dict(files=len(receipts), definitions=len(definitions),
                   native_callbacks=sum(r['native_callbacks'] for r in receipts),
                   positioned_callbacks=sum(r['positioned_callbacks'] for r in receipts),
                   per_input=receipts, binary_sha256=hashlib.sha256(binary.read_bytes()).hexdigest())
    (args.out / 'summary.json').write_text(json.dumps(summary, indent=2) + '\n')
    print(json.dumps(summary, sort_keys=True))


if __name__ == '__main__':
    main()
