import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import test from 'node:test';
import {deriveLineageSample,evaluateLineagePair} from './lineage-oracle.mjs';

const here=new URL('.',import.meta.url);
const v1=JSON.parse(await readFile(new URL('performance-contract.json',here)));
const v2=JSON.parse(await readFile(new URL('cache-lineage-contract.json',here)));
const sha=char=>char.repeat(64),origin='http://127.0.0.1:43123';
const specs=[
  ['/enhanced','Document','text/html',101],['/source.css','Stylesheet','text/css',102],
  ['/hero.css','Stylesheet','text/css',103],['/assets/logo-light.svg','Image','image/svg+xml',104],
  ['/assets/logo-dark.svg','Image','image/svg+xml',105],['/assets/hero.jpg','Image','image/jpeg',106],
  ['/assets/inter-subset.woff2','Font','font/woff2',107],
];
const resources=()=>specs.map(([path,type,mime,bytes],index)=>({path,type,mime,bytes,sha256:sha(String(index+1))}));
const bodyError={name:'Error',command:'Network.getResponseBody',message:v2.cacheProof.warmMissingBodyError.replace(/^Error: /,'')};
const expected=()=>({schema:1,origin,contextId:'context-1',browserPid:1234,manifestSHA:sha('a'),
  font:{url:origin+'/assets/inter-subset.woff2',sha256:sha('7'),bytes:107,mime:'font/woff2',cacheControl:v2.cacheProof.staticCacheControl},
  allowedPaths:specs.map(([path])=>path),missingBodyError:structuredClone(bodyError),resources:resources()});
const painted=()=>[{selector:'body > main:nth-child(1)',fonts:[{isCustomFont:true,familyName:'Inter Variable',postScriptName:'InterVariable',glyphCount:8}]}];
const responseHeaders=resource=>({'content-type':resource.mime,'content-length':String(resource.bytes),
  'cache-control':resource.path==='/enhanced'?'no-store':v2.cacheProof.staticCacheControl,etag:`"sha256-${resource.sha256}"`,
  'content-security-policy':"default-src 'none'",'x-content-type-options':'nosniff',connection:'close'});
function phase(name,base) {
  const warm=name==='warm',loader=name+'-loader',items=resources();
  const requests=items.map((resource,index)=>{const requestSequence=base+index*5,cached=warm&&resource.path!=='/enhanced';return {
    id:`${name}-${index}`,url:origin+resource.path,method:'GET',type:resource.type,frameId:'frame-1',loaderId:loader,
    status:200,redirect:false,fromDiskCache:cached,fromPrefetchCache:false,fromServiceWorker:false,
    requestHeaders:{accept:'*/*'},responseHeaders:responseHeaders(resource),requestSequence,responseSequence:requestSequence+2,
    cacheSequence:cached?requestSequence+3:null,finishSequence:requestSequence+4,
    body:warm&&resource.type==='Font'?{available:false,error:structuredClone(bodyError)}:{available:true,bytes:resource.bytes,sha256:resource.sha256},
  }});
  const served=warm?[items[0]]:items;
  return {documentURL:origin+'/enhanced',frameId:'frame-1',loaderId:loader,navigationType:'navigate',requests,
    serverRequests:served.map(resource=>({url:origin+resource.path,method:'GET',status:200,bytes:resource.bytes,sha256:resource.sha256,
      requestHeaders:{accept:'*/*'},responseHeaders:responseHeaders(resource)})),
    unexpectedRequests:[],metrics:{scripts:0,fontReady:true,imagesDecoded:3,visibleImages:2,
      visibleLinks:['/start','/learn'],keyboardLinks:['/start','/learn'],usableMs:500,fcpMs:200,lcpMs:300,heapBytes:1000},
    render:{pngSHA:sha('c'),pngWidth:390,pngHeight:1000,geometry:[{x:0,y:0,width:390,height:1000}],paintedFonts:painted(),fontReady:true}};
}
const valid=()=>({origin,contextId:'context-1',browserPid:1234,manifestBefore:sha('a'),manifestAfter:sha('a'),repeat:1,width:390,scheme:'light',
  setup:{freshContext:true,cacheClearedBeforeCold:true,serviceWorkersBlocked:true,interceptionEnabled:false,applicationScripts:0,priorRequests:0},
  navigationCount:2,cold:phase('cold',10),warm:phase('warm',100)});
const codes=result=>result.failures.map(item=>item.code);
const rejects=(name,mutate,want)=>test(name,()=>{const input=valid(),pins=expected();mutate(input,pins);const out=evaluateLineagePair(v1,v2,pins,input);assert.equal(out.passed,false);if(want)assert.ok(codes(out).some(code=>code.includes(want)),JSON.stringify(out.failures));});

