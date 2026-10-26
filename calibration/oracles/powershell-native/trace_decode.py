"""Actual native function frames only; parser sites supply scope, never targets."""
from dataclasses import dataclass
import json
from pathlib import Path


@dataclass(frozen=True)
class Limits:
    raw_bytes: int = 128 << 20
    rows: int = 1_000_000
    sites: int = 1_000_000


def bare(name):
    for separator in '.:#/\\ )':name=name.rsplit(separator,1)[-1]
    return name


def decode(path,population,limits=Limits()):
    if Path(path).stat().st_size>limits.raw_bytes:raise ValueError('whole-unit refusal: native byte cap')
    raw=json.loads(Path(path).read_text())
    if raw['pwsh']!='7.6.9' or raw['refused']:raise ValueError('whole-unit refusal: wrong runtime or native refusal')
    if len(raw['calls'])>limits.rows or len(population['sites'])>limits.sites:
        raise ValueError('whole-unit refusal: native row or site cap')
    lookup={}
    for index,site in enumerate(population['sites']):
        if site['kind']=='command':
            lookup.setdefault((site['path'],site['line'],site['column']),[]).append((index,site))
    pairs,hits=set(),set()
    unpaired=0
    for path,line,column,caller,target,invoked in raw['calls']:
        candidates=lookup.get((path,line,column),[])
        if len(candidates)!=1:
            unpaired+=1;continue
        index,site=candidates[0]
        # Alias invocation and actual target are distinct native fields. The
        # alias spelling scopes the site; only actual native target is truth.
        if invoked.casefold()!=site['written'].casefold() or bare(caller).casefold()!=bare(site['from']).casefold():
            unpaired+=1;continue
        pairs.add((path,bare(caller),bare(target)))
        hits.add(index)
    return {'tool':'PowerShell native command-breakpoint function frames','version':'7.6.9','tier':'A',
            'calls':[{'path':p,'from':f,'to':t} for p,f,t in sorted(pairs)],
            'disclose':['OL-R11: observed executed native function calls in current process only; no recall on unexecuted code. Cmdlets, members, child-process calls and ambiguous callbacks remain unattested.']}, {
            'literal_sites':len(population['sites']),'attested_sites':len(hits),
            'coverage':len(hits)/len(population['sites']),'native_pairs':len(pairs),
            'unattested_sites':len(population['sites'])-len(hits),'unpaired_native_rows':unpaired,
            'native_events':raw['native_events'],'original_breakpoints':raw['original_breakpoints'],
            'retained_breakpoints':raw['retained_breakpoints'],'driver_status':raw['driver_status'],
            'exception_sha256':raw['exception_sha256']}
