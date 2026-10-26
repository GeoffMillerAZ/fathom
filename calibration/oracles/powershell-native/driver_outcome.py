"""Aggregate original driver summaries, without publishing test names or output."""
import re


def outcome(text,process_exit,driver_status):
    text=re.sub(r'\x1b\[[0-9;]*m','',text)
    stopped='P tests failed!' in text
    if stopped:
        rows=[tuple(map(int,m)) for m in re.findall(r'^passed (\d+), failed (\d+), total (\d+)\s*$',text,re.M)]
        if not rows:raise ValueError('missing original P-test summaries')
        counts={'passed':sum(r[0] for r in rows),'failed':sum(r[1] for r in rows),'skipped':0,
                'summary_rows':len(rows),'count_scope':'original P-test driver summaries; no remaining phase executed'}
        status='stopped-after-p-test-failures'
    else:
        rows=re.findall(r'Tests Passed: ([\d,]+), Failed: ([\d,]+), Skipped: ([\d,]+), Inconclusive: ([\d,]+), NotRun: ([\d,]+)',text)
        if not rows:raise ValueError('missing original final test summary')
        passed,failed,skipped,inconclusive,not_run=[int(x.replace(',','')) for x in rows[-1]]
        counts=dict(passed=passed,failed=failed,skipped=skipped,inconclusive=inconclusive,not_run=not_run,
                    summary_rows=1,count_scope='original final test-driver summary')
        status='tests-failed' if failed else 'tests-passed'
    if process_exit or driver_status!='completed':status='process-or-driver-failure'
    return dict(status=status,process_exit=process_exit,native_driver_status=driver_status,test_results=counts)
