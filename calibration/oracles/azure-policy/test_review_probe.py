"""Binding review: refusal privacy, native isolation and receipt boundaries."""
import json
import subprocess
import tempfile
import unittest
from contextlib import ExitStack
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import patch
import probe_linux as probe


class ReviewProbeTest(unittest.TestCase):
    def launch_refusal(self, error=None, controls_failed=False):
        with tempfile.TemporaryDirectory() as directory, ExitStack() as stack:
            root = Path(directory)
            out, cache = root / 'out', root / 'cache'
            stack.enter_context(patch('sys.argv', ['probe', '-cache', str(cache), '-out', str(out)]+(['--controls-failed'] if controls_failed else [])))
            stack.enter_context(patch.object(probe.platform, 'system', return_value='Linux'))
            stack.enter_context(patch.object(probe.platform, 'machine', return_value='x86_64'))
            stack.enter_context(patch.object(probe.os, 'getloadavg', return_value=(0, 0, 0)))
            stack.enter_context(patch.object(probe.os, 'nice'))
            stack.enter_context(patch.object(probe.shutil, 'disk_usage', return_value=SimpleNamespace(free=30*1024**3)))
            stack.enter_context(patch.object(probe.shutil, 'which', return_value='/usr/bin/unshare'))
            prepare = stack.enter_context(patch.object(probe, 'prepare'))
            run = stack.enter_context(patch.object(probe.subprocess, 'run', side_effect=error,
                                                   return_value=SimpleNamespace(returncode=0 if controls_failed else 1)))
            popen=stack.enter_context(patch.object(probe.subprocess,'Popen'))
            code = probe.main()
            receipt = json.loads((out / 'receipt.json').read_text())
            self.assertEqual(2, code)
            probe.validate(receipt, probe.CONTRACT)
            prepare.assert_not_called()
            popen.assert_not_called()
            self.assertEqual('filesystem_controls' if controls_failed else 'userns', receipt['failed_stage'])
            self.assertNotIn(directory, json.dumps(receipt))
            self.assertIn('kernel_capabilities', receipt)
            return receipt

    def test_preflight_failure_refuses_before_prepare(self):
        self.launch_refusal()

    def test_captured_ubuntu24_uid_map_refusal_is_private_and_stops_prepare(self):
        # T-R12: same public run 40394016064; kernel 6.17.0-1022-azure.
        # Replay only the captured stderr and refusal, not a local Linux run.
        stderr=probe.HERE.joinpath('runner-userns-ubuntu-24.04.stderr.txt').read_bytes()
        def refuse(command,**kwargs):
            self.assertIn('--user',command)
            self.assertIn('--map-root-user',command)
            kwargs['stderr'].write(stderr)
            return SimpleNamespace(returncode=1)
        receipt=self.launch_refusal(error=refuse)
        self.assertEqual('userns',receipt['failed_stage'])
        self.assertEqual(len(stderr),receipt['preflight_stderr_bytes'])
        self.assertEqual(probe.digest(stderr),receipt['preflight_stderr_sha256'])
        self.assertNotIn('Operation not permitted',json.dumps(receipt))

    def test_required_kernel_control_failure_refuses_before_prepare(self):
        self.launch_refusal(controls_failed=True)

    def test_preflight_timeout_never_publishes_argv_or_path(self):
        error = subprocess.TimeoutExpired(['/private/identity/probe', '/private/identity/input'], 10)
        receipt = self.launch_refusal(error)
        self.assertNotIn('/private/', json.dumps(receipt))
        self.assertEqual('TimeoutExpired', receipt['failure_class'])

    def test_vendor_summary_rejects_noninteger_error_codes(self):
        with self.assertRaises(ValueError):
            probe.summary({'error': {'code': '/private/identity/free-text'}})

    def test_receipt_schema_rejects_free_text_and_unknown_fields(self):
        from receipt_contract import validate
        good = probe.receipt_base()
        good['status'] = 'probe_refused'
        validate(good, probe.CONTRACT)
        for key, value in [('reason', '/private/person'), ('failure_class', 'person'),
                           ('capability_keys', ['../../person']), ('worker_returncode', True),
                           ('initialize', {'unknown': '/private/person'})]:
            with self.subTest(key=key), self.assertRaises(ValueError):
                validate(dict(good, **{key:value}), probe.CONTRACT)
        with self.assertRaises(ValueError):
            validate(dict(good, kernel_capabilities={'unexpected': 1}), probe.CONTRACT)

    def test_namespace_worker_cannot_use_host_network(self):
        with patch.object(probe.os, 'getpid', return_value=1), \
                patch.object(probe.socket, 'if_nameindex', return_value=[(1,'lo'),(2,'eth0')]), \
                patch.object(probe.subprocess, 'Popen') as popen:
            with self.assertRaisesRegex(ValueError, 'loopback'):
                probe.worker(Path('/unused'), Path('/unused'))
            popen.assert_not_called()

    def test_one_evaluation_error_does_not_skip_remaining_controls(self):
        from unittest.mock import MagicMock
        first = MagicMock()
        first.request.side_effect = ConnectionRefusedError('/private/identity')
        second = MagicMock()
        second.getresponse.return_value.status = 200
        second.getresponse.return_value.read.return_value = b'{}'
        third = MagicMock()
        third.getresponse.return_value.status = 200
        third.getresponse.return_value.read.return_value = b'{}'
        with tempfile.TemporaryDirectory() as d, patch.object(probe, 'wait_for_port') as ready, \
                patch.object(probe.http.client, 'HTTPConnection', side_effect=[first, second, third]):
            rows = probe.evaluate_controls(json.loads(probe.INPUT.read_text()), Path(d))
        ready.assert_called_once()
        self.assertEqual(3, len(rows))
        self.assertEqual('ConnectionRefusedError', rows[0]['failure_class'])
        self.assertEqual([200,200], [r['http_status'] for r in rows[1:]])
        self.assertNotIn('/private/', json.dumps(rows))

    def test_import_and_loopback_refusals_are_distinct(self):
        for last, stage in [(40,'import'),(41,'loopback'),(42,'mountns')]:
            with self.subTest(stage=stage), tempfile.TemporaryDirectory() as d, ExitStack() as stack:
                root=Path(d);out=root/'out'
                stack.enter_context(patch('sys.argv',['probe','-cache',str(root/'cache'),'-out',str(out)]))
                for target,value in [('system','Linux'),('machine','x86_64')]:
                    stack.enter_context(patch.object(probe.platform,target,return_value=value))
                stack.enter_context(patch.object(probe.os,'getloadavg',return_value=(0,0,0)))
                stack.enter_context(patch.object(probe.shutil,'disk_usage',return_value=SimpleNamespace(free=30*1024**3)))
                stack.enter_context(patch.object(probe.shutil,'which',return_value='/usr/bin/unshare'))
                prepare=stack.enter_context(patch.object(probe,'prepare'))
                run=stack.enter_context(patch.object(probe.subprocess,'run',side_effect=[SimpleNamespace(returncode=n) for n in [0,0,0,last]]))
                self.assertEqual(2,probe.main())
                prepare.assert_not_called()
                self.assertEqual(stage,json.loads((out/'receipt.json').read_text())['failed_stage'])
                argv=run.call_args.args[0];code=argv[argv.index('-c')+1]
                self.assertIn('--mount',argv)
                # Execute only our authored preflight with all OS operations mocked.
                with patch.object(probe,'enable_loopback'), patch.object(probe,'isolate_filesystem'), patch.object(probe.socket,'if_nameindex',return_value=[(1,'lo')]), patch('sys.argv',['preflight',str(probe.HERE),str(out/'worker'),str(root/'cache'),str(root)]):
                    exec(compile(code,'authored-preflight','exec'),{})

    def launched_worker(self, *, poison=False, timeout=False, invalid_trace=False, contradicted_trace=False):
        from unittest.mock import MagicMock
        from receipt_contract import validate
        with tempfile.TemporaryDirectory() as d, ExitStack() as stack:
            root=Path(d).resolve();out=root/'out';public=root/'public'/'receipt.json'
            stack.enter_context(patch('sys.argv',['probe','-cache',str(root/'cache'),'-out',str(out),'-publish',str(public)]))
            stack.enter_context(patch.object(probe.platform,'system',return_value='Linux'))
            stack.enter_context(patch.object(probe.platform,'machine',return_value='x86_64'))
            stack.enter_context(patch.object(probe.os,'getloadavg',return_value=(0,0,0)))
            stack.enter_context(patch.object(probe.os,'nice'))
            stack.enter_context(patch.object(probe.shutil,'disk_usage',return_value=SimpleNamespace(free=30*1024**3)))
            stack.enter_context(patch.object(probe,'trace_identity',return_value=dict(sha256='0'*64,version=[5,16])))
            stack.enter_context(patch.object(probe.resource,'setrlimit'))
            original_is_file=Path.is_file
            stack.enter_context(patch.object(Path,'is_file',lambda p: True if str(p)=='/usr/bin/strace' else original_is_file(p)))
            stack.enter_context(patch.object(probe.shutil,'which',return_value='/usr/bin/unshare'))
            stack.enter_context(patch.object(probe.subprocess,'run',return_value=SimpleNamespace(returncode=0)))
            stack.enter_context(patch.object(probe,'prepare'))
            verify=stack.enter_context(patch.object(probe,'verify'))
            copied=stack.enter_context(patch.object(probe,'copy_payload'))
            if timeout: stack.enter_context(patch.object(probe,'TIMEOUT',0.001))
            if invalid_trace: stack.enter_context(patch.object(probe,'diagnostics',side_effect=ValueError('authored incomplete trace')))
            else: stack.enter_context(patch.object(probe,'diagnostics',return_value=dict(files=[],largest_private_file=None,files_over_previous_limit=0,sigxfsz_observed=False,limit_file=None,correlated_limit_files=0,vendor_exec_observed=not contradicted_trace,native_returncode=None if contradicted_trace else 0,files_truncated=False,omitted_file_events=0,snapshot_truncated=False)))
            process=MagicMock(pid=12345)
            def finish(*args,**kwargs):
                if timeout and kwargs.get('timeout'):
                    raise subprocess.TimeoutExpired(['/private/vendor'],180)
                receipt=probe.receipt_base();receipt.update(status='native_probe_failed',failed_stage='initialize',native_executed=True,filesystem_readonly_outside_out=True,vendor_capabilities_dropped=True,filesystem_dotdot_writes_refused=True,filesystem_write_probes=15,native_returncode=0,initialize=probe.summary({'error':{'code':-1}}),stderr_bytes=0,stderr_sha256='0'*64,lsp_bytes=0,lsp_sha256='0'*64)
                if poison:receipt['arbitrary_vendor_field']='/private/person'
                (out/'worker'/'receipt.json').write_text(json.dumps(receipt))
                return 2
            process.wait.side_effect=finish
            popen=stack.enter_context(patch.object(probe.subprocess,'Popen',return_value=process))
            kill=stack.enter_context(patch.object(probe.os,'killpg'))
            self.assertEqual(2,probe.main())
            self.assertTrue(verify.called)
            self.assertEqual(out/'worker'/'payload',copied.call_args.args[1])
            command=popen.call_args.args[0]
            for flag in ['--pid','--fork','--kill-child','--mount','--mount-proc']:self.assertIn(flag,command)
            self.assertTrue(popen.call_args.kwargs['start_new_session'])
            kill.assert_called_with(12345,probe.signal.SIGKILL)
            receipt=json.loads(public.read_text());validate(receipt,probe.CONTRACT)
            self.assertNotIn('/private/',json.dumps(receipt))
            return receipt

    def test_normal_worker_exit_kills_group_and_validates_before_publication(self):
        self.assertEqual('native_probe_failed',self.launched_worker()['status'])

    def test_worker_timeout_kills_group_and_publishes_only_timeout_flags(self):
        self.assertEqual('probe_timeout',self.launched_worker(timeout=True)['status'])

    def test_worker_extra_field_refuses_entire_receipt(self):
        self.assertEqual('receipt_invalid',self.launched_worker(poison=True)['status'])

    def test_symlink_receipt_refused(self):
        from receipt_contract import read_receipt
        with tempfile.TemporaryDirectory() as d:
            root=Path(d);target=root/'target';target.write_text('{}');link=root/'link';link.symlink_to(target)
            with self.assertRaises(OSError):read_receipt(link,probe.CONTRACT)

    def test_completed_receipt_requires_clean_shutdown(self):
        from receipt_contract import validate
        receipt=probe.receipt_base();receipt.update(status='probe_complete_requires_analysis',shutdown_clean=False)
        with self.assertRaises(ValueError):validate(receipt,probe.CONTRACT)

    def test_shutdown_timeout_cannot_leave_completed_status(self):
        from unittest.mock import MagicMock
        with tempfile.TemporaryDirectory() as d, ExitStack() as stack:
            out=Path(d)/'out';out.mkdir()
            stack.enter_context(patch.object(probe.os,'getpid',return_value=1))
            stack.enter_context(patch.object(probe.socket,'if_nameindex',return_value=[(1,'lo')]))
            stack.enter_context(patch.object(probe,'enable_loopback'))
            stack.enter_context(patch.object(probe,'isolate_filesystem',return_value=dict(filesystem_readonly_outside_out=True,vendor_capabilities_dropped=True,filesystem_dotdot_writes_refused=True,filesystem_write_probes=15)))
            stack.enter_context(patch.object(probe.resource,'setrlimit'))
            stack.enter_context(patch.object(probe,'kill_group'))
            process=MagicMock(returncode=0)
            process.stderr=__import__('io').BytesIO(b'')
            stack.enter_context(patch.object(probe.subprocess,'Popen',return_value=process))
            lsp=MagicMock()
            def response(ident,*args):
                if ident==5:raise TimeoutError('/private/shutdown')
                return {'result':{'capabilities':{}}} if ident==1 else {'result':None}
            lsp.response.side_effect=response
            stack.enter_context(patch.object(probe,'LSP',return_value=lsp))
            evaluations=[dict(variant=v,http_status=200,sha256='0'*64,bytes=2) for v in ['valid','unknown_parameter','changed_input_fact']]
            stack.enter_context(patch.object(probe,'evaluate_controls',return_value=evaluations))
            reader,writer=probe.os.pipe()
            try:
                self.assertEqual(2,probe.worker(Path(d)/'payload',out,trace_fd=writer))
            finally:probe.os.close(reader)
            receipt=json.loads((out/'receipt.json').read_text())
            self.assertEqual('native_probe_failed',receipt['status'])
            self.assertEqual('shutdown',receipt['failed_stage'])
            self.assertFalse(receipt['shutdown_clean'])
            self.assertNotIn('/private/',json.dumps(receipt))

    def test_workflow_contract_uploads_only_receipt(self):
        here=probe.HERE
        workflow=(here/'probe-linux-x64.workflow.yml').read_text()
        self.assertIn("on:\n  push:\n    branches: ['tools/azure-policy-probe-run']",workflow)
        self.assertIn('actions/upload-artifact@cf430e030ddbb5b0abf93d22962f4752f3646cd9 # v7 (Node 24)',workflow)
        self.assertIn('runs-on: ubuntu-22.04',workflow)
        self.assertNotIn('container:',workflow)
        self.assertIn('persist-credentials: false',workflow)
        self.assertIn('path: ${{ runner.temp }}/azure-policy-public/receipt.json',workflow)
        self.assertNotIn('workflow_dispatch',workflow)
        script=(here/'run_probe_workflow.sh').read_text()
        self.assertIn('-min-free-gib 1 -max-load 6',script)
        self.assertIn('"$private/launcher.log" 2>&1',script)
        self.assertIn('validated=true',script)
        self.assertIn('main() {',script)
        self.assertTrue(script.endswith('main "$@"\nexit\n'))

    def test_closed_schema_accepts_complete_bounded_control_receipt(self):
        from receipt_contract import write_receipt, read_receipt
        receipt=probe.receipt_base()
        receipt.update(status='probe_complete_requires_analysis',native_executed=True,
                       shutdown_clean=True,netns_loopback_only=True,measured_interfaces=1,native_returncode=0,
                       filesystem_readonly_outside_out=True,vendor_capabilities_dropped=True,filesystem_dotdot_writes_refused=True,filesystem_write_probes=15,
                       initialize=probe.summary({'result':{}}),cache_command=probe.summary({'result':None}),
                       definition=probe.summary({'result':[]}),references=probe.summary({'result':[]}),
                       evaluations=[dict(variant=v,http_status=200,sha256='0'*64,bytes=2) for v in
                                    ['valid','unknown_parameter','changed_input_fact']])
        with tempfile.TemporaryDirectory() as d:
            path=Path(d)/'receipt.json'
            write_receipt(path,receipt,probe.CONTRACT)
            self.assertEqual(receipt,read_receipt(path,probe.CONTRACT))
            receipt['evaluations'][1]['failure_class']='TimeoutError'
            write_receipt(path,receipt,probe.CONTRACT)
            with self.assertRaisesRegex(ValueError,'failure'):read_receipt(path,probe.CONTRACT)

    def test_fifo_receipt_refuses_without_waiting_for_a_writer(self):
        from receipt_contract import read_receipt
        with tempfile.TemporaryDirectory() as d:
            path=Path(d)/'receipt.json';probe.os.mkfifo(path)
            with self.assertRaisesRegex(ValueError,'regular'):read_receipt(path,probe.CONTRACT)

    def test_receipt_cap_plus_one_refuses_whole_unit(self):
        from receipt_contract import read_receipt,MAX_RECEIPT
        with tempfile.TemporaryDirectory() as d:
            path=Path(d)/'receipt.json';path.write_bytes(b' '*(MAX_RECEIPT+1))
            with self.assertRaisesRegex(ValueError,'cap'):read_receipt(path,probe.CONTRACT)

    def test_invalid_diagnostic_preserves_validated_native_outcome(self):
        receipt=self.launched_worker(invalid_trace=True)
        self.assertEqual('native_probe_failed',receipt['status'])
        self.assertEqual('initialize',receipt['failed_stage'])
        self.assertTrue(receipt['native_executed'])
        self.assertEqual(0,receipt['native_returncode'])
        self.assertEqual(-1,receipt['initialize']['error_code'])
        self.assertEqual(0,receipt['stderr_bytes'])
        self.assertEqual(0,receipt['lsp_bytes'])
        self.assertEqual('refused',receipt['diagnostic_status'])
        self.assertIsNone(receipt['write_diagnostics'])

    def test_missing_strace_refuses_before_prepare(self):
        with tempfile.TemporaryDirectory() as directory,ExitStack() as stack:
            root=Path(directory).resolve();out=root/'out'
            stack.enter_context(patch('sys.argv',['probe','-cache',str(root/'cache'),'-out',str(out)]))
            stack.enter_context(patch.object(probe.platform,'system',return_value='Linux'))
            stack.enter_context(patch.object(probe.platform,'machine',return_value='x86_64'))
            stack.enter_context(patch.object(probe.os,'getloadavg',return_value=(0,0,0)))
            stack.enter_context(patch.object(probe.shutil,'disk_usage',return_value=SimpleNamespace(free=30*1024**3)))
            stack.enter_context(patch.object(probe.shutil,'which',return_value='/usr/bin/unshare'))
            stack.enter_context(patch.object(probe.subprocess,'run',return_value=SimpleNamespace(returncode=0)))
            stack.enter_context(patch.object(Path,'is_file',return_value=False))
            prepare=stack.enter_context(patch.object(probe,'prepare'))
            self.assertEqual(2,probe.main());prepare.assert_not_called()
            receipt=json.loads((out/'receipt.json').read_text())
            self.assertEqual('tracing',receipt['failed_stage']);self.assertFalse(receipt['native_executed'])

    def test_contradicted_trace_publishes_refusal_without_native_claim(self):
        value=self.launched_worker(contradicted_trace=True)
        self.assertEqual('contradicted',value['diagnostic_status'])
        self.assertEqual('probe_refused',value['status']);self.assertEqual('tracing',value['failed_stage'])
        self.assertIsNone(value['native_executed']);self.assertIsNone(value['native_returncode'])
        self.assertEqual(-1,value['initialize']['error_code'])
        self.assertEqual(0,value['stderr_bytes']);self.assertEqual(0,value['lsp_bytes'])
