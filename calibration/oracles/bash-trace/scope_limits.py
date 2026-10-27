"""Whole-unit source/population refusal, with lower-only fixture limits."""
from dataclasses import dataclass


@dataclass(frozen=True)
class Limits:
    files:int=2048
    source_bytes:int=64<<20
    file_bytes:int=2<<20
    sites:int=1_000_000


def validate(root,files,source_sites=(),call_sites=(),limits=Limits()):
    hard=Limits()
    if any(v<1 or v>getattr(hard,k) for k,v in vars(limits).items()):raise ValueError('whole-unit refusal: invalid source limits')
    sizes=[(root/p).stat().st_size for p in files]
    if not files or len(files)>limits.files:raise ValueError('whole-unit refusal: source file count cap')
    if sum(sizes)>limits.source_bytes:raise ValueError('whole-unit refusal: source total byte cap')
    if max(sizes)>limits.file_bytes:raise ValueError('whole-unit refusal: source individual byte cap')
    if len(source_sites)>limits.sites or len(call_sites)>limits.sites:raise ValueError('whole-unit refusal: source/call population cap')


def merge_observations(destination, incoming, max_items=1_000_000):
    if not 1<=max_items<=1_000_000:raise ValueError('whole-unit refusal: invalid native union cap')
    for item in incoming:
        destination.add(item)
        if len(destination)>max_items:raise ValueError('whole-unit refusal: combined native observation cap')
