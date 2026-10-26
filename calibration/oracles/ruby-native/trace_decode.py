"""Attribute actual TracePoint answers conservatively, retaining all unknowns."""
import json
from collections import Counter, defaultdict
from dataclasses import dataclass
from pathlib import Path


@dataclass(frozen=True)
class Limits:
    files: int = 64
    total_bytes: int = 256 << 20
    file_bytes: int = 128 << 20
    rows: int = 1_000_000
    functions: int = 100_000


def decode(raw_files, syntax, limits=Limits()):
    if not raw_files or len(raw_files) > limits.files:
        raise ValueError('whole-unit refusal: missing native output or file count cap')
    if sum(Path(p).stat().st_size for p in raw_files) > limits.total_bytes:
        raise ValueError('whole-unit refusal: total TracePoint byte cap')
    sites = defaultdict(list)
    blocked = {(x['path'],line) for x in syntax['nonliteral_lines'] for line in range(x['line'],x['end']+1)}
    for number, site in enumerate(syntax['sites']):
        sites[(site['path'],site['line'],site['written_name'])].append(number)
    hits, pairs, counts = set(), set(), Counter()
    for path in raw_files:
        if Path(path).stat().st_size > limits.file_bytes:
            raise ValueError('whole-unit refusal: raw TracePoint byte cap')
        data = json.loads(Path(path).read_text())
        if data['refused'] or data['ruby'] != '3.3.8':
            raise ValueError('whole-unit refusal: raw TracePoint cap or wrong release')
        if len(data['calls']) > limits.rows or len(data['functions']) > limits.functions:
            raise ValueError('whole-unit refusal: native row or function count cap')
        counts['native_events'] += data['native_entries']
        ranges = defaultdict(list)
        for relative, name, start, end in data['functions']:
            ranges[relative].append((name,start,end))
        for relative, line, native_caller, actual_target, invoked_name in data['calls']:
            eligible = sites.get((relative,line,actual_target),[])
            if invoked_name != actual_target or (relative,line) in blocked or len(eligible) != 1:
                counts['unattested_native_rows'] += 1
                continue
            enclosing = [(name,end-start) for name,start,end in ranges[relative] if start <= line <= end]
            if not enclosing:
                counts['missing_native_caller_range'] += 1
                continue
            width = min(size for _,size in enclosing)
            callers = {name for name,size in enclosing if size == width}
            if callers != {native_caller}:
                counts['native_callback_or_ambiguous_caller'] += 1
                continue
            # Caller and target come from native stack/method identity; never
            # copy the syntax walk's caller. Alias/ambiguous sites stay misses.
            pairs.add((relative,native_caller,actual_target))
            hits.add(eligible[0])
    return {'tool':'Ruby native TracePoint and caller_locations','version':'Ruby 3.3.8','tier':'A',
            'calls':[{'path':p,'from':f,'to':t} for p,f,t in sorted(pairs)],
            'disclose':['OL-R10: actual executed literal population only; all original unexecuted/ambiguous sites remain in coverage denominator.']}, {
            'literal_sites':len(syntax['sites']),'attested_sites':len(hits),
            'coverage':len(hits)/len(syntax['sites']) if syntax['sites'] else None,
            'unattested_sites':len(syntax['sites'])-len(hits),'native_pairs':len(pairs),'native_counts':dict(counts)}
