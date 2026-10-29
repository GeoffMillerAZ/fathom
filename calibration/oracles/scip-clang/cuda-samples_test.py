"""Offline driver regressions. No compiler, indexer or measured subject required."""
import copy
import importlib.util
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import types
import unittest
from unittest.mock import patch

HERE = Path(__file__).resolve().parent
DRIVER = HERE / 'cuda-samples.sh'


class CUDAReadingNotes(unittest.TestCase):
    def test_recorded_reading_notes_only(self):
        spec = importlib.util.spec_from_file_location('cuda_notes', HERE / 'cuda-reading-notes.py')
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        original = json.loads((HERE.parents[2] / 'docs/bench/readings/lang/cuda.json').read_text())
        # Start with the historical wording even after the committed reading is refreshed.
        original['notes'] = [n.split('; the predicted 38th')[0] for n in original['notes']]
        original['notes'] = [re.sub(r'; the median run.*', '; the median run, 0.96, is this reading', n)
                             if ': throughput read ' in n else n for n in original['notes']]
        updated = module.refresh_notes(copy.deepcopy(original))
        self.assertEqual({k: v for k, v in original.items() if k != 'notes'},
                         {k: v for k, v in updated.items() if k != 'notes'})
        stand = next(n for n in updated['notes'] if 'stand in for a function' in n)
        self.assertIn('the predicted 38th extern "C" function is in eigenvalues/util.h, a header the dialect does not own', stand)
        throughput = next(n for n in updated['notes'] if ': throughput read ' in n)
        self.assertIn('median run 1 (wall 1.72)', throughput)
        self.assertIn('equal CPU rates retain run order', throughput)
        self.assertEqual(module.refresh_notes(copy.deepcopy(updated)), updated)


