from dataclasses import replace
import json
from pathlib import Path
import tempfile
import unittest

from combine_runtime import combine
from trace_decode import Limits


class PhaseCombinationTests(unittest.TestCase):
    def setUp(self):
        root=Path(__file__).resolve().parents[3]/'workspace/tmp'
        root.mkdir(parents=True,exist_ok=True)
        temp=tempfile.TemporaryDirectory(dir=root)
        self.addCleanup(temp.cleanup)
        self.paths=[Path(temp.name)/(str(n)+'.json') for n in range(2)]
        row={'pwsh':'7.6.9','refused':False,'calls':[['native.ps1',3,4,'caller','target','alias']],
             'native_events':1,'original_breakpoints':1,'retained_breakpoints':1,
             'driver_status':'completed','exception_sha256':None}
        for path in self.paths:path.write_text(json.dumps(row))
        self.population={'sites':[{'path':'native.ps1','line':3,'column':4,'from':'caller','written':'alias','kind':'command'}]}

    def test_duplicate_events_are_not_double_coverage(self):
        raw=combine(self.paths,self.population)
        self.assertEqual(len(raw['calls']),1)
        self.assertEqual(raw['native_events'],2)

    def test_both_original_phases_retained(self):
        row=json.loads(self.paths[1].read_text())
        row['calls'][0][1]=5
        self.paths[1].write_text(json.dumps(row))
        self.assertEqual(len(combine(self.paths,self.population)['calls']),2)

    def test_combined_byte_cap_plus_one(self):
        total=sum(p.stat().st_size for p in self.paths)
        with self.assertRaisesRegex(ValueError,'combined native byte cap'):
            combine(self.paths,self.population,replace(Limits(),raw_bytes=total-1))

    def test_combined_row_cap_plus_one(self):
        row=json.loads(self.paths[1].read_text())
        row['calls'][0][1]=5
        self.paths[1].write_text(json.dumps(row))
        with self.assertRaisesRegex(ValueError,'combined native row cap'):
            combine(self.paths,self.population,replace(Limits(),rows=1))

    def test_exception_in_either_phase_remains_disclosed(self):
        row=json.loads(self.paths[1].read_text())
        row['driver_status']='driver-exception'
        row['exception_sha256']='0'*64
        self.paths[1].write_text(json.dumps(row))
        result=combine(self.paths,self.population)
        self.assertNotEqual(result['driver_status'],'completed')
        self.assertEqual(result['phase_exception_hashes'],[None,'0'*64])

    def test_test_failures_are_not_completed_even_when_observer_returned(self):
        row=json.loads(self.paths[0].read_text())
        row['phase_statuses']=[{'status':'stopped-after-p-test-failures','process_exit':0,'test_results':{'failed':3}}]
        self.paths[0].write_text(json.dumps(row))
        result=combine(self.paths,self.population)
        self.assertNotEqual(result['driver_status'],'completed')
        self.assertEqual(result['phase_statuses'][0]['test_results']['failed'],3)


if __name__=='__main__':unittest.main()
