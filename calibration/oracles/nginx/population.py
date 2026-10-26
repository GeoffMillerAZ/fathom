"""Decode whole-input native registration observations, never scanner pairs."""
from collections import Counter

MARKER = 'ORACLE_NGINX_UPSTREAM_V1 '
PASS = {'proxy_pass', 'fastcgi_pass', 'uwsgi_pass', 'scgi_pass', 'grpc_pass', 'memcached_pass'}
MAX_EVENTS = 10000
MAX_STDERR_BYTES = 8 * 1024**2


def decode_events(stderr):
    if len(stderr.encode('utf8')) > MAX_STDERR_BYTES:
        raise ValueError('native observer stderr exceeds whole-profile budget')
    events = []
    for raw in stderr.splitlines():
        if 'ORACLE_NGINX_UPSTREAM_ERROR' in raw:
            raise ValueError('native observer reported missing metadata')
        if 'ORACLE_NGINX_UPSTREAM_' in raw and MARKER not in raw:
            raise ValueError('unknown native observer protocol')
        if MARKER not in raw:
            continue
        if len(events) == MAX_EVENTS:
            raise ValueError('native observer event count exceeds whole-profile budget')
        fields = raw.split(MARKER, 1)[1].split(' ')
        if len(fields) != 8:
            raise ValueError('incomplete native observer record')
        domain, pointer, flags, line = fields[:4]
        try:
            event = dict(domain=domain, pointer=int(pointer, 16), flags=int(flags), line=int(line))
            event.update(zip(('path', 'command', 'argument', 'host'),
                             (bytes.fromhex(value).decode('utf8') for value in fields[4:])))
        except (ValueError, UnicodeError) as error:
            raise ValueError('malformed native observer record') from error
        if domain not in ('http', 'stream') or not event['pointer'] or event['line'] < 1 or not event['path']:
            raise ValueError('invalid native identity/source metadata')
        events.append(event)
    return events


def local_population(events, target_path, locate_owner):
    declarations = {}
    for event in events:
        if event['flags'] & 1:
            if event['command'] != 'upstream' or not event['host']:
                raise ValueError('unclassified native declaration')
            key = event['domain'], event['pointer']
            if key in declarations:
                raise ValueError('duplicate native declaration identity')
            declarations[key] = event
    refs, counts = [], Counter()
    for event in events:
        if event['flags'] & 1:
            counts['declarations'] += 1
            continue
        if event['command'] not in PASS:
            raise ValueError('unclassified native registration call')
        declaration = declarations.get((event['domain'], event['pointer']))
        if event['path'] != target_path:
            counts['context_only_sites'] += 1
        elif declaration is None:
            counts['implicit_or_external_sites'] += 1
        elif declaration['path'] != target_path:
            counts['cross_file_sites_ungraded'] += 1
        else:
            owner = locate_owner(event)
            if not owner:
                raise ValueError('native site lacks an unambiguous enclosing owner')
            refs.append({'path': target_path, 'from': owner, 'to': declaration['host']})
            counts['native_local_sites'] += 1
    # Distinct source sites can form the same semantic owner-target pair.
    unique = {(r['path'], r['from'], r['to']) for r in refs}
    return [dict(path=p, **{'from': f, 'to': t}) for p, f, t in sorted(unique)], dict(counts)
