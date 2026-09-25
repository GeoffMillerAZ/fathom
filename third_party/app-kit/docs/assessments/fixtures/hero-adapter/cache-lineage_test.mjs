import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import {readFile} from 'node:fs/promises';
import test from 'node:test';
import v1 from './performance-contract.json' with {type:'json'};
import v2 from './cache-lineage-contract.json' with {type:'json'};
import {checkCacheLineage} from './cache-lineage.mjs';

const digest=char=>char.repeat(64),immutable='private,max-age=3600,immutable';
const origin='http://127.0.0.1:43123',paths=['/enhanced','/source.css','/hero.css','/assets/logo-light.svg','/assets/logo-dark.svg','/assets/hero.jpg','/assets/inter.woff2'];
const fontSHA=digest('a'),bodyError={name:'Error',command:'Network.getResponseBody',message:'cdpSession.send: Protocol error (Network.getResponseBody): No data found for resource with given identifier'};
const expected=()=>({schema:1,origin,contextId:'isolated-1',browserPid:1234,manifestSHA:digest('b'),font:{url:origin+'/assets/inter.woff2',sha256:fontSHA,bytes:17,mime:'font/woff2',cacheControl:immutable},allowedPaths:[...paths],missingBodyError:structuredClone(bodyError)});
const fonts=()=>[{selector:'body > p:nth-child(1)',fonts:[{isCustomFont:true,familyName:'Inter Variable',postScriptName:'InterVariable',glyphCount:4}]}];
const response=(body,type,document=false)=>({etag:`"sha256-${body.sha256}"`,'content-type':type,'content-length':String(body.bytes),'cache-control':document?'no-store':immutable,'content-security-policy':"default-src 'none'",'x-content-type-options':'nosniff',connection:'close'});
function phase(name,base){
  const warm=name==='warm',loader=warm?'warm-loader':'cold-loader';
  const requests=paths.map((path,index)=>{
    const font=path.endsWith('.woff2'),document=index===0,requestSequence=base+index*4,body=font?(warm?{available:false,error:structuredClone(bodyError)}:{available:true,bytes:17,sha256:fontSHA}):{available:true,bytes:index+1,sha256:digest(String(index))};
    const responseSequence=requestSequence+(warm&&font?2:1),cacheSequence=warm&&font?requestSequence+1:null,headerBody=font?{bytes:17,sha256:fontSHA}:body;
    return {id:`${name}-${index}`,url:origin+path,method:'GET',type:document?'Document':font?'Font':'Other',frameId:'frame-1',loaderId:loader,status:200,redirect:false,fromDiskCache:false,fromPrefetchCache:false,fromServiceWorker:false,requestHeaders:{accept:'*/*'},responseHeaders:response(headerBody,font?'font/woff2':document?'text/html':'application/octet-stream',document),requestSequence,responseSequence,finishSequence:requestSequence+3,cacheSequence,body};
  });
  const serverPaths=warm?[paths[0]]:paths;
  return {documentURL:origin+'/enhanced',frameId:'frame-1',loaderId:loader,navigationType:'navigate',requests,serverRequests:serverPaths.map(path=>{const request=requests.find(item=>item.url===origin+path);return {url:origin+path,method:'GET',status:200,bytes:request.body.bytes,sha256:request.body.sha256};}),render:{pngSHA:digest('c'),geometry:[{x:0,y:0,width:390,height:1000}],paintedFonts:fonts(),fontReady:true}};
}
const valid=()=>({origin,contextId:'isolated-1',browserPid:1234,manifestBefore:digest('b'),manifestAfter:digest('b'),setup:{freshContext:true,cacheClearedBeforeCold:true,serviceWorkersBlocked:true,interceptionEnabled:false,applicationScripts:0,priorRequests:0},navigationCount:2,cold:phase('cold',10),warm:phase('warm',100)});
const reject=(name,mutate)=>test(name,()=>{const pair=valid();mutate(pair);const out=checkCacheLineage(expected(),pair);assert.equal(out.matched,false);assert.ok(out.failures.length);});

