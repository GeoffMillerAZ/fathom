"""Authored controls for the writable-scratch-only vendor boundary."""
import errno
import json
import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch


class FilesystemIsolationTest(unittest.TestCase):
    def test_mount_table_cap_plus_one_refuses_whole_setup(self):
        import filesystem_isolation as fs
        with patch.object(Path,'read_text',return_value='authored\n'*1025):
            with self.assertRaisesRegex(ValueError,'cap'):fs.mount_table()

    def test_writable_protected_directory_refuses(self):
        import filesystem_isolation as fs
        with tempfile.TemporaryDirectory() as directory:
            path=Path(directory)
            with self.assertRaisesRegex(ValueError,'writable'):
                fs.require_readonly(path)
            self.assertEqual([],list(path.iterdir()))

    def test_only_erofs_proves_readonly(self):
        import filesystem_isolation as fs
        with patch.object(fs.os,'open',side_effect=OSError(errno.EROFS,'authored')):
            fs.require_readonly(Path('/authored'))
        with patch.object(fs.os,'open',side_effect=PermissionError(errno.EACCES,'authored')):
            with self.assertRaises(OSError):fs.require_readonly(Path('/authored'))

    def test_every_visible_mount_except_scratch_is_readonly(self):
        import filesystem_isolation as fs
        scratch=Path('/authored/scratch')
        mounts=[(Path('/'),'rw'),(Path('/home'),'rw'),(Path('/tmp'),'rw'),
                (Path('/proc'),'ro'),(scratch,'rw')]
        with patch.object(fs,'mount_table',return_value=mounts),patch.object(fs.os,'chroot') as chroot,patch.object(fs.os,'chdir'),patch.object(fs.subprocess,'run') as run:
            fs.mount_readonly(scratch)
        commands=[call.args[0] for call in run.call_args_list]
        self.assertIn(['/usr/bin/mount','--bind',str(scratch),str(scratch)],commands)
        self.assertNotIn(['/usr/bin/mount','--rbind','/','/'],commands)
        chroot.assert_not_called()
        remounted=[c[-1] for c in commands if 'remount,bind,ro' in c]
        self.assertEqual({'/','/home','/tmp'},set(remounted))
        self.assertNotIn(str(scratch),remounted)
        self.assertEqual('/',remounted[-1])

    def test_overmounted_proc_hides_its_inherited_children(self):
        import filesystem_isolation as fs
        data=('10 1 8:1 / / rw - ext4 root rw\n'
              '20 10 0:1 / /proc rw - proc proc rw\n'
              '21 20 0:2 / /proc/sys/fs/binfmt_misc rw - autofs systemd-1 rw\n'
              '22 21 0:3 / /proc/sys/fs/binfmt_misc rw - binfmt_misc binfmt_misc rw\n'
              '30 20 0:4 / /proc rw - proc proc rw\n')
        self.assertEqual([(Path('/'),'rw'),(Path('/proc'),'rw')],fs.parse_mountinfo(data))

    # T-R12: GitHub-hosted run 40394016064, image ubuntu22/20261012.322,
    # kernel 6.8.0-1064-azure. The 29-line capture is stored verbatim.
    def test_real_runner_mountinfo_selects_topmost_binfmt_mount(self):
        import filesystem_isolation as fs
        raw=Path(__file__).with_name('runner-mountinfo-ubuntu-22.04.txt').read_text()
        self.assertEqual(29,len(raw.splitlines()))
        mounts=fs.parse_mountinfo(raw)
        self.assertEqual(28,len(mounts))
        self.assertEqual(len(mounts),len({path for path,_ in mounts}))
        options=dict(mounts)[Path('/proc/sys/fs/binfmt_misc')]
        self.assertEqual('rw,nosuid,nodev,noexec,relatime',options)

    def test_real_runner_proc_overmount_skips_hidden_binfmt_remount(self):
        import filesystem_isolation as fs
        raw=Path(__file__).with_name('runner-mountinfo-ubuntu-22.04.txt').read_text()
        # Only this added procfs row models --mount-proc; it was not captured.
        raw+='900 27 0:99 / /proc rw,nosuid,nodev,noexec,relatime - proc proc rw\n'
        mounts=fs.parse_mountinfo(raw)
        self.assertEqual(27,len(mounts))
        self.assertEqual(len(mounts),len({path for path,_ in mounts}))
        self.assertNotIn(Path('/proc/sys/fs/binfmt_misc'),dict(mounts))
        with patch.object(fs,'mount_table',return_value=mounts),patch.object(fs.os,'chdir'),patch.object(fs.subprocess,'run') as run:
            fs.mount_readonly(Path('/authored/scratch'))
        paths=[call.args[0][-1] for call in run.call_args_list if 'remount,bind,ro' in call.args[0]]
        self.assertNotIn('/proc/sys/fs/binfmt_misc',paths)
        self.assertEqual(1,paths.count('/proc'))
        self.assertEqual(1,paths.count('/'))

    def test_last_sibling_mount_hides_earlier_mount_and_descendants(self):
        import filesystem_isolation as fs
        data=('10 1 8:1 / / rw - ext4 root rw\n'
              '20 10 0:1 / /opt rw - tmpfs tmpfs rw\n'
              '21 20 0:2 / /opt/hidden rw - tmpfs tmpfs rw\n'
              '30 10 0:3 / /opt ro - tmpfs tmpfs ro\n')
        self.assertEqual([(Path('/'),'rw'),(Path('/opt'),'ro')],fs.parse_mountinfo(data))

    def test_stacked_namespace_root_refuses_instead_of_hiding_real_submounts(self):
        import filesystem_isolation as fs
        data=('10 1 8:1 / / rw - ext4 root rw\n'
              '11 10 0:1 / /run rw - tmpfs tmpfs rw\n'
              '12 10 0:2 / /dev/shm rw - tmpfs tmpfs rw\n'
              '20 10 8:1 / / ro - ext4 root rw\n'
              '21 20 0:1 / /run ro - tmpfs tmpfs rw\n')
        with self.assertRaisesRegex(ValueError,'mount stacked on namespace root'):
            fs.parse_mountinfo(data)

    def test_write_controls_preserve_dot_dot_spellings(self):
        import filesystem_isolation as fs
        protected=[Path('/authored/checkout'),Path('/authored/temp')]
        with patch.object(fs,'require_no_privileges'),patch.object(fs,'require_readonly') as readonly,patch.object(fs.os,'open',side_effect=OSError(errno.EROFS,'authored')),patch.object(fs.tempfile,'TemporaryFile'):
            result=fs.write_controls(Path('/scratch'),protected)
        self.assertEqual([str(c.args[0]) for c in readonly.call_args_list],
                         ['/authored/checkout','/../authored/checkout','/proc/../authored/checkout',
                          '/authored/temp','/../authored/temp','/proc/../authored/temp'])
        self.assertEqual(9,result['filesystem_write_probes'])
        self.assertTrue(result['filesystem_dotdot_writes_refused'])

    def test_vendor_command_cannot_retain_mount_privileges(self):
        import filesystem_isolation as fs
        command=fs.restricted_command(['/authored/payload/PolicyLanguageServer'])
        for flag in ('--bounding-set=-all','--inh-caps=-all','--ambient-caps=-all','--no-new-privs'):
            self.assertIn(flag,command)
        good=''.join(key+':\t0\n' for key in ('CapInh','CapPrm','CapEff','CapBnd','CapAmb'))+'NoNewPrivs:\t1\n'
        with patch.object(Path,'read_text',return_value=good):fs.require_no_privileges()
        with patch.object(Path,'read_text',return_value=good.replace('CapEff:\t0','CapEff:\t1')):
            with self.assertRaisesRegex(ValueError,'capabilities'):fs.require_no_privileges()

    def test_write_boundary_measured_in_restricted_child(self):
        import filesystem_isolation as fs
        flags=dict(filesystem_readonly_outside_out=True,vendor_capabilities_dropped=True,filesystem_dotdot_writes_refused=True,filesystem_write_probes=9)
        with patch.object(fs.os,'getpid',return_value=1),patch.object(fs,'mount_readonly'),patch.object(fs.subprocess,'run',return_value=subprocess.CompletedProcess([],0,json.dumps(flags).encode())) as run:
            self.assertEqual(flags,fs.isolate_filesystem(Path('/scratch'),[Path('/checkout'),Path('/runner-temp')]))
        command=run.call_args.args[0]
        self.assertIn('--no-new-privs',command)
        self.assertEqual(subprocess.PIPE,run.call_args.kwargs['stdout'])

    def test_successful_write_probe_refuses_worker_before_vendor_popen(self):
        import probe_linux as probe
        with patch.object(probe.os,'getpid',return_value=1),patch.object(probe.socket,'if_nameindex',return_value=[(1,'lo')]),patch.object(probe,'isolate_filesystem',side_effect=ValueError('outside scratch is writable')),patch.object(probe.subprocess,'Popen') as popen:
            with self.assertRaisesRegex(ValueError,'writable'):
                probe.worker(Path('/unused'),Path('/unused'),[Path('/checkout'),Path('/runner-temp')])
            popen.assert_not_called()

    def test_outer_private_log_refuses_symlink(self):
        import probe_linux as probe
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory);target=root/'target';target.write_text('unchanged')
            (root/'log').symlink_to(target)
            with self.assertRaises(OSError):probe.private_log(root/'log',b'hostile')
            self.assertEqual('unchanged',target.read_text())

    def test_private_payload_is_a_verified_copy_not_the_download(self):
        import probe_linux as probe
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory);payload=root/'download';payload.mkdir()
            (payload/'PolicyLanguageServer').write_bytes(b'authored payload')
            pin=probe.payload_identity(payload)
            copied=root/'worker'/'payload';copied.parent.mkdir()
            probe.copy_payload(payload,copied,pin)
            (copied/'PolicyLanguageServer').write_bytes(b'worker mutation')
            self.assertEqual(b'authored payload',(payload/'PolicyLanguageServer').read_bytes())
            with self.assertRaisesRegex(ValueError,'identity'):probe.verify(copied,pin)

    def test_native_execution_requires_measured_readonly_flags(self):
        import probe_linux as probe
        base=probe.receipt_base();base.update(status='native_probe_failed',native_executed=True)
        with self.assertRaises(ValueError):probe.validate(base,probe.CONTRACT)
        base.update(filesystem_readonly_outside_out=True,vendor_capabilities_dropped=True,filesystem_dotdot_writes_refused=True,filesystem_write_probes=15)
        probe.validate(base,probe.CONTRACT)
        with self.assertRaises(ValueError):probe.validate(dict(base,filesystem_readonly_outside_out=False),probe.CONTRACT)

    def test_private_publication_exception_still_publishes_outer_refusal(self):
        import probe_linux as probe
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory);out=root/'private';out.mkdir();public=root/'public';public.mkdir()
            base=probe.receipt_base();base.update(status='probe_refused',failed_stage='mountns')
            original=probe.write_receipt
            def fail_private(path,*args):
                if path.parent==out:raise OSError('authored private tail refusal')
                return original(path,*args)
            with patch.object(probe,'write_receipt',side_effect=fail_private):
                self.assertEqual(2,probe.publish_receipt(base,out,public/'receipt.json',probe.CONTRACT))
            result=json.loads((public/'receipt.json').read_text())
            self.assertEqual('receipt_invalid',result['status'])
            self.assertEqual('receipt',result['failed_stage'])

    def test_fixed_temporary_name_cannot_poison_publication(self):
        import probe_linux as probe
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory);(root/'.receipt-validated.tmp').symlink_to(root/'absent')
            value=probe.receipt_base();value.update(status='probe_refused',failed_stage='mountns')
            probe.write_receipt(root/'receipt.json',value,probe.CONTRACT)
            self.assertEqual(value,json.loads((root/'receipt.json').read_text()))
