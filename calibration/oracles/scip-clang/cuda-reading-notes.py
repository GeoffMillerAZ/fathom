"""Refresh CUDA driver disclosures from recorded evidence, without measuring."""
import json
from pathlib import Path
import re
import sys


def median_note(name, commit, cpu, wall):
    # Python's stable sort preserves run order for equal recorded CPU rates.
    med = sorted(range(len(cpu)), key=lambda i: cpu[i])[len(cpu) // 2]
    def rates(xs):
        return (', '.join(f'{x:.2f}' for x in xs[:-1]) + ' and ' if len(xs) > 1 else '') + f'{xs[-1]:.2f}'
    return (f'{name}: throughput read {len(cpu)} times at fathom {commit} in one VM, one subject '
            f'(owner ruling C-R5): {rates(cpu)} MB/s per core (wall {rates(wall)}); '
            f'the median run {med + 1} (wall {wall[med]:.2f}), {cpu[med]:.2f}, is this reading '
            f'(sort by recorded CPU rate, then take zero-based position {len(cpu) // 2}; '
            'equal CPU rates retain run order)')


def refresh_notes(reading):
    """Only notes change. Refuse a throughput record inconsistent with its reading."""
    notes = []
    for note in reading['notes']:
        match = re.fullmatch(r'(.+): throughput read (\d+) times at fathom (\S+) in one VM, one subject '
                             r'\(owner ruling C-R5\): ([\d., and]+) MB/s per core \(wall ([\d., and]+)\); .*', note)
        if match:
            name, count, commit, cpu, wall = match.groups()
            cpu, wall = ([float(x) for x in re.findall(r'\d+\.\d+', xs)] for xs in (cpu, wall))
            if not cpu or len(cpu) != int(count) or len(wall) != len(cpu):
                raise ValueError('incomplete throughput run record')
            med = sorted(range(len(cpu)), key=lambda i: cpu[i])[len(cpu) // 2]
            tenancy = reading['tenancy']
            if (cpu[med], wall[med]) != (tenancy['throughput_mb_s'], tenancy['wall_throughput_mb_s']):
                raise ValueError('recorded median does not match the reading')
            note = median_note(name, commit, cpu, wall)
        if (any(s['name'] == 'NVIDIA/cuda-samples' and s['sha'] == 'cab7c66b4fcf35b09ea8fbc4c3c1a9272e92d997'
                for s in reading['subjects']) and '37 of the compiler\'s function definitions' in note
                and 'stand in for a function' in note and '; the predicted 38th' not in note):
            note += '; the predicted 38th extern "C" function is in eigenvalues/util.h, a header the dialect does not own'
        notes.append(note)
    reading['notes'] = sorted(notes)
    return reading


def write_reading(path, reading):
    text = json.dumps(reading, indent=2, ensure_ascii=False)
    Path(path).write_text(text.replace('<', '\\u003c').replace('>', '\\u003e').replace('&', '\\u0026') + '\n')


if __name__ == '__main__':
    if len(sys.argv) != 3 or sys.argv[1] != '--refresh-notes':
        sys.exit('usage: cuda-reading-notes.py --refresh-notes <reading.json>')
    path = Path(sys.argv[2])
    write_reading(path, refresh_notes(json.loads(path.read_text())))
