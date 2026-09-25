import {isDeepStrictEqual} from 'node:util';

const candidate={
  'missing-required':{value:'',submitted:null,valid:false,prompt:'Selection unavailable'},
  'missing-custom':{value:'',submitted:null,valid:false,prompt:'Choose cadence'},
  'missing-optional':{value:'',submitted:null,valid:true,prompt:'Selection unavailable'},
  'empty-duplicate':{value:'',submitted:null,valid:false,prompt:'Choose cadence'},
  known:{value:'weekly',submitted:'weekly',valid:true,prompt:'Weekly digest'},
  'native-default':{value:'daily',submitted:'daily',valid:true,prompt:'Daily digest'},
};
const historical={...candidate,
  'missing-required':{value:'daily',submitted:'daily',valid:true,prompt:'Daily digest'},
  'missing-custom':{value:'daily',submitted:'daily',valid:true,prompt:'Daily digest'},
  'missing-optional':{value:'daily',submitted:'daily',valid:true,prompt:'Daily digest'},
  'empty-duplicate':{value:'',submitted:'',valid:true,prompt:'No digest'},
};
const weekly={value:'weekly',submitted:'weekly',valid:true,prompt:'Weekly digest'};
const defectIDs=new Set(['missing-required','missing-custom','missing-optional','empty-duplicate']);
const cases=Object.keys(candidate),lanes=['wide-light','narrow-dark'];

export function candidateExpected(id){if(!Object.hasOwn(candidate,id))throw Error(`unknown case ${id}`);return {...candidate[id]};}
export function evaluateState(variant,id,phase,actual){
  if(!['baseline','candidate'].includes(variant)||!Object.hasOwn(candidate,id)||!['initial','reset','weekly'].includes(phase))throw Error(`unknown oracle key ${variant}/${id}/${phase}`);
  const expected=phase==='weekly'?weekly:(variant==='baseline'?historical:candidate)[id];
  return {ok:isDeepStrictEqual(actual,expected),expected:{...expected},candidate:candidateExpected(id),baselineDefect:variant==='baseline'&&phase!=='weekly'&&defectIDs.has(id)};
}
export function auditReport(report){
  if(!['baseline','candidate'].includes(report?.variant)||!Array.isArray(report.observations))throw Error('invalid select-state report');
  const seen=new Set();
  for(const cell of report.observations){
    const key=`${cell.lane}/${cell.case}`;if(seen.has(key)||!lanes.includes(cell.lane)||!cases.includes(cell.case))throw Error(`invalid observation ${key}`);seen.add(key);
    if(!isDeepStrictEqual(cell.expected,candidateExpected(cell.case)))throw Error(`candidate oracle changed: ${key}`);
    if(!Array.isArray(cell.trace)||cell.trace.length!==3)throw Error(`trace shape changed: ${key}`);
    for(const [index,phase] of ['initial','reset','weekly'].entries()){
      const step=cell.trace[index];if(step?.step!==phase||!evaluateState(report.variant,cell.case,phase,step.state).ok)throw Error(`state mismatch: ${key}/${phase}`);
    }
  }
  if(seen.size!==cases.length*lanes.length)throw Error(`observed ${seen.size} cells, want ${cases.length*lanes.length}`);
  return {variant:report.variant,cells:seen.size,states:seen.size*3};
}