test('derives a passing pair without trusting supplied summaries',()=>{
  const pair=valid();pair.cold.bodyBytes=0;pair.warm.cacheHits=0;pair.warm.assetFailures=[];
  const before=structuredClone(pair),out=evaluateLineagePair(v1,v2,expected(),pair);
  assert.equal(out.passed,true,JSON.stringify(out.failures));assert.deepEqual(pair,before);
  assert.equal(out.samples[0].bodyBytes,728);assert.equal(out.samples[1].bodyBytes,101);
  assert.equal(out.samples[0].cacheHits,0);assert.equal(out.samples[1].cacheHits,6);
  assert.equal(out.lineage.directWarmBodyObserved,false);assert.equal(out.lineage.verification,'not-verified');
});
test('accepts request-bound memory-cache events without claiming disk cache',()=>{
  const pair=valid();for(const request of pair.warm.requests)request.fromDiskCache=false;
  const out=evaluateLineagePair(v1,v2,expected(),pair);assert.equal(out.passed,true,JSON.stringify(out.failures));assert.equal(out.samples[1].cacheHits,6);
});
test('derives the cold original-control profile without broadening pair admission',()=>{
  const pair=valid(),pins=expected(),font=pins.resources.at(-1);
  font.path='/assets/inter.woff2';pins.font.url=origin+font.path;pins.allowedPaths[pins.allowedPaths.length-1]=font.path;
  pair.cold.requests.at(-1).url=origin+font.path;pair.cold.serverRequests.at(-1).url=origin+font.path;
  const sample=deriveLineageSample(v1,pins,pair,'cold','original-control');assert.deepEqual(sample.assetFailures,[]);
  assert.equal(deriveLineageSample(v1,pins,pair,'cold').assetFailures.length>0,true);
});

rejects('rejects a wrong non-font browser body',pair=>pair.cold.requests[1].body.sha256=sha('f'),'assets.failed');
rejects('rejects a missing non-font body',pair=>pair.warm.requests[2].body={available:false,error:structuredClone(bodyError)},'assets.failed');
rejects('rejects a forged non-font server identity',pair=>pair.cold.serverRequests[3].bytes++,'assets.failed');
rejects('rejects a relabeled document',pair=>{for(const phaseName of ['cold','warm']){const phase=pair[phaseName];phase.documentURL=origin+'/source.css';phase.requests[0].type='Other';phase.requests[1].type='Document'}pair.warm.serverRequests[0]={...pair.warm.serverRequests[0],url:origin+'/source.css',bytes:102,sha256:sha('2')};},'assets.failed');
rejects('rejects a wrong resource type',pair=>pair.cold.requests[3].type='Other','assets.failed');
rejects('rejects representation header drift',pair=>pair.warm.requests[2].responseHeaders['cache-control']='no-store','assets.failed');
rejects('rejects an unbound resource manifest',(_pair,pins)=>pins.resources[4].sha256=sha('e'),'assets.failed');
rejects('rejects non-finite metrics',pair=>pair.warm.metrics.fcpMs=NaN,'sample.fcpMs');
rejects('rejects malformed unexpected-request facts',pair=>pair.cold.unexpectedRequests=null,'requests.unexpected');
rejects('rejects wrong rendered dimensions',pair=>pair.warm.render.pngWidth=391,'render.warm.dimensions');
rejects('rejects a missing warm phase',pair=>delete pair.warm,'lineage.phase.warm');
rejects('rejects Chrome warm font header elision despite passing numeric metrics',pair=>pair.warm.requests.at(-1).requestHeaders={referer:''},'lineage.warm.fontHeaders');
rejects('rejects unmatched Chrome Other loading failures despite passing numeric metrics',pair=>pair.cold.unexpectedRequests=['loading failed: unknown-request: net::ERR_CACHE_MISS'],'requests.unexpected');

test('rejects a weakened or different v2 workload without mutating v1',()=>{
  const original=structuredClone(v1),changed=structuredClone(v2);changed.budgets.warm.bodyBytes++;
  const out=evaluateLineagePair(v1,changed,expected(),valid());assert.equal(out.passed,false);
  assert.ok(codes(out).includes('contract.budgets'));assert.deepEqual(v1,original);
});
test('malformed resource inventory fails closed without throwing',()=>{
  const pins=expected();pins.resources=[null];
  assert.doesNotThrow(()=>evaluateLineagePair(v1,v2,pins,valid()));
  assert.equal(evaluateLineagePair(v1,v2,pins,valid()).passed,false);
});
test('oversized request/server arrays and malformed coordinates fail without unbounded traversal or throws',()=>{
  const pair=valid();pair.cold.requests=Array(10000).fill(pair.cold.requests[0]);pair.warm.serverRequests=Array(10000).fill(pair.warm.serverRequests[0]);
  assert.doesNotThrow(()=>evaluateLineagePair(v1,v2,expected(),pair));assert.equal(evaluateLineagePair(v1,v2,expected(),pair).passed,false);
  for(const field of ['widths','schemes']){const contract=structuredClone(v2);contract[field]={};assert.doesNotThrow(()=>evaluateLineagePair(v1,contract,expected(),valid()));assert.equal(evaluateLineagePair(v1,contract,expected(),valid()).passed,false)}
});
