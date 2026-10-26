"""Grade the complete canonical and separate holdout cohorts from T-R7 output."""
import argparse
import json
import subprocess
from pathlib import Path

HERE = Path(__file__).resolve().parent
NOTE = ('T-R7/P-T1: vendor CircleCI YAML language server 0.54.0, pinned by version/checksum, '
        'queries every command/executor declaration across all five canonical and three separate holdout configs. '
        'Every returned location is classified and all workflow executor parameter/matrix pairs enter recall truth. '
        'Same-name command/job defect: definition points to the job and references on the command also return its workflow job entry; '
        'the compiler adjudicates the command expansion, and the extra workflow entry is recorded as an oracle error. '
        'Two external orb uses remain unscored. Compiler cross-checks are required by the instrument for namespace errors. '
        'No extractor or parameter-matrix change. Small-cohort timing is non-informative; no dialect-wide throughput claim.')

if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('-indexbench', type=Path, required=True)
    parser.add_argument('-native-dir', type=Path, required=True)
    parser.add_argument('-out-dir', type=Path, required=True)
    args = parser.parse_args()
    protocol = json.loads((HERE / 'lsp-protocol-r3.json').read_text())
    args.out_dir.mkdir(parents=True, exist_ok=True)
    for cohort in ('canonical', 'holdout'):
        output = args.out_dir / (cohort + '.json')
        if output.exists():
            raise SystemExit('fresh output required: ' + str(output))
        subjects = [s for s in protocol['subjects'] if (s['source_shard'] == 'circleci.yaml') == (cohort == 'canonical')]
        for subject in subjects:
            name = subject['name'].split('/')[-1]
            identity = name if cohort == 'canonical' else subject['name']
            command = [str(args.indexbench.resolve()), 'lang-oracle', '-dialect', 'circleci',
                       '-subject', subject['checkout'], '-subject-name', identity,
                       '-tool-oracle', str(args.native_dir / (name + '-defs.json')),
                       '-tool-oracle', str(args.native_dir / (name + '-refs.json')),
                       '-out', str(output), '-note', NOTE]
            subprocess.run(command, check=True)
        command = [str(args.indexbench.resolve()), 'lang-retime', '-reading', str(output), '-runs', '3', '-write']
        for subject in subjects:
            identity = subject['name'].split('/')[-1] if cohort == 'canonical' else subject['name']
            command += ['-subject', identity + '=' + subject['checkout']]
        subprocess.run(command, check=True)
