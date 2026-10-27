"""Strict standard-library validator for this probe's closed JSON schema."""
import json
import os
import re
import stat
import tempfile
from pathlib import Path

SCHEMA = Path(__file__).with_name('probe-linux-receipt.schema.json')
MAX_RECEIPT = 128 * 1024


def validate(value, schema, root=None):
    supported = {'$schema', '$defs', '$ref', 'title', 'type', 'const', 'enum', 'properties', 'required', 'additionalProperties', 'items', 'pattern', 'maxItems', 'minItems', 'minimum', 'maximum', 'allOf', 'if', 'then'}
    if set(schema) - supported:
        raise ValueError('unsupported receipt schema keyword')
    root = schema if root is None else root
    if '$ref' in schema:
        target = root
        for part in schema['$ref'][2:].split('/'):
            target = target[part]
        return validate(value, target, root)
    kinds = {'null': lambda x: x is None, 'boolean': lambda x: type(x) is bool,
             'integer': lambda x: type(x) is int, 'string': lambda x: type(x) is str,
             'object': lambda x: type(x) is dict, 'array': lambda x: type(x) is list}
    if 'type' in schema:
        types = schema['type'] if isinstance(schema['type'], list) else [schema['type']]
        if not any(kinds[k](value) for k in types):
            raise ValueError('receipt type invalid')
    if 'const' in schema and (type(value) is not type(schema['const']) or value != schema['const']):
        raise ValueError('receipt constant invalid')
    if 'enum' in schema and not any(type(value) is type(x) and value == x for x in schema['enum']):
        raise ValueError('receipt enum invalid')
    if isinstance(value, dict):
        if any(key not in value for key in schema.get('required', [])):
            raise ValueError('receipt required field absent')
        properties = schema.get('properties', {})
        if schema.get('additionalProperties') is False and set(value) - set(properties):
            raise ValueError('receipt extra field refused')
        for key in value.keys() & properties.keys():
            validate(value[key], properties[key], root)
    if isinstance(value, list):
        if len(value) > schema.get('maxItems', MAX_RECEIPT) or len(value) < schema.get('minItems', 0):
            raise ValueError('receipt array length invalid')
        for item in value:
            if 'items' in schema:
                validate(item, schema['items'], root)
    if isinstance(value, str) and 'pattern' in schema and not re.fullmatch(schema['pattern'], value):
        raise ValueError('receipt string pattern invalid')
    if type(value) is int and (value < schema.get('minimum', -2**63) or value > schema.get('maximum', 2**63-1)):
        raise ValueError('receipt integer bound invalid')
    for part in schema.get('allOf', []):
        validate(value, part, root)
    if 'if' in schema:
        try:
            validate(value, schema['if'], root)
        except ValueError:
            pass
        else:
            validate(value, schema['then'], root)
    return value


def read_receipt(path, schema, identity=None):
    # Open only a bounded regular file after the PID namespace is gone.
    fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
    with os.fdopen(fd, 'rb') as source:
        if not stat.S_ISREG(os.fstat(source.fileno()).st_mode):
            raise ValueError('receipt must be regular')
        raw = source.read(MAX_RECEIPT + 1)
    if len(raw) > MAX_RECEIPT:
        raise ValueError('receipt exceeds cap')
    value = json.loads(raw)
    validate(value, schema)
    if value['status'] == 'pending':
        raise ValueError('pending receipt cannot publish')
    if identity and any(value.get(k) != v for k, v in identity.items()):
        raise ValueError('receipt launcher identity mismatch')
    if value['status'] == 'probe_complete_requires_analysis':
        if 'failure_class' in value or any('failure_class' in row for row in value['evaluations']):
            raise ValueError('completed receipt carries failure')
        if [row['variant'] for row in value['evaluations']] != ['valid', 'unknown_parameter', 'changed_input_fact']:
            raise ValueError('completed receipt control population invalid')
    return value


def write_receipt(path, value, schema, public=False):
    validate(value, schema)
    if public and value.get('native_executed') is True:
        for key in ['write_diagnostics','diagnostic_status','trace_within_cap','trace_tool']:
            if key not in value: raise ValueError('public native receipt lacks outer diagnostics')
        diagnostic=value['write_diagnostics']
        if value['diagnostic_status']=='refused':
            if diagnostic is not None or 'diagnostic_failure_class' not in value:
                raise ValueError('diagnostic refusal must be explicit')
        elif (diagnostic is None or not diagnostic['vendor_exec_observed'] or
              diagnostic['native_returncode']!=value.get('native_returncode') or
              value.get('trace_writer_hidden_from_vendor') is not True):
            raise ValueError('public native receipt disagrees with outer trace')
    raw=(json.dumps(value,indent=2)+'\n').encode()
    if len(raw)>MAX_RECEIPT: raise ValueError('receipt exceeds cap')
    fd, name = tempfile.mkstemp(prefix='.receipt-validated.', suffix='.tmp', dir=path.parent)
    temporary = Path(name)
    try:
        with os.fdopen(fd, 'wb') as target:
            target.write(raw)
        os.replace(temporary, path)
    finally:
        temporary.unlink(missing_ok=True)
