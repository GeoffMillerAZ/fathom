"""Build the pinned bench-only typed Kong probe into private scratch."""
import argparse
import hashlib
import json
import os
import shutil
import subprocess
from pathlib import Path
from instrument_native import instrument

HERE = Path(__file__).resolve().parent
MODULE = 'github.com/kong/go-database-reconciler@v1.42.4'


def run(command, cwd=None, **kwargs):
    if shutil.disk_usage(Path.home()).free < 20 * 1024**3:
        raise ValueError('free disk below 20 GiB before native build step')
    return subprocess.check_output(command, cwd=cwd or HERE / 'native', **kwargs)


def build(out):
    for key in ('GOCACHE', 'GOMODCACHE', 'TMPDIR', 'GOTMPDIR'):
        if not os.environ.get(key):
            raise ValueError('explicit private ' + key + ' required')
    if os.environ.get('GOWORK') != 'off' or os.environ.get('GOTOOLCHAIN') != 'local':
        raise ValueError('GOWORK=off and GOTOOLCHAIN=local required')
    out.mkdir(parents=True, exist_ok=False)
    info = json.loads(run(['go', 'mod', 'download', '-json', MODULE], text=True))
    if info.get('Error'):
        raise ValueError('pinned module download failed')
    root = Path(info['Dir'])
    pins = json.loads((HERE / 'native-source-pins-r1.json').read_text())
    for name in ('reader.go', 'types.go', 'builder.go'):
        expected = next(row['sha256'] for row in pins['sources'] if row['path'] == name)
        if hashlib.sha256((root / 'pkg/file' / name).read_bytes()).hexdigest() != expected:
            raise ValueError('native source identity mismatch: ' + name)
    # Go forbids overlays beneath GOMODCACHE even though the overlay is
    # read-only. Use a checksum-verified private copy and a scratch modfile;
    # neither the committed module nor the shared cache gets a local replace.
    # A stable relative replacement is also needed: -trimpath does not
    # remove absolute local replacement names from Go's build-info section.
    for name in ('go.mod', 'go.sum', 'main.go', 'main_test.go'):
        shutil.copyfile(HERE / 'native' / name, out / name)
    native_copy = out / 'native-source'
    shutil.copytree(root, native_copy)
    for directory, _, _ in os.walk(native_copy):
        Path(directory).chmod(0o700)
    modfile = out / 'probe.mod'
    modfile.write_bytes((HERE / 'native/go.mod').read_bytes())
    if (HERE / 'native/go.sum').exists():
        modfile.with_suffix('.sum').write_bytes((HERE / 'native/go.sum').read_bytes())
    run(['go', 'mod', 'edit', '-modfile', str(modfile),
         '-replace', 'github.com/kong/go-database-reconciler=./native-source'], cwd=out)
    original = native_copy / 'pkg/file/types.go'
    observer_dir = out / 'observer'
    observer_dir.mkdir()
    observed = observer_dir / 'types.observed.go'
    observed.write_bytes(instrument(original.read_bytes()))
    overlay = out / 'overlay.json'
    overlay.write_text(json.dumps({'Replace': {str(original): str(observed)}}))
    run(['go', 'mod', 'tidy', '-modfile', str(modfile)], cwd=out)
    run(['go', 'build', '-trimpath', '-buildvcs=false', '-modfile', str(modfile), '-overlay', str(overlay), '-o', str(out / 'kong-native'), '.'], cwd=out)
    receipt = dict(tool='Kong owner typed loader probe', version='go-database-reconciler v1.42.4',
                   deck_version='1.68.0', native_source_commit=pins['vendor_library_commit'],
                   go_version=run(['go', 'version'], text=True).strip(),
                   build_flags=['-trimpath', '-buildvcs=false'], local_replacement='./native-source',
                   module_sum=info['Sum'], module_go_mod_sum=info['GoModSum'],
                   binary_sha256=hashlib.sha256((out / 'kong-native').read_bytes()).hexdigest(),
                   original_types_sha256=hashlib.sha256(original.read_bytes()).hexdigest(),
                   observed_types_sha256=hashlib.sha256(observed.read_bytes()).hexdigest(),
                   instrument_sha256=hashlib.sha256((HERE / 'instrument_native.py').read_bytes()).hexdigest(),
                   probe_source_sha256=hashlib.sha256((HERE / 'native/main.go').read_bytes()).hexdigest(),
                   native_runtime_executed=False, calibration_measured=False)
    (out / 'build-receipt.json').write_text(json.dumps(receipt, indent=2) + '\n')
    return receipt


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('-out', type=Path, required=True)
    parser.add_argument('-verify-second-build', type=Path)
    args = parser.parse_args()
    receipt = build(args.out.resolve())
    if args.verify_second_build:
        second = args.verify_second_build.resolve()
        check = build(second)
        if receipt['binary_sha256'] != check['binary_sha256']:
            raise ValueError('independently located native builds differ')
        receipt.update(reproducible_across_two_scratch_roots=True,
                       second_binary_sha256=check['binary_sha256'])
        (args.out / 'build-receipt.json').write_text(json.dumps(receipt, indent=2) + '\n')
        shutil.rmtree(second)
    print(json.dumps(receipt, sort_keys=True))
