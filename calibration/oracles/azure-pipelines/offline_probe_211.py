"""Validate authored offline evaluator controls; no support metric or resolver."""
import argparse
import hashlib
import json
from pathlib import Path
import sys
import tempfile


class ReceiptRefusal(ValueError):
    """Whole-unit refusal of an untrusted feasibility receipt."""


def decode_receipt(data):
    def unique_object(pairs):
        result = {}
        for key, value in pairs:
            if key in result:
                raise ReceiptRefusal('duplicate JSON key')
            result[key] = value
        return result
    if len(data) > 1024**2:
        raise ReceiptRefusal('receipt parse cap')
    try:
        return json.loads(data, object_pairs_hook=unique_object)
    except (ValueError, UnicodeError, RecursionError) as exc:
        raise ReceiptRefusal('invalid JSON receipt') from exc


def refuse_aliases(inputs, outputs):
    for output in outputs:
        for source in inputs:
            if output.resolve() == source.resolve() or (output.exists() and source.exists() and output.samefile(source)):
                raise ReceiptRefusal('aliased input/output')


def invalidate_summary(path):
    # Only the explicitly owned success artifact is invalidated, never inputs.
    if path.is_symlink() or path.is_file():
        path.unlink()


def write_summary(path, summary):
    # A failed write must never leave a partially serialized success receipt.
    temporary = None
    try:
        with tempfile.NamedTemporaryFile(mode='w', dir=path.parent, delete=False) as stream:
            temporary = Path(stream.name)
            stream.write(json.dumps(summary, indent=2)+'\n')
        temporary.replace(path)
    finally:
        if temporary is not None and temporary.exists():
            temporary.unlink()


def validate(raw):
    if not isinstance(raw, dict):
        raise ReceiptRefusal('receipt must be an object')
    if set(raw) != {'classification','support_claim','native_owner_attested','cases'}:
        raise ReceiptRefusal('unexpected receipt fields')
    if raw['classification'] != 'feasibility-only' or raw['support_claim'] is not False or raw['native_owner_attested'] is not False:
        raise ReceiptRefusal('unattested controls cannot claim support')
    expected = {
        'template': ('evaluated',['Build','Publish'],['Compile','Release'],['Publish:Build']),
        'changed': ('evaluated',['Changed','Publish'],['Compile','Release'],['Publish:Changed']),
        'malformed': ('refused',[],[],[]), 'missing': ('refused',[],[],[]),
    }
    rows=raw['cases']
    if not isinstance(rows,list) or len(rows)!=4:
        raise ReceiptRefusal('whole control population required')
    seen=set()
    for row in rows:
        if not isinstance(row, dict):
            raise ReceiptRefusal('case must be an object')
        if set(row)!={'case','status','stages','jobs','dependencies'}:
            raise ReceiptRefusal('unexpected case fields')
        if not isinstance(row['case'], str) or not isinstance(row['status'], str):
            raise ReceiptRefusal('case and status must be strings')
        for field in ('stages', 'jobs', 'dependencies'):
            if not isinstance(row[field], list) or any(not isinstance(value, str) for value in row[field]):
                raise ReceiptRefusal('case outputs must be string arrays')
        name=row['case']
        if name in seen or name not in expected:
            raise ReceiptRefusal('duplicate or unknown case')
        seen.add(name)
        actual=(row['status'],row['stages'],row['jobs'],row['dependencies'])
        if actual!=expected[name]:
            raise ReceiptRefusal('failed authored control: '+name)
    return dict(inputs=4,evaluated=2,refused=2,unexpected_failures=0,
                classification='feasibility-only',support_claim=False,native_owner_attested=False,
                canonical_files_measured=0,reading_changed=False)


def main():
    parser=argparse.ArgumentParser()
    parser.add_argument('--raw',type=Path,required=True)
    parser.add_argument('--out',type=Path,required=True)
    args=parser.parse_args()
    try:
        refuse_aliases([args.raw], [args.out])
        invalidate_summary(args.out)
        if args.raw.stat().st_size>1024**2:
            raise ReceiptRefusal('receipt parse cap')
        data=args.raw.read_bytes()
        summary=validate(decode_receipt(data))
        summary['raw_sha256']=hashlib.sha256(data).hexdigest()
        write_summary(args.out, summary)
    except (ReceiptRefusal, OSError) as exc:
        print('receipt_refused: '+(str(exc) if isinstance(exc, ReceiptRefusal) else 'receipt I/O failure'), file=sys.stderr)
        return 1
    print(json.dumps(summary))
    return 0


if __name__=='__main__':
    sys.exit(main())
