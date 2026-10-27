"""Audit all original command sites against the producer's own occurrences."""
import importlib.util
from pathlib import Path

spec=importlib.util.spec_from_file_location('scip_wire',Path(__file__).resolve().parents[1]/'scip-wire-census/census.py')
wire=importlib.util.module_from_spec(spec);spec.loader.exec_module(wire)


def audit(path,files,population,max_bytes=32<<20,max_documents=2048,max_occurrences=1_000_000):
    if not(1<=max_bytes<=32<<20 and 1<=max_documents<=2048 and 1<=max_occurrences<=1_000_000):raise ValueError('whole-unit refusal: invalid SCIP caps')
    if Path(path).stat().st_size>max_bytes:raise ValueError('whole-unit refusal: SCIP byte cap')
    docs=set();positions=set();functions=set();occurrences=0;function_definitions=0;missing_enclosing_ranges=0
    for n,kind,document in wire.fields(Path(path).read_bytes()):
        if n!=2:continue
        values=list(wire.fields(document))
        relative=next((wire.relative_path(value) for n,k,value in values if n==1),None)
        if relative not in files or relative in docs:raise ValueError('whole-unit refusal: duplicate or outside-scope SCIP document')
        docs.add(relative)
        if len(docs)>max_documents:raise ValueError('whole-unit refusal: SCIP document cap')
        for n,k,value in values:
            if n!=2:continue
            occurrences+=1
            if occurrences>max_occurrences:raise ValueError('whole-unit refusal: SCIP occurrence cap')
            fields=list(wire.fields(value))
            symbol=next((v.decode() for n,k,v in fields if n==2),'')
            own,_,typed,enclosing,enclosing_seen,enclosing_typed,definition=wire.occurrence(value)
            if definition and symbol.startswith('scip-shell shell ') and symbol.endswith('().'):
                function_definitions+=1
                if not ((enclosing_seen and len(enclosing) in (3,4)) or enclosing_typed):missing_enclosing_ranges+=1
            if definition or len(own) not in (3,4):continue
            position=(relative,own[0]+1,own[1]+1)
            if symbol.startswith('scip-shell shell ') and symbol.endswith('().'):
                positions.add(position);functions.add(position)
            elif symbol.startswith('scip-shell system '):positions.add(position)
    hits={i for i,s in enumerate(population['sites']) if s['literal'] and (s['path'],s['line'],s['column']) in positions}
    function_hits={i for i,s in enumerate(population['sites']) if (s['path'],s['line'],s['column']) in functions}
    return dict(indexed_files=len(docs),unindexed_files=len(set(files)-docs),occurrences=occurrences,
        function_definitions=function_definitions,function_definitions_without_native_enclosing_range=missing_enclosing_ranges,
        pair_diagnostic_tier=('C: generic scorer uses product caller ranges when native enclosing ranges are missing; independent raw occurrence coverage is separate' if missing_enclosing_ranges else 'native enclosing ranges present; maintained-indexer qualification remains separate'),
        original_command_sites=len(population['sites']),native_command_sites=len(hits),
        native_function_sites=len(function_hits),unattested_command_sites=len(population['sites'])-len(hits),
        unindexed_file_command_sites=sum(s['path'] not in docs for s in population['sites']),
        population='all original shfmt CallExpr sites, dynamic included; only native function/system-symbol occurrences attest literal command targets; no source/variable occurrence counted as a resolved call')


def diagnostic_tier(audited):
    # Generic scoring borrows product caller extents when native ranges are absent.
    return 'C' if audited['function_definitions_without_native_enclosing_range'] else 'A'