class CUDADatabaseLocation(unittest.TestCase):
    def test_translated_database_and_consumers(self):
        driver = DRIVER.read_text()
        # Execute the driver's complete database translation stage with compiler
        # subprocesses replaced by successful responses. No toolchain is invoked.
        stage = driver.split('# 3 and 4: the derived databases.\n', 1)[1]
        code = stage.split("<<'EOF'\n", 1)[1].split('\nEOF\n', 1)[0]
        with tempfile.TemporaryDirectory() as tmp:
            work = Path(tmp)
            subject = work / 'subject'
            subject.mkdir()
            source = subject / 'k.cu'
            source.write_text('void k() {}\n')
            entry = {'directory': str(subject), 'file': str(source), 'command': f'nvcc -c {source} -o k.o'}
            (work / 'nvcc_compile_commands.json').write_text(json.dumps([entry]))
            with patch('sys.argv', ['driver', str(work), str(subject), str(work / 'cuda')]), \
                 patch('subprocess.run', return_value=types.SimpleNamespace(returncode=0, stderr='')):
                exec(compile(code, 'cuda-samples.sh:translation', 'exec'), {})
            self.assertEqual(sorted(p.name for p in subject.iterdir()), ['k.cu'])
            database = work / 'compile_commands.json'
            self.assertEqual(json.loads(database.read_text())[0]['file'], str(source))
            # Native tier A must select the same external database as tier B.
            native = work / 'build-clang'
            native.mkdir()
            (native / 'compile_commands.json').write_text(json.dumps([entry]))
            selection = re.search(r'if \[ "\$scip_tier" = A \]; then.*?\nfi', driver, re.S).group()
            env = dict(os.environ, work=str(work), subj=str(subject), scip_tier='A')
            subprocess.run(['bash', '-eu', '-c', selection], env=env, check=True)
            self.assertEqual(json.loads(database.read_text()), [entry])
            self.assertEqual(sorted(p.name for p in subject.iterdir()), ['k.cu'])
            # Round 5 R5-1: the database is written at <work>/subject/build, where fathom's
            # include-path resolver reads it (internal/fusion/compdb.go), as an adopter's CMake build puts it.
            publish = re.search(r'\n(python3 - "\$work" "\$subj" "\$target" "\$companion" <<\'EOF\'\n.*?\nEOF)\n', driver, re.S).group(1)
            env.update(target='x86_64-pc-linux-gnu', companion=str(work / 'compdb' / 'cuda.compile_commands.json'))
            subprocess.run(['bash', '-eu', '-c', publish], env=env, check=True)
            resolved = subject / 'build' / 'compile_commands.json'
            self.assertEqual(json.loads(resolved.read_text()), json.loads(database.read_text()))
            self.assertEqual(sorted(str(p.relative_to(subject)) for p in subject.rglob('*') if p.is_file()),
                             ['build/compile_commands.json', 'k.cu'])
            # Exercise the real scip-clang launch line with a local recording stub: the index reads the
            # database the resolver reads.
            stub = work / 'run.sh'
            stub.write_text('test -f "$3/compile_commands.json" || exit 1\nprintf "%s" "$3" > "$4"\n')
            line = next(l for l in driver.splitlines() if l.startswith('bash "$here/run.sh"'))
            env.update(here=str(work), tools=str(work))
            subprocess.run(['bash', '-eu', '-c', line], env=env, check=True)
            self.assertEqual((work / 'index.scip').read_text(), str(subject / 'build'))
            # Host call and range companions must consume that same database.
            stub = work / 'go'
            stub.write_text('#!/bin/sh\nprintf "%s\\n" "$@" > "$CAPTURE"\n')
            stub.chmod(0o755)
            env.update(repo=str(HERE.parents[2]), PATH=str(work) + os.pathsep + env['PATH'])
            for kind in ('clang-calls-host.json', 'preproc.ranges.json'):
                line = next(l for l in driver.splitlines() if l.startswith('( cd ') and f'> "$work/{kind}"' in l)
                capture = work / 'args.txt'
                env['CAPTURE'] = str(capture)
                subprocess.run(['bash', '-eu', '-c', line], env=env, check=True)
                args = capture.read_text().splitlines()
                self.assertEqual(args[args.index('-compdb') + 1], str(database))

    def test_explicit_target_and_companion(self):
        # Round 7 R7-1: every entry of the database fathom's resolver reads names the target the run
        # compiles for, so the reading's compile_db records an explicit target, not the host default; the
        # same database is published subject-relative, without a toolkit path, for lang-retime -compile-db.
        driver = DRIVER.read_text()
        publish = re.search(r'\n(python3 - "\$work" "\$subj" "\$target" "\$companion" <<\'EOF\'\n.*?\nEOF)\n', driver, re.S).group(1)
        self.assertIn('companion="$repo/docs/bench/readings/lang/compdb/cuda.compile_commands.json"', driver)
        self.assertRegex(driver, r'\ntarget=\$\(clang\+\+ -print-target-triple\)')
        with tempfile.TemporaryDirectory() as tmp:
            work = Path(tmp)
            subject = work / 'subject'
            (subject / 'Samples' / 'a' / 'inc').mkdir(parents=True)
            cuda = work / 'cuda'
            src = subject / 'Samples' / 'a' / 'k.cu'
            cmd = ['clang++', '-x', 'cuda', '--cuda-path=' + str(cuda), '--cuda-host-only', '-I' + str(cuda / 'include'),
                   '-I', str(subject / 'Samples' / 'a' / 'inc'), '-isystem', str(cuda / 'targets' / 'include'),
                   '-I' + str(subject / 'Common'), '-DX=1', '-c', str(src), '-o', '/dev/null']
            entries = [{'directory': str(subject), 'file': str(src), 'command': ' '.join(cmd)},
                       {'directory': str(subject), 'file': str(subject / 'b.cu'),
                        'command': 'clang++ --target=x86_64-unknown-linux-gnu -c %s' % (subject / 'b.cu')}]
            (work / 'compile_commands.json').write_text(json.dumps(entries))
            companion = work / 'out' / 'cuda.compile_commands.json'
            env = dict(os.environ, work=str(work), subj=str(subject), target='x86_64-pc-linux-gnu', companion=str(companion))
            subprocess.run(['bash', '-eu', '-c', publish], env=env, check=True)
            database = json.loads((work / 'compile_commands.json').read_text())
            resolved = json.loads((subject / 'build' / 'compile_commands.json').read_text())
            self.assertEqual(resolved, database)
            first = database[0]['command'].split()
            self.assertEqual(first[:2], ['clang++', '--target=x86_64-pc-linux-gnu'])
            self.assertEqual(first[2:], cmd[1:])
            # an entry that already names a target keeps it, once
            self.assertEqual(database[1]['command'].split().count('--target=x86_64-unknown-linux-gnu'), 1)
            self.assertNotIn('--target=x86_64-pc-linux-gnu', database[1]['command'])
            published = json.loads(companion.read_text())
            self.assertEqual(published[0], {'directory': '.', 'file': 'Samples/a/k.cu', 'command': ' '.join(
                ['clang++', '--target=x86_64-pc-linux-gnu', '-x', 'cuda', '--cuda-host-only', '-I', 'Samples/a/inc',
                 '-ICommon', '-DX=1', '-c', 'Samples/a/k.cu'])})
            self.assertEqual(published[1]['file'], 'b.cu')
            text = companion.read_text()
            self.assertNotIn(str(work), text)
            self.assertFalse(any(t.startswith('/') for e in published for t in e['command'].split()))
            self.assertEqual(sorted(str(p.relative_to(subject)) for p in subject.rglob('*') if p.is_file()),
                             ['build/compile_commands.json'])

    def test_ast_defs_jobs(self):
        # Round 3 R3-1: one decoded translation-unit AST peaks near 12.4 GB, so a small host
        # runs the clang-ast-defs step at AST_DEFS_JOBS=1; unset, it keeps -jobs 2.
        driver = DRIVER.read_text()
        line = next(l for l in driver.splitlines() if l.startswith('( cd ') and '> "$work/preproc.ranges.json"' in l)
        with tempfile.TemporaryDirectory() as tmp:
            work = Path(tmp)
            stub = work / 'go'
            stub.write_text('#!/bin/sh\nprintf "%s\\n" "$@" > "$CAPTURE"\n')
            stub.chmod(0o755)
            env = {k: v for k, v in os.environ.items() if k != 'AST_DEFS_JOBS'}
            env.update(work=str(work), subj=str(work), scip_tier='B', repo=str(HERE.parents[2]),
                       CAPTURE=str(work / 'args.txt'), PATH=str(work) + os.pathsep + env['PATH'])
            for jobs, want in ((None, '2'), ('1', '1')):
                if jobs:
                    env['AST_DEFS_JOBS'] = jobs
                subprocess.run(['bash', '-eu', '-c', line], env=env, check=True)
                args = (work / 'args.txt').read_text().splitlines()
                self.assertEqual(args[args.index('-jobs') + 1], want)


if __name__ == '__main__':
    unittest.main()
