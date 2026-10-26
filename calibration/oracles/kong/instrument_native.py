"""Observe the owner's typed service fields before its URL normalization.

The original parser's decisions, arguments, control flow and returns stay exact.
This overlay never writes the shared module cache.
"""
import hashlib
import json
from pathlib import Path

HERE = Path(__file__).resolve().parent
DECLARATION = ('// Bench-only read-only typed-field observer.\n'
               'var OracleServiceObserver func(name, id, host, url *string)\n\n')
HOOK = ('\tif OracleServiceObserver != nil {\n'
        '\t\tOracleServiceObserver(service.Name, service.ID, service.Host, service.URL)\n'
        '\t}\n')
ANCHOR = '// FService represents a Kong Service'
RETURN = '\treturn copyFromService(service, s)'


def instrument(source):
    pins = json.loads((HERE / 'native-source-pins-r1.json').read_text())
    expected = next(row['sha256'] for row in pins['sources'] if row['path'] == 'types.go')
    if hashlib.sha256(source).hexdigest() != expected:
        raise ValueError('native source identity mismatch')
    text = source.decode('utf8')
    if text.count(ANCHOR) != 1 or text.count(RETURN) != 2 or 'OracleServiceObserver' in text:
        raise ValueError('native observer anchors changed')
    observed = text.replace(ANCHOR, DECLARATION + ANCHOR).replace(RETURN, HOOK + RETURN)
    restored = observed.replace(DECLARATION, '', 1).replace(HOOK, '')
    if restored.encode() != source:
        raise ValueError('observer does not restore exact owner source')
    return observed.encode()
