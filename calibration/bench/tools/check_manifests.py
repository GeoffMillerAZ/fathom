"""Check the portable preparation commands without executing setup or tests."""
import json
from pathlib import Path
import subprocess

root = Path(__file__).resolve().parents[3]
for path in sorted((root / 'calibration/bench.d').glob('*.yaml')):
    data = json.loads(path.read_text().split('\n', 1)[1])
    bench = data['bench']
    for stage in ['setup', 'test']:
        command = ' && '.join(bench[stage])
        subprocess.run(['sh', '-n', '-c', command], check=True)
        for binding in ['TMPDIR=/work/tmp', 'UV_PROJECT_ENVIRONMENT=/work/venv',
                        'UV_CACHE_DIR=/work/uv-cache', 'COREPACK_HOME=/work/corepack',
                        'GOFLAGS=-mod=readonly', 'GOCACHE=/work/gocache']:
            assert binding in command, (path.name, stage, binding)
        assert 'cd /work/source' in command
    assert 'git -C /subject ls-files -z > /work/tracked-files' in bench['setup'][0]
    assert 'tar -C /work/source -xf /work/source.tar' in bench['setup'][0]
    assert 'GOPROXY=off' in bench['test'][0]
    assert 'PIP_NO_INDEX=1' in bench['test'][0]
    if 'uv' in bench['toolchain']['versions']:
        assert 'pip install --require-hashes -r /bench-inputs/uv-requirements.txt' in bench['setup'][0]
    if data['name'] == 'textualize-rich':
        assert 'cp /bench-inputs/textualize-rich-requirements.txt /work/requirements.txt' in bench['setup'][0]
print('PASS: manifest shell syntax, explicit environment, source/export staging and offline flags')
