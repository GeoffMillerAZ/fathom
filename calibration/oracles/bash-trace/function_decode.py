"""Actual native function names, with strict line/caller ambiguity refusal."""
from collections import defaultdict
from pathlib import Path


def decode(path,root,population,max_bytes=32<<20,max_fields=1_000_000,max_field_bytes=1<<20):
    path=Path(path);root=Path(root).resolve()
    if path.stat().st_size>max_bytes:raise ValueError('whole-unit refusal: function trace byte cap')
    data=path.read_bytes()
    if data and not data.endswith(b'\0'):raise ValueError('whole-unit refusal: incomplete native call field')
    if data.count(b'\0')>max_fields:raise ValueError('whole-unit refusal: function field count cap')
    parts=data.split(b'\0')
    if any(len(p)>max_field_bytes for p in parts):raise ValueError('whole-unit refusal: function field byte cap')
    if (len(parts)-1)%6:raise ValueError('whole-unit refusal: malformed native call record')
    byline=defaultdict(list)
    for index,site in enumerate(population['sites']):byline[(site['path'],site['line'])].append((index,site))
    pairs,hits=set(),set();unknown=0
    for i in range(0,len(parts)-1,6):
        marker,caller_file,line,caller,callee,callee_file=[p.decode('utf-8','surrogateescape') for p in parts[i:i+6]]
        if marker!='CALL':raise ValueError('whole-unit refusal: unknown native call record')
        # Full native filenames only; never infer a relative frame from the
        # decoder's current directory or from product syntax/output.
        if not Path(caller_file).is_absolute() or not Path(callee_file).is_absolute():unknown+=1;continue
        try:
            relative=Path(caller_file).resolve().relative_to(root).as_posix()
            Path(callee_file).resolve().relative_to(root)
        except ValueError:unknown+=1;continue
        candidates=byline.get((relative,int(line)),[])
        if len(candidates)!=1:unknown+=1;continue
        index,site=candidates[0]
        if not site['literal'] or site['from']!=caller:unknown+=1;continue
        pairs.add((relative,caller,callee));hits.add(index)
    return pairs,hits,unknown
