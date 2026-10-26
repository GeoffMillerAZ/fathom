import json,os
from pathlib import Path
import signal,subprocess,sys,tempfile,unittest


class OwnedSupervisorTests(unittest.TestCase):
    def setUp(self):
        workspace=Path(__file__).resolve().parents[3]/'workspace/tmp'
        workspace.mkdir(parents=True,exist_ok=True)
        temp=tempfile.TemporaryDirectory(dir=workspace)
        self.addCleanup(temp.cleanup)
        self.root=Path(temp.name)
        self.status=self.root/'status.json'

    def run_child(self,script,timeout):
        return subprocess.run([sys.executable,'-B',str(Path(__file__).with_name('supervise.py').resolve()),
            '--status',str(self.status),'--timeout',str(timeout),'--',sys.executable,'-B','-c',script],
            start_new_session=True,capture_output=True)

    def test_normal_exit_records_driver_code_before_self_group_cleanup(self):
        result=self.run_child('raise SystemExit(7)',1)
        self.assertEqual(result.returncode,-signal.SIGKILL)
        data=json.loads(self.status.read_text())
        self.assertEqual(data['driver_exit'],7)
        self.assertFalse(data['timeout'])

    def test_timeout_refuses_completed_driver_claim(self):
        result=self.run_child('import time;time.sleep(10)',.05)
        self.assertEqual(result.returncode,-signal.SIGKILL)
        data=json.loads(self.status.read_text())
        self.assertTrue(data['timeout'])
        self.assertIsNone(data['driver_exit'])


if __name__=='__main__':unittest.main()
