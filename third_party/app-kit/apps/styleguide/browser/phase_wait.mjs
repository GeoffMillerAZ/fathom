// Private acquisition barrier: an observed response is not a complete phase.
import {performance} from 'node:perf_hooks';
import {setTimeout as pause} from 'node:timers/promises';

export async function waitForPhase({inspect,expected,timeoutMs=10000}) {
  if(typeof inspect!=='function'||!Array.isArray(expected)||expected.length===0||
      !Number.isFinite(timeoutMs)||timeoutMs<=0)throw Error('Invalid phase barrier inputs');
  const wanted=[...expected].sort(),deadline=performance.now()+timeoutMs;
  for(;;){
    const {requests,responses,pending}=inspect();
    if(!Array.isArray(requests)||!Array.isArray(responses)||!Number.isSafeInteger(pending)||pending<0)throw Error('Invalid phase barrier observation');
    if(requests.length>wanted.length||responses.length>wanted.length)throw Error('Unexpected phase traffic');
    if(responses.some(r=>r.read_error||r.sizes_error))throw Error('Phase response acquisition failed');
    const key=r=>`${r.method} ${r.path}`;
    if(requests.length===wanted.length&&responses.length===wanted.length){
      if(JSON.stringify(requests.map(key).sort())!==JSON.stringify(wanted)||JSON.stringify(responses.map(key).sort())!==JSON.stringify(wanted))throw Error('Phase closure differs');
      const completed=responses.every(r=>Number.isFinite(r.finished_ms)&&Number.isSafeInteger(r.response_body_bytes)&&r.response_body_bytes>0);
      if(completed&&pending===0)return;
    }
    if(performance.now()>=deadline)throw Error('Complete phase response deadline exceeded');
    await pause(5);
  }
}
