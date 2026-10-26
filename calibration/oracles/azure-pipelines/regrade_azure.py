"""Reproduce the Azure reading and review disclosures on both pinned scopes."""
import argparse
import json
import subprocess
from pathlib import Path
import yaml
from azure_oracle import measure

NOTE = ('Azure review: all 16 literal same-file dependsOn pairs are from microsoft/azure-pipelines-yaml; '
        'dotnet/runtime contributes no such pairs. The original 227/228 includes 91 pipeline file entities '
        'selected by a shared file rule; independent stage/job/deployment declarations match 136/137. '
        'PyYAML rejects templates/deploy-to-existing-kubernetes-cluster.yml (mustache starter). '
        'Duplicate extends occurrences remain in truth; decoder work is held by T-R2. '
        'Earlier YAML claims win before Azure dispatch; ENGINE regression validation is recorded separately.')

if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('-indexbench', type=Path, required=True)
    parser.add_argument('-subjects', type=Path, required=True)
    parser.add_argument('-out', type=Path, required=True)
    args = parser.parse_args()
    args.out.mkdir(parents=True, exist_ok=True)
    reading = args.out / 'azure-pipelines.json'
    if reading.exists():
        raise SystemExit('fresh output required')
    shard = yaml.safe_load(Path('calibration/corpus.d/azure-pipelines.yaml').read_text())
    retime = [str(args.indexbench.resolve()), 'lang-retime', '-reading', str(reading), '-runs', '3', '-write']
    for repo in shard['repos']:
        checkout = args.subjects / repo['name'].split('/')[-1]
        if subprocess.check_output(['git', '-C', str(checkout), 'rev-parse', 'HEAD'], text=True).strip() != repo['sha']:
            raise SystemExit('wrong subject pin')
        if subprocess.check_output(['git', '-C', str(checkout), 'status', '--porcelain'], text=True).strip():
            raise SystemExit('dirty subject')
        subject = checkout / repo.get('subpath', '')
        data, rejected = measure(subject)
        truth = args.out / (repo['name'].replace('/', '--') + '.json')
        truth.write_text(json.dumps(data, indent=2) + '\n')
        print(repo['name'], 'truth definitions', len(data['definitions']), 'refs', len(data['references']), 'rejected', rejected, flush=True)
        subprocess.run([str(args.indexbench.resolve()), 'lang-oracle', '-dialect', 'azure-pipelines',
                        '-subject', str(subject), '-subject-name', repo['name'], '-tool-oracle', str(truth),
                        '-out', str(reading), '-note', NOTE], check=True)
        retime += ['-subject', repo['name'] + '=' + str(subject)]
    subprocess.run(retime, check=True)
