#!/usr/bin/env python3
"""Regrade the frozen lane reading with the exact main D19 harness, never merge JSON."""
import argparse
import copy
import hashlib
import json
import os
from pathlib import Path
import subprocess
import tempfile
import time

DIALECT = 'kustomize'
ROOT = Path(__file__).resolve().parents[3]
READING = 'docs/bench/readings/lang/' + DIALECT + '.json'


def snapshot(commit):
    return subprocess.check_output(['git', 'show', commit + ':' + READING], cwd=ROOT)


def differences(a, b, prefix=''):
    if isinstance(a, dict) and isinstance(b, dict):
        result = []
        for key in sorted(set(a) | set(b)):
            if not prefix and key == 'notes':
                continue
            path = prefix + '.' + key if prefix else key
            if key not in a or key not in b:
                result.append(path)
            else:
                result.extend(differences(a[key], b[key], path))
        return result
    return [] if a == b else [prefix]


def admit_launch():
    os.setpriority(os.PRIO_PROCESS, 0, max(10, os.getpriority(os.PRIO_PROCESS, 0)))
    if os.getpriority(os.PRIO_PROCESS, 0) < 10:
        raise RuntimeError('actual nice must be at least 10')
    while os.getloadavg()[0] > 9:
        time.sleep(20)


def run(args):
    lane_raw, main_raw = snapshot(args.lane_commit), snapshot(args.main_commit)
    lane, main = json.loads(lane_raw), json.loads(main_raw)
    binary = Path(args.indexbench).resolve(strict=True)
    admit_launch()
    identity = subprocess.check_output(['go', 'version', '-m', str(binary)], text=True)
    full_main = subprocess.check_output(['git', 'rev-parse', args.main_commit], cwd=ROOT, text=True).strip()
    if 'vcs.revision=' + full_main not in identity or 'vcs.modified=false' not in identity:
        raise RuntimeError('regrade binary must be a clean build of the named main commit')
    candidate = copy.deepcopy(lane)
    if DIALECT == 'kustomize':
        candidate['notes'] = [note.split(' D19 (#7) is not on main:')[0]
                              if ' D19 (#7) is not on main:' in note else
                              note.replace('Leave-one-subject-out under current harness rules,',
                                           'Frozen pre-D19 leave-one-subject-out diagnostics,')
                              for note in candidate.get('notes', [])]
    scratch = ROOT / 'workspace/tmp'
    scratch.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix=DIALECT + '-d19-', dir=scratch) as temp:
        output = Path(temp) / (DIALECT + '.json')
        def grade(data):
            output.write_text(json.dumps(data, indent=2) + '\n')
            admit_launch()
            subprocess.run([str(binary), 'lang-regrade', '-commit', args.main_commit, str(output)],
                           cwd=ROOT, check=True)
            return json.loads(output.read_text())
        preview = grade(candidate)
        # Regrading may recompute grades/metric availability and manifest requirements,
        # never frozen subjects, populations, timings, oracle tiers or extraction identity.
        for key in ['subjects', 'counts', 'tenancy', 'metric_tiers', 'oracles', 'required_kinds',
                    'kind_intersection', 'measured_at', 'fathom_commit', 'extractor']:
            if preview.get(key) != lane.get(key):
                raise RuntimeError('regrade changed frozen measurement: ' + key)
        lane_delta = differences(lane, preview)
        main_delta = differences(main, preview)
        counts = preview['counts']
        precision, recall = counts['xref_fathom'], counts['xref_truth']
        note = ('R-D19 at ' + args.main_commit + ': relative to lane ' + args.lane_commit +
                ', harness-rule non-note field changes: ' + (', '.join(lane_delta) or 'none') +
                '; notes record D19 provenance; all frozen measurements preserved. Relative to main, lane evidence changes: ' +
                (', '.join(main_delta) or 'none') + '. Own L3 denominators are xref_fathom=' +
                str(precision) + ' (precision) and xref_truth=' + str(recall) +
                ' (recall), each requiring at least 20; grade L3=' + preview['grade']['L3'] +
                '. Manifest l3_evidence bar: ' + ('none; no subject minimum applies' if not preview.get('l3_evidence') else json.dumps(preview['l3_evidence'], sort_keys=True)) + '.')
        leave_one_out = []
        if DIALECT == 'kustomize':
            for subject in preview['subjects']:
                p = precision - subject['counts'].get('xref_fathom', 0)
                r = recall - subject['counts'].get('xref_truth', 0)
                leave_one_out.append({'subject': subject['name'], 'xref_fathom': p,
                                      'xref_truth': r, 'both_count_floors_met': min(p, r) >= 20})
            critical = [row for row in leave_one_out if not row['both_count_floors_met']]
            note += (' One-subject count-floor dependence remains: ' + '; '.join(
                row['subject'] + ' removed leaves ' + str(row['xref_fathom']) + '/' + str(row['xref_truth'])
                for row in critical) + ', below 20. Selection remains privately preselected, not broad or holdout evidence.'
                if critical else ' No single subject governs the D19 count floor.')
        candidate['notes'] = candidate.get('notes', []) + [note]
        final = grade(candidate)
        if differences(preview, final):
            raise RuntimeError('adding explanatory notes changed graded fields')
        destination = Path(args.out)
        destination.parent.mkdir(parents=True, exist_ok=True)
        destination.write_bytes(output.read_bytes())
    receipt = {'dialect': DIALECT, 'harness_commit': args.main_commit,
               'lane_commit': args.lane_commit, 'method': 'main lang-regrade from frozen lane counts; no extraction or oracle rerun',
               'main_reading_sha256': hashlib.sha256(main_raw).hexdigest(),
               'lane_reading_sha256': hashlib.sha256(lane_raw).hexdigest(),
               'result_reading_sha256': hashlib.sha256(destination.read_bytes()).hexdigest(),
               'lane_to_result_fields': lane_delta, 'main_to_result_fields': main_delta,
               'xref_precision_denominator': precision, 'xref_recall_denominator': recall,
               'grade': final['grade'], 'level_met': final['level_met'],
               'frozen_measurements_preserved': True, 'exact_harness_bytes': True,
               'leave_one_subject_out_count_floor': leave_one_out}
    Path(args.receipt).write_text(json.dumps(receipt, indent=2, sort_keys=True) + '\n')


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--indexbench', required=True)
    parser.add_argument('--lane-commit', required=True)
    parser.add_argument('--main-commit', required=True)
    parser.add_argument('--out', default=READING)
    parser.add_argument('--receipt', required=True)
    run(parser.parse_args())
