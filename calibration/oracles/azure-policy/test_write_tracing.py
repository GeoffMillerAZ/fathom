"""Authored diagnostics bounds; no vendor launch or Linux authority claim."""
import io
import json
import os
import tempfile
import unittest
from pathlib import Path
from write_tracing import BoundedCapture, trace_command
from write_diagnostics import diagnostics
from receipt_contract import read_receipt

class WriteTracingTest(unittest.TestCase):
    def test_trace_decodes_fds_but_excludes_buffer_contents(self):
        command=trace_command('/usr/bin/strace',9,['/authored/program'])
        self.assertIn('-yy',command);self.assertIn('-s',command)
        self.assertEqual('0',command[command.index('-s')+1])
        self.assertIn('/proc/self/fd/9',command)
        self.assertNotIn('-e write=',command)
    def test_capture_cap_plus_one_keeps_bounded_prefix_and_flags_refusal(self):
        output=io.BytesIO();capture=BoundedCapture(io.BytesIO(b'12345'),output,4)
        capture.thread.join(1)
        self.assertEqual(b'1234',output.getvalue());self.assertTrue(capture.overflow)
    def test_capture_exact_limit_reaches_eof(self):
        output=io.BytesIO();capture=BoundedCapture(io.BytesIO(b'1234'),output,4)
        capture.thread.join(1)
        self.assertFalse(capture.overflow);self.assertIsNone(capture.error)
    def test_original_real_receipt_replays_under_its_frozen_schema(self):
        here=Path(__file__).parent
        schema=json.loads((here/'probe-receipt-run40407284519.schema.json').read_text())
        receipt=read_receipt(here/'probe-receipt-run40407284519.json',schema)
        self.assertEqual(-25,receipt['native_returncode']);self.assertEqual('initialize',receipt['failed_stage'])
        self.assertTrue(receipt['filesystem_readonly_outside_out']);self.assertFalse(receipt['tier_granted'])
    def test_unicode_and_escaped_path_identity_matches_snapshot(self):
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory).resolve();home=root/'home';home.mkdir()
            path=home/'ab';path.write_bytes(b'x')
            escaped=str(path)[:-1]+r'\142'
            result=diagnostics(f'1 write(3<{escaped}>, ""..., 1) = 1\n',root)
            self.assertEqual(1,len(result['files']));self.assertEqual(1,result['files'][0]['final_size_bytes'])
    def test_two_distinct_limit_files_are_not_collapsed_to_a_guess(self):
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory).resolve();(root/'home').mkdir()
            trace=''.join(f'{pid} ftruncate(3</memfd:authored{pid}>(deleted), 9) = -1 EFBIG (File too large)\n{pid} --- SIGXFSZ {{}} ---\n' for pid in [1,2])
            result=diagnostics(trace,root)
            self.assertEqual(2,result['correlated_limit_files']);self.assertIsNone(result['limit_file'])
    def test_primary_native_exit_is_attested_separately_from_tracer_status(self):
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory).resolve();(root/'home').mkdir()
            binary=root/'payload'/'PolicyLanguageServer'
            trace=f'3 execve("{binary}", [""...], 0x1 /* 2 vars */) = 0\n3 --- SIGXFSZ {{}} ---\n3 +++ killed by SIGXFSZ +++\n'
            value=diagnostics(trace,root)
            self.assertTrue(value['vendor_exec_observed']);self.assertEqual(-25,value['native_returncode'])
    def test_successful_bare_descriptor_refuses_missing_path_coverage(self):
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory).resolve();(root/'home').mkdir()
            with self.assertRaises(ValueError):diagnostics('1 write(3, ""..., 1) = 1\n',root)
    def test_public_native_receipt_without_trace_refuses(self):
        from receipt_contract import write_receipt
        from probe_linux import receipt_base, CONTRACT
        value=receipt_base();value.update(status='native_probe_failed',failed_stage='initialize',native_executed=True,filesystem_readonly_outside_out=True,vendor_capabilities_dropped=True,filesystem_dotdot_writes_refused=True,filesystem_write_probes=15)
        with tempfile.TemporaryDirectory() as directory:
            path=Path(directory)/'receipt.json'
            with self.assertRaises(ValueError):write_receipt(path,value,CONTRACT,public=True)
            self.assertFalse(path.exists())
    def test_trace_writer_is_outside_private_pid_and_proc_view(self):
        from vendor_exec import namespace_command
        command=namespace_command(Path('/private/home'),Path('/private/payload'))
        for flag in ['--mount','--pid','--fork','--kill-child','--mount-proc']:self.assertIn(flag,command)
    def test_private_log_cap_and_symlink_cannot_bypass_output_budget(self):
        from probe_linux import read_private_log, LIMIT
        with tempfile.TemporaryDirectory() as directory:
            path=Path(directory)/'private-log'
            with path.open('wb') as target:target.truncate(LIMIT+1)
            with self.assertRaises(ValueError):read_private_log(path)
            link=Path(directory)/'link';link.symlink_to(path)
            with self.assertRaises(OSError):read_private_log(link)
    def test_unsafe_relative_filename_is_redacted_instead_of_refusing(self):
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory);(root/'home').mkdir();(root/'home'/'unsafe\nname').write_bytes(b'')
            self.assertIsNone(diagnostics('',root)['files'][0]['relative_path'])
    def test_directory_count_cap_plus_one_flags_partial_snapshot(self):
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory);home=root/'home';home.mkdir()
            for index in range(257):(home/str(index)).mkdir()
            self.assertTrue(diagnostics('',root)['snapshot_truncated'])
    def test_vendor_has_ordinary_signal_semantics_and_only_raises_child_limit(self):
        from unittest.mock import patch
        import vendor_exec as vendor
        with patch.object(vendor,'require_no_privileges') as privileges, patch.object(vendor.os,'getpid',return_value=2), patch.object(vendor.os,'getppid',return_value=1), patch.object(vendor,'close_extra_fds') as closed, patch.object(vendor.resource,'setrlimit') as limits, patch.object(vendor.os,'execv') as executed:
            vendor.execute(Path('/private/payload'))
            privileges.assert_called_once();closed.assert_called_once()
            limits.assert_any_call(vendor.resource.RLIMIT_FSIZE,(256*1024**2,256*1024**2))
            self.assertEqual('/private/payload/PolicyLanguageServer',executed.call_args.args[0])
        with patch.object(vendor,'require_no_privileges'),patch.object(vendor.os,'getpid',return_value=1):
            with self.assertRaises(ValueError):vendor.execute(Path('/private/payload'))
    def test_relative_path_total_cap_plus_one_redacts(self):
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory).resolve();(root/'home').mkdir()
            relative='home/'+'/'.join(['a'*127]*3+['b'*124])
            self.assertEqual(513,len(relative))
            trace=f'1 write(3<{root/relative}>, ""..., 1) = 1\n'
            self.assertIsNone(diagnostics(trace,root)['files'][0]['relative_path'])
    def test_receipt_write_cap_plus_one_refuses_before_creating_file(self):
        import copy
        from probe_linux import receipt_base, CONTRACT
        from receipt_contract import write_receipt
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory).resolve();(root/'home').mkdir();(root/'home'/'a').write_bytes(b'')
            diagnostic=diagnostics('',root);row=diagnostic['files'][0]
            row['relative_path']='home/'+'/'.join(['a'*127]*3+['b'*123])
            diagnostic['files']=[copy.deepcopy(row) for _ in range(65)]
            value=receipt_base();value.update(status='probe_refused',failed_stage='tracing',write_diagnostics=diagnostic)
            path=root/'receipt.json'
            with self.assertRaises(ValueError):write_receipt(path,value,CONTRACT)
            self.assertFalse(path.exists())

    def test_worst_admissible_diagnostic_and_complete_receipt_fit_together(self):
        from probe_linux import CONTRACT
        from receipt_contract import write_receipt, MAX_RECEIPT
        def largest(schema):
            if '$ref' in schema:
                target=CONTRACT
                for part in schema['$ref'][2:].split('/'):target=target[part]
                return largest(target)
            if 'const' in schema:return schema['const']
            if 'enum' in schema:return max(schema['enum'],key=lambda x:len(str(x)))
            kind=schema['type'];kind=next(x for x in kind if x!='null') if isinstance(kind,list) else kind
            if kind=='object':return {key:largest(value) for key,value in schema['properties'].items()}
            if kind=='array':return [largest(schema['items']) for _ in range(schema.get('maxItems',schema.get('minItems',0)))]
            if kind=='boolean':return True
            if kind=='integer':return schema.get('maximum',2**63-1)
            if kind=='string':return 'home/'+'/'.join(['a'*127]*3+['b'*123]) if 'home/' in schema.get('pattern','') else '0'*64
            raise AssertionError('unhandled closed-schema type')
        import write_diagnostics as writes
        self.assertEqual(writes.MAX_FILES,CONTRACT['$defs']['write_diagnostics']['properties']['files']['maxItems'])
        value=largest(CONTRACT)
        value.update(status='native_probe_failed',native_executed=True,failed_stage='initialize',diagnostic_status='partial',trace_writer_hidden_from_vendor=True)
        value['write_diagnostics']['native_returncode']=value['native_returncode']
        with tempfile.TemporaryDirectory() as directory:
            output=Path(directory)/'receipt.json'
            write_receipt(output,value,CONTRACT,public=True)
            self.assertLessEqual(output.stat().st_size,MAX_RECEIPT)

    def test_contradicted_trace_withdraws_native_claim_and_is_publishable(self):
        from probe_linux import receipt_base, attach_diagnostics, CONTRACT
        from receipt_contract import write_receipt
        for trace_kind in ['missing_exec','missing_exit']:
            with self.subTest(trace_kind=trace_kind), tempfile.TemporaryDirectory() as directory:
                root=Path(directory).resolve();(root/'home').mkdir()
                trace='' if trace_kind=='missing_exec' else f'1 execve("{root}/payload/PolicyLanguageServer", [], 0x1) = 0\n'
                value=receipt_base();value.update(status='native_probe_failed',failed_stage='initialize',native_executed=True,native_returncode=0)
                attach_diagnostics(value,trace,root,dict(sha256='0'*64,version=[5,16]))
                self.assertEqual('contradicted',value['diagnostic_status'])
                self.assertIsNone(value['native_executed']);self.assertIsNone(value['native_returncode'])
                self.assertEqual('probe_refused',value['status']);self.assertEqual('tracing',value['failed_stage'])
                self.assertIsNotNone(value['write_diagnostics'])
                write_receipt(root/'receipt.json',value,CONTRACT,public=True)
                for incorrect in [True,False]:
                    value['native_executed']=incorrect
                    with self.assertRaises(ValueError):write_receipt(root/'invalid.json',value,CONTRACT,public=True)

    def test_snapshot_entry_cap_plus_one_is_sorted_before_selection(self):
        from contextlib import contextmanager
        from unittest.mock import patch
        import write_diagnostics as writes
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory).resolve();home=root/'home';home.mkdir()
            for index in range(4098):(home/f'f{index:04}').touch()
            original=os.scandir
            results=[]
            for reverse in [False,True]:
                @contextmanager
                def reordered(path):
                    with original(path) as entries:ordered=sorted(entries,key=lambda entry:entry.name,reverse=reverse)
                    yield iter(ordered)
                with patch.object(writes.os,'scandir',reordered):results.append(writes.diagnostics('',root))
            self.assertTrue(results[0]['snapshot_truncated'])
            self.assertEqual(64,len(results[0]['files']))
            self.assertEqual(4032,results[0]['omitted_file_events'])
            self.assertEqual([f'home/f{index:04}' for index in range(64)],[row['relative_path'] for row in results[0]['files']])
            self.assertEqual(results[0],results[1])

    def test_snapshot_entry_cap_plus_one_flags_partial_snapshot(self):
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory).resolve();home=root/'home';home.mkdir()
            for index in range(4096):(home/f'f{index:04}').touch()
            self.assertFalse(diagnostics('',root)['snapshot_truncated'])
            (home/'f4096').touch()
            value=diagnostics('',root)
            self.assertTrue(value['snapshot_truncated'])
            self.assertEqual(4032,value['omitted_file_events'])

    def test_trace_target_cap_plus_one_preserves_partial_native_result(self):
        from probe_linux import receipt_base, attach_diagnostics, CONTRACT
        from receipt_contract import write_receipt
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory).resolve();(root/'home').mkdir()
            trace=f'1 execve("{root}/payload/PolicyLanguageServer", [], 0x1) = 0\n'
            trace+=''.join(f'1 write(3<{root}/home/target{index:03}>, ""..., 1) = 1\n' for index in range(65))
            trace+='1 +++ exited with 0 +++\n'
            value=receipt_base();value.update(status='native_probe_failed',failed_stage='initialize',native_executed=True,native_returncode=0,filesystem_readonly_outside_out=True,vendor_capabilities_dropped=True,filesystem_dotdot_writes_refused=True,filesystem_write_probes=15)
            attach_diagnostics(value,trace,root,dict(sha256='0'*64,version=[5,16]))
            diagnostic=value['write_diagnostics']
            self.assertEqual('partial',value['diagnostic_status']);self.assertTrue(value['native_executed'])
            self.assertTrue(diagnostic['files_truncated']);self.assertFalse(diagnostic['snapshot_truncated'])
            self.assertEqual(64,len(diagnostic['files']));self.assertEqual(1,diagnostic['omitted_file_events'])
            self.assertEqual(64,sum(row['observed_write_bytes'] for row in diagnostic['files']))
            write_receipt(root/'receipt.json',value,CONTRACT,public=True)


    def test_real_pre_vendor_refusal_preserves_unspent_rerun_evidence(self):
        from probe_linux import CONTRACT
        value=read_receipt(Path(__file__).with_name('probe-receipt-run40487438655.json'),CONTRACT)
        self.assertEqual('filesystem_controls',value['failed_stage'])
        self.assertEqual('probe_refused',value['status'])
        self.assertEqual('ValueError',value['failure_class'])
        self.assertIs(value['native_executed'],False)
        self.assertFalse(value['calibration_measured']);self.assertFalse(value['tier_granted'])