test('v2 preserves the v1 workload and exact frozen v1 contract hash',async()=>{
  const bytes=await readFile(new URL('./performance-contract.json',import.meta.url));
  assert.equal(createHash('sha256').update(bytes).digest('hex'),v2.strictV1ContractSHA256);
  for(const key of ['widths','schemes','repetitions','phases','viewportHeight','network','cpuSlowdown'])assert.deepEqual(v2[key],v1[key],key);
  assert.deepEqual(v2.budgets,v1.budgets);
});
test('accepts cache event before response without claiming direct warm bytes',()=>{
  const input=valid(),before=structuredClone(input),out=checkCacheLineage(expected(),input);
  assert.deepEqual(input,before);assert.deepEqual({...out,failures:[]},{identityMethod:'isolated-cache-lineage',directWarmBodyObserved:false,verification:'not-verified',matched:true,failures:[],coldRequestId:'cold-6',warmRequestId:'warm-6'});
});
test('accepts identical font header maps with a different insertion order',()=>{const pair=valid(),h=pair.warm.requests[6].responseHeaders;pair.warm.requests[6].responseHeaders=Object.fromEntries(Object.entries(h).reverse());assert.equal(checkCacheLineage(expected(),pair).matched,true);});
test('invalid expected input fails closed',()=>{for(const mutate of [x=>x.schema=2,x=>x.origin='https://x',x=>x.allowedPaths.pop(),x=>x.font.cacheControl='public,max-age=1',x=>x.missingBodyError={name:'Error'}]){const input=expected();mutate(input);assert.equal(checkCacheLineage(input,valid()).matched,false);}});
reject('rejects wrong cold font bytes',p=>p.cold.requests[6].body.bytes=18);
reject('rejects prior prime activity',p=>p.setup.priorRequests=1);
reject('rejects changed font response headers',p=>p.warm.requests[6].responseHeaders.etag='"other"');
reject('rejects missing cold cache fact on CSS',p=>p.cold.requests[1].fromDiskCache=true);
reject('rejects absent font cache event',p=>p.warm.requests[6].cacheSequence=null);
reject('rejects cache event after finish',p=>p.warm.requests[6].cacheSequence=p.warm.requests[6].finishSequence);
reject('rejects service worker traffic',p=>p.warm.requests[6].fromServiceWorker=true);
reject('rejects prefetch traffic',p=>p.warm.requests[6].fromPrefetchCache=true);
reject('rejects warm font server contact',p=>p.warm.serverRequests.push({url:origin+'/assets/inter.woff2',method:'GET',status:200,bytes:17,sha256:fontSHA}));
reject('rejects document cache or alternate document URL',p=>{p.warm.requests[0].fromDiskCache=true;p.warm.documentURL=origin+'/other';});
reject('rejects reused loader / BFCache substitute',p=>p.warm.loaderId=p.cold.loaderId);
reject('rejects system font evidence',p=>p.warm.render.paintedFonts[0].fonts[0].familyName='Lucida Grande');
reject('rejects manifest drift',p=>p.manifestAfter=digest('d'));
reject('rejects NaN geometry',p=>p.warm.render.geometry[0].width=NaN);
reject('rejects more than 512 geometry records',p=>p.warm.render.geometry=Array.from({length:513},()=>({x:0,y:0,width:1,height:1})));
reject('rejects null request',p=>p.warm.requests[2]=null);
reject('rejects duplicate request path',p=>p.warm.requests[2].url=origin+'/source.css');
reject('rejects duplicate request ID across phases',p=>p.warm.requests[0].id='cold-0');
reject('rejects duplicate global event sequences',p=>p.warm.requests[2].cacheSequence=p.warm.requests[2].responseSequence);
reject('rejects range, proxy authorization and validators',p=>Object.assign(p.warm.requests[6].requestHeaders,{range:'bytes=0-1','proxy-authorization':'x','if-none-match':'x'}));
reject('rejects Vary and encoded responses',p=>{p.cold.requests[6].responseHeaders.vary='Accept-Encoding';p.cold.requests[1].responseHeaders['content-encoding']='gzip';});
reject('rejects Set-Cookie response header',p=>p.warm.requests[1].responseHeaders['set-cookie']='x=y');
reject('rejects malformed null request headers without throwing',p=>p.warm.requests[2].requestHeaders=null);
reject('rejects malformed numeric response headers without throwing',p=>p.warm.requests[2].responseHeaders=7);
reject('rejects malformed font response headers without throwing',p=>p.warm.requests[6].responseHeaders=null);
reject('rejects oversized request header values',p=>p.warm.requests[2].requestHeaders.accept='x'.repeat(8193));
reject('rejects unbounded painted-font arrays',p=>p.warm.render.paintedFonts[0].fonts=Array.from({length:65},()=>p.warm.render.paintedFonts[0].fonts[0]));
reject('rejects a substituted warm body',p=>p.warm.requests[6].body={available:true,bytes:17,sha256:fontSHA});
reject('rejects a changed missing-body error',p=>p.warm.requests[6].body.error.message='different');
reject('rejects cross-phase event overlap',p=>p.warm.requests[0].requestSequence=20);
reject('rejects unequal PNG identity',p=>p.warm.render.pngSHA=digest('d'));
reject('rejects unequal geometry',p=>p.warm.render.geometry[0].width=391);
