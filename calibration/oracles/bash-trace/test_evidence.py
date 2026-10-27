import copy,hashlib,json,subprocess,sys,tempfile,unittest
from pathlib import Path
from unittest.mock import patch
import evidence


class HistoricalReceiptTests(unittest.TestCase):
    def setUp(self):
        self.data=json.loads(evidence.DEFAULT.read_text())
        self.data['current_code_sha256']={p.name:evidence.sha(p) for p in evidence.HERE.iterdir() if p.is_file()}
        parent=evidence.REPO/'workspace/tmp';parent.mkdir(parents=True,exist_ok=True)
        temp=tempfile.TemporaryDirectory(dir=parent);self.addCleanup(temp.cleanup)
        self.root=Path(temp.name)
        self.reading=self.root/'docs/bench/readings/lang/bash.json'
        self.reading.parent.mkdir(parents=True)
        self.reading.write_bytes((evidence.REPO/'docs/bench/readings/lang/bash.json').read_bytes())

    def save_reading(self,reading):
        self.reading.write_text(json.dumps(reading))
        self.data['canonical_reading_sha256']=hashlib.sha256(self.reading.read_bytes()).hexdigest()

    def check(self):
        # Only the current reading lives in the fixture. Historical Git objects
        # remain real; current code hashes were refreshed in setUp.
        with patch.object(evidence,'REPO',self.root):
            return evidence.check(self.data)

    def test_metric_change_cannot_delete_followup_guard(self):
        reading=json.loads(self.reading.read_text());reading['grade']='fixture-changed-grade'
        self.save_reading(reading);del self.data['followup']
        with self.assertRaisesRegex(AssertionError,'followup required'):
            self.check()

    def test_revision_swap_must_match_receipt_recorded_at_that_revision(self):
        # This inherited revision has identical decoder bytes, but its receipt
        # still records the earlier reading rather than its refreshed notes.
        revision='938893258c6bdb10987ff779e699cf796e22a500'
        historical=subprocess.check_output(['git','show',revision+':docs/bench/readings/lang/bash.json'],cwd=evidence.REPO)
        self.data['followup']['historical_replay_revision']=revision
        self.data['followup']['historical_canonical_reading_sha256']=hashlib.sha256(historical).hexdigest()
        self.data['runtime']['code_revision']=revision
        for row in self.data['runtime']['subjects'].values():row['decoder_code_revision']=revision
        with self.assertRaisesRegex(AssertionError,'historical receipt reading hash'):
            self.check()

    def test_metric_change_with_guard_stays_refused(self):
        reading=json.loads(self.reading.read_text());reading['grade']='fixture-changed-grade'
        self.save_reading(reading)
        with self.assertRaisesRegex(AssertionError,'beyond inherited notes'):
            self.check()

    def test_revision_swap_without_followup_still_requires_guard(self):
        revision='938893258c6bdb10987ff779e699cf796e22a500'
        self.data['runtime']['code_revision']=revision
        for row in self.data['runtime']['subjects'].values():row['decoder_code_revision']=revision
        del self.data['followup']
        with self.assertRaisesRegex(AssertionError,'followup required'):
            self.check()

    def test_notes_only_change_is_allowed(self):
        reading=json.loads(self.reading.read_text());reading['notes']='fixture notes only'
        self.save_reading(reading)
        self.assertTrue(self.check())

    def test_no_followup_needed_when_reading_matches_historical_bytes(self):
        revision=self.data['followup']['historical_replay_revision']
        self.reading.write_bytes(subprocess.check_output(['git','show',revision+':docs/bench/readings/lang/bash.json'],cwd=evidence.REPO))
        self.data['canonical_reading_sha256']=hashlib.sha256(self.reading.read_bytes()).hexdigest()
        del self.data['followup']
        self.assertTrue(self.check())

    def test_revision_is_validated_before_git(self):
        for revision in ['--output=fixture','a'*39,'z'*40,None]:
            with self.subTest(revision=revision):
                data=copy.deepcopy(self.data)
                data['followup']['historical_replay_revision']=revision
                data['runtime']['code_revision']=revision
                for row in data['runtime']['subjects'].values():row['decoder_code_revision']=revision
                with patch('redecode.subprocess.check_output') as git,patch.object(evidence,'REPO',self.root):
                    with self.assertRaisesRegex(AssertionError,'40-hex'):
                        evidence.check(data,verify_code=False)
                    git.assert_not_called()

    def test_check_and_historical_hash_work_outside_repo(self):
        fixture=self.root/'receipt.json';fixture.write_text(json.dumps(self.data))
        foreign=self.root/'unrelated'
        subprocess.run(['git','init','--quiet',str(foreign)],check=True)
        code=('import sys,json;sys.path.insert(0,sys.argv[1]);import evidence;'
              'evidence.check(json.load(open(sys.argv[2])));'
              'evidence.historical_hash(sys.argv[3],"redecode.py")')
        result=subprocess.run([sys.executable,'-B','-c',code,str(evidence.HERE),str(fixture),
                               self.data['followup']['historical_replay_revision']],
                              cwd=foreign,text=True,capture_output=True)
        self.assertEqual(result.returncode,0,result.stderr)


if __name__=='__main__':unittest.main()
